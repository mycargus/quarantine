# ADR-036: Use GITHUB_TOKEN as Default CLI Auth; Server-Side Writes via Dashboard

**Status:** Proposed (amended 2026-04-20)
**Date:** 2026-04-19
**Supersedes:** [ADR-026](026-v2-cli-external-token-injection.md)
**Revises:** [ADR-008](008-auth-strategy.md), [ADR-011](011-system-architecture.md)

## Context

v1 requires users to create a PAT and store it as a CI secret
(`QUARANTINE_GITHUB_TOKEN`). The stated goal for v2 was to eliminate this
friction via a GitHub App.

ADR-026 chose `actions/create-github-app-token` as the v2 approach: users
configure 2 CI secrets (App ID + private key) and add a workflow step to
generate a short-lived installation token before invoking the CLI. Zero CLI
code changes, but 2 secrets to manage per repo (or per org).

On closer examination, `GITHUB_TOKEN` -- automatically provided by GitHub
Actions to every workflow run -- already satisfies all CLI requirements:

- `contents: write` -- read/write state on `quarantine/state` branch
- `issues: write` -- create quarantine issues
- `pull-requests: write` -- post PR comments

The user adds a `permissions:` block to their workflow file. No secrets to
create, rotate, or lose. The token is auto-provisioned, auto-rotated, and
scoped to the repo.

The rate limit is 1,000 req/hr/repo (GitHub Enterprise Cloud: 15,000/hr).
A typical quarantine run uses ~10-20 API calls. This is sufficient for most
repos. High-volume repos (many parallel CI runs per hour) may hit the limit.

The GitHub App's primary value for v2 is in the **dashboard**: OAuth login,
automatic repository discovery via `GET /app/installations`, and user
permission filtering. None of these require App credentials in the user's CI
workflow. The user installs the App on their org (one click, selects repos);
the dashboard, which already holds App credentials (M12-M15), does the rest.

When a user does hit CLI rate limits, the dashboard can act as a token proxy:
the CLI requests an installation token from the dashboard's `/api/ci-token`
endpoint. The dashboard mints a token scoped to the requesting repo. If the
dashboard is unreachable, the CLI falls back to `GITHUB_TOKEN`. The build
never breaks.

## Decision

**Use `GITHUB_TOKEN` as the default CLI auth mechanism for v2. No App
credentials in the user's CI workflow.**

### Default path (zero secrets)

1. User adds `quarantine run <suite>` to their workflow with a `permissions:`
   block:
   ```yaml
   permissions:
     contents: write
     issues: write
     pull-requests: write
   ```
2. Done. `GITHUB_TOKEN` is auto-provisioned by GitHub Actions. The CLI picks
   it up through the existing `GITHUB_TOKEN` env var fallback (already
   implemented in v1).

### Rate-limit upgrade path (no secrets, one env var)

When a user needs more than 1,000 req/hr:

1. Org admin installs the quarantine App on their org (one click, selects
   repos). No CI changes required.
2. User sets `QUARANTINE_DASHBOARD_URL` as a CI secret:
   ```yaml
   # .github/workflows/ci.yml
   jobs:
     test:
       env:
         QUARANTINE_DASHBOARD_URL: https://dashboard.example.com
       steps:
         - run: quarantine run unit -- npm test
   ```
   The dashboard URL is never stored in `.quarantine/config.yml`. It is a
   CI secret only, preventing credential-redirect attacks via malicious PRs.
3. CLI on startup: requests an installation token from
   `$QUARANTINE_DASHBOARD_URL/api/ci-token`. Falls back to `GITHUB_TOKEN`
   on any failure (timeout, 4xx, 5xx, network error).

### Token resolution order

The CLI resolves its GitHub token in this order, stopping at the first
success:

1. **`QUARANTINE_GITHUB_TOKEN`** (if set) -- use it. Skip everything else.
   This is the v1 PAT path. Backward-compatible. Explicit token always wins.
2. **Dashboard proxy** (if `QUARANTINE_DASHBOARD_URL` is set, AND step 1 did
   not match) -- call `POST /api/ci-token` on the dashboard. On success, use
   the returned installation token
   (5,000-12,500 req/hr, `quarantine[bot]` identity). On failure, log a
   warning and continue to step 3.
3. **`GITHUB_TOKEN`** (if set) -- use it. This is the zero-secrets default
   (1,000 req/hr, `github-actions[bot]` identity).
4. **None** -- exit 2 with error:
   `"No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN."`

### `/api/ci-token` security model

The dashboard endpoint verifies the caller's identity using the
`GITHUB_TOKEN` from the CI environment:

1. CLI sends `POST /api/ci-token` with `Authorization: Bearer <GITHUB_TOKEN>`.
2. Dashboard verifies the token using a two-phase check:
   **a. Prefix check (soft gate):** If the token has a known non-installation
   prefix (`ghp_`, `github_pat_`, `ghu_`, `gho_`), skip the behavioral check
   and return 404 immediately. If the token has the `ghs_` prefix or an
   unrecognized prefix, proceed to the behavioral check. This is a fast-path
   optimization — known-bad prefixes are rejected cheaply, but unknown
   prefixes always fall through to the authoritative behavioral check.
   **b. Behavioral check (authoritative):** Dashboard calls
   `GET /installation/repositories` using the received token. This returns
   exactly the repos the token is scoped to. `GITHUB_TOKEN` in GitHub Actions
   is scoped to one repo, so the response identifies which repo the caller
   belongs to. If this call fails (403, 401, network error), the token is
   rejected.
   **Note:** The `ghs_` prefix is observed behavior of GitHub's current token
   format, not a formally guaranteed API contract. The behavioral check is
   the authoritative verification — it works regardless of prefix format
   changes. The prefix check only rejects known-bad prefixes; it never
   rejects unknown prefixes.
3. Dashboard looks up its own installations to check if the identified repo
   is covered by the quarantine App.
4. If yes: mints an installation token scoped to that single repo via
   `POST /app/installations/{id}/access_tokens` with
   `repositories: ["<repo-name>"]`.
5. Returns the scoped installation token. Token expires in 1 hour (GitHub
   platform constraint).
6. If the repo is not in any installation, the token has a known-bad prefix,
   or the behavioral check fails: returns 404 (no information leakage about
   which repos have the App installed).

**Token caching:** Installation tokens are valid for 1 hour. The dashboard
caches them per repo (keyed by `owner/repo`) and returns the cached token
for subsequent requests. Tokens are refreshed proactively when less than
5 minutes remain before expiry (same pattern as `InstallationTokenProvider`
from M12). This reduces the per-request cost from 2 GitHub API calls
(verify + mint) to 1 (verify only) in steady state, and to 0 when the
verification result is also cached (TTL: 15 seconds).

**Rate limiting:** The `/api/ci-token` endpoint enforces rate limits to
prevent abuse from runaway CI loops or misconfigured workflows:
- 10 requests/minute per repo
- 10 requests/minute per token fingerprint (`SHA-256(token)[0:16]`)
Per-IP rate limiting is intentionally omitted — GitHub-hosted CI runners
share egress IP pools across unrelated organizations, making IP an unreliable
identity signal. Per-repo and per-fingerprint limits provide meaningful abuse
isolation. Excess requests receive 429 with a `Retry-After` header. The CLI
treats 429 the same as any other failure: falls back to `GITHUB_TOKEN`.

**Attack surface analysis:** An attacker with a valid `GITHUB_TOKEN` for
repo X can obtain a quarantine installation token for repo X. This is not
an escalation -- the `GITHUB_TOKEN` already grants write access to repo X.
The installation token provides higher rate limits and the `quarantine[bot]`
identity, but no additional permissions beyond what the App was granted on
that repo. The `repositories` parameter in step 4 ensures the token cannot
access other repos in the installation.

### Dashboard proxy timeout

The proxy call is bounded to prevent CI slowdown:

- **Default:** 3 seconds.
- **Minimum:** 3 seconds. Values below 3s are clamped to 3s.
- **Configurable** via `QUARANTINE_DASHBOARD_TIMEOUT` env var
  (Go duration string: `3s`, `5s`, `10s`).
- **Happy-path latency:** Network round-trip time (~100-500ms typically).
  The timeout value is only reached on failure (dashboard down, network
  partition).
- On timeout: CLI logs a warning and falls back to `GITHUB_TOKEN`. No retry.

### Dashboard env vars

Dashboard proxy configuration is env-var only. No config file entries.
This prevents credential-redirect attacks via malicious PRs that modify
committed config files.

| Env var | Purpose | Default |
|---------|---------|---------|
| `QUARANTINE_DASHBOARD_URL` | Dashboard proxy URL | (unset — proxy disabled) |
| `QUARANTINE_DASHBOARD_TIMEOUT` | Proxy call timeout | `3s` (minimum `3s`) |

### HTTPS enforcement

The CLI sends `GITHUB_TOKEN` in the `Authorization` header to the dashboard.
Transmitting it over plaintext HTTP is a credential leak. The CLI rejects
`QUARANTINE_DASHBOARD_URL` values that do not use HTTPS:

```
Error [config]: QUARANTINE_DASHBOARD_URL must use HTTPS (got "http://dashboard.example.com").
```

**Exception:** `http://localhost:*` and `http://127.0.0.1:*` are allowed
when the `CI` environment variable is not set (local development only).
In CI environments (`CI=true`), HTTPS is always required.

### Missing GITHUB_TOKEN with proxy configured

When `QUARANTINE_DASHBOARD_URL` is set but `GITHUB_TOKEN` is not available
(and `QUARANTINE_GITHUB_TOKEN` is not set), the CLI cannot call the proxy
(no token to send as proof of identity). The CLI logs a warning:

```
[quarantine] WARNING: QUARANTINE_DASHBOARD_URL is set but GITHUB_TOKEN is
not available. Cannot request installation token. Set QUARANTINE_GITHUB_TOKEN
or run in GitHub Actions.
```

Then proceeds to step 4 of the token resolution order (exit 2 with error).

### Bot identity

- **Default path** (`GITHUB_TOKEN`): issues and PR comments appear as
  `github-actions[bot]`.
- **Upgrade path** (dashboard proxy returns installation token): issues and
  PR comments appear as `quarantine[bot]` (or whatever the App is named).
  This is automatic -- GitHub attributes API actions to the App when an
  installation token is used.

### What this means for ADR-008 and ADR-011

**ADR-008 revision:** The App is no longer required for CLI auth. The App
provides: (a) dashboard OAuth login, auto-discovery, and permission filtering,
and (b) optional CLI rate-limit upgrade via the dashboard proxy. The v2 scope
boundaries section changes from "CLI unchanged, users use
`actions/create-github-app-token`" to "CLI unchanged, users use
`GITHUB_TOKEN` by default."

**ADR-011 revision:** The principle "CLI never needs to know the dashboard
exists" is relaxed to: "CLI never *depends* on the dashboard." When
`QUARANTINE_DASHBOARD_URL` is set, the CLI makes one outbound HTTP call to the
dashboard at startup, bounded by a configurable timeout (default 3s). On
any failure, the CLI proceeds with `GITHUB_TOKEN`. A dashboard outage
degrades rate limits (and loses `quarantine[bot]` identity), but does not
break the build and does not affect quarantine correctness.

**ADR-026 superseded:** No `actions/create-github-app-token` step required.
No App ID or private key in the user's CI workflow.

## Alternatives Considered

- **ADR-026 approach (actions/create-github-app-token):** Users configure 2
  CI secrets (App ID + private key) and add a workflow step. Rejected: does
  not meet the zero-secrets goal. Distributing the App's private key to every
  adopting org creates a shared-key security model (one leaked key
  compromises all installations) or requires each org to register its own App
  (defeating the simplicity goal).

- **PAT (v1 approach, unchanged):** Users create and manage a PAT. 5,000
  req/hr. Rejected: fails zero-secrets goal. PATs are tied to human accounts,
  do not expire automatically, and break when the token owner leaves the org.

- **Webhook-driven (v3 model pulled forward):** Server does all GitHub API
  work after CI completes; CLI only runs tests and uploads artifacts.
  Rejected: major architecture change, requires a reliable public endpoint,
  introduces eventual consistency (state/issues/comments update after CI
  finishes, not during), webhook delivery not guaranteed by GitHub (requires
  polling fallback anyway).

- **GITHUB_TOKEN only, no proxy:** Simplest possible design. Rejected as the
  complete solution: leaves no upgrade path for high-volume repos that hit
  1,000 req/hr. The dashboard proxy preserves simplicity for most users while
  offering a clean upgrade for power users.

## Consequences

**Positive:**

- (+) Zero secrets for the common case. No PAT to create, rotate, or lose.
- (+) `permissions:` block is a one-time copy-paste, not credential
  management.
- (+) Rate-limit upgrade requires no secrets -- one config value (or env var)
  and an App installation click.
- (+) Upgrade path also provides `quarantine[bot]` identity on issues and PR
  comments at no additional cost.
- (+) App credentials are held centrally on the dashboard. No private key
  distribution to user orgs.
- (+) Fork PRs handled safely: the CLI detects fork PR context
  (`GITHUB_EVENT_NAME=pull_request` + head repo differs from base repo) and
  skips quarantine processing entirely, executing the raw test command
  without quarantine wrapping. This eliminates attack vectors from untrusted
  fork code (XML injection, retry amplification, env var exfiltration).
  Enforced at the CLI level — no reliance on workflow configuration.
- (+) CLI token resolution order is additive -- v1 `QUARANTINE_GITHUB_TOKEN`
  still works, no breaking changes.
- (+) Dashboard proxy failure is invisible to the build -- falls back to
  `GITHUB_TOKEN` within the timeout window.

**Negative:**

- (-) 1,000 req/hr default. High-volume repos must configure the dashboard
  proxy and install the App.
- (-) **Behavioral change from error-handling spec:** The current
  error-handling spec (Category 1, degraded mode trigger 3) enters degraded
  mode when no GitHub token is set. This ADR changes step 4 to exit 2 with
  an error. Rationale: running `quarantine run` without any token is a
  configuration error, not a transient failure — degraded mode is for
  recoverable situations (API errors, rate limits) where the next run may
  succeed. A missing token will never self-resolve. The error-handling spec
  must be updated to reflect this change. (`--strict` behavior is unchanged:
  still exit 2.)
- (-) `permissions:` block is required for repos created after Feb 2023
  (GitHub's restricted default). If omitted, the CLI gets a read-only token
  and write operations fail. Must be clearly documented with a copy-paste
  snippet.
- (-) CLI gains a soft dashboard dependency when `QUARANTINE_DASHBOARD_URL` is
  configured. Adds ~100-500ms to CLI startup on the happy path (network
  round-trip). On failure (dashboard down), adds up to `dashboard.timeout`
  (default 3s) before falling back to `GITHUB_TOKEN`.
- (-) `GITHUB_TOKEN` is single-repo scoped. Cannot be used for cross-repo
  operations (not a current requirement).
- (-) `/api/ci-token` is a new public endpoint on the dashboard. Requires
  HTTPS enforcement on the CLI side, rate limiting (10/min per repo, 10/min
  per token fingerprint) on the dashboard side, and `GITHUB_TOKEN`
  verification via prefix check + `GET /installation/repositories`.
- (-) Verification requires one GitHub API call per unique CI token request
  (cached 15s, keyed by token fingerprint + repo). Installation token minting
  is cached per repo (1 hour TTL, proactive refresh). Steady-state cost
  per request is near zero.
- (-) Implementation requirements for security, UX, and operational concerns
  are specified in `docs/specs/v2-auth-token-proxy.md`.

## Amendment (2026-04-20): Server-Side Writes Replace Token Proxy

### Motivation

Security and architecture review identified three problems with the token proxy
approach:

1. **The `permissions:` block cost is unacceptable.** Specifying `permissions:`
   in a GitHub Actions workflow triggers an all-or-nothing reset: all
   unspecified permissions become `none` (only `metadata: read` is retained).
   This breaks other workflow steps (checkout, caching, deployments) unless the
   user audits and re-adds every permission. No surveyed product (Codecov,
   Snyk, Renovate, Dependabot) imposes this cost -- they either run server-side
   or use dedicated tokens. `GITHUB_TOKEN` write access requires
   `contents: write`, `issues: write`, and `pull-requests: write`, but even
   `contents: read` (needed for `actions/checkout`) is lost unless explicitly
   listed.

2. **The token proxy is a novel pattern with no industry precedent.** Sending
   `GITHUB_TOKEN` to a third party for credential exchange has no parallel in
   Codecov, Snyk, Renovate, or any surveyed tool. OIDC verification (the
   industry standard, used by Codecov, Chainguard Octo STS, and cloud
   providers) is more secure but still requires `id-token: write` in the
   `permissions:` block, triggering the same all-or-nothing problem. The proxy
   also introduces privilege escalation: the recommended `permissions:` block
   omits `actions: read`, but the minted installation token inherits it from
   the App registration.

3. **All CLI writes are post-hoc reporting.** The CI exit code (pass/fail) is
   determined locally before any GitHub API write. State updates, issue
   creation, and PR comments are bookkeeping that does not affect the build
   outcome. Moving them server-side eliminates all write credentials from CI
   without changing the developer experience for the most critical signal (did
   CI pass or fail?).

### Revised decision

**Replace the token proxy with server-side writes. The CLI uses `GITHUB_TOKEN`
for reads only. The dashboard performs all GitHub writes using its own App
installation tokens.**

#### v2 default path (zero config)

```yaml
# .github/workflows/ci.yml
steps:
  - uses: actions/checkout@v4
  - run: quarantine run unit
  - uses: actions/upload-artifact@v4
    if: always()
    with:
      name: quarantine-results-${{ github.run_id }}
      path: .quarantine/results.json
```

No `permissions:` block. No secrets. No env vars. The CLI reads quarantine
state using the default `GITHUB_TOKEN` (`contents: read`, granted by default
on all repos including restricted-default repos created after Feb 2023). The
CLI writes results to disk. The workflow uploads the artifact. The dashboard
processes the artifact and performs all writes (state updates, issue creation,
PR comments) using its own App installation tokens from M12-M15.

#### Token resolution (simplified)

1. **`QUARANTINE_GITHUB_TOKEN`** (if set) -- v1 PAT mode. CLI does all reads
   AND writes itself. Dashboard not involved. Backward-compatible.
2. **`GITHUB_TOKEN`** (if set) -- v2 mode. CLI reads state, runs tests, writes
   results to disk. No GitHub write API calls. Dashboard handles writes.
3. **Neither** -- exit 2 with error.

The four-step resolution order from the original decision is replaced by this
two-step order. The dashboard proxy step and all associated machinery
(`/api/ci-token`, HTTPS enforcement, proxy timeout, rate limiting, behavioral
check, verification cache, scoped token minting) are removed.

#### CLI behavioral modes

| Token available | Mode | CLI reads | CLI writes to GitHub | Dashboard writes |
|-----------------|------|-----------|---------------------|-----------------|
| `QUARANTINE_GITHUB_TOKEN` | v1 (PAT) | State branch, closed issues | State, issues, PR comment | Not involved |
| `GITHUB_TOKEN` only | v2 (server-side) | State branch | Nothing -- results to disk only | State, issues, PR comment (from artifact) |
| Neither | Error | -- | -- | -- |

In v2 mode the CLI:
1. Reads quarantine state from the state branch (read-only, `GITHUB_TOKEN`).
   If the state branch does not exist (404), proceeds with empty state (no
   exclusions) -- does NOT exit 2.
2. Runs tests, retries failures, classifies flaky/genuine/unresolved
3. Writes `.quarantine/results.json` to disk (includes `cli_mode: "v2"` so
   the dashboard knows writes were not performed by the CLI)
4. Exits with appropriate code (0/1/2)
5. Does NOT call any GitHub write APIs

#### `quarantine init` in v2

`quarantine init` is run by users locally (never in CI). It creates
`.quarantine/config.yml` (local file) and the `quarantine/state` branch
(GitHub API write). Users running init locally authenticate with a PAT,
`gh auth`, or other local credential -- init is not affected by the read-only
CI constraint.

If init has not been run, the dashboard creates the state branch on first
artifact processing. The CLI handles a missing state branch by running all
tests with no exclusions (empty state). This means v2 users can start using
quarantine without running init at all -- the dashboard handles setup.

#### Immediate feedback via webhooks (v3)

v2 writes are delayed by the artifact polling interval (~5 minutes). For
near-instant feedback (~30 seconds), the `workflow_run.completed` webhook
(deferred to v3 per [ADR-027](027-v2-webhooks-deferred.md)) is the natural
solution:

1. GitHub sends `workflow_run.completed` to the dashboard
2. Dashboard fetches the quarantine artifact from the completed run
3. Dashboard writes state, creates issues, posts PR comment

This requires zero CI-side configuration changes -- the webhook fires
automatically for any workflow run in a repo where the App is installed.

**Upgrade path timeline:**
- v2: Artifact polling (~5 min delay). Zero CI config.
- v3: `workflow_run.completed` webhook (~30s delay). Still zero CI config.

#### Fork PR handling (retained from original)

Fork PR detection ([SEC-7](../specs/v2-auth-token-proxy.md)) is retained. On
fork PRs, the CLI skips quarantine processing entirely, executing the raw test
command without quarantine wrapping. This prevents JUnit XML injection into
results that the dashboard would process. Event payload parsing hardening
(SEC-9) is also retained.

### What this replaces

**Removed from original decision:**
- The `/api/ci-token` endpoint and all token proxy machinery
- The four-step token resolution order (proxy step removed)
- `QUARANTINE_DASHBOARD_URL` and `QUARANTINE_DASHBOARD_TIMEOUT` env vars
- HTTPS enforcement for dashboard URL, localhost exception
- Proxy rate limiting (per-repo, per-fingerprint)
- Token verification (prefix check, behavioral check, verification cache)
- Scoped token minting (`repositories` parameter)
- Bot identity upgrade via installation token
- SEC-1 through SEC-6b from the companion spec
- All proxy-related UX messages (UX-3, UX-4, UX-5, UX-8)
- Proxy error handling (ERR-2, ERR-3, Category 4)

**Retained from original decision:**
- `GITHUB_TOKEN` as default auth (the core insight)
- `QUARANTINE_GITHUB_TOKEN` backward compatibility (v1 PAT mode)
- Fork PR detection (SEC-7) and event payload hardening (SEC-9)
- Exit 2 on no token (ERR-1)
- UX-1 (403 detection) for v1 PAT mode and state read errors

### Revised ADR-008 and ADR-011 impact

**ADR-008 revision (updated):** The App provides: (a) dashboard OAuth login,
auto-discovery, and permission filtering, and (b) server-side GitHub writes
(state, issues, PR comments) on behalf of CLI runs. The CLI never uses App
credentials, directly or indirectly.

**ADR-011 revision (updated):** The principle "CLI never needs to know the
dashboard exists" is fully preserved in v2. The CLI has no dashboard dependency
-- it reads state from GitHub and writes results to disk. The dashboard
processes artifacts independently. This is a stronger separation than the
original decision, which introduced a soft dashboard dependency via the proxy.

### Revised consequences

**Positive (changes from original):**

- (+) Zero CI configuration. No `permissions:` block, no secrets, no env vars.
- (+) No write-capable credentials in CI at all. The `GITHUB_TOKEN` used for
  state reads is read-only by default.
- (+) No new public endpoint on the dashboard (no `/api/ci-token`).
- (+) CLI has zero dashboard dependency. Dashboard outage does not affect CI in
  any way -- builds run normally, results are written to disk, dashboard
  catches up when it recovers.
- (+) Simpler implementation: no proxy, no OIDC, no behavioral check, no token
  caching, no verification cache, no HTTPS enforcement, no rate limiting.
- (+) Aligns with industry practice: server-side writes are how Dependabot,
  Renovate (hosted), and Codecov handle GitHub API writes.

**Negative (changes from original):**

- (-) State updates, issue creation, and PR comments are delayed by the
  artifact polling interval (~5 minutes in v2). The CI exit code (pass/fail)
  is still immediate. Webhooks in v3 reduce this to ~30 seconds.
- (-) Dashboard is required for quarantine management in v2. Without the
  dashboard, the CLI runs tests and detects flaky tests, but state is never
  updated, issues are never created, and PR comments are never posted. The
  quarantine function degrades to retry-only mode.
- (-) Write logic (issue body rendering, PR comment rendering, state merge,
  CAS, dedup) must be reimplemented in TypeScript for the dashboard. Parity
  with CLI behavior must be verified via contract tests against shared fixtures.
- (-) Slightly wider stale-state window between runs. A test detected as flaky
  in run N may not be excluded until run N+2 (instead of N+1) if the dashboard
  has not processed run N's artifact before run N+1 starts. The CAS merge
  mechanism handles concurrent updates correctly.
- (-) No `quarantine[bot]` identity upgrade path in v2. Issues and PR comments
  are created by the App's bot identity (determined by the dashboard's
  installation token), not configurable by the user. In v1 PAT mode, they
  appear as the PAT owner.

### Alternatives reconsidered

The original "Alternatives Considered" section rejected the webhook-driven
model as a "major architecture change." This amendment adopts a variant of
that model (artifact-based, not webhook-triggered) as the v2 approach, with
webhooks as the v3 optimization. The key insight: the CLI's writes are all
post-hoc reporting that can be deferred without affecting CI correctness.
