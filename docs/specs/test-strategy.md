# Test Strategy

> Last updated: 2026-03-28
>
> Guiding principles for how we test. The goal is confidence in correctness
> without over-engineering. Specific test cases live in scenario files
> (`docs/scenarios/`) and in the tests themselves — this document covers
> *why* and *how*, not *what*.

## Assertion Style

All tests use the RITEway assertion pattern: `Given / Should / Actual / Expected`.

- **Go (CLI):** [`github.com/mycargus/riteway-golang`](https://github.com/mycargus/riteway-golang)
- **TypeScript (Dashboard):** [`github.com/paralleldrive/riteway`](https://github.com/paralleldrive/riteway)

This makes test intent immediately clear and failure messages self-documenting. Every assertion answers: "Given [context], should [expectation]."

## Guiding Principles

### 1. Functional Core, Imperative Shell

Separate pure logic from I/O. Pure functions (parsing, merging, decision-making) are unit tested exhaustively. I/O boundaries (API calls, file system, process execution) are thin and tested at the integration level.

**Validation:** After implementing scenarios, use `/mikey:testify` to audit code design and verify adherence to this principle. The testify agent identifies mixed logic/I/O, excessive mocking, and missing error-path tests.

### 2. Test the Interface, Not the Implementation

Tests assert on inputs and outputs of public interfaces. Internal data structures, private methods, and implementation details are not tested directly. Refactoring internals should not break tests.

### 3. No Mocks in Unit Tests

Unit tests operate on pure functions that don't need mocks. If a unit test requires a mock, that's a design smell — the code under test is mixing logic with I/O. Extract the logic into a pure function.

### 4. Real Dependencies at Integration Boundaries

Integration tests use real instances of internal dependencies (real SQLite, real file system) and mock only external services (GitHub API). This catches the integration bugs that mocks hide.

### 5. Contract Tests Bridge Components

When two independently developed components exchange data (e.g., CLI produces JSON that the dashboard consumes), a shared JSON Schema is the contract. Both sides validate against the schema in their own test suites. This is the primary mechanism ensuring components integrate correctly without end-to-end coupling.

### 6. Scenarios Drive Coverage

Given/When/Then scenario files (`docs/scenarios/`) are the source of truth for what behaviors exist. Tests implement these scenarios. If a behavior isn't in a scenario, it probably doesn't need a test. If it's in a scenario, it must have a test.

**Workflow:** Use `/mikey:tdd <scenario-file>` to implement scenarios interactively. The TDD agent guides you through each scenario, generating test code and implementation following Functional Core / Imperative Shell design automatically.

### 7. Fail Fast, Fail Clearly

Tests should fail immediately on the first broken assertion with a message that tells you what went wrong without reading the test code. The RITEway pattern enforces this — every failure includes the given context and expected vs actual values.

### 8. No Flaky Tests (Practice What We Preach)

Our own test suite must have zero flaky tests. Any test that fails intermittently is either fixed immediately or deleted. Non-determinism in tests comes from shared state, time, randomness, or network — eliminate or control all four.

## Test Layers

| Layer | Scope | External I/O | Runs on |
|-------|-------|-------------|---------|
| **Unit** | Pure functions in isolation | None | Every commit |
| **Integration** | Full component flows against mock external APIs | Mock HTTP server | Every commit |
| **E2E** | Full system flows against real external dependencies | Real GitHub API, real binary | Main branch / manual |
| **Contract** | Schema validation of shared data formats | None | Every commit |

### Unit Tests

Cover all pure logic: parsing, validation, merging, decision-making, command construction. No network, no disk I/O beyond temp files. These are fast, deterministic, and form the bulk of the test suite.

### Integration Tests

Exercise full component flows end-to-end within the component boundary. A mock HTTP server stands in for external APIs, returning canned responses. These validate that the I/O shell correctly orchestrates the functional core.

### E2E Tests

Exercise the full system — compiled binary + real GitHub API — against a dedicated test repository. Written in JavaScript (Vitest + [`riteway`](https://github.com/paralleldrive/riteway) assertions) and located in `test/e2e/` at the repository root. These catch issues that mocks cannot: API behavior changes, response format drift, auth edge cases, and real-world integration bugs.

Run on the main branch and on PRs from within the repository (not forks, which cannot access secrets). See `test/e2e/README.md` for setup instructions.

### Contract Tests

Validate that shared data formats (JSON schemas) are respected by both producers and consumers. Golden fixture files are validated against schemas as a build step — if a schema changes, tests break immediately on both sides.

## Test Organization Conventions

- **Go:** Build tags separate test layers. `go test ./...` runs units only. `-tags=integration` adds integration tests.
- **TypeScript (Dashboard):** Standard test runner. Integration tests in a separate `test/integration/` directory.
- **JavaScript (E2E):** Vitest in `test/e2e/`. Uses RITEway-style `assert` helper. Run with `make e2e-test`.
- **Test data:** Fixtures live in `testdata/` directories adjacent to the code they test.
- **Coverage threshold:** Not specified. Revisit once there is enough code to establish a meaningful baseline.

## Mutation Testing

Coverage tells you which lines are executed; mutation testing tells you whether those lines are actually verified. A test that executes a branch without asserting on its effect will pass even when the branch is deleted or inverted — that's the gap mutation testing closes.

### Tool — `/test-mutation`

Mutation testing uses the `/claude-swe-workflows:test-mutation` skill, part of the [`claude-swe-workflows`](https://github.com/chrisallenlane/claude-swe-workflows) Claude Code plugin. Unlike traditional mutation tools (gremlins, Stryker) that only report survivors, this skill finds survivors **and writes new tests to kill them**.

```bash
make cli-mutate    # run /test-mutation scoped to the CLI
/test-mutation     # run interactively from Claude Code (prompts for scope)
```

The skill:
- Discovers all source modules, prompts for scope, then runs in autopilot
- Applies mutations one at a time, spawns a language-appropriate SME agent to write targeted tests for each survivor
- Verifies each kill by re-applying the mutation with the new test in place
- Commits after each module with a descriptive message
- Tracks progress in `.test-mutations.json` (gitignored) so sessions can resume

**When to run:** After implementing a new milestone, or whenever you want to improve test depth in a specific area.

### CI

Mutation testing does not run in CI. It is a periodic quality improvement activity, not a PR gate.

### Known false positives

Mutants that cannot be killed by any realistic test input:

- `config.go` — YAML mapping loop bound (`i+1 < len` → `i+1 <= len`). Only differs for odd-length node arrays, which valid YAML cannot produce.
- `state.go` — Timestamp `<` → `<=` in `MergeAt`. When timestamps are equal, `earliest = existing.FirstFlakyAt` is a no-op regardless of which branch is taken.
- `run.go` — `Summary.Total > 0` → `>= 0` in `allTestsQuarantined`. An earlier guard already returns for `Total == 0`, making the two conditions equivalent for all reachable inputs.

### What mutation testing does not replace

Mutation testing validates that existing tests are meaningful. It does not replace the scenario-driven process for deciding *which* behaviors need tests in the first place. Use `/mikey:tdd` to add new behaviors, then use mutation results to verify the tests you wrote actually catch regressions.

## What We Deliberately Skip

- **Browser-level E2E tests:** Integration tests cover loader-to-render. Full browser tests add cost without proportional confidence at current scale.
- **Performance benchmarks:** Not needed at v1 scale. Revisit when data volumes warrant it.
- **Exhaustive negative testing of external APIs:** We trust external API contracts and test our error handling, not their failure modes.

## Development Tools

**`/mikey:tdd`** — Test-driven development agent. Takes a scenario file and implements it interactively, generating tests and code that follow Functional Core / Imperative Shell principles. Use when implementing a new milestone scenario.

**`/mikey:testify`** — Test quality auditor. Reviews code for design issues (mixed I/O and logic), excessive mocking, implementation detail testing, and missing error-path coverage. Use after implementing scenarios to validate alignment with this strategy.

**`/claude-swe-workflows:test-mutation`** — Mutation testing workflow. Finds surviving mutations across all source modules and writes targeted tests to kill them. Multi-session with progress tracking. From the [`claude-swe-workflows`](https://github.com/chrisallenlane/claude-swe-workflows) plugin. Use after implementing a milestone.

---

*References: [architecture.md](../planning/architecture.md), scenario files in `docs/scenarios/`, Claude Code skills: [`/mikey:tdd`, `/mikey:testify`](https://github.com/mycargus/mikey-claude-plugins), [`/test-mutation`](https://github.com/chrisallenlane/claude-swe-workflows).*
