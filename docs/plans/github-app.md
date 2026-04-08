# Plan: Quarantine GitHub App

## Context

Quarantine v1 authenticates to GitHub via Personal Access Tokens (PATs) stored as CI secrets. This creates friction that scales badly: every repo needs manual PAT configuration, tokens are long-lived secrets tied to individual human accounts, the default `GITHUB_TOKEN` in Actions is limited to 1,000 req/hr, and branch protection can block writes to the `quarantine/state` branch.

A GitHub App solves these problems by providing org-wide installation (configure once, all repos covered), short-lived installation tokens (1-hour expiry, 5,000-12,500 req/hr), fine-grained permissions, branch protection bypass eligibility, and an independent bot identity. It also unlocks dashboard OAuth login and automatic repository discovery.

**v2 scope:** GitHub Actions is the only supported CI. The CLI requires no code changes -- users authenticate via `actions/create-github-app-token`, which outputs a token the CLI consumes through the existing `QUARANTINE_GITHUB_TOKEN` env var. Dashboard changes are the primary scope: OAuth login (via remix-auth), App-based installation token generation for artifact polling, and automatic repository discovery via a background polling loop.

**Deferred to future milestones:**
- **CLI-native App auth** (JWT generation in Go, `TokenProvider` interface, `InstallationTokenProvider`): needed only when non-GitHub-Actions CI is supported (Jenkins, GitLab, etc.).
- **Webhooks** (real-time unquarantine, instant artifact ingestion): deferred to v3 pending architectural review of public endpoint exposure.

This plan is based on official GitHub documentation and the existing v2 plans in ADR-008, the architecture spec, and the v2 scenarios.

---

## 1. User Stories

### Org Admin

- **US-1.1:** As an org admin, I want to install a single GitHub App on my organization so that all repositories (or a selected subset) can use quarantine without configuring individual PATs per repo.
- **US-1.2:** As an org admin, I want the App to use fine-grained permissions (contents, issues, pull requests, actions) so I can audit exactly what the tool accesses, instead of granting broad `repo` scope via a PAT.
- **US-1.3:** As an org admin, I want to add the Quarantine App to repository ruleset bypass lists so the CLI can write to `quarantine/state` even when branch protection is enabled.
- **US-1.4:** As an org admin, I want the dashboard to auto-discover repositories after I install the App, without manually editing `dashboard.yml` for every new repo.
- **US-1.5:** As an org admin, I want to suspend or uninstall the App and have the system degrade gracefully (CLI falls back to cached state, dashboard stops polling) rather than causing CI failures.

### Developer

- **US-2.1:** As a developer, I want to log into the quarantine dashboard via "Sign in with GitHub" so I can view test stability data without needing a separate account.
- **US-2.2:** As a developer, I want the dashboard to only show repositories I have access to (based on my GitHub permissions), not every repo where the App is installed.
- **US-2.3:** As a developer, I want PR comments and issues to appear as the App's bot identity (e.g., "quarantine[bot]") rather than from a personal account.
- **US-2.4:** As a developer, I want the CLI to continue working with a PAT if my team hasn't migrated to the App, so the upgrade isn't forced.

### DevOps / Platform Engineer

- **US-3.1:** As a platform engineer, I want to configure the App's credentials (App ID, private key) as org-level CI secrets once, then use `actions/create-github-app-token` so all repositories get short-lived installation tokens automatically.
- **US-3.2:** As a platform engineer, I want the dashboard to use the App's private key to generate its own installation tokens (1-hour expiry, 5,000-12,500 req/hr) instead of relying on a long-lived PAT.
- **US-3.3:** As a platform engineer, I want rate limit usage logged and monitored so I can track GitHub API consumption across all installations.

---

## 2. Functional Requirements

### 2.1 App Registration and Configuration

| ID | Requirement |
|----|-------------|
| FR-APP-1 | Register the GitHub App under the `mycargus` account. Name: 34 chars max, globally unique (e.g., "quarantine"). |
| FR-APP-2 | Request exactly these permissions: `contents:read+write`, `issues:read+write`, `pull_requests:write`, `actions:read`. |
| FR-APP-3 | Register the App with webhooks **disabled**. No webhook URL, no webhook secret. Webhooks are deferred to v3. |
| FR-APP-4 | Register an OAuth callback URL for dashboard login (e.g., `https://<dashboard-host>/auth/github/callback`). |
| FR-APP-5 | Optionally register a setup URL for post-installation redirect (e.g., `https://<dashboard-host>/setup`). |
| FR-APP-6 | Store the App's private key (PEM, PKCS#1 RSAPrivateKey) in a secrets manager or CI secret. Never in source code, config files, or `quarantine.yml`. |
| FR-APP-7 | Register a separate "dev" App for development and testing with its own credentials and private key. Dev and prod Apps are independent registrations. Both have webhooks disabled. |

### 2.2 CLI Authentication (Unchanged)

The v2 CLI requires **no code changes** for GitHub App auth. The standard `actions/create-github-app-token` action handles JWT generation, installation ID lookup, and token exchange. The CLI receives the resulting token as `QUARANTINE_GITHUB_TOKEN`.

**Token resolution order (unchanged from v1):**

```
1. QUARANTINE_GITHUB_TOKEN env var  -> Bearer token (PAT or App token)
2. GITHUB_TOKEN env var             -> Bearer token (Actions default)
3. none                             -> degraded mode (warning, not fatal)
```

| ID | Requirement |
|----|-------------|
| FR-CLI-1 | CLI token resolution is unchanged. `QUARANTINE_GITHUB_TOKEN` takes priority over `GITHUB_TOKEN`. No new env vars. |
| FR-CLI-2 | The recommended CI integration uses `actions/create-github-app-token` to generate a short-lived token, passed as `QUARANTINE_GITHUB_TOKEN`. The CLI cannot distinguish App tokens from PATs -- both are Bearer tokens. |
| FR-CLI-3 | CLI-native App auth (JWT generation in Go, `TokenProvider` interface, `InstallationTokenProvider`) is deferred to the milestone that adds non-GitHub-Actions CI support. |

**Recommended CI usage:**

```yaml
- uses: actions/create-github-app-token@v3
  id: app-token
  with:
    app-id: ${{ vars.QUARANTINE_APP_ID }}
    private-key: ${{ secrets.QUARANTINE_APP_PRIVATE_KEY }}

- name: Run tests with quarantine
  run: quarantine run -- jest --ci
  env:
    QUARANTINE_GITHUB_TOKEN: ${{ steps.app-token.outputs.token }}
```

### 2.3 Dashboard Authentication (OAuth via remix-auth + Installation Tokens)

**Note:** Dashboard env vars (below) are separate from CLI env vars. The CLI uses only `QUARANTINE_GITHUB_TOKEN` or `GITHUB_TOKEN` (unchanged from v1). CI workflows use `QUARANTINE_APP_ID` and `QUARANTINE_APP_PRIVATE_KEY` for `actions/create-github-app-token` (see section 2.2). The full env var inventory across components is in section 5.4 (CI secrets table).

| ID | Requirement |
|----|-------------|
| FR-DASH-1 | GitHub OAuth login uses Remix 3's first-party `@remix-run/auth` with `createGitHubAuthProvider` and `@remix-run/auth-middleware` for route protection (ADR-029). The library handles OAuth redirect, PKCE code challenge, code exchange, and user profile fetching. |
| FR-DASH-2 | Session lifetime matches GitHub access token lifetime (8 hours). No refresh tokens stored or used. User re-authenticates via OAuth after session expiry. |
| FR-DASH-3 | Session: encrypted httpOnly secure SameSite=Lax cookie with Max-Age=28800 (8hr). Encrypted cookie holds access token and user profile via `createCookieSessionStorage`. No server-side session table. |
| FR-DASH-4 | Dashboard uses user's access token to call `GET /user/installations` then `GET /user/installations/{id}/repositories` per installation (all paginated, `per_page=100`, following `Link` header `rel="next"`) and filters repos to only those the user can access. |
| FR-DASH-5 | For server-side GitHub API calls (artifact polling, installation discovery), the dashboard generates installation tokens from the App's private key. Replaces the PAT from `QUARANTINE_GITHUB_TOKEN`. |
| FR-DASH-6 | `dashboard.yml` gains `source: github-app` mode. Repo list populated from App installations. `source: manual` continues to work with PATs for backward compatibility. When `source: github-app`, the `repos` array is silently ignored if present. The two modes are mutually exclusive. |
| FR-DASH-7 | Unauthenticated access limited to `/auth/login`, `/auth/github/callback`, `/auth/logout`, `/health`. All other routes return 401. `/auth/logout` is public so that users with expired sessions can still trigger logout without receiving a 401. |
| FR-DASH-8 | Rate limits per ADR-015: 20 req/min/IP unauthenticated, 300 req/min/user authenticated. |
| FR-DASH-9 | New routes: `/auth/login`, `/auth/github/callback`, `/auth/logout`, `/health`. |

**Dashboard env vars for App auth:**

| Env Var | Required | Description |
|---------|----------|-------------|
| `QUARANTINE_APP_CLIENT_ID` | Yes | App client ID (e.g., `Iv23li...`). Used as JWT `iss` claim. |
| `QUARANTINE_APP_CLIENT_SECRET` | Yes | App client secret. Used by `@remix-run/auth` for OAuth token exchange. |
| `QUARANTINE_APP_PRIVATE_KEY_PATH` | Yes* | Path to PEM file. |
| `QUARANTINE_APP_PRIVATE_KEY` | Yes* | PEM contents as env var value. Alternative to path. |
| `QUARANTINE_APP_ORIGIN` | Yes | Dashboard origin for constructing OAuth `redirectUri` (e.g., `http://localhost:3000` for dev, `https://dashboard.example.com` for prod). |

\* One of `QUARANTINE_APP_PRIVATE_KEY_PATH` or `QUARANTINE_APP_PRIVATE_KEY` is required.

**Note:** Installation IDs are discovered dynamically via `GET /app/installations` (M12), not configured as env vars.

### 2.4 Dashboard App Auth (JWT + Installation Tokens)

| ID | Requirement |
|----|-------------|
| FR-AUTH-1 | JWT generation is a pure function: `generateJWT(clientID: string, privateKeyPEM: string, now: Date): string`. Produces RS256 JWT with `iss` = client ID, `iat` = now-60s, `exp` = now+9min. |
| FR-AUTH-2 | Installation token exchange: `POST /app/installations/{id}/access_tokens` with JWT in `Authorization: Bearer` header. Returns token with `expires_at`. |
| FR-AUTH-3 | Installation token requests omit the `permissions` body parameter, inheriting the App's configured permissions. Avoids maintaining the permission list in two places. |
| FR-AUTH-4 | `InstallationTokenProvider` caches the installation token and refreshes when < 5 minutes remain before expiry. At most one token exchange per ~55 minutes per installation. |
| FR-AUTH-5 | If installation token exchange fails, dashboard logs a warning. Artifact polling for that installation's repos pauses until the next successful token exchange (triggered by the discovery loop's next interval). Installation discovery continues on schedule. Dashboard continues serving cached data. |

### 2.5 Installation Discovery

| ID | Requirement |
|----|-------------|
| FR-INST-1 | SQLite `installations` table: `id` (GitHub installation ID, numeric), `account_login`, `account_type`, `suspended_at` (ISO 8601 timestamp or NULL; NULL = active, timestamp = suspended; GitHub API has no `status` field — suspension state is conveyed via `suspended_at`), `repository_selection` (all/selected), `removed_at` (ISO 8601 timestamp or NULL; installations that no longer appear in `GET /app/installations` are marked with a non-NULL `removed_at`), `created_at`, `updated_at`. |
| FR-INST-2 | Add nullable `installation_id` column to existing `projects` table (FK to `installations.id`). When `source: github-app`, projects have an `installation_id`. When `source: manual`, `installation_id` is NULL. No separate `installation_repos` join table. |
| FR-INST-3 | When `source: github-app`, artifact polling uses projects discovered from installations instead of manual `repos` list. |
| FR-INST-4 | **Startup sync:** On startup, dashboard generates a JWT, calls `GET /app/installations` to list installations (paginated, `per_page=100`, following `Link` header `rel="next"`), then generates an installation token per installation and calls `GET /installation/repositories` to list repos (paginated). Upserts into `installations` and `projects` tables. Blocks serving traffic until complete. |
| FR-INST-5 | **Background discovery loop:** After startup, re-sync installations every 15 minutes via `setInterval`. First tick fires 15 minutes after startup (not immediately, since startup sync already ran). |
| FR-INST-6 | **Shutdown:** `process.on('SIGTERM', ...)` and `process.on('SIGINT', ...)` clear the interval. |
| FR-INST-7 | **Error resilience:** `syncInstallations()` catches all errors internally, logs them, never throws. Failed syncs leave existing `projects` table unchanged. The loop continues on next interval. |
| FR-INST-8 | **Suspended/removed installations:** The GitHub API conveys suspension via `suspended_at` (null = active, ISO 8601 timestamp = suspended). Dashboard stores this value in `installations.suspended_at`. Installations that no longer appear in `GET /app/installations` are marked with a non-NULL `removed_at` timestamp (deletion is inferred by sync diff, not an API field). Artifact polling skips repos linked to suspended or removed installations. |
| FR-INST-9 | Use GitHub numeric `id` for installations, repos, and users. Never names or slugs (per GitHub best practices -- names can change). |

### 2.6 Branch Protection Bypass

| ID | Requirement |
|----|-------------|
| FR-BP-1 | Branch protection bypass is a **repo-admin action**, not an App capability. The App's `contents:write` makes it eligible, but a repo admin must add it to the repository ruleset bypass list. |
| FR-BP-2 | Documentation instructs admins on adding the App to their ruleset bypass list (Settings > Rules > Rulesets > Bypass list > Add App). |
| FR-BP-3 | When bypassed, the CLI's Contents API writes to `quarantine/state` succeed without the Actions cache fallback. |
| FR-BP-4 | When NOT bypassed, existing fallback behavior (warn + use Actions cache) continues to work. |

---

## 3. Non-Functional Requirements

### 3.1 Security

| ID | Requirement |
|----|-------------|
| NFR-SEC-1 | Private key read from file path (`QUARANTINE_APP_PRIVATE_KEY_PATH`) or injected as env var value (`QUARANTINE_APP_PRIVATE_KEY`). Dashboard-only in v2. Never in source code or config files. |
| NFR-SEC-2 | JWT: `iat` = now minus 60s (clock skew tolerance), `exp` = max 10 minutes from `iat`, `alg` = RS256, `iss` = App client ID. |
| NFR-SEC-3 | OAuth handled by `@remix-run/auth` with `createGitHubAuthProvider` (ADR-029), which manages PKCE code challenge, state parameter (CSRF), and code exchange. HTTPS redirect URIs enforced in production via `QUARANTINE_APP_ORIGIN`. |
| NFR-SEC-4 | Session cookies: httpOnly, secure (HTTPS-only), SameSite=Lax, Max-Age=28800. Encrypted cookie holds access token and user profile. No server-side session table. |
| NFR-SEC-5 | Auth events logged with timestamps and user IDs. Token values never logged. |

### 3.2 Performance

| ID | Requirement |
|----|-------------|
| NFR-PERF-1 | Installation tokens cached, refreshed only when < 5 min remaining. Max one `POST /access_tokens` per ~55 min per installation. |
| NFR-PERF-2 | JWT generation: < 1ms overhead (pure CPU, RSA signing, no I/O). |
| NFR-PERF-3 | Rate limit budget monitored via `X-RateLimit-Remaining`. Warning logged at < 20% remaining. |
| NFR-PERF-4 | Installation discovery: ~2+ API calls per poll cycle (varies with installation/repo count due to pagination at `per_page=100`). At 15-min intervals, negligible rate limit impact for typical orgs (< 500 repos). |
| NFR-PERF-5 | On-demand artifact pull debouncing (per ADR-015): max 1 pull per repo per 5 minutes regardless of user page loads. Existing v1 behavior, unchanged by App auth. |

### 3.3 Reliability

| ID | Requirement |
|----|-------------|
| NFR-REL-1 | Missing/invalid App credentials on dashboard: startup fails with descriptive error. Artifact polling and installation discovery do not start. |
| NFR-REL-2 | Installation suspended: dashboard pauses artifact polling for affected repos, resumes on unsuspension (detected by next discovery poll). |
| NFR-REL-3 | Mixed auth supported: some repos use PATs (via `source: manual`), others use App (via `source: github-app`). No global cutover required. |
| NFR-REL-4 | Discovery loop error isolation: a failed `syncInstallations()` call does not crash the process or affect in-flight HTTP requests. Existing project data remains available. |

### 3.4 Observability

| ID | Requirement |
|----|-------------|
| NFR-OBS-1 | CLI logs auth method at startup: `Auth: PAT (QUARANTINE_GITHUB_TOKEN)` or `Auth: PAT (GITHUB_TOKEN)`. CLI cannot distinguish App-generated tokens from PATs; this is expected. |
| NFR-OBS-2 | Rate limit headers logged in verbose mode (existing behavior). |
| NFR-OBS-3 | Dashboard logs installation token refresh events with timestamp and new expiry. |
| NFR-OBS-4 | Dashboard logs installation discovery results: installations found, repos added/removed, errors. |

---

## 4. Constraints

### 4.1 GitHub Platform Constraints

| Constraint | Impact |
|-----------|--------|
| JWT `exp` max 10 min from `iat` | JWTs regenerated per token request (or cached ~9 min). |
| Installation token expiry: 1 hour | Token provider must track expiry and refresh proactively. |
| User access token expiry: 8 hours | Session cookie expires at 8 hours — no refresh tokens, user re-authenticates via OAuth. |
| Max 25 private keys per App | Supports rotation with headroom. Keys don't expire (manual revocation). |
| Installation token rate limits: 5,000 base, max 12,500 req/hr | Scales +50/hr per repo beyond 20 and +50/hr per user beyond 20. |
| Secondary rate limits: 100 concurrent, 900 pts/min | Dashboard must stagger API calls. |
| Content creation: 80/min, 500/hr | Limits state writes. Not a concern at expected volume. |
| OAuth token exchange on github.com, not api.github.com | Different base URL for OAuth vs REST API. Handled by `@remix-run/auth` GitHub provider. |
| Branch protection bypass is repo-admin configured | App cannot self-grant bypass. Requires user documentation. |

### 4.2 Architecture Constraints (from ADRs)

| Constraint | Source |
|-----------|--------|
| v1 PAT auth must continue to work | ADR-008 |
| No SaaS in CI path -- CLI never calls dashboard | ADR-011 |
| Auth failures are warnings, never fatal (CLI) | Error handling spec |
| GitHub IS the backend -- no external message broker | ADR-011 |
| No secrets in `quarantine.yml` | CLAUDE.md |
| Functional Core / Imperative Shell | Test strategy |
| No mocks in unit tests | Test strategy |
| v2 CLI unchanged -- external token injection only | This plan |
| OAuth via `@remix-run/auth` -- no custom OAuth implementation (ADR-029) | This plan |

### 4.3 Scope Exclusions

| Excluded | Reason |
|----------|--------|
| CLI-native App auth (Go JWT, TokenProvider, InstallationTokenProvider) | Deferred until non-GitHub-Actions CI support is added. `actions/create-github-app-token` covers v2. |
| Webhooks (real-time unquarantine, instant artifact ingestion) | Deferred to v3. Requires public endpoint exposure; architectural review pending. |
| Code sync adapter (skip markers) | Separate v2 feature. Depends on App but not part of App auth. |
| Jira, Slack, email integration | Separate v2+ features. |
| Jenkins/GitLab CI support | Separate v2+ feature. Triggers CLI-native App auth work. |
| Multi-org support | v3 feature. |
| App Manifest flow (programmatic registration) | Manual registration sufficient for v2 launch. |
| GitHub Checks API | Not in current permissions. Future enhancement. |
| GHES support | BaseURL override exists but GHES has different App registration. Deferred. |

---

## 5. Test Plan

### 5.1 Unit Tests

Pure functions, no I/O, no mocks.

**Dashboard JWT Generation** (`dashboard/app/lib/jwt.server.ts`, `.test.ts`)
- `generateJWT(clientID, privateKeyPEM, now)` produces valid RS256 JWT
- Verify decoded claims: `iss` = clientID, `iat` = now-60s, `exp` = now+9min
- Invalid PEM returns descriptive error
- Public key (not private) returns error
- Empty clientID returns error
- Test key pair generated programmatically in test -- no fixture keys committed

**Installation Discovery Debounce** (`dashboard/app/lib/installation-sync.server.ts`, `.test.ts`)
- `shouldSyncInstallations(lastSyncedAt, now, intervalMs)` returns boolean
- Stale timestamp returns true
- Fresh timestamp returns false
- Null/missing timestamp returns true
- Exact boundary returns false
- Follows existing `shouldPull()` pattern from `sync.server.ts`

**Config Validation** (`dashboard/app/lib/config.server.ts`, `.test.ts`)
- `source: github-app` validates without `repos` array
- `source: github-app` with `repos` present validates (silently ignored)
- `source: manual` without `repos` returns validation error (existing behavior)

**One-time setup:** None. Test keys generated programmatically. No external dependencies.

### 5.2 Integration Tests

Full component flows with mock HTTP servers. Real SQLite.

**Dashboard OAuth Flow** (`dashboard/test/oauth-flow.integration.test.ts`)
- Verify `@remix-run/auth` GitHub provider is configured with correct client ID and callback URL
- Authenticated `GET /` -> 200
- Unauthenticated `GET /` -> 401
- `GET /auth/login` -> redirects to GitHub OAuth URL
- `GET /auth/logout` -> clears session, redirects to login
- `GET /health` -> 200 (unauthenticated, always accessible)
- Note: full OAuth code exchange is tested by `@remix-run/auth` library. Dashboard integration tests verify route protection and session behavior.

**Dashboard User Permission Filtering** (`dashboard/test/user-permissions.integration.test.ts`)
- Mock GitHub API serves `GET /user/installations` returning 2 installations (3 repos total)
- User with access to 2 of 3 repos: filtered response shows only accessible repos
- User with access to 0 repos: empty project list, not an error
- Invalid/expired user access token: returns 401, session cleared

**Dashboard Installation Token Exchange** (`dashboard/test/installation-token.integration.test.ts`)
- `httptest`-style mock serves `POST /app/installations/{id}/access_tokens` returning canned token
- `InstallationTokenProvider` generates JWT, exchanges for token, uses it for API calls
- Token caching: second API call reuses cached token (counter on mock tracks calls)
- Token refresh: after simulated expiry, next call triggers new exchange
- Network error during exchange: returns error, dashboard logs warning

**Dashboard Installation Discovery** (`dashboard/test/installation-sync.integration.test.ts`)
- Mock GitHub API returns 2 installations with 3 repos total. After sync, `installations` table has 2 rows, `projects` table has 3 rows with correct `installation_id` foreign keys.
- Idempotency: run sync twice with same mock response. Row counts unchanged, `updated_at` timestamps refreshed.
- Repo added: first sync returns 2 repos, second sync returns 3 repos. Third project row appears.
- Repo removed: first sync returns 3 repos, second sync returns 2 repos. Removed project's `installation_id` set to NULL. Historical data (`test_runs`, `quarantined_tests`) preserved.
- Installation suspended: mock returns installation with non-null `suspended_at` timestamp. `installations.suspended_at` column updated, artifact polling skipped for those repos.
- Installation removed: mock returns empty installations list (previously known installations are absent). Installation rows marked with non-null `removed_at` timestamp. Historical data preserved.
- API error during sync: mock returns 500. Sync logs warning, does not throw, existing `projects` table unchanged.
- JWT auth failure: mock returns 401 on token exchange. Sync logs warning, skips that installation.

**Installation Discovery Background Loop** (`dashboard/test/installation-loop.integration.test.ts`)
- Startup blocks serving: start server with mock GitHub API. Verify `installations` table is populated before the first HTTP response is served. (`GET /` immediately after start returns App-discovered repos.)
- Interval fires: use short interval (100ms) in test. Verify sync function called at least twice within 500ms via mock call counter.
- Shutdown cleans up: start server, trigger shutdown signal, verify sync function is NOT called after shutdown + one interval.
- First tick is delayed: start server with short interval. Verify sync runs once at startup, then next call happens after one interval (not immediately). At t=0 expect 1 call (startup), at t=interval expect 2 calls.
- Error doesn't kill the loop: mock returns 500 on first interval tick, 200 on second. Verify loop continues after error and eventually succeeds.

**One-time setup:** None. Mock servers and in-memory SQLite created per test.

### 5.3 Contract Tests

Prism-based, offline, no credentials.

**New endpoints to add to `schemas/github-api.json`:**
- `POST /app/installations/{installation_id}/access_tokens` (201, 401, 403, 404)
- `GET /app/installations` (200)
- `GET /installation/repositories` (200)

Extract these from the official GitHub OpenAPI spec at `github/rest-api-description` (bundled format, `descriptions/api.github.com/api.github.com.json`).

**JS Contract Tests** (`test/contract/github-app.test.js`)
- `POST /app/installations/{id}/access_tokens` -> 201 with `token`, `expires_at`, `permissions`
- Same with `Prefer: code=401` -> 401 (bad JWT)
- Same with `Prefer: code=403` -> 403 (App suspended or insufficient permissions)
- Same with `Prefer: code=404` -> 404 (installation not found)
- `GET /app/installations` -> 200 with array of installation objects
- `GET /installation/repositories` -> 200 with repository list
- Uses existing Prism test patterns from `test/contract/github-artifacts.test.js`

**OAuth token exchange is NOT contract-tested**: The endpoint (`github.com/login/oauth/access_token`) is on `github.com`, not `api.github.com`. Prism can't serve it from the REST API spec. Covered by `@remix-run/auth` library tests and dashboard integration tests.

**One-time setup:**
- Add 3 new endpoint definitions to `schemas/github-api.json` from official GitHub OpenAPI spec
- No changes to `scripts/run-contract-tests.sh` (existing Prism startup handles new paths)

### 5.4 E2E Tests

Real GitHub API, real App.

**Test fixtures:**
- `mycargus/quarantine-test-fixture` — existing PAT-based E2E tests (unchanged, `e2e` CI job)
- `mycargus/quarantine-app-test-fixture` — App-based E2E tests (new, `e2e-app` CI job)

**App E2E Test Scenarios** (`test/e2e/github-app-dashboard.test.js`, runs in `e2e-app` job)
- **Real installation token exchange:** Dashboard generates JWT from dev App credentials, calls `POST /app/installations/{id}/access_tokens`, receives valid token. Verify token works for `GET /repos/{owner}/{repo}`.
- **Real installation discovery:** Dashboard calls `GET /app/installations` and `GET /installation/{id}/repositories` with real credentials. Verify fixture repo (`quarantine-app-test-fixture`) appears in results.
- **Real artifact polling with App token:** Dashboard uses installation token to list and download artifacts from fixture repo. Verify same behavior as existing PAT-based E2E tests.

**CLI E2E with App token** (runs in `e2e-app` job)
- Existing CLI E2E test suite runs against `mycargus/quarantine-app-test-fixture` with `QUARANTINE_GITHUB_TOKEN` set to a token generated by `actions/create-github-app-token`. Validates the recommended CI integration path (ADR-026) end-to-end.

**Browser E2E — OAuth login** (`test/e2e-browser/dashboard-oauth.test.ts`, runs in `e2e-app` job)
- Playwright navigates to `/auth/login`, completes GitHub OAuth with a dedicated test account (App pre-authorized, TOTP 2FA), and verifies session cookie + authenticated page load + logout. Uses `@playwright/test`.

**Existing CLI E2E:** Unchanged. PAT-based tests continue running in the existing `e2e` CI job against `mycargus/quarantine-test-fixture`.

**One-time setup steps:**

1. **Register dev GitHub App** (under `mycargus` account):
   - Permissions: `contents:read+write`, `issues:read+write`, `pull_requests:write`, `actions:read`
   - Webhooks: **disabled** (uncheck "Active")
   - Token expiration: **enabled** (Settings > Optional features > User-to-server token expiration — required for 8-hour session alignment)
   - Generate private key, download PEM file
   - Note the App ID, Client ID, Client Secret

2. **Create and configure `mycargus/quarantine-app-test-fixture`:**
   - Create the repo under `mycargus`
   - Set up `quarantine.yml`, `quarantine/state` branch with empty `quarantine.json`
   - Add GitHub Actions workflows that upload `quarantine-results-{run_id}` artifacts (matching the existing fixture pattern)
   - Verify artifacts are uploaded and accessible

3. **Install dev App on fixture repo:**
   - Go to the dev App's installation page
   - Install on `mycargus` account, select `quarantine-app-test-fixture`
   - Note the installation ID (from URL or API: `GET /repos/mycargus/quarantine-app-test-fixture/installation`)

4. **Create dedicated GitHub test account for Playwright OAuth E2E:**
   - Create a GitHub account (e.g., `quarantine-test-bot`)
   - Enable TOTP 2FA and note the TOTP secret
   - Authorize the dev App on this account (visit the App's authorization URL, click Authorize)
   - Grant the test account read access to `quarantine-app-test-fixture`

5. **Configure CI secrets** (on the quarantine repo, Settings > Secrets and variables > Actions):

   | Type | Name | Value |
   |------|------|-------|
   | Secret | `QUARANTINE_APP_PRIVATE_KEY` | PEM file contents |
   | Secret | `QUARANTINE_APP_CLIENT_SECRET` | Dev App's client secret |
   | Secret | `QUARANTINE_E2E_GITHUB_USERNAME` | Test account username |
   | Secret | `QUARANTINE_E2E_GITHUB_PASSWORD` | Test account password |
   | Secret | `QUARANTINE_E2E_GITHUB_TOTP_SECRET` | Test account TOTP secret (base32) |
   | Variable | `QUARANTINE_APP_ID` | Dev App's numeric App ID (for `actions/create-github-app-token`) |
   | Variable | `QUARANTINE_APP_CLIENT_ID` | Dev App's client ID (for dashboard JWT `iss`) |
   | Variable | `QUARANTINE_APP_INSTALLATION_ID` | Installation ID on fixture |
   | Variable | `QUARANTINE_APP_ORIGIN` | `http://localhost:3000` (for E2E dashboard tests) |
   | Variable | `QUARANTINE_GH_APP_TEST_OWNER` | `mycargus` |
   | Variable | `QUARANTINE_GH_APP_TEST_REPO` | `quarantine-app-test-fixture` |

6. **CI workflow:** Add a separate `e2e-app` job in `.github/workflows/ci.yml` with its own concurrency group (`e2e-app-quarantine-app-test-fixture`). Passes App credentials and test account credentials as env vars. Installs Playwright browsers. Uses `CI` environment for secret access. Gates execution to trusted branches only. The existing `e2e` job is unchanged.

7. **Production App:** Registered separately when feature ships. Own private key, credentials. Never shares credentials with dev App.

---

## 6. Key Integration Points (Existing Code)

These are the critical files where changes will land. **No CLI files are modified.**

| File | Current Role | Change |
|------|-------------|--------|
| `dashboard/app/server.ts` | HTTP server with fetch-router | Add startup installation sync (blocking), background discovery loop, shutdown hook. |
| `dashboard/app/routes.ts` | Two routes (`/`, `/projects/:owner/:repo`) | Add `/auth/login`, `/auth/github/callback`, `/auth/logout`, `/health`. |
| `dashboard/app/lib/github.server.ts` | Reads PAT from env for artifact polling | Support installation token generation when `source: github-app`. |
| `dashboard/app/lib/db.server.ts` | SQLite schema with 3 tables | Add `installations` table, add `installation_id` column to `projects`. |
| `dashboard/app/lib/config.server.ts` | Validates `dashboard.yml` with `source: manual` | Add `source: github-app` validation (no `repos` required). |
| `dashboard/dashboard.yml` | `source: manual` with explicit repo list | Add `source: github-app` mode. |
| `schemas/github-api.json` | Vendored OpenAPI spec for contract tests | Add App auth and installation endpoints. |

**New dashboard files:**

| File | Purpose |
|------|---------|
| `dashboard/app/lib/github-client.server.ts` | GitHub HTTP client wrapper: rate limit monitoring, `parseLinkHeader` pagination helper, `fetchAllPages` utility |
| `dashboard/app/lib/jwt.server.ts` | Pure function: `generateJWT(clientID, privateKeyPEM, now)` |
| `dashboard/app/lib/installation-token.server.ts` | `InstallationTokenProvider`: JWT exchange, caching, refresh |
| `dashboard/app/lib/installation-sync.server.ts` | `syncInstallations()`: discovery via `GET /app/installations` + repos |
| `dashboard/app/lib/auth.server.ts` | `@remix-run/auth` configuration, GitHub provider, cookie session + SQLite token management |
| `dashboard/app/lib/permissions.server.ts` | `filterProjectsByUserAccess()`: pure function for user permission filtering |

**New repo-root files:**

| File | Purpose |
|------|---------|
| `test/e2e-browser/dashboard-oauth.test.ts` | Playwright browser E2E: OAuth login flow against real GitHub |
| `test/e2e-browser/playwright.config.ts` | Playwright configuration for browser E2E tests |

---

## 7. Open Questions

1. **App name availability:** Need to check if "quarantine" is available as a GitHub App name (globally unique, max 34 chars). Alternatives: "quarantine-ci", "quarantine-app", "quarantine-flaky".
2. **Private key in CI:** GitHub Actions secrets have a 48KB size limit. RSA private keys (2048-bit) are ~1.7KB, well within limits. But the multiline PEM format may need encoding (base64 the whole file, decode at runtime in the dashboard).
3. ~~**remix-auth Remix 3 compatibility:**~~ **Resolved by ADR-029.** Community `remix-auth` v4 is incompatible with Remix 3. Remix 3 shipped first-party `@remix-run/auth` v0.1.0 (2026-03-25) with `createGitHubAuthProvider`. ADR-029 adopts the first-party packages.
