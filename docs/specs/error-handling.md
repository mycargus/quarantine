# Error Handling Strategy

> Last updated: 2026-03-17
>
> Pre-implementation task 7. Defines how the CLI and dashboard handle every
> category of error. The guiding principle: **never break the build due to
> Quarantine's own failure.**

## Guiding Principles

1. **The CLI must NEVER exit non-zero due to its own infrastructure failure if
   the test suite itself passed.** The exit code always reflects test results,
   not quarantine infrastructure status.
2. **All quarantine infrastructure errors are logged as warnings, never fatal**
   (unless `--strict` is set).
3. **The dashboard failing has zero impact on CI.** The CLI never talks to the
   dashboard. The dashboard discovers data autonomously from GitHub Artifacts.
4. **Prefer degraded mode over failure in every case.** Running all tests
   without quarantine state is better than refusing to run.

## Exit Codes

| Code | Meaning | When |
|------|---------|------|
| 0 | Success | Tests passed. Includes degraded mode where tests passed. |
| 1 | Test failure | Real, non-flaky test failures exist after retries. |
| 2 | Quarantine error | Not initialized, invalid command/flags, `--strict` infrastructure failure, `doctor` failure, `init` failure. |

**Exit code 1 exclusively means "your tests failed."** There is no ambiguity.

**Cobra usage error override:** Cobra defaults to printing usage and returning
an error, which `main()` translates to exit 2. This is correct for our
semantics: usage errors (unknown flags, missing arguments) are quarantine errors,
not test failures. Override via `cmd.SetFlagErrorFunc()` to print a clean error
message and exit 2, suppressing cobra's default behavior of printing the full
help text on every flag typo.

**Per-command exit codes:**

| Command | Exit 0 | Exit 1 | Exit 2 |
|---------|--------|--------|--------|
| `quarantine run` | Tests passed (including degraded mode) | Real test failures | Not initialized; `--strict` infrastructure failure |
| `quarantine init` | Initialization succeeded | -- | Initialization failed (bad token, no repo access, branch creation failed) |
| `quarantine doctor` | Config and state are valid | -- | Validation failed (missing config, bad YAML, unreachable state branch) |
| `quarantine version` | Always | -- | -- |

---

## Category 1: GitHub API Errors (CLI)

These errors occur when the CLI interacts with the GitHub Contents API
(quarantine.json), Issues API, Search API, or PR Comments API.

### Default behavior (no `--strict`)

All GitHub API errors during `quarantine run` trigger degraded mode. The CLI
logs warnings and continues with whatever state it has (cached or none).

### Error-by-error behavior

#### 401 Unauthorized

**Cause:** Invalid token, expired token, revoked token.

**Behavior:**
- Log: `[quarantine] WARNING: GitHub API returned 401 (unauthorized). Check QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.`
- Enter degraded mode (no quarantine state, all tests run).
- Do not retry. A 401 will not resolve by retrying.

**`init` context:** Exit 2 with actionable message: token is required for
initialization.

#### 403 Forbidden

**Cause:** Token lacks required scopes (needs `repo`), SSO enforcement, IP
allowlist restriction, or secondary rate limit (abuse detection).

**Behavior:**
- Check `Retry-After` header. If present, this is a secondary rate limit --
  treat as 429 (see below).
- Otherwise: log warning with the `X-GitHub-Request-Id` header for debugging.
- Enter degraded mode.

**`init` context:** Exit 2 with message suggesting the token needs `repo` scope.

#### 404 Not Found

**Cause:** Branch does not exist, repository does not exist, or token lacks
visibility to the repo.

**Behavior depends on command context:**

| Context | Behavior |
|---------|----------|
| `init` -- state branch missing | Expected. Create `quarantine/state` branch with empty `quarantine.json`. This is the normal init flow. |
| `run` -- state branch missing | **Not initialized.** Log: `Quarantine is not initialized for this repository. Run 'quarantine init' first.` Exit 2. |
| `run` -- quarantine.json missing on existing branch | Treat as empty quarantine state. Log warning. Proceed normally (all tests run, flaky detection still works). |
| `run` -- repo not found | Log warning. Enter degraded mode (token may lack visibility). |

#### 409 Conflict (CAS failure on quarantine.json)

**Cause:** Another CI build updated quarantine.json between our read and write
(optimistic concurrency via SHA-based compare-and-swap).

**Behavior:**
1. Re-read quarantine.json from the branch (get new SHA and content).
2. Merge: union of both test sets. If the same test_id exists in both, keep
   the entry with the later `last_flaky_at` timestamp. **Quarantine wins on
   conflict** -- if one side has a test quarantined and the other removed it,
   keep it quarantined (ADR-012).
3. Retry the write with the new SHA.
4. Maximum 3 retry attempts.
5. If all 3 retries fail: log warning, skip the write. The quarantine state
   update is lost for this run but will be re-detected on the next run.
   This is safe because the test is still flaky and will be caught again.

**Implementation notes:**
- The merge is a pure function: `merge(local_state, remote_state) -> merged_state`.
- Unit-test the merge function extensively -- it is the core of concurrency safety.
- Log each retry attempt: `[quarantine] WARNING: CAS conflict on quarantine.json (attempt 2/3), re-reading and merging.`

#### 422 Unprocessable Entity

**Cause:** Malformed request body, invalid ref name, content too large (>1 MB
for Contents API).

**Behavior:**
- Log the full error response body at `--verbose` level.
- Log warning: `[quarantine] WARNING: GitHub API rejected request (422). This may indicate a bug in quarantine.`
- Enter degraded mode for reads. Skip the write for writes.
- **Content too large (quarantine.json >1 MB):** This indicates an abnormally
  large quarantine list (~2,500+ tests). Log a specific warning suggesting
  the user review quarantined tests and close resolved issues.

#### 429 Rate Limited

**Cause:** Primary rate limit (5,000 req/hr for PAT, 1,000 for GITHUB_TOKEN)
or secondary rate limit (abuse detection).

**Behavior:**
1. Read `Retry-After` header (seconds) or `X-RateLimit-Reset` header (Unix
   timestamp).
2. If `Retry-After` is present: wait that many seconds, then retry once.
3. If `X-RateLimit-Reset` is present and the wait is <30 seconds: wait, retry.
4. If the wait is >30 seconds or headers are missing: log warning, enter
   degraded mode. Do not block CI for more than 30 seconds waiting on rate
   limits.
5. Log: `[quarantine] WARNING: GitHub API rate limited. Remaining: {X-RateLimit-Remaining}. Resets at: {X-RateLimit-Reset}.`

**Proactive rate limit awareness:** After every GitHub API response, check
`X-RateLimit-Remaining`. If below 50, log a warning:
`[quarantine] WARNING: GitHub API rate limit low ({remaining} remaining, resets at {reset}).`
This gives users early notice before hitting the limit.

#### 500, 502, 503 Server Error

**Cause:** GitHub is experiencing issues.

**Behavior:**
1. Wait 2 seconds.
2. Retry once.
3. If the retry also fails: log warning, enter degraded mode.
4. Do not retry more than once -- GitHub outages tend to last minutes, not
   seconds. Blocking CI longer is worse than degraded mode.

#### Timeout (network)

**Cause:** DNS failure, network partition, GitHub unreachable, corporate proxy
issues.

**Behavior:**
- HTTP client timeout: 10 seconds per request.
- On timeout: retry once after 2 seconds.
- If retry also times out: log warning, enter degraded mode.
- Log: `[quarantine] WARNING: GitHub API request timed out. Running in degraded mode.`

### GitHub API error summary table

| HTTP Status | Retry? | Max Wait | Degraded Mode? | Exit Code |
|-------------|--------|----------|----------------|-----------|
| 401 | No | -- | Yes | 0 (if tests pass) |
| 403 | Only if Retry-After | 30s | Yes | 0 (if tests pass) |
| 404 (`init`) | No | -- | Create branch | 0 or 2 |
| 404 (`run`, no branch) | No | -- | No | 2 (not initialized) |
| 404 (`run`, no file) | No | -- | Yes (empty state) | 0 (if tests pass) |
| 409 | Yes (re-read + merge) | 3 attempts | Skip write after 3 | 0 (if tests pass) |
| 422 | No | -- | Yes | 0 (if tests pass) |
| 429 | Yes (once) | 30s | Yes | 0 (if tests pass) |
| 500/502/503 | Yes (once) | 2s | Yes | 0 (if tests pass) |
| Timeout | Yes (once) | 2s | Yes | 0 (if tests pass) |

---

## Category 2: Test Runner Errors (CLI)

These errors occur when the CLI executes the user's test command and attempts
to parse the results. Test runner errors are fundamentally different from
infrastructure errors: they reflect problems with the user's test suite or
environment, not with quarantine.

### Error-by-error behavior

#### Test command not found

**Cause:** The command in `quarantine run -- <cmd>` is not on `$PATH`, is
misspelled, or the binary does not exist.

**Detection:** `exec.Command` returns `exec.ErrNotFound` (or the OS returns
exit code 127 on Unix).

**Behavior:**
- Exit 2.
- Print: `Error: command not found: "<cmd>". Ensure the test runner is installed and on your PATH.`
- This is a quarantine error (not a test failure) because no tests ran.

#### Test command exits non-zero (normal test failure)

**Cause:** Tests failed. This is the expected, common case.

**Behavior:**
- Parse JUnit XML output.
- Proceed with normal flaky detection, quarantine, retry logic.
- Exit code is determined by test results after quarantine processing:
  - Exit 0 if all failures were quarantined or identified as flaky.
  - Exit 1 if real (non-flaky, non-quarantined) failures remain.

**Important:** A non-zero exit from the test runner does NOT mean quarantine
should exit non-zero. The XML results determine the exit code.

#### No XML produced (test runner crash)

**Cause:** The test runner crashed before writing output, segfault, OOM kill,
or the test command does not produce JUnit XML at all.

**Detection:** After the test command exits, glob for XML files at the
configured `junitxml` path. No files match.

**Behavior:**
- Log warning: `[quarantine] WARNING: No JUnit XML found at "{junitxml_glob}". Cannot determine test results.`
- If the test runner exited 0: exit 0 (tests apparently passed, just no XML).
- If the test runner exited non-zero: exit with the runner's exit code. The CLI
  cannot distinguish test failures from crashes without XML, so it passes
  through the runner's signal.
- Do not attempt flaky detection, quarantine, or state updates.

#### Malformed XML (single file)

**Cause:** Truncated output (runner killed mid-write), encoding issues, or a
non-JUnit XML file matching the glob.

**Detection:** XML parse error from `encoding/xml`.

**Behavior:**
- Log warning: `[quarantine] WARNING: Failed to parse JUnit XML at "{file}": {parse_error}. Skipping this file.`
- If this was the only XML file: treat as "no XML produced" (see above).
- If other valid XML files exist: proceed with the valid files (see "multiple
  XML" below).

#### XML not found at configured path

**Cause:** The `junitxml` glob in `quarantine.yml` does not match any files,
but the configured path looks intentional (not the default).

**Detection:** Glob returns zero matches.

**Behavior:**
- Log error with remediation: `[quarantine] WARNING: No files matched junitxml pattern "{pattern}". Check your quarantine.yml junitxml setting or pass --junitxml.`
- Same exit logic as "no XML produced."

**Distinction from "no XML produced":** The message is different (suggests
fixing the path), but the exit behavior is identical.

#### Multiple XML files, some malformed

**Cause:** Parallel test runners (Jest `--shard`, RSpec `parallel_tests`) each
write an XML file. One or more files are corrupt (e.g., a shard was killed).

**Behavior:**
- Parse all files. For each file that fails to parse, log a per-file warning.
- Proceed with the successfully parsed files.
- Log summary: `[quarantine] WARNING: Parsed {n}/{total} JUnit XML files. {failed} files were malformed and skipped.`
- Flaky detection and quarantine proceed on the partial results.
- Exit code is determined by the parsed results only.
- This is the correct behavior because partial results are better than no
  results, and the malformed files likely represent crashed shards whose
  failures would surface through other mechanisms.

### Test runner error summary table

| Error | Exit Code | Quarantine Processing? |
|-------|-----------|----------------------|
| Command not found | 2 | No |
| Non-zero exit + valid XML | 0 or 1 (from XML) | Yes (full) |
| Non-zero exit + no XML | Runner's exit code | No |
| Non-zero exit + malformed XML (only file) | Runner's exit code | No |
| Zero exit + no XML | 0 | No |
| Multiple XML, some malformed | 0 or 1 (from valid XML) | Yes (partial) |

---

## Category 3: Dashboard Errors

Dashboard errors have **zero impact on CI.** The dashboard is a non-critical
analytics component. It reads from GitHub Artifacts and has no communication
channel with the CLI.

### GitHub API errors (artifact polling)

**Cause:** Same HTTP errors as Category 1 (401, 403, 404, 429, 500, timeout).

**Behavior:**
- Log the error with context (repo name, endpoint, status code).
- Skip this poll cycle.
- Retry on the next scheduled poll cycle (default: 5 minutes).
- For 429: read `Retry-After` and delay the next poll for that repo accordingly.
- For persistent failures (e.g., 5 consecutive failures for the same repo):
  apply exponential backoff to that repo's poll interval, up to 1 hour.
  Reset backoff on first success.
- Do not crash the dashboard process. Other repos continue polling normally.

**Circuit breaker:** If all repos for an org fail consecutively (e.g., org-wide
token revocation), pause polling for that org. Log at error level. Resume
when an admin reconfigures the token.

### SQLite write failures

**Cause:** Disk full, file permission error, WAL corruption, concurrent write
contention beyond SQLite's limits.

**Behavior:**
- Log at error level with the SQLite error code and message.
- Return HTTP 500 to any web UI request that depends on the failed write.
- For ingestion writes: skip the artifact, log a warning. It will be retried
  on the next poll cycle (the dashboard tracks last-successfully-ingested
  artifact, not last-attempted).
- **Disk full:** Log a specific message: `Dashboard error: SQLite write failed (disk full). Free disk space on the volume mount.`
- **WAL corruption:** This is rare. Log at error level. The dashboard should
  have a documented recovery procedure (copy the DB, run `PRAGMA integrity_check`, rebuild if needed). This is an ops concern, not an automatic recovery.

### Artifact download failures

**Cause:** Network timeout, artifact deleted (past 90-day retention), GitHub
API error during download redirect.

**Behavior:**
- Log warning with the artifact ID and repo.
- Skip the artifact.
- Retry on the next poll cycle.
- For deleted artifacts (410 Gone or 404): mark as permanently skipped in the
  poll state so it is not retried indefinitely. Log:
  `Dashboard: artifact {id} for {repo} is no longer available (deleted or expired). Skipping permanently.`

### Malformed artifact JSON

**Cause:** Bug in CLI output, tampering, schema version mismatch, partially
uploaded artifact.

**Behavior:**
- Validate artifact JSON against `test-result.schema.json` before ingestion.
- If validation fails: log warning with the artifact ID, repo, and validation
  error. Skip the artifact. Mark as permanently skipped.
- Log: `Dashboard WARNING: artifact {id} for {repo} failed schema validation: {error}. Skipping.`
- Do not crash or halt ingestion of other artifacts.

### Dashboard error summary table

| Error | Impact on CI | Dashboard Behavior |
|-------|-------------|-------------------|
| GitHub API polling failure | None | Skip cycle, retry next cycle |
| GitHub API 429 | None | Respect Retry-After, delay repo poll |
| SQLite write failure | None | Return 500 to UI, retry ingestion next cycle |
| SQLite disk full | None | Log error, all writes fail until resolved |
| Artifact download failure | None | Skip, retry next cycle |
| Artifact deleted (404/410) | None | Mark permanently skipped |
| Malformed artifact JSON | None | Skip permanently, log warning |

---

## Degraded Mode

Degraded mode means the CLI runs tests without some or all quarantine
functionality. Tests still execute. Flaky detection via retry still works
(it does not require GitHub). Only quarantine state (exclusions) and
post-run actions (state updates, issues, PR comments) are affected.

### What works in degraded mode

| Feature | Requires GitHub? | Degraded behavior |
|---------|-----------------|-------------------|
| Run tests | No | Works normally |
| Parse JUnit XML | No | Works normally |
| Retry failing tests | No | Works normally |
| Detect flaky tests | No | Works normally |
| Exclude quarantined tests from execution | Yes (quarantine.json) | **All tests run** (no exclusions) |
| Update quarantine.json | Yes (Contents API) | Skipped. Flaky tests re-detected next successful run. |
| Create GitHub Issues | Yes (Issues API) | Skipped. Issues created next successful run. |
| Post PR comments | Yes (Issues API) | Skipped. |
| Check issue status (unquarantine) | Yes (Search API) | Skipped. Quarantine state unchanged. |
| Write results to disk | No | Works normally |

### No pending.json

There is no `pending.json` or local queue for deferred writes. This was
considered and removed from v1. Rationale:

- **Complexity:** Managing a local pending queue across CI runs (which are
  typically stateless) adds significant complexity for minimal benefit.
- **Re-detection is cheap:** If a flaky test is not written to quarantine.json
  this run, it will fail again on a future run, be retried, detected as flaky,
  and quarantined then. The cost is one extra "flaky failure detected" cycle.
- **Quarantined test exclusion:** Since quarantined tests are excluded from
  execution (not post-hoc XML rewriting), missing quarantine state means all
  tests run. This is safe -- the test is flaky, so it may or may not fail.
  If it fails, retry catches it. If it passes, no harm done.

### Degraded mode triggers

Degraded mode is entered when any of these occur during `quarantine run`:

1. GitHub API returns an error on quarantine.json read (401, 403, 404 for
   missing file, 422, 429, 5xx, timeout).
2. GitHub Actions cache miss (when API read already failed).
3. GitHub token is not set (`QUARANTINE_GITHUB_TOKEN` and `GITHUB_TOKEN` both
   empty).

### Degraded mode communication

Two mechanisms, both always active in v1:

#### 1. Stderr warning (always)

```
[quarantine] WARNING: running in degraded mode (GitHub API returned 401 unauthorized)
```

Format: `[quarantine] WARNING: running in degraded mode ({reason})`

- Printed to stderr so it does not interfere with test runner stdout.
- Printed once at the start of degraded mode entry, not repeated per skipped
  action.
- The reason is human-readable and specific (not a generic "infrastructure
  error").

#### 2. GitHub Actions annotation (when in GHA)

```
::warning title=Quarantine Degraded Mode::Running in degraded mode: GitHub API returned 401 unauthorized. Quarantine state unavailable; all tests will run.
```

- Emitted when the `GITHUB_ACTIONS` environment variable is set (value `true`).
- Appears as a yellow warning banner on the workflow run summary page.
- Includes the reason and a brief explanation of impact.

**Forward compatibility:** Stderr works in every CI environment. The GHA
`::warning` annotation is GitHub Actions-specific but harmless if printed in
other environments (it is just a text line on stderr). Future CI provider
support can add provider-specific annotations without changing the stderr
behavior.

### Degraded mode per-action behavior

When degraded mode is active, each post-test action is attempted independently
and skipped on failure:

```
1. Read quarantine.json     -> failed (triggered degraded mode)
2. Run tests                -> proceed (no exclusions)
3. Parse XML                -> proceed
4. Retry failures           -> proceed (flaky detection still works)
5. Update quarantine.json   -> skip (log warning)
6. Create GitHub Issues     -> skip (log warning)
7. Post PR comment          -> skip (log warning)
8. Check issue status       -> skip (log warning)
9. Write results to disk    -> proceed (local I/O, no GitHub dependency)
```

Each skipped action logs a specific warning:
```
[quarantine] WARNING: skipping quarantine state update (degraded mode)
[quarantine] WARNING: skipping GitHub Issue creation (degraded mode)
[quarantine] WARNING: skipping PR comment (degraded mode)
```

---

## `--strict` Mode

`--strict` changes the CLI's response to infrastructure errors from degraded
mode to exit 2.

### When to use

- **CI setup verification:** After running `quarantine init`, run
  `quarantine run --strict -- <test command>` once to confirm everything works
  end-to-end.
- **Debugging:** When quarantine is not behaving as expected and you want to
  see infrastructure errors surfaced as failures.
- **High-assurance environments:** Where running tests without quarantine state
  is unacceptable (rare -- most teams prefer degraded mode).

### Behavior changes with `--strict`

| Scenario | Default | `--strict` |
|----------|---------|------------|
| GitHub API error on read | Degraded mode, exit 0 (if tests pass) | Exit 2 |
| GitHub token not set | Degraded mode, exit 0 (if tests pass) | Exit 2 |
| CAS conflict after 3 retries | Skip write, exit 0 (if tests pass) | Exit 2 |
| GitHub API error on write | Skip write, exit 0 (if tests pass) | Exit 2 |
| GitHub API error on issue creation | Skip, exit 0 (if tests pass) | Exit 2 |
| Test command not found | Exit 2 | Exit 2 (same) |
| Real test failures | Exit 1 | Exit 1 (same) |
| No XML produced, runner exited non-zero | Runner's exit code | Runner's exit code (same) |

**What does NOT change with `--strict`:**
- Test runner errors (Category 2) are not affected. A missing test command is
  always exit 2. A test failure is always exit 1. `--strict` only affects
  quarantine infrastructure errors.
- Warning messages are still logged (stderr + GHA annotation). The difference
  is that the exit code changes from 0 to 2.

### `--strict` error message format

```
[quarantine] ERROR: infrastructure failure (--strict mode): GitHub API returned 401 unauthorized
[quarantine] ERROR: exiting with code 2. Remove --strict to run in degraded mode.
```

---

## Implementation Guidance

### Error type hierarchy (Go)

```go
// QuarantineError wraps all quarantine-specific errors with category
// and severity information for exit code determination.
type QuarantineError struct {
    Category    ErrorCategory  // GitHubAPI, TestRunner, Config, State
    Severity    ErrorSeverity  // Warning (degraded), Fatal (exit 2)
    StatusCode  int            // HTTP status code (0 if not HTTP)
    Retryable   bool
    Message     string
    Wrapped     error
}

type ErrorCategory int
const (
    ErrCategoryGitHubAPI ErrorCategory = iota
    ErrCategoryTestRunner
    ErrCategoryConfig
    ErrCategoryState
)

type ErrorSeverity int
const (
    ErrSeverityWarning ErrorSeverity = iota  // degraded mode
    ErrSeverityFatal                          // exit 2
)
```

### Exit code determination logic

The exit code is determined at the top level after all processing completes.
Individual functions return errors; they do not call `os.Exit()`.

```
function determineExitCode(testResult, infraErrors, strict):
    if testResult == CommandNotFound:
        return 2
    if testResult == NoXML and runnerExitCode != 0:
        return runnerExitCode
    if strict and len(infraErrors) > 0:
        return 2
    if testResult has real failures:
        return 1
    return 0
```

### Logging conventions

| Level | Prefix | When |
|-------|--------|------|
| Error | `[quarantine] ERROR:` | Fatal errors that cause exit 2 |
| Warning | `[quarantine] WARNING:` | Degraded mode, skipped actions, partial parse |
| Info | `[quarantine]` | Normal operation summaries (only with `--verbose`) |

All log output goes to stderr. Stdout is reserved for the test runner's output.

### HTTP client configuration

```go
httpClient := &http.Client{
    Timeout: 10 * time.Second,
}
```

- 10-second timeout per request.
- No global retry middleware -- retries are handled per-call based on the
  error category rules above.
- `User-Agent: quarantine-cli/{version}` header on all requests.

### Testing error handling

Every error path described in this document must have a corresponding test.
Use the golden fixture approach (pre-implementation task 10) for XML parse
errors. Use HTTP mocks (e.g., `net/http/httptest`) for GitHub API errors.

Key test scenarios:
- 409 conflict: simulate concurrent CAS failure and verify merge logic.
- 429 rate limit: verify Retry-After parsing and backoff.
- Degraded mode: verify exit 0 when tests pass despite infrastructure failure.
- `--strict`: verify exit 2 on the same infrastructure failure.
- Partial XML parse: verify that valid files are processed when some are
  malformed.
- Command not found: verify exit 2 with diagnostic message.

---

## Cross-References

- **ADR-003:** Quarantine mechanism (test exclusion, not XML rewriting).
- **ADR-006:** State storage (quarantine/state branch, Contents API).
- **ADR-012:** Concurrency strategy (SHA-based CAS, quarantine wins on conflict).
- **ADR-015:** Rate limiting and backoff.
- `docs/planning/architecture.md` section 7.4: Degraded mode overview.
