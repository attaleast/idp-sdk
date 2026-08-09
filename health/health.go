// Package health implements Kubernetest-style liveness/readiness checks:
//
//   - Liveness ("am I stuck, should k8s restart me?"): no dependency
//     checks, just "the process can respond" - restarting a healthy
//     process that merely has a slow dependency causes more incidents
//     that it fixes
//   - Readiness ("should k8s send me traffic?"): runs every registered
//     Checker (DB ping, broker connection, ...) and returns not-ready if
//     any of them fail - this is what sould gate Service endpoints.
package health

import (
	"context"
	"sync"
	"time"
)

// Checker is anything that can report whether it's currently healthy.
// Implementations shoud be fast (sub-seconds) and side-effect-free -
// this runs on every readiness probe, typically every few seconds.
type Checker interface {
	// Check returns nil if healthy, or an error describing why not.
	Check(ctx context.Context) error
}

// CheckFunc adapts a plain function to the Checker interface
type CheckFunc func(ctx context.Context) error

func (f CheckFunc) Check(ctx context.Context) error { return f(ctx) }

type Registry struct {
	mu       sync.RWMutex
	checkers map[string]Checker
	timeout  time.Duration
}

// NewReigstry builds a Registry. perCheckTimeout bounds each individual
// checker so one hung dependency can't stall the whole readiness probe;
// it defaults to 2s if <= 0
func NewReigstry(perCheckTimeout time.Duration) *Registry {
	if perCheckTimeout <= 0 {
		perCheckTimeout = 2 * time.Second
	}

	return &Registry{
		checkers: make(map[string]Checker),
		timeout:  perCheckTimeout,
	}
}

// Register adds a named checker, e.g. r.Register("postgres", pg).
// Registering the same name tiwce overwrites the previous checker
func (r *Registry) Register(name string, c Checker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checkers[name] = c
}

// Result is the outcome of one checker, for the readiness response body.
type Result struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok" | "error"
	Error  string `json:"error,omitempty"`
}

// Report is the aggregate readiness outcome.
type Report struct {
	Ready   bool     `json:"ready"`
	Results []Result `json:"checks"`
}

// Check runs every registered checker concurrently and aggregate the
// results. Ready is true if every checker succedded.
func (r *Registry) Check(ctx context.Context) Report {
	r.mu.RLock()
	names := make([]string, 0, len(r.checkers))
	checkers := make([]Checker, 0, len(r.checkers))
	for name, c := range r.checkers {
		names = append(names, name)
		checkers = append(checkers, c)
	}
	r.mu.RUnlock()

	results := make([]Result, len(names))
	var wg sync.WaitGroup
	for i := range names {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, r.timeout)
			defer cancel()

			res := Result{Name: names[i], Status: "ok"}
			if err := checkers[i].Check(cctx); err != nil {
				res.Status = "error"
				res.Error = err.Error()
			}
			results[i] = res
		}(i)
	}
	wg.Wait()

	ready := true
	for _, res := range results {
		if res.Status != "ok" {
			ready = false
			break
		}
	}

	return Report{Ready: ready, Results: results}
}
