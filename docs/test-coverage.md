# Observability — Test-Coverage Ledger (round-252)

> **CONST-050(B) symbol → test ledger.** Lists every exported symbol of every
> production package under `pkg/`, the test files that exercise it, and the
> round-252 anti-bluff invariant the test certifies. A row that lists a symbol
> without a test is a CONST-050(B) gap; a row that lists a test without a
> runtime assertion on user-visible state is a §11.9 / CONST-035 bluff.

> Verbatim 2026-05-19 operator mandate (Article XI §11.9): *"all existing
> tests and Challenges do work in anti-bluff manner - they MUST confirm that
> all tested codebase really works as expected! We had been in position that
> all tests do execute with success and all Challenges as well, but in
> reality the most of the features does not work and can't be used! This
> MUST NOT be the case and execution of tests and Challenges MUST guarantee
> the quality, the completition and full usability by end users of the
> product!"*

Round: **round-252** (mirror round-220 + 242..249 cadence — deep-doc +
Challenge enrichment with bilingual fixture + paired-mutation).

## Scope

Seven production packages exercised by this ledger:

| Package | Path | Role |
|---|---|---|
| `trace` | `pkg/trace/` | OpenTelemetry tracer with OTLP / Jaeger / Zipkin / stdout / NoOp exporter factories |
| `metrics` | `pkg/metrics/` | Prometheus Collector with auto-registered counters / histograms / gauges + NoOp |
| `logging` | `pkg/logging/` | logrus-backed Logger with correlation ID context propagation + NoOp |
| `health` | `pkg/health/` | parallel multi-component Aggregator with required + optional semantics |
| `analytics` | `pkg/analytics/` | ClickHouse Collector with graceful NoOp fallback |
| `i18n` | `pkg/i18n/` | Translator interface + NoopTranslator |
| `gin` + `middleware` | `pkg/gin/`, `pkg/middleware/` | Web-handler glue: HTTP metrics + health endpoint |

## pkg/trace

| Symbol | Test file | Anti-bluff invariant certified |
|---|---|---|
| `ExporterType` | `tracer_test.go` | Enum values resolve to real exporter factories — no silent fallback. |
| `TracerConfig` | `tracer_test.go` | `DefaultConfig` returns non-zero `ServiceName`, sample rate in [0, 1]. |
| `DefaultConfig` | `tracer_test.go` | Round-trip parse + InitTracer succeeds. |
| `Tracer` | `tracer_test.go` | Holds non-nil OTel TracerProvider after `InitTracer`. |
| `ExporterFactory` | `tracer_test.go` | Test factory injection swaps exporter without touching real network. |
| `ResourceFactory` | `tracer_test.go` | Test resource factory honours `ServiceName` + `ServiceVersion` attrs. |
| `SetTestExporterFactory` | `tracer_test.go` | Override is honoured; original factory restored after defer. |
| `SetTestResourceFactory` | `tracer_test.go` | Override is honoured. |
| `InitTracer` | `tracer_test.go` + round-252 runner | Returns non-nil Tracer for every ExporterType; Shutdown returns nil on cold tracer. |
| `Shutdown` | `tracer_test.go` + round-252 runner | Idempotent; second call does not panic. |
| `StartSpan` | `tracer_test.go` + round-252 runner | Returns non-nil span; SpanContext is valid. |
| `StartClientSpan` | `tracer_test.go` | SpanKindClient stamped on resulting span. |
| `StartInternalSpan` | `tracer_test.go` | SpanKindInternal stamped on resulting span. |
| `RecordError` | `tracer_test.go` | Span status set to Error with the supplied message. |
| `SetOK` | `tracer_test.go` | Span status set to Ok. |
| `EndSpanWithError` | `tracer_test.go` | Span ended exactly once; error recorded if non-nil. |
| `TraceFunc` | `tracer_test.go` + round-252 runner | Propagates error from func; span ended even when func panics-recovered. |
| `TraceFuncWithResult` | `tracer_test.go` | Generic preserves result type + error. |
| `TimedSpan` | `tracer_test.go` | Returned finish func records duration attribute non-zero. |
| `Provider` | `tracer_test.go` | Returns the underlying `*sdktrace.TracerProvider` exactly once. |
| `BuildResourceForTesting` | `tracer_test.go` | Resource carries ServiceName + ServiceVersion attrs. |

## pkg/metrics

| Symbol | Test file | Anti-bluff invariant certified |
|---|---|---|
| `Collector` | `metrics_test.go` + round-252 runner | Interface satisfied by both PrometheusCollector and NoOpCollector. |
| `PrometheusConfig` | `metrics_test.go` | Namespace / Subsystem honoured in registered metric name. |
| `DefaultPrometheusConfig` | `metrics_test.go` | Returns a non-nil registry. |
| `PrometheusCollector` | `metrics_test.go` + round-252 runner | Holds a non-nil prometheus.Registry. |
| `NewPrometheusCollector` | `metrics_test.go` + round-252 runner | Returns non-nil instance; double-register of same metric is no-op (idempotent). |
| `SetTranslator` | `metrics_coverage_test.go` + `i18n_callsite_test.go` | Replaces tr() output without touching internal map directly. |
| `RegisterCounter` | `metrics_test.go` + round-252 runner | Returned counter increments observed via registry Gather. |
| `RegisterHistogram` | `metrics_test.go` | Returned histogram records sample observed via registry Gather. |
| `RegisterGauge` | `metrics_test.go` | Returned gauge value observable via registry Gather. |
| `IncrementCounter` | `metrics_test.go` + round-252 runner | Auto-creates counter when not pre-registered; non-ASCII label values do not panic. |
| `AddCounter` | `metrics_test.go` | Adds float64 delta accurately. |
| `RecordLatency` | `metrics_test.go` + round-252 runner | Stored in histogram bucket; non-ASCII endpoint label safe. |
| `RecordValue` | `metrics_test.go` | Generic value recording path. |
| `SetGauge` | `metrics_test.go` | Gauge holds last-set value. |
| `NoOpCollector` | `metrics_test.go` + round-252 runner | Implements Collector with zero-side-effect methods. |

## pkg/logging

| Symbol | Test file | Anti-bluff invariant certified |
|---|---|---|
| `Logger` | `logging_test.go` + round-252 runner | Interface satisfied by LogrusAdapter + NoOpLogger. |
| `Level` | `logging_test.go` | Maps cleanly to logrus levels (Debug/Info/Warn/Error). |
| `Config` | `logging_test.go` | Format + Level honoured by adapter. |
| `DefaultConfig` | `logging_test.go` | Returns non-nil config, Info level default. |
| `LogrusAdapter` | `logging_test.go` + round-252 runner | Writes structured JSON with all set fields. |
| `NewLogrusAdapter` | `logging_test.go` + round-252 runner | Returns non-nil adapter. |
| `Info` / `Warn` / `Error` / `Debug` | `logging_test.go` + round-252 runner | Captured output contains message + level marker; non-ASCII message preserved verbatim. |
| `WithField` | `logging_test.go` + round-252 runner | Adds field to subsequent log lines without mutating parent logger. |
| `WithFields` | `logging_test.go` | Adds map of fields. |
| `WithCorrelationID` | `logging_test.go` + round-252 runner | Adds correlation_id field; non-ASCII ID preserved. |
| `WithError` | `logging_test.go` | Adds error field; nil error tolerated. |
| `ContextWithCorrelationID` | `logging_test.go` + round-252 runner | Round-trip via CorrelationIDFromContext returns the stored ID. |
| `CorrelationIDFromContext` | `logging_test.go` + round-252 runner | Returns empty string on absence; non-ASCII ID round-trips byte-for-byte. |
| `WithContext` | `logging_test.go` | Extracts correlation ID + applies to logger when present. |
| `NoOpLogger` | `logging_test.go` + round-252 runner | All methods silently discard input. |

## pkg/health

| Symbol | Test file | Anti-bluff invariant certified |
|---|---|---|
| `Status` | `health_test.go` + round-252 runner | Enum values {healthy, degraded, unhealthy} stable. |
| `CheckFunc` | `health_test.go` + round-252 runner | Called with caller ctx; honours cancellation. |
| `ComponentResult` | `health_test.go` + round-252 runner | Carries Name + Duration + Error fields. |
| `Report` | `health_test.go` + round-252 runner | Status reflects worst-of components; required-fail → unhealthy, optional-fail → degraded. |
| `Checker` | `health_test.go` | Interface satisfied by Aggregator. |
| `Aggregator` | `health_test.go` + round-252 runner | Concurrent component checks complete within configured timeout. |
| `AggregatorConfig` | `health_test.go` | Timeout honoured by Check. |
| `DefaultAggregatorConfig` | `health_test.go` | Returns non-nil config with non-zero timeout. |
| `NewAggregator` | `health_test.go` + round-252 runner | Returns non-nil aggregator; nil config falls back to defaults. |
| `Register` | `health_test.go` + round-252 runner | Required component; failure makes Report status unhealthy. Non-ASCII component name preserved. |
| `RegisterOptional` | `health_test.go` + round-252 runner | Optional component; failure makes Report status degraded (not unhealthy). |
| `Check` | `health_test.go` + round-252 runner | Returns Report with one ComponentResult per registered component; runs in parallel. |
| `ComponentCount` | `health_test.go` | Reflects registered required + optional counts. |
| `StaticCheck` | `health_test.go` + round-252 runner | Returns CheckFunc that yields the supplied error verbatim. |
| `TCPCheck` | `health_test.go` | Real `net.DialContext` probe — round-22 anti-bluff repair (no simulated TCP). |

## pkg/analytics

| Symbol | Test file | Anti-bluff invariant certified |
|---|---|---|
| `RowsInterface` | `analytics_test.go` | Abstracts `*sql.Rows` for ClickHouse driver swap. |
| `Event` | `analytics_test.go` + round-252 runner | Carries Name + Timestamp + Properties + Tags. |
| `AggregatedStats` | `analytics_test.go` | Holds GroupBy + Count + Window. |
| `Collector` | `analytics_test.go` + round-252 runner | Interface satisfied by ClickHouseCollector + NoOpCollector. |
| `ClickHouseConfig` | `analytics_test.go` | Host / Port / Database / Table all honoured by DSN construction. |
| `SQLOpener` | `analytics_test.go` | Abstracts `sql.Open` for tests + production. |
| `DefaultSQLOpener` | `analytics_test.go` | Calls `sql.Open` verbatim. |
| `ClickHouseCollector` | `analytics_test.go` | Holds non-nil *sql.DB + Config. |
| `NewClickHouseCollector` | `analytics_test.go` | Returns non-nil collector. |
| `NewClickHouseCollectorWithOpener` | `analytics_test.go` | Injectable opener for sqlmock-driven unit tests. |
| `Track` | `analytics_test.go` + round-252 runner | Single-event insert path; sqlmock observes INSERT. Non-ASCII properties accepted. |
| `TrackBatch` | `analytics_test.go` | Multi-event insert; transactional. |
| `Query` | `analytics_test.go` | Returns AggregatedStats with correct counts. |
| `ExecuteReadQuery` | `analytics_test.go` | Whitelisted read query path; rejects writes. |
| `Close` | `analytics_test.go` | Idempotent; second call returns nil. |
| `NoOpCollector` | `analytics_test.go` + round-252 runner | All methods silently succeed; safe fallback for missing ClickHouse. |
| `NewCollector` | `analytics_test.go` + round-252 runner | Returns ClickHouse path when reachable, NoOp on connect failure. Graceful degradation. |

## pkg/i18n

| Symbol | Test file | Anti-bluff invariant certified |
|---|---|---|
| `Translator` | `translator_test.go` + round-252 runner | Interface satisfied by NoopTranslator. |
| `NoopTranslator` | `translator_test.go` + round-252 runner | Returns key verbatim — no silent translation. CONST-046 compliance via consumer-provided real translator. |

## pkg/gin

| Symbol | Test file | Anti-bluff invariant certified |
|---|---|---|
| `MetricsMiddleware` | `gin_test.go` | Records HTTP request count + latency per route. |
| `HealthHandler` | `gin_test.go` | Returns JSON body with Status + per-component results; HTTP status reflects aggregator status. |

## pkg/middleware

| Symbol | Test file | Anti-bluff invariant certified |
|---|---|---|
| `MetricsReporter` | `middleware_test.go` | Interface satisfied by metrics.Collector. |
| `Middleware` | `middleware_test.go` | Wraps `http.Handler`; records request count + latency + status. |

## Test-type matrix coverage (CONST-050(B))

| Test type | Path / mechanism | Round-252 status |
|---|---|---|
| Unit | `pkg/**/_test.go` (no integration tag) | PRESENT — covered by every package above |
| Integration | `tests/integration/` (consumer-driven; module is a library) | PRESENT (stub directories ready for consumer-provided real-stack drivers) |
| E2E | `tests/e2e/` | PRESENT (stub directories ready for consumer-provided real-stack drivers) |
| Full automation | `challenges/runner/main.go` (round-252) + `challenges/observability_describe_challenge.sh` | NEW round-252 — bilingual runner + paired-mutation gate |
| Security | `tests/security/` | PRESENT (consumer drives) |
| DDoS | `challenges/scripts/ddos_health_flood_challenge.sh` | PRESENT (round-85 / Phase 7) |
| Scaling | `challenges/scripts/scaling_horizontal_challenge.sh` | PRESENT |
| Chaos | `challenges/scripts/chaos_failure_injection_challenge.sh` | PRESENT |
| Stress | `challenges/scripts/stress_sustained_load_challenge.sh` | PRESENT |
| Performance | `tests/benchmark/` | PRESENT |
| Benchmarking | `tests/benchmark/` | PRESENT |
| UI | `challenges/scripts/ui_terminal_interaction_challenge.sh` | PRESENT |
| UX | `challenges/scripts/ux_end_to_end_flow_challenge.sh` | PRESENT |
| Challenges | `challenges/scripts/observability_*_challenge.sh` + round-252 describe | PRESENT + ENRICHED |
| HelixQA | Consumer drives via `HelixQA` submodule | OUT-OF-SCOPE (library, not service) |

## Anti-bluff guarantees (Article XI §11.9 + CONST-035)

1. **Real production code paths.** The round-252 runner does NOT inject mocks
   into the production packages. It uses each package's public constructor
   (`trace.InitTracer`, `metrics.NewPrometheusCollector`,
   `logging.NewLogrusAdapter`, `health.NewAggregator`,
   `analytics.NewCollector`, `i18n.NoopTranslator{}`) with real arguments —
   identical to what a downstream consumer (e.g. HelixLLM) would do.

2. **Non-ASCII byte preservation.** Every primitive is exercised with the
   bilingual fixture (`tests/fixtures/i18n/payloads.json`) covering en, sr
   (Cyrillic), ja (Japanese), ar (Arabic), zh-CN. The runner asserts the
   bytes survive intact through:
   - `trace.Tracer.StartSpan(ctx, span_name)` → captured span.Name()
   - `logging.LogrusAdapter.WithField(...).WithCorrelationID(id).Info(msg)`
     → captured log buffer
   - `logging.ContextWithCorrelationID` round-trip → equality on returned ID
   - `health.Aggregator.Register(name, check)` → Report component name
   - `analytics.NoOpCollector.Track(ctx, evt)` → evt.Properties values

3. **Paired-mutation gate.** `observability_describe_challenge.sh
   --anti-bluff-mutate` plants a deliberate ledger-vs-source drift
   (`StartSpan` → `StartBogus_MUTATED`), reruns validation, and asserts the
   gate FAILS with exit 99. This proves the gate actually catches drift
   instead of rubber-stamping.

4. **Race detector.** `go test -race ./...` runs as part of CONST-035 +
   round-85 unit-test discipline; concurrent health checks + the runner's
   parallel collector usage will surface data races as test failures.

5. **No grep-only PASS.** Every PASS line in the runner output carries the
   actual asserted bytes (span name preview, correlation ID, log JSON
   field) — not just "metadata present" rubber stamps.

## Where to extend

When adding a new exported symbol to `pkg/<package>/`, the round-252
discipline requires:

1. Add the symbol to the corresponding table above with the test file and
   anti-bluff invariant it certifies.
2. Add a runner section to `challenges/runner/main.go` exercising it with a
   bilingual fixture input.
3. Run `bash challenges/observability_describe_challenge.sh` — must exit 0.
4. Run `bash challenges/observability_describe_challenge.sh
   --anti-bluff-mutate` — must exit 99 (mutation detected).

Drift between the ledger and the source tree is caught automatically by
Section 2 of the describe challenge — any newly-added exported symbol that
is not cross-referenced here will cause the gate to FAIL with the explicit
"ledger missing symbol pkg.Symbol" line.
