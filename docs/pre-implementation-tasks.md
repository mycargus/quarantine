# Pre-Implementation Tasks

> Last updated: 2026-03-14
>
> These tasks should be completed before (or during early) implementation to
> reduce ambiguity and avoid rework. They are ordered by impact on
> implementation speed.

## Status

| # | Task | Status | Priority |
|---|------|--------|----------|
| 1 | CLI interface specification | Not started | P0 |
| 2 | GitHub API interaction inventory | Not started | P0 |
| 3 | Implementation milestones | Not started | P0 |
| 4 | quarantine.yml full schema | Not started | P0 |
| 5 | Sequence diagrams for key flows | Not started | P1 |
| 6 | Test strategy for the project itself | Not started | P1 |
| 7 | Error handling strategy | Not started | P1 |
| 8 | CLAUDE.md for the repo | Done | P1 |
| 9 | JSON interface schemas | Not started | P0 |
| 10 | Golden test fixtures | Not started | P0 |
| 11 | Repo scaffolding and build infrastructure | Not started | P0 |
| 12 | Claude Code hooks for scope enforcement | Not started | P1 |

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
quarantine validate [flags]
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

**Specific open questions to resolve:**
- `quarantine run`: What flags? At minimum: `--retries`, `--config`,
  `--junitxml` (glob pattern), `--dry-run`, `--verbose`. What else?
- `quarantine run` exit codes: 0 = all tests pass (quarantined failures
  suppressed), 1 = real failures exist, 2 = quarantine itself errored (but
  still ran tests). Are there others?
- `quarantine validate`: What does it output? Resolved config including
  auto-detected values? Errors? Warnings?
- Should there be a `quarantine init` command to create quarantine.yml and
  the quarantine/state branch?
- Output verbosity levels: default, `--verbose`, `--quiet`, `--json`?
- PR comment format: exact markdown template

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

**Specific open questions to resolve:**
- Artifact upload: does the CLI use the `actions/upload-artifact` action (only
  works in GitHub Actions) or the REST API (works anywhere but more complex)?
  What about non-GitHub-Actions CI (v2)?
- Can the CLI determine if it's running inside a GitHub Actions workflow
  (check for `GITHUB_ACTIONS` env var)?
- For PR comments: how does the CLI determine the PR number? From
  `GITHUB_REF`, `GITHUB_EVENT_PATH`, or a flag?
- Rate limit tracking: should the CLI read `X-RateLimit-Remaining` headers
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
      - quarantine validate command
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

**Open questions:**
- Is this the right granularity?
- Should any milestones be split or merged?
- What's the minimum viable demo for stakeholders at your employer?

---

## 4. quarantine.yml Full Schema

**Goal:** Define every configuration field, its type, default, validation
rules, and behavior.

**Deliverable:** `docs/config-schema.md`

### What to define

```yaml
# quarantine.yml -- full schema

version: 1                          # Required. Schema version.

framework: jest                     # Optional. Auto-detected if omitted.
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

storage:
  backend: branch                   # Optional. Default: branch.
                                    # Valid: branch, actions-cache
  branch: quarantine/state          # Optional. Default: quarantine/state.
                                    # Only used when backend: branch.

rerun_command: ""                   # Optional. Override auto-detected rerun
                                    # command template. Uses {name}, {classname},
                                    # {file} placeholders.
```

**Open questions:**
- Should the config support per-framework overrides (e.g., different retry
  count for slow integration tests)?
- Should there be an `exclude` field to ignore specific tests or patterns?
- Should there be an `issue` section configuring label names, assignees, etc.?
- Where does the GitHub token come from -- only env var, or also config?
  (Recommendation: env var only, never in config file, to avoid committing
  secrets.)

---

## 5. Sequence Diagrams for Key Flows

**Goal:** Step-by-step sequence diagrams for the main flows, showing exact
order of operations, API calls, decision points, and error paths.

**Deliverable:** `docs/sequence-diagrams.md` (using Mermaid or text-based
diagrams)

### Flows to diagram

1. **Happy path: flaky test detected and quarantined**
   CLI starts -> read quarantine.json -> run tests -> parse XML -> test fails
   -> retry test -> passes on retry -> flag as flaky -> update quarantine.json
   (with CAS) -> create GitHub issue (with dedup check) -> post PR comment ->
   upload artifact -> exit 0

2. **Happy path: quarantined test suppressed**
   CLI starts -> read quarantine.json -> run tests -> quarantined test fails ->
   suppress (no retry) -> check if issue closed -> still open -> rewrite XML ->
   exit 0

3. **Quarantined test's issue is closed**
   CLI starts -> read quarantine.json -> check issue status -> issue closed ->
   remove from quarantine.json -> run tests -> test runs normally (not
   suppressed)

4. **Concurrent builds: CAS conflict on quarantine.json**
   Build A reads (SHA1) -> Build B reads (SHA1) -> Build A writes (ok, SHA2)
   -> Build B writes (409) -> Build B re-reads (SHA2) -> Build B merges ->
   Build B writes (ok, SHA3)

5. **Degraded mode: GitHub API unreachable**
   CLI starts -> try read quarantine.json -> timeout -> try Actions cache ->
   cache hit -> use cached data -> run tests -> try write quarantine.json ->
   timeout -> try upload artifact -> timeout -> log warnings -> exit based on
   test results only

6. **Degraded mode: no cache, no API (first run + outage)**
   CLI starts -> try read -> fail -> try cache -> miss -> log warning -> run
   tests without quarantine -> detect flaky tests -> write to
   .quarantine/pending.json -> exit based on all test results (no suppression)

7. **Dashboard: artifact ingestion**
   Poll timer fires -> list artifacts since last poll -> for each new artifact
   -> download -> parse JSON -> upsert into SQLite -> update last-poll
   timestamp

### Open questions
- Should diagrams use Mermaid (renderable in GitHub) or ASCII?

---

## 6. Test Strategy for the Project Itself

**Goal:** Define how we test the Quarantine CLI and dashboard so we have
confidence in correctness without over-engineering the test suite.

**Deliverable:** `docs/test-strategy.md`

### Areas to cover

**CLI unit tests (Go):**
- JUnit XML parsing (various framework outputs, malformed XML, multiple files)
- quarantine.json read/write/merge logic
- Framework auto-detection from XML
- Rerun command construction per framework
- Config file parsing and validation
- Exit code determination logic

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

### Open questions
- Real GitHub API or mocked? Real gives more confidence but is slower, flakier
  (ironic), and needs a test repo. Mocked is faster but may diverge from
  reality. Recommendation: mocked for CI, real for a periodic integration
  test suite.
- Should the project use its own tool (Quarantine) to manage its own flaky
  tests? (Dogfooding.)
- What coverage threshold, if any?

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
| 404 Not Found | Branch/repo doesn't exist | First run: create branch. Otherwise: log error, degraded mode |
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

### Open questions
- Should degraded mode be surfaced to the user beyond log output? E.g., a
  GitHub commit status check "Quarantine: degraded (GitHub API unreachable)"?
- Should the CLI have a `--strict` mode that DOES fail on infrastructure
  errors (for debugging)?

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

### Open questions

- Should schemas use JSON Schema draft 2020-12 or an earlier draft for
  broader tooling support?

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
      flaky-on-retry.xml       # test that fails then passes
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

### Open questions

- Should fixtures include parameterized test output? (e.g., Jest `test.each`,
  RSpec shared examples)
- Should we include output from parallel runners (jest with --shard, rspec
  with parallel_tests)?

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
dash-build      # cd dashboard && npm run build
dash-test       # cd dashboard && npm test
dash-lint       # cd dashboard && npm run lint
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

### Open questions

- Go module path: `github.com/{org}/quarantine`? What org/user name?
- Dashboard: React Router v7 with Vite as bundler? (This is the default)
- Linting: golangci-lint for Go, ESLint + Prettier for TypeScript?
- Should there be a root-level `go.work` file if CLI and dashboard are in the
  same repo?

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

## Dependencies Between Tasks

```
Task 4 (config schema) ──┐
Task 1 (CLI spec)  ──────┤
Task 2 (API inventory) ──┼──> Task 3 (milestones) ──> Task 8 (CLAUDE.md) [done]
Task 7 (error handling) ─┘
Task 5 (sequence diagrams) -- can be done anytime, helps validate 1+2
Task 6 (test strategy) -- can be done alongside M1/M2
Task 9 (JSON schemas) ──> Task 10 (fixtures) ──> Task 11 (scaffolding)
Task 4 (config schema) ──> Task 9 (JSON schemas)
Task 12 (hooks) -- can be done alongside M1, benefits from Task 3 (milestones)
```

Tasks 1, 2, 4, and 7 inform the milestones. The CLAUDE.md is best written
once milestones and conventions are defined. Sequence diagrams help validate
the CLI spec and API inventory. Test strategy can evolve alongside early
implementation.

Tasks 9, 10, and 11 form a chain enabling agentic development: JSON schemas
define the contracts, golden fixtures provide test data against those schemas,
and repo scaffolding wires everything together so agents can start coding
immediately. Task 9 depends on task 4 (config schema) since one of the JSON
schemas validates quarantine.yml.
