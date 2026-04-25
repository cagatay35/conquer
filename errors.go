package conquer

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors.
var (
	// ErrScopeClosed is returned when Go() is called on a scope that is
	// already closing or closed.
	ErrScopeClosed = errors.New("conquer: scope is closed")
)

// PanicError wraps a recovered panic value with the goroutine's stack trace
// at the point of the panic. It implements the error interface and supports
// unwrapping if the underlying panic value is itself an error.
type PanicError struct {
	// Value is the original value passed to panic().
	Value any
	// Stack is the raw stack trace captured via runtime.Stack().
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("conquer: panic recovered: %v\n\ngoroutine stack:\n%s", e.Value, e.Stack)
}

// Unwrap returns the underlying error if the panic value is an error,
// enabling errors.Is and errors.As to work through panic boundaries.
func (e *PanicError) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}

// MultiError holds multiple errors collected from concurrent goroutines.
// It implements the multi-unwrap interface (Go 1.20+) so that errors.Is
// and errors.As traverse all contained errors.
type MultiError struct {
	Errors []error
}

func (e *MultiError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "conquer: %d errors occurred:\n", len(e.Errors))
	for i, err := range e.Errors {
		fmt.Fprintf(&b, "  [%d] %s\n", i, err.Error())
	}
	return b.String()
}

// Unwrap returns the list of contained errors, satisfying the
// implicit multi-error interface used by errors.Is and errors.As
// since Go 1.20.
func (e *MultiError) Unwrap() []error {
	return e.Errors
}

// joinErrors combines a slice of errors into a single error.
// Returns nil if no errors, the single error if only one,
// or a *MultiError if multiple.
func joinErrors(errs []error) error {
	filtered := errs[:0:0]
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}

	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return &MultiError{Errors: filtered}
	}
}
