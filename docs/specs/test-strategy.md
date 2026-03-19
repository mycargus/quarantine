# Test Strategy

> Last updated: 2026-03-18
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

Exercise the full system — compiled binary + real GitHub API — against a dedicated test repository. Written in JavaScript (Vitest + [`riteway`](https://github.com/paralleldrive/riteway) assertions) and located in `e2e/` at the repository root. These catch issues that mocks cannot: API behavior changes, response format drift, auth edge cases, and real-world integration bugs.

Run on the main branch and on PRs from within the repository (not forks, which cannot access secrets). See `e2e/README.md` for setup instructions.

### Contract Tests

Validate that shared data formats (JSON schemas) are respected by both producers and consumers. Golden fixture files are validated against schemas as a build step — if a schema changes, tests break immediately on both sides.

## Test Organization Conventions

- **Go:** Build tags separate test layers. `go test ./...` runs units only. `-tags=integration` adds integration tests.
- **TypeScript (Dashboard):** Standard test runner. Integration tests in a separate `test/integration/` directory.
- **JavaScript (E2E):** Vitest in `e2e/`. Uses RITEway-style `assert` helper. Run with `cd e2e && pnpm test`.
- **Test data:** Fixtures live in `testdata/` directories adjacent to the code they test.
- **Coverage threshold:** Not specified. Revisit once there is enough code to establish a meaningful baseline.

## What We Deliberately Skip

- **Browser-level E2E tests:** Integration tests cover loader-to-render. Full browser tests add cost without proportional confidence at current scale.
- **Performance benchmarks:** Not needed at v1 scale. Revisit when data volumes warrant it.
- **Exhaustive negative testing of external APIs:** We trust external API contracts and test our error handling, not their failure modes.

## Development Tools

**`/mikey:tdd`** — Test-driven development agent. Takes a scenario file and implements it interactively, generating tests and code that follow Functional Core / Imperative Shell principles. Use when implementing a new milestone scenario.

**`/mikey:testify`** — Test quality auditor. Reviews code for design issues (mixed I/O and logic), excessive mocking, implementation detail testing, and missing error-path coverage. Use after implementing scenarios to validate alignment with this strategy.

---

*References: [architecture.md](../planning/architecture.md), scenario files in `docs/scenarios/`, Claude Code skills: [`/mikey:tdd`, `/mikey:testify`](https://github.com/mycargus/mikey-claude-plugins).*
