# ADR-026: v2 CLI Auth via External Token Injection

**Status:** Accepted
**Date:** 2026-04-05

## Context

v2 adds GitHub App support. The CLI needs to authenticate with App-generated tokens in CI. Two approaches exist:

1. **CLI-native App auth:** Build JWT generation (RS256), `TokenProvider` interface, `InstallationTokenProvider` with caching/refresh directly in the Go CLI. Requires 3 new env vars (`QUARANTINE_APP_CLIENT_ID`, `QUARANTINE_APP_PRIVATE_KEY[_PATH]`, `QUARANTINE_APP_INSTALLATION_ID`).
2. **External token injection:** Use `actions/create-github-app-token` (GitHub's official Action) to generate a short-lived token before invoking the CLI. Pass the token as `QUARANTINE_GITHUB_TOKEN`. Zero CLI code changes.

v2 scope is GitHub Actions only (ADR-013). Every GitHub App in the Actions ecosystem uses approach 2. The CLI already accepts any Bearer token via `QUARANTINE_GITHUB_TOKEN`.

## Decision

Use external token injection via `actions/create-github-app-token` for v2. The CLI requires no code changes. Platform engineers configure 2 CI secrets (App ID + private key) and add the action step to their workflow. The action handles JWT generation, installation ID lookup, and token exchange.

CLI-native App auth is deferred to the milestone that adds non-GitHub-Actions CI support (Jenkins, GitLab, etc.), where `actions/create-github-app-token` is not available. See `docs/plans/cli-native-app-auth.md`.

## Alternatives Considered

- **CLI-native App auth in v2:** Adds significant Go code (JWT generation, token provider interface, caching, refresh logic, 3 new env vars) for a use case fully served by a 3-line workflow step. Over-engineering for v2.
- **Both simultaneously:** YAGNI. The external injection path works for 100% of v2 users (GitHub Actions only).

## Consequences

- (+) Zero CLI code changes for v2 App auth.
- (+) Follows the standard GitHub App + Actions pattern that every other tool uses.
- (+) Simpler CI configuration: 2 secrets (App ID + private key) instead of 3+ env vars.
- (+) CLI mental model unchanged -- a token is a token regardless of source.
- (-) No CLI-native token refresh. Token expires after 1 hour. Acceptable: CI runs are typically minutes. If a token expires mid-run, the CLI degrades gracefully (existing behavior).
- (-) Only works in GitHub Actions. Non-Actions CI environments require CLI-native auth (deferred).
- (-) CLI observability cannot distinguish App tokens from PATs in logs.
