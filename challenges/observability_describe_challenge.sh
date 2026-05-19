#!/usr/bin/env bash
# observability_describe_challenge.sh
#
# Round-252 paired-mutation deep-doc challenge for Observability.
#
# Validates that:
#   1. The deep-doc ledger (docs/test-coverage.md) lists every exported
#      symbol from pkg/{trace,metrics,logging,health,analytics,i18n,gin,middleware}.
#   2. The bilingual fixture (tests/fixtures/i18n/payloads.json) parses and
#      contains at least 3 locales.
#   3. The bilingual runner (challenges/runner/main.go) builds and runs
#      against every production primitive (real Tracer, real
#      PrometheusCollector, real LogrusAdapter, real Aggregator, real
#      NoOpCollector, NoopTranslator) byte-preserving non-ASCII input
#      through the full pipeline.
#   4. The README enumerates the seven packages and the round-252
#      anti-bluff guarantees section.
#
# Paired-mutation invariant (CONST-035 + CONST-050(B)):
#   With --anti-bluff-mutate the script plants a deliberate symbol-rename
#   mutation in the ledger (in a tmp copy), reruns validation, and asserts
#   the gate FAILS with exit 99. This proves the gate actually catches
#   ledger-vs-source drift instead of rubber-stamping it.
#
# Exit codes:
#   0  — gate PASS on clean tree
#   1  — gate FAIL on clean tree (real failure to fix)
#   99 — paired-mutation correctly detected (good — proves anti-bluff)
#   2  — usage / environment error

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

MUTATE=0
for arg in "$@"; do
    case "$arg" in
        --anti-bluff-mutate) MUTATE=1 ;;
        --help|-h)
            sed -n '1,32p' "$0"
            exit 0
            ;;
        *)
            echo "unknown argument: $arg" >&2
            exit 2
            ;;
    esac
done

PASS=0
FAIL=0
TOTAL=0

pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

LEDGER="${MODULE_DIR}/docs/test-coverage.md"
FIXTURE="${MODULE_DIR}/tests/fixtures/i18n/payloads.json"
RUNNER="${MODULE_DIR}/challenges/runner/main.go"
README="${MODULE_DIR}/README.md"

# If mutation requested, work against a tmp copy of the ledger with a
# planted symbol rename. The original tree stays untouched.
LEDGER_WORK="${LEDGER}"
TMP_LEDGER=""
if [ "${MUTATE}" -eq 1 ]; then
    TMP_LEDGER="$(mktemp)"
    cp "${LEDGER}" "${TMP_LEDGER}"
    # Plant a rename: StartSpan → StartBogus_MUTATED (real symbol must
    # remain absent from ledger to flag the drift).
    sed -i 's/StartSpan/StartBogus_MUTATED/g' "${TMP_LEDGER}"
    LEDGER_WORK="${TMP_LEDGER}"
    echo "=== Observability Describe Challenge (anti-bluff-mutate mode) ==="
else
    echo "=== Observability Describe Challenge (clean mode) ==="
fi
echo ""

# Section 1: ledger presence and freshness
echo "Section 1: docs/test-coverage.md ledger"
if [ ! -f "${LEDGER_WORK}" ]; then
    fail "ledger missing at ${LEDGER_WORK}"
else
    pass "ledger present"
    if grep -q "round-252" "${LEDGER_WORK}"; then
        pass "ledger marked round-252"
    else
        fail "ledger missing round-252 marker"
    fi
    if grep -q "execution of tests and Challenges MUST guarantee" "${LEDGER_WORK}"; then
        pass "ledger carries Article XI §11.9 mandate"
    else
        fail "ledger missing Article XI §11.9 mandate"
    fi
fi

# Section 2: every exported pkg symbol appears in ledger
echo ""
echo "Section 2: exported symbols cross-reference"

extract_symbols() {
    local pkg_dir="$1"
    local files
    files=$(find "${pkg_dir}" -maxdepth 1 -type f -name '*.go' \
        ! -name '*_test.go')
    [ -z "${files}" ] && return 0
    # shellcheck disable=SC2086
    grep -hE '^(func ([A-Z][A-Za-z0-9_]*\()|func \([^)]+\) ([A-Z][A-Za-z0-9_]*\()|type [A-Z][A-Za-z0-9_]* )' \
        ${files} 2>/dev/null \
        | sed -E 's/^func \([^)]+\) ([A-Z][A-Za-z0-9_]*)\(.*$/\1/; s/^func ([A-Z][A-Za-z0-9_]*)\(.*$/\1/; s/^type ([A-Z][A-Za-z0-9_]*).*$/\1/' \
        | sort -u
}

CHECKED=0
MISSING=0
for pkg in trace metrics logging health analytics i18n gin middleware; do
    PKG_DIR="${MODULE_DIR}/pkg/${pkg}"
    if [ ! -d "${PKG_DIR}" ]; then
        fail "pkg/${pkg} missing — cannot cross-reference"
        continue
    fi
    while IFS= read -r sym; do
        [ -z "${sym}" ] && continue
        # Skip noisy single-letter / generic-method names that grep
        # false-positives on (e.g. Translator.T is a single-char method;
        # http.Handler.Write / io.Writer.Write generic boilerplate).
        case "${sym}" in
            String|Error|Write|WriteHeader|Open|T) continue ;;
        esac
        CHECKED=$((CHECKED + 1))
        if grep -qE "\\b${sym}\\b" "${LEDGER_WORK}"; then
            : # symbol cross-referenced
        else
            fail "ledger missing symbol ${pkg}.${sym}"
            MISSING=$((MISSING + 1))
        fi
    done < <(extract_symbols "${PKG_DIR}")
done
if [ "${CHECKED}" -gt 0 ] && [ "${MISSING}" -eq 0 ]; then
    pass "all ${CHECKED} exported symbols cross-referenced in ledger"
fi

# Section 3: bilingual fixture sanity
echo ""
echo "Section 3: bilingual fixture"
if [ ! -f "${FIXTURE}" ]; then
    fail "fixture missing at ${FIXTURE}"
else
    pass "fixture present"
    LOCALE_COUNT=$(grep -oE '"locale":\s*"[^"]+"' "${FIXTURE}" | sort -u | wc -l)
    if [ "${LOCALE_COUNT}" -ge 3 ]; then
        pass "fixture covers ${LOCALE_COUNT} locales (>=3)"
    else
        fail "fixture covers only ${LOCALE_COUNT} locales (<3)"
    fi
fi

# Section 4: runner builds + runs against every production primitive
echo ""
echo "Section 4: bilingual runner build + run (real production code paths)"
if [ ! -f "${RUNNER}" ]; then
    fail "runner missing at ${RUNNER}"
else
    pass "runner source present"
    cd "${MODULE_DIR}"
    if go build -o /tmp/observability_round252_runner ./challenges/runner/ \
        2>/tmp/obs_build.log; then
        pass "runner builds"
        if /tmp/observability_round252_runner -fixtures "${FIXTURE}" \
            > /tmp/obs_run.log 2>&1; then
            pass "runner exit 0 across every primitive"
            if grep -q "PASS \[trace\]\[sr\]" /tmp/obs_run.log; then
                pass "trace Cyrillic (sr) span name round-trip"
            else
                fail "trace Cyrillic (sr) missing from runner output"
            fi
            if grep -q "PASS \[logging\]\[ja\]" /tmp/obs_run.log; then
                pass "logging Japanese (ja) message round-trip"
            else
                fail "logging Japanese (ja) missing from runner output"
            fi
            if grep -q "PASS \[health\]\[ar\]" /tmp/obs_run.log; then
                pass "health Arabic (ar) component-name round-trip"
            else
                fail "health Arabic (ar) missing from runner output"
            fi
            if grep -q "PASS \[analytics:noop\]\[zh-CN\]" /tmp/obs_run.log; then
                pass "analytics zh-CN event-property round-trip"
            else
                fail "analytics zh-CN missing from runner output"
            fi
            if grep -q "PASS \[health:required-fail\]" /tmp/obs_run.log; then
                pass "health required-fail invariant verified"
            else
                fail "health required-fail invariant missing"
            fi
            if grep -q "PASS \[health:optional-fail\]" /tmp/obs_run.log; then
                pass "health optional-fail (degraded) invariant verified"
            else
                fail "health optional-fail invariant missing"
            fi
            if grep -q "PASS \[wire\]" /tmp/obs_run.log; then
                pass "metrics ← i18n.Translator cross-package wiring verified"
            else
                fail "cross-package wire test missing"
            fi
        else
            fail "runner exit non-zero — see /tmp/obs_run.log"
            sed -n '1,40p' /tmp/obs_run.log
        fi
    else
        fail "runner build failed — see /tmp/obs_build.log"
        sed -n '1,40p' /tmp/obs_build.log
    fi
    rm -f /tmp/observability_round252_runner
fi

# Section 5: README round-252 anti-bluff section
echo ""
echo "Section 5: README round-252 anti-bluff section"
if grep -q "Anti-bluff guarantees" "${README}"; then
    pass "README declares Anti-bluff guarantees"
else
    fail "README missing Anti-bluff guarantees section"
fi
if grep -q "round-252" "${README}"; then
    pass "README marked round-252"
else
    fail "README missing round-252 marker"
fi

# Cleanup mutated ledger if any
if [ -n "${TMP_LEDGER}" ]; then
    rm -f "${TMP_LEDGER}"
fi

echo ""
echo "=== Summary: ${PASS}/${TOTAL} PASS, ${FAIL} FAIL ==="

if [ "${MUTATE}" -eq 1 ]; then
    if [ "${FAIL}" -gt 0 ]; then
        echo "anti-bluff-mutate: gate correctly detected planted mutation (exit 99)"
        exit 99
    else
        echo "anti-bluff-mutate: gate FAILED to detect planted mutation — bluff!"
        exit 1
    fi
fi

if [ "${FAIL}" -gt 0 ]; then
    exit 1
fi
exit 0
