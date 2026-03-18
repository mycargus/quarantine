# User Scenarios

This document describes user scenarios for Quarantine in Given-When-Then format. Each scenario is tagged `[v1]` (initial release) or `[v2+]` (future release).

---

## Core Flows

### Scenario 1: First-time setup [v1]

**Given** a developer has a project with an existing test suite (e.g., Jest) and a GitHub Actions CI pipeline, and Quarantine CLI is not yet installed
**When** the developer installs the CLI via `curl -sSL https://github.com/mycargus/quarantine/releases/latest/download/quarantine-linux-amd64 -o /usr/local/bin/quarantine && chmod +x /usr/local/bin/quarantine`, runs `quarantine init` which interactively prompts for framework, retries, and JUnit XML path, creating a `quarantine.yml` in the repo root with the following content:

```yaml
version: 1
framework: jest
retries: 3
issue_tracker: github
labels:
  - quarantine
notifications:
  github_pr_comment: true
```

and updates their CI workflow to replace `jest --ci --reporters=default --reporters=jest-junit` with `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`
**Then** `quarantine init` validates the GitHub token and repository permissions, creates the `quarantine/state` branch in the repository if it does not exist, writes an empty `quarantine.json` (`{ "version": 1, "tests": {} }`) to that branch via the GitHub Contents API, and the build proceeds normally on the next CI run

---

### Scenario 2: Normal CI run with no flaky tests [v1]

**Given** the CLI is configured in CI, `quarantine.json` on the `quarantine/state` branch contains zero quarantined tests, and all tests in the suite are deterministic
**When** the developer pushes a commit and CI executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`
**Then** the CLI runs the test suite once, all tests pass on the first attempt, no retries are triggered, no changes are made to `quarantine.json`, the test results are written to disk (`.quarantine/results.json`), no PR comment is posted (nothing to report), and the CI build exits with status code 0

---

### Scenario 3: CI run detects a new flaky test [v1]

**Given** the CLI is configured in CI, `quarantine.json` has no entry for the test `PaymentService > should handle charge timeout`, and this test is non-deterministic
**When** CI executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit` and `should handle charge timeout` fails on the first run but passes on retry 2 of 3
**Then** the CLI identifies `should handle charge timeout` as flaky, fetches the current `quarantine.json` from the `quarantine/state` branch (recording its SHA for optimistic concurrency), adds an entry for the test with timestamp and first-seen metadata, writes the updated `quarantine.json` back via the Contents API using compare-and-swap, creates a GitHub Issue titled "Flaky test: PaymentService > should handle charge timeout" with the `quarantine` label and a test-specific label (e.g., `test:should_handle_charge_timeout`), posts a PR comment summarizing the newly quarantined test, writes results to disk (`.quarantine/results.json`), and the CI build exits with status code 0 (pass, since the failure was flaky)

---

### Scenario 4: CI run with a previously quarantined test [v1]

**Given** `quarantine.json` on the `quarantine/state` branch contains an entry for `PaymentService > should handle charge timeout` with status `quarantined`, and the corresponding GitHub Issue is still open
**When** CI executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`
**Then** the CLI recognizes the test is in the quarantine list and excludes it from execution via framework-specific exclusion flags (the test does not run at all), posts a PR comment noting which tests were excluded from execution, writes results to disk (`.quarantine/results.json`), and the CI build exits with status code 0

---

### Scenario 5: quarantine init — required setup step [v1]

**Given** a developer has a project with a Jest test suite and a GitHub Actions CI pipeline, and Quarantine CLI is installed but `quarantine init` has not been run
**When** the developer runs `quarantine init` from the repo root
**Then** the CLI interactively prompts for framework (e.g., `jest`, `rspec`, `vitest`), retry count (default: 3), and JUnit XML output path; creates a `quarantine.yml` in the repo root with the chosen values (including `version: 1`, `issue_tracker: github`, `labels: [quarantine]`, `notifications: { github_pr_comment: true }`); validates the GitHub token (`QUARANTINE_GITHUB_TOKEN` or `GITHUB_TOKEN`) and tests repository permissions; creates the `quarantine/state` branch with an empty `quarantine.json` (`{ "version": 1, "tests": {} }`); and prints a summary of what was configured and next steps for CI integration

**When** the developer runs `quarantine run` without having run `quarantine init` first
**Then** the CLI prints an error: `"Quarantine is not initialized for this repository. Run 'quarantine init' first."` and exits with code 2

---

### Scenario 6: CI run with a real failure [v1]

**Given** the CLI is configured in CI, `quarantine.json` has no entry for `CheckoutService > should apply discount`, and this test has a genuine bug
**When** CI executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit` and `should apply discount` fails on all 3 retries
**Then** the CLI determines the test is a real (deterministic) failure, does NOT add it to `quarantine.json`, does NOT create a GitHub Issue, writes results to disk (`.quarantine/results.json`), posts a PR comment noting the hard failure, and the CI build exits with a non-zero status code (build fails)

---

### Scenario 7: Multiple flaky tests detected in a single run [v1]

**Given** the CLI is configured in CI with `--retries 3`, and `quarantine.json` has no entries for `SearchService > should fuzzy match` or `ApiService > should handle rate limit`
**When** CI executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit` and both `should fuzzy match` (fails run 1, passes run 2) and `should handle rate limit` (fails run 1, fails run 2, passes run 3) are detected as flaky
**Then** the CLI adds both tests to `quarantine.json` in a single write (atomic update via Contents API with SHA-based compare-and-swap), creates two separate GitHub Issues each with the `quarantine` label and their respective test-specific labels, posts a single PR comment summarizing both newly quarantined tests, writes results to disk (`.quarantine/results.json`), and the CI build exits with status code 0

---

### Scenario 8: Quarantined test's GitHub issue is closed [v1]

**Given** `quarantine.json` contains an entry for `PaymentService > should handle charge timeout`, and a GitHub Issue titled "Flaky test: PaymentService > should handle charge timeout" exists with the `quarantine` label
**When** a developer closes the GitHub Issue (indicating the flaky test has been fixed)
**Then** on the next CI run, the CLI performs a batch issue status check via the GitHub Search API (one call returns all closed quarantine issues), detects the issue is closed, removes the `should handle charge timeout` entry from `quarantine.json` via Contents API with compare-and-swap, and the test is no longer excluded from execution — it runs normally as part of the test suite, and if it fails, it fails the build like any other test

---

### Scenario 9: Concurrent CI builds detect the same flaky test simultaneously [v1]

**Given** the CLI is configured in CI, `quarantine.json` has no entry for `CacheService > should handle eviction`, and two CI builds (Build A and Build B) are running in parallel on different PRs
**When** both builds detect `should handle eviction` as flaky and both attempt to create a GitHub Issue for it
**Then** the first build to reach GitHub creates the Issue titled "Flaky test: CacheService > should handle eviction" with the `quarantine` label; the second build uses check-before-create (searches for an existing open issue with matching label/title) and finds the issue already exists, so it skips issue creation; both builds succeed without duplicate issues

---

### Scenario 10: Concurrent CI builds update quarantine.json simultaneously [v1]

**Given** the CLI is configured in CI, two CI builds (Build A and Build B) are running in parallel, both have fetched `quarantine.json` at SHA `abc123` from the `quarantine/state` branch
**When** Build A writes its update to `quarantine.json` first (new SHA `def456`), and then Build B attempts to write its update using the stale SHA `abc123`
**Then** Build B's write fails with a 409 Conflict from the GitHub Contents API because the SHA no longer matches, Build B re-fetches `quarantine.json` at the new SHA `def456`, merges its changes with Build A's changes, and retries the write with the updated SHA, resulting in a `quarantine.json` that contains both builds' updates without data loss

---

### Scenario 10b: Concurrent quarantine and unquarantine race [v1]

**Given** `quarantine.json` contains an entry for `CacheService > should handle eviction` with an open GitHub Issue, two CI builds (Build A and Build B) are running in parallel, and Build A detects a new flaky test `ApiService > should handle timeout` while Build B finds that the issue for `should handle eviction` has been closed and removes it from its local copy of `quarantine.json`
**When** Build A writes first (adding `should handle timeout`, keeping `should handle eviction`), and Build B attempts to write (removing `should handle eviction`) using a stale SHA, triggering a 409 Conflict
**Then** Build B re-reads `quarantine.json`, merges using the quarantine-wins (union) strategy, and the resulting `quarantine.json` contains both `should handle eviction` and `should handle timeout`; Build B logs: "Test 'CacheService > should handle eviction' was unquarantined (issue closed) but re-quarantined due to concurrent detection. It will be unquarantined on the next build."; on the very next CI run, the CLI checks the issue status, finds it closed, and removes `should handle eviction` from `quarantine.json` — the impact is one extra build cycle at most

---

## Degraded Mode

### Scenario 11: CI run when GitHub API is unreachable [v1]

**Given** the CLI is configured in CI and the GitHub API is unreachable (network failure, rate limit exceeded, or API outage)
**When** CI executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`
**Then** the CLI logs a warning: "Unable to reach GitHub API. Running in degraded mode.", falls back to a cached copy of `quarantine.json` from the GitHub Actions cache if available, runs the test suite without quarantine exclusions if no cache exists, still retries failing tests per `--retries 3`, does NOT attempt to update `quarantine.json` or create issues, writes results to disk (`.quarantine/results.json`), and exits with a status code based on the test results. Any flaky tests detected during the degraded run will be re-detected on the next successful run when GitHub API connectivity is restored

---

### Scenario 12: CI run when dashboard is unreachable [v1]

**Given** the CLI is configured in CI, the GitHub API is reachable, but the React Router v7 dashboard is unreachable
**When** CI executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit` and detects a flaky test
**Then** the CLI operates normally: updates `quarantine.json`, creates GitHub Issues, posts PR comments, and writes results to disk; the dashboard being unreachable has no effect on the CLI's behavior since the dashboard pulls data from GitHub Artifacts independently; the CI build exits with status code 0

---

### Scenario 13: Dashboard reconnects and syncs missed results from artifacts [v1]

**Given** the React Router v7 dashboard was unreachable for 2 hours, during which 5 CI runs completed and uploaded results as GitHub Artifacts
**When** the dashboard comes back online and its background polling cycle triggers (every 5 minutes)
**Then** the dashboard queries the GitHub Artifacts API for all runs since its last successful sync, downloads and ingests the 5 missed result sets in chronological order, updates its internal view of flaky test trends and status, and displays accurate, up-to-date information to users without any manual intervention

---

### Scenario 14: CI run with no API access and empty cache [v1]

**Given** the CLI is configured in CI, the `quarantine/state` branch exists and contains a `quarantine.json` with 4 previously quarantined tests, the GitHub Actions cache is empty (e.g., cache expired or was manually cleared), and the GitHub API is completely unreachable (network outage, DNS failure, or total GitHub downtime)
**When** CI executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`
**Then** the CLI attempts to fetch `quarantine.json` from the `quarantine/state` branch via the GitHub Contents API and fails, attempts to load a cached copy from the GitHub Actions cache and finds none, logs a warning: "Unable to reach GitHub API and no cached quarantine state available. Running without quarantine exclusions.", runs the full test suite without excluding any quarantined tests, retries any failing tests per `--retries 3`, writes results to disk (`.quarantine/results.json`), logs warnings that quarantine state could not be read or updated, and exits with a status code based on the test results (flaky failures are still forgiven via retry). Any flaky tests detected during this run will be re-detected and quarantined on the next run when GitHub API connectivity is restored

---

## Dashboard

### Scenario 15: User views org-wide flaky test overview [v1]

**Given** the user is logged into the React Router v7 dashboard and the organization has 4 repositories with Quarantine configured, containing a combined 12 quarantined tests
**When** the user navigates to the org-level overview page
**Then** the dashboard displays a summary showing total quarantined tests across all repos (12), a breakdown per repository with test counts, the most recently quarantined tests, and links to drill into each project's details

---

### Scenario 16: User views single project's flaky test details and trends [v1]

**Given** the user is on the dashboard and selects the repository `acme/payments-service`, which has 3 quarantined tests
**When** the project detail page loads
**Then** the dashboard displays a list of all 3 quarantined tests with their names, date first quarantined, last flaky occurrence, links to their corresponding GitHub Issues, and a trend chart showing flaky test count over time (data derived from ingested GitHub Artifacts history)

---

### Scenario 17: User filters and searches quarantined tests on dashboard [v1]

**Given** the user is on the dashboard viewing a repository with 15 quarantined tests
**When** the user types "timeout" into the search bar and selects the filter "Status: Still Failing"
**Then** the dashboard filters the list to show only quarantined tests whose names contain "timeout" and whose most recent run result was a failure, updating the displayed count accordingly

---

### Scenario 18: Dashboard polls artifacts and ingests new results [v1]

**Given** the dashboard is running and its last successful poll was 5 minutes ago
**When** the background polling interval (5 minutes) elapses
**Then** the dashboard queries the GitHub Artifacts API for each configured repository for new artifacts since the last poll timestamp, downloads any new result artifacts, parses the structured JSON result format (see architecture section 5.2), updates the internal data store with new test run results, and refreshes any active user sessions via the hybrid polling mechanism (background poll completed, on-demand data now available)

---

### Scenario 19: Dashboard handles stale or inactive repos [v2+]

**Given** the dashboard is configured to poll 10 repositories, but 3 of them have had no CI activity in over 30 days
**When** a background polling cycle runs
**Then** the dashboard uses adaptive polling to reduce the frequency for the 3 inactive repos (e.g., polling once per hour instead of every 5 minutes), continues polling the 7 active repos at the normal 5-minute interval, and if an inactive repo produces a new artifact, the dashboard detects it on the next (less frequent) poll and resumes normal polling frequency for that repo

---

## Branch Protection

### Scenario 20: CLI updates quarantine.json on unprotected branch [v1]

**Given** the `quarantine/state` branch in the repository is not protected, and the CLI has detected a new flaky test
**When** the CLI writes the updated `quarantine.json` to the `quarantine/state` branch via the GitHub Contents API
**Then** the write succeeds directly via the Contents API PUT endpoint with SHA-based optimistic concurrency, and `quarantine.json` is updated on the `quarantine/state` branch

---

### Scenario 21: CLI updates quarantine.json when branch is protected [v1]

**Given** the `quarantine/state` branch has branch protection rules enabled (e.g., required reviews, status checks), and the CLI has detected a new flaky test
**When** the CLI attempts to write `quarantine.json` to the `quarantine/state` branch via the Contents API and receives a 403 or 422 error indicating the branch is protected
**Then** the CLI falls back to storing the pending quarantine state update in the GitHub Actions cache (keyed by run ID), logs a warning: "Branch 'quarantine/state' is protected. Quarantine state saved to Actions cache. A workflow with write access must apply the update.", and the CI build still exits with status code 0 (the flaky test is treated as quarantined for this run based on the pending update)

---

## Configuration

### Scenario 22: User runs quarantine doctor [v1]

**Given** a `quarantine.yml` file exists in the repo root with the following content:

```yaml
version: 1
framework: jest
retries: 3
issue_tracker: github
labels:
  - quarantine
notifications:
  github_pr_comment: true
```

**When** the developer runs `quarantine doctor` from the repo root
**Then** the CLI reads `quarantine.yml`, validates all fields against the expected schema (including `version`, `framework`, `issue_tracker`, `labels`, and `notifications`), confirms the framework is supported in v1 (`jest`, `rspec`, or `vitest`), confirms the retry count is a positive integer, confirms the issue tracker is valid for v1, validates forward-compatible fields (e.g., `issue_tracker: jira` produces "Error: 'issue_tracker: jira' is not supported in v1. Supported values: github."), and prints "quarantine.yml is valid." with exit code 0; if the file is missing, the CLI prints "Error: quarantine.yml not found in the current directory." and exits with code 1; if a field is invalid (e.g., `retries: -1`), the CLI prints "Error: 'retries' must be a positive integer." and exits with code 1

---

### Scenario 22b: Invalid forward-compatible config value [v1]

**Given** a `quarantine.yml` file exists in the repo root with the following content:

```yaml
version: 1
framework: jest
retries: 3
issue_tracker: jira
labels:
  - quarantine
```

**When** the developer runs `quarantine doctor` from the repo root
**Then** the CLI reads `quarantine.yml`, detects that `issue_tracker: jira` is a known but unsupported value in v1, and prints: "Error: 'issue_tracker: jira' is not supported in v1. Supported values: github." and exits with code 1

---

### Scenario 22c: quarantine init with Jest recommendation [v1]

**Given** a developer has a project with a Jest test suite and runs `quarantine init`, selecting `jest` as the framework
**When** `quarantine init` completes framework selection
**Then** the CLI recommends configuring `jest-junit` for clean `test_id` output, printing guidance such as: "Recommended: install jest-junit and add to your Jest config for reliable test identification. See https://github.com/jest-community/jest-junit for setup instructions.", and proceeds with the remaining init steps (retry count, JUnit XML path, config creation, branch setup)

---

### Scenario 23: User overrides auto-detected framework in quarantine.yml [v1]

**Given** the project contains both `jest.config.js` and `vitest.config.ts` files, and the CLI's auto-detection would pick `jest` by default
**When** the developer sets `framework: vitest` in `quarantine.yml` and runs `quarantine run --retries 3 -- vitest run --reporter=junit`
**Then** the CLI uses `vitest` as the framework (honoring the explicit override), parses Vitest-formatted JUnit XML output for test results, and processes flaky test detection using Vitest-specific test identifiers

---

### Scenario 24: User customizes retry count [v1]

**Given** `quarantine.yml` exists with `retries: 5`
**When** the developer runs `quarantine run -- jest --ci --reporters=default --reporters=jest-junit` without the `--retries` flag
**Then** the CLI reads the retry count from `quarantine.yml` and retries each failing test up to 5 times; if the developer runs `quarantine run --retries 2 -- jest --ci --reporters=default --reporters=jest-junit`, the CLI flag overrides the config file and retries only 2 times

---

## Edge Cases

### Scenario 33: All tests in the suite are quarantined [v1]

**Given** the CLI is configured in CI, `quarantine.json` on the `quarantine/state` branch contains entries for every test in the suite (e.g., 50 out of 50 tests are quarantined), and the corresponding GitHub Issues are all open
**When** CI executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`
**Then** the CLI reads the quarantine list from `quarantine.json`, constructs exclusion flags that exclude all 50 tests from execution, and the test runner executes 0 tests. Some test frameworks (notably Jest) exit non-zero when no tests match (e.g., Jest exits with code 1 and the message "No tests found"). The CLI detects this condition by examining the JUnit XML output: if the XML contains zero test cases (or no XML is produced) and the test runner exited non-zero, the CLI checks whether the number of quarantined exclusions equals or exceeds the expected test count. If all failures can be attributed to an empty suite caused by quarantine exclusions, the CLI treats this as a successful run and exits with code 0. The CLI posts a PR comment noting: "All 50 tests in this suite are currently quarantined. No tests were executed. Consider reviewing quarantined tests and closing resolved issues." The results artifact contains `summary.total: 0`, `summary.quarantined: 50`, and an empty `tests` array, reflecting that no tests were executed. The CLI logs a warning to stderr: `[quarantine] WARNING: All tests excluded by quarantine. No tests executed.`

---

### Scenario 34: Search API result limit exceeded during unquarantine detection [v1]

**Given** the CLI is configured in CI, `quarantine.json` contains 5 currently quarantined tests, and the repository has a long history of quarantine issues -- over 1,000 closed GitHub Issues with the `quarantine` label accumulated over months of CI activity
**When** CI executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit` and the CLI performs the batch issue status check via `GET /search/issues?q=repo:{owner}/{repo}+is:issue+is:closed+label:quarantine`, and the GitHub Search API returns `total_count: 1,247` but caps the retrievable results at 1,000 items (the documented Search API maximum)
**Then** the CLI paginates through all available results (up to 1,000 items at 100 per page = 10 pages), matches closed issue numbers against the `issue_number` fields in `quarantine.json` entries, and unquarantines any tests whose issues appear in the retrieved results. If a quarantined test's closed issue falls outside the 1,000-result window (i.e., it was one of the oldest 247 closed issues not returned by the API), that test remains quarantined for this run. The CLI detects this condition when the Search API response includes `total_count` greater than the number of retrievable items, and logs a warning: `[quarantine] WARNING: GitHub Search API returned 1,000 of 1,247 closed quarantine issues. Some closed issues may not be detected. Consider narrowing the search with a date filter or manually closing stale quarantine issues.` The missed unquarantine is non-critical: the test remains quarantined (excluded from execution) until a subsequent run retrieves the closed issue, or until a developer notices and manually removes the entry. This is consistent with the quarantine-wins principle (ADR-012) -- erring on the side of keeping a test quarantined is safer than accidentally re-enabling a flaky test. Future mitigation: add a `closed_since` date filter to the search query scoped to the test's `quarantined_at` timestamp, which keeps results within the 1,000-item window for most repositories.

---

### Scenario 35: Concurrent unquarantine race with quarantine-wins resolution [v1]

**Given** `quarantine.json` on the `quarantine/state` branch contains entries for `PaymentService > should handle charge timeout` (test X, issue #42) and `SearchService > should fuzzy match` (test Y, issue #55), both with open GitHub Issues. Two CI builds (Build A and Build B) are running in parallel. A developer closes issue #42 for test X between the moment Build A reads `quarantine.json` and the moment Build B reads it.
**When** Build A reads `quarantine.json` at SHA `abc123` (test X and test Y present), performs the batch issue status check, and does NOT see issue #42 as closed (it was still open at the time of Build A's search query). Build A runs tests, detects a new flaky test `ApiService > should handle timeout` (test Z), and writes `quarantine.json` with test X, test Y, and test Z (SHA `abc123` -> `def456`, write succeeds). Meanwhile, Build B reads `quarantine.json` at SHA `abc123` (stale, before Build A's write), performs the batch issue status check, and DOES see issue #42 as closed (the developer closed it after Build A's check but before Build B's check). Build B removes test X from its local copy and attempts to write `quarantine.json` with only test Y (using stale SHA `abc123`).
**Then** Build B's write fails with a 409 Conflict because the SHA no longer matches (Build A already wrote `def456`). Build B re-reads `quarantine.json` at SHA `def456` (which now contains test X, test Y, and test Z). Build B merges using the quarantine-wins (union) strategy per ADR-012: test X is present in the remote state (Build A's write) and absent in Build B's local state (removed due to closed issue), so the union merge preserves test X. The resulting `quarantine.json` contains test X, test Y, and test Z. Build B writes this merged state and logs: `[quarantine] WARNING: Test 'PaymentService > should handle charge timeout' was unquarantined (issue closed) but re-quarantined due to concurrent update. It will be unquarantined on the next build.` On the very next CI run (Build C), the CLI reads `quarantine.json`, performs the batch issue status check, finds issue #42 is closed, removes test X, and writes the update. The impact of the race is one extra build cycle where test X remains quarantined -- it is excluded from execution rather than running and potentially failing, which is the safe default.

---

## v2+ Flows

### Scenario 25: Code sync adapter creates PR with skip markers [v2+]

**Given** the CLI has quarantined `tests/test_payment.py::test_charge_timeout` and updated `quarantine.json`, and the code sync adapter is enabled in `quarantine.yml` with `code_sync: true`
**When** the adapter runs (triggered by the quarantine.json update)
**Then** the adapter opens a new PR titled "quarantine: skip tests/test_payment.py::test_charge_timeout" that adds a `@pytest.mark.skip(reason="Quarantined: flaky — see #42")` decorator to the `test_charge_timeout` function in `tests/test_payment.py`, referencing the GitHub Issue number, with a PR body explaining the change and linking to the issue

---

### Scenario 26: Code sync adapter removes skip markers when issue is closed [v2+]

**Given** the GitHub Issue for `tests/test_payment.py::test_charge_timeout` has been closed, the test has been removed from `quarantine.json`, and the source code still contains `@pytest.mark.skip(reason="Quarantined: flaky -- see #42")`
**When** the code sync adapter detects the issue closure (via webhook or polling)
**Then** the adapter opens a new PR titled "unquarantine: re-enable tests/test_payment.py::test_charge_timeout" that removes the `@pytest.mark.skip` decorator from the test function, with a PR body noting the linked issue was resolved

---

### Scenario 27: User installs GitHub App on org [v2+]

**Given** an organization admin navigates to the Quarantine GitHub App installation page
**When** the admin installs the GitHub App on the organization, granting access to all repositories (or a selected subset)
**Then** the GitHub App sends an `installation` webhook event to the dashboard, the dashboard auto-discovers all accessible repositories, begins polling them for CI artifacts, and displays the newly discovered repositories on the org overview page within the next polling cycle

---

### Scenario 28: User logs into dashboard via GitHub OAuth [v2+]

**Given** the React Router v7 dashboard is deployed and configured with GitHub OAuth credentials (client ID and secret), and a developer has a GitHub account that belongs to an organization with Quarantine installed
**When** the developer clicks "Sign in with GitHub" on the dashboard login page
**Then** the dashboard redirects to GitHub's OAuth authorization page, the developer authorizes the app, GitHub redirects back to the dashboard with an auth code, the dashboard exchanges the code for an access token, creates a session for the user, and displays the org-level overview filtered to repos the user has access to

---

### Scenario 29: Slack notification when flaky test count exceeds threshold [v2+]

**Given** `quarantine.yml` is configured with:

```yaml
notifications:
  github_pr_comment: true
  slack:
    webhook_url: https://hooks.slack.com/services/T00/B00/xxxxx
    threshold: 10
```

and the organization currently has 9 quarantined tests
**When** a CI run detects a new flaky test, bringing the total to 10
**Then** the CLI (or dashboard) sends a Slack notification to the configured webhook with a message: "Warning: 10 tests are now quarantined in acme/payments-service. Review the dashboard: https://quarantine.example.com/acme/payments-service"

---

### Scenario 30: Jenkins CI run (non-GitHub Actions environment) [v2+]

**Given** a project uses Jenkins for CI (not GitHub Actions), the Quarantine CLI is installed on the Jenkins agent, and `quarantine.yml` is configured with `ci_provider: jenkins`
**When** the Jenkins pipeline executes `quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit` and detects a flaky test
**Then** the CLI updates `quarantine.json` on the `quarantine/state` branch via the GitHub Contents API (same as GitHub Actions), creates a GitHub Issue, but instead of relying on `actions/upload-artifact` (unavailable in Jenkins), the CLI uploads results via the dashboard's HTTP API endpoint or stores them in a configured artifact backend, and the Jenkins build exits with status code 0

---

### Scenario 31: Jira ticket created instead of GitHub issue [v2+]

**Given** `quarantine.yml` is configured with:

```yaml
issue_tracker: jira
jira:
  host: https://acme.atlassian.net
  project: FLAKY
  issue_type: Bug
```

**When** the CLI detects `tests/test_payment.py::test_charge_timeout` as flaky
**Then** the CLI creates a Jira ticket in the `FLAKY` project with summary "Flaky test: tests/test_payment.py::test_charge_timeout", issue type `Bug`, a description including failure logs and retry details, and a label `quarantine`; the `quarantine.json` entry references the Jira ticket key (e.g., `FLAKY-123`) instead of a GitHub Issue number; when `FLAKY-123` is transitioned to Done, the test is unquarantined on the next CI run

---

### Scenario 32: Periodic flaky test report generated [v2+]

**Given** `quarantine.yml` is configured with:

```yaml
reports:
  schedule: weekly
  email: team-leads@acme.com
```

and the dashboard has accumulated flaky test data over the past week
**When** the scheduled report trigger fires (e.g., every Monday at 9:00 AM UTC)
**Then** the dashboard generates a report summarizing: total quarantined tests (new this week, resolved this week, still active), longest-quarantined tests, most frequently flaky tests, and trend data (improving or worsening); the report is emailed to `team-leads@acme.com` with a link to the full dashboard view