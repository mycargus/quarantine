# ADR-008: API Key Auth (v1), GitHub App + OAuth (v2)

**Status:** Accepted
**Date:** 2026-03-14

## Context

The system needs authentication for: CLI accessing GitHub API (to read/write quarantine.json, create issues, post PR comments) and dashboard accessing GitHub API (to pull artifacts, check issue status). The CLI does not upload artifacts directly — it writes results to a local file and the GitHub Actions workflow handles upload (ADR-007). v2 adds dashboard web UI authentication.

## Decision

- **v1:** GitHub Personal Access Token (PAT) configured as CI secret. The CLI checks `QUARANTINE_GITHUB_TOKEN` first, then falls back to `GITHUB_TOKEN` if available. Same or separate PAT for dashboard to access GitHub Artifacts API. No CLI-to-dashboard auth needed (CLI does not talk to dashboard). Note: `GITHUB_TOKEN` in GitHub Actions is limited to 1,000 req/hr/repository. PATs get 5,000/hr. Recommend PAT or GitHub App for repos with high CI volume.
- **v2:** GitHub App installed on org. Provides:
  - Short-lived installation tokens (1-hour expiry) that the application must explicitly request via the GitHub API. SDKs (e.g., Octokit) handle token refresh transparently, but it is client-side logic, not a platform feature. No manual PAT management.
  - Fine-grained permissions (contents:write on quarantine/state branch, issues:write, pull_requests:write, actions:read).
  - Higher rate limits (5,000/hr minimum, scaling to 12,500 based on repo count, vs. 1,000/hr for `GITHUB_TOKEN`).
  - Org-wide access (install once, all repos covered).
  - Dashboard web UI uses GitHub OAuth via remix-auth + remix-auth-github for user login.

## Alternatives Considered

- **GitHub App from v1:** More secure and proper, but adds complexity (App registration, private key management, installation token exchange). Deferred to v2 to reduce initial scope.
- **Custom API key system:** Unnecessary in v1 since CLI talks to GitHub, not dashboard. Would be needed if using push model, but we chose pull model (ADR-007).
- **OAuth-only (no PAT support):** Does not work well in CI environments where interactive login is not possible.

## Consequences

- (+) v1 is dead simple -- one env var in CI config.
- (+) v2 eliminates manual token management entirely.
- (+) GitHub App provides the cleanest permission model.
- (+) OAuth for dashboard gives seamless user experience.
- (-) v1 PATs are long-lived secrets that must be rotated manually. Acceptable for initial deployment.
- (-) v2 GitHub App adds operational complexity (App registration, webhook secret, private key storage).
- (-) Transition from PAT to App requires CLI config change, but can be backwards-compatible (support both).
