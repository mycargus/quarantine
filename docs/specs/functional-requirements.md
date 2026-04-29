# Functional Requirements

## Product Summary

Quarantine automatically detects, disables (quarantines), and tracks flaky
(non-deterministic) tests in CI pipelines. It wraps the user's test command,
detects flaky tests by re-running failures, quarantines them so builds pass,
creates GitHub issues, and provides a dashboard for visibility.

---

## 1.1 Flaky Test Detection

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.1.1 | CLI wraps the user's existing test command (from the suite's `command` field in config) via `quarantine run [suite-name]`. The command is executed via `exec.Command` unmodified — quarantine never appends flags. | [v1 M9] |
| FR-1.1.2 | Parses JUnit XML output to identify test failures. Supports a glob pattern for multiple XML files (e.g., `junitxml: "results/*.xml"`); the CLI merges all matching files before processing. | [v1] |
| FR-1.1.3 | Re-runs individual failing tests N times (configurable per suite, default 3). | [v1] |
| FR-1.1.4 | If a test passes on any retry, it is flagged as flaky. | [v1] |
| FR-1.1.5 | Framework-agnostic: any test runner that produces JUnit XML works without quarantine code changes (ADR-030). No auto-detection of framework at runtime. | [v1 M9] |
| FR-1.1.6 | `quarantine init` detects test frameworks (jest/vitest from `package.json`, rspec from `Gemfile`) and pre-fills suite entries in config with `junitxml` and `rerun_command` defaults. Detection is advisory — never fatal, never gates functionality. | [v1 M9] |
| FR-1.1.7 | `rerun_command` is a required per-suite field (YAML array). Placeholder substitution (`{name}`, `{classname}`, `{file}`) is performed within array elements before `exec.Command` is called. No shell involved. | [v1 M9] |
| FR-1.1.8 | When the test command exits non-zero and no JUnit XML is found, quarantine classifies this as a command crash (not test failures) and exits 2 with a diagnostic. | [v1 M9] |
| FR-1.1.9 | When `rerun_command` fails for a specific test, that test is classified as `"unresolved"` (not flaky, not genuine failure). The run continues processing other tests. Exit code 2 (unless genuine failures exist, then exit 1). | [v1 M9] |
| FR-1.1.10 | `quarantine status [suite-name]` shows quarantined test count, estimated CI time (from `duration_ms` in recent artifacts), and oldest quarantined tests with issue links and age. | [v1 M10] |

## 1.2 Quarantine Management

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.2.1 | Maintains per-suite state files at `.quarantine/<suite-name>/state.json` on the dedicated `quarantine/state` branch (ADR-032). | [v1 M9] |
| FR-1.2.2 | Quarantined test failures are recognized by checking the suite's state file; their failures are ignored in exit code determination. Quarantined tests still execute (exclusion deferred to v2). | [v1 M9] |
| FR-1.2.3 | Build exits 0 if only quarantined tests failed; exits 1 for genuine (non-quarantined) failures; exits 2 for quarantine infrastructure errors (unresolved tests, crashes, config errors). | [v1 M9] |
| FR-1.2.4 | On each run, batch-checks GitHub issue status for quarantined tests; removes from in-memory state any test whose issue is now closed. | [v1] |
| FR-1.2.5 | Uses optimistic concurrency for per-suite state file updates (SHA-based compare-and-swap via GitHub Contents API, with retry on conflict). | [v1] |
| FR-1.2.6 | Writes `.quarantine/<suite>/quarantined-files.txt` (deduplicated file paths of quarantined tests) before executing the test command, as a composable building block for user-driven file-level exclusion. | [v1 M10] |

## 1.3 GitHub Integration

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.3.1 | Creates GitHub issues for newly detected flaky tests with label `quarantine:<suite-name>:<test_hash>`. One issue per flaky test per suite. | [v1 M9] |
| FR-1.3.2 | Posts or updates a per-suite PR comment identified by `<!-- quarantine:<suite-name> -->`. Each suite has its own independent comment. | [v1 M9] |
| FR-1.3.3 | Performs check-before-create for issues using suite-scoped dedup label (`quarantine:<suite-name>:<test_hash>`); no duplicate issues per suite. | [v1 M9] |
| FR-1.3.4 | Uses the GitHub Contents API for per-suite state file management at `.quarantine/<suite-name>/state.json`. | [v1 M9] |

## 1.4 Configuration

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.4.1 | Configuration is stored in `.quarantine/config.yml` at the repository root. Quarantine discovers the repo root via git; walking up from a subdirectory is supported. | [v1 M9] |
| FR-1.4.2 | Configurable options include: `test_suites` array (each suite: `name`, `command`, `junitxml`, `rerun_command` [all required], `retries`, `timeout`, `rerun_timeout` [optional]); shared `github`, `labels`, `notifications`, `storage` fields. Suite management via `quarantine suite list` and `quarantine suite remove`. | [v1 M9/M10] |
| FR-1.4.3 | `quarantine doctor` validates the full config: non-empty `test_suites`, suite name constraints (`[a-z0-9][a-z0-9-]*`, max 30), required fields, duration format; warns on detected framework-level retry config (Jest `retryTimes`, Vitest `retry`, RSpec `rspec-retry`). | [v1 M9] |
| FR-1.4.4 | `.quarantine/config.yml` is the sole source of truth for the GitHub target (`github.owner`, `github.repo`) at runtime. `run` and `doctor` never read the working tree's git remote URL. Missing or empty `github.owner`/`github.repo` is a config error (exit 2) in `init`, `run`, and `doctor`. `quarantine init` is a two-phase flow: phase 1 writes a partial config with an empty `github` block and exits 2; phase 2 (re-run after hand-edit) completes setup and exits 0. As a best-effort bootstrapping hint, phase 1 may inspect `git remote -v` and append non-authoritative YAML comments under the `github` block listing detected github.com candidates (`# origin -> owner/repo`); these comments do not participate in resolution and are ignored by phase 2 and all other commands. `quarantine doctor` verifies the configured target with a single `GET /repos/{owner}/{repo}` reachability call; it does not introspect token scopes or feature flags. | [v1 M20] |

## 1.5 Dashboard

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.5.1 | Web UI displays quarantined tests per project, trends over time, and test stability metrics. | [v1] |
| FR-1.5.2 | Pulls test results from GitHub Artifacts (not pushed by the CLI). | [v1] |
| FR-1.5.3 | Uses hybrid polling: background sync every 5 minutes plus on-demand sync when a user views a page. | [v1] |
| FR-1.5.4 | Uses SQLite for historical data storage. | [v1] |
| FR-1.5.5 | Provides cross-repo visibility (one dashboard instance for an entire org). | [v1] |
| FR-1.5.6 | Built with Remix 3 and TypeScript. Responsive layout via CSS (no build step required). | [v1] |

## 1.6 Degraded Mode

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.6.1 | CLI operates normally if the dashboard is unreachable; results are stored in GitHub Artifacts. | [v1] |
| FR-1.6.2 | CLI operates normally if the GitHub API is unreachable; falls back to cached `quarantine.json`. | [v1] |
| FR-1.6.3 | CLI must never fail the build due to Quarantine infrastructure issues. | [v1] |

## 1.7 Code Sync Adapter

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.7.1 | Automated PRs to add framework-specific skip markers in source code for quarantined tests. | [v2+] |
| FR-1.7.2 | Automated PRs to remove skip markers when the corresponding issue is closed. | [v2+] |
| FR-1.7.3 | Changes are batched: one PR per day with all quarantine changes, not one PR per test. | [v2+] |
| FR-1.7.4 | Optional feature, toggled per project in configuration. | [v2+] |

## 1.8 Monorepo Support

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.8.1 | v1 uses one `.quarantine/config.yml` per repository with a `test_suites` array. Multiple suites in one config provide namespace isolation between test runners in a monorepo context. | [v1 M9] |
| FR-1.8.2 | Each suite has its own state file at `.quarantine/<suite-name>/state.json`, providing per-namespace test ID scope. No global uniqueness assumption across suites. | [v1 M9] |
| FR-1.8.3 | Test IDs (`file_path::classname::name`) are unique within a suite's state file. Scoping at the state-file level allows the same test ID to exist in two suites independently. | [v1 M9] |

## 1.9 Expanded CI Support

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.9.1 | Support Jenkins, GitLab CI, and Bitbucket Pipelines. | [v2+] |
| FR-1.9.2 | CI-provider detection for appropriate artifact/cache storage backend. | [v2+] |

## 1.10 Expanded Ticket Tracker Support

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.10.1 | Jira integration for issue creation and lifecycle management. | [v2+] |
| FR-1.10.2 | Tracker is configurable in `quarantine.yml`. | [v2+] |

## 1.11 Authentication

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.11.1 | CLI authenticates via GitHub PAT for contents, issues, and PR comment operations. | [v1] |
| FR-1.11.2 | Dashboard uses a GitHub PAT to pull artifacts. | [v1] |
| FR-1.11.3 | No API keys required in v1 (CLI talks directly to GitHub, not the dashboard). | [v1] |
| FR-1.11.4 | GitHub App installed on org with short-lived tokens (1-hour expiry, client-side refresh) and fine-grained permissions. | [v2+] |
| FR-1.11.5 | GitHub OAuth for dashboard login via remix-auth. | [v2+] |
| FR-1.11.6 | Org-level integration: install once, all repos covered. | [v2+] |

## 1.13 GitHub App Registration

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.13.1 | Register the GitHub App with permissions: `contents:read+write`, `issues:read+write`, `pull_requests:write`, `actions:read`. | [v2] |
| FR-1.13.2 | Register the App with webhooks disabled. No webhook URL, no webhook secret. Webhooks deferred to v3 (ADR-027). | [v2] |
| FR-1.13.3 | Register an OAuth callback URL for dashboard login (e.g., `https://<dashboard-host>/auth/github/callback`). | [v2] |
| FR-1.13.4 | Store the App's private key (PEM, PKCS#1 RSAPrivateKey) in a secrets manager or CI secret. Never in source code, config files, or `quarantine.yml`. | [v2] |
| FR-1.13.5 | Register a separate dev App for development and testing with its own credentials and private key. | [v2] |

## 1.14 Dashboard App Auth (JWT + Installation Tokens)

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.14.1 | JWT generation is a pure function: `generateJWT(clientID, privateKeyPEM, now)`. Produces RS256 JWT with `iss` = client ID, `iat` = now-60s, `exp` = now+9min. | [v2] |
| FR-1.14.2 | Installation token exchange: `POST /app/installations/{id}/access_tokens` with JWT in `Authorization: Bearer` header. Returns token with `expires_at`. | [v2] |
| FR-1.14.3 | Installation token requests omit the `permissions` body parameter, inheriting the App's configured permissions. | [v2] |
| FR-1.14.4 | `InstallationTokenProvider` caches the installation token and refreshes when < 5 minutes remain before expiry. At most one token exchange per ~55 minutes per installation. | [v2] |
| FR-1.14.5 | If installation token exchange fails, dashboard logs a warning. Artifact polling for that installation's repos pauses until the next successful token exchange. Dashboard continues serving cached data. | [v2] |

## 1.15 Dashboard OAuth

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.15.1 | GitHub OAuth login uses Remix 3's first-party `@remix-run/auth` with `createGitHubAuthProvider` and `@remix-run/auth-middleware` for route protection (ADR-029). Application owns SQLite session storage and token refresh. | [v2] |
| FR-1.15.2 | Session lifetime matches GitHub access token lifetime (8 hours). No refresh tokens are stored or used. When the session cookie expires, the user re-authenticates via OAuth. This eliminates refresh token rotation complexity. | [v2] |
| FR-1.15.3 | Session: encrypted `httpOnly` `secure` `SameSite=Lax` cookie with `Max-Age: 28800` (8 hours). The encrypted cookie holds the access token and user profile via `createCookieSessionStorage`. No server-side session table. | [v2] |
| FR-1.15.4 | Dashboard uses user's access token to call `GET /user/installations`, then `GET /user/installations/{id}/repositories` per installation (all paginated calls use `per_page=100` and follow `Link` header `rel="next"` to fetch all pages), and filters projects to only repos the user can access. | [v2] |
| FR-1.15.5 | For server-side GitHub API calls (artifact polling, installation discovery), the dashboard generates installation tokens from the App's private key. Replaces the PAT from `QUARANTINE_GITHUB_TOKEN`. | [v2] |
| FR-1.15.6 | `dashboard.yml` gains `source: github-app` mode. Repo list populated from App installations. `source: manual` continues to work for backward compatibility. When `source: github-app`, the `repos` array is silently ignored if present. | [v2] |
| FR-1.15.7 | Unauthenticated access limited to `/auth/login`, `/auth/github/callback`, `/auth/logout`, `/health`. All other routes return 401. `/auth/logout` is public so that users with expired sessions can still trigger logout without receiving a 401. | [v2] |
| FR-1.15.8 | New routes: `/auth/login`, `/auth/github/callback`, `/auth/logout`, `/health`. | [v2] |

## 1.16 Installation Discovery

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.16.1 | SQLite `installations` table: `id` (GitHub installation ID, numeric), `account_login`, `account_type`, `suspended_at` (ISO 8601 timestamp or NULL; NULL = active, timestamp = suspended; GitHub API has no `status` field — suspension state is conveyed via `suspended_at`), `repository_selection` (all/selected), `created_at`, `updated_at`. Installations that no longer appear in `GET /app/installations` are marked with a non-NULL `removed_at` timestamp (deletion is inferred by sync diff, not an API field). | [v2] |
| FR-1.16.2 | Add nullable `installation_id` column to existing `projects` table (FK to `installations.id`). When `source: github-app`, projects have an `installation_id`. When `source: manual`, `installation_id` is NULL. | [v2] |
| FR-1.16.3 | When `source: github-app`, artifact polling uses projects discovered from installations instead of manual `repos` list. | [v2] |
| FR-1.16.4 | Startup sync: on startup, dashboard calls `GET /app/installations` then `GET /installation/repositories` per installation (all paginated calls use `per_page=100` and follow `Link` header `rel="next"` to fetch all pages), upserts into tables, blocks serving traffic until complete. | [v2] |
| FR-1.16.5 | Background discovery loop: re-sync installations every 15 minutes via `setInterval`. First tick fires 15 minutes after startup (not immediately). Each sync paginates all API calls. | [v2] |
| FR-1.16.6 | Shutdown: `process.on('SIGTERM')` and `process.on('SIGINT')` clear the interval. | [v2] |
| FR-1.16.7 | Error resilience: `syncInstallations()` catches all errors internally, logs them, never throws. Failed syncs leave existing `projects` table unchanged. | [v2] |
| FR-1.16.8 | Suspended installations: dashboard reads `suspended_at` from the GitHub API response (`null` = active, ISO 8601 timestamp = suspended) and stores it in the `installations.suspended_at` column. Installations that no longer appear in `GET /app/installations` are marked with a non-NULL `removed_at` timestamp. Artifact polling skips repos linked to suspended or removed installations. | [v2] |
| FR-1.16.9 | Use GitHub numeric `id` for installations, repos, and users. Never names or slugs. | [v2] |
| FR-1.16.10 | Repo removed from installation: project's `installation_id` set to NULL. Historical data (`test_runs`, `quarantined_tests`) preserved. | [v2] |

## 1.17 Branch Protection Bypass

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.17.1 | Branch protection bypass is a repo-admin action, not an App capability. The App's `contents:write` makes it eligible, but a repo admin must add it to the repository ruleset bypass list. | [v2] |
| FR-1.17.2 | Documentation instructs admins on adding the App to their ruleset bypass list. | [v2] |

## 1.12 Notifications

| ID | Requirement | Version |
|----|-------------|---------|
| FR-1.12.1 | PR comments posted on the triggering PR/commit. | [v1] |
| FR-1.12.2 | Slack integration for flaky test notifications. | [v2+] |
| FR-1.12.3 | Email notifications for flaky test events. | [v2+] |
| FR-1.12.4 | Configurable threshold alerts (e.g., "notify when >N tests quarantined"). | [v2+] |
| FR-1.12.5 | Periodic flaky test summary reports. | [v2+] |

---

## Constraints

| ID | Constraint |
|----|-----------|
| C-1 | v1 target customer is the user's employer (enterprise environment). |
| C-2 | v1 CI support: GitHub Actions only for artifact/cache features; the CLI itself runs in any CI. |
| C-3 | v1 ticket tracker: GitHub Issues only. |
| C-4 | Primary project goal: learning and resume portfolio. |
| C-5 | Secondary project goal: potential monetization. |
