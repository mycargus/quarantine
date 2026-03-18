# GitHub API Edge Cases

### Scenario 59: Search API result limit exceeded during unquarantine detection [M4]

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
