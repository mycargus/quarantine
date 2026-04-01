#!/usr/bin/env bash
# Blocks AI-triggered release commands. Releases must be initiated by humans.

input="$1"

if echo "$input" | grep -qE '(make release|scripts/release\.sh|git push origin v[0-9]|gh release create)'; then
  echo "REJECT: Release commands must be run by a human, not AI."
  echo "Use 'make release VERSION=vX.Y.Z' from your terminal."
  exit 1
fi

exit 0
