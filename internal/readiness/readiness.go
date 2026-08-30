package readiness

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

type Check func(context.Context) error

type Dependency struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	err    error
}

type Report struct {
	Ready        bool         `json:"ready"`
	Dependencies []Dependency `json:"dependencies"`
}

type Checker struct {
	checks  map[string]Check
	timeout time.Duration
	mu      sync.Mutex
	running map[string]struct{}
}

func New(checks map[string]Check, timeout time.Duration) *Checker {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	cloned := make(map[string]Check, len(checks))
	for name, check := range checks {
		if name != "" && check != nil {
			cloned[name] = check
		}
	}
	return &Checker{checks: cloned, timeout: timeout, running: make(map[string]struct{}, len(cloned))}
}

func (c *Checker) Run(ctx context.Context) Report {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil || len(c.checks) == 0 {
		return Report{Ready: true, Dependencies: []Dependency{}}
	}

	results := make([]Dependency, 0, len(c.checks))
	resultCh := make(chan Dependency, len(c.checks))
	for name, check := range c.checks {
		name, check := name, check
		if !c.tryStart(name) {
			// A previous probe is still executing. This can only happen when a
			// custom check ignores cancellation; do not start another goroutine
			// on every readiness request.
			resultCh <- Dependency{Name: name, Status: "timeout", err: context.DeadlineExceeded}
			continue
		}
		go c.runCheck(ctx, name, check, resultCh)
	}

	report := Report{Ready: true, Dependencies: results}
	for range c.checks {
		result := <-resultCh
		if result.Status != "ok" {
			report.Ready = false
		}
		report.Dependencies = append(report.Dependencies, result)
	}
	sort.Slice(report.Dependencies, func(i, j int) bool {
		return report.Dependencies[i].Name < report.Dependencies[j].Name
	})
	return report
}

func (c *Checker) tryStart(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.running[name]; exists {
		return false
	}
	c.running[name] = struct{}{}
	return true
}

func (c *Checker) finish(name string) {
	c.mu.Lock()
	delete(c.running, name)
	c.mu.Unlock()
}

func (c *Checker) runCheck(ctx context.Context, name string, check Check, resultCh chan<- Dependency) {
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	checkResult := make(chan error, 1)
	go func() {
		defer c.finish(name)
		checkResult <- check(checkCtx)
	}()

	var err error
	select {
	case err = <-checkResult:
	case <-checkCtx.Done():
		err = checkCtx.Err()
	}
	status := "ok"
	if err != nil {
		status = "unavailable"
		if errors.Is(err, context.DeadlineExceeded) {
			status = "timeout"
		}
	}
	resultCh <- Dependency{Name: name, Status: status, err: err}
}

func (r Report) Errors() map[string]error {
	errs := map[string]error{}
	for _, dependency := range r.Dependencies {
		if dependency.err != nil {
			errs[dependency.Name] = dependency.err
		}
	}
	return errs
}
