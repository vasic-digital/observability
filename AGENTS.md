# AGENTS.md - Multi-Agent Coordination Guide

## INHERITED FROM the Helix Constitution

This module is governed by the Helix Constitution. All rules in the
constitution's `AGENTS.md` and the `Constitution.md` it references apply
unconditionally. Locate the constitution from any nested depth via its
`find_constitution.sh` helper — do NOT hardcode a path (this module stays
fully decoupled and project-agnostic per §11.4.28).

Canonical reference: https://github.com/HelixDevelopment/HelixConstitution

## Overview

This document provides guidance for AI agents working on the `digital.vasic.observability` module. The module is a generic, reusable Go library for application observability with five packages: `trace`, `metrics`, `logging`, `health`, and `analytics`.

## Module Boundaries

- **Module path**: `digital.vasic.observability`
- **Go version**: 1.24+
- **Package root**: `pkg/`
- **No main package** -- this is a library module, not an executable.

## Package Responsibilities

| Package | Owner Concern | Dependencies |
|---------|--------------|--------------|
| `pkg/trace` | Distributed tracing via OpenTelemetry | `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk` |
| `pkg/metrics` | Prometheus metric collection | `github.com/prometheus/client_golang` |
| `pkg/logging` | Structured logging with correlation IDs | `github.com/sirupsen/logrus` |
| `pkg/health` | Health check aggregation | stdlib only (`context`, `sync`, `time`, `fmt`) |
| `pkg/analytics` | ClickHouse event tracking | `github.com/ClickHouse/clickhouse-go/v2`, `github.com/sirupsen/logrus` |

## Inter-Package Rules

1. **No circular imports.** Packages must NOT import each other. Each package is fully independent.
2. **Interface-driven.** Each package defines its own interface (`Collector`, `Logger`, `Checker`, `Collector`). Consumers depend on interfaces, not concrete types.
3. **No shared state.** Packages do not share global variables. The `trace` package sets the global OTel provider but other packages do not read it.

## Agent Coordination Patterns

### When Adding a New Exporter to `trace`

1. Add the new `ExporterType` constant.
2. Add a case in `InitTracer` switch.
3. Add the setup function (follow `setupOTLPExporter` pattern).
4. Add unit tests with table-driven cases.
5. Update `CLAUDE.md` package table and `README.md` feature list.

### When Adding a New Metric Type to `metrics`

1. Add the method to the `Collector` interface.
2. Implement it on `PrometheusCollector`.
3. Add a no-op implementation on `NoOpCollector`.
4. Add `RegisterX` and `getOrCreateX` methods following the existing pattern.
5. Add table-driven tests.

### When Adding a New Log Level or Method to `logging`

1. Add the method to the `Logger` interface.
2. Implement on `LogrusAdapter`.
3. Implement on `NoOpLogger`.
4. Add tests.

### When Adding a New Health Check Pattern to `health`

1. Add the `CheckFunc` factory (follow `StaticCheck`, `TCPCheck` patterns).
2. If adding new status types, add to the `Status` constants.
3. Update `buildReport` aggregation logic if status priority changes.
4. Add tests.

### When Modifying `analytics`

1. All query construction must validate identifiers via `isValidIdentifier` to prevent SQL injection.
2. The `NewCollector` factory must always fall back to `NoOpCollector` on connection failure.
3. Batch operations must use transactions.
4. Add tests (unit tests can use `NoOpCollector`; integration tests require ClickHouse).

## Testing Conventions

- **File naming**: `<package>_test.go` in the same directory as the source.
- **Test naming**: `Test<Struct>_<Method>_<Scenario>` (e.g., `TestAggregator_Check_AllHealthy`).
- **Table-driven tests** with `testify/assert` and `testify/require`.
- **Unit tests**: Run with `go test ./... -short`. Must not require external services.
- **Integration tests**: Use `//go:build integration` tag. Require running ClickHouse, OTLP collector, etc.
- **Benchmarks**: Use `Benchmark<Function>` naming. Place in `tests/benchmark/`.

## Commit and PR Conventions

- Conventional Commits: `<type>(<package>): <description>`
  - `feat(trace): add Zipkin exporter support`
  - `fix(metrics): handle nil labels in SetGauge`
  - `test(health): add timeout scenario tests`
  - `docs(analytics): update Query method documentation`
- One logical change per commit.
- All tests must pass before committing: `go test ./... -count=1 -race`
- Run `go fmt ./...` and `go vet ./...` before every commit.

## Code Quality Gates

Before any PR is merged:

1. `go fmt ./...` -- no formatting changes.
2. `go vet ./...` -- no warnings.
3. `go test ./... -count=1 -race` -- all tests pass, no race conditions.
4. `go test ./... -short` -- unit tests pass independently.
5. All exported types and functions have GoDoc comments.
6. No new dependencies added without justification.

## Concurrency Safety

All public types are safe for concurrent use:

- `Tracer` uses `sync.RWMutex` for provider access.
- `PrometheusCollector` uses `sync.RWMutex` with double-checked locking for lazy metric creation.
- `LogrusAdapter` uses `sync.RWMutex` for entry access.
- `Aggregator` uses `sync.RWMutex` for component list; `Check` runs checks in parallel with `sync.WaitGroup`.
- `ClickHouseCollector` uses `sync.RWMutex` for config access.

Agents must maintain this invariant when making changes.

## File Structure

```
Observability/
  go.mod
  go.sum
  CLAUDE.md
  AGENTS.md
  README.md
  pkg/
    trace/
      tracer.go
      tracer_test.go
    metrics/
      metrics.go
      metrics_test.go
    logging/
      logging.go
      logging_test.go
    health/
      health.go
      health_test.go
    analytics/
      analytics.go
      analytics_test.go
  docs/
    USER_GUIDE.md
    ARCHITECTURE.md
    API_REFERENCE.md
    CONTRIBUTING.md
    CHANGELOG.md
    diagrams/
      architecture.mmd
      sequence.mmd
      class.mmd
```
