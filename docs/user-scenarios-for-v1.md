# v1 User Scenarios

This document describes all user scenarios for Quarantine v1 in Given-When-Then
format. Each scenario is tagged with the milestone where it first becomes
testable.

For v2+ scenarios, see `docs/user-scenarios.md`.

---

## Initialization

### Scenario 1: First-time setup with Jest [M1]

**Given** a developer has a project with a Jest test suite and a GitHub Actions
CI pipeline, and Quarantine CLI is installed but not yet initialized

**When** the developer runs `quarantine init` from the repo root

**Then** the CLI interactively prompts for:
1. Framework: `Which test framework? [rspec/jest/vitest]` — developer enters
   `jest`
2. Retries: `How many retries for failing tests? [3]` — developer presses
   enter (accepts default)
3. JUnit XML path: `Path/glob for JUnit XML output? [junit.xml]` — developer
   presses enter (accepts default)

The CLI then:
- Creates `quarantine.yml` in the current directory:
  ```yaml
  version: 1
  framework: jest
  ```
  (Fields matching defaults are omitted except `framework`, which is always
  written.)
- Validates the GitHub token (`QUARANTINE_GITHUB_TOKEN`, falls back to
  `GITHUB_TOKEN`). Makes an authenticated API call to verify the token is valid.
- Auto-detects `github.owner` and `github.repo` from the `origin` git remote.
- Verifies the token has sufficient permissions by reading repository metadata
  via `GET /repos/{owner}/{repo}`.
- Creates the `quarantine/state` branch by reading the default branch HEAD SHA
  via `GET /repos/{owner}/{repo}/git/ref/heads/{default_branch}`, then creating
  the branch via `POST /repos/{owner}/{repo}/git/refs`.
- Writes an empty `quarantine.json` to the new branch via
  `PUT /repos/{owner}/{repo}/contents/quarantine.json`:
  ```json
  {
    "version": 1,
    "updated_at": "2026-03-18T12:00:00Z",
    "tests": {}
  }
  ```
- Prints recommended `jest-junit` configuration:
  ```
  Recommended jest-junit configuration (in jest.config.js or package.json):

    "jest-junit": {
      "classNameTemplate": "{classname}",
      "titleTemplate": "{title}",
      "ancestorSeparator": " > ",
      "addFileAttribute": "true"
    }

  This produces well-structured JUnit XML for quarantine's test identification.
  ```
- Prints a summary:
  ```
  Quarantine initialized successfully.

    Config:     quarantine.yml (created)
    Framework:  jest
    Retries:    3
    JUnit XML:  junit.xml
    Branch:     quarantine/state (created)

  Next steps:
    1. Add quarantine to your CI workflow:

       - name: Run tests
         run: quarantine run -- jest --ci --reporters=default --reporters=jest-junit
         env:
           QUARANTINE_GITHUB_TOKEN: ${{ secrets.QUARANTINE_GITHUB_TOKEN }}

    2. Upload results as an artifact:

       - name: Upload quarantine results
         if: always()
         uses: actions/upload-artifact@v4
         with:
           name: quarantine-results-${{ github.run_id }}
           path: .quarantine/results.json
  ```
- Exits with code 0.

---

### Scenario 2: quarantine init with RSpec [M1]

**Given** a developer has a project with an RSpec test suite

**When** the developer runs `quarantine init` and selects `rspec` as the
framework, accepting defaults for retries (3) and JUnit XML path (`rspec.xml`)

**Then** the CLI creates `quarantine.yml` with `framework: rspec`, validates the
token, creates the branch, and prints the summary. No framework-specific
recommendation is printed (unlike Jest's `jest-junit` guidance). The JUnit XML
default is `rspec.xml`. The workflow snippet uses:
```
run: quarantine run -- rspec --format RspecJunitFormatter --out rspec.xml
```
Exits with code 0.

---

### Scenario 3: quarantine init with Vitest [M1]

**Given** a developer has a project with a Vitest test suite

**When** the developer runs `quarantine init` and selects `vitest` as the
framework, accepting defaults for retries (3) and JUnit XML path
(`junit-report.xml`)

**Then** the CLI creates `quarantine.yml` with `framework: vitest`, validates
the token, creates the branch, and prints the summary. The workflow snippet
uses:
```
run: quarantine run -- vitest run --reporter=junit
```
Exits with code 0.

---

### Scenario 4: quarantine init when quarantine.yml already exists [M1]

**Given** a developer has already run `quarantine init` and a `quarantine.yml`
file exists in the repo root

**When** the developer runs `quarantine init` again

**Then** the CLI detects the existing `quarantine.yml` and prompts:
`quarantine.yml already exists. Overwrite? [y/N]`

If the developer enters `y`: the CLI proceeds with the interactive prompts and
overwrites the existing file.

If the developer enters `n` or presses enter (default): the CLI prints
`Aborted. Existing quarantine.yml preserved.` and exits with code 0.

---

### Scenario 5: quarantine init when quarantine/state branch already exists [M1]

**Given** a developer has already run `quarantine init` and the
`quarantine/state` branch exists in the GitHub repository with a
`quarantine.json` file

**When** the developer runs `quarantine init` again (e.g., after recreating
`quarantine.yml`)

**Then** the CLI detects that the branch already exists (via
`GET /repos/{owner}/{repo}/git/ref/heads/quarantine/state` returning 200),
prints a warning: `Branch 'quarantine/state' already exists. Skipping branch
creation.`, and does NOT overwrite the existing `quarantine.json`. The rest of
the init flow (config creation, token validation, summary) proceeds normally.
Exits with code 0.

---

### Scenario 6: quarantine init with no GitHub token [M1]

**Given** a developer has Quarantine CLI installed, but neither
`QUARANTINE_GITHUB_TOKEN` nor `GITHUB_TOKEN` is set in the environment

**When** the developer runs `quarantine init`

**Then** the CLI completes the interactive prompts and creates `quarantine.yml`
locally, but when it attempts to validate the GitHub token, it prints:
```
Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.

  export QUARANTINE_GITHUB_TOKEN=ghp_your_token_here

Required token scope: repo (read/write contents, create issues, post PR comments)
```
Exits with code 2. The `quarantine.yml` file has already been created (partial
init — config created, GitHub setup not completed).

---

### Scenario 7: quarantine init with insufficient token permissions [M1]

**Given** a developer has `GITHUB_TOKEN` set but the token lacks the `repo`
scope (e.g., it has only `read:org` scope)

**When** the developer runs `quarantine init` and the CLI attempts to verify
repository access via `GET /repos/{owner}/{repo}`, receiving a 403 Forbidden

**Then** the CLI prints:
```
Error: GitHub token lacks permission to access repository 'my-org/my-project'.
Required scope: repo. Check your token permissions at https://github.com/settings/tokens
```
Exits with code 2. Init failures are always fatal with diagnostics — no
degraded mode (per milestones.md).

---

### Scenario 8: quarantine init when not a git repository [M1]

**Given** a developer runs `quarantine init` in a directory that is not a git
repository (no `.git` directory)

**When** the CLI attempts to auto-detect `github.owner` and `github.repo` from
the git remote

**Then** the CLI prints:
```
Error: Not a git repository. Run 'quarantine init' from the root of a git repository.
```
Exits with code 2.

---

### Scenario 9: quarantine init with non-GitHub remote [M1]

**Given** a developer's git repository has its `origin` remote set to a
non-GitHub URL (e.g., `https://gitlab.com/my-org/my-project.git`)

**When** the developer runs `quarantine init` and the CLI attempts to parse the
`origin` remote URL

**Then** the CLI prints:
```
Error: Remote 'origin' is not a GitHub URL: https://gitlab.com/my-org/my-project.git
Quarantine v1 supports GitHub repositories only. Set github.owner and github.repo
in quarantine.yml manually if using a non-standard remote.
```
Exits with code 2.

---

### Scenario 10: quarantine init with invalid framework input [M1]

**Given** a developer runs `quarantine init`

**When** the CLI prompts `Which test framework? [rspec/jest/vitest]` and the
developer enters `pytest`

**Then** the CLI prints: `Invalid framework 'pytest'. Supported: rspec, jest,
vitest.` and re-prompts: `Which test framework? [rspec/jest/vitest]`. The prompt
repeats until a valid value is entered.

---

### Scenario 11: quarantine init with GitHub API unreachable [M1]

**Given** a developer has a valid GitHub token but the GitHub API is unreachable
(network failure, DNS resolution failure, or GitHub outage)

**When** the developer runs `quarantine init` and the CLI attempts to verify
repository access

**Then** the CLI retries once after 2 seconds. If the retry also fails, prints:
```
Error: Unable to reach GitHub API: connection timed out.
Check your network connection and try again.
```
Exits with code 2. Init does NOT use degraded mode — failures are fatal with
diagnostics.

---

### Scenario 12: quarantine run without prior init [M1]

**Given** a developer has the CLI installed but has not run `quarantine init`
(no `quarantine.yml` in the repo root, no `quarantine/state` branch)

**When** the developer runs `quarantine run -- jest --ci`

**Then** the CLI checks for `quarantine.yml` and finds it missing. Prints:
```
Quarantine is not initialized for this repository. Run 'quarantine init' first.
```
Exits with code 2. The test command is NOT executed.

---

## Configuration Validation

### Scenario 13: quarantine doctor — valid configuration [M1]

**Given** a `quarantine.yml` file exists in the repo root with the following
content:
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

**Then** the CLI reads `quarantine.yml`, validates all fields against the schema
(including `version`, `framework`, `issue_tracker`, `labels`, and
`notifications`), resolves auto-detected values (`github.owner` and
`github.repo` from the `origin` git remote, framework-specific `junitxml`
default), and prints the resolved configuration:
```
quarantine.yml is valid.

  Resolved configuration:
    version:         1
    framework:       jest
    retries:         3
    junitxml:        junit.xml (default)
    github.owner:    my-org (auto-detected)
    github.repo:     my-project (auto-detected)
    issue_tracker:   github
    labels:          [quarantine]
    notifications:   github_pr_comment: true
    storage.branch:  quarantine/state (default)
```
If `QUARANTINE_GITHUB_TOKEN` or `GITHUB_TOKEN` is not set, appends a warning:
`Warning: No GitHub token found in environment. 'quarantine run' will fail
unless QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN is set.`

Exits with code 0.

---

### Scenario 14: quarantine doctor — missing config file [M1]

**Given** no `quarantine.yml` file exists in the current directory

**When** the developer runs `quarantine doctor`

**Then** the CLI prints:
```
Error: quarantine.yml not found in the current directory.
Run 'quarantine init' to create one.
```
Exits with code 2.

---

### Scenario 15: quarantine doctor — invalid field values [M1]

**Given** a `quarantine.yml` file exists with `retries: -1`

**When** the developer runs `quarantine doctor`

**Then** the CLI prints:
```
Error: Invalid retries value: -1. Must be between 1 and 10.
```
Exits with code 2. Each invalid field produces a separate error line. All errors
are reported (not short-circuited on first error).

---

### Scenario 16: quarantine doctor — forward-compatible config value [M1]

**Given** a `quarantine.yml` file exists with:
```yaml
version: 1
framework: jest
retries: 3
issue_tracker: jira
```

**When** the developer runs `quarantine doctor`

**Then** the CLI prints:
```
Error: Unsupported issue_tracker 'jira'. This version supports: github.
Jira support is planned for a future release.
```
Exits with code 2. Forward-compatible fields (`issue_tracker`, `labels`,
`notifications`) have restricted allowed values in v1 that expand in v2 without
schema changes (ADR-021).

---

### Scenario 17: quarantine doctor — unknown fields [M1]

**Given** a `quarantine.yml` file exists with:
```yaml
version: 1
framework: jest
custom_field: something
notifications:
  github_pr_comment: true
  slack: true
```

**When** the developer runs `quarantine doctor`

**Then** the CLI reports:
- Warning: `Unknown field 'custom_field' in quarantine.yml will be ignored.`
  (unknown top-level keys produce warnings, not errors)
- Error: `Unknown notification channel 'slack'. This version supports:
  github_pr_comment. Slack and email notifications are planned for a future
  release.` (unknown keys under `notifications` are errors because they may
  indicate a user expecting functionality that doesn't exist)

Exits with code 2 (errors present). Warnings alone would exit 0.

---

### Scenario 18: quarantine doctor — custom config path [M1]

**Given** a valid configuration file exists at `config/quarantine.yml` (not the
default location)

**When** the developer runs `quarantine doctor --config config/quarantine.yml`

**Then** the CLI reads from the specified path, validates, and prints the
resolved configuration. Exits with code 0. The `--config` flag works the same
way for `quarantine run`.

---

## Core Flows

### Scenario 19: Normal CI run with no flaky tests [M2]

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

## Concurrency

### Scenario 27: Concurrent CI builds detect the same flaky test simultaneously [M5]

**Given** the CLI is configured in CI, `quarantine.json` has no entry for
`CacheService > should handle eviction`, and two CI builds (Build A and Build B)
are running in parallel on different PRs

**When** both builds detect `should handle eviction` as flaky and both attempt
to create a GitHub Issue for it

**Then** the first build to reach GitHub creates the Issue titled
`[Quarantine] CacheService > should handle eviction` with labels `["quarantine",
"quarantine:{test_hash}"]`. The second build uses check-before-create (searches
for an existing open issue with matching deterministic label
`quarantine:{test_hash}`) and finds the issue already exists, so it skips issue
creation. Both builds succeed without duplicate issues.

**Note:** There is a small race window between the dedup search returning
"no issue" and the POST to create one. In rare cases, both builds may create
issues. This is accepted per ADR-012 — a human closes the duplicate, and the
next build finds the first issue.

---

### Scenario 28: Concurrent CI builds update quarantine.json simultaneously (CAS conflict) [M4]

**Given** two CI builds (Build A and Build B) are running in parallel. Both have
fetched `quarantine.json` at SHA `abc123` from the `quarantine/state` branch.

**When** Build A writes its update to `quarantine.json` first (new SHA
`def456`), and then Build B attempts to write its update using the stale SHA
`abc123`

**Then** Build B's write fails with a 409 Conflict from the GitHub Contents API
because the SHA no longer matches (ADR-006, ADR-012). Build B:
1. Re-fetches `quarantine.json` at the new SHA `def456`.
2. Merges its changes with Build A's changes using the union strategy
   (quarantine wins — per ADR-012).
3. Retries the write with the updated SHA.
4. The resulting `quarantine.json` contains both builds' updates without data
   loss.

If all 3 retry attempts fail (409 each time), Build B logs a warning and skips
the write. The flaky test will be re-detected on the next CI run. This is safe
because re-detection is cheap.

---

### Scenario 29: Concurrent quarantine and unquarantine race [M4]

**Given** `quarantine.json` contains an entry for
`CacheService > should handle eviction` with an open GitHub Issue. Two CI builds
(Build A and Build B) are running in parallel. Build A detects a new flaky test
`ApiService > should handle timeout` while Build B finds that the issue for
`should handle eviction` has been closed and removes it from its local copy of
`quarantine.json`.

**When** Build A writes first (adding `should handle timeout`, keeping
`should handle eviction`), and Build B attempts to write (removing
`should handle eviction`) using a stale SHA, triggering a 409 Conflict

**Then** Build B re-reads `quarantine.json`, merges using the quarantine-wins
(union) strategy per ADR-012, and the resulting `quarantine.json` contains both
`should handle eviction` and `should handle timeout`. Build B logs:
`[quarantine] WARNING: Test 'CacheService > should handle eviction' was
unquarantined (issue closed) but re-quarantined due to concurrent update. It
will be unquarantined on the next build.`

On the very next CI run, the CLI checks the issue status, finds it closed, and
removes `should handle eviction` from `quarantine.json`. The impact is one extra
build cycle where the test remains quarantined — excluded from execution
(Jest/Vitest) or failure-suppressed (RSpec) rather than running and potentially
failing. This is the safe default.

---

## Degraded Mode

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

## Dashboard

### Scenario 36: User views org-wide flaky test overview [M7]

**Given** the user is viewing the dashboard and the `dashboard.yml`
configuration includes 4 repositories with Quarantine configured, containing a
combined 12 quarantined tests

**When** the user navigates to the org-level overview page

**Then** the dashboard displays a summary showing total quarantined tests across
all repos (12), a breakdown per repository with test counts, the most recently
quarantined tests, and links to drill into each project's details.

---

### Scenario 37: User views single project's flaky test details and trends [M7]

**Given** the user selects the repository `acme/payments-service`, which has
3 quarantined tests

**When** the project detail page loads

**Then** the dashboard displays a list of all 3 quarantined tests with their
names, date first quarantined, last flaky occurrence, links to their
corresponding GitHub Issues, and a trend chart showing flaky test count over
time (data derived from ingested GitHub Artifacts history).

---

### Scenario 38: User filters and searches quarantined tests on dashboard [M7]

**Given** the user is viewing a repository with 15 quarantined tests

**When** the user types `timeout` into the search bar and selects the filter
`Status: Still Failing`

**Then** the dashboard filters the list to show only quarantined tests whose
names contain `timeout` and whose most recent run result was a failure,
updating the displayed count accordingly.

---

### Scenario 39: Dashboard polls artifacts and ingests new results [M6]

**Given** the dashboard is running with `dashboard.yml` configured as:
```yaml
source: manual
repos:
  - owner: mycargus
    repo: my-app
poll_interval: 300
```
and its last successful poll was 5 minutes ago

**When** the background polling interval elapses

**Then** the dashboard:
1. Queries the GitHub Artifacts API for `mycargus/my-app` with
   `If-None-Match` (ETag) header for conditional requests.
2. Filters artifacts by name prefix `quarantine-results`.
3. Downloads any new artifacts (follows 302 redirect to blob storage).
4. Extracts zip, parses JSON, validates against
   `schemas/test-result.schema.json`.
5. Upserts into SQLite (keyed by `run_id` for idempotency).
6. If the ETag matches (304 Not Modified), skips processing — this does not
   count against the rate limit.

---

### Scenario 40: Dashboard circuit breaker pauses polling after failures [M6]

**Given** the dashboard is polling a repository and 3 consecutive GitHub API
calls have failed (e.g., 500 Internal Server Error)

**When** the circuit breaker threshold (3 consecutive failures) is reached

**Then** the dashboard pauses polling for that repository for 30 minutes. After
the pause, the dashboard resumes polling. On the first successful poll, the
circuit breaker resets. Per ADR-015.

---

## Branch Protection

### Scenario 41: CLI updates quarantine.json on unprotected branch [M4]

**Given** the `quarantine/state` branch is not protected, and the CLI has
detected a new flaky test

**When** the CLI writes the updated `quarantine.json` to the `quarantine/state`
branch via the GitHub Contents API

**Then** the write succeeds directly via the Contents API PUT with SHA-based
optimistic concurrency, and `quarantine.json` is updated.

---

### Scenario 42: CLI updates quarantine.json when branch is protected [M4]

**Given** the `quarantine/state` branch has branch protection rules enabled
(e.g., required reviews, status checks), and the CLI has detected a new flaky
test

**When** the CLI attempts to write `quarantine.json` via the Contents API and
receives a 403 or 422 error indicating the branch is protected

**Then** the CLI falls back to storing the pending quarantine state update in
the GitHub Actions cache (keyed by run ID), logs:
`[quarantine] WARNING: Branch 'quarantine/state' is protected. Quarantine state
saved to Actions cache. A workflow with write access must apply the update.`
The CI build still exits with code 0 (the flaky test is treated as quarantined
for this run based on the pending update).

---

## CLI Flags & Configuration

### Scenario 43: User overrides framework in quarantine.yml [M2]

**Given** the project contains both `jest.config.js` and `vitest.config.ts`
files, and `quarantine.yml` has `framework: vitest`

**When** the developer runs
`quarantine run --retries 3 -- vitest run --reporter=junit`

**Then** the CLI uses `vitest` as the framework (per the config), parses
Vitest-formatted JUnit XML output, and uses Vitest-specific rerun commands for
retries. The `framework` field in config is the source of truth — there is no
auto-detection (ADR-010, amended).

---

### Scenario 44: User customizes retry count [M3]

**Given** `quarantine.yml` has `retries: 5`

**When** the developer runs `quarantine run -- jest --ci ...` (no `--retries`
flag)

**Then** the CLI reads `retries: 5` from config and retries each failing test up
to 5 times. Config resolution order: CLI flags > config values > auto-detected >
defaults (per docs/cli-spec.md).

**When** the developer runs `quarantine run --retries 2 -- jest --ci ...`

**Then** the CLI flag overrides the config: retries only 2 times.

---

### Scenario 45: --dry-run flag [M4]

**Given** the CLI is configured in CI and would normally detect a flaky test,
update `quarantine.json`, and create an issue

**When** the developer runs
`quarantine run --dry-run --retries 3 -- jest --ci ...`

**Then** the CLI runs tests, parses XML, detects the flaky test, but does NOT:
- Write to `quarantine.json`
- Create GitHub Issues
- Post PR comments

Instead, it prints a summary of what would have been done:
```
[quarantine] DRY RUN — no changes written.
  Would quarantine: PaymentService > should handle charge timeout
  Would create issue: [Quarantine] PaymentService > should handle charge timeout
  Would post PR comment on PR #42
```
Writes results to `.quarantine/results.json` (results are always written).
Exits with code 0.

---

### Scenario 46: --exclude patterns [M4]

**Given** `quarantine.yml` has:
```yaml
exclude:
  - "test/integration/**"
```
and the developer also passes `--exclude "**::SlowServiceTest::*"`

**When** a test matching `test/integration/api_test.js::ApiTest::should connect`
fails on the first run

**Then** the CLI merges exclude patterns from config and CLI flags. The test
matches the config pattern `test/integration/**` (matched against `test_id`
using glob syntax). Quarantine ignores it entirely — no retry, no quarantine,
no issue creation. It behaves as if quarantine is not installed for that test.
The test's failure still affects the exit code normally (exit 1 if it fails).

---

### Scenario 47: --pr flag override and auto-detection [M5]

**Given** the CLI is running in GitHub Actions on a PR build

**When** the developer runs `quarantine run --pr 99 -- jest --ci ...`

**Then** the CLI uses PR number 99 (from the `--pr` flag) instead of
auto-detecting from `GITHUB_EVENT_PATH`. Posts the quarantine comment on PR #99.

**When** the developer runs `quarantine run -- jest --ci ...` (no `--pr` flag)
and `GITHUB_EVENT_PATH` is set to a JSON file containing
`{"pull_request": {"number": 42}}`

**Then** the CLI auto-detects PR number 42 from the event JSON and posts the
comment there.

**When** the developer runs `quarantine run -- jest --ci ...` on a non-PR build
(e.g., push to main) with no `--pr` flag and `GITHUB_EVENT_PATH` does not
contain a `pull_request` field

**Then** the CLI skips posting a PR comment. No error — PR comments are
best-effort.

---

### Scenario 48: PR comment suppressed via config [M5]

**Given** `quarantine.yml` has:
```yaml
notifications:
  github_pr_comment: false
```

**When** the CLI detects a flaky test during a PR build

**Then** the CLI does NOT post or update a PR comment, even though a PR number
is available. All other behavior (quarantine state update, issue creation,
results file) proceeds normally.

---

### Scenario 49: PR comment updated on second run [M5]

**Given** the CLI previously posted a PR comment on PR #42 containing the
`<!-- quarantine-bot -->` HTML marker

**When** a second CI run on the same PR detects additional flaky tests

**Then** the CLI:
1. Lists comments on PR #42
   (`GET /repos/{owner}/{repo}/issues/42/comments?per_page=100`).
2. Scans for the `<!-- quarantine-bot -->` marker.
3. Finds the existing comment and updates it via
   `PATCH /repos/{owner}/{repo}/issues/comments/{comment_id}` with the new
   summary (replacing the old content entirely).
4. No duplicate comments are created.

---

### Scenario 50: Custom rerun_command template [M3]

**Given** `quarantine.yml` has:
```yaml
framework: jest
rerun_command: "npx jest --testNamePattern '{name}' --config jest.ci.config.js"
```

**When** the CLI detects a failing test `should handle timeout` and needs to
retry it

**Then** the CLI uses the custom `rerun_command` template instead of the
auto-detected Jest rerun command. It substitutes `{name}` with the test name:
`npx jest --testNamePattern 'should handle timeout' --config jest.ci.config.js`.
The `{classname}` and `{file}` placeholders are also available for templates
that need them.

---

### Scenario 51: --verbose and --quiet flags [M2]

**Given** the CLI is configured in CI

**When** the developer runs `quarantine run --verbose -- jest --ci ...`

**Then** the CLI outputs detailed information: API calls made, retry attempts
with outcomes, config resolution trace, quarantine state details, and timing
information. Test runner output is passed through unmodified.

**When** the developer runs `quarantine run --quiet -- jest --ci ...`

**Then** the CLI outputs only warnings and errors. Test runner output is passed
through unmodified. No `[quarantine] Reading quarantine state... OK` lines.

**When** the developer runs `quarantine run --verbose --quiet -- jest --ci ...`

**Then** the CLI prints an error:
`Error: --verbose and --quiet are mutually exclusive.`
Exits with code 2.

---

## Test Runner Edge Cases

### Scenario 52: quarantine run without -- separator [M2]

**Given** the CLI is configured in CI

**When** the developer runs `quarantine run jest --ci` (missing `--` separator)

**Then** the CLI prints:
```
Error: missing '--' separator. Usage: quarantine run [flags] -- <test command>

Example: quarantine run --retries 3 -- jest --ci
```
Exits with code 2. The test command is NOT executed.

---

### Scenario 53: Test command not found [M2]

**Given** the CLI is configured in CI and `quarantine.yml` is valid

**When** the developer runs `quarantine run -- jset --ci` (typo: `jset` instead
of `jest`) and the command is not found on PATH

**Then** the CLI prints:
```
Error: command not found: "jset". Ensure the test runner is installed and on PATH.
```
Exits with code 2. No tests ran — this is a quarantine error, not a test
failure.

---

### Scenario 54: No JUnit XML produced [M2]

**Given** the CLI is configured in CI with `junitxml: junit.xml` and the test
runner crashes before producing XML output (e.g., segfault, OOM, or the test
command doesn't produce JUnit XML)

**When** the developer runs `quarantine run -- jest --ci` and the test command
exits non-zero, and no file matches the `junit.xml` glob

**Then** the CLI logs:
`[quarantine] WARNING: No JUnit XML found at 'junit.xml'. Cannot determine test
results. Suggest checking --junitxml flag or jest-junit configuration.`
Exits with the test runner's exit code (since the CLI cannot determine whether
the failure was a test failure or infrastructure issue).

---

### Scenario 55: Malformed JUnit XML [M2]

**Given** a single JUnit XML file exists but is truncated or contains invalid
XML

**When** the CLI attempts to parse it after the test run

**Then** the CLI logs:
`[quarantine] WARNING: Failed to parse junit.xml: XML syntax error at line 42.
Skipping.`
Treats this as "no XML produced" and exits with the test runner's exit code.

---

### Scenario 56: Multiple XML files, some malformed (parallel runners) [M2]

**Given** the project uses Jest with `--shard` and produces 4 JUnit XML files.
3 are valid and 1 is truncated.

**When** the CLI parses the XML files matching the glob pattern

**Then** the CLI:
1. Parses all 4 files. Logs a warning for the malformed one:
   `[quarantine] WARNING: Failed to parse results/shard-3.xml: unexpected EOF.
   Skipping.`
2. Merges results from the 3 valid files.
3. Logs: `[quarantine] Parsed 3/4 JUnit XML files. 1 malformed, skipped.`
4. Proceeds with flaky detection and quarantine logic using the partial results.
5. Exits based on the partial results (correct: partial results better than
   none).

---

### Scenario 57: All tests in the suite are quarantined — Jest/Vitest [M4]

**Given** `quarantine.json` contains entries for every test in the suite (e.g.,
50 out of 50 tests are quarantined), and all corresponding GitHub Issues are
open. The project uses Jest.

**When** CI executes `quarantine run -- jest --ci ...`

**Then** the CLI constructs exclusion flags that exclude all 50 tests from
execution. The test runner executes 0 tests. Jest exits non-zero with "No tests
found." The CLI detects this condition: the JUnit XML contains zero test cases
(or no XML is produced) and the number of quarantined exclusions equals or
exceeds the expected test count. The CLI treats this as a successful run. Posts a
PR comment:
`All 50 tests in this suite are currently quarantined. No tests were executed.
Consider reviewing quarantined tests and closing resolved issues.`
Results artifact contains `summary.total: 0`, `summary.quarantined: 50`.
Logs to stderr:
`[quarantine] WARNING: All tests excluded by quarantine. No tests executed.`
Exits with code 0.

---

### Scenario 58: All tests in the suite are quarantined — RSpec [M4]

**Given** `quarantine.json` contains entries for every test in the suite (e.g.,
50 out of 50 tests), all issues open. The project uses RSpec.

**When** CI executes `quarantine run -- rspec ...`

**Then** because RSpec uses post-execution filtering (not pre-execution
exclusion), all 50 tests still run. If any fail, their failures are suppressed
from the exit code (all are quarantined). The CLI posts a PR comment noting all
50 tests are quarantined and suggests reviewing. Exits with code 0.

---

## GitHub API Edge Cases

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

## Configuration Edge Cases

### Scenario 64: Config resolution order [M2]

**Given** `quarantine.yml` has `retries: 5` and `junitxml: "custom/*.xml"`.
The `origin` git remote points to `github.com/my-org/my-project`.

**When** the developer runs
`quarantine run --retries 2 --junitxml "override.xml" -- jest --ci`

**Then** the CLI applies config resolution in priority order:
1. CLI flags: `retries: 2`, `junitxml: "override.xml"` (highest priority)
2. Config file: `retries: 5`, `junitxml: "custom/*.xml"` (overridden by flags)
3. Auto-detected: `github.owner: my-org`, `github.repo: my-project`
4. Defaults: `storage.branch: quarantine/state`, etc. (lowest priority)

Result: retries=2, junitxml="override.xml", github.owner="my-org".

---

### Scenario 65: Minimal valid config [M1]

**Given** `quarantine.yml` contains only:
```yaml
version: 1
framework: jest
```

**When** the developer runs `quarantine doctor`

**Then** the CLI applies all defaults:
- `retries: 3`
- `junitxml: junit.xml` (Jest framework default)
- `issue_tracker: github`
- `labels: [quarantine]`
- `notifications.github_pr_comment: true`
- `storage.branch: quarantine/state`
- `github.owner` and `github.repo` auto-detected from git remote

Prints the resolved configuration. Exits with code 0.

---

### Scenario 66: Unsupported config version [M1]

**Given** `quarantine.yml` contains `version: 2`

**When** the developer runs `quarantine doctor`

**Then** the CLI prints:
```
Error: Unsupported config version: 2. This version of the CLI supports version 1.
```
Exits with code 2.
