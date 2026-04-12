# Non-Functional Requirements

## 2.1 Performance

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.1.1 | CLI overhead must be less than 5 seconds added to a test run, excluding retry time. | [v1] |
| NFR-2.1.2 | Dashboard page load must be under 2 seconds. | [v1] |
| NFR-2.1.3 | System must support 50+ concurrent CI builds without conflicts. Per-suite state files (ADR-032) eliminate CAS contention between parallel suite runs. | [v1 M9] |
| NFR-2.1.4 | Dashboard state enumeration cost scales linearly with suite count: 1 directory listing + N state file reads per poll cycle. For a repo with 5 suites on a 5-minute poll, this is approximately 72 API calls/hr — within `GITHUB_TOKEN` limits (1,000/hr). | [v1 M10] |

## 2.2 Reliability

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.2.1 | CLI must never break a build due to its own failure. Quarantine errors (config, API, crash, timeout, unresolved tests) produce exit 2, not exit 1. Exit 1 is reserved exclusively for confirmed genuine test failures. | [v1] |
| NFR-2.2.2 | Graceful degradation when GitHub API or dashboard is unreachable. | [v1] |
| NFR-2.2.3 | Result ingestion must be idempotent (duplicate artifact processing is safe). | [v1] |
| NFR-2.2.4 | `quarantine init` must be idempotent: re-running on an already-initialized repo skips existing artifacts without overwriting config or state. | [v1 M9] |
| NFR-2.2.5 | Per-suite state files are independent: a failure in one suite's CAS update must not affect other suites' state (ADR-032). | [v1 M9] |

## 2.3 Security

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.3.1 | Dashboard is internal-only (deployed behind corporate network). | [v1] |
| NFR-2.3.2 | Reverse proxy with rate limiting at 60 requests/min/IP. | [v1] |
| NFR-2.3.3 | Unauthenticated rate limit: 20 req/min/IP; authenticated rate limit: 300 req/min/user. Both layers use **fixed-window** counters (reset every 60 seconds from the start of each window). Respond with HTTP 429 and `Retry-After` header (seconds remaining until window reset) per RFC 6585 / RFC 9110. Implementation: two-layer custom middleware using the `remix/fetch-router` middleware system (the router provides middleware chaining; rate limiting logic is application code) — (1) IP-based limit before auth resolution, (2) user-based limit after `session()` + `auth()`. IP extraction: read client IP from a configurable header (default `X-Forwarded-For`, first entry) with fallback to socket remote address, to support deployment behind a reverse proxy (NFR-2.3.1). Rate limit counters accept an injectable clock for testability. | [v2+] |
| NFR-2.3.4 | Artifact polling is debounced (max 1 pull per repo per 5 minutes). | [v1] |
| NFR-2.3.5 | GitHub API circuit breaker triggers on consecutive failures. | [v1] |

## 2.4 Scalability

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.4.1 | SQLite WAL mode for dashboard concurrent reads. | [v1] |
| NFR-2.4.2 | Staggered artifact polling across repos to avoid thundering herd. | [v1] |
| NFR-2.4.3 | Adaptive polling: inactive repos are polled less frequently. | [v2+] |

## 2.5 Deployment

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.5.1 | CLI is a single Go binary, cross-compiled for linux/darwin on amd64 and arm64. | [v1] |
| NFR-2.5.2 | CLI distributed via GitHub Releases as a direct download. | [v1] |

## 2.6 Compatibility

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.6.1 | CI-provider agnostic: works in any CI environment that can run a binary. | [v1] |
| NFR-2.6.2 | Native GitHub Actions integration for artifacts and cache. | [v1] |
| NFR-2.6.3 | Supports JUnit XML output from any test framework. No framework field in config; quarantine is framework-agnostic (ADR-030). | [v1 M9] |
| NFR-2.6.4 | Suite `command` is a YAML array executed via `exec.Command` (no shell). Works on Linux and macOS (linux/darwin, amd64/arm64). Windows out of scope for v1. | [v1 M9] |
| NFR-2.6.5 | Suite names (`[a-z0-9][a-z0-9-]*`, max 30 chars) are safe for use as file system directory names, HTML comment markers, CLI arguments, and GitHub label components on all supported platforms. | [v1 M9] |

## 2.11 Timeout and Signal Handling

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.11.1 | Per-suite `timeout` (default 30m) enforced for the suite command: SIGTERM sent at timeout, SIGKILL after 5-second grace period. Partial JUnit XML processed if present at kill time. | [v1 M10] |
| NFR-2.11.2 | Per-suite `rerun_timeout` (default 5m) enforced for each individual rerun invocation: SIGKILL on timeout, test classified `"unresolved"`, run continues. | [v1 M10] |
| NFR-2.11.3 | Signal forwarding: SIGINT/SIGTERM forwarded directly to the child process via `cmd.Process.Signal(sig)`. No shell layer; no process group management required. | [v1 M9] |
| NFR-2.11.4 | All exit-2 error messages use a parseable prefix (`Error [config]:`, `Error [crash]:`, `Error [timeout]:`, `Error [rerun]:`) enabling log-based debugging without additional exit codes. | [v1 M10] |

## 2.7 App Auth Security

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.7.1 | Private key read from file path (`QUARANTINE_APP_PRIVATE_KEY_PATH`) or injected as env var value (`QUARANTINE_APP_PRIVATE_KEY`). Client ID from `QUARANTINE_APP_CLIENT_ID`. Dashboard-only in v2. Never in source code or config files. | [v2] |
| NFR-2.7.2 | JWT: `iat` = now minus 60s (clock skew tolerance), `exp` = max 10 minutes from `iat`, `alg` = RS256, `iss` = App client ID (recommended over App ID per GitHub docs). | [v2] |
| NFR-2.7.3 | OAuth handled by `@remix-run/auth` with `createGitHubAuthProvider` (ADR-029), which manages PKCE code challenge and code exchange. HTTPS redirect URIs enforced in production via `QUARANTINE_APP_ORIGIN`. | [v2] |
| NFR-2.7.4 | Session cookies: httpOnly, secure (HTTPS-only), SameSite=Lax, Max-Age=28800 (8 hours). Encrypted cookie holds access token and user profile via `createCookieSessionStorage`. No server-side session table. Session expires when the access token does — no refresh tokens stored. | [v2] |
| NFR-2.7.5 | Auth events logged with timestamps and user IDs. Token values never logged. | [v2] |

## 2.8 App Auth Performance

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.8.1 | Installation tokens cached, refreshed only when < 5 min remaining. Max one `POST /access_tokens` per ~55 min per installation. | [v2] |
| NFR-2.8.2 | JWT generation < 1ms overhead (pure CPU, RSA signing, no I/O). | [v2] |
| NFR-2.8.3 | Rate limit budget monitored via `X-RateLimit-Remaining`. Warning logged at < 20% remaining. | [v2] |

## 2.9 App Auth Reliability

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.9.1 | Missing/invalid App credentials on dashboard startup: fail with descriptive error. Artifact polling and installation discovery do not start. | [v2] |
| NFR-2.9.2 | Installation suspended (GitHub API returns non-null `suspended_at`): dashboard stores the timestamp in `installations.suspended_at` and pauses artifact polling for affected repos. Resumes on unsuspension (detected by next discovery poll when `suspended_at` returns to null). Installations that no longer appear in `GET /app/installations` are marked with a non-NULL `removed_at` timestamp; their repos are also skipped. | [v2] |
| NFR-2.9.3 | Mixed auth supported: some repos use PATs (via `source: manual`), others use App (via `source: github-app`). No global cutover required. | [v2] |
| NFR-2.9.4 | Discovery loop error isolation: a failed `syncInstallations()` call does not crash the process or affect in-flight HTTP requests. Existing project data remains available. | [v2] |

## 2.10 App Auth Observability

| ID | Requirement | Version |
|----|-------------|---------|
| NFR-2.10.1 | Dashboard logs installation token refresh events with timestamp and new expiry. | [v2] |
| NFR-2.10.2 | Dashboard logs installation discovery results: installations found, repos added/removed, errors. | [v2] |
