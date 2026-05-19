# digital.vasic.observability

A generic, reusable Go module for application observability: distributed tracing, metrics, structured logging, health checks, and analytics.

## Installation

```bash
go get digital.vasic.observability
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"

    "digital.vasic.observability/pkg/trace"
    "digital.vasic.observability/pkg/metrics"
    "digital.vasic.observability/pkg/logging"
    "digital.vasic.observability/pkg/health"
)

func main() {
    // Initialize tracing
    tracer, _ := trace.InitTracer(&trace.TracerConfig{
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
        ExporterType:   trace.ExporterStdout,
        SampleRate:     1.0,
    })
    defer tracer.Shutdown(context.Background())

    // Create a span
    ctx, span := tracer.StartSpan(context.Background(), "main.operation")
    defer span.End()

    // Trace a function
    tracer.TraceFunc(ctx, "db.query", func(ctx context.Context) error {
        time.Sleep(10 * time.Millisecond)
        return nil
    })

    // Initialize metrics
    collector := metrics.NewPrometheusCollector(&metrics.PrometheusConfig{
        Namespace: "myapp",
    })
    collector.IncrementCounter("requests_total", map[string]string{
        "method": "GET",
        "status": "200",
    })
    collector.RecordLatency("request_duration", 150*time.Millisecond, map[string]string{
        "endpoint": "/api/v1/users",
    })

    // Initialize logging
    logger := logging.NewLogrusAdapter(&logging.Config{
        Level:       logging.InfoLevel,
        Format:      "json",
        ServiceName: "my-service",
    })
    logger.WithCorrelationID("req-123").Info("Request processed")

    // Initialize health checks
    agg := health.NewAggregator(nil)
    agg.Register("database", health.StaticCheck(nil))
    agg.RegisterOptional("cache", health.StaticCheck(nil))
    report := agg.Check(context.Background())
    fmt.Printf("System health: %s\n", report.Status)
}
```

## Features

- **Distributed tracing** with OpenTelemetry (OTLP, Jaeger, Zipkin, stdout exporters)
- **Prometheus metrics** with auto-created counters, histograms, and gauges
- **Structured logging** with correlation ID propagation via context
- **Health check aggregation** with required/optional component support
- **ClickHouse analytics** with graceful NoOp fallback
- **Thread-safe** concurrent access across all packages
- **Generic interfaces** for easy mocking and testing

## Packages

| Package | Description |
|---------|-------------|
| `pkg/trace` | OpenTelemetry tracing with multiple exporter backends |
| `pkg/metrics` | Prometheus metrics collection with auto-registration |
| `pkg/logging` | Structured logging with correlation IDs |
| `pkg/health` | Health check aggregation for multi-component systems |
| `pkg/analytics` | ClickHouse analytics with NoOp fallback |

## Tracing

```go
// Create tracer with OTLP exporter
tracer, _ := trace.InitTracer(&trace.TracerConfig{
    ServiceName:  "my-service",
    ExporterType: trace.ExporterOTLP,
    Endpoint:     "localhost:4318",
    SampleRate:   0.5,
})
defer tracer.Shutdown(context.Background())

// Simple span
ctx, span := tracer.StartSpan(ctx, "operation", attribute.String("key", "val"))
defer span.End()

// Client span (for outgoing calls)
ctx, span := tracer.StartClientSpan(ctx, "http.request")
defer trace.EndSpanWithError(span, err)

// Timed span with automatic duration recording
ctx, finish := tracer.TimedSpan(ctx, "slow.operation")
defer finish(nil)

// Trace a function
err := tracer.TraceFunc(ctx, "db.query", func(ctx context.Context) error {
    return db.QueryRow(ctx, "SELECT 1")
})

// Trace with result
result, err := trace.TraceFuncWithResult(tracer, ctx, "compute", func(ctx context.Context) (int, error) {
    return 42, nil
})
```

## Metrics

```go
// With custom Prometheus registry
reg := prometheus.NewRegistry()
collector := metrics.NewPrometheusCollector(&metrics.PrometheusConfig{
    Namespace: "myapp",
    Subsystem: "api",
    Registry:  reg,
})

// Pre-register metrics (optional, auto-created on use)
collector.RegisterCounter("requests_total", "Total requests", []string{"method"})
collector.RegisterHistogram("duration_seconds", "Duration", []string{"endpoint"}, nil)
collector.RegisterGauge("connections", "Active connections", []string{"pool"})

// Record metrics
collector.IncrementCounter("requests_total", map[string]string{"method": "POST"})
collector.RecordLatency("duration_seconds", 100*time.Millisecond, map[string]string{"endpoint": "/api"})
collector.SetGauge("connections", 42, map[string]string{"pool": "main"})

// No-op collector for tests
var c metrics.Collector = &metrics.NoOpCollector{}
```

## Logging

```go
// Create logger
logger := logging.NewLogrusAdapter(&logging.Config{
    Level:       logging.DebugLevel,
    Format:      "json",
    ServiceName: "my-service",
})

// Structured logging
logger.WithField("user_id", "123").Info("User logged in")
logger.WithFields(map[string]interface{}{
    "request_id": "abc",
    "latency_ms": 150,
}).Debug("Request completed")
logger.WithError(err).Error("Operation failed")

// Correlation ID propagation
ctx := logging.ContextWithCorrelationID(ctx, "corr-123")
enriched := logging.WithContext(logger, ctx)
enriched.Info("Request with correlation ID")

// Extract correlation ID
id := logging.CorrelationIDFromContext(ctx)
```

## Health Checks

```go
agg := health.NewAggregator(&health.AggregatorConfig{
    Timeout: 3 * time.Second,
})

// Required: failure = unhealthy
agg.Register("database", func(ctx context.Context) error {
    return db.PingContext(ctx)
})

// Optional: failure = degraded
agg.RegisterOptional("cache", func(ctx context.Context) error {
    return redis.Ping(ctx).Err()
})

// Check all components (runs in parallel)
report := agg.Check(ctx)
// report.Status: "healthy", "degraded", or "unhealthy"
// report.Components: individual results with durations
```

## Analytics

```go
// Auto-fallback to NoOp if ClickHouse unavailable
collector := analytics.NewCollector(&analytics.ClickHouseConfig{
    Host:     "localhost",
    Port:     9000,
    Database: "analytics",
    Table:    "events",
}, logger)
defer collector.Close()

// Track events
collector.Track(ctx, analytics.Event{
    Name:      "request.completed",
    Timestamp: time.Now(),
    Properties: map[string]interface{}{"duration_ms": 250},
    Tags:       map[string]string{"service": "api"},
})

// Batch tracking
collector.TrackBatch(ctx, events)

// Query aggregated stats
stats, _ := collector.Query(ctx, "events", "name", 24*time.Hour)
```

## Anti-bluff guarantees (round-252)

Every PASS reported by Observability's test + Challenge suite MUST certify
that the feature actually works for an end user — not merely that the code
compiles or the function was called. This section enumerates the
load-bearing invariants that distinguish a real PASS from a bluff PASS.
The round-252 deep-doc + Challenge enrichment is the mechanical enforcement.

**Verbatim 2026-05-19 operator mandate (Article XI §11.9):**

> *"all existing tests and Challenges do work in anti-bluff manner - they
> MUST confirm that all tested codebase really works as expected! We had
> been in position that all tests do execute with success and all
> Challenges as well, but in reality the most of the features does not
> work and can't be used! This MUST NOT be the case and execution of
> tests and Challenges MUST guarantee the quality, the completition and
> full usability by end users of the product!"*

### What round-252 ships

- `docs/test-coverage.md` — exhaustive symbol-to-test ledger for every
  exported identifier in `pkg/{trace,metrics,logging,health,analytics,i18n,gin,middleware}`,
  paired with the anti-bluff invariant each test certifies.
- `tests/fixtures/i18n/payloads.json` — bilingual fixture set covering en,
  sr (Cyrillic), ja (Japanese), ar (Arabic), zh-CN. Every primitive is
  exercised with non-ASCII text to guard against silent UTF-8 corruption.
- `challenges/runner/main.go` — Go runner that drives every Observability
  primitive (Tracer, PrometheusCollector, LogrusAdapter, Aggregator,
  analytics.NoOpCollector, NoopTranslator) through real production code
  paths with non-ASCII inputs and asserts byte-for-byte preservation.
- `challenges/observability_describe_challenge.sh` — paired-mutation gate.
  Clean run asserts ledger ↔ source cross-reference + runner exit 0 +
  per-primitive PASS coverage. Mutated run (`--anti-bluff-mutate`) plants
  a deliberate ledger-vs-source drift and asserts the gate FAILS — proving
  it actually catches drift instead of rubber-stamping.

### Acceptance demo

```bash
# Clean run — must exit 0
cd Observability
bash challenges/observability_describe_challenge.sh

# Paired-mutation run — must exit 99 (mutation correctly detected)
bash challenges/observability_describe_challenge.sh --anti-bluff-mutate

# Race-detector run across every package — must PASS
GOMAXPROCS=2 nice -n 19 go test -count=1 -race -p 1 ./...
```

### Invariants certified

1. **Real production code paths.** The runner does NOT inject mocks into
   the production packages. `trace.InitTracer`, `metrics.NewPrometheusCollector`,
   `logging.NewLogrusAdapter`, `health.NewAggregator`, `analytics.NewCollector`,
   and `i18n.NoopTranslator{}` are constructed exactly as a downstream
   consumer (e.g. HelixLLM) would construct them.
2. **Non-ASCII byte preservation.** Span names, log messages, correlation
   IDs, health component names, and analytics event property values
   survive the pipeline byte-for-byte. Specifically:
   - `trace.Tracer.StartSpan(ctx, span_name)` keeps `span_name` verbatim.
   - `logging.WithCorrelationID(...)` round-trips the ID through
     `ContextWithCorrelationID` + `CorrelationIDFromContext`.
   - Captured JSON log buffer MUST contain the original message bytes.
   - `health.Aggregator.Register(name, ...)` preserves `name` in the
     resulting `Report.Components[].Name`.
3. **Health-status invariants.** Required-component failure → `unhealthy`,
   optional-component failure → `degraded`, all-healthy → `healthy`.
   The runner exercises all three transitions.
4. **Metric label safety.** Non-ASCII label values do not panic
   Prometheus's label validation; the recommended path is key
   namespacing (not value sanitization).
5. **Graceful degradation.** `analytics.NewCollector` returns a NoOp
   collector when ClickHouse is unreachable; downstream consumers never
   observe a panic or nil dereference.
6. **Cross-package wiring.** `metrics.PrometheusCollector.SetTranslator`
   accepts an `i18n.Translator` and consumes it from the increment path.
7. **Race-safety.** Parallel health checks complete without data races
   under `-race`.
8. **Paired-mutation gate.** A planted ledger-vs-source drift is detected
   by the describe challenge — the gate is not a rubber stamp.

### How to extend without bluffing

Adding a new exported symbol to `pkg/<package>/`:

1. Add the symbol to the table in `docs/test-coverage.md` with the test
   file + anti-bluff invariant it certifies.
2. Add a runner section to `challenges/runner/main.go` exercising it with
   a bilingual fixture input.
3. Run `bash challenges/observability_describe_challenge.sh` — must
   exit 0.
4. Run `bash challenges/observability_describe_challenge.sh --anti-bluff-mutate`
   — must exit 99.

A new symbol not added to the ledger is detected automatically by
Section 2 of the describe challenge (`ledger missing symbol pkg.Symbol`).
A test that does not certify a user-visible invariant is detected by
review against the round-252 invariants list above.

