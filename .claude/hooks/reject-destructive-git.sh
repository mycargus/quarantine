#!/usr/bin/env bash
# Rejects destructive git commands

command="$1"

if echo "$command" | grep -qE '(git\s+push\s+.*--force|git\s+reset\s+--hard|git\s+branch\s+-D|git\s+clean\s+-f|git\s+checkout\s+\.)'; then
  echo "REJECT: Destructive git command detected. Make sure this is intentional."
  exit 1
fi

exit 0
