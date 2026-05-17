// Package health provides health check aggregation for services with multiple
// dependencies. It supports registering component health checks and computing
// an overall system health status.
package health

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Status represents the health status of a component or the overall system.
type Status string

const (
	// StatusHealthy indicates the component is fully operational.
	StatusHealthy Status = "healthy"
	// StatusDegraded indicates the component is operational with reduced
	// capability.
	StatusDegraded Status = "degraded"
	// StatusUnhealthy indicates the component is not operational.
	StatusUnhealthy Status = "unhealthy"
)

// CheckFunc is a function that checks the health of a component.
// It returns an error if the component is unhealthy.
type CheckFunc func(ctx context.Context) error

// ComponentResult holds the health check result for a single component.
type ComponentResult struct {
	// Name is the component identifier.
	Name string `json:"name"`
	// Status is the health status.
	Status Status `json:"status"`
	// Message provides additional detail about the status.
	Message string `json:"message,omitempty"`
	// Duration is how long the health check took.
	Duration time.Duration `json:"duration"`
	// LastChecked is when this check was last performed.
	LastChecked time.Time `json:"last_checked"`
}

// Report holds the aggregated health check results.
type Report struct {
	// Status is the overall system status.
	Status Status `json:"status"`
	// Components contains individual component results.
	Components []ComponentResult `json:"components"`
	// Timestamp is when this report was generated.
	Timestamp time.Time `json:"timestamp"`
}

// component stores a registered health check.
type component struct {
	name     string
	check    CheckFunc
	required bool
}

// Checker interface defines the contract for health check implementations.
type Checker interface {
	// Check performs a health check and returns the overall status.
	Check(ctx context.Context) *Report
}

// Aggregator combines multiple component health checks into an overall
// system health status. Required components cause the system to be
// unhealthy if they fail; optional components cause degraded status.
type Aggregator struct {
	mu         sync.RWMutex
	components []component
	timeout    time.Duration
}

// AggregatorConfig configures the health check aggregator.
type AggregatorConfig struct {
	// Timeout is the maximum duration for each individual health check.
	// Defaults to 5 seconds.
	Timeout time.Duration
}

// DefaultAggregatorConfig returns an AggregatorConfig with sensible defaults.
func DefaultAggregatorConfig() *AggregatorConfig {
	return &AggregatorConfig{
		Timeout: 5 * time.Second,
	}
}

// NewAggregator creates a new health check aggregator.
func NewAggregator(config *AggregatorConfig) *Aggregator {
	if config == nil {
		config = DefaultAggregatorConfig()
	}

	return &Aggregator{
		timeout: config.Timeout,
	}
}

// Register adds a required component health check. If this component
// fails, the overall status will be Unhealthy.
func (a *Aggregator) Register(name string, check CheckFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.components = append(a.components, component{
		name:     name,
		check:    check,
		required: true,
	})
}

// RegisterOptional adds an optional component health check. If this
// component fails, the overall status will be Degraded (not Unhealthy).
func (a *Aggregator) RegisterOptional(name string, check CheckFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.components = append(a.components, component{
		name:     name,
		check:    check,
		required: false,
	})
}

// Check performs all registered health checks in parallel and returns
// an aggregated report.
func (a *Aggregator) Check(ctx context.Context) *Report {
	a.mu.RLock()
	components := make([]component, len(a.components))
	copy(components, a.components)
	a.mu.RUnlock()

	results := make([]ComponentResult, len(components))
	var wg sync.WaitGroup

	for i, comp := range components {
		wg.Add(1)
		go func(idx int, c component) {
			defer wg.Done()
			results[idx] = a.checkComponent(ctx, c)
		}(i, comp)
	}

	wg.Wait()

	return a.buildReport(components, results)
}

// ComponentCount returns the number of registered components.
func (a *Aggregator) ComponentCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.components)
}

// checkComponent executes a single component health check with a timeout.
func (a *Aggregator) checkComponent(
	ctx context.Context,
	comp component,
) ComponentResult {
	checkCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	start := time.Now()

	type checkResult struct {
		err error
	}

	ch := make(chan checkResult, 1)
	go func() {
		ch <- checkResult{err: comp.check(checkCtx)}
	}()

	select {
	case result := <-ch:
		duration := time.Since(start)
		if result.err != nil {
			return ComponentResult{
				Name:        comp.name,
				Status:      StatusUnhealthy,
				Message:     result.err.Error(),
				Duration:    duration,
				LastChecked: time.Now(),
			}
		}
		return ComponentResult{
			Name:        comp.name,
			Status:      StatusHealthy,
			Duration:    duration,
			LastChecked: time.Now(),
		}

	case <-checkCtx.Done():
		return ComponentResult{
			Name:        comp.name,
			Status:      StatusUnhealthy,
			Message:     fmt.Sprintf("health check timed out after %s", a.timeout),
			Duration:    time.Since(start),
			LastChecked: time.Now(),
		}
	}
}

// buildReport aggregates component results into an overall report.
func (a *Aggregator) buildReport(
	components []component,
	results []ComponentResult,
) *Report {
	overall := StatusHealthy

	for i, result := range results {
		if result.Status == StatusUnhealthy {
			if components[i].required {
				overall = StatusUnhealthy
			} else if overall == StatusHealthy {
				overall = StatusDegraded
			}
		} else if result.Status == StatusDegraded {
			if overall == StatusHealthy {
				overall = StatusDegraded
			}
		}
	}

	return &Report{
		Status:     overall,
		Components: results,
		Timestamp:  time.Now(),
	}
}

// StaticCheck returns a CheckFunc that always returns the given error
// (or nil for healthy). Useful for tests and fixed-status components.
func StaticCheck(err error) CheckFunc {
	return func(_ context.Context) error {
		return err
	}
}

// TCPCheck returns a CheckFunc that performs a REAL net.DialContext
// TCP connectivity probe to the given address.
//
// §11.4 / CONST-035 — Per round-22 audit, the previous implementation
// was a permanent-failure placeholder disguised as a usable export:
// every registered TCPCheck reported the dependency permanently
// unhealthy, silently corrupting health-aggregator signals. The
// "dependency-free" rationale was a §11.4 PASS-bluff — refusing to
// import the only stdlib package that does TCP probes makes the
// function useless. stdlib `net` is not a meaningful "dependency".
//
// Real implementation now: net.Dialer.DialContext respects the
// context deadline and returns the actual TCP error on failure (or
// nil on success after closing the connection).
func TCPCheck(address string) CheckFunc {
	return func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			deadline = time.Now().Add(5 * time.Second)
		}
		timeout := time.Until(deadline)
		if timeout <= 0 {
			return fmt.Errorf("TCPCheck %s: context already expired", address)
		}
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", address)
		if err != nil {
			return fmt.Errorf("TCPCheck %s: %w", address, err)
		}
		_ = conn.Close()
		return nil
	}
}
