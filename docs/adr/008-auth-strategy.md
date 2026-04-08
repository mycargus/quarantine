# ADR-008: API Key Auth (v1), GitHub App + OAuth (v2)

**Status:** Accepted (refined by ADR-026, ADR-027, ADR-029)
**Date:** 2026-03-14

## Context

The system needs authentication for: CLI accessing GitHub API (to read/write quarantine.json, create issues, post PR comments) and dashboard accessing GitHub API (to pull artifacts, check issue status). The CLI does not upload artifacts directly — it writes results to a local file and the GitHub Actions workflow handles upload (ADR-007). v2 adds dashboard web UI authentication.

### v1 PAT friction (motivation for v2)

v1 authenticates via PATs stored as CI secrets. This creates friction that scales badly:

- **Per-repo configuration:** Every repo needs manual PAT setup. No org-wide rollout.
- **Long-lived secrets:** PATs are tied to individual human accounts and must be rotated manually. If the token owner leaves, all repos break.
- **Low rate limits:** The default `GITHUB_TOKEN` in Actions is limited to 1,000 req/hr/repo. PATs get 5,000/hr, but are still shared across all repos using the same token.
- **Branch protection conflicts:** PATs can be blocked by branch protection rules on `quarantine/state`, requiring the Actions cache fallback path.
- **No bot identity:** PR comments and issues appear from the token owner's account, not a recognizable bot.

A GitHub App solves all five problems: org-wide installation (configure once, all repos covered), short-lived installation tokens (1-hour expiry, 5,000-12,500 req/hr), fine-grained permissions, branch protection bypass eligibility, and an independent bot identity. It also unlocks dashboard OAuth login and automatic repository discovery.

### v2 scope boundaries

- **CLI unchanged:** Users authenticate via `actions/create-github-app-token` (ADR-026), which outputs a token the CLI consumes through the existing `QUARANTINE_GITHUB_TOKEN` env var. Zero CLI code changes.
- **Dashboard is the primary scope:** OAuth login (via `@remix-run/auth`, ADR-029), App-based installation token generation for artifact polling, and automatic repository discovery via a background polling loop.
- **GitHub Actions only:** v2 supports only GitHub Actions CI. CLI-native App auth (JWT generation in Go) is deferred until non-GitHub-Actions CI support is added.
- **No webhooks:** Deferred to v3 pending architectural review of public endpoint exposure (ADR-027).

## Decision

- **v1:** GitHub Personal Access Token (PAT) configured as CI secret. The CLI checks `QUARANTINE_GITHUB_TOKEN` first, then falls back to `GITHUB_TOKEN` if available. Same or separate PAT for dashboard to access GitHub Artifacts API. No CLI-to-dashboard auth needed (CLI does not talk to dashboard). Note: `GITHUB_TOKEN` in GitHub Actions is limited to 1,000 req/hr/repository. PATs get 5,000/hr. Recommend PAT or GitHub App for repos with high CI volume.
- **v2:** GitHub App installed on org. Provides:
  - Short-lived installation tokens (1-hour expiry) that the application must explicitly request via the GitHub API. SDKs (e.g., Octokit) handle token refresh transparently, but it is client-side logic, not a platform feature. No manual PAT management.
  - Fine-grained permissions (contents:read+write, issues:read+write, pull_requests:write, actions:read).
  - Higher rate limits (5,000/hr minimum, scaling to 12,500 based on repo count, vs. 1,000/hr for `GITHUB_TOKEN`).
  - Org-wide access (install once, all repos covered).
  - Branch protection bypass eligibility (repo admin adds App to ruleset bypass list).
  - Dashboard web UI uses GitHub OAuth via `@remix-run/auth` with `createGitHubAuthProvider` (ADR-029).

## Alternatives Considered

- **GitHub App from v1:** More secure and proper, but adds complexity (App registration, private key management, installation token exchange). Deferred to v2 to reduce initial scope.
- **Custom API key system:** Unnecessary in v1 since CLI talks to GitHub, not dashboard. Would be needed if using push model, but we chose pull model (ADR-007).
- **OAuth-only (no PAT support):** Does not work well in CI environments where interactive login is not possible.

## Consequences

- (+) v1 is dead simple -- one env var in CI config.
- (+) v2 eliminates manual token management entirely.
- (+) GitHub App provides the cleanest permission model.
- (+) OAuth for dashboard gives seamless user experience.
- (+) Branch protection bypass eligibility removes the need for the Actions cache fallback path.
- (+) Bot identity for PR comments and issues (e.g., "quarantine[bot]").
- (-) v1 PATs are long-lived secrets that must be rotated manually. Acceptable for initial deployment.
- (-) v2 GitHub App adds operational complexity (App registration, private key storage). No webhook secret needed (ADR-027).
- (-) Transition from PAT to App is backwards-compatible (`source: manual` continues to work).
