package readiness

import (
	"context"
	"errors"
	"sort"
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
	return &Checker{checks: cloned, timeout: timeout}
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
		go func() {
			checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()

			checkResult := make(chan error, 1)
			go func() {
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
		}()
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

func (r Report) Errors() map[string]error {
	errs := map[string]error{}
	for _, dependency := range r.Dependencies {
		if dependency.err != nil {
			errs[dependency.Name] = dependency.err
		}
	}
	return errs
}
