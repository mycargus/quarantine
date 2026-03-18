#!/bin/bash
# Rejects direct dependencies between CLI and dashboard
# per ADR-011: they communicate only through GitHub Artifacts

file_path="$1"
content="$2"

# CLI importing dashboard code
if echo "$file_path" | grep -q "^cli/" && echo "$content" | grep -qE '(dashboard/|from.*dashboard|import.*dashboard)'; then
  echo "REJECT: CLI must not import dashboard code. CLI and dashboard communicate only through GitHub (ADR-011)."
  exit 1
fi

# Dashboard importing CLI code
if echo "$file_path" | grep -q "^dashboard/" && echo "$content" | grep -qE '(cli/internal|from.*cli/|import.*cli/)'; then
  echo "REJECT: Dashboard must not import CLI code. CLI and dashboard communicate only through GitHub (ADR-011)."
  exit 1
fi

exit 0
