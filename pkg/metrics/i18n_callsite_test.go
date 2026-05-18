// CONST-035 / CONST-046 sentinel coverage for round-121 i18n migration.
// Tests guard against regressions that drop SetTranslator wiring or
// stop piping translator output into PrometheusCollector auto-created
// metric help text. Per CONST-050(A) fakes are permitted in *_test.go.
package metrics

import (
	"sync"
	"testing"

	"digital.vasic.observability/pkg/i18n"

	"github.com/prometheus/client_golang/prometheus"
)

// recordingTranslator captures the i18n keys the production code emits
// and returns a sentinel string so call-sites can be asserted on.
type recordingTranslator struct {
	mu   sync.Mutex
	seen []string
}

func (r *recordingTranslator) T(key string, _ map[string]any) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, key)
	return "translated:" + key
}

func (r *recordingTranslator) keys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.seen))
	copy(out, r.seen)
	return out
}

func newIsolatedCollector(t *testing.T) *PrometheusCollector {
	t.Helper()
	reg := prometheus.NewRegistry()
	cfg := &PrometheusConfig{
		Namespace: "i18nsentinel",
		Subsystem: "metrics",
		Registry:  reg,
	}
	return NewPrometheusCollector(cfg)
}

func TestPrometheusCollector_SetTranslator_OverridesAutoCreatedCounterHelp(t *testing.T) {
	c := newIsolatedCollector(t)
	rt := &recordingTranslator{}
	c.SetTranslator(rt)

	got := c.renderHelp(
		"observability_metrics_auto_created_counter",
		map[string]any{"name": "requests_total"},
		"Auto-created counter: requests_total",
	)
	if got != "translated:observability_metrics_auto_created_counter" {
		t.Fatalf("counter branch: got %q, want translator output", got)
	}
	keys := rt.keys()
	if len(keys) != 1 || keys[0] != "observability_metrics_auto_created_counter" {
		t.Fatalf("recorded keys = %v, want exactly [observability_metrics_auto_created_counter]", keys)
	}
}

func TestPrometheusCollector_SetTranslator_OverridesAutoCreatedHistogramHelp(t *testing.T) {
	c := newIsolatedCollector(t)
	rt := &recordingTranslator{}
	c.SetTranslator(rt)

	got := c.renderHelp(
		"observability_metrics_auto_created_histogram",
		map[string]any{"name": "latency_seconds"},
		"Auto-created histogram: latency_seconds",
	)
	if got != "translated:observability_metrics_auto_created_histogram" {
		t.Fatalf("histogram branch: got %q, want translator output", got)
	}
}

func TestPrometheusCollector_SetTranslator_OverridesAutoCreatedGaugeHelp(t *testing.T) {
	c := newIsolatedCollector(t)
	rt := &recordingTranslator{}
	c.SetTranslator(rt)

	got := c.renderHelp(
		"observability_metrics_auto_created_gauge",
		map[string]any{"name": "queue_depth"},
		"Auto-created gauge: queue_depth",
	)
	if got != "translated:observability_metrics_auto_created_gauge" {
		t.Fatalf("gauge branch: got %q, want translator output", got)
	}
}

func TestPrometheusCollector_NoopTranslator_PreservesEnglishCounter(t *testing.T) {
	// Default (no SetTranslator call) MUST render legacy English so
	// existing Prometheus-help-text assertions keep working — guards
	// against a regression that ships keys-as-prose if the fallback
	// path is removed.
	c := newIsolatedCollector(t)

	got := c.renderHelp(
		"observability_metrics_auto_created_counter",
		map[string]any{"name": "requests_total"},
		"Auto-created counter: requests_total",
	)
	if got != "Auto-created counter: requests_total" {
		t.Fatalf("noop fallback (counter): got %q, want legacy English", got)
	}
}

func TestPrometheusCollector_NoopTranslator_PreservesEnglishHistogram(t *testing.T) {
	c := newIsolatedCollector(t)

	got := c.renderHelp(
		"observability_metrics_auto_created_histogram",
		map[string]any{"name": "latency_seconds"},
		"Auto-created histogram: latency_seconds",
	)
	if got != "Auto-created histogram: latency_seconds" {
		t.Fatalf("noop fallback (histogram): got %q, want legacy English", got)
	}
}

func TestPrometheusCollector_NoopTranslator_PreservesEnglishGauge(t *testing.T) {
	c := newIsolatedCollector(t)

	got := c.renderHelp(
		"observability_metrics_auto_created_gauge",
		map[string]any{"name": "queue_depth"},
		"Auto-created gauge: queue_depth",
	)
	if got != "Auto-created gauge: queue_depth" {
		t.Fatalf("noop fallback (gauge): got %q, want legacy English", got)
	}
}

func TestPrometheusCollector_SetTranslator_NilIsNoop(t *testing.T) {
	c := newIsolatedCollector(t)
	c.SetTranslator(nil) // MUST NOT panic, MUST NOT overwrite the default

	got := c.renderHelp(
		"observability_metrics_auto_created_gauge",
		map[string]any{"name": "queue_depth"},
		"Auto-created gauge: queue_depth",
	)
	if got != "Auto-created gauge: queue_depth" {
		t.Fatalf("after nil SetTranslator: got %q, want default fallback", got)
	}
}

// CONST-051(B) decoupling check: NoopTranslator interface satisfaction
// + Translator type both live in vasic-digital/Observability's own
// pkg/i18n — no parent-tree reach. This compile-time assertion is the
// §11.4 paired-mutation pair for the translator-import regression mode.
var _ i18n.Translator = i18n.NoopTranslator{}
