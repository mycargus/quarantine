# Degraded Mode

### Scenario 30: CI run when GitHub API is unreachable [M4]

**Given** the CLI is configured in CI and the GitHub API is unreachable (network
failure, rate limit exceeded, or API outage)

**When** CI executes
`quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`

**Then** the CLI:
1. Attempts to fetch `quarantine.json` from the `quarantine/state` branch.
   Fails (timeout or HTTP error).
2. Falls back to a cached copy of `quarantine.json` from the GitHub Actions
   cache (key: `quarantine-state-latest`), if available.
3. If cache hit: uses cached state for quarantine exclusions (may be stale).
   If cache miss: runs all tests without exclusions.
4. Runs the test suite. Retries failing tests per `--retries 3`.
5. Does NOT attempt to update `quarantine.json`, create issues, or post PR
   comments.
6. Logs to stderr: `[quarantine] WARNING: running in degraded mode (GitHub API
   returned 503). Quarantine state unavailable.`
7. When `GITHUB_ACTIONS` env is set, emits:
   `::warning title=Quarantine Degraded Mode::GitHub API returned 503. Running
   in degraded mode.`
8. Writes results to `.quarantine/results.json`.
9. Exits based on test results only. Flaky tests detected during retry are
   still forgiven (exit 0 if no genuine failures).

Any flaky tests detected during the degraded run will be re-detected and
quarantined on the next successful run when GitHub API connectivity is restored.

---

### Scenario 31: CI run when dashboard is unreachable [M4]

**Given** the CLI is configured in CI, the GitHub API is reachable, but the
dashboard is unreachable

**When** CI executes `quarantine run` and detects a flaky test

**Then** the CLI operates normally: updates `quarantine.json`, creates GitHub
Issues, posts PR comments, and writes results to disk. The dashboard being
unreachable has no effect on the CLI's behavior because the dashboard pulls
data from GitHub Artifacts independently — the CLI never communicates with the
dashboard (per ADR-011). Exits with code 0.

---

### Scenario 32: Dashboard reconnects and syncs missed results from artifacts [M6]

**Given** the dashboard was unreachable for 2 hours, during which 5 CI runs
completed and uploaded results as GitHub Artifacts

**When** the dashboard comes back online and its background polling cycle
triggers (every 5 minutes, staggered per repo)

**Then** the dashboard queries the GitHub Artifacts API for all artifacts since
its last successful sync (tracked via `last_synced` timestamp per repo),
downloads and ingests the 5 missed result sets in chronological order, validates
each against `schemas/test-result.schema.json`, upserts into SQLite (keyed by
`run_id` for idempotency), and displays accurate, up-to-date information
without manual intervention.

---

### Scenario 33: CI run with no API access and empty cache [M4]

**Given** the CLI is configured in CI, the `quarantine/state` branch exists and
contains a `quarantine.json` with 4 previously quarantined tests, the GitHub
Actions cache is empty (cache expired or manually cleared), and the GitHub API
is completely unreachable

**When** CI executes
`quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`

**Then** the CLI:
1. Attempts to fetch `quarantine.json` from the branch — fails.
2. Attempts to load cached copy from Actions cache — cache miss.
3. Logs: `[quarantine] WARNING: Unable to reach GitHub API and no cached
   quarantine state available. Running without quarantine exclusions.`
4. Runs the full test suite without excluding any quarantined tests. All 4
   previously-quarantined tests now run.
5. Retries any failing tests per `--retries 3`.
6. Writes results to `.quarantine/results.json`.
7. Logs warnings that quarantine state could not be read or updated.
8. Exits based on test results. Flaky failures are still forgiven via retry.

Any flaky tests detected will be re-detected and quarantined on the next run
when connectivity is restored.

---

### Scenario 34: Degraded mode with --strict [M4]

**Given** the CLI is configured in CI with `--strict` and the GitHub API is
unreachable

**When** CI executes `quarantine run --strict --retries 3 -- jest --ci ...`

**Then** the CLI attempts to fetch `quarantine.json`, fails, and instead of
entering degraded mode, prints:
```
[quarantine] ERROR: infrastructure failure (--strict mode): GitHub API returned 503.
[quarantine] ERROR: exiting with code 2. Remove --strict to run in degraded mode.
```
Exits with code 2 without running any tests. This is the intended behavior for
`--strict` — infrastructure errors are fatal rather than degraded (per
docs/error-handling.md). Useful for verifying setup after `quarantine init`.

---

### Scenario 35: CI run with no GitHub token set [M4]

**Given** the CLI is configured in CI but neither `QUARANTINE_GITHUB_TOKEN` nor
`GITHUB_TOKEN` is set in the environment. A valid `quarantine.yml` exists.

**When** CI executes `quarantine run -- jest --ci ...`

**Then** the CLI detects the missing token, logs:
`[quarantine] WARNING: No GitHub token found. Running in degraded mode.`
Runs the test suite without quarantine state. Retries still work. Exits based
on test results only.

With `--strict`, this would exit 2 instead.

---
