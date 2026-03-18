# Test Strategy

> Last updated: 2026-03-17
>
> How we test the Quarantine CLI and dashboard. The goal is confidence in
> correctness without over-engineering. This document covers what to test,
> how to test it, and the tools involved.

## Assertion Libraries

- **Go (CLI):** [`github.com/mycargus/riteway-golang`](https://github.com/mycargus/riteway-golang)
- **TypeScript (Dashboard):** [`github.com/paralleldrive/riteway`](https://github.com/paralleldrive/riteway)

Both libraries enforce a consistent assertion style: `Given / Should / Actual / Expected`. This makes test intent immediately clear and failure messages self-documenting.

## Coverage Threshold

Not specified. Revisit during implementation once there is enough code to establish a meaningful baseline.

---

## CLI Unit Tests (Go)

Unit tests cover the CLI's core logic in isolation. No network calls, no filesystem side effects beyond temp files.

### JUnit XML Parsing

- Parse valid output from each v1 framework (Jest, RSpec, Vitest)
- Handle malformed XML (truncated, invalid encoding, missing attributes)
- Merge results from multiple XML files (parallel runner output like Jest `--shard`, RSpec `parallel_tests`)
- Handle empty XML (zero test cases)
- Handle XML with only skipped tests

Test fixtures live in `testdata/junit-xml/{jest,rspec,vitest}/`. Tests compare parsed output against expected JSON in `testdata/expected/`.

### `test_id` Construction

- Construct `file_path::classname::name` from JUnit XML attributes for each framework
- Handle missing `file_path` attribute (Jest without `addFileAttribute`)
- Handle special characters in test names (parentheses, quotes, colons)
- Verify `::` delimiter is never ambiguous with actual test content

### `quarantine.json` Read/Write/Merge

- Deserialize and serialize `quarantine.json` correctly
- Union merge: concurrent builds that each detect different flaky tests produce a combined result
- Quarantine wins on conflict: if one build quarantines a test and another does not, the test stays quarantined (ADR-012)
- Remove unquarantined tests (issue closed) from state
- Handle empty state (first run, no quarantined tests)

### Framework-Specific Exclusion Flag Construction

- **Jest:** `--testPathIgnorePatterns` for whole-file exclusion, `--testNamePattern` with negative lookahead for individual tests, combined when both apply
- **RSpec:** File-level exclusion by omitting spec files from the file list
- **Vitest:** `--exclude` for file-level, `-t` with negative lookahead for individual tests
- Regex special characters in test names are escaped correctly
- No exclusion flags when quarantine list is empty

### Framework-Specific Rerun Command Construction

- **Jest:** `jest --testNamePattern "{name}"`
- **RSpec:** `rspec -e "{name}"`
- **Vitest:** `vitest run --reporter=junit {file} -t "{name}"`
- Custom `rerun_command` template with `{name}`, `{classname}`, `{file}` placeholders
- Placeholder values are properly escaped for shell execution

### Config File Parsing and Validation

- Parse valid `quarantine.yml` with all fields
- Parse minimal `quarantine.yml` (only `version` and `framework`)
- Apply defaults for optional fields (`retries: 3`, framework-specific `junitxml`)
- Reject unknown `framework` values
- Reject `retries` outside 1-10 range
- Forward-compatible field validation: reject unsupported `issue_tracker` values (only `github` in v1), reject unsupported `labels` (only `[quarantine]` in v1), reject unsupported `notifications` keys (only `github_pr_comment` in v1)
- Warn on unknown fields (do not reject)
- Reject config files with tokens or secrets

### Exit Code Determination

- Exit 0: all tests passed
- Exit 0: only flaky failures (detected and quarantined)
- Exit 0: degraded mode but tests passed
- Exit 1: at least one genuine failure after retries
- Exit 2: not initialized, invalid config, bad flags
- Exit 2: infrastructure failure with `--strict`
- Exit 2: `--verbose` and `--quiet` both set

### Exclude Pattern Matching

- Glob patterns match against `test_id` (`file_path::classname::name`)
- `**` matches across path separators in the `file_path` portion
- `*` matches within a single segment
- Patterns from config and `--exclude` flags are merged
- Excluded tests are completely ignored (no retry, no quarantine, no issue)

---

## CLI Integration Tests

### Against Mocked GitHub API (CI)

These run on every CI build. A mock HTTP server stands in for the GitHub API, returning canned responses for Contents API, Issues API, Search API, etc.

- Full `quarantine run` flow: read state, execute tests, parse XML, retry failures, update state, create issues, post PR comment
- `quarantine init` flow: interactive prompts produce config, branch creation via API
- `quarantine run` without prior `init`: refuses with exit 2
- `quarantine doctor`: reports errors and warnings correctly
- Degraded mode: mock returns 503 for Contents API, CLI falls back to cache then runs without state
- `--strict` mode: mock returns errors, CLI exits 2 instead of degrading
- `--dry-run`: no API writes occur

### Against Real GitHub API (Periodic)

These run on a schedule (not on every push) against a dedicated test repository. They validate that the CLI works with the real GitHub API, catching issues that mocks cannot (API changes, edge cases in response formats, rate limiting behavior).

- Full end-to-end: `quarantine run` against the test repo with real flaky tests
- CAS conflict simulation: two processes run concurrently, both updating `quarantine.json`. One gets a 409, retries, and the union merge produces correct state.
- Issue creation and dedup: verify issues are created with deterministic labels and duplicates are avoided
- PR comment post and update: verify the hidden HTML marker `<!-- quarantine-bot -->` is used to find and update existing comments
- Branch creation: `quarantine init` creates the `quarantine/state` branch on a fresh repo

### End-to-End with Real Test Suites

- Run a real Jest test suite (with a known flaky test) through `quarantine run`, verify the flaky test is detected, quarantined, and the exit code is 0
- Same for RSpec and Vitest
- These use small test projects in the test repo (see "Test Repo for Integration" below)

---

## Dashboard Unit Tests (TypeScript)

### Artifact JSON Parsing

- Parse valid test result JSON matching `test-result.schema.json`
- Handle missing optional fields
- Reject malformed JSON (log warning, skip artifact)

### SQLite Query Correctness

- Upsert test runs and test results correctly
- Query flaky test trends (count over time, per-project)
- Query quarantine duration (time between quarantine and unquarantine events)
- Cross-repo rollup queries return correct aggregations
- Handle empty database (first run, no data)

### Polling Logic

- Debounce: on-demand pull does not fire more than once per repo per 5 minutes
- ETag handling: conditional requests skip download when content has not changed
- Error resilience: a failed poll cycle does not block subsequent cycles

---

## Dashboard Integration Tests

### Artifact Ingestion from Mock GitHub API

- Mock the Artifacts API to return a list of artifacts, then serve artifact content on download
- Verify artifacts are downloaded, parsed, and inserted into SQLite
- Verify `last_synced` and `last_etag` are updated on the project record
- Verify malformed artifacts are skipped with a warning (not a crash)

### Web UI Rendering

- Loader data (from SQLite queries) renders into the expected page structure
- Project listing page shows projects with test run counts
- Quarantined test list shows test names, issue links, quarantine dates
- Empty states render correctly (no projects, no test runs, no quarantined tests)

---

## Contract Tests

Contract tests validate that the CLI and dashboard agree on data formats. They run as part of `make schemas-validate` and in CI.

| Schema | Validates |
|--------|-----------|
| `schemas/test-result.schema.json` | CLI output in `.quarantine/results.json` and dashboard's artifact ingestion input |
| `schemas/quarantine-state.schema.json` | CLI read/write of `quarantine.json` on the `quarantine/state` branch |
| `schemas/quarantine-config.schema.json` | CLI parsing and validation of `quarantine.yml` |

### How they work

- **CLI side (Go):** After each unit/integration test that produces a `results.json` or `quarantine.json`, validate the output against the corresponding JSON Schema using `santhosh-tekuri/jsonschema`.
- **Dashboard side (TypeScript):** Artifact parsing tests validate input against `test-result.schema.json` using `ajv`.
- **Golden fixtures:** Files in `testdata/expected/` are validated against schemas as a build step. If a schema changes, fixture tests break immediately.

Contract tests are the primary mechanism ensuring the two independently developed components (CLI and dashboard) will integrate correctly.

---

## Test Repo for Integration

A dedicated GitHub repository (e.g., `mycargus/quarantine-test-suite`) provides a controlled environment for integration and end-to-end testing.

### Contents

The repo contains three small test projects, one per v1 framework:

- **Jest project:** A few passing tests plus tests that fail ~30% of the time (`Math.random() < 0.3`)
- **RSpec project:** Same pattern -- deterministic failures at a predetermined rate
- **Vitest project:** Same pattern

### Purpose

- Validates the full end-to-end flow: `quarantine init`, `quarantine run`, flaky detection, state update, issue creation, PR comment
- Tests real GitHub API interactions (Contents API, Issues API, Search API)
- CAS conflict simulation with concurrent CI jobs
- Verifies framework-specific JUnit XML parsing against real (not hand-crafted) output

### Usage

- **Periodic CI job:** Runs on a schedule (e.g., daily or weekly), not on every push to the Quarantine repo
- **Manual trigger:** Can be run on-demand for debugging or verifying a release
- The test repo is separate from the Quarantine source repo to avoid circular dependencies

---

## Test Organization

### CLI (`cli/`)

```
cli/
  internal/
    parser/
      parser_test.go          # JUnit XML parsing, test_id construction
    config/
      config_test.go          # quarantine.yml parsing and validation
    quarantine/
      state_test.go           # quarantine.json read/write/merge
    runner/
      runner_test.go          # Rerun command construction, exclusion flags
      exitcode_test.go        # Exit code determination
    github/
      client_test.go          # GitHub API client (against mock server)
  test/
    integration_test.go       # Full CLI flow against mock GitHub API
    e2e_test.go               # End-to-end against real test repo (build tag: e2e)
```

Integration and e2e tests use Go build tags so they can be excluded from `go test ./...`:

- `go test ./...` -- runs unit tests only
- `go test -tags=integration ./...` -- adds integration tests (mock GitHub API)
- `go test -tags=e2e ./...` -- adds end-to-end tests (real GitHub API, requires token)

### Dashboard (`dashboard/`)

```
dashboard/
  app/
    lib/
      ingest.server.test.ts    # Artifact parsing, SQLite ingestion
      db.server.test.ts        # SQLite query correctness
      github.server.test.ts    # Polling logic
    routes/
      _index.test.tsx          # Page rendering
  test/
    integration/
      ingestion.test.ts        # Full ingestion pipeline against mock API
```

### Makefile Targets

```makefile
cli-test              # go test ./...
cli-test-integration  # go test -tags=integration ./...
cli-test-e2e          # go test -tags=e2e ./... (requires QUARANTINE_GITHUB_TOKEN)
dash-test             # cd dashboard && pnpm test
schemas-validate      # validate golden fixtures against JSON schemas
test-all              # cli-test + dash-test + schemas-validate
```

---

## What We Deliberately Do Not Test

- **Frameworks beyond v1 scope:** No pytest, Go, Maven test fixtures or parsing tests.
- **GitHub App auth flow:** v2+ concern.
- **Dashboard write-back to GitHub:** The dashboard is read-only in v1.
- **Performance benchmarks:** Not needed at v1 scale. Revisit if quarantine.json approaches thousands of entries.
- **Browser-level E2E tests (Playwright, Cypress):** Dashboard integration tests cover loader-to-render. Full browser tests are v2+ if warranted.

---

*References: [pre-implementation-tasks.md](pre-implementation-tasks.md) (task 6),
[cli-spec.md](cli-spec.md), [architecture.md](architecture.md),
[ADR-012](adr/012-concurrency-model.md), [ADR-016](adr/016-v1-framework-scope.md).*
