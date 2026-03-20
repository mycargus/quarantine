# Core Flows

### Scenario 19: Normal CI run with no flaky tests [M2]

**Risk:** The CLI introduces false positives, unnecessary retries, or non-zero exit codes when all tests pass, breaking builds that have no flaky tests.

**Given** the CLI is configured in CI, `quarantine.json` on the
`quarantine/state` branch contains zero quarantined tests, and all tests in the
suite are deterministic

**When** the developer pushes a commit and CI executes
`quarantine run -- jest --ci --reporters=default --reporters=jest-junit`

**Then** the CLI:
1. Reads `quarantine.json` from the `quarantine/state` branch — no quarantined
   tests.
2. Runs the test suite once. All tests pass on the first attempt.
3. No retries are triggered.
4. No changes are made to `quarantine.json`.
5. Writes results to `.quarantine/results.json`.
6. No PR comment is posted (nothing to report).
7. Exits with code 0.

---

### Scenario 20: CI run detects a new flaky test [M3]

**Risk:** A flaky test is not detected, continues breaking builds intermittently, and no issue is created to track it.

**Given** the CLI is configured in CI, `quarantine.json` has no entry for the
test `PaymentService > should handle charge timeout`, and this test is
non-deterministic

**When** CI executes
`quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`
and `should handle charge timeout` fails on the first run but passes on retry
2 of 3

**Then** the CLI:
1. Identifies `should handle charge timeout` as flaky (failed initially, passed
   on retry — per ADR-001, re-run detection).
2. Fetches the current `quarantine.json` from the `quarantine/state` branch,
   recording its SHA for optimistic concurrency (ADR-012).
3. Adds an entry for the test with `test_id`
   (`file_path::classname::name`, per ADR-020), timestamp, and first-seen
   metadata.
4. Writes the updated `quarantine.json` back via the Contents API using
   compare-and-swap (SHA-based CAS, per ADR-006).
5. Creates a GitHub Issue titled `[Quarantine] PaymentService > should handle
   charge timeout` with labels `["quarantine", "quarantine:{test_hash}"]`
   (dedup via deterministic label, per ADR-009).
6. Posts a PR comment summarizing the newly quarantined test, identified by the
   `<!-- quarantine-bot -->` HTML marker (per ADR-009).
7. Writes results to `.quarantine/results.json` including retry details.
8. Exits with code 0 (pass, since the failure was flaky — not a real failure).

---

### Scenario 21: CI run with a previously quarantined test — Jest or Vitest (pre-execution exclusion) [M4]

**Risk:** A quarantined test is not excluded from execution, runs, fails, and breaks the build despite being quarantined (ADR-003).

**Given** `quarantine.json` on the `quarantine/state` branch contains an entry
for `PaymentService > should handle charge timeout` with status `quarantined`,
and the corresponding GitHub Issue is still open. The project uses Jest (or
Vitest).

**When** CI executes
`quarantine run -- jest --ci --reporters=default --reporters=jest-junit`

**Then** the CLI:
1. Reads `quarantine.json` and finds the quarantined test.
2. Batch-checks issue status via the GitHub Search API
   (`GET /search/issues?q=repo:{owner}/{repo} is:issue is:closed label:quarantine`).
   The issue is still open — test stays quarantined.
3. Augments the test command with framework-specific exclusion flags to prevent
   the quarantined test from executing:
   - **Jest:** `--testPathIgnorePatterns` for full-file exclusion, or
     `--testNamePattern` with negative lookahead for partial-file exclusion.
   - **Vitest:** `--exclude` for file-level, or `-t` with negative lookahead
     for partial-file.
4. The quarantined test does not run at all — it does not appear in the test
   runner output or JUnit XML. (Per ADR-003: Jest and Vitest support
   pre-execution exclusion.)
5. Posts a PR comment noting which tests were excluded from execution.
6. Writes results to `.quarantine/results.json`.
7. Exits with code 0.

---

### Scenario 22: CI run with a previously quarantined test — RSpec (post-execution filtering) [M4]

**Risk:** A quarantined RSpec test's failure is not suppressed from the exit code, breaking the build despite the test being quarantined (ADR-003).

**Given** `quarantine.json` on the `quarantine/state` branch contains an entry
for `User#valid? returns true for valid attributes` with status `quarantined`,
and the corresponding GitHub Issue is still open. The project uses RSpec.

**When** CI executes
`quarantine run -- rspec --format RspecJunitFormatter --out rspec.xml`

**Then** the CLI:
1. Reads `quarantine.json` and finds the quarantined test.
2. Batch-checks issue status — the issue is still open.
3. Runs the RSpec test command **without excluding the quarantined test**.
   (Per ADR-003: RSpec lacks reliable pre-execution exclusion for individual
   tests within a file without code changes. The test still runs.)
4. Parses the JUnit XML output. If the quarantined test failed:
   - The failure is **suppressed from the exit code**. The CLI does not count
     it as a real failure.
   - This is the same behavior as the initial flaky detection: isolate flaky
     tests from the final pass/fail signal.
5. Posts a PR comment noting which quarantined tests ran and had their failures
   suppressed.
6. Writes results to `.quarantine/results.json` with the quarantined test
   marked as `status: "quarantined"` and `original_status: "failed"`.
7. Exits with code 0 (even though the quarantined test failed, because its
   failure is suppressed).

**Why RSpec differs from Jest/Vitest:** RSpec does not provide a built-in
mechanism to exclude individual tests by name without modifying source code
(e.g., adding `xit` or a custom tag). Pre-execution file-level exclusion is
possible (omit files from the rspec command), but individual test exclusion
within a file is not. Users who want CI time savings for RSpec can add custom
tags to quarantined tests in their source code, but Quarantine does not require
or automate this in v1. (Code sync adapter in v2 may add skip markers
automatically.)

---

### Scenario 23: CI run with a real failure [M3]

**Risk:** A genuinely broken test is misclassified as flaky and quarantined, hiding a real bug from the build signal.

**Given** the CLI is configured in CI, `quarantine.json` has no entry for
`CheckoutService > should apply discount`, and this test has a genuine bug

**When** CI executes
`quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`
and `should apply discount` fails on all 3 retries

**Then** the CLI:
1. Determines the test is a real (deterministic) failure — it failed on every
   retry attempt.
2. Does NOT add it to `quarantine.json`. (Only tests that pass on retry are
   classified as flaky.)
3. Does NOT create a GitHub Issue for it.
4. Writes results to `.quarantine/results.json`.
5. Posts a PR comment noting the hard failure.
6. Exits with code 1 (test failure).

---

### Scenario 24: Multiple flaky tests detected in a single run [M3]

**Risk:** When multiple flaky tests are detected in one run, some are missed or duplicate issues are created due to non-atomic state updates.

**Given** the CLI is configured in CI with `--retries 3`, and `quarantine.json`
has no entries for `SearchService > should fuzzy match` or
`ApiService > should handle rate limit`

**When** CI executes
`quarantine run --retries 3 -- jest --ci --reporters=default --reporters=jest-junit`
and both `should fuzzy match` (fails run 1, passes run 2) and `should handle
rate limit` (fails run 1, fails run 2, passes run 3) are detected as flaky

**Then** the CLI:
1. Adds both tests to `quarantine.json` in a single write (atomic update via
   Contents API with SHA-based compare-and-swap).
2. Creates two separate GitHub Issues, each with the `quarantine` label and
   their respective `quarantine:{test_hash}` labels.
3. Posts a single PR comment summarizing both newly quarantined tests.
4. Writes results to `.quarantine/results.json`.
5. Exits with code 0.

---

### Scenario 25: Quarantined test's GitHub issue is closed (unquarantine) [M4]

**Risk:** Closing a GitHub Issue does not actually unquarantine the test, leaving it permanently excluded from builds even after being fixed (ADR-017).

**Given** `quarantine.json` contains an entry for
`PaymentService > should handle charge timeout` with `issue_number: 42`, and
GitHub Issue #42 has been closed by a developer (indicating the flaky test has
been fixed — per ADR-017, unquarantine happens only when a human closes the
issue)

**When** the next CI run executes `quarantine run`

**Then** the CLI:
1. Reads `quarantine.json`.
2. Performs a batch issue status check via the GitHub Search API
   (`GET /search/issues?q=repo:{owner}/{repo} is:issue is:closed label:quarantine`).
   One API call returns all closed quarantine issues.
3. Detects Issue #42 is closed.
4. Removes the `should handle charge timeout` entry from `quarantine.json` via
   Contents API with compare-and-swap.
5. The test is no longer excluded from execution (Jest/Vitest) or no longer has
   its failure suppressed (RSpec) — it runs normally as part of the suite.
6. If the test fails, it fails the build like any other test (exit 1). If it
   passes, exit 0.

---

### Scenario 26: CI run with mixed results — flaky, quarantined, real failures, and passes [M4]

**Risk:** The interaction of multiple test states (quarantined, unquarantined, flaky, failing, passing) produces an incorrect exit code or corrupted quarantine state.

**Given** the CLI is configured in CI with `--retries 3`. `quarantine.json`
contains entries for tests A (quarantined, issue open) and test B (quarantined,
issue closed). The suite also contains test C (deterministic pass), test D
(non-deterministic / flaky), and test E (genuine bug).

**When** CI executes `quarantine run --retries 3 -- jest --ci ...`

**Then** the CLI:
1. Reads `quarantine.json` — finds test A (quarantined) and test B
   (to be unquarantined, issue closed).
2. Removes test B from quarantine state.
3. Excludes test A from execution (Jest/Vitest pre-execution).
4. Runs tests B, C, D, E (test A excluded).
5. Test C passes. Test D fails then passes on retry (flaky). Test E fails all
   retries (genuine). Test B passes (was fixed).
6. Updates `quarantine.json`: removes test B, adds test D.
7. Creates a GitHub Issue for test D.
8. Posts PR comment with all sections:
   - **Quarantined (excluded):** test A
   - **Unquarantined:** test B (issue closed)
   - **Flaky detected:** test D (newly quarantined)
   - **Failed:** test E (genuine failure)
9. Writes results to `.quarantine/results.json`.
10. Exits with code 1 (test E is a genuine failure).

---

### Scenario 67: Normal CI run with RSpec — all tests pass [M2]

**Risk:** RSpec JUnit XML uses a different structure (classname-based file path extraction, no `file` attribute) that could silently produce incorrect test IDs or fail to parse, undetected because only Jest is tested end-to-end.

**Given** the CLI is configured with `framework: rspec` and `junitxml: rspec.xml`
in `quarantine.yml`, the `quarantine/state` branch exists, and all tests in the
suite are deterministic

**When** CI executes
`quarantine run -- rspec --format RspecJunitFormatter --out rspec.xml`

**Then** the CLI:
1. Checks `quarantine/state` branch exists — OK.
2. Runs the RSpec command. All tests pass.
3. Parses the RSpec-formatted JUnit XML (`rspec.xml`). Extracts `file_path` from
   the classname using RSpec-specific rules (per cli-spec.md).
4. Constructs `test_id` as `file_path::classname::name` for each test case.
5. Writes results to `.quarantine/results.json` with correct test counts and
   `framework: "rspec"`.
6. Exits with code 0.

---

### Scenario 68: Normal CI run with Vitest — all tests pass [M2]

**Risk:** Vitest JUnit XML uses the suite `name` attribute for file path extraction, which differs from both Jest (explicit `file` attribute) and RSpec (classname-based). Incorrect parsing would produce wrong test IDs.

**Given** the CLI is configured with `framework: vitest` and
`junitxml: junit-report.xml` in `quarantine.yml`, the `quarantine/state` branch
exists, and all tests in the suite are deterministic

**When** CI executes `quarantine run -- vitest run --reporter=junit`

**Then** the CLI:
1. Checks `quarantine/state` branch exists — OK.
2. Runs the Vitest command. All tests pass.
3. Parses the Vitest-formatted JUnit XML (`junit-report.xml`). Extracts
   `file_path` from the suite `name` attribute using Vitest-specific rules
   (per cli-spec.md).
4. Constructs `test_id` as `file_path::classname::name` for each test case.
5. Writes results to `.quarantine/results.json` with correct test counts and
   `framework: "vitest"`.
6. Exits with code 0.

---

### Scenario 69: CI run with test failures — no retries [M2]

**Risk:** Without retry logic (M3), the CLI might exit 0 on test failure or exit 2 (quarantine error) instead of correctly propagating exit 1 for test failures.

**Given** the CLI is configured in CI, the `quarantine/state` branch exists, and
one test in the suite has a genuine bug

**When** CI executes
`quarantine run -- jest --ci --reporters=default --reporters=jest-junit`
and the test `CheckoutService > should apply discount` fails

**Then** the CLI:
1. Checks `quarantine/state` branch exists — OK.
2. Runs the test suite. The runner exits non-zero.
3. Parses the JUnit XML. Finds `should apply discount` with status `failed`.
4. Writes results to `.quarantine/results.json` with `summary.failed: 1`.
5. Exits with code 1 (test failure — not a quarantine error).

---
