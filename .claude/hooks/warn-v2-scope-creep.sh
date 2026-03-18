#!/bin/bash
# Warns when v2+ features are being added (Jira, Slack, email integration)
# These are explicitly deferred per CLAUDE.md scope

content="$1"

if echo "$content" | grep -qiE '(jira|atlassian|slack|webhook_url|smtp|sendgrid|email.*notification)'; then
  echo "WARNING: Possible v2+ scope creep detected (Jira/Slack/email integration)."
  echo "v1 scope: GitHub Issues only, PR comments only. See CLAUDE.md."
  # Exit 0 = warning only, does not block
  exit 0
fi

exit 0
