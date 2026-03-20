#!/usr/bin/env bash
# Rejects writes that contain GitHub token patterns
# Tokens must come from env vars only, never in config files

content="$1"

if echo "$content" | grep -qE '(ghp_[a-zA-Z0-9]{36}|gho_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}|ghs_[a-zA-Z0-9]{36})'; then
  echo "REJECT: GitHub token pattern detected in file content."
  echo "Tokens must come from environment variables (QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN), never in config files."
  exit 1
fi

exit 0
