# ADR-009: PR Comments as v1 Notification Channel

**Status:** Accepted
**Date:** 2026-03-14

## Context

When flaky tests are detected, the team needs to be notified. Options include PR comments, Slack, email, or multiple channels.

## Decision

GitHub PR comments for v1. The CLI posts a comment on the PR (or commit) that triggered the build, summarizing: which tests were detected as flaky, retry results, quarantine status, and links to created GitHub issues. When multiple builds run on the same PR, the CLI updates the existing Quarantine comment rather than posting a new one. This prevents notification noise on active PRs.

Format:

```
**Quarantine** detected 2 flaky tests in this build:
- `test_payment_processing` -- passed on retry 2/3 -> quarantined (PROJ-1234)
- `test_email_send` -- passed on retry 1/3 -> quarantined (PROJ-1235)

Build result: **pass** (2 failures suppressed)
```

v2 adds Slack and email notifications, plus configurable threshold alerts.

## Alternatives Considered

- **Slack:** Requires OAuth app installation, workspace configuration, channel selection. High setup friction. Deferred to v2.
- **Email:** Requires SMTP/SES configuration, email collection, preference management. High setup friction and low engagement. Deferred to v2.
- **GitHub commit status checks:** Useful supplement but not enough detail for notification. Could be added alongside PR comments.

## Consequences

- (+) Zero additional setup -- uses the same GitHub token already configured for CLI.
- (+) Context is exactly where the developer is looking (the PR).
- (+) Rich formatting with links to issues.
- (+) Natural integration with code review workflow.
- (-) Only visible if there is a PR (direct pushes to main will not get comments). Mitigated by also logging to CI output.
- (-) Can get noisy on PRs with many builds. Mitigated by updating existing comment rather than posting new ones.
