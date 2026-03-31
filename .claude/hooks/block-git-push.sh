#!/usr/bin/env bash
# Blocks any git push during implement-milestone.
# Receives JSON on stdin with tool_input.command.

COMMAND=$(jq -r '.tool_input.command // empty' 2>/dev/null < /dev/stdin)

if echo "$COMMAND" | grep -qE 'git\s+push'; then
  echo "BLOCKED: git push is not allowed during /implement-milestone. Commit locally only." >&2
  exit 2
fi

exit 0
