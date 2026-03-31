#!/usr/bin/env bash
# Run all contract tests: Go (CLI) + JS (dashboard) against a shared Prism instance.
#
# Usage: ./scripts/run-contract-tests.sh
#
# Requires:
#   - node/pnpm in PATH (test/ deps must be installed)
#   - go in PATH (cli/ deps must be installed)
#   - @stoplight/prism-cli installed in test/node_modules

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── Find a free port ────────────────────────────────────────────────────────
PORT=$(node -e "
  const net = require('net');
  const s = net.createServer();
  s.listen(0, () => { const p = s.address().port; s.close(() => process.stdout.write(String(p))); });
")

if [ -z "$PORT" ]; then
  echo "ERROR: Could not find a free port for Prism" >&2
  exit 1
fi

# ── Start Prism ─────────────────────────────────────────────────────────────
PRISM_LOG=$(mktemp)
"${REPO_ROOT}/test/node_modules/.bin/prism" mock \
  "${REPO_ROOT}/schemas/github-api.json" \
  -p "$PORT" \
  --errors \
  >"$PRISM_LOG" 2>&1 &
PRISM_PID=$!

# Kill Prism and clean up on exit (success or failure)
cleanup() {
  kill "$PRISM_PID" 2>/dev/null || true
  wait "$PRISM_PID" 2>/dev/null || true
  rm -f "$PRISM_LOG"
}
trap cleanup EXIT

# ── Wait for Prism to be ready ───────────────────────────────────────────────
TIMEOUT=10
ELAPSED=0
until grep -q "Prism is listening" "$PRISM_LOG" 2>/dev/null; do
  if [ "$ELAPSED" -ge "$TIMEOUT" ]; then
    echo "ERROR: Prism did not start within ${TIMEOUT}s" >&2
    echo "Prism output:" >&2
    cat "$PRISM_LOG" >&2
    exit 1
  fi
  sleep 1
  ELAPSED=$((ELAPSED + 1))
done

export PRISM_URL="http://127.0.0.1:${PORT}"
echo "[contract] Prism listening on ${PRISM_URL}"
echo "[contract] Note: 'unknown format' warnings from Prism are expected — it skips format constraints (date-time, uri) but still validates types and required fields."

# ── Run Go contract tests ────────────────────────────────────────────────────
echo "[contract] Running Go contract tests..."
cd "${REPO_ROOT}/cli"
# PRISM_URL is read by newPrismClient(t), which calls t.Setenv for QUARANTINE_GITHUB_API_BASE_URL
# and GITHUB_TOKEN per-test. Do NOT set QUARANTINE_GITHUB_API_BASE_URL globally here — it
# would break non-contract tests like TestInitAPIUnreachable that run in the same invocation.
PRISM_URL="${PRISM_URL}" go test -tags contract -count=1 ./...

# ── Run JS contract tests ────────────────────────────────────────────────────
echo "[contract] Running JS contract tests..."
cd "${REPO_ROOT}/test"
PRISM_URL="${PRISM_URL}" pnpm run test:contract

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "[contract] ✓ All contract tests passed."
echo "[contract]   Vendored spec : schemas/github-api.json"
echo "[contract]   Go tests      : cli/internal/github/*_contract_test.go"
echo "[contract]   JS tests      : test/contract/*.test.js"
