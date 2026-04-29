# Implementation Milestones

> Last updated: 2026-03-17
>
> Pre-implementation task 3. Defines buildable, testable, demoable increments
> for v1 with clear dependency order.
>
> Related docs:
> - `docs/specs/cli-spec.md` -- CLI interface specification
> - `docs/specs/config-schema.md` -- quarantine.yml full schema
> - `docs/specs/github-api-inventory.md` -- GitHub API interactions
> - `docs/specs/error-handling.md` -- error handling strategy
> - `docs/specs/architecture.md` -- system design

## Design Principles

- **Small scope per milestone.** Each milestone is the smallest useful
  increment that can be built, tested, and demoed independently.
- **Each milestone is buildable, testable, demoable.** No milestone produces
  only internal scaffolding without a visible artifact.
- **Minimum viable demo: M5.** The first milestone where the tool is visible
  to stakeholders without reading CI logs (GitHub Issues + PR comments).
- **Phase 2 enables parallel development.** The CLI agent and dashboard agent
  work independently against a shared JSON schema contract.

## Milestone Overview

```
Phase 1 -- Foundation (sequential, single agent: cli-dev)
  M1: CLI scaffolding + quarantine init
  M2: Test execution + JUnit XML parsing
  M3: Flaky detection via retry

Phase 2 -- Parallel development (two agents)
  Agent A (cli-dev):
    M4: Quarantine state + exclusion
    M5: GitHub Issues + PR comments + result artifacts

  Agent B (dashboard-dev):
    M6: Dashboard scaffolding + data ingestion

Phase 3 -- Integration and polish (sequential)
  M7: Dashboard analytics and UI
  M8: Polish and hardening

Phase 4 -- Multi-Suite Conversion (depends on M1-M8 verified)
    M9: Core conversion. Config (.quarantine/config.yml, test_suites array,
          junitxml/rerun_command always required), run [suite-name], per-suite
          state/notifications, init with framework detection, delete old format.
    M10: Additive features. suite list/remove, quarantine status, timeouts,
          quarantined-files.txt, dashboard migration, schema updates.
    See ADR-010, ADR-030, ADR-031, ADR-032.

Phase 5 -- Init UX improvements (depends on M9)
  M11: Init UX improvements

Phase 6 -- GitHub App Integration (v2, depends on M10 complete)
  Prerequisite: register dev GitHub App (M16 one-time setup step 1)
  Sequential:
    M12: Dashboard App auth foundation (JWT + InstallationTokenProvider + rate limit monitoring + contract tests)
    M13: Dashboard OAuth login (@remix-run/auth, sessions, route protection, rate limiting) (ADR-029) (depends on M12)
    M14: Installation discovery (depends on M13)
    M15: github-app mode integration + user permission filtering (depends on M14)
    M16: E2E integration + CI (depends on M15)

Phase 6b -- Server-Side Writes (v2, depends on M16; ADR-036 amendment)
  Parallel (disjoint codebases: Go CLI vs TypeScript dashboard):
    M17: CLI v2 read-only mode (GITHUB_TOKEN for reads, no writes, fork PR detection)
    M18: Dashboard write processing (state updates, issue creation, PR comments from artifacts)
  Sequential (depends on M17 + M18):
    M19: v2 server-side writes E2E

Phase 7 -- Webhooks (v3, depends on M16 and public endpoint ADR)
  Immediate feedback: workflow_run.completed triggers instant artifact
    processing (state + issues + PR comments within ~30s)
  issue.closed, installation lifecycle, state consolidation webhooks
    (see docs/plans/webhooks.md)
```

---

## Phase 1 -- Foundation

Phase 1 is sequential. Each milestone builds on the prior one. A single agent
(`cli-dev`) owns all three milestones.

---

### M1: CLI Scaffolding + `quarantine init`

**Owner:** `cli-dev`

**Dependencies:** None (first milestone).

**Scope -- included:**

- Go project scaffolding: `cobra` CLI framework, Go module
  (`github.com/mycargus/quarantine`), Makefile with `cli-build`, `cli-test`,
  `cli-lint` targets.
- Directory structure: `cli/cmd/quarantine/main.go`, `cli/internal/config/`,
  `cli/internal/github/`.
- `quarantine init` command:
  - Interactive prompts: framework (required, no default), retries (default 3),
    JUnit XML path (framework-specific default).
  - Creates `quarantine.yml` in the current directory with prompted values.
  - Validates GitHub token (`QUARANTINE_GITHUB_TOKEN`, falls back to
    `GITHUB_TOKEN`). Errors with exit 2 if neither is set.
  - Tests repository read/write access via GitHub API.
  - Creates `quarantine/state` branch with empty `quarantine.json`
    (`{"version":1,"updated_at":"...","tests":{}}`). Skips if branch exists.
  - Jest-specific guidance: prints recommended `jest-junit` configuration.
  - Prints summary and next steps (workflow snippet).
  - Exit codes: 0 = success, 2 = failure.
  - `init` does NOT use degraded mode. Failures are fatal with diagnostics.
- `quarantine doctor` command:
  - Reads and validates `quarantine.yml` against config schema rules.
  - Resolves auto-detected values (`github.owner`, `github.repo` from git
    remote; framework-specific `junitxml` default).
  - Checks forward-compatible field restrictions (v1: `issue_tracker: github`,
    `labels: [quarantine]`, `notifications.github_pr_comment` only).
  - Warns if GitHub token is missing from environment.
  - Prints resolved configuration on success, errors and warnings on failure.
  - Exit codes: 0 = valid, 2 = invalid.
  - Supports `--config PATH` flag.
- `quarantine version` command: prints `quarantine v{version}` to stdout,
  exits 0.
- Config parsing: `quarantine.yml` parser in `cli/internal/config/` with full
  validation per `docs/specs/config-schema.md`.
- GitHub client foundation: authenticated HTTP client with token resolution,
  `User-Agent: quarantine-cli/{version}` header, 10-second timeout.
- GitHub API interactions used in this milestone:
  - `GET /repos/{owner}/{repo}` (verify repo access during init).
  - `GET /repos/{owner}/{repo}/git/ref/heads/{default_branch}` (get HEAD SHA
    for branch creation).
  - `POST /repos/{owner}/{repo}/git/refs` (create `quarantine/state` branch).
  - `PUT /repos/{owner}/{repo}/contents/quarantine.json` (create initial empty
    file on the new branch).

**Scope -- explicitly excluded:**

- No `quarantine run`. No test execution.
- No JUnit XML parsing.
- No retry logic.
- No quarantine state read/write (beyond initial creation during init).
- No GitHub Issues, PR comments, or artifact upload.
- No `--yes` flag for non-interactive init (v2).

**Acceptance criteria:**

1. `quarantine init` interactively creates a valid `quarantine.yml` and
   `quarantine/state` branch with empty `quarantine.json`.
2. `quarantine doctor` reports errors for invalid config and prints resolved
   config for valid config.
3. `quarantine version` prints the version string.
4. `make cli-build` produces a binary. `make cli-test` passes. `make cli-lint`
   passes.
5. Unit tests cover: config parsing and validation (all fields, all error
   cases), token resolution, git remote parsing.
6. E2E test: `quarantine init` against a real GitHub repo creates the
   branch and file.

**Key implementation notes:**

- Override cobra's default exit-2 for usage errors via
  `cmd.SetFlagErrorFunc()` per `docs/specs/error-handling.md`.
- The `framework` field is required with no auto-detection. `quarantine init`
  sets it via interactive prompt.
- Token is never stored in `quarantine.yml`. Auth is environment variables
  only.
- Error messages for `init` failures must be actionable (e.g., "GitHub token
  lacks permission to create branches. Required scope: repo.").

---

### M2: Test Execution + JUnit XML Parsing

**Owner:** `cli-dev`

**Dependencies:** M1 (config parsing, project scaffolding).

**Scope -- included:**

- `quarantine run -- <command>` executes the user's test command as a
  subprocess.
- The `--` separator is required. If omitted, prints usage error and exits 2.
- Prerequisite check: verifies `quarantine/state` branch exists. If not,
  prints "Run 'quarantine init' first." and exits 2.
- Reads `quarantine.yml` (or `--config PATH`). Merges CLI flag overrides per
  config resolution order (flags > config > auto-detected > defaults).
- Parses JUnit XML output after the test command completes:
  - Resolves `junitxml` glob pattern. Supports single file and multiple files
    (parallel runners).
  - Handles framework-specific XML variations (Jest/jest-junit, RSpec/
    rspec_junit_formatter, Vitest built-in).
  - Constructs `test_id` as `file_path::classname::name` per
    `docs/specs/cli-spec.md`.
- Detects failures from parsed XML.
- Writes result JSON to `.quarantine/results.json` (creates directory if
  needed). Result format follows `schemas/test-result.schema.json`.
- Exit codes:
  - 0 = all tests passed.
  - 1 = test failures exist.
  - 2 = quarantine error (not initialized, bad command, missing XML).
- Test runner error handling per `docs/specs/error-handling.md` Category 2:
  - Command not found: exit 2 with diagnostic.
  - Non-zero exit + valid XML: exit based on parsed results.
  - No XML produced: exit with runner's exit code, log warning.
  - Malformed XML: log warning, treat as no results.
  - Multiple XML files, some malformed: parse what is possible, proceed with
    valid files.
- Flags implemented in this milestone: `--config`, `--junitxml`, `--verbose`,
  `--quiet`. Mutual exclusion enforced for `--verbose`/`--quiet`.
- Signal handling: forwards SIGINT/SIGTERM to the child test process.
- Test fixtures (`testdata/`) used for parser unit tests.

**Scope -- explicitly excluded:**

- No retry. A failing test is reported as failed, not retried.
- No quarantine state read (beyond the init check). No exclusions.
- No GitHub API calls during `quarantine run` (beyond the branch existence
  check).
- No GitHub Issues, PR comments, or state updates.
- No `--retries`, `--dry-run`, `--strict`, `--pr`, `--exclude` flags.
- No exclude pattern matching.

**Acceptance criteria:**

1. `quarantine run -- jest --ci` executes Jest, parses JUnit XML, and writes
   `results.json` with correct test counts.
2. `quarantine run -- rspec` executes RSpec and parses its XML output.
3. `quarantine run -- vitest run` executes Vitest and parses its XML output.
4. Exit 0 when all tests pass. Exit 1 when any test fails.
5. Exit 2 when `quarantine init` has not been run.
6. Glob patterns for `junitxml` resolve multiple files and merge results.
7. Malformed XML produces a warning and the CLI exits with the runner's code.
8. Unit tests cover: JUnit XML parsing for all three frameworks (using test
   fixtures), test_id construction, result JSON serialization, exit code
   determination.

**Key implementation notes:**

- The branch existence check is a lightweight `GET /repos/{owner}/{repo}/
  git/ref/heads/quarantine/state`. A 404 means not initialized.
- JUnit XML has no official schema. Test against real framework output using
  fixtures from `testdata/`.
- `file_path` extraction is framework-specific. See `docs/specs/cli-spec.md`
  "Framework-Specific `file_path` Extraction" table.
- Result JSON must include all metadata needed for dashboard ingestion
  (commit SHA, branch, PR number, timestamp, CLI version).

---

### M3: Flaky Detection via Retry

**Owner:** `cli-dev`

**Dependencies:** M2 (test execution, XML parsing).

**Scope -- included:**

- Re-runs individual failing tests using framework-specific rerun commands:
  - Jest: `jest --testNamePattern "{name}"`
  - RSpec: `rspec -e "{name}"`
  - Vitest: `vitest run --reporter=junit {file} -t "{name}"`
- Configurable retry count: `--retries N` flag (range 1-10, overrides config).
  Default from `quarantine.yml` or 3.
- Flaky classification: a test that fails initially but passes on any retry
  is classified as flaky.
- Genuine failure classification: a test that fails all retries is a genuine
  failure.
- Exit code reflects real failures only: flaky tests = pass (exit 0), genuine
  failures = exit 1.
- `rerun_command` config override: when set in `quarantine.yml`, uses the
  custom template with `{name}`, `{classname}`, `{file}` placeholders instead
  of the framework-specific default.
- Updated result JSON: includes retry details per test (attempt number,
  status, duration).
- Informational output: logs each retry attempt with test name, attempt
  number, and result.

**Scope -- explicitly excluded:**

- No quarantine state read/write. No test exclusions. Flaky tests are detected
  and reported but not persisted to `quarantine.json`.
- No GitHub API calls (beyond the init check from M2).
- No GitHub Issues, PR comments.
- No `--dry-run`, `--strict`, `--pr`, `--exclude` flags.
- No exclude pattern matching.

**Acceptance criteria:**

1. A known-flaky test (fails first, passes on retry) is detected and reported
   as flaky. CLI exits 0.
2. A genuinely failing test (fails all retries) is reported as a genuine
   failure. CLI exits 1.
3. `--retries 1` re-runs each failure once. `--retries 5` re-runs up to 5
   times.
4. Framework-specific rerun commands work for Jest, RSpec, and Vitest.
5. Custom `rerun_command` in config is used when set.
6. Result JSON includes retry details for each retried test.
7. Unit tests cover: flaky classification logic, rerun command construction
   for all three frameworks, placeholder substitution, retry count bounds
   validation.

**Key implementation notes:**

- Rerun commands run the test in isolation. Jest `--testNamePattern` uses regex
  matching -- special characters in test names must be escaped.
- RSpec `-e "{name}"` matches against `full_description` and may match
  multiple tests with similar names. This is a known limitation documented in
  `docs/specs/cli-spec.md`.
- The retry loop should exit early on the first passing attempt (no need to
  retry further once flakiness is confirmed).

---

## Phase 2 -- Parallel Development

Phase 2 enables two agents to work independently. Agent A (`cli-dev`)
continues CLI development with GitHub integration. Agent B (`dashboard-dev`)
starts dashboard development using test fixtures and the shared JSON
schema contract.

**Why this works:** Agent B develops against fixture data from `testdata/` and
validates against `schemas/test-result.schema.json`. It does not need a working
CLI or real GitHub artifacts. The two components are integrated in Phase 3.

**Contract tests:** Both agents validate their output/input against the same
JSON schemas (`test-result.schema.json`, `quarantine-state.schema.json`). This
ensures compatibility when integrated.

---

### Agent A (cli-dev)

---

### M4: Quarantine State + Exclusion

**Owner:** `cli-dev`

**Dependencies:** M3 (flaky detection, retry logic).

**Scope -- included:**

- Reads `quarantine.json` from `quarantine/state` branch via GitHub Contents
  API at the start of `quarantine run`.
- Writes updated `quarantine.json` with optimistic concurrency (SHA-based
  compare-and-swap). Retry up to 3 times on 409 conflict with re-read and
  merge. Merge semantics per `docs/specs/error-handling.md`: union of test sets,
  quarantine wins on conflict.
- Batch check issue status via GitHub Search API: one call returns all closed
  issues with `quarantine` label. Tests whose issues are closed are
  unquarantined (removed from `quarantine.json`).
- Excludes quarantined tests from execution via framework-specific flags:
  - Jest: `--testPathIgnorePatterns` for full-file exclusion,
    `--testNamePattern` with negative lookahead for partial-file exclusion.
  - RSpec: file-level exclusion by omitting files from the rspec command.
    Individual test exclusion deferred (run full file, rely on retry for
    partial-file cases).
  - Vitest: `--exclude` for file-level, `-t` with negative lookahead for
    partial-file.
- GitHub Actions cache fallback for degraded mode: when Contents API read
  fails, attempt `actions/cache` read (key: `quarantine-state-latest`).
- Degraded mode: when quarantine state is unavailable, all tests run (no
  exclusions). Flaky detection via retry still works. Warnings logged to
  stderr. GHA `::warning` annotation when `GITHUB_ACTIONS` is set.
- `--strict` mode: infrastructure errors cause exit 2 instead of degraded
  mode.
- `--dry-run` flag: runs tests and parses XML but does not update
  `quarantine.json`. Prints summary of what would have been done.
- `--exclude PATTERN` flag: additional exclude patterns merged with config
  `exclude` field. Patterns match against `test_id` using glob syntax.
- Exclude pattern matching per `docs/specs/cli-spec.md`: `*`, `**`, `?` supported.
  Excluded tests are ignored entirely (no retry, no quarantine, no issue).
- Rate limit header tracking: read `X-RateLimit-Remaining` after every API
  call, warn when below 10% of limit.
- Updated result JSON: includes quarantine state details (excluded tests,
  unquarantined tests).
- GitHub API interactions used in this milestone:
  - `GET /repos/{owner}/{repo}/contents/quarantine.json?ref=quarantine/state`
    (read state).
  - `PUT /repos/{owner}/{repo}/contents/quarantine.json` (write state with
    CAS).
  - `GET /search/issues?q=repo:{owner}/{repo}+is:issue+is:closed+label:quarantine`
    (batch check closed issues).
- Error handling per `docs/specs/error-handling.md` and
  `docs/specs/github-api-inventory.md`: 401, 403, 404, 409, 422, 429, 5xx,
  timeout.

**Scope -- explicitly excluded:**

- No GitHub Issue creation. Flaky tests are added to `quarantine.json` but no
  tracking issue is created yet.
- No PR comments.
- No `--pr` flag.

**Acceptance criteria:**

1. `quarantine.json` reflects newly detected flaky tests after a run.
2. Quarantined tests are excluded from execution (they do not appear in test
   runner output or JUnit XML).
3. Closing a quarantine issue (manually, for testing) causes the test to be
   unquarantined on the next run.
4. CAS conflict: when two concurrent writes conflict, the merge produces the
   union of quarantined tests.
5. Degraded mode: when GitHub API is unreachable, all tests run and the CLI
   exits based on test results only.
6. `--strict`: infrastructure errors cause exit 2.
7. `--dry-run`: no state changes written.
8. `--exclude`: matching tests are ignored by quarantine.
9. Unit tests cover: quarantine state read/write/merge, CAS retry logic,
   framework-specific exclusion flag construction, exclude pattern matching,
   degraded mode behavior, exit code determination with quarantine state.
10. E2E test: full flow against a real GitHub repo with a known-flaky
    test.

**Key implementation notes:**

- The merge function (`merge(local, remote) -> merged`) is the core of
  concurrency safety. Test it extensively.
- Search API has a separate rate limit: 30 req/min for authenticated users.
  The CLI makes at most 1 search call per run (plus pagination).
- Pagination: if `total_count` > 100, paginate closed-issue search.
- `incomplete_results: true` in search response means results may be truncated
  by GitHub's search index lag. Log a warning.

---

### M5: GitHub Issues + PR Comments + Result Artifacts

**Owner:** `cli-dev`

**Dependencies:** M4 (quarantine state, exclusion, GitHub client).

**Scope -- included:**

- Creates GitHub Issues for newly detected flaky tests:
  - Dedup check via Search API: `repo:{owner}/{repo} is:issue is:open
    label:quarantine label:quarantine:{test_hash}`.
  - If no existing issue found, creates one with title
    `[Quarantine] {test_name}`, body with test details and retry results,
    labels `["quarantine", "quarantine:{test_hash}"]`.
  - `test_hash` is a deterministic identifier derived from `test_id`.
  - GitHub auto-creates labels on first use. No separate label creation call.
  - Race condition on dedup is accepted (duplicate issue closed by human).
  - **Issue creation scope (ADR-022):** Issues are only created when the flaky
    test already exists on the base branch. Two-step detection:
    (a) `git fetch origin ${GITHUB_BASE_REF} --depth=1` to ensure the base ref
    is available (shallow clone protection).
    (b) If the test file is new (`git diff --name-only --diff-filter=A`), all
    tests in it are new → skip issue creation.
    (c) If the file pre-exists, search `git diff origin/${GITHUB_BASE_REF} -- {file}`
    added lines for the test name. If found → test case is new to PR → skip
    issue creation.
    In both skip cases, `quarantine.json` is not updated, no issue is created,
    but a PR comment warns the developer. Fallback: if `GITHUB_BASE_REF` is
    unset, `git fetch` fails, or the diff cannot be computed, treat the test as
    pre-existing (create issue).
- Posts or updates PR comments:
  - PR number auto-detected from `GITHUB_EVENT_PATH` (GitHub Actions). `--pr`
    flag overrides.
  - Comment identified by `<!-- quarantine-bot -->` HTML marker (first line).
  - Lists existing PR comments, scans for marker. If found, updates via PATCH.
    If not found, creates via POST.
  - Comment template per `docs/specs/cli-spec.md` "PR Comment Template": summary
    table, conditional sections for flaky, quarantined, unquarantined,
    failures.
  - Skipped when `notifications.github_pr_comment: false` in config.
  - Skipped when no PR number is available (non-PR builds).
- `--pr N` flag: override PR number.
- Result JSON includes all metadata for dashboard ingestion: commit SHA,
  branch, PR number, timestamp, CLI version, summary counts, per-test
  results with retry details, quarantine state, issue numbers and URLs.
- Workflow snippet documentation for `actions/upload-artifact`:
  ```yaml
  - name: Upload quarantine results
    if: always()
    uses: actions/upload-artifact@v4
    with:
      name: quarantine-results-${{ github.run_id }}
      path: .quarantine/results.json
  ```
  Matrix-safe naming: `quarantine-results-${{ github.run_id }}-${{ matrix.key }}`
- `--exclude PATTERN` interaction with issue creation: excluded tests never
  trigger issue creation.
- GitHub API interactions added in this milestone:
  - `GET /search/issues` (dedup check for existing issue).
  - `POST /repos/{owner}/{repo}/issues` (create issue).
  - `GET /repos/{owner}/{repo}/issues/{pr_number}/comments` (list PR
    comments to find existing quarantine comment).
  - `POST /repos/{owner}/{repo}/issues/{pr_number}/comments` (post new PR
    comment).
  - `PATCH /repos/{owner}/{repo}/issues/comments/{comment_id}` (update
    existing PR comment).
- Error handling for all new API interactions per
  `docs/specs/github-api-inventory.md`: dedup search failure falls through to
  create (worst case: duplicate issue). PR comment failure is best-effort
  (no impact on quarantine correctness). Issue creation failure logs warning,
  entry written to `quarantine.json` without `issue_number`.

**Scope -- explicitly excluded:**

- No Artifacts REST API upload. The CLI writes results to disk; the workflow
  uploads via `actions/upload-artifact`.
- No dashboard integration. Result JSON is written to disk only.

**Acceptance criteria:**

1. A newly detected flaky test triggers GitHub Issue creation with correct
   title, body, and labels.
2. A second run with the same flaky test does NOT create a duplicate issue
   (dedup works).
3. PR comment is posted on the triggering PR with the quarantine summary.
4. A second run on the same PR updates the existing comment (no duplicate
   comments).
5. `--pr` flag overrides auto-detected PR number.
6. When `notifications.github_pr_comment: false`, no PR comment is posted.
7. `.quarantine/results.json` contains complete metadata for dashboard
   ingestion.
8. **First stakeholder demo:** Run quarantine in CI, observe: quarantined test
   excluded from execution, new flaky test triggers issue and PR comment,
   results file written.
9. Unit tests cover: issue body template rendering, PR comment template
   rendering with all conditional sections, dedup search query construction,
   `test_hash` generation, PR number detection from `GITHUB_EVENT_PATH`.
10. E2E test: full end-to-end against a real GitHub repo and PR.
11. Flaky test in a PR-only file: no issue created, PR comment warns developer
    (Scenario 72).
12. Flaky test in a pre-existing file, pre-existing test case, triggered from a
    PR: issue created and PR comment links to it (Scenario 73).
13. Flaky new test case added to a pre-existing file: no issue created, PR
    comment warns developer (Scenario 74).

**Key implementation notes:**

- The Search API dedup check and issue creation have a small race window.
  This is accepted per ADR-012. A human closes the duplicate.
- PR comment search is capped at 100 most recent comments to limit API calls.
  If the quarantine comment is older, a new one is created.
- Issue body should include the failure message and retry results table for
  developer actionability.
- The `410 Gone` response on issue creation means issues are disabled on the
  repo. Skip issue creation for all tests in this run.
- **Shallow clone protection (ADR-022):** GitHub Actions defaults to
  `fetch-depth: 1`, which does not include the base branch ref. The CLI must
  run `git fetch origin ${GITHUB_BASE_REF} --depth=1` before the diff check.
  If the fetch fails, fall back to creating the issue (safe default).
- **Test name search in diff is heuristic:** Searches added `+` lines for the
  test's `name` attribute from JUnit XML. Dynamic test names (e.g.,
  `test.each`) may not match — falls through to "create issue" (safe default).

---

### Agent B (dashboard-dev)

---

### M6: Dashboard Scaffolding + Data Ingestion

**Owner:** `dashboard-dev`

**Dependencies:** JSON schema contract (`schemas/test-result.schema.json`,
`schemas/quarantine-state.schema.json`) and test fixtures (`testdata/`).
No dependency on a working CLI.

**Scope -- included:**

- App scaffolding: TypeScript, Tailwind CSS, Biome (linting/formatting), pnpm.
- `dashboard/` directory structure: `app/routes/`, `app/lib/`, `app/components/`.
- `dashboard.yml` configuration file:
  - `source: manual` (explicit repo list, no auto-discovery in v1).
  - Repo list with owner/repo pairs.
  - GitHub token reference (env var name, not the token itself).
  - Poll interval (default 300 seconds).
  - Example:
    ```yaml
    source: manual
    repos:
      - owner: mycargus
        repo: my-app
      - owner: mycargus
        repo: other-app
    poll_interval: 300
    ```
- SQLite schema and migrations: `orgs`, `projects`, `tests`, `test_runs`,
  `test_results`, `quarantine_events` tables. WAL mode.
- Artifact polling pipeline:
  - Background worker polls GitHub Artifacts API per configured repos.
  - Uses `If-None-Match` (ETag) for conditional requests.
  - Filters artifacts by name prefix (`quarantine-result`).
  - Downloads new artifacts (follows 302 redirect to blob storage).
  - Extracts zip, parses JSON, validates against
    `schemas/test-result.schema.json`.
  - Upserts into SQLite (keyed by `run_id` for idempotency).
  - Debounced on-demand pull: max 1 per repo per 5 minutes.
- Circuit breaker per `docs/specs/github-api-inventory.md`: 3 consecutive failures
  for a repo triggers 30-minute pause.
- Error handling per `docs/specs/error-handling.md` Category 3: skip cycle on API
  failure, mark permanently skipped on 404/410 for artifacts, validate JSON
  before ingestion.
- Basic project listing page: shows configured repos with test run count and
  last sync timestamp.
- Makefile targets: `dash-test`, `dash-lint`.
- Developed against test fixtures from `testdata/expected/`. No real
  CLI or GitHub artifacts needed.
- Contract validation: artifact JSON parsing tested against
  `schemas/test-result.schema.json`.

**Scope -- explicitly excluded:**

- No analytics charts or trend visualization.
- No quarantined test list.
- No filters or search.
- No cross-repo overview.
- No authentication (network-level access only in v1).

**Acceptance criteria:**

1. `pnpm dev` starts the dashboard and shows a project listing page.
2. `pnpm build` produces a production build.
3. SQLite schema is created via migrations on first run.
4. Artifact polling ingests test fixtures into SQLite correctly.
5. Project listing page shows repo names, test run counts, and last sync time.
6. `dashboard.yml` with `source: manual` is parsed and repos are configured.
7. ETag-based conditional requests avoid re-downloading unchanged data.
8. Circuit breaker pauses polling after 3 consecutive failures for a repo.
9. Malformed artifact JSON is skipped with a warning (not a crash).
10. Unit tests cover: artifact JSON parsing, SQLite upsert logic, config
    parsing, circuit breaker behavior.
11. Contract tests: fixture JSON validates against the shared schema.

**Key implementation notes:**

- The dashboard has NO communication channel with the CLI. It discovers data
  autonomously from GitHub Artifacts.
- SQLite WAL mode enables concurrent reads during write operations.
- The `source: manual` config is the v1 approach. Auto-discovery via GitHub
  App org repo listing is v2+.
- Artifact retention is 90 days (GitHub default). The dashboard stores
  historical data in SQLite beyond that window.
- Use `ajv` (with `ajv/dist/2020` for draft 2020-12) for JSON schema
  validation.

---

## Phase 3 -- Integration and Polish

Phase 3 is sequential. M7 integrates dashboard with real CLI output. M8
hardens and documents the entire system.

---

### M7: Dashboard Analytics and UI

**Owner:** `dashboard-dev`

**Dependencies:** M6 (dashboard scaffolding, data ingestion).

**Scope -- included:**

- Quarantined test list per project: shows all currently quarantined tests
  with issue links, quarantine date, flaky count, last flaky occurrence.
- Trend chart: flaky test count over time (per project).
- Filters and search:
  - Filter by project (route-based), date range, test status.
  - Search by test name or test_id.
- Cross-repo overview page: aggregated stats across all configured repos
  (total quarantined tests, total flaky detections, most flaky tests).
- On-demand sync on page load: check debounce, pull artifacts from GitHub,
  ingest into both `test_runs` and `quarantined_tests` (FR-1.5.2, FR-1.5.3).
  Sync failure degrades gracefully — page renders with existing data.
- Ingest pipeline populates `quarantined_tests` from artifact test entries:
  `status: "quarantined"` → upsert with quarantined_at, last_run_status,
  issue_url; `status: "flaky"` → increment flaky_count, update last_failure_at.
- Dashboard renders data pulled from real GitHub Artifacts via on-demand sync.
- Responsive layout via CSS (no build step).

**Scope -- explicitly excluded:**

- No manual quarantine/unquarantine from the dashboard UI (read-only in v1).
- No authentication beyond network-level.
- No export or API endpoints.
- No background polling daemon (M8).

**Acceptance criteria:**

1. Project detail page shows a list of quarantined tests with issue links.
   (FR-1.5.1)
2. Trend charts render correctly with at least 7 days of data.
3. Filters narrow the displayed data correctly (date range, status, search
   all compose).
4. Search returns matching tests by name or test_id.
5. Cross-repo overview page shows aggregated statistics. (FR-1.5.5)
6. Dashboard pulls and displays data from real GitHub Artifacts via on-demand
   sync (not just test fixtures). (FR-1.5.2)
7. UI is responsive on desktop and tablet viewports. (FR-1.5.6)
8. Unit and interface tests cover: chart data queries, filter logic, search
   functionality, cross-repo aggregation.
9. On-demand sync fires on page load when data is stale, reads token from
   env, and respects the debounce. (FR-1.5.3, NFR-2.3.4)
10. Artifact ingestion populates `quarantined_tests` with `quarantined_at`
    preserved on conflict. (FR-1.5.1, FR-1.5.4)
11. Flaky detections increment `flaky_count` and update `last_failure_at`.
    (FR-1.5.1)
12. On-demand sync failure degrades gracefully — page renders with existing
    SQLite data. (FR-1.6.1 analogy)

**Key implementation notes:**

- Trend data requires multiple test runs over time. Interface testing should
  use a series of fixture files representing runs on different dates.
- The dashboard stores a SHA-256 hash of `test_id` internally for efficient
  indexing. This is an internal optimization, not part of the cross-system
  contract.

---

### M8: Polish and Hardening

**Owner:** `cli-dev` + `dashboard-dev` (collaborative)

**Dependencies:** M5 (CLI feature-complete), M7 (dashboard feature-complete).

**Scope -- included:**

- **Structured CLI logging:** Replace ad-hoc `[quarantine] WARNING:` stderr
  prints with a proper leveled logging system (ERROR / INFO / DEBUG). The
  `--verbose` flag promotes to INFO+DEBUG output; a `QUARANTINE_DEBUG` env
  var (or similar) enables debug-level output without CLI flags. Current
  warnings are already correct in content; this is a presentation/consistency
  improvement. Deferred from M5 to keep scope focused.
- Comprehensive error handling testing: all error paths from
  `docs/specs/error-handling.md` have corresponding tests (CLI and dashboard).
- Degraded mode testing: simulate GitHub API failures and verify correct
  behavior across all degraded scenarios.
- Documentation:
  - README.md: project overview, quick start, CI integration guide.
  - Setup guide: step-by-step for GitHub Actions with all three frameworks.
  - Workflow examples: basic, matrix jobs, monorepo (multiple `quarantine.yml`
    future consideration noted).
- Parameterized test fixture support:
  - Jest `test.each`: verify `test_id` construction handles parameterized
    names.
  - RSpec shared examples: verify `test_id` construction handles shared
    context names.
  - Vitest parameterized tests: verify `test_id` construction.
  - Fixtures added to `testdata/` for parameterized test output.
- CLI binary cross-compilation: linux/darwin x amd64/arm64 (4 targets).
- Release automation: GitHub Actions workflow for building, checksumming
  (SHA256), and publishing release assets.

**Scope -- explicitly excluded:**

- No new features. M8 is exclusively polish, hardening, and documentation.
- No v2 features (GitHub App, Jira, Slack, auto-fix, etc.).

**Acceptance criteria:**

1. Every error path documented in `docs/specs/error-handling.md` has at least one
   test.
2. Degraded mode scenarios pass: no quarantine state, API timeout, CAS
   conflict exhaustion, token expired.
3. README includes: quick start (< 5 minutes to first run), CI integration
   for all three frameworks, troubleshooting for common errors.
6. Parameterized test fixtures parse correctly and produce valid `test_id`
   values.
7. Cross-compiled binaries run on their target platforms (at minimum: linux
   amd64, darwin arm64).
8. `make test-all` passes (cli-test + dash-test + schemas-validate).
9. v1 feature-complete per `docs/specs/architecture.md` section 8 roadmap.

**Key implementation notes:**

- Parameterized tests are the most common source of `test_id` construction
  edge cases. Jest `test.each` generates names like
  `addition > adds 1 + 2 to equal 3`. RSpec shared examples generate names
  like `User when admin has admin privileges`.
- Release checksums use SHA-256, published as a separate file per ADR-014.

---

---

## Phase 4 -- Multi-Suite Conversion

Phase 4 converts the single-framework CLI (M1--M8) to the multi-suite model
defined across ADR-010, ADR-030, ADR-031, and ADR-032. Two milestones: M9 changes
the irreducibly coupled core; M10 adds features on the stable surface M9
provides.

---

### M9: Core Conversion

**Owner:** `cli-dev`

**Dependencies:** M1--M8 verified (current codebase as of 2026-04-11).

**Scope -- included:**

- Config parsed from `.quarantine/config.yml` with `test_suites` array;
  `junitxml` and `rerun_command` required per suite (ADR-010). No `framework`,
  `exclude`, `rerun_command` top-level fields.
- `quarantine run [suite-name]`: reads suite's `command` from config and
  executes via `exec.Command` (no `--` separator, no CLI args for test command).
  Suite command never modified or appended to (ADR-031).
- Suite selection: single suite no-arg runs it; multiple suites require name;
  no suites → exit 2 with guidance.
- Per-suite state files at `.quarantine/<suite>/state.json` on state branch
  (ADR-032).
- Per-suite PR comment marker `<!-- quarantine:<suite-name> -->` (ADR-032).
- Per-suite issue dedup label `quarantine:<suite-name>:<hash>` (ADR-032).
- Command crash detection: non-zero exit + no JUnit XML → exit 2 + diagnostic
  (ADR-031).
- Rerun command failure: test classified as `"unresolved"`, run continues,
  exit 2 (ADR-031).
- `quarantine init`: creates `.quarantine/config.yml` with pre-filled suite
  entries for detected frameworks; creates `.quarantine/.gitignore`; creates
  state branch with `README.md`; idempotent (re-run skips existing artifacts).
- `quarantine doctor`: validates `test_suites` array, suite name uniqueness and
  format, required fields; warns on detected framework-level retry config (D9).
- Delete old format: `quarantine.yml`, `--config`, `--`, `--output`,
  `--exclude` flags, `Framework` type, exclusion logic, `SplitShellArgs`.

**Scope -- explicitly excluded:**

- New CLI commands (`suite list`, `suite remove`, `quarantine status`) — M10.
- Timeout enforcement (`timeout`, `rerun_timeout` fields) — M10.
- `.quarantine/<suite>/quarantined-files.txt` generation — M10.
- Dashboard migration — M10.
- Schema file updates — M10.
- Full test exclusion — deferred to v2.

**Acceptance criteria:**

1. `.quarantine/config.yml` parsed with `test_suites` array; `junitxml` and
   `rerun_command` are required per suite; missing fields → exit 2 with
   diagnostic. (FR-1.4.1, ADR-010)
2. `quarantine run [suite-name]` executes the suite's `command` unmodified via
   `exec.Command`; no `--` separator; command never appended to. (FR-1.1.1,
   ADR-031)
3. Single suite configured + no name arg → runs the single suite. (FR-1.1.1)
4. Multiple suites + no name arg → exit 2 with list of available suite names.
   (FR-1.4.3)
5. Per-suite state file created at `.quarantine/<suite>/state.json` on the
   state branch on first run; subsequent runs update it via CAS. (FR-1.2.1,
   ADR-032)
6. PR comment for each suite uses `<!-- quarantine:<suite-name> -->` marker;
   suites on same PR each have independent comments. (FR-1.3.2, ADR-032)
7. Issue dedup label is `quarantine:<suite-name>:<hash>` for each suite;
   same test in two suites creates two separate issues. (FR-1.3.1, FR-1.3.3,
   ADR-032)
8. `quarantine init` detects jest/vitest/rspec, pre-fills suite entries,
   creates config + .gitignore + state branch; re-running is idempotent.
   (FR-1.4.1, FR-1.4.2)
9. `quarantine doctor` validates suite names (`[a-z0-9][a-z0-9-]*`, max 30),
   required fields, non-empty `test_suites`; warns on detected
   `retryTimes`/`retry`/`rspec-retry`. (FR-1.4.3)
10. Command crash (non-zero exit + no JUnit XML) → exit 2 with
    `Error [crash]: ...` diagnostic. (ADR-031, NFR-2.2.1)
11. Rerun command failure → test classified `"unresolved"`, run continues,
    exit 2; genuine failures still exit 1 (priority). (ADR-031)
12. Old format artifacts removed: `quarantine.yml` support, `--config` flag,
    `--` separator, `--output` flag, `--exclude` flag, `BuildExclusionArgs`,
    `Framework` type. (ADR-010, ADR-030)
13. `make cli-build && make cli-test && make cli-lint` pass.

**Key implementation notes:**

- See ADR-010 (config model), ADR-031 (command execution), ADR-032 (state isolation) for migration impact
- 68+ Go files reference old identifiers (`quarantine.yml`, `framework`,
  `quarantine-bot`, `quarantine.json`, `BuildExclusionArgs`, `SplitShellArgs`).
  All must be updated.
- Run `/create-user-scenario` for all M9 scenarios before coding.
- Run `/create-adr` for any design decisions not yet covered by ADR-010,
  ADR-031, or ADR-032.

---

### M10: Additive Features

**Owner:** `cli-dev` + `dashboard-dev`

**Dependencies:** M9 complete.

**Scope -- included:**

- `quarantine suite list`: prints configured suites with name, command, junitxml.
- `quarantine suite remove <name>`: removes suite from config with confirmation;
  preserves state file on state branch; prints ramifications before confirming.
- `quarantine status [suite-name]`: reads state + recent artifacts; shows
  quarantined test count, avg duration, estimated CI time, oldest quarantined
  tests (with age and issue number).
- Timeout enforcement: `timeout` (default 30m) and `rerun_timeout` (default 5m)
  per-suite config fields; SIGTERM then SIGKILL; partial XML processed on
  command timeout; rerun timeout → `"unresolved"`. (ADR-031)
- Writes `.quarantine/<suite>/quarantined-files.txt` before command execution:
  newline-delimited quarantined file paths, deduplicated. (plan D5)
- Error prefix convention for all exit-2 diagnostics:
  `Error [config]:`, `Error [crash]:`, `Error [timeout]:`, `Error [rerun]:`.
  (ADR-031)
- Dashboard: enumerate per-suite state files from `.quarantine/` directory on
  state branch (ADR-032); handle 404 gracefully.
- Schema files updated: `schemas/test-result.schema.json` adds `suite_name`,
  removes `framework`, adds `"unresolved"` status, `error`, `rerun_exit_code`;
  `schemas/quarantine-state.schema.json` updated for per-suite path semantics.

**Scope -- explicitly excluded:**

- `quarantine suite add` — deferred; users edit YAML directly (plan D4).
- Combined PR comment aggregation — deferred to v2.
- Per-framework exclusion adapters — deferred to v2.
- State consolidation (`quarantine state consolidate`) — deferred to v2.

**Acceptance criteria:**

1. `quarantine suite list` prints each suite's name, command, and junitxml
   path. (FR-1.4.2)
2. `quarantine suite remove backend` prints ramifications, asks `[y/N]`,
   removes suite from config on confirmation; state file on branch preserved.
   (FR-1.4.2)
3. `quarantine status backend` shows quarantined count, estimated CI time,
   oldest quarantined tests with age and issue numbers. (FR-1.1.1)
4. `quarantine status` with no arg and multiple suites shows summary for all.
   (FR-1.1.1)
5. `timeout: 30m` enforced: SIGTERM sent at 30m, SIGKILL after 5s grace;
   partial JUnit XML processed; exit 2 with `Error [timeout]: ...`. (ADR-031)
6. `rerun_timeout: 5m` enforced: hanging rerun killed; test classified
   `"unresolved"`; run continues; other tests processed. (ADR-031)
7. `.quarantine/<suite>/quarantined-files.txt` written before command
   execution; deduplicated; empty when no quarantined tests. (plan D5)
8. Error prefix convention applied: all exit-2 messages use `Error [config]:`,
   `Error [crash]:`, `Error [timeout]:`, or `Error [rerun]:`. (ADR-031)
9. Dashboard enumerates `.quarantine/` on state branch; reads per-suite
   `state.json`; handles 404 on `.quarantine/` gracefully. (ADR-032, FR-1.5.2)
10. Results JSON has `suite_name` field, no `framework` field, `"unresolved"`
    status, `error` and `rerun_exit_code` on unresolved tests. (ADR-031)
11. Schema files updated and schema validation passes (`make contract-test`).
    (NFR-2.6.3)
12. `make test-all` passes (cli-test + dash-test + contract-test + e2e-test).
    (NFR-2.5.1)

**Key implementation notes:**

- `quarantine status` reads duration data from recent artifacts (`duration_ms`
  in results.json). Omit the duration line if no artifacts are available.
- `quarantine suite remove` must NOT delete the state file on the state branch
  (preserves history per plan decision D6).
- Run `/create-user-scenario` for M10 scenarios before coding.

---

## Phase 5 -- Init UX Improvements

### M11: Init UX Improvements

**Status: Superseded.** M9's implementation of `quarantine init` is already
non-interactive — it auto-detects frameworks, creates config with defaults,
and the user edits YAML afterward. The flags (`--yes`, `--retries`,
`--junitxml`) and TTY detection proposed here were solutions to interactive
prompts that no longer exist. Per-suite retries validation (1–10) was
implemented as part of M9's config validation.

**Owner:** `cli-dev`

**Dependencies:** M9 (multi-suite init + framework detection foundation).

**Scope -- included:**

- `--yes`/`-y`, `--retries`, `--junitxml` flags on `quarantine init`.
- `--yes` mode: fully non-interactive; overwrites existing config with a log
  message.
- Flags without `--yes`: each flag skips its prompt; others still appear.
- Retries range validation (1–10) in both interactive and non-interactive paths.
- TTY detection: warn when stdin is not a terminal and `--yes` is absent.

**Scope -- explicitly excluded:**

- Package manager detection (no consumer in v1).
- JUnit XML path detection from config files.
- `--config` flag on `quarantine init` (removed per ADR-010 amendment).
- `--framework` flag (framework field removed per ADR-030).
- Changes to `quarantine run`, `quarantine doctor`, or `quarantine version`.

**Acceptance criteria:**

1. `quarantine init --yes` creates config without prompts, exits 0.
2. `quarantine init --yes --retries 11` exits 2 with actionable error.
3. Interactive retries prompt rejects out-of-range values and re-prompts.
4. `make cli-build && make cli-test && make cli-lint` pass.

**Key implementation notes:**

- M9 already implements framework detection and pre-filling. M11 adds
  per-suite retries validation on top of that foundation.

---

## Milestone Dependency Graph

```
M1 (scaffolding + init)
  |
  v
M2 (test execution + XML parsing)
  |
  v
M3 (flaky detection + retry)
  |
  +-----------+-----------+
  |                       |
  v                       v
M4 (state + exclusion)   M6 (dashboard scaffolding)
  |                       |
  v                       v
M5 (issues + PR comments) M7 (dashboard analytics)
  |                       |
  |           +-----------+
  |           |
  v           v
            M8 (polish + hardening)

--- multi-suite conversion (foundational model change) ---

M9 (core conversion: config, run, state, notifications)
  |
  v
M10 (additive: suite commands, timeouts, dashboard, schemas)
  |
  v
M11 (init UX: --yes flag, TTY detection) — SUPERSEDED (init is already non-interactive)

--- v2 begins after multi-suite is stable ---

M12 (App auth foundation)
  |
  v
M13 (OAuth login)
  |
  v
M14 (installation discovery)
  |
  v
M15 (github-app mode integration)
  |
  v
M16 (E2E integration + CI)

--- server-side writes (ADR-036 amendment) ---

M17 (CLI v2 read-only mode)    M18 (dashboard write processing)
  |                               |   (also depends on M6/M7 artifact
  |                               |    pipeline + M15 installation tokens)
  +---------------+---------------+
                  |
                  v
        M19 (v2 server-side writes E2E)

--- v3: webhooks for immediate feedback ---

workflow_run.completed -> instant artifact processing (~30s)

--- Jenkins / non-GitHub-origin support (v1, parallel to v2) ---

M20 (relax origin constraint: non-github.com origins, init partial-success,
     config-based owner/repo, Jenkins integration guide)
     depends on: M9 (multi-suite config format)
```

---

## Phase 6 -- GitHub App Integration (v2)

v2 adds GitHub App support for org-wide installation, fine-grained permissions,
higher rate limits, dashboard OAuth login, and automatic repository discovery.
v2 targets GitHub Actions only. Built on the multi-suite surface from Phase 5
(M9/M10). See ADR-008 for the motivation behind the migration from PATs to
GitHub App auth.

**User stories driving v2:**

| ID | As a... | I want to... | So that... |
|----|---------|-------------|------------|
| US-1.1 | Org admin | Install a single GitHub App on my org | All repos can use quarantine without per-repo PAT configuration |
| US-1.2 | Org admin | Use fine-grained permissions | I can audit exactly what the tool accesses |
| US-1.3 | Org admin | Add the App to ruleset bypass lists | The CLI can write to `quarantine/state` with branch protection enabled |
| US-1.4 | Org admin | Have the dashboard auto-discover repos | I don't manually edit `dashboard.yml` for every new repo |
| US-1.5 | Org admin | Suspend/uninstall the App gracefully | CI falls back to cached state, dashboard stops polling — no CI failures |
| US-2.1 | Developer | Log in via "Sign in with GitHub" | I can view test data without a separate account |
| US-2.2 | Developer | See only repos I have access to | Data is filtered by my GitHub permissions |
| US-2.3 | Developer | See the App's bot identity on comments/issues | PR comments aren't from a personal account |
| US-2.4 | Developer | Keep using a PAT if my team hasn't migrated | The upgrade isn't forced |
| US-3.1 | Platform engineer | Configure App credentials once at org level | All repos get short-lived installation tokens via `actions/create-github-app-token` |
| US-3.2 | Platform engineer | Have the dashboard generate its own tokens | No long-lived PAT for dashboard artifact polling |
| US-3.3 | Platform engineer | Monitor rate limit usage | I can track GitHub API consumption across installations |

**Key decisions (ADRs):**
- ADR-026: CLI uses `actions/create-github-app-token` (no CLI code changes).
- ADR-027: Webhooks deferred to v3.
- ADR-028: Dashboard OAuth via remix-auth (superseded by ADR-029).
- ADR-029: Dashboard OAuth via first-party `@remix-run/auth` (no custom implementation).

**v2 scope summary:**
- Register GitHub App (webhooks disabled).
- Dashboard: OAuth login via `@remix-run/auth` with `createGitHubAuthProvider` (ADR-029).
- Dashboard: Installation token generation (JWT + token exchange in TypeScript).
- Dashboard: Installation discovery via startup sync + 15-minute background loop.
- Dashboard: `source: github-app` mode in `dashboard.yml`.
- Dashboard: `installations` table + `installation_id` on `projects`.
- CLI: No code changes. External token injection via `actions/create-github-app-token`.
- Contract tests: App auth endpoints added to `schemas/github-api.json`.

**Deferred to future milestones:**
- CLI-native App auth (JWT in Go, TokenProvider): triggered by non-GitHub-Actions
  CI support. See `docs/plans/cli-native-app-auth.md`.
- Webhooks (real-time unquarantine, instant artifact ingestion): deferred to v3.
  See `docs/plans/webhooks.md`.

---

## Phase 7 — Jenkins / Non-GitHub-Origin Support

### M20: Relax Git Origin Constraint

**Owner:** `cli-dev`

**Dependencies:** M9 (multi-suite config format; `.quarantine/config.yml` with `test_suites`).

**Scope:** See [docs/milestones/m20.md](m20.md) and [ADR-037](../adr/037-relax-origin-host-constraint.md).

- Removes git-remote scanning from `run` and `doctor`; `.quarantine/config.yml` is the sole authoritative source of `github.owner`/`github.repo`.
- `quarantine init` becomes a two-phase flow: phase 1 writes a partial config with an empty `github` block and exits 2 with hand-edit instructions; phase 2 (re-run after edit) validates the token, creates `quarantine/state`, and exits 0.
- `quarantine init` phase 1 emits non-authoritative YAML comments listing detected github.com remotes (e.g., `# origin -> owner/repo`, `# upstream -> owner/repo`) so fork+upstream users see both candidates rather than getting one silently chosen. Comments are ignored by all other code paths.
- `quarantine run` and `quarantine doctor` exit 2 with `Error [config]:` when `github.owner` or `github.repo` is missing or empty.
- `quarantine doctor` performs a single `GET /repos/{owner}/{repo}` reachability check; does not inspect git origin, token scopes, or feature flags.
- Jenkins integration guide added.
- Amends ADR-013 and ADR-019.

**Acceptance criteria:** See [m20.md](m20.md#acceptance-criteria).

---

## Cross-References

- `docs/specs/cli-spec.md`: CLI commands, flags, exit codes, output format.
- `docs/specs/config-schema.md`: quarantine.yml field definitions and validation.
- `docs/specs/github-api-inventory.md`: API endpoints, error handling, rate limits.
- `docs/specs/error-handling.md`: error categories, degraded mode, `--strict`.
- `docs/specs/architecture.md`: system design, data model, deployment.
- `schemas/test-result.schema.json`: contract between CLI and dashboard.
- `schemas/quarantine-state.schema.json`: quarantine.json format.
