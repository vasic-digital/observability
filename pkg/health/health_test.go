package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus_Constants(t *testing.T) {
	assert.Equal(t, Status("healthy"), StatusHealthy)
	assert.Equal(t, Status("degraded"), StatusDegraded)
	assert.Equal(t, Status("unhealthy"), StatusUnhealthy)
}

func TestDefaultAggregatorConfig(t *testing.T) {
	cfg := DefaultAggregatorConfig()
	assert.Equal(t, 5*time.Second, cfg.Timeout)
}

func TestNewAggregator_CustomConfig(t *testing.T) {
	agg := NewAggregator(&AggregatorConfig{Timeout: 10 * time.Second})
	assert.Equal(t, 10*time.Second, agg.timeout)
}

func TestAggregator_Register(t *testing.T) {
	agg := NewAggregator(nil)
	assert.Equal(t, 0, agg.ComponentCount())

	agg.Register("db", StaticCheck(nil))
	assert.Equal(t, 1, agg.ComponentCount())

	agg.RegisterOptional("cache", StaticCheck(nil))
	assert.Equal(t, 2, agg.ComponentCount())
}

func TestAggregator_Check_AllHealthy(t *testing.T) {
	agg := NewAggregator(nil)
	agg.Register("db", StaticCheck(nil))
	agg.Register("redis", StaticCheck(nil))
	agg.RegisterOptional("cache", StaticCheck(nil))

	report := agg.Check(context.Background())

	assert.Equal(t, StatusHealthy, report.Status)
	assert.Len(t, report.Components, 3)
	for _, comp := range report.Components {
		assert.Equal(t, StatusHealthy, comp.Status)
	}
	assert.False(t, report.Timestamp.IsZero())
}

func TestAggregator_Check_RequiredUnhealthy(t *testing.T) {
	agg := NewAggregator(nil)
	agg.Register("db", StaticCheck(errors.New("connection refused")))
	agg.Register("redis", StaticCheck(nil))

	report := agg.Check(context.Background())

	assert.Equal(t, StatusUnhealthy, report.Status)

	var dbResult *ComponentResult
	for i := range report.Components {
		if report.Components[i].Name == "db" {
			dbResult = &report.Components[i]
		}
	}
	require.NotNil(t, dbResult)
	assert.Equal(t, StatusUnhealthy, dbResult.Status)
	assert.Equal(t, "connection refused", dbResult.Message)
}

func TestAggregator_Check_OptionalUnhealthy(t *testing.T) {
	agg := NewAggregator(nil)
	agg.Register("db", StaticCheck(nil))
	agg.RegisterOptional("cache", StaticCheck(errors.New("cache down")))

	report := agg.Check(context.Background())

	assert.Equal(t, StatusDegraded, report.Status)
}

func TestAggregator_Check_RequiredAndOptionalUnhealthy(t *testing.T) {
	agg := NewAggregator(nil)
	agg.Register("db", StaticCheck(errors.New("db down")))
	agg.RegisterOptional("cache", StaticCheck(errors.New("cache down")))

	report := agg.Check(context.Background())

	// Required failure takes precedence
	assert.Equal(t, StatusUnhealthy, report.Status)
}

func TestAggregator_Check_Timeout(t *testing.T) {
	agg := NewAggregator(&AggregatorConfig{Timeout: 50 * time.Millisecond})
	agg.Register("slow", func(ctx context.Context) error {
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	report := agg.Check(context.Background())

	assert.Equal(t, StatusUnhealthy, report.Status)
	require.Len(t, report.Components, 1)
	assert.Contains(t, report.Components[0].Message, "timed out")
}

func TestAggregator_Check_EmptyComponents(t *testing.T) {
	agg := NewAggregator(nil)
	report := agg.Check(context.Background())

	assert.Equal(t, StatusHealthy, report.Status)
	assert.Empty(t, report.Components)
}

func TestAggregator_Check_Duration(t *testing.T) {
	agg := NewAggregator(nil)
	agg.Register("fast", func(_ context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	report := agg.Check(context.Background())
	require.Len(t, report.Components, 1)
	assert.Greater(t, report.Components[0].Duration, time.Duration(0))
}

func TestAggregator_Check_LastChecked(t *testing.T) {
	agg := NewAggregator(nil)
	agg.Register("comp", StaticCheck(nil))

	before := time.Now()
	report := agg.Check(context.Background())
	after := time.Now()

	require.Len(t, report.Components, 1)
	assert.True(t,
		report.Components[0].LastChecked.After(before) ||
			report.Components[0].LastChecked.Equal(before),
	)
	assert.True(t,
		report.Components[0].LastChecked.Before(after) ||
			report.Components[0].LastChecked.Equal(after),
	)
}

func TestAggregator_Check_Parallel(t *testing.T) {
	agg := NewAggregator(nil)

	// Add multiple checks that each take 50ms
	for i := 0; i < 5; i++ {
		agg.Register("comp", func(_ context.Context) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})
	}

	start := time.Now()
	report := agg.Check(context.Background())
	duration := time.Since(start)

	// Should complete in ~50ms, not ~250ms (parallel execution)
	assert.Equal(t, StatusHealthy, report.Status)
	assert.Less(t, duration, 200*time.Millisecond)
}

func TestAggregator_ConcurrentAccess(t *testing.T) {
	// bluff-scan: no-assert-ok (concurrency test — go test -race catches data races; absence of panic == correctness)
	agg := NewAggregator(nil)

	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			agg.Register("comp", StaticCheck(nil))
			agg.Check(context.Background())
		}()
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestStaticCheck(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectErr bool
	}{
		{name: "healthy", err: nil, expectErr: false},
		{
			name:      "unhealthy",
			err:       errors.New("down"),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := StaticCheck(tt.err)
			err := check(context.Background())
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAggregator_MixedStatuses(t *testing.T) {
	tests := []struct {
		name           string
		required       []error
		optional       []error
		expectedStatus Status
	}{
		{
			name:           "all healthy",
			required:       []error{nil, nil},
			optional:       []error{nil},
			expectedStatus: StatusHealthy,
		},
		{
			name:           "required unhealthy",
			required:       []error{errors.New("fail"), nil},
			optional:       []error{nil},
			expectedStatus: StatusUnhealthy,
		},
		{
			name:           "optional unhealthy",
			required:       []error{nil},
			optional:       []error{errors.New("fail")},
			expectedStatus: StatusDegraded,
		},
		{
			name:           "both unhealthy",
			required:       []error{errors.New("fail")},
			optional:       []error{errors.New("fail")},
			expectedStatus: StatusUnhealthy,
		},
		{
			name:           "no components",
			required:       nil,
			optional:       nil,
			expectedStatus: StatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := NewAggregator(nil)
			for i, err := range tt.required {
				name := "required"
				if i > 0 {
					name = "required_" + string(rune('a'+i))
				}
				agg.Register(name, StaticCheck(err))
			}
			for i, err := range tt.optional {
				name := "optional"
				if i > 0 {
					name = "optional_" + string(rune('a'+i))
				}
				agg.RegisterOptional(name, StaticCheck(err))
			}

			report := agg.Check(context.Background())
			assert.Equal(t, tt.expectedStatus, report.Status)
		})
	}
}

func TestAggregator_ImplementsChecker(t *testing.T) {
	var _ Checker = &Aggregator{}
}

// TestTCPCheck: per round-22 §11.4 fix (commit pending), TCPCheck
// now performs a REAL net.DialContext probe instead of returning a
// "placeholder" error. The previous test asserted the placeholder
// message — CERTIFYING the bluff per CONST-035 §11.4.4. Tightened
// to assert real-probe semantics:
//   - unreachable address (localhost:* with no listener) → error
//     wrapping the dial failure, error message contains the address
//   - expired context → "context already expired" (unchanged short-circuit)
func TestTCPCheck(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		ctx         func() context.Context
		errorMatch  string
	}{
		{
			name:    "with deadline - real dial fails on unbound port",
			address: "127.0.0.1:1", // port 1 is reserved; refusal expected
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				t.Cleanup(cancel)
				return ctx
			},
			errorMatch: "127.0.0.1:1",
		},
		{
			name:    "without deadline - default 5s timeout still errors on unbound port",
			address: "127.0.0.1:1",
			ctx: func() context.Context {
				return context.Background()
			},
			errorMatch: "127.0.0.1:1",
		},
		{
			name:    "with expired context",
			address: "localhost:8080",
			ctx: func() context.Context {
				ctx, cancel := context.WithDeadline(
					context.Background(),
					time.Now().Add(-1*time.Second),
				)
				t.Cleanup(cancel)
				return ctx
			},
			errorMatch: "context already expired",
		},
		{
			name:    "error includes address",
			address: "127.0.0.1:1",
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				t.Cleanup(cancel)
				return ctx
			},
			errorMatch: "127.0.0.1:1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := TCPCheck(tt.address)
			err := check(tt.ctx())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMatch)
		})
	}
}

func TestBuildReport_DegradedStatus(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*Aggregator)
		expectedStatus Status
		description    string
	}{
		{
			name: "degraded when optional component unhealthy",
			setupFunc: func(agg *Aggregator) {
				agg.Register("db", StaticCheck(nil))
				agg.RegisterOptional("cache", StaticCheck(errors.New("cache unavailable")))
			},
			expectedStatus: StatusDegraded,
			description:    "system should be degraded when optional fails",
		},
		{
			name: "multiple optional unhealthy - still degraded",
			setupFunc: func(agg *Aggregator) {
				agg.Register("db", StaticCheck(nil))
				agg.RegisterOptional("cache", StaticCheck(errors.New("cache down")))
				agg.RegisterOptional("search", StaticCheck(errors.New("search down")))
			},
			expectedStatus: StatusDegraded,
			description:    "multiple optional failures should be degraded",
		},
		{
			name: "required healthy optional unhealthy mixed",
			setupFunc: func(agg *Aggregator) {
				agg.Register("db", StaticCheck(nil))
				agg.Register("redis", StaticCheck(nil))
				agg.RegisterOptional("metrics", StaticCheck(errors.New("metrics down")))
				agg.RegisterOptional("logging", StaticCheck(nil))
			},
			expectedStatus: StatusDegraded,
			description:    "mix of healthy optional and unhealthy optional",
		},
		{
			name: "unhealthy takes precedence over degraded",
			setupFunc: func(agg *Aggregator) {
				// First add optional that would make it degraded
				agg.RegisterOptional("cache", StaticCheck(errors.New("cache down")))
				// Then add required that makes it unhealthy
				agg.Register("db", StaticCheck(errors.New("db down")))
			},
			expectedStatus: StatusUnhealthy,
			description:    "required failure overrides degraded from optional",
		},
		{
			name: "order independent - required unhealthy last",
			setupFunc: func(agg *Aggregator) {
				agg.Register("db", StaticCheck(nil))
				agg.RegisterOptional("cache", StaticCheck(errors.New("cache down")))
				agg.Register("auth", StaticCheck(errors.New("auth down")))
			},
			expectedStatus: StatusUnhealthy,
			description:    "required failure even when added last",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := NewAggregator(nil)
			tt.setupFunc(agg)

			report := agg.Check(context.Background())
			assert.Equal(t, tt.expectedStatus, report.Status, tt.description)
		})
	}
}

func TestBuildReport_ComponentDegradedStatus(t *testing.T) {
	// Test when a component itself returns StatusDegraded
	// This tests the else if branch in buildReport for StatusDegraded
	agg := NewAggregator(nil)

	// Create a custom check that simulates a component returning degraded status
	// by using a check that returns an error, then manually manipulating for testing
	// Since we can't directly return StatusDegraded from CheckFunc, we need to test
	// the buildReport logic directly

	// First test: verify that when components slice has a degraded result,
	// the overall status becomes degraded
	components := []component{
		{name: "healthy-comp", check: StaticCheck(nil), required: true},
	}
	results := []ComponentResult{
		{
			Name:        "healthy-comp",
			Status:      StatusDegraded,
			Message:     "partially operational",
			Duration:    10 * time.Millisecond,
			LastChecked: time.Now(),
		},
	}

	report := agg.buildReport(components, results)
	assert.Equal(t, StatusDegraded, report.Status,
		"component with degraded status should make overall status degraded")
}

func TestBuildReport_DegradedDoesNotOverrideUnhealthy(t *testing.T) {
	agg := NewAggregator(nil)

	// Test that once status is unhealthy, degraded doesn't change it back
	components := []component{
		{name: "required-failed", check: StaticCheck(nil), required: true},
		{name: "degraded-comp", check: StaticCheck(nil), required: true},
	}
	results := []ComponentResult{
		{
			Name:        "required-failed",
			Status:      StatusUnhealthy,
			Message:     "connection refused",
			Duration:    10 * time.Millisecond,
			LastChecked: time.Now(),
		},
		{
			Name:        "degraded-comp",
			Status:      StatusDegraded,
			Message:     "partial failure",
			Duration:    10 * time.Millisecond,
			LastChecked: time.Now(),
		},
	}

	report := agg.buildReport(components, results)
	assert.Equal(t, StatusUnhealthy, report.Status,
		"unhealthy status should not be overridden by degraded")
}

func TestBuildReport_MultipleDegradedComponents(t *testing.T) {
	agg := NewAggregator(nil)

	// Test multiple components returning degraded status
	components := []component{
		{name: "comp1", check: StaticCheck(nil), required: true},
		{name: "comp2", check: StaticCheck(nil), required: true},
		{name: "comp3", check: StaticCheck(nil), required: false},
	}
	results := []ComponentResult{
		{
			Name:        "comp1",
			Status:      StatusHealthy,
			Duration:    10 * time.Millisecond,
			LastChecked: time.Now(),
		},
		{
			Name:        "comp2",
			Status:      StatusDegraded,
			Message:     "slow response",
			Duration:    10 * time.Millisecond,
			LastChecked: time.Now(),
		},
		{
			Name:        "comp3",
			Status:      StatusDegraded,
			Message:     "cache miss rate high",
			Duration:    10 * time.Millisecond,
			LastChecked: time.Now(),
		},
	}

	report := agg.buildReport(components, results)
	assert.Equal(t, StatusDegraded, report.Status,
		"multiple degraded components should result in degraded status")
	assert.Len(t, report.Components, 3)
}
