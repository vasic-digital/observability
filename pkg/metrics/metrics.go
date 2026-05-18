// Package metrics provides Prometheus-based metrics collection with a generic
// collector interface and a ready-to-use Prometheus implementation.
package metrics

import (
	"fmt"
	"sync"
	"time"

	"digital.vasic.observability/pkg/i18n"

	"github.com/prometheus/client_golang/prometheus"
)

// Collector defines the interface for recording application metrics.
type Collector interface {
	// IncrementCounter increments a named counter by 1, with optional labels.
	IncrementCounter(name string, labels map[string]string)
	// AddCounter adds a value to a named counter.
	AddCounter(name string, value float64, labels map[string]string)
	// RecordLatency records a duration observation in a named histogram.
	RecordLatency(name string, duration time.Duration, labels map[string]string)
	// RecordValue records a float64 observation in a named histogram.
	RecordValue(name string, value float64, labels map[string]string)
	// SetGauge sets the current value of a named gauge.
	SetGauge(name string, value float64, labels map[string]string)
}

// PrometheusConfig configures the Prometheus collector.
type PrometheusConfig struct {
	// Namespace is prepended to all metric names (e.g., "myapp").
	Namespace string
	// Subsystem is inserted between namespace and metric name.
	Subsystem string
	// DefaultBuckets defines histogram bucket boundaries.
	// If nil, prometheus.DefBuckets is used.
	DefaultBuckets []float64
	// Registry is the Prometheus registry to use.
	// If nil, prometheus.DefaultRegisterer is used.
	Registry prometheus.Registerer
}

// DefaultPrometheusConfig returns a PrometheusConfig with sensible defaults.
func DefaultPrometheusConfig() *PrometheusConfig {
	return &PrometheusConfig{
		DefaultBuckets: prometheus.DefBuckets,
	}
}

// PrometheusCollector implements Collector using Prometheus metrics.
type PrometheusCollector struct {
	config     *PrometheusConfig
	mu         sync.RWMutex
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
	gauges     map[string]*prometheus.GaugeVec
	registerer prometheus.Registerer

	// translator is the i18n seam for auto-created metric help text.
	// Defaults to NoopTranslator (key-verbatim passthrough) so legacy
	// English fallback is preserved unless an embedder explicitly wires
	// a project-side translator via SetTranslator. Per CONST-046 +
	// CONST-051(B): submodule MUST NOT reach into any parent project.
	translator i18n.Translator

	// testHookBeforeLock is called between RUnlock and Lock in getOrCreate* methods.
	// This is only for testing the double-check locking pattern. It is nil in production.
	testHookBeforeLock func(name string)
}

// NewPrometheusCollector creates a new Prometheus-backed metrics collector.
func NewPrometheusCollector(config *PrometheusConfig) *PrometheusCollector {
	if config == nil {
		config = DefaultPrometheusConfig()
	}

	registerer := config.Registry
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	return &PrometheusCollector{
		config:     config,
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
		registerer: registerer,
		translator: i18n.NoopTranslator{},
	}
}

// SetTranslator wires the i18n translator used to render auto-created
// metric help-text strings. Passing nil is a no-op (the collector
// continues to use the NoopTranslator default). Per CONST-046 the
// consuming project injects its locale-aware translator; per
// CONST-051(B) the Observability submodule does not reach into the
// parent for a default.
func (c *PrometheusCollector) SetTranslator(t i18n.Translator) {
	if t == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.translator = t
}

// tr returns the active translator, defaulting to NoopTranslator. The
// double-checked-locking pattern in getOrCreate* methods relies on
// this method being safe under read-lock.
func (c *PrometheusCollector) tr() i18n.Translator {
	if c.translator == nil {
		return i18n.NoopTranslator{}
	}
	return c.translator
}

// renderHelp is the centralised i18n seam for auto-created metric
// help-text strings. It is invoked from getOrCreateCounter /
// getOrCreateHistogram / getOrCreateGauge. NoopTranslator returns
// the key verbatim; the fallback parameter preserves the legacy
// English shape ("Auto-created counter: name") for embedders that
// have not wired a translator. Per CONST-046 user-facing strings
// MUST go through this seam; per CONST-035 the legacy fallback
// keeps prior runtime assertions valid.
func (c *PrometheusCollector) renderHelp(key string, params map[string]any, fallback string) string {
	out := c.tr().T(key, params)
	if out == key {
		return fallback
	}
	return out
}

// RegisterCounter pre-registers a counter with known label names.
// This is optional; counters are auto-created on first use.
func (c *PrometheusCollector) RegisterCounter(
	name, help string,
	labelNames []string,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.counters[name]; exists {
		return nil
	}

	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: c.config.Namespace,
		Subsystem: c.config.Subsystem,
		Name:      name,
		Help:      help,
	}, labelNames)

	if err := c.registerer.Register(cv); err != nil {
		return fmt.Errorf("failed to register counter %s: %w", name, err)
	}

	c.counters[name] = cv
	return nil
}

// RegisterHistogram pre-registers a histogram with known label names.
func (c *PrometheusCollector) RegisterHistogram(
	name, help string,
	labelNames []string,
	buckets []float64,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.histograms[name]; exists {
		return nil
	}

	if len(buckets) == 0 {
		buckets = c.config.DefaultBuckets
	}

	hv := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: c.config.Namespace,
		Subsystem: c.config.Subsystem,
		Name:      name,
		Help:      help,
		Buckets:   buckets,
	}, labelNames)

	if err := c.registerer.Register(hv); err != nil {
		return fmt.Errorf("failed to register histogram %s: %w", name, err)
	}

	c.histograms[name] = hv
	return nil
}

// RegisterGauge pre-registers a gauge with known label names.
func (c *PrometheusCollector) RegisterGauge(
	name, help string,
	labelNames []string,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.gauges[name]; exists {
		return nil
	}

	gv := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: c.config.Namespace,
		Subsystem: c.config.Subsystem,
		Name:      name,
		Help:      help,
	}, labelNames)

	if err := c.registerer.Register(gv); err != nil {
		return fmt.Errorf("failed to register gauge %s: %w", name, err)
	}

	c.gauges[name] = gv
	return nil
}

// IncrementCounter increments a named counter by 1.
func (c *PrometheusCollector) IncrementCounter(
	name string,
	labels map[string]string,
) {
	c.AddCounter(name, 1, labels)
}

// AddCounter adds a value to a named counter.
func (c *PrometheusCollector) AddCounter(
	name string,
	value float64,
	labels map[string]string,
) {
	cv := c.getOrCreateCounter(name, labels)
	if cv == nil {
		return
	}
	cv.With(toPrometheusLabels(labels)).Add(value)
}

// RecordLatency records a duration in seconds in a named histogram.
func (c *PrometheusCollector) RecordLatency(
	name string,
	duration time.Duration,
	labels map[string]string,
) {
	c.RecordValue(name, duration.Seconds(), labels)
}

// RecordValue records a float64 value in a named histogram.
func (c *PrometheusCollector) RecordValue(
	name string,
	value float64,
	labels map[string]string,
) {
	hv := c.getOrCreateHistogram(name, labels)
	if hv == nil {
		return
	}
	hv.With(toPrometheusLabels(labels)).Observe(value)
}

// SetGauge sets the current value of a named gauge.
func (c *PrometheusCollector) SetGauge(
	name string,
	value float64,
	labels map[string]string,
) {
	gv := c.getOrCreateGauge(name, labels)
	if gv == nil {
		return
	}
	gv.With(toPrometheusLabels(labels)).Set(value)
}

// getOrCreateCounter returns an existing counter or creates a new one.
func (c *PrometheusCollector) getOrCreateCounter(
	name string,
	labels map[string]string,
) *prometheus.CounterVec {
	c.mu.RLock()
	cv, exists := c.counters[name]
	c.mu.RUnlock()

	if exists {
		return cv
	}

	// Test hook for double-check locking coverage
	if c.testHookBeforeLock != nil {
		c.testHookBeforeLock(name)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if cv, exists = c.counters[name]; exists {
		return cv
	}

	labelNames := labelKeys(labels)
	cv = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: c.config.Namespace,
		Subsystem: c.config.Subsystem,
		Name:      name,
		Help: c.renderHelp(
			"observability_metrics_auto_created_counter",
			map[string]any{"name": name},
			"Auto-created counter: "+name,
		),
	}, labelNames)

	if err := c.registerer.Register(cv); err != nil {
		return nil
	}

	c.counters[name] = cv
	return cv
}

// getOrCreateHistogram returns an existing histogram or creates a new one.
func (c *PrometheusCollector) getOrCreateHistogram(
	name string,
	labels map[string]string,
) *prometheus.HistogramVec {
	c.mu.RLock()
	hv, exists := c.histograms[name]
	c.mu.RUnlock()

	if exists {
		return hv
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if hv, exists = c.histograms[name]; exists {
		return hv
	}

	labelNames := labelKeys(labels)
	hv = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: c.config.Namespace,
		Subsystem: c.config.Subsystem,
		Name:      name,
		Help: c.renderHelp(
			"observability_metrics_auto_created_histogram",
			map[string]any{"name": name},
			"Auto-created histogram: "+name,
		),
		Buckets: c.config.DefaultBuckets,
	}, labelNames)

	if err := c.registerer.Register(hv); err != nil {
		return nil
	}

	c.histograms[name] = hv
	return hv
}

// getOrCreateGauge returns an existing gauge or creates a new one.
func (c *PrometheusCollector) getOrCreateGauge(
	name string,
	labels map[string]string,
) *prometheus.GaugeVec {
	c.mu.RLock()
	gv, exists := c.gauges[name]
	c.mu.RUnlock()

	if exists {
		return gv
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if gv, exists = c.gauges[name]; exists {
		return gv
	}

	labelNames := labelKeys(labels)
	gv = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: c.config.Namespace,
		Subsystem: c.config.Subsystem,
		Name:      name,
		Help: c.renderHelp(
			"observability_metrics_auto_created_gauge",
			map[string]any{"name": name},
			"Auto-created gauge: "+name,
		),
	}, labelNames)

	if err := c.registerer.Register(gv); err != nil {
		return nil
	}

	c.gauges[name] = gv
	return gv
}

// NoOpCollector is a Collector that discards all metrics. Useful for tests
// and environments where metrics collection is disabled.
type NoOpCollector struct {
	// discarded counts discarded metric calls (for debugging)
	discarded int
}

// IncrementCounter is a no-op.
func (n *NoOpCollector) IncrementCounter(_ string, _ map[string]string) {
	n.discarded++
}

// AddCounter is a no-op.
func (n *NoOpCollector) AddCounter(_ string, _ float64, _ map[string]string) {
	n.discarded++
}

// RecordLatency is a no-op.
func (n *NoOpCollector) RecordLatency(
	_ string,
	_ time.Duration,
	_ map[string]string,
) {
	n.discarded++
}

// RecordValue is a no-op.
func (n *NoOpCollector) RecordValue(_ string, _ float64, _ map[string]string) {
	n.discarded++
}

// SetGauge is a no-op.
func (n *NoOpCollector) SetGauge(_ string, _ float64, _ map[string]string) {
	n.discarded++
}

// labelKeys extracts sorted keys from a labels map.
func labelKeys(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	return keys
}

// toPrometheusLabels converts a map to prometheus.Labels.
func toPrometheusLabels(labels map[string]string) prometheus.Labels {
	if labels == nil {
		return prometheus.Labels{}
	}
	return prometheus.Labels(labels)
}
