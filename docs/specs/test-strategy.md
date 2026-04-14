# Test Strategy

> Last updated: 2026-04-13
>
> Guiding principles for how we test. The goal is confidence in correctness
> without over-engineering. Specific test cases live in scenario files
> (`docs/scenarios/`) and in the tests themselves — this document covers
> *why* and *how*, not *what*.
>
> Layer definitions and terminology follow Mikey's Test Pyramid (MTP).
> The MTP defines four layers — Unit, Interface, Contract, E2E — each
> with distinct scope, allowed dependencies, and role in the pyramid.

## Assertion Style

All tests use the RITEway assertion pattern: `Given / Should / Actual / Expected`.

- **Go (CLI):** [`github.com/mycargus/riteway-golang`](https://github.com/mycargus/riteway-golang)
- **TypeScript (Dashboard):** [`github.com/paralleldrive/riteway`](https://github.com/paralleldrive/riteway)

This makes test intent immediately clear and failure messages self-documenting. Every assertion answers: "Given [context], should [expectation]."

## Guiding Principles

### 1. Functional Core, Imperative Shell

Separate pure logic from I/O. Pure functions (parsing, merging, decision-making) are unit tested exhaustively. I/O boundaries (API calls, file system, process execution) are thin and tested at the interface level.

**Validation:** After implementing scenarios, use `/mikey:testify` to audit code design and verify adherence to this principle. The testify agent identifies mixed logic/I/O, excessive mocking, and missing error-path tests.

### 2. Test the Interface, Not the Implementation

Tests assert on inputs and outputs of public interfaces. Internal data structures, private methods, and implementation details are not tested directly. Refactoring internals should not break tests.

### 3. No Mocks in Unit Tests

Unit tests operate on pure functions that don't need mocks. If a unit test requires a mock, that's a design smell — the code under test is mixing logic with I/O. Extract the logic into a pure function.

### 4. Real Dependencies at Interface Boundaries

Interface tests use real instances of internal dependencies (real SQLite, real file system) and mock only external services (GitHub API). This catches the bugs that mocks hide at the boundaries where components meet real infrastructure.

### 5. Contract Tests Bridge Components

When two independently developed components exchange data (e.g., CLI produces JSON that the dashboard consumes), a shared JSON Schema is the contract. Both sides validate against the schema in their own test suites. This is the primary mechanism ensuring components integrate correctly without end-to-end coupling. See [contracts.md](contracts.md) for a complete inventory of every producer-consumer boundary and its validation status.

### 6. Scenarios Drive Coverage

Given/When/Then scenario files (`docs/scenarios/`) are the source of truth for what behaviors exist. Tests implement these scenarios. If a behavior isn't in a scenario, it probably doesn't need a test. If it's in a scenario, it must have a test.

**Workflow:** Use `/mikey:tdd <scenario-file>` to implement scenarios interactively. The TDD agent guides you through each scenario, generating test code and implementation following Functional Core / Imperative Shell design automatically.

**Verification:** `/verify-milestone` audits scenario-to-test traceability as part of milestone acceptance. It reads each scenario file, searches for corresponding tests by scenario number and title, and reports covered vs missing scenarios. This is the enforcement mechanism — run it after implementing a milestone to confirm all scenarios have tests.

### 7. Fail Fast, Fail Clearly

Tests should fail immediately on the first broken assertion with a message that tells you what went wrong without reading the test code. The RITEway pattern enforces this — every failure includes the given context and expected vs actual values.

### 8. No Flaky Tests (Practice What We Preach)

Our own test suite must have zero flaky tests. Any test that fails intermittently is either fixed immediately or deleted. Non-determinism in tests comes from shared state, time, randomness, or network — eliminate or control all four.

### 9. Replicate Bugs at the Lowest Layer

When a bug surfaces at a higher layer (E2E or Interface), replicate it with a lower-layer test before fixing. The lower-layer test ensures the bug stays dead through future refactors, runs faster, and pinpoints the root cause. A high-layer failure signals both a bug in the code and a gap in the lower-layer tests.

## Test Layers

| Layer | Scope | External I/O | Runs on |
|-------|-------|-------------|---------|
| **Unit** | Pure functions in isolation | None | Every commit |
| **Interface** | Single-component flows through the public interface (CLI binary or HTTP routes) | Mock HTTP server | Every commit |
| **Contract** | Schema validation of shared data formats | None | Every commit |
| **E2E** | Full system flows against real external dependencies | Real GitHub API, real binary | Main branch / manual |

### Unit Tests

Cover all pure logic: parsing, validation, merging, decision-making, command construction. No network, no disk I/O beyond temp files. These are fast, deterministic, and form the bulk of the test suite.

### Interface Tests

Exercise a single component through its public interface — the CLI binary or HTTP API routes — within the component boundary. External APIs are replaced by a mock HTTP server returning canned responses. These validate that the I/O shell correctly orchestrates the functional core through the same entry points a real user would exercise.

**CLI:** Tests invoke the compiled `quarantine` binary via `exec.Command` with mock HTTP servers standing in for GitHub.

**Dashboard:** Tests make HTTP requests to Remix route endpoints with the GitHub Artifacts API stubbed, exercising the full framework stack (routing, loaders, rendering) with real SQLite.

### E2E Tests

Exercise the full system — compiled binary + real GitHub API — against dedicated test repositories. E2E tests are written in JavaScript (Vitest + [`riteway`](https://github.com/paralleldrive/riteway) assertions) and located in `test/e2e/` at the repository root. These catch issues that mocks cannot: API behavior changes, response format drift, auth edge cases, and real-world integration bugs.

**Test fixtures:**
- `mycargus/quarantine-test-fixture` — PAT-based CLI and dashboard E2E tests (existing `e2e` CI job)
- `mycargus/quarantine-app-test-fixture` — App-based E2E tests: token exchange, installation discovery, artifact polling with App tokens, CLI with App tokens (separate `e2e-app` CI job). The fixture repo runs quarantine with deliberately flaky tests to produce real quarantine result artifacts.

Run on the main branch and on PRs from within the repository (not forks, which cannot access secrets). See `test/e2e/README.md` for setup instructions.

### Contract Tests

Validate that shared data formats are respected by both producers and consumers. Contract tests use Prism to validate against vendored OpenAPI specs offline, with no credentials required. If a schema changes, tests break immediately on both sides. See [contracts.md](contracts.md) for the full inventory of producer-consumer boundaries.

## Test Organization Conventions

- **Go:** `make cli-test` runs all CLI tests (unit and interface).
- **TypeScript (Dashboard):** Unit tests are colocated with source files (`dashboard/app/**/*.test.ts`). Interface tests live in `dashboard/test/*.interface.test.ts` — these make HTTP requests to Remix routes with external APIs (GitHub, etc.) stubbed, exercising the full framework stack with real SQLite. See `dashboard/test/README.md` for scope.
- **JavaScript (E2E):** Vitest in `test/e2e/`. Uses RITEway-style `assert` helper. Run with `make e2e-test`.
- **Test data:** Fixtures live in `testdata/` directories adjacent to the code they test.
- **Coverage:** Measure but don't enforce thresholds. Code coverage ≠ requirements coverage — 100% line coverage can still hide bugs if the wrong behaviors are tested. The value of coverage data is knowing which files and lines *aren't* tested, which is especially useful in non-compiled languages where untested code paths can harbor undetected errors. Use coverage reports as a diagnostic tool, not a gate.

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

- **Browser-level testing:** Interface and E2E tests exercise the dashboard via HTTP requests to route endpoints, not through a browser. API-level testing provides sufficient confidence for a read-only dashboard at current scale. Revisit if interactive UI features are added.
- **Performance benchmarks:** Not needed at v1 scale. Revisit when data volumes warrant it.
- **Exhaustive negative testing of external APIs:** We trust external API contracts and test our error handling, not their failure modes.

## Development Tools

**`/mikey:tdd`** — Test-driven development agent. Takes a scenario file and implements it interactively, generating tests and code that follow Functional Core / Imperative Shell principles. Use when implementing a new milestone scenario.

**`/mikey:testify`** — Test quality auditor. Reviews code for design issues (mixed I/O and logic), excessive mocking, implementation detail testing, and missing error-path coverage. Use after implementing scenarios to validate alignment with this strategy.

**`/claude-swe-workflows:test-mutation`** — Mutation testing workflow. Finds surviving mutations across all source modules and writes targeted tests to kill them. Multi-session with progress tracking. From the [`claude-swe-workflows`](https://github.com/chrisallenlane/claude-swe-workflows) plugin. Use after implementing a milestone.

---

*References: [architecture.md](architecture.md), [contracts.md](contracts.md), scenario files in `docs/scenarios/`, Claude Code skills: [`/mikey:tdd`, `/mikey:testify`](https://github.com/mycargus/mikey-claude-plugins), [`/test-mutation`](https://github.com/chrisallenlane/claude-swe-workflows).*
