package conquer

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/cagatay35/conquer/internal/panicutil"
	"github.com/cagatay35/conquer/internal/semaphore"
	"github.com/cagatay35/conquer/internal/taskpool"
)

const (
	scopeActive  uint32 = iota
	scopeClosing
	scopeClosed
)

// Scope manages the lifecycle of a group of goroutines. It guarantees that
// all goroutines launched via Go() complete before the owning Run() returns.
//
// A Scope cannot be created directly; use Run to create one.
type Scope struct {
	ctx    context.Context
	cancel context.CancelCauseFunc

	wg sync.WaitGroup

	mu   sync.Mutex
	errs []error

	state atomic.Uint32
	count atomic.Int64

	sem     *semaphore.Weighted
	opts    options
	metrics Metrics
	hasObs  bool // true if non-noop metrics or panicHandler set
}

// Run creates a new Scope, executes fn with it, waits for all goroutines
// launched via Scope.Go to complete, and returns any errors.
//
// The Scope's context is derived from ctx. If the scope has a timeout
// (via WithTimeout), the context will be cancelled when the timeout elapses.
//
// Run guarantees:
//   - All goroutines have completed when Run returns
//   - All panics are recovered and returned as *PanicError
//   - If ErrorStrategy is FailFast (default), the first error cancels remaining goroutines
//   - The Scope never escapes the fn callback, preventing misuse
func Run(ctx context.Context, fn func(s *Scope) error, opts ...Option) error {
	if len(opts) == 0 {
		return runScopeFast(ctx, fn)
	}
	o := resolveOptions(opts)
	return runScope(ctx, fn, o)
}

// runScopeFast is the zero-overhead fast path for Run() with no options.
// Avoids option resolution and uses the default FailFast strategy.
func runScopeFast(ctx context.Context, fn func(s *Scope) error) error {
	scopeCtx, cancel := context.WithCancelCause(ctx)

	s := &Scope{
		ctx:     scopeCtx,
		cancel:  cancel,
		metrics: defaultMetrics,
	}

	var fnErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				fnErr = &PanicError{
					Value: r,
					Stack: panicutil.CaptureStack(2),
				}
			}
		}()
		fnErr = fn(s)
	}()

	s.state.Store(scopeClosing)
	s.wg.Wait()
	s.state.Store(scopeClosed)
	cancel(nil)

	if fnErr != nil {
		if len(s.errs) == 0 {
			return fnErr
		}
		return joinErrors(append([]error{fnErr}, s.errs...))
	}

	switch len(s.errs) {
	case 0:
		return nil
	case 1:
		return s.errs[0]
	default:
		return &MultiError{Errors: s.errs}
	}
}

func runScope(ctx context.Context, fn func(s *Scope) error, o options) error {
	scopeCtx, cancel := context.WithCancelCause(ctx)

	if o.timeout > 0 {
		var timeoutCancel context.CancelFunc
		scopeCtx, timeoutCancel = context.WithTimeout(scopeCtx, o.timeout)
		defer timeoutCancel()
	}

	m := o.metrics
	hasObs := m != nil || o.panicHandler != nil
	if m == nil {
		m = defaultMetrics
	}

	s := &Scope{
		ctx:     scopeCtx,
		cancel:  cancel,
		opts:    o,
		metrics: m,
		hasObs:  hasObs,
	}

	if o.maxGoroutines > 0 {
		s.sem = semaphore.New(o.maxGoroutines)
	}

	if hasObs {
		m.ScopeCreated(o.name)
	}

	var fnErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				pe := &PanicError{
					Value: r,
					Stack: panicutil.CaptureStack(2),
				}
				fnErr = pe
				if o.panicHandler != nil {
					o.panicHandler(pe)
				}
			}
		}()
		fnErr = fn(s)
	}()

	s.state.Store(scopeClosing)
	s.wg.Wait()
	s.state.Store(scopeClosed)

	cancel(nil)

	if hasObs {
		m.ScopeClosed(o.name, 0, s.count.Load())
	}

	s.mu.Lock()
	errs := s.errs
	s.mu.Unlock()

	if fnErr != nil {
		errs = append([]error{fnErr}, errs...)
	}

	return joinErrors(errs)
}

// Go launches fn in a new goroutine managed by this Scope.
//
// The goroutine receives a context derived from the Scope's context.
// When the scope is cancelled, this context will be cancelled too.
//
// Panics in fn are recovered and captured as *PanicError.
// If the scope is closing or closed, Go records ErrScopeClosed
// and does not launch a goroutine.
//
// If WithMaxGoroutines is set, Go blocks until a slot is available
// or the scope's context is cancelled.
func (s *Scope) Go(fn func(ctx context.Context) error) {
	if fn == nil {
		return
	}

	if s.state.Load() != scopeActive {
		s.addError(ErrScopeClosed)
		return
	}

	s.wg.Add(1)
	s.count.Add(1)

	if s.sem != nil {
		if err := s.sem.Acquire(s.ctx); err != nil {
			s.wg.Done()
			s.addError(err)
			return
		}
	}

	task := taskpool.Get()
	task.Fn = fn

	go s.runTask(task)
}

func (s *Scope) runTask(task *taskpool.Task) {
	// Single combined defer for minimum overhead.
	defer func() {
		r := recover()
		if r != nil {
			pe := &PanicError{
				Value: r,
				Stack: panicutil.CaptureStack(2),
			}
			if s.hasObs {
				s.metrics.GoroutinePanicked(s.opts.name, pe)
				if s.opts.panicHandler != nil {
					s.opts.panicHandler(pe)
				}
			}
			s.addError(pe)
			if s.opts.errorStrategy == FailFast {
				s.cancel(pe)
			}
		}
		if s.sem != nil {
			s.sem.Release()
		}
		taskpool.Put(task)
		s.wg.Done()
	}()

	err := task.Fn(s.ctx)
	if err != nil {
		s.addError(err)
		if s.opts.errorStrategy == FailFast {
			s.cancel(err)
		}
	}
}

// Child creates a nested child scope within the current scope.
// The child inherits the parent's context, so cancelling the parent
// also cancels the child. The parent waits for the child scope to
// complete as part of its own lifecycle.
//
// Child scope options are independent from the parent's options.
func (s *Scope) Child(fn func(cs *Scope) error, opts ...Option) error {
	o := resolveOptions(opts)

	var childErr error
	s.Go(func(ctx context.Context) error {
		childErr = runScope(ctx, fn, o)
		return childErr
	})

	return nil
}

func (s *Scope) addError(err error) {
	s.mu.Lock()
	s.errs = append(s.errs, err)
	s.mu.Unlock()
}

func newInternalSemaphore(n int) *semaphore.Weighted {
	return semaphore.New(n)
}
