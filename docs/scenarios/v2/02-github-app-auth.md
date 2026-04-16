# GitHub App Auth Scenarios

Scenarios for GitHub App authentication, installation discovery, OAuth login,
and dashboard `source: github-app` mode. Drives M12 through M16.

For general v2+ scenarios, see [01-v2-scenarios.md](01-v2-scenarios.md).

---

## JWT Generation (M12)

### Scenario 12: Dashboard generates a valid JWT for App authentication [v2]

**Risk:** An incorrectly formed JWT causes silent 401 failures on every installation token exchange, blocking all App-based artifact polling.

**Given** the dashboard has a valid App client ID and RSA private key (PEM format)
**When** `generateJWT(clientID, privateKeyPEM, now)` is called
**Then** it returns an RS256 JWT with `iss` equal to the client ID, `iat` equal to now minus 60 seconds, and `exp` equal to now plus 9 minutes

### Scenario 13: JWT generation rejects invalid private key [v2]

**Risk:** A misconfigured private key (wrong format, public key, empty) produces a cryptic runtime error instead of a clear message, delaying diagnosis.

**Given** the dashboard has an App client ID
**When** `generateJWT` is called with an invalid PEM (e.g., a public key, a malformed string, or an empty string)
**Then** it throws a descriptive error identifying the problem (e.g., "Expected an RSA private key, got a public key")

---

## Installation Token Exchange (M12)

### Scenario 14: Dashboard exchanges JWT for an installation token [v2]

**Risk:** A failed token exchange silently degrades to no-auth API calls, which hit the 60 req/hr unauthenticated limit and cause cascading failures.

**Given** the dashboard has generated a valid JWT and knows the installation ID
**When** it calls `POST /app/installations/{id}/access_tokens` with the JWT as a Bearer token
**Then** it receives an installation token with an `expires_at` timestamp, and the request body does not include a `permissions` field

### Scenario 15: Installation token is cached and reused [v2]

**Risk:** Generating a new token on every API call wastes rate limit budget and adds latency.

**Given** the dashboard has a cached installation token that expires in 30 minutes
**When** a second API call needs an installation token
**Then** the cached token is returned without making another `POST /access_tokens` request

### Scenario 16: Installation token is refreshed before expiry [v2]

**Risk:** An expired token causes a burst of 401 errors until the next refresh cycle.

**Given** the dashboard has a cached installation token that expires in 4 minutes (less than the 5-minute refresh threshold)
**When** an API call needs an installation token
**Then** the provider generates a new JWT, exchanges it for a fresh token, caches the new token, and returns it

### Scenario 17: Installation token exchange fails gracefully [v2]

**Risk:** A network error during token exchange crashes the dashboard or blocks all artifact polling permanently.

**Given** the dashboard attempts to exchange a JWT for an installation token
**When** the `POST /access_tokens` call fails (network error, 500, etc.)
**Then** the provider logs a warning with the error details, returns an error to the caller, and does not throw an unhandled exception

---

## OAuth Login (M13)

### Scenario 18: Unauthenticated user is redirected to login [v2]

**Risk:** Dashboard pages are accessible without authentication, exposing test result data to unauthorized users.

**Given** a user has no active session
**When** they request `GET /` (or any protected route)
**Then** the dashboard returns 401

### Scenario 19: Login redirects to GitHub OAuth [v2]

**Risk:** A broken OAuth redirect silently fails, leaving users unable to log in.

**Given** a user navigates to `/auth/login`
**When** the dashboard processes the request
**Then** it redirects to GitHub's OAuth authorization URL with the correct client ID and callback URL

### Scenario 20: Health endpoint is always accessible [v2]

**Risk:** Monitoring probes fail when they hit the auth wall, causing false alerts.

**Given** no authentication is provided
**When** a request is made to `GET /health`
**Then** the dashboard returns 200

### Scenario 21: Logout invalidates the session [v2]

**Risk:** A logged-out user's session remains valid, allowing continued access after they intended to leave.

**Given** a user has an active authenticated session
**When** they request `GET /auth/logout`
**Then** the session cookie is cleared (there is no server-side session table — session state lives entirely in the encrypted cookie), and a subsequent request to a protected route returns 401

### Scenario 22: Session cookie uses secure attributes [v2]

**Risk:** Session cookies without httpOnly/secure/SameSite flags are vulnerable to XSS theft, MITM interception, and CSRF attacks.

**Given** a user successfully completes OAuth login
**When** the dashboard sets the session cookie
**Then** the cookie has `httpOnly`, `secure`, `SameSite=Lax`, and `Max-Age=28800` attributes, and the encrypted cookie payload contains the access token and user profile (no server-side session table)

### ~~Scenario 23: Expired access token is refreshed automatically [v2]~~ REMOVED

Removed: no refresh tokens. Session cookie expires after 8 hours (matching access token lifetime). User re-authenticates via OAuth.

### ~~Scenario 24: Failed token refresh invalidates the session [v2]~~ REMOVED

Removed: no refresh tokens. Session expiry is handled by cookie `Max-Age`.

---

## Installation Discovery (M14)

### Scenario 25: Startup sync discovers installations before serving traffic [v2]

**Risk:** The dashboard serves an empty project list on first page load because installation discovery hasn't completed yet.

**Given** the dashboard is configured with `source: github-app` and valid App credentials
**When** the dashboard process starts
**Then** it calls `GET /app/installations`, then `GET /installation/repositories` per installation, populates the `installations` and `projects` tables, and only then begins serving HTTP traffic

### Scenario 26: Background loop re-syncs installations periodically [v2]

**Risk:** New repos added to the App installation are not discovered until the dashboard restarts.

**Given** the dashboard has completed startup sync
**When** 15 minutes have elapsed since the last sync
**Then** `syncInstallations()` runs again, discovering any newly added or removed repositories

### Scenario 27: Shutdown stops the discovery loop cleanly [v2]

**Risk:** The discovery loop fires during shutdown, causing errors or data corruption.

**Given** the dashboard is running with an active discovery loop
**When** the process receives SIGTERM or SIGINT
**Then** the interval is cleared and no further sync calls are made

### Scenario 28: Discovery sync failure does not crash the dashboard [v2]

**Risk:** A transient GitHub API error during discovery kills the process, taking down the entire dashboard.

**Given** the background discovery loop is running
**When** `GET /app/installations` returns a 500 error
**Then** `syncInstallations()` logs the error, does not throw, leaves the existing `projects` table unchanged, and the loop continues on the next interval

### Scenario 29: Suspended installation pauses artifact polling [v2]

**Risk:** The dashboard wastes rate limit budget polling repos for a suspended App installation that will always return 403.

**Given** a discovery sync returns an installation with a non-null `suspended_at` timestamp (the GitHub API has no `status` field — suspension is conveyed via `suspended_at`)
**When** the next artifact polling cycle runs
**Then** repos linked to the suspended installation are skipped; repos linked to active installations (where `suspended_at` is null) continue polling normally

### Scenario 30: Repo removed from installation preserves historical data [v2]

**Risk:** Removing a repo from the App installation deletes all historical test run data, losing months of trend information.

**Given** the dashboard has 3 projects linked to an installation, with historical test runs and quarantined test data
**When** a discovery sync returns only 2 repos (one was removed from the installation)
**Then** the removed project's `installation_id` is set to NULL, but its `test_runs` and `quarantined_tests` data is preserved in SQLite

### Scenario 31: Config validation accepts source: github-app [v2]

**Risk:** A typo in `dashboard.yml` causes the dashboard to start in manual mode without warning, ignoring the App installation entirely.

**Given** `dashboard.yml` contains `source: github-app`
**When** the config is parsed and validated
**Then** validation passes without requiring a `repos` array; if `repos` is present, it is silently ignored

### Scenario 32: Missing App credentials at startup fails fast [v2]

**Risk:** The dashboard starts without App credentials and silently fails on every API call, producing confusing error logs instead of a clear startup error.

**Given** `dashboard.yml` has `source: github-app` but `QUARANTINE_APP_CLIENT_ID` or `QUARANTINE_APP_PRIVATE_KEY` is not set
**When** the dashboard process starts
**Then** startup fails immediately with a descriptive error message identifying the missing credential; the HTTP server does not start

---

## github-app Mode Integration (M15)

### Scenario 33: Artifact polling uses installation tokens [v2]

**Risk:** Artifact polling falls back to the v1 PAT path even when the App is configured, wasting the higher rate limits that App tokens provide.

**Given** the dashboard is configured with `source: github-app` and installation discovery has populated the `projects` table
**When** the artifact polling cycle runs for an App-discovered project
**Then** the dashboard uses an installation token (from `InstallationTokenProvider`) for the GitHub API calls, not a PAT

### Scenario 34: User sees only repos they have access to [v2]

**Risk:** A logged-in user sees repositories they don't have permission to access, exposing data across team boundaries within the org.

**Given** the App is installed on an org with 10 repos, and a logged-in user has access to 3 of those repos
**When** the user views the dashboard home page
**Then** the dashboard calls `GET /user/installations` then `GET /user/installations/{id}/repositories` per installation with the user's access token, intersects with App-discovered projects, and displays only the 3 repos the user can access

### Scenario 35: User with no accessible repos sees an empty list [v2]

**Risk:** A user with no repo access gets an error page instead of an empty state, suggesting the dashboard is broken.

**Given** the App is installed on an org, and a logged-in user has access to zero repos in the installation
**When** the user views the dashboard home page
**Then** the dashboard displays an empty project list, not an error

### Scenario 36: Manual-mode repos continue using PATs [v2]

**Risk:** Migrating to `source: github-app` breaks artifact polling for repos that were configured manually with PATs.

**Given** the dashboard runs with `source: github-app` for org-installed repos, and some projects were previously configured with `source: manual` (PAT-based)
**When** artifact polling runs
**Then** App-discovered repos use installation tokens; manually-configured repos continue using the PAT from `QUARANTINE_GITHUB_TOKEN`

---

## E2E Integration + CI (M16)

### Scenario 38: Real installation token exchange succeeds end-to-end [v2]

**Risk:** Interface tests with mocks pass but the real GitHub App API rejects our JWT format, token exchange, or API calls.

**Given** the CI environment has real dev App credentials (private key, client ID, installation ID)
**When** the E2E test generates a JWT, exchanges it for an installation token, and uses the token to call `GET /repos/{owner}/{repo}`
**Then** all API calls succeed with real GitHub responses; the fixture repo is accessible via the App token

### Scenario 39: Real installation discovery returns fixture repo [v2]

**Risk:** The discovery logic works against mocks but fails against real GitHub pagination, response shapes, or permission boundaries.

**Given** the dev App is installed on the `mycargus/quarantine-app-test-fixture` repo
**When** the E2E test calls `GET /app/installations` and `GET /installation/repositories` with real credentials
**Then** the fixture repo appears in the discovered repo list

### Scenario 40: Artifact polling with installation token succeeds end-to-end [v2]

**Risk:** Artifact polling works with PATs but fails with App installation tokens due to different permission resolution, token format, or API behavior.

**Given** the dev App is installed on the fixture repo and the repo has uploaded quarantine result artifacts
**When** the E2E test uses an installation token (from `InstallationTokenProvider`) to list and download artifacts from the fixture repo
**Then** artifact listing and download succeed identically to existing PAT-based E2E polling tests

---

## Rate Limiting (M13)

### Scenario 41: Rate limit exceeded for unauthenticated request [v2]

**Risk:** Without clear rate limit feedback, automated tools or attackers could hammer the dashboard with no backoff signal.

**Given** an unauthenticated client has exceeded 20 requests in the current 1-minute window
**When** the client sends another request to any endpoint (including `/health`)
**Then** the dashboard returns HTTP 429 with a `Retry-After` header indicating seconds until the window resets

### Scenario 42: Rate limit exceeded for authenticated request [v2]

**Risk:** A legitimate user running a script or rapidly navigating is silently throttled with a generic error instead of a standard rate limit response.

**Given** an authenticated user has exceeded 300 requests in the current 1-minute window
**When** the user sends another request to a protected route
**Then** the dashboard returns HTTP 429 with a `Retry-After` header indicating seconds until the window resets

---

## Startup Sync Timeout (M14)

### Scenario 43: Startup sync timeout causes fast failure [v2]

**Risk:** A slow or unresponsive GitHub API blocks the dashboard from starting indefinitely, with no error message or timeout.

**Given** the dashboard is configured with `source: github-app` and valid App credentials
**When** `syncInstallations()` does not complete within 60 seconds (e.g., GitHub API is unresponsive)
**Then** the dashboard logs a descriptive timeout error and exits with a non-zero code; the HTTP server does not start

---

## Concurrent Token Exchange (M12)

### Scenario 44: Concurrent callers coalesce into a single token exchange [v2]

**Risk:** Two simultaneous callers requesting an installation token when the cache is empty or expired trigger two parallel `POST /access_tokens` calls, wasting rate limit budget and potentially causing race conditions in the token cache.

**Given** the `InstallationTokenProvider` has no cached token (or the cached token has expired)
**When** two callers request `getToken(installationId)` simultaneously
**Then** exactly one `POST /app/installations/{id}/access_tokens` request is made (verified via mock server call counter); both callers receive the same token; the token is cached for subsequent calls

---

## Rate Limit Monitoring (M12)

### Scenario 50: GitHub HTTP client logs warning when rate limit drops below 20% [v2]

**Risk:** Without monitoring, the dashboard silently exhausts its rate limit budget and starts getting 403s across all API calls.

**Given** a GitHub API response includes `X-RateLimit-Remaining: 150` and `X-RateLimit-Limit: 1000` (15% remaining)
**When** the GitHub HTTP client processes the response
**Then** a warning is logged including the remaining count, limit, and resource

### Scenario 51: No rate limit warning when above 20% threshold [v2]

**Risk:** False-positive warnings cause alert fatigue and mask real rate limit issues.

**Given** a GitHub API response includes `X-RateLimit-Remaining: 800` and `X-RateLimit-Limit: 1000` (80% remaining)
**When** the GitHub HTTP client processes the response
**Then** no rate limit warning is logged

### Scenario 52: Missing rate limit headers handled gracefully [v2]

**Risk:** Some GitHub endpoints omit rate limit headers; crashing on missing headers breaks the entire polling loop.

**Given** a GitHub API response does not include `X-RateLimit-Remaining` or `X-RateLimit-Limit` headers
**When** the GitHub HTTP client processes the response
**Then** no error occurs and no warning is logged

---

## JWT Edge Cases (M12)

### Scenario 53: JWT generation rejects empty client ID [v2]

**Risk:** An empty client ID produces a JWT with an empty `iss` claim that GitHub silently rejects with 401, making the root cause hard to diagnose.

**Given** `generateJWT` is called with an empty string as the client ID and a valid private key
**When** the function validates its inputs
**Then** it throws a descriptive error identifying the problem (e.g., "Client ID must not be empty")

---

## Private Key Resolution (M12)

### Scenario 54: Private key resolved from environment variable value [v2]

**Risk:** If env var resolution is broken, the dashboard can't authenticate even when credentials are correctly configured.

**Given** `QUARANTINE_APP_PRIVATE_KEY` is set to a valid PEM-encoded RSA private key and `QUARANTINE_APP_PRIVATE_KEY_PATH` is not set
**When** the private key is resolved for JWT generation
**Then** the PEM value from `QUARANTINE_APP_PRIVATE_KEY` is used

### Scenario 55: Private key resolved from file path [v2]

**Risk:** File-based key resolution is the preferred production pattern (mounted secrets); a broken file reader silently falls through.

**Given** `QUARANTINE_APP_PRIVATE_KEY_PATH` is set to a path containing a valid PEM file and `QUARANTINE_APP_PRIVATE_KEY` is not set
**When** the private key is resolved for JWT generation
**Then** the PEM content is read from the file at the specified path

### Scenario 56: Private key file path does not exist [v2]

**Risk:** A misconfigured path causes a cryptic JWT error later instead of a clear error at initialization.

**Given** `QUARANTINE_APP_PRIVATE_KEY_PATH` is set to `/nonexistent/key.pem`
**When** the private key resolution runs
**Then** it throws a descriptive error identifying the missing file path

### Scenario 57: Both key sources set — file path takes precedence [v2]

**Risk:** Ambiguous precedence causes the wrong key to be used, leading to silent 401 failures that are hard to diagnose because "the key is set."

**Given** both `QUARANTINE_APP_PRIVATE_KEY` and `QUARANTINE_APP_PRIVATE_KEY_PATH` are set to valid but different keys
**When** the private key is resolved for JWT generation
**Then** the file at `QUARANTINE_APP_PRIVATE_KEY_PATH` is read and used (file path takes precedence, matching the Kubernetes mounted-secret pattern)

---

## Concurrent Token Exchange Edge Cases (M12)

### Scenario 58: Concurrent token exchange failure returns error to all waiters [v2]

**Risk:** If the coalesced exchange fails, only the first caller gets the error; the second hangs indefinitely.

**Given** the `InstallationTokenProvider` has no cached token and two callers request `getToken(installationId)` simultaneously
**When** the single `POST /access_tokens` exchange fails (e.g., 500)
**Then** both callers receive the same error; neither hangs or returns stale data

### Scenario 59: Concurrent exchanges for different installations run independently [v2]

**Risk:** A global mutex blocks all installations when only one is exchanging, adding unnecessary latency.

**Given** the `InstallationTokenProvider` has no cached tokens for installation A or installation B
**When** caller 1 requests `getToken(installationA)` and caller 2 requests `getToken(installationB)` simultaneously
**Then** two independent `POST /access_tokens` calls are made (one per installation); neither blocks the other

---

## ~~Concurrent Token Refresh (M13)~~ REMOVED

### ~~Scenario 45: Concurrent requests coalesce into a single token refresh [v2]~~ REMOVED

Removed: no refresh tokens. Session cookie expires after 8 hours. No concurrent refresh race condition possible.

---

## Removed Installation Detection (M14)

### Scenario 46: Uninstalled App is detected and marked as removed [v2]

**Risk:** An App uninstalled from an org continues to appear as active in the dashboard, causing failed API calls and confusing operators.

**Given** the dashboard has 2 installations in the `installations` table from a previous sync
**When** a discovery sync calls `GET /app/installations` and only 1 installation is returned (the other was uninstalled)
**Then** the missing installation is marked with a non-NULL `removed_at` timestamp, its repos are skipped during artifact polling, and historical test data is preserved

---

## Pagination (M14, M15)

### Scenario 47: Installation discovery paginates through all installations and repos [v2]

**Risk:** An org with more than 100 repos across its installations only sees the first page, silently excluding repos from discovery and artifact polling.

**Given** the App is installed on an org with 150 repos across 2 installations (one with 110 repos, one with 40)
**When** `syncInstallations()` runs
**Then** it fetches all pages of `GET /app/installations` and `GET /installation/repositories` (using `per_page=100` and following `Link` header `rel="next"`), and all 150 repos appear in the `projects` table

### Scenario 48: User permission filtering paginates through all accessible repos [v2]

**Risk:** A user with access to more than 100 repos only sees the first page of results, causing repos to disappear from their dashboard view.

**Given** a logged-in user has access to 120 repos across 2 installations
**When** the dashboard filters projects by user access
**Then** it fetches all pages of `GET /user/installations` and `GET /user/installations/{id}/repositories` (using `per_page=100` and following `Link` header `rel="next"`), and all 120 accessible repos are included in the filtered project list

---

## Expired Session Logout (M13)

### Scenario 49: Logout with expired or absent session does not error [v2]

**Risk:** A user whose session has expired clicks "logout" and receives a confusing 401 error instead of being redirected to the login page.

**Given** a user has no active session (cookie expired or never set)
**When** they request `GET /auth/logout`
**Then** the dashboard redirects to `/auth/login` without returning an error (clearing the cookie is a no-op when the session is already expired)

---

## OAuth Callback (M13)

### Scenario 60: OAuth callback completes login and sets session cookie [v2]

**Risk:** A broken callback handler leaves users stranded after authorizing on GitHub — they are redirected to the callback URL but receive no session, forcing them to repeat the OAuth flow indefinitely.

**Given** a user has authorized the Quarantine App on GitHub and GitHub has redirected them to `/auth/github/callback` with a valid authorization code and CSRF state parameter
**When** the dashboard processes the `GET /auth/github/callback?code={code}&state={state}` request
**Then** `@remix-run/auth` validates the state parameter and PKCE challenge, exchanges the code for a GitHub access token, stores the access token and user profile in an encrypted session cookie with `httpOnly`, `secure`, `SameSite=Lax`, and `Max-Age=28800` attributes, and responds with HTTP 302 redirecting the user to `/`

---

### Scenario 61: Authenticated user accesses protected route successfully [v2]

**Risk:** The `requireAuth()` middleware incorrectly challenges valid sessions, causing a login loop for users who are already authenticated.

**Given** a user has a valid, non-expired encrypted session cookie containing their access token and user profile
**When** they request `GET /` (or any protected route)
**Then** the dashboard returns HTTP 200

---

### Scenario 64: OAuth callback with invalid authorization code returns an error [v2]

**Risk:** An invalid or expired authorization code (e.g., replayed redirect, stale tab) silently succeeds or panics, creating a security vulnerability or an unhandled exception.

**Given** GitHub has redirected the user to `/auth/github/callback` but the authorization code is invalid or has already been consumed (GitHub returns an error on the token exchange)
**When** the dashboard processes the callback request
**Then** the dashboard responds with an HTTP error status (4xx) and does NOT set a session cookie

---

## Auth Event Logging (M13)

### Scenario 62: Login event is logged with timestamp and user ID [v2]

**Risk:** Without auth audit logs, security incidents (account takeover, unexpected logins) are invisible to operators with no way to reconstruct who logged in and when.

**Given** a user successfully completes OAuth login via `/auth/github/callback`
**When** the dashboard stores the session
**Then** a log entry is written that includes the event type, an ISO 8601 timestamp, and the user's GitHub user ID; the access token value (matching the `ghu_` prefix format) does NOT appear anywhere in the log output

---

### Scenario 63: Logout event is logged with timestamp and user ID [v2]

**Risk:** Logout events are not audited, making it impossible to reconstruct session termination during a security incident response.

**Given** a user with an active authenticated session requests `GET /auth/logout`
**When** the dashboard clears the session cookie
**Then** a log entry is written that includes the event type, an ISO 8601 timestamp, and the user's GitHub user ID; the access token value does NOT appear anywhere in the log output

---

### Scenario 65: Token values never appear in any auth-related log output [v2]

**Risk:** Auth middleware or session serialization code inadvertently logs the session object or request headers verbatim, exposing bearer tokens to anyone with log read access.

**Given** the dashboard processes any auth-related operation (login callback, logout, session validation on a protected route)
**When** the operation writes to the application log
**Then** the log output does not contain any substring matching a GitHub user-to-server token format (the `ghu_` prefix); this holds for both success paths and error paths

---

## OAuth Configuration (M13)

### Scenario 66: Dashboard fails fast when OAuth environment variables are missing [v2]

**Risk:** The dashboard starts without required OAuth credentials and silently fails on every login attempt with cryptic internal errors, rather than giving operators an immediate, actionable startup message.

**Given** any one of `QUARANTINE_APP_CLIENT_ID`, `QUARANTINE_APP_CLIENT_SECRET`, or `QUARANTINE_APP_ORIGIN` is not set in the environment
**When** the dashboard process starts
**Then** startup fails immediately with a descriptive error message identifying which variable is missing; the HTTP server does not start

---

## Rate Limit Window Reset (M13)

### Scenario 67: Rate limit counter resets after the window expires [v2]

**Risk:** A fixed-window counter that never resets permanently blocks clients after a burst, making the dashboard unusable beyond the first rate-limited minute.

**Given** an unauthenticated client has exceeded 20 requests in the current 1-minute window (receiving HTTP 429 responses), and the injectable clock is advanced past the 60-second window boundary
**When** the client sends a new request
**Then** the request is processed normally (HTTP status is not 429) because the fixed-window counter has reset for the new window
