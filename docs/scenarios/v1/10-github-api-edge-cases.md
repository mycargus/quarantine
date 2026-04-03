# GitHub API Edge Cases

### Scenario 59: Search API result limit exceeded during unquarantine detection [M4]

**Risk:** The CLI fails or behaves unpredictably when the repository has more than 1,000 closed quarantine issues, instead of gracefully degrading per the quarantine-wins principle (ADR-012).

**Given** `quarantine.json` contains 5 currently quarantined tests, and the
repository has over 1,000 closed GitHub Issues with the `quarantine` label
accumulated over months of CI activity

**When** the CLI performs the batch issue status check via the GitHub Search
API, and the API returns `total_count: 1247` but caps retrievable results at
1,000 items (the Search API maximum)

**Then** the CLI paginates through all available results (up to 1,000 items at
100 per page = 10 pages), matches closed issue numbers against the
`issue_number` fields in `quarantine.json` entries, and unquarantines any tests
whose issues appear in the retrieved results.

If a quarantined test's closed issue falls outside the 1,000-result window, that
test remains quarantined for this run. The CLI logs:
`[quarantine] WARNING: GitHub Search API returned 1,000 of 1,247 closed
quarantine issues. Some closed issues may not be detected. Consider narrowing
the search with a date filter or manually closing stale quarantine issues.`

This is consistent with the quarantine-wins principle (ADR-012) — erring on the
side of keeping a test quarantined is safer than accidentally re-enabling a
flaky test. The missed unquarantine is non-critical: the test remains
quarantined until a subsequent run retrieves the closed issue.

---

### Scenario 60: Rate limit warning [M4]

**Risk:** Users exhaust their GitHub API rate limit without warning, causing unexpected degraded mode entries across all subsequent CI builds until the limit resets (ADR-015).

**Given** the CLI is running in CI and the GitHub API responds with rate limit
headers showing `X-RateLimit-Remaining: 47` out of `X-RateLimit-Limit: 1000`
(below 10% remaining)

**When** the CLI reads the rate limit headers after an API call

**Then** the CLI logs:
`[quarantine] WARNING: GitHub API rate limit low (47 remaining, resets at
14:30 UTC). Consider using a PAT for higher limits (5,000 req/hr vs
1,000 req/hr for GITHUB_TOKEN).`
The CLI continues operating normally — this is informational only.

---

### Scenario 61: Issues disabled on repository [M5]

**Risk:** The CLI crashes or exits 2 when GitHub Issues are disabled on the repository, breaking builds even though issue creation is non-critical.

**Given** the CLI detects a flaky test and attempts to create a GitHub Issue,
but GitHub Issues are disabled on the repository

**When** the CLI calls `POST /repos/{owner}/{repo}/issues` and receives a
410 Gone response

**Then** the CLI logs:
`[quarantine] WARNING: GitHub Issues are disabled on this repository. Skipping
issue creation for all flaky tests in this run.`
The CLI skips issue creation for ALL flaky tests (not just the current one).
The test is still added to `quarantine.json` (without `issue_number`). PR
comments and results are still written. Exits normally.

---

### Scenario 62: quarantine.json exceeds size limit [M4]

**Risk:** The CLI crashes or exits 2 when `quarantine.json` exceeds the 1 MB Contents API limit, breaking builds instead of gracefully skipping the write.

**Given** `quarantine.json` has grown large (approaching the 1 MB Contents API
limit) due to many quarantined tests

**When** the CLI attempts to write the updated file and receives a 422
Unprocessable Entity response

**Then** the CLI logs:
`[quarantine] WARNING: quarantine.json exceeds 1 MB (GitHub Contents API limit).
Review and close resolved quarantine issues to reduce size. Skipping state
update.`
The CLI does not crash or exit 2. It skips the write and proceeds with the rest
of the flow (issue creation, PR comment, results). Exits based on test results.

---

### Scenario 63: CAS conflict exhaustion (all 3 retries fail) [M4]

**Risk:** Exhausting CAS retries causes the CLI to exit 2, breaking builds during periods of high concurrent CI activity (ADR-012).

**Given** the CLI detects a flaky test and attempts to update `quarantine.json`,
but 3 other concurrent builds are also writing, and every CAS retry encounters
a 409 conflict

**When** all 3 CAS retry attempts fail (each time: re-read, merge, attempt
write, 409)

**Then** the CLI logs:
`[quarantine] WARNING: Failed to update quarantine.json after 3 CAS retries
(concurrent builds). The flaky test will be re-detected on the next run.`
The CLI does NOT exit 2. It proceeds with issue creation and PR comment (the
test was detected as flaky even if state wasn't persisted). Exits based on test
results.

---

### Scenario 64: Contents API base64 newline wrapping [M4]

**Risk:** The CLI fails to decode `quarantine.json` if GitHub changes its base64 encoding format, causing silent degraded mode or corrupt state.

**Given** `quarantine.json` exists on the `quarantine/state` branch with one
quarantined test entry

**When** the CLI reads it via `GET /repos/{owner}/{repo}/contents/quarantine.json`
and the Contents API returns the file content as base64 with `\n` inserted every
60 characters (GitHub's standard wrapping behavior)

**Then** the CLI strips all `\n` characters from the `content` field before
base64-decoding, successfully parses the resulting JSON into a valid `State`
struct, and the quarantined test entry is present in the state.

---

### Scenario 66: quarantine.json entry written before issue creation [M4]

**Risk:** The Go `State.Entry` struct uses `omitempty` on `IssueNumber` and `IssueURL`, but `quarantine-state.schema.json` marks both as `required`. When an entry is written to state before issue creation completes (e.g., issue creation fails or is skipped due to 410 Gone), the marshaled JSON omits these fields, producing output that violates the schema.

**Given** the CLI detects a flaky test and writes it to `quarantine.json`, but
issue creation has not yet completed (or was skipped because Issues are disabled
on the repo)

**When** the CLI marshals the state to JSON via `State.MarshalAt()`

**Then** the entry in `quarantine.json` either includes `issue_number` and
`issue_url` with valid values, or the schema and struct agree on how to
represent their absence (both treat them as optional, or both require them with
a sentinel value like `null`). The marshaled JSON conforms to
`quarantine-state.schema.json`.

---

### Scenario 65: Contents API branch-not-found vs file-not-found [M4]

**Risk:** The CLI misinterprets a missing branch as a missing file (or vice versa), leading to incorrect degraded mode behavior or failed init.

**Given** a repository where the `quarantine/state` branch does not exist

**When** the CLI calls `GET /repos/{owner}/{repo}/contents/quarantine.json?ref=quarantine/state`
and receives a 404 response with the message `"No commit found for the ref quarantine/state"`

**Then** the CLI identifies this as a missing branch (not a missing file on an
existing branch), logs a warning, and enters degraded mode — running all tests
without quarantine filtering.

**And given** a repository where the `quarantine/state` branch exists but
`quarantine.json` has not yet been created

**When** the CLI calls the same endpoint and receives a 404 response without the
`"No commit found for the ref"` message

**Then** the CLI identifies this as a missing file on an existing branch and
treats it as an empty quarantine state (no tests quarantined), not as a branch
error.

---

### Scenario 95: 401 Unauthorized enters degraded mode with actionable warning [M8]

**Risk:** A revoked or expired token silently breaks quarantine without identifying which token to check, causing confusing degraded-mode CI runs.

**Given** the CLI is configured in CI and the GitHub API returns 401 Unauthorized when the CLI reads `quarantine.json`

**When** the CLI executes `quarantine run -- jest --ci ...`

**Then** the CLI:
1. Does not retry (a 401 will not resolve by retrying).
2. Logs to stderr: `[quarantine] WARNING: GitHub API returned 401 (unauthorized). Check QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.`
3. Enters degraded mode: runs all tests without quarantine exclusions.
4. Skips state update, issue creation, and PR comment (each logs its own skipped warning).
5. Writes results to disk.
6. Exits based on test results only (exit 0 if tests pass).

When `GITHUB_ACTIONS=true`, also emits:
`::warning title=Quarantine Degraded Mode::GitHub API returned 401 unauthorized. Check QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.`

---

### Scenario 96: 403 Forbidden without Retry-After enters degraded mode [M8]

**Risk:** A token with insufficient scopes causes the CLI to crash instead of degrading, breaking builds when token permissions change.

**Given** the CLI is configured in CI and the GitHub API returns 403 Forbidden without a `Retry-After` header (e.g., token lacks `repo` scope), including an `X-GitHub-Request-Id` header in the response

**When** the CLI reads `quarantine.json`

**Then** the CLI:
1. Logs a warning including the request ID: `[quarantine] WARNING: GitHub API returned 403 (forbidden). Request ID: abc123def456. Ensure your token has 'repo' scope.`
2. Enters degraded mode.
3. Does not retry.
4. Exits based on test results only.

---

### Scenario 97: 403 Forbidden with Retry-After treated as secondary rate limit [M8]

**Risk:** GitHub's secondary rate limit (403 with `Retry-After`) is treated as a permanent permission failure instead of a temporary limit, causing unnecessary degraded mode.

**Given** the CLI is configured in CI and the GitHub API returns 403 Forbidden with a `Retry-After: 10` header

**When** the CLI reads `quarantine.json`

**Then** the CLI treats the response identically to a 429 with `Retry-After: 10`: waits 10 seconds and retries once. If the retry succeeds, operation continues normally. If the retry also fails, the CLI enters degraded mode with a warning.

---

### Scenario 98: 5xx Server Error retries once then enters degraded mode [M8]

**Risk:** A transient GitHub server error that resolves in seconds permanently blocks the run if not retried, while retrying indefinitely blocks CI for minutes.

**Given** the CLI is configured in CI and the GitHub API returns 500 (or 502 or 503) when the CLI reads `quarantine.json`

**When** the CLI handles the 5xx response

**Then** the CLI:
1. Waits 2 seconds.
2. Retries the request once.
3. If the retry succeeds: continues normally.
4. If the retry also returns 5xx: logs `[quarantine] WARNING: GitHub API server error (502). Running in degraded mode.` and enters degraded mode.

Does not retry more than once.

---

### Scenario 99: Network timeout retries once then enters degraded mode [M8]

**Risk:** A transient network timeout causes permanent degraded mode when a single retry would succeed, but a permanent outage blocks CI for more than the 10-second HTTP timeout.

**Given** the CLI is configured with a 10-second HTTP timeout and the GitHub API request times out

**When** the CLI attempts to read `quarantine.json`

**Then** the CLI:
1. Waits 2 seconds.
2. Retries the request once.
3. If the retry also times out: logs `[quarantine] WARNING: GitHub API request timed out. Running in degraded mode.` and enters degraded mode.
4. If the retry succeeds: continues normally.

Total maximum wait before degraded mode: 10s (initial timeout) + 2s (wait) + 10s (retry timeout) = 22 seconds.

---

### Scenario 100: 429 with short Retry-After waits and retries [M8]

**Risk:** A transient rate limit causes immediate degraded mode when waiting a few seconds would allow the request to succeed.

**Given** the CLI is configured in CI and the GitHub API returns 429 Too Many Requests with `Retry-After: 8` (and `X-RateLimit-Remaining: 0`)

**When** the CLI reads `quarantine.json`

**Then** the CLI:
1. Logs: `[quarantine] WARNING: GitHub API rate limited. Remaining: 0. Resets at: {time}.`
2. Reads the `Retry-After: 8` header and waits 8 seconds.
3. Retries the request once.
4. If the retry succeeds: continues normally (not degraded mode).
5. If the retry also fails: enters degraded mode.

---

### Scenario 101: 429 with long Retry-After enters degraded mode immediately [M8]

**Risk:** The CLI blocks CI for minutes waiting on a rate limit reset, causing builds to time out.

**Given** the CLI is configured in CI and the GitHub API returns 429 Too Many Requests with `Retry-After: 120`

**When** the CLI reads `quarantine.json`

**Then** the CLI determines the wait exceeds 30 seconds and does not wait. It immediately enters degraded mode and logs:
`[quarantine] WARNING: GitHub API rate limited (120s wait exceeds 30s threshold). Running in degraded mode.`

---
