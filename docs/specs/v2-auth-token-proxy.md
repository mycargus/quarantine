# v2 Auth Token Proxy: Implementation Requirements

> Companion to [ADR-036](../adr/036-v2-cli-auth-github-token-default.md).
> This document captures security, UX, and operational requirements for the
> dashboard token proxy (`/api/ci-token`) and the CLI's v2 token resolution.
> ADR-036 records the decision; this spec records how to implement it safely.
>
> Last updated: 2026-04-19

---

## 1. Security Requirements

### SEC-1: Token handling hygiene

The dashboard receives a raw `GITHUB_TOKEN` in the `Authorization` header.
This token must be treated as ephemeral verification material:

- **Never log** the token value. Use a wrapper type that redacts on
  `String()` / `JSON.stringify()` / template interpolation.
- **Never persist** the token (no database, no cache of the raw value, no
  disk write).
- **Discard** the token from memory after the verification call
  (`GET /installation/repositories`) completes.

### SEC-2: Verification cache must be token-bound

The verification cache must be keyed by a fingerprint of the specific
`GITHUB_TOKEN`, not just by repo name. Use `SHA-256(token)` truncated to
16 bytes as the cache key component.

Without this, a stolen or expired token that was previously verified could be
replayed within the TTL window to obtain a new installation token.

- **Cache key:** `SHA-256(token)[0:16] + ":" + owner/repo`
- **TTL:** 15 seconds. Short TTL limits the replay window after token
  revocation. `GITHUB_TOKEN` is valid for the duration of a workflow run
  (up to 6 hours); a longer TTL would allow post-revocation replay.
- **Eviction:** Cache entries older than TTL MUST be evicted periodically
  to prevent unbounded memory growth. Use a sweep interval of 60 seconds
  or evict lazily on access.

### SEC-3: App permission parity

The GitHub App MUST NOT have permissions beyond what the CLI requires:

| Permission | Required | Purpose |
|------------|----------|---------|
| Contents | Read & write | State branch |
| Issues | Read & write | Quarantine issues |
| Pull requests | Write | PR comments |
| Actions | Read | Dashboard artifact download only |

This is a **hard requirement**, not a recommendation. If the App has broader
permissions (e.g., `administration`, `members`), the proxy becomes a privilege
escalation vector — a `GITHUB_TOKEN` with limited scope could be exchanged for
an installation token with broader scope.

Document this constraint in the GitHub App setup guide and verify it in the
dashboard's startup health check.

### SEC-4: Dashboard URL is env-var only

The dashboard URL MUST only come from the `QUARANTINE_DASHBOARD_URL`
environment variable. There is no `dashboard.url` config file option.

Rationale: a config file entry is committed to the repository. Any developer
with write access could change it to redirect CI runners' `GITHUB_TOKEN`
values to a malicious server via a PR. An env var set as a CI secret is not
modifiable by PRs.

### SEC-5: Localhost exception restricted to non-CI

The HTTPS exception for `http://localhost:*` and `http://127.0.0.1:*`
(ADR-036 line 190) must be disabled when the `CI` environment variable is
set to `true`. On shared CI runners, another process on the same machine
could bind to the expected port and intercept tokens.

- `CI=true` + `http://localhost:*` URL = reject with error:
  `"Error [config]: QUARANTINE_DASHBOARD_URL must use HTTPS in CI environments (got 'http://localhost:3000')."`
- `CI` unset or empty + `http://localhost:*` = allow (local development).

### SEC-6: Rate limiting

The `/api/ci-token` endpoint enforces two rate limit dimensions:

- **10 requests/minute per repo** (`owner/repo`)
- **10 requests/minute per token fingerprint** (`SHA-256(token)[0:16]`)

Per-IP rate limiting is intentionally omitted — GitHub-hosted CI runners
share egress IP pools across unrelated organizations, making IP an unreliable
identity signal.

Rate limit counters MUST evict expired entries periodically to prevent
unbounded memory growth. Use a sweep interval or lazy eviction on access.

### SEC-6a: Scoped token minting

The `/api/ci-token` handler MUST pass `repositories: ["<repo-name>"]` when
calling `POST /app/installations/{id}/access_tokens`. This scopes the minted
token to the single requesting repo.

The existing `InstallationTokenProvider` (from M12) does NOT pass the
`repositories` parameter — it mints tokens scoped to the entire installation.
The `/api/ci-token` handler MUST NOT reuse `InstallationTokenProvider`
without modification. Either extend it with an optional `repositories`
parameter or create a separate code path for proxy token minting.

Without this, a `GITHUB_TOKEN` scoped to repo X would obtain an installation
token with access to all repos in the installation — a privilege escalation.

### SEC-6b: SSRF protection on behavioral check

The behavioral check (`GET /installation/repositories`) makes an outbound
HTTP request using a caller-supplied token. The target URL MUST be hardcoded
to the dashboard's configured GitHub API base URL (e.g.,
`https://api.github.com` or the GHES API URL from environment config).

No request input (headers, body, query parameters) may influence the target
URL of the behavioral check. This prevents SSRF via crafted tokens or
request parameters.

### SEC-7: CLI skips quarantine processing on fork PRs

Fork PRs run untrusted code. Running quarantine on fork PRs creates attack
vectors: JUnit XML injection into issues/comments, retry amplification
(multiplied attacker execution budget), and environment variable exfiltration
during extended retry windows.

The CLI MUST detect fork PR context and skip quarantine processing entirely,
falling through to execute the raw test command:

- **Detection:** `GITHUB_EVENT_NAME=pull_request` + read the event payload
  at `GITHUB_EVENT_PATH` and check
  `head.repo.full_name != base.repo.full_name`.
- **Behavior:** Log a notice and exec the test command directly without
  quarantine wrapping (no state read, no retries, no issue creation, no
  proxy call):
  ```
  [quarantine] NOTE: Fork PR detected. Quarantine is skipped on fork PRs (security). Tests run normally.
  ```
- **Exit code:** Pass through the test command's exit code unmodified.
- **Non-GitHub-Actions environments:** If `GITHUB_EVENT_NAME` is not set,
  fork detection does not apply (the CLI cannot determine the context).

This is enforced at the CLI level — no reliance on users configuring
workflows correctly.

### SEC-9: Event payload parsing hardening

SEC-7 requires reading `GITHUB_EVENT_PATH` (a JSON file on the runner
filesystem). The CLI MUST handle failure gracefully:

- **Missing file** (`GITHUB_EVENT_PATH` unset or file does not exist): Skip
  fork detection, proceed normally. The CLI cannot determine context.
- **Malformed JSON**: Skip fork detection, proceed normally. Log at verbose
  level only (not a warning — this is a non-critical detection path).
- **Missing fields** (`head.repo.full_name` or `base.repo.full_name` absent):
  Skip fork detection, proceed normally.

The principle: fork detection is a safety optimization. If detection fails,
the safe default is to proceed as if this is NOT a fork PR (full quarantine
processing). A corrupt event file must not become a DoS vector or prevent
quarantine from running.

### SEC-8: Revocation procedure

If a dashboard instance is compromised:

1. **Uninstall the GitHub App** from affected orgs — immediately stops new
   installation token minting.
2. **Rotate the dashboard URL** — update `QUARANTINE_DASHBOARD_URL` in all
   affected CI secrets.
3. **Revoke outstanding installation tokens** — GitHub automatically revokes
   tokens when the App is uninstalled.

Document this in the operational runbook (not in the ADR).

---

## 2. UX Requirements

### UX-1: Detect missing `permissions:` block

This is the most likely onboarding failure. When `GITHUB_TOKEN` is resolved
(step 3) and the first write operation (e.g., state update, issue creation)
returns 403, the CLI must detect and surface the cause:

- **Log:**
  ```
  [quarantine] ERROR: GITHUB_TOKEN lacks write permissions (403 Forbidden).
  Add a permissions: block to your workflow file:
    permissions:
      contents: write
      issues: write
      pull-requests: write
  If you already have a permissions block, check for SAML SSO enforcement or IP allowlist restrictions.
  See: https://docs.github.com/en/actions/security-for-github-actions/security-guides/automatic-token-authentication#permissions-for-the-github_token
  ```
- **GitHub Actions annotation:**
  ```
  ::error title=Missing permissions::GITHUB_TOKEN lacks write permissions. Add a permissions: block to your workflow. See workflow documentation.
  ```
- This is a fatal error (exit 2), not degraded mode — a permissions
  misconfiguration will not self-resolve.

### UX-2: Distinguish error messages by token path

Error messages must indicate which token resolution step failed, so users know
what to fix:

| Failure | Message |
|---------|---------|
| `QUARANTINE_GITHUB_TOKEN` 401 | `"QUARANTINE_GITHUB_TOKEN is invalid or expired (401). Check the token value."` |
| Dashboard proxy failed | `"Dashboard proxy at {url} failed ({reason}). Falling back to GITHUB_TOKEN."` |
| `GITHUB_TOKEN` 403 on write | `"GITHUB_TOKEN lacks write permissions (403). Add a permissions: block..."` (UX-1) |
| No token at all | `"No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN."` |
| No token + `QUARANTINE_DASHBOARD_URL` set | `"No GitHub token found. GITHUB_TOKEN is required for dashboard proxy authentication. Set GITHUB_TOKEN in your workflow or set QUARANTINE_GITHUB_TOKEN."` |

### UX-3: Prominent proxy failure warning

When the dashboard proxy call fails (timeout, 4xx, 5xx, network error), the
warning must be prominent and include the URL that failed:

- **Stderr:**
  ```
  [quarantine] WARNING: Dashboard proxy at https://dashboard.example.com/api/ci-token failed (timeout after 3s). Falling back to GITHUB_TOKEN (1,000 req/hr).
  ```
- **GitHub Actions annotation:**
  ```
  ::warning title=Dashboard Proxy Unavailable::Proxy at https://dashboard.example.com/api/ci-token failed (timeout after 3s). Using GITHUB_TOKEN with 1,000 req/hr rate limit.
  ```

### UX-4: Notice when explicit token skips proxy

When `QUARANTINE_GITHUB_TOKEN` is set AND `QUARANTINE_DASHBOARD_URL` is set, the
proxy is silently skipped (step 1 wins). Log a notice so users understand why
their bot identity config is not taking effect:

```
[quarantine] NOTE: Using QUARANTINE_GITHUB_TOKEN (dashboard proxy skipped).
```

### UX-5: Notice when timeout is clamped

When `QUARANTINE_DASHBOARD_TIMEOUT` is set below the 3s minimum and clamped:

```
[quarantine] NOTE: QUARANTINE_DASHBOARD_TIMEOUT clamped to minimum 3s (configured: 1s).
```

Emit at config parse time, not at proxy call time.

### UX-6: Context-aware "run in GitHub Actions" message

The warning for missing `GITHUB_TOKEN` with proxy configured (ADR-036 line
200) says `"Set QUARANTINE_GITHUB_TOKEN or run in GitHub Actions."` This is
not actionable if the user is already in GitHub Actions.

When `GITHUB_ACTIONS=true`:
```
[quarantine] WARNING: QUARANTINE_DASHBOARD_URL is set but GITHUB_TOKEN is
not available. Check that your workflow has a permissions: block with
contents: write, issues: write, pull-requests: write.
```

When `GITHUB_ACTIONS` is not set:
```
[quarantine] WARNING: QUARANTINE_DASHBOARD_URL is set but GITHUB_TOKEN is
not available. Set QUARANTINE_GITHUB_TOKEN or run in GitHub Actions.
```

### UX-7: `permissions:` block prerequisite in default path

The "Default path" section of ADR-036 says "Done." after step 2, but
`quarantine init` must have been run first. The quick-start documentation
must present the full sequence:

1. `quarantine init`
2. Add `permissions:` block to workflow
3. Add `quarantine run <suite>` step

### UX-8: Dashboard proxy discoverability

The dashboard URL is env-var only (SEC-4), so `.quarantine/config.yml` gives
no hint that a rate-limit upgrade path exists. `quarantine doctor` output
SHOULD mention the upgrade path when the CLI detects rate limit pressure:

```
[quarantine] NOTE: GitHub API rate limit low (50 remaining). For higher
limits, set QUARANTINE_DASHBOARD_URL. See: <docs-url>
```

This is informational — it does not leak any URL or secret.

---

## 3. Operational Requirements

### OPS-1: App permission verification at startup

On dashboard startup, verify the App's configured permissions match SEC-3.
If the App has permissions beyond the required set, log a warning:

```
[dashboard] WARNING: GitHub App has 'administration:write' permission which
is not required by quarantine. Consider reducing App permissions to minimize
proxy escalation risk.
```

### OPS-2: `pull_request_target` warning in setup guide

The GitHub App setup guide and CI integration docs must warn:

> `pull_request_target` provides write-capable `GITHUB_TOKEN` for fork PRs
> but runs in the context of the base repo. Do not use `pull_request_target`
> to check out and execute untrusted fork code. The CLI's graceful degradation
> on fork PRs (read-only `GITHUB_TOKEN`) is the intended safe behavior for
> `pull_request` triggers.

### OPS-3: GHES rate limit note

ADR-036 assumes github.com rate limits. Add a note to documentation:

> GitHub Enterprise Server (GHES) has configurable rate limits that may
> differ from github.com defaults. Verify your GHES rate limit configuration
> if the CLI reports unexpected 429 errors.

### OPS-4: `actions: read` exclusion documented

The CLI's `permissions:` block intentionally excludes `actions: read` because
artifact upload is handled by the workflow's `actions/upload-artifact` step,
not the CLI. Document this in the CI integration guide to preempt questions.

---

## 4. Error-handling spec updates required

ADR-036 introduces changes that require updates to `docs/specs/error-handling.md`:

1. **Degraded mode trigger 3** (line 456): Change "GitHub token is not set"
   from degraded mode to exit 2. A missing token is a configuration error,
   not a transient failure.
2. **New Category 4: Dashboard Proxy Errors (CLI):** Add error handling for
   the `/api/ci-token` call — timeout, 4xx, 5xx, network error — all trigger
   fallback to `GITHUB_TOKEN`, not degraded mode.
3. **401 warning message** (line 66): When the 401 comes from a proxy-minted
   token (not a user-provided token), the message should differ:
   `"Dashboard-minted token rejected (401). This may indicate the App was
   uninstalled or the token expired."` vs the existing env-var-focused message.

---

## 5. E2E Test Coverage

### E2E-1: CLI + dashboard proxy round-trip

The CLI and dashboard proxy are tested independently at the interface layer
in their respective milestones. At least one E2E test must verify the full
round-trip against real external dependencies:

1. CLI reads `QUARANTINE_DASHBOARD_URL` env var
2. CLI sends `POST /api/ci-token` with `GITHUB_TOKEN`
3. Dashboard verifies the token (prefix + behavioral check)
4. Dashboard mints a scoped installation token
5. CLI receives the token and uses it for subsequent API calls
6. API calls succeed with the installation token

This test belongs in `test/e2e/` and requires a GitHub App installation on
the test fixture repo. Per the test strategy, E2E tests observe real fixture
CI output — they never run the CLI binary directly.

Also test the fallback path:
- Dashboard unreachable → CLI falls back to `GITHUB_TOKEN`
- Dashboard returns 404 (repo not in installation) → CLI falls back

---

## Cross-references

- [ADR-036](../adr/036-v2-cli-auth-github-token-default.md) — Decision record
- [Error handling spec](error-handling.md) — Requires updates (section 5)
- [Non-functional requirements](non-functional-requirements.md) — SEC items
  map to NFR-2.3, NFR-2.7; UX items map to NFR-2.11.4 (parseable error prefixes)
- [GitHub App setup guide](../../GITHUB_APP_SETUP.md) — OPS items
