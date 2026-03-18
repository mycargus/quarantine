#!/bin/bash
# Warns before destructive git commands

command="$1"

if echo "$command" | grep -qE '(git\s+push\s+.*--force|git\s+reset\s+--hard|git\s+branch\s+-D|git\s+clean\s+-f|git\s+checkout\s+\.)'; then
  echo "WARNING: Destructive git command detected. Make sure this is intentional."
  # Exit 0 = warning only, does not block
  exit 0
fi

exit 0
