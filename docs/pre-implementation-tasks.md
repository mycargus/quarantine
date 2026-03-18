# Pre-Implementation Tasks

> Last updated: 2026-03-17
>
> These tasks should be completed before (or during early) implementation to
> reduce ambiguity and avoid rework. They are ordered by impact on
> implementation speed.

## Status

| # | Task | Status | Priority | Deliverable |
|---|------|--------|----------|-------------|
| 1 | CLI interface specification | Done | P0 | `docs/cli-spec.md` |
| 2 | GitHub API interaction inventory | Done | P0 | `docs/github-api-inventory.md` |
| 3 | Implementation milestones | Done | P0 | `docs/milestones.md` |
| 4 | quarantine.yml full schema | Done | P0 | `docs/config-schema.md` |
| 5 | Sequence diagrams for key flows | Done | P1 | `docs/sequence-diagrams.md` |
| 6 | Test strategy for the project itself | Done | P1 | `docs/test-strategy.md` |
| 7 | Error handling strategy | Done | P1 | `docs/error-handling.md` |
| 8 | CLAUDE.md for the repo | Done | P1 | `CLAUDE.md` |
| 9 | JSON interface schemas | Done | P0 | `schemas/*.schema.json` |
| 10 | Golden test fixtures | Done | P0 | `testdata/` (30 files) |
| 11 | Repo scaffolding and build infrastructure | Done | P0 | `cli/`, `dashboard/`, `Makefile` |
| 12 | Claude Code hooks for scope enforcement | Done | P1 | `.claude/hooks/`, `.claude/settings.json` |

P0 = complete before coding. P1 = can be done alongside early milestones.

---

## 1. CLI Interface Specification

**Goal:** Define every command, subcommand, flag, argument, exit code, and
output format so an implementer can code without re-reading the architecture
doc.

**Deliverable:** `docs/cli-spec.md`

### What to define

**Commands:**

```
quarantine run [flags] -- <test command>
quarantine doctor [flags]
quarantine version
```

For each command, specify:
- Description (one sentence)
- Arguments (positional and named)
- Flags with types, defaults, and descriptions
- Environment variable overrides (e.g., QUARANTINE_GITHUB_TOKEN)
- Exit codes and their meanings
- Stdout/stderr output format (human-readable vs machine-readable)
- Examples

**Resolved decisions:**
- **`quarantine init` command:** Required before `quarantine run`. Interactive:
  prompts for framework, retries, JUnit XML path. Creates `quarantine.yml`,
  validates GitHub token/permissions, creates `quarantine/state` branch.
  `--yes` flag (accept defaults, non-interactive) deferred to v2.
- **`quarantine run` without init:** Refuses with error:
  `"Quarantine is not initialized for this repository. Run 'quarantine init' first."`
- **Output verbosity:** `--verbose`, `--quiet` in v1. `--json` deferred to v2.
- **PR comment format:** Designed as part of the CLI spec deliverable.
  PR comment identification via hidden HTML marker `<!-- quarantine-bot -->`.
- **Exit codes:**
  - 0 = success (tests passed; includes degraded mode where tests passed)
  - 1 = test failure (real, non-flaky failures exist)
  - 2 = quarantine error (not initialized, bad command, `--strict` infrastructure failure)
  - `doctor` and `init`: 0 = success, 2 = failure
  - Exit code 1 exclusively means "your tests failed." No ambiguity.
  - Research pending: CI runner conventions for exit codes 0/1/2.
- **`--strict` mode:** v1. Infrastructure errors cause exit 2 instead of
  degraded mode. Useful for debugging and verifying setup.
- **Quarantined tests don't run.** CLI excludes them via framework-specific
  flags before test execution. No JUnit XML rewriting. ADR-003 needs updating.
- **`framework` is required** in `quarantine.yml`. No auto-detection.
  `quarantine init` sets it interactively.

**Commands (updated):**

```
quarantine init
quarantine run [flags] -- <test command>
quarantine doctor [flags]
quarantine version
```

**Remaining questions to resolve in cli-spec.md:**
- Full flag inventory for `quarantine run` (at minimum: `--retries`, `--config`,
  `--junitxml`, `--dry-run`, `--verbose`, `--quiet`, `--strict`, `--pr`,
  `--exclude`)
- `quarantine doctor` output format
- `quarantine init` interactive prompts and validation steps
- Framework-specific test exclusion mechanisms (Jest, RSpec, Vitest)

---

## 2. GitHub API Interaction Inventory

**Goal:** Enumerate every GitHub API call the CLI and dashboard make, with
exact endpoints, methods, required permissions, error handling, and retry
behavior.

**Deliverable:** `docs/github-api-inventory.md`

### What to define

For each API interaction, specify:
- Caller (CLI or Dashboard)
- Endpoint (e.g., `GET /repos/{owner}/{repo}/contents/{path}`)
- When it's called (which step in the data flow)
- Required token scope / GitHub App permission
- Request parameters
- Success response handling
- Error responses to handle (not just 409/429 -- also 403, 404, 422, 500,
  502, 503, timeouts)
- Retry behavior
- Degraded-mode behavior (what happens if this call fails)

**Known API interactions to document:**

CLI:
1. Read quarantine.json from branch (Contents API GET)
2. Write quarantine.json to branch (Contents API PUT, with SHA CAS)
3. Create quarantine/state branch (Refs API POST -- first run only)
4. Search for existing GitHub issue (Search API or Issues API GET)
5. Create GitHub issue (Issues API POST)
6. Post PR comment (Issues API POST -- PRs are issues)
7. Update existing PR comment (Issues API PATCH)
8. Check if issue is closed (Issues API GET)
9. Upload artifact (Actions Artifacts API -- via actions/upload-artifact or
   REST)
10. Read Actions cache (actions/cache action -- within workflow only)
11. Write Actions cache (actions/cache action -- within workflow only)

Dashboard:
12. List artifacts for a repo (Actions Artifacts API GET)
13. Download artifact (Actions Artifacts API GET + redirect)
14. List repositories in org (Repos API GET -- v2, GitHub App)
15. OAuth flow endpoints (v2)

**Resolved decisions:**
- **Artifact upload:** CLI writes result JSON to a local file
  (e.g., `.quarantine/results.json`). The user's GitHub Actions workflow
  uses `actions/upload-artifact` to upload it. CLI does file I/O only --
  no Artifacts REST API complexity. REST API upload is a v2 concern for
  non-GHA CI environments. Artifact naming collisions in matrix jobs are
  handled via documentation (workflow snippet with matrix-safe naming).
- **GHA detection:** CLI checks `GITHUB_ACTIONS` env var to determine if
  running inside GitHub Actions (used for `::warning` annotations, etc.).
- **PR number detection:** CLI auto-detects from `GITHUB_EVENT_PATH` env
  var (reads the JSON file, extracts PR number). `--pr` flag overrides
  for forward compatibility with non-GHA environments.
- **Unquarantine detection:** Batch via GitHub Search API -- one call
  returns all closed quarantine issues. Compare against `quarantine.json`.
  One API call instead of N (avoids 5-25s latency with many quarantined tests).
- Item 9 (artifact upload) updated: CLI writes to disk, workflow uploads.
  Items 10-11 (Actions cache) unchanged.
- **Dashboard artifact ingestion:** v1 uses GitHub Artifacts polling.
  Research pending on Jenkins/GitLab artifact systems for v2 per-provider
  dashboard pollers.

**Remaining questions to resolve in github-api-inventory.md:**
- Rate limit tracking: should CLI read `X-RateLimit-Remaining` headers
  and log warnings when approaching the limit?

---

## 3. Implementation Milestones

**Goal:** Break v1 into buildable, testable, demoable increments with clear
dependency order.

**Deliverable:** `docs/milestones.md`

### Proposed milestone structure

Each milestone should define: scope, acceptance criteria, dependencies on
prior milestones, and estimated complexity.

Milestones are organized into three phases. Phase 2 enables parallel
development by two agents, which can save significant calendar time.

```
Phase 1 -- Foundation (sequential, single agent):
  M1: CLI core -- run tests and parse JUnit XML
      - quarantine run -- <command> executes the test command
      - Parses JUnit XML output (single file and glob)
      - Detects failures
      - Framework auto-detection (RSpec, Jest, Vitest)
      - No GitHub integration, no quarantine logic
      - Exit 0 if all pass, exit 1 if failures
      Acceptance: CLI wraps jest/rspec/vitest, parses results, prints summary

  M2: Flaky detection via retry
      - Re-runs individual failing tests using framework-specific commands
      - Identifies flaky tests (fail then pass on retry)
      - Rewrites JUnit XML (failures -> skips for flaky tests)
      - Exit code reflects real failures only
      Acceptance: CLI detects a known-flaky test and suppresses its failure

Phase 2 -- Parallel development (two agents):
  Agent A (CLI GitHub integration):
    M3: Quarantine state on GitHub
        - Reads quarantine.json from quarantine/state branch
        - Writes updated quarantine.json with optimistic concurrency
        - Creates branch on first run
        - Already-quarantined tests suppressed without retry
        - Actions cache fallback for degraded mode
        Acceptance: quarantine.json on branch reflects detected flaky tests

    M4: GitHub Issues and PR comments
        - Creates issues for newly quarantined tests (with dedup)
        - Posts/updates PR comments with summary
        - Checks if issues are closed -> unquarantines tests
        Acceptance: flaky test triggers issue creation and PR comment

    M5: GitHub Artifacts upload
        - Uploads structured JSON results as GitHub Artifact
        - Includes run metadata (commit, branch, timestamp, CI info)
        Acceptance: results visible in GitHub Actions artifact list

  Agent B (Dashboard, uses fixture data -- no working CLI needed):
    M6: Dashboard -- data ingestion
        - React Router v7 app scaffolding
        - SQLite schema and migrations
        - Artifact polling and ingestion pipeline
        - Basic project listing page
        - Developed against golden test fixtures, not real CLI output
        Acceptance: dashboard shows list of projects with test run counts

Phase 3 -- Integration and polish (sequential):
  M7: Dashboard -- analytics and UI
      - Quarantined test list per project
      - Trend charts (flaky test count over time, test stability)
      - Filters and search
      - Cross-repo overview page
      - Integration tested with real CLI output from M5
      Acceptance: dashboard shows actionable flaky test data with trends

  M8: Polish and hardening
      - Comprehensive error handling (see task 7)
      - Degraded mode testing
      - quarantine doctor command
      - CLI Docker image
      - Documentation (README, setup guide)
      Acceptance: v1 feature-complete, tested, documented
```

### Why Phase 2 can be parallelized

Agent B can start M6 as soon as the test-result JSON schema (task 9) and
golden fixtures (task 10) exist. It develops the dashboard ingestion pipeline
and UI against fixture data. It doesn't need a working CLI or real GitHub
artifacts. This can save significant calendar time.

### Contract tests between agents

Both Agent A's CLI output and Agent B's dashboard input should be validated
against the same JSON schema (from task 9). This ensures the two components
will work together when integrated in Phase 3.

**Resolved decisions:**
- **Granularity:** Keep milestones as small as possible. Break into more
  if needed during implementation.
- **Minimum viable demo:** M4 (GitHub Issues + PR comments). First
  milestone where the tool is visible to stakeholders without reading CI logs.
- Milestones may be further split in the milestones.md deliverable.
- **M5 (Artifacts upload) folded into M4.** CLI writes results to disk as
  part of normal operation. Artifact upload is a workflow snippet (docs).
- **Milestones need holistic rework** due to:
  - `quarantine init` is now required (needs its own milestone or M1 scope)
  - Quarantined tests are excluded from execution (not XML rewriting)
  - `framework` is required (no auto-detection in M1)
  - `pending.json` removed
  - `storage.backend: actions-cache` removed from config
  - Dashboard config (`dashboard.yml`) needs to be in M6 scope

---

## 4. quarantine.yml Full Schema

**Goal:** Define every configuration field, its type, default, validation
rules, and behavior.

**Deliverable:** `docs/config-schema.md`

### What to define

```yaml
# quarantine.yml -- full schema

version: 1                          # Required. Schema version.

framework: jest                     # Required. Set by `quarantine init`.
                                    # Valid: rspec, jest, vitest (v1)
                                    #        pytest, go, maven (v2+)

retries: 3                          # Optional. Default: 3. Range: 1-10.
                                    # Number of times to re-run a failing test
                                    # before declaring it a real failure.

junitxml: "results/*.xml"           # Optional. Default: framework-specific.
                                    # Glob pattern for JUnit XML output files.
                                    # Supports multiple files (parallel runners).

github:
  owner: my-org                     # Optional. Auto-detected from git remote.
  repo: my-project                  # Optional. Auto-detected from git remote.

issue_tracker: github               # Optional. Default: github.
                                    # v1: only "github" accepted.
                                    # v2+: jira, etc.

labels:                             # Optional. Default: [quarantine].
  - quarantine                      # v1: only ["quarantine"] accepted.
                                    # v2+: custom labels.

notifications:                      # Optional. Default: { github_pr_comment: true }.
  github_pr_comment: true           # v1: only valid key. true/false.
                                    # v2+: adds slack, possibly email.
  # slack:                          # v2+
  #   webhook_url: https://...
  #   threshold: 10

storage:
  branch: quarantine/state          # Optional. Default: quarantine/state.

exclude:                            # Optional. Default: none.
  - "test/integration/**"           # Patterns for tests quarantine ignores
  - "TestSlowService"               # entirely (no retry, no quarantine, no issue).
                                    # Pattern matching target pending test_id research.

rerun_command: ""                   # Optional. Override auto-detected rerun
                                    # command template. Uses {name}, {classname},
                                    # {file} placeholders.
```

**Resolved decisions:**
- **`framework`:** Required field. No auto-detection. Set by `quarantine init`.
- **`issue_tracker`:** Forward-compatible. v1: only `github`. Error on other values.
- **`labels`:** Forward-compatible. v1: only `[quarantine]`. Error on other values.
- **`notifications`:** Nested object. v1: only `github_pr_comment: true/false`.
  Error on other keys (e.g., `slack`). Defaults to `{ github_pr_comment: true }`.
- **`storage.backend`:** Removed. v1 only supports branch storage. `actions-cache`
  is an internal degraded-mode detail, not a user-facing config option.
- **`exclude` field:** v1. Quarantine ignores matched tests entirely -- no retry,
  no quarantine, no issue. Behavior is as if quarantine is not installed for those tests.
  What patterns match against (test_id? classname? name?) pending research.
- **Per-framework overrides:** Deferred to v2.
- **GitHub token:** Env vars only (`QUARANTINE_GITHUB_TOKEN`, falls back to
  `GITHUB_TOKEN`). Never in config file. No exceptions.

---

## 5. Sequence Diagrams for Key Flows

**Goal:** Step-by-step sequence diagrams for the main flows, showing exact
order of operations, API calls, decision points, and error paths.

**Deliverable:** `docs/sequence-diagrams.md` (using Mermaid or text-based
diagrams)

### Flows to diagram

1. **Happy path: flaky test detected and quarantined**
   CLI starts -> read quarantine.json -> batch check issue status (Search API)
   -> remove unquarantined tests -> exclude quarantined tests from command ->
   run tests -> parse XML -> test fails -> retry test -> passes on retry ->
   flag as flaky -> update quarantine.json (with CAS) -> create GitHub issue
   (with dedup check) -> post/update PR comment -> write results to disk ->
   exit 0

2. **Happy path: quarantined test excluded from execution**
   CLI starts -> read quarantine.json -> quarantined tests exist -> batch
   check issue status -> all issues still open -> construct test command with
   framework-specific exclusion flags -> run tests (quarantined tests don't
   execute) -> parse XML -> all ran tests pass -> write results to disk ->
   exit 0

3. **Quarantined test's issue is closed (unquarantine)**
   CLI starts -> read quarantine.json -> batch check issue status (Search API)
   -> issue closed -> remove test from quarantine.json (CAS write) -> test is
   no longer excluded -> run tests -> test runs normally -> if it fails, it
   fails the build like any other test

4. **Concurrent builds: CAS conflict on quarantine.json**
   Build A reads (SHA1) -> Build B reads (SHA1) -> Build A writes (ok, SHA2)
   -> Build B writes (409) -> Build B re-reads (SHA2) -> Build B merges ->
   Build B writes (ok, SHA3)

5. **Degraded mode: GitHub API unreachable**
   CLI starts -> try read quarantine.json -> timeout -> try Actions cache ->
   cache hit -> use cached data -> exclude quarantined tests -> run tests ->
   try write quarantine.json -> timeout -> log warnings + emit ::warning
   annotation -> write results to disk -> exit 0 (based on test results only)

6. **Degraded mode: no cache, no API**
   CLI starts -> try read -> fail -> try cache -> miss -> log warning -> run
   all tests (no quarantine state available, no exclusions) -> detect flaky
   tests via retry -> write results to disk -> log warnings -> exit based on
   test results (flaky failures still forgiven via retry)

7. **Dashboard: artifact ingestion**
   Poll timer fires -> list artifacts since last poll -> for each new artifact
   -> download -> parse JSON -> upsert into SQLite -> update last-poll
   timestamp

8. **quarantine init (new)**
   User runs `quarantine init` -> interactive prompts (framework, retries,
   JUnit XML path) -> write quarantine.yml -> validate GitHub token ->
   test repo access -> create quarantine/state branch with empty
   quarantine.json -> print summary and next steps

**Resolved:** Use Mermaid (renderable in GitHub).

---

## 6. Test Strategy for the Project Itself

**Goal:** Define how we test the Quarantine CLI and dashboard so we have
confidence in correctness without over-engineering the test suite.

**Deliverable:** `docs/test-strategy.md`

### Areas to cover

**CLI unit tests (Go):**
- JUnit XML parsing (various framework outputs, malformed XML, multiple files)
- quarantine.json read/write/merge logic
- Framework-specific test exclusion flag construction (Jest, RSpec, Vitest)
- Rerun command construction per framework
- Config file parsing and validation (including forward-compatible field validation)
- Exit code determination logic
- test_id construction from JUnit XML attributes

**CLI integration tests:**
- Against a real GitHub repo (or GitHub API mock?)
- End-to-end: run a real test suite (small Jest/RSpec/Vitest project with a
  known flaky test) through the CLI
- CAS conflict simulation (two processes updating quarantine.json concurrently)
- Degraded mode (mock GitHub API returning errors)

**Dashboard unit tests (TypeScript):**
- Artifact JSON parsing
- SQLite query correctness
- Polling logic (debounce, adaptive frequency)

**Dashboard integration tests:**
- Artifact ingestion from mock GitHub API
- Web UI rendering (loader data -> rendered page)

**Contract tests:**
- Validate CLI output against test-result.schema.json
- Validate dashboard input parsing against test-result.schema.json
- Validate quarantine.json read/write against quarantine-state.schema.json
- These ensure CLI and dashboard agree on data formats even when developed by independent agents
- Run as part of `make schemas-validate` and in CI

**Resolved decisions:**
- **GitHub API: mocked for CI, real for periodic integration tests.** A
  dedicated test repo with purposefully flaky tests (failing a predetermined
  % of the time) will be used for integration testing. Quarantine runs
  regularly against this repo for maximum confidence.
- **Dogfooding:** Only if it adds significant value. The test repo approach
  provides more controlled confidence.
- **Test assertion libraries:** Go: `github.com/mycargus/riteway-golang`.
  TypeScript: `github.com/paralleldrive/riteway`.
- Coverage threshold: not specified. Revisit during implementation.

---

## 7. Error Handling Strategy

**Goal:** Define how the CLI and dashboard handle every category of error,
guided by the principle: "never break the build due to Quarantine's own
failure."

**Deliverable:** `docs/error-handling.md`

### Error categories

**Category 1: GitHub API errors (CLI)**

| Error | Cause | Behavior |
|-------|-------|----------|
| 401 Unauthorized | Bad token | Log error, run in degraded mode |
| 403 Forbidden | Insufficient permissions | Log error, run in degraded mode |
| 404 Not Found | Branch/repo doesn't exist | `init`: create branch. `run`: not initialized error (exit 2). Otherwise: log error, degraded mode |
| 409 Conflict | CAS conflict on quarantine.json | Re-read, merge, retry (max 3) |
| 422 Validation | Malformed request | Log error, degraded mode |
| 429 Rate Limited | Too many requests | Backoff per Retry-After header, retry |
| 500/502/503 | GitHub server error | Retry once after 2s, then degraded mode |
| Timeout | Network issue | Retry once after 2s, then degraded mode |

**Category 2: Test runner errors (CLI)**

| Error | Cause | Behavior |
|-------|-------|----------|
| Test command not found | Bad PATH or command | Exit 2 (quarantine error), print diagnostic |
| Test command exits non-zero | Test failures (expected) | Parse XML, proceed normally |
| Test command crashes (no XML produced) | Runner error | Exit with runner's exit code, log warning that no XML was found |
| JUnit XML malformed | Parser error | Log warning, treat as if no tests ran, exit with runner's exit code |
| JUnit XML not found | Wrong path/glob | Log error suggesting --junitxml flag, exit with runner's exit code |
| Multiple XML files, some malformed | Partial parse | Parse what we can, log warnings for failures, proceed |

**Category 3: Dashboard errors**

| Error | Cause | Behavior |
|-------|-------|----------|
| GitHub API errors (polling) | Same as CLI | Log, skip this poll cycle, retry next cycle |
| SQLite write failure | Disk full, corruption | Log error, return 500 to UI, alert admin |
| Artifact download failure | Network, deleted artifact | Log, skip artifact, retry next cycle |
| Malformed artifact JSON | Bug or tampering | Log warning, skip artifact |

### Guiding principles
1. The CLI must NEVER exit non-zero due to its own failure if the test suite
   itself passed. Exit code must always reflect test results, not quarantine
   infrastructure status.
2. All quarantine infrastructure errors are logged as warnings, never fatal.
3. The dashboard failing has zero impact on CI.
4. Prefer degraded mode over failure in every case.

**Resolved decisions:**
- **Degraded mode communication:** Two mechanisms, both in v1:
  1. Stderr warning line: `[quarantine] WARNING: running in degraded mode
     (reason)` -- always emitted, works everywhere.
  2. GitHub Actions `::warning` annotation: shows as yellow warning banner
     on workflow run summary. Emitted when `GITHUB_ACTIONS` env var is set.
  Forward-compatible: stderr works in any CI; GHA annotation is v1-specific
  but harmless elsewhere.
- **`--strict` mode:** Yes, v1. When set, infrastructure errors cause exit 2
  instead of degraded mode. Useful for debugging and verifying setup.

---

## 8. CLAUDE.md for the Repo

**Goal:** Create a CLAUDE.md that gives Claude Code the context it needs to
assist with implementation efficiently in future sessions.

**Deliverable:** `CLAUDE.md` in repo root

### What to include

- Project summary (one paragraph)
- Architecture overview (pointer to docs/architecture.md)
- Tech stack: Go CLI, React Router v7 + SQLite dashboard
- Repo structure (once established)
- Key decisions (pointer to docs/adr/)
- v1 scope: RSpec, Jest, Vitest; GitHub Actions; GitHub Issues; PR comments
- Coding conventions (once established): Go formatting, TypeScript formatting,
  test patterns
- How to build and test (once established)
- What NOT to do: don't add frameworks beyond v1 scope, don't add features
  beyond current milestone, don't create SaaS infrastructure dependencies in
  the CLI critical path

### When to create
After the repo structure is initialized (M1). Update as conventions are
established.

---

## 9. JSON Interface Schemas

**Goal:** Define the JSON schemas that serve as contracts between CLI and
dashboard, enabling parallel development by independent agents.

**Deliverable:** `schemas/` directory with JSON Schema files

**Priority:** P0

### Schemas to create

1. `quarantine-state.schema.json` -- The quarantine.json format stored on the
   GitHub branch. Must match the data model in architecture.md section 5.1.
2. `test-result.schema.json` -- The artifact payload format that the CLI
   uploads and the dashboard ingests. Must match architecture.md section 5.2.
3. `quarantine-config.schema.json` -- Validation schema for quarantine.yml.
   Must match the full schema defined in task 4.

### Why this matters for agentic development

- Two agents can work on CLI and dashboard independently, both validating
  against the same schemas
- Schema changes are explicit -- a PR that modifies a schema signals a
  contract change
- Both Go (CLI) and TypeScript (dashboard) can validate against JSON Schema
  at build/test time

**Resolved decisions:**
- JSON Schema draft 2020-12. Go library: `santhosh-tekuri/jsonschema`.
  TypeScript library: `ajv` (with `ajv/dist/2020`).
- **`test_id` format:** Human-readable composite `file_path::classname::name`.
  `::` delimiter does not appear in any framework's JUnit XML output.
  Raw components (`file_path`, `classname`, `name`) stored alongside `test_id`
  in both `quarantine.json` and test result artifacts.
- Dashboard stores a SHA-256 hash of `test_id` internally for efficient
  indexing. The hash is not part of the cross-system contract.

---

## 10. Golden Test Fixtures

**Goal:** Create sample JUnit XML files from each v1 framework (RSpec, Jest,
Vitest) plus expected parsed output. These enable agents to implement and
verify XML parsing without running real test suites.

**Deliverable:** `testdata/` directory

**Priority:** P0

### Structure

```
testdata/
  junit-xml/
    jest/
      single-pass.xml          # all tests pass
      single-failure.xml       # one real failure
      flaky-run-1.xml          # test fails (first execution)
      flaky-run-2.xml          # same test passes (retry execution)
      multiple-suites.xml      # parallel runner output (multiple files)
      malformed.xml            # truncated/invalid XML
    rspec/
      single-pass.xml
      single-failure.xml
      flaky-on-retry.xml
      multiple-suites.xml
      malformed.xml
    vitest/
      single-pass.xml
      single-failure.xml
      flaky-on-retry.xml
      multiple-suites.xml
      malformed.xml
  expected/
    jest-single-failure.json   # expected parsed output from CLI
    rspec-single-failure.json
    vitest-single-failure.json
    ...
```

### How to create these

- Run real Jest, RSpec, and Vitest test suites with known-flaky tests and
  capture the XML output
- Alternatively, hand-craft XML based on official format documentation, but
  real output is preferred for accuracy
- Expected output JSON files should match the test-result.schema.json schema

### Why this matters for agentic development

- Agents can implement JUnit XML parsing and immediately verify correctness
- No need to install Jest/RSpec/Vitest to work on the parser
- Fixtures also serve as documentation of framework-specific XML variations
- Malformed fixtures enable testing error handling paths

**Resolved decisions:**
- **Parameterized test output:** Yes, at end of v1. (Jest `test.each`,
  RSpec shared examples, Vitest parameterized.)
- **Parallel runner output:** Yes, as early as possible in v1. (Jest `--shard`,
  RSpec `parallel_tests`, Vitest threads.) Include early to avoid concurrency
  bugs that could be difficult to address later.

---

## 11. Repo Scaffolding and Build Infrastructure

**Goal:** Set up the directory structure, build tools, test infrastructure,
and one example test per component so agents can start coding immediately.

**Deliverable:** Initial repo structure with Makefile, Go module, package.json,
test scaffolding

**Priority:** P0

### Structure

```
quarantine/
  CLAUDE.md
  Makefile
  schemas/                    # JSON Schema files (task 9)
  testdata/                   # Golden fixtures (task 10)
  cli/
    CLAUDE.md                 # Go-specific conventions and commands
    go.mod
    go.sum
    cmd/quarantine/
      main.go                 # Entry point
    internal/
      parser/
        parser.go             # JUnit XML parser
        parser_test.go        # One example test using fixtures
      config/
        config.go             # quarantine.yml parsing
      runner/
        runner.go             # Test command execution
      github/
        client.go             # GitHub API interactions
      quarantine/
        state.go              # quarantine.json read/write/merge
  dashboard/
    CLAUDE.md                 # TypeScript-specific conventions and commands
    package.json
    tsconfig.json
    app/
      routes/                 # React Router v7 route modules
      lib/
        db.server.ts          # SQLite operations
        github.server.ts      # GitHub Artifacts polling
        ingest.server.ts      # Artifact JSON ingestion
```

### Makefile targets

```makefile
cli-build       # go build -o bin/quarantine ./cmd/quarantine
cli-test        # go test ./...
cli-lint        # golangci-lint run
dash-build      # cd dashboard && pnpm run build
dash-test       # cd dashboard && pnpm test
dash-lint       # cd dashboard && pnpm run lint
schemas-validate # validate fixtures against JSON schemas
test-all        # cli-test + dash-test + schemas-validate
```

### Per-component CLAUDE.md files

- `cli/CLAUDE.md`: Go conventions, build/test commands, package
  responsibilities, what not to touch
- `dashboard/CLAUDE.md`: TypeScript conventions, build/test commands, DB
  migration patterns

### Why this matters for agentic development

- Agents can immediately build, test, and lint without figuring out project
  structure
- One example test per component establishes the testing pattern for agents
  to follow
- Per-component CLAUDE.md scopes agent context (dashboard agent doesn't need
  Go knowledge)
- Makefile provides standard verification commands

**Resolved decisions:**
- **Go module path:** `github.com/mycargus/quarantine`
- **Dashboard bundler:** React Router v7 with Vite (default).
- **Go linting:** `golangci-lint` (de facto standard meta-linter for Go).
- **TypeScript linting/formatting:** Biome. Package manager: pnpm.
- **`go.work`:** Not needed. Single Go module (`cli/`) and separate
  TypeScript project (`dashboard/`). Independent build systems.

---

## 12. Claude Code Hooks for Scope Enforcement

**Goal:** Prevent agents from drifting out of v1 scope or violating architectural constraints.

**Deliverable:** `.claude/hooks.json` or hookify rules

**Priority:** P1

**Suggested hooks:**

PreToolUse (Edit/Write):
- Reject changes that add pytest, go test, or maven support without explicit approval
- Reject changes that import or reference frameworks not in v1 scope
- Reject changes that add direct dependencies between CLI and dashboard (they communicate only through GitHub)
- Warn if a file outside the current milestone's scope is being modified

PreToolUse (Bash):
- Warn before running destructive git commands

General guardrails:
- Token/secret detection: reject writes to quarantine.yml or config files that contain token patterns
- Scope creep detection: flag additions of Jira, Slack, or email integration code (v2+)

**Why this matters for agentic development:**
- Agents working autonomously may not remember scope boundaries from CLAUDE.md
- Hooks provide automated guardrails that catch mistakes before they're committed
- Particularly valuable when multiple agents work in parallel on different components

---

## M6 Pre-Implementation Tasks

These tasks should be completed before dashboard implementation begins.

### Artifact naming and dashboard discovery convention

**Priority:** P0 for M6
**Status:** Open

Define the integration contract between the CLI's local output and the dashboard's artifact polling:

- **Artifact name convention:** What should users name the artifact in `actions/upload-artifact`? Consider matrix builds (each job needs a unique name).
- **Dashboard search pattern:** How does the dashboard discover quarantine result artifacts among all artifacts in a repo?
- **Name prefix vs metadata:** Should the dashboard search by artifact name prefix (e.g., `quarantine-results*`) or by inspecting artifact contents?
- **Documentation:** The `quarantine init` command should output a workflow snippet with the correct artifact name for the user to paste.

The CLI output path is already defined: `.quarantine/results.json` (configurable via `--output`).

---

## Dependencies Between Tasks

```
Task 4 (config schema) ──┬──> Task 1 (CLI spec) ──┐
                         │                         │
                         ├─────────────────────────┤
Task 2 (API inventory) ──┤                         ├──> Task 3 (milestones)
Task 7 (error handling) ─┘                         │         ──> Task 8 (CLAUDE.md) [done]
                                                   │
Research: test_id, exit codes, CI artifacts ────────┘

Task 5 (sequence diagrams) -- can be done anytime, helps validate 1+2
Task 6 (test strategy) -- can be done alongside M1/M2
Task 9 (JSON schemas) ──> Task 10 (fixtures) ──> Task 11 (scaffolding)
Task 4 (config schema) ──> Task 9 (JSON schemas)
Task 12 (hooks) -- can be done alongside M1, benefits from Task 3 (milestones)
```

**Research completed (2026-03-17):**

1. **Exit codes (see `docs/ci-exit-code-research.md`):** 0/1/2 is safe and
   conventional. All major CI runners treat non-zero as failure with no
   special meaning for specific codes. GitLab CI and Buildkite support
   exit-code-specific `allow_failure`/`soft_fail`. Must override cobra's
   default exit-2 for usage errors via `cmd.SetFlagErrorFunc()`.

2. **JUnit XML / test_id (see `docs/junit-xml-research.md`):** No official
   schema. `test_id` = human-readable composite `file_path::classname::name`.
   `::` delimiter absent from all framework output. Jest needs recommended
   `jest-junit` config (suggested during `quarantine init`). Dashboard uses
   hash internally for indexing.

3. **CI artifact APIs (see `docs/ci-artifact-api-research.md`):** Per-provider
   dashboard pollers viable for v2. GitLab CI has best API (P1), Jenkins
   viable with caveats (P2), Buildkite clean (P3), CircleCI verbose (P4).
   All support an `ArtifactPoller` interface pattern.

Tasks 1, 2, 4, and 7 inform the milestones. The CLAUDE.md is best written
once milestones and conventions are defined. Sequence diagrams help validate
the CLI spec and API inventory. Test strategy can evolve alongside early
implementation.

Tasks 9, 10, and 11 form a chain enabling agentic development: JSON schemas
define the contracts, golden fixtures provide test data against those schemas,
and repo scaffolding wires everything together so agents can start coding
immediately. Task 9 depends on task 4 (config schema) since one of the JSON
schemas validates quarantine.yml.
