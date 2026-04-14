#!/usr/bin/env bash
# Rejects writes to test/e2e/ that contain patterns characteristic of
# puppeteered tests: running the CLI binary, creating fake test runners,
# or writing hand-crafted JUnit XML. E2E tests must observe fixture CI
# output, not arrange their own scenarios.

input="$1"

# Only check writes/edits targeting test/e2e/ files
file_path=$(echo "$input" | grep -o '"file_path":"[^"]*"' | head -1 | sed 's/"file_path":"//;s/"//')
if [ -z "$file_path" ]; then
  file_path=$(echo "$input" | grep -o '"command":"[^"]*"' | head -1)
fi

case "$file_path" in
  *test/e2e/*.test.js*) ;;
  *) exit 0 ;;
esac

content=$(echo "$input" | grep -o '"content":"[^"]*"' | head -1 || echo "$input" | grep -o '"new_string":"[^"]*"' | head -1)

if echo "$content" | grep -qE 'spawnSync\(binPath|execSync.*quarantine|makeScript\(|fake-jest|"#!/bin/sh'; then
  echo "REJECT: E2E tests must observe fixture CI output — they must never run the CLI binary or create fake test runners."
  echo "If this test needs controlled inputs, it belongs in the Interface layer (cli/ with mock HTTP servers)."
  echo "See: test/e2e/README.md and docs/specs/test-strategy.md (E2E section)"
  exit 1
fi

exit 0
