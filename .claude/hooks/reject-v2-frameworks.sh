#!/bin/bash
# Rejects edits/writes that add v2+ framework support (pytest, go test, maven)
# per ADR-016: v1 supports only RSpec, Jest, Vitest

content="$1"

if echo "$content" | grep -qiE '(pytest|go\s+test|maven|surefire|junit5|testng|phpunit|nunit|xunit)'; then
  echo "REJECT: v2+ framework reference detected. v1 only supports RSpec, Jest, and Vitest (ADR-016)."
  echo "If this is intentional, get explicit approval before proceeding."
  exit 1
fi

exit 0
