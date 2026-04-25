# conquer

**Structured concurrency for Go.** Zero goroutine leaks by construction. Comparable performance to `errgroup` with panic recovery, generics, and 20+ concurrency primitives.

```go
go get github.com/cagatay35/conquer
```

Requires **Go 1.22+**.

---

## Why conquer?

| Feature | `sync.WaitGroup` | `errgroup` | **conquer** |
|---|---|---|---|
| Panic recovery | No | No | **Yes, with stack trace** |
| Goroutine leak prevention | No | Partial | **By construction** |
| Error collection | No | First only | **First or All** |
| Generics support | No | No | **Full** |
| Concurrency limiting | No | Yes | **Yes** |
| Nested scopes | No | No | **Yes** |
| Async/Await | No | No | **Yes** |
| Pipeline | No | No | **Yes** |
| Retry / Race | No | No | **Yes** |
| Observability hooks | No | No | **Yes** |

---

## Quick Start

The simplest way to run managed goroutines:

```go
// Single goroutine with panic recovery
err := conquer.Go(ctx, func(ctx context.Context) error {
    return doWork(ctx)
})

// Two goroutines in parallel
err := conquer.Go2(ctx, fetchUser, fetchOrders)

// N goroutines in parallel
err := conquer.GoN(ctx, tasks...)
```

For more control, use `Run` with a `Scope`:

```go
err := conquer.Run(ctx, func(s *conquer.Scope) error {
    s.Go(func(ctx context.Context) error { return task1(ctx) })
    s.Go(func(ctx context.Context) error { return task2(ctx) })
    return nil
})
// ALL goroutines guaranteed complete here. No exceptions.
```

---

## Complete API Reference

### Scope -- Goroutine Lifecycle Management

```go
// Run creates a scope, runs fn, waits for all goroutines, returns errors
err := conquer.Run(ctx, func(s *conquer.Scope) error {
    s.Go(func(ctx context.Context) error { return work(ctx) })

    // Nested scope with its own concurrency limit
    s.Child(func(cs *conquer.Scope) error {
        cs.Go(func(ctx context.Context) error { return childWork(ctx) })
        return nil
    }, conquer.WithMaxGoroutines(3))

    return nil
}, conquer.WithTimeout(30*time.Second))
```

### Pool -- Bounded Worker Pool

```go
pool := conquer.NewPool(ctx, conquer.WithMaxGoroutines(10))

for _, item := range items {
    item := item
    pool.Go(func(ctx context.Context) error {
        return process(ctx, item)
    })
}

err := pool.Wait() // blocks until all complete
```

### ResultPool -- Typed Results in Submission Order

```go
rp := conquer.NewResultPool[*User](ctx, conquer.WithMaxGoroutines(5))

for _, id := range userIDs {
    id := id
    rp.Go(func(ctx context.Context) (*User, error) {
        return fetchUser(ctx, id)
    })
}

users, err := rp.Wait() // results in submission order
```

### Map / ForEach / Filter / First / Reduce

```go
// Concurrent transform -- results in input order
users, err := conquer.Map(ctx, userIDs, func(ctx context.Context, id int64) (*User, error) {
    return userService.Get(ctx, id)
}, conquer.WithMaxGoroutines(10))

// Concurrent side effects
err := conquer.ForEach(ctx, items, func(ctx context.Context, item Item) error {
    return sendNotification(ctx, item)
})

// Concurrent filter
active, err := conquer.Filter(ctx, users, func(ctx context.Context, u *User) (bool, error) {
    return u.IsActive(ctx)
})

// First successful result -- cancels the rest
result, err := conquer.First(ctx, endpoints, func(ctx context.Context, ep string) (*Resp, error) {
    return http.Get(ctx, ep)
})

// Map + sequential reduce
sum, err := conquer.Reduce(ctx, items, 0,
    func(ctx context.Context, item Item) (int, error) { return item.Value, nil },
    func(a, b int) int { return a + b },
)
```

### Async / Future -- Async-Await Pattern

```go
err := conquer.Run(ctx, func(s *conquer.Scope) error {
    userF := conquer.Async(s, func(ctx context.Context) (*User, error) {
        return fetchUser(ctx, id)
    })
    ordersF := conquer.Async(s, func(ctx context.Context) ([]Order, error) {
        return fetchOrders(ctx, id)
    })

    user, err := userF.Await(ctx)
    if err != nil { return err }
    orders, err := ordersF.Await(ctx)
    if err != nil { return err }

    return buildResponse(user, orders)
})
```

### Race -- First Successful Result

```go
body, err := conquer.Race(ctx,
    func(ctx context.Context) ([]byte, error) { return fetchFromCDN1(ctx) },
    func(ctx context.Context) ([]byte, error) { return fetchFromCDN2(ctx) },
    func(ctx context.Context) ([]byte, error) { return fetchFromCDN3(ctx) },
)
```

### Retry -- Automatic Retry with Backoff

```go
// Exponential backoff: 100ms, 200ms, 400ms...
resp, err := conquer.Retry(ctx, 5, 100*time.Millisecond,
    func(ctx context.Context) (*http.Response, error) {
        return client.Do(ctx, req)
    })

// Custom backoff strategy
resp, err := conquer.RetryWithFn(ctx, 5,
    func(attempt int) time.Duration {
        return time.Duration(attempt*attempt) * time.Second // quadratic
    },
    func(ctx context.Context) (*http.Response, error) {
        return client.Do(ctx, req)
    })
```

### Pipeline -- Multi-Stage Processing

```go
out, errCh := conquer.Pipeline3(ctx, source,
    conquer.NewStage("validate", validateRecord, 4),  // 4 workers
    conquer.NewStage("enrich", enrichRecord, 8),       // 8 workers
    conquer.NewStage("persist", saveRecord, 2),        // 2 workers
)

for record := range out {
    fmt.Println(record)
}
if err := <-errCh; err != nil {
    log.Fatal(err)
}
```

### FanOut / FanIn / Merge

```go
// Distribute work across 5 workers
results := conquer.FanOut(ctx, inputCh, 5, processItem)
values, err := conquer.Drain(results)

// Merge multiple channels into one
merged := conquer.FanIn(ctx, ch1, ch2, ch3)

// Generate a source channel
source := conquer.Generate(ctx, func(ctx context.Context) (int, bool) {
    return nextItem(), hasMore()
})
```

### Tee -- Split a Channel

```go
// Split one channel into two
ch1, ch2 := conquer.Tee2(ctx, source)

// Split into N channels
channels := conquer.TeeN(ctx, source, 5)
```

### Throttle -- Rate Limiting

```go
// At most 1 call per 100ms
throttled := conquer.Throttle(100*time.Millisecond, callExternalAPI)
result, err := throttled(ctx, request)
```

### Batch Processor

```go
bp := conquer.NewBatchProcessor(ctx, func(ctx context.Context, batch []Event) error {
    return bulkInsert(ctx, batch)
}, conquer.BatchSize(100), conquer.BatchInterval(5*time.Second))

for _, event := range events {
    bp.Add(event)
}
bp.Close() // flushes remaining
```

### Chunk / MapChunked -- Batch Processing Helpers

```go
// Split a slice into chunks
chunks := conquer.Chunk(items, 100)

// Process in chunks concurrently -- useful for batch APIs
results, err := conquer.MapChunked(ctx, userIDs, 50,
    func(ctx context.Context, ids []int64) ([]*User, error) {
        return userService.BatchGet(ctx, ids) // batch API
    },
    conquer.WithMaxGoroutines(5),
)
```

### Channel Utilities

```go
// Collect all values from a channel into a slice
items := conquer.Collect(ctx, ch)

// Send all items from a slice to a channel
err := conquer.SendAll(ctx, ch, items)
```

---

## Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithMaxGoroutines(n)` | Max concurrent goroutines | Unlimited |
| `WithErrorStrategy(s)` | `FailFast` or `CollectAll` | `FailFast` |
| `WithTimeout(d)` | Scope timeout | No timeout |
| `WithPanicHandler(fn)` | Callback on panic recovery | None |
| `WithMetrics(m)` | Observability sink | NoopMetrics |
| `WithName(s)` | Scope name for metrics/debugging | "" |

**FailFast** (default): First error cancels the scope's context, signalling all goroutines to stop.

**CollectAll**: All goroutines run to completion. Errors are collected and returned as `*MultiError`.

---

## Using conquer in Your Project

### 1. Installation

```bash
cd your-project
go get github.com/cagatay35/conquer
```

### 2. Import

```go
import "github.com/cagatay35/conquer"
```

### 3. Replace errgroup

**Before (errgroup):**
```go
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return fetchUser(ctx) })
g.Go(func() error { return fetchOrders(ctx) })
err := g.Wait()
// If fetchUser panics -> entire process crashes
```

**After (conquer):**
```go
err := conquer.Go2(ctx, fetchUser, fetchOrders)
// If fetchUser panics -> recovered, returned as *PanicError with stack trace
```

### 4. Replace WaitGroup

**Before (sync.WaitGroup):**
```go
var wg sync.WaitGroup
var mu sync.Mutex
var errs []error
for _, url := range urls {
    url := url
    wg.Add(1)
    go func() {
        defer wg.Done()
        defer func() {
            if r := recover(); r != nil { /* manual recovery */ }
        }()
        if err := fetch(url); err != nil {
            mu.Lock()
            errs = append(errs, err)
            mu.Unlock()
        }
    }()
}
wg.Wait()
```

**After (conquer):**
```go
err := conquer.ForEach(ctx, urls, func(ctx context.Context, url string) error {
    return fetch(ctx, url)
}, conquer.WithMaxGoroutines(10))
```

### 5. Real-World Example: API Handler

```go
func GetUserProfile(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    userID := chi.URLParam(r, "id")

    err := conquer.Run(ctx, func(s *conquer.Scope) error {
        userF := conquer.Async(s, func(ctx context.Context) (*User, error) {
            return userRepo.FindByID(ctx, userID)
        })
        ordersF := conquer.Async(s, func(ctx context.Context) ([]Order, error) {
            return orderRepo.FindByUser(ctx, userID)
        })
        statsF := conquer.Async(s, func(ctx context.Context) (*Stats, error) {
            return statsService.GetUserStats(ctx, userID)
        })

        user, err := userF.Await(ctx)
        if err != nil { return err }
        orders, err := ordersF.Await(ctx)
        if err != nil { return err }
        stats, err := statsF.Await(ctx)
        if err != nil { return err }

        json.NewEncoder(w).Encode(ProfileResponse{
            User:   user,
            Orders: orders,
            Stats:  stats,
        })
        return nil
    }, conquer.WithTimeout(5*time.Second))

    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}
```

### 6. Real-World Example: ETL Job

```go
func RunETLJob(ctx context.Context, db *sql.DB, es *elastic.Client) error {
    source := make(chan RawRecord, 100)

    // Producer
    go func() {
        defer close(source)
        rows, _ := db.QueryContext(ctx, "SELECT * FROM raw_data")
        defer rows.Close()
        for rows.Next() {
            var r RawRecord
            rows.Scan(&r.ID, &r.Data)
            source <- r
        }
    }()

    // 3-stage pipeline: validate -> transform -> index
    out, errCh := conquer.Pipeline3(ctx, source,
        conquer.NewStage("validate", func(ctx context.Context, r RawRecord) (RawRecord, error) {
            if r.Data == "" { return r, fmt.Errorf("empty record %d", r.ID) }
            return r, nil
        }, 2),
        conquer.NewStage("transform", func(ctx context.Context, r RawRecord) (ESDoc, error) {
            return ESDoc{ID: r.ID, Body: transform(r.Data)}, nil
        }, 8),
        conquer.NewStage("index", func(ctx context.Context, doc ESDoc) (string, error) {
            _, err := es.Index().BodyJson(doc).Do(ctx)
            return doc.ID, err
        }, 4),
    )

    count := 0
    for range out {
        count++
    }

    log.Printf("Indexed %d documents", count)
    return <-errCh
}
```

### 7. Real-World Example: Web Crawler

```go
func Crawl(ctx context.Context, seeds []string, maxDepth int) ([]Page, error) {
    var mu sync.Mutex
    visited := make(map[string]bool)
    var pages []Page

    var crawlLevel func(urls []string, depth int) error
    crawlLevel = func(urls []string, depth int) error {
        if depth > maxDepth { return nil }

        results, err := conquer.Map(ctx, urls, func(ctx context.Context, url string) (*Page, error) {
            resp, err := conquer.Retry(ctx, 3, time.Second, func(ctx context.Context) (*http.Response, error) {
                return http.Get(url)
            })
            if err != nil { return nil, err }
            return parsePage(resp)
        }, conquer.WithMaxGoroutines(10))

        if err != nil { return err }

        var nextURLs []string
        for _, page := range results {
            mu.Lock()
            if !visited[page.URL] {
                visited[page.URL] = true
                pages = append(pages, *page)
                nextURLs = append(nextURLs, page.Links...)
            }
            mu.Unlock()
        }

        return crawlLevel(nextURLs, depth+1)
    }

    return pages, crawlLevel(seeds, 0)
}
```

---

## Testing Utilities

```go
import "github.com/cagatay35/conquer/scopetest"

func TestMyService(t *testing.T) {
    defer scopetest.CheckLeaks(t) // fails if goroutines leak

    err := myService.Process(context.Background())

    scopetest.AssertPanic(t, err)           // assert error wraps a panic
    scopetest.AssertNoPanic(t, err)         // assert no panics
    scopetest.AssertMultiError(t, err, 3)   // assert 3 collected errors
}
```

---

## Benchmarks

Apple M1 Max, Go 1.25:

```
                          ns/op     B/op    allocs/op
conquer.Run + 1 Go()       593      312         4
errgroup + 1 Go()          551      184         4      <-- almost identical

conquer.Run + 10 Go()    3,735      528        13
errgroup + 10 Go()       2,978      400        13

conquer.Run + 100 Go()  44,991    2,716       103
errgroup + 100 Go()     33,911    2,560       103

conquer.Go()               607      312         4      <-- single goroutine shortcut
conquer.Go2()              913      336         5
conquer.Race(3)          2,268    1,321        18
conquer.Chunk(10000)       737    2,688         1      <-- zero-copy chunking
```

For 1 goroutine, conquer is only **7% slower** than bare errgroup while providing panic recovery, error strategies, metrics hooks, and generics.

At scale, the remaining overhead comes from `context.WithCancelCause` (required for FailFast cancellation). The allocation count is identical to errgroup.

---

## Safety Guarantees

- **Zero goroutine leaks**: Scopes cannot escape `Run()` -- goroutines can never outlive their owner
- **Panic containment**: All panics are recovered with full stack traces as `*PanicError`
- **Race-free**: All shared state synchronized; all tests pass with `-race`
- **No `unsafe`**: Zero use of unsafe.Pointer
- **Context-aware**: All blocking operations respect context cancellation
- **Bounded resources**: `WithMaxGoroutines` enforces limits via semaphore
- **Task object pooling**: Global `sync.Pool` recycles goroutine wrappers to reduce GC pressure

---

## Author

**Çağatay Gökçel** - Technical Lead

---

## License

MIT
