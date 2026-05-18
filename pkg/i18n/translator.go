// Package i18n provides the locale-aware message translation seam for the
// Observability submodule. It is the CONST-046-compliant indirection
// between production code in pkg/metrics/* (and future user-facing surfaces
// in pkg/logging, pkg/health, pkg/analytics) and the locale bundles under
// bundles/.
//
// Per CONST-051(B), this package is fully decoupled from any consuming
// project: callers inject a Translator (or rely on the NoopTranslator
// default), and the Observability submodule never reaches into a parent
// project to discover its own catalogue.
//
// Usage:
//
//	t := i18n.NoopTranslator{} // production default — returns key verbatim
//	msg := t.T("observability_metrics_auto_created_counter", map[string]any{
//	    "name": "request_total",
//	})
//
// Consuming projects wire a real translator that loads
// bundles/active.en.yaml + locale overrides; the Observability submodule
// remains project-not-aware.
package i18n

// Translator is the message-resolution seam. Implementations MUST be
// safe for concurrent use.
type Translator interface {
	// T returns the localised string for key in the active locale,
	// substituting params by name. When the key is unknown the
	// implementation SHOULD return the key verbatim so production
	// surfaces stay actionable rather than blank.
	T(key string, params map[string]any) string
}

// NoopTranslator is the zero-dependency production-safe default returned
// by package consumers that have not yet wired a project-side
// translator. It returns the key verbatim, which keeps the legacy
// surface shape ("observability_metrics_auto_created_counter") rather
// than an empty string — actionable for downstream string assertions
// and visible in logs.
type NoopTranslator struct{}

// T satisfies Translator by returning the key unchanged. Params are
// ignored on purpose: the noop implementation has no template engine.
func (NoopTranslator) T(key string, _ map[string]any) string {
	return key
}
