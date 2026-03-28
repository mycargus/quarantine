---
name: implement-milestone
description: Implement a predefined milestone using TDD, testify validation, and atomic commits. Use when the user asks to implement, continue, or work on a milestone.
disable-model-invocation: true
user-invocable: true
argument-hint: "<milestone-number> [--from <scenario-number>]"
allowed-tools: Read, Grep, Glob, Bash, Edit, Write, Agent, Skill
---

Implement milestone $1.

## Phase 1: Orientation

### Read the manifest

Read `docs/milestones/m$1.md`. Extract:
- Acceptance scenarios (file references and scenario numbers)
- Acceptance criteria (numbered list with FR/NFR IDs)
- Verification section (flow invariants, build commands)
- Scope section (MUST / MUST NOT items)

If the manifest does not exist, stop and report: "No manifest found at `docs/milestones/m$1.md`."

### Check for gaps

Answer these questions to the user before writing any code:
- Do the acceptance scenarios cover all acceptance criteria? If no, flag gaps.
- Are there ambiguities or contradictions? If so, stop and ask.

Wait for user confirmation before proceeding.

### Detect already-committed work

Run `git log --oneline --grep="milestone $1:" | head -20` to find previously committed scenarios.

Cross-reference with the manifest's scenario list. Report which scenarios are done and which remain:

```
Already committed:
  - Scenario 20: <commit subject>
  - Scenario 24: <commit subject>

Remaining:
  - Scenario 72
  - Scenario 73
```

If a `--from` argument was provided, skip to that scenario number. Otherwise, start with the first remaining scenario.

## Phase 2: Per-Scenario Loop

Work through remaining scenarios ONE AT A TIME. For each scenario, execute these steps in strict order. Do NOT reorder or skip steps.

### Step 1 — TDD

Invoke `/mikey:tdd --validate <scenario-file>#<scenario-number>`.

**CRITICAL:** When the `/mikey:tdd` skill expands, it will instruct you to spawn a `tdd-agent` using the Agent tool (`subagent_type: "mikey:tdd-agent"`). You MUST do this. Do NOT write tests, write implementation code, or run test commands yourself. All Red-Green-Refactor work MUST be delegated to the `tdd-agent`. If you find yourself reading source files, editing test files, or running `go test` directly, STOP — you are bypassing the agent.

Rules:
- ONE scenario per `/mikey:tdd` invocation. Do NOT batch.
- Start with integration or e2e tests — they catch real issues faster than unit tests and drive better design.
- Add unit tests for pure functions extracted during the Refactor step.

### Step 2 — Validate

Invoke `/mikey:testify <test-file-path> --with-design`.

**Note:** `/mikey:tdd --validate` causes the tdd-agent to run testify internally, but that is its own internal check. You MUST still invoke `/mikey:testify` here as an independent gate — the tdd-agent's internal run may have missed issues or used a narrower scope.

GATE: You MUST fix ALL issues (HIGH, MEDIUM, and LOW) before proceeding to step 3. If testify reports issues, fix them and re-run testify until the report is clean. Do NOT commit with open issues.

### Step 3 — Commit

GATE: Do NOT commit until step 2 reports zero open issues.

1. Run the build gate. Read the manifest's Verification section for the exact build commands. If none are specified, infer from the milestone's scope:
   - CLI milestones: `make cli-build && make cli-test && make cli-lint`
   - Dashboard milestones: `make dash-build && make dash-test && make dash-lint`
2. If any command fails, fix the issue and re-run. Do NOT commit a failing build.
3. Stage and commit with message: `milestone $1: <description of what changed>`
4. Each commit is a safe rollback point. Never accumulate uncommitted work across scenarios.

### Step 4 — E2E (conditional)

E2E tests verify that real external APIs behave as integration test mocks assume. They catch mock-fidelity drift: response shape changes, header behavior, redirect chains, pagination, auth edge cases, and eventual consistency.

After completing each scenario's commit, determine whether it needs E2E coverage.

#### 4a. Inventory the scenario's external API interactions

List every external API call the **production code path** exercises — not just what the test calls. If a function accepts an injected dependency (e.g., `fetchFn`) for testing but uses real HTTP in production, the production path IS an external API interaction and MUST be evaluated for E2E coverage. Dependency injection makes unit testing possible; it does not eliminate mock-fidelity risk.

For each call, note:
- The HTTP method and endpoint pattern (e.g., `GET /search/issues?q=...`)
- What the integration test mock returns (canned response shape, headers, status codes)
- Whether the real API could diverge from this mock in ways that matter

**High mock-fidelity-risk** (always need E2E):
- Response shapes the code destructures (e.g., `data.artifacts`, `response.headers.get("etag")`)
- Conditional request round-trips (ETag/If-None-Match, Last-Modified/If-Modified-Since)
- Redirect chains (302 → blob storage download)
- Search/query API formats (query string must match what the provider indexes)
- Pagination (code assumes single page with `per_page=100` or similar)
- Sequential state (second call depends on state from first call)
- Auth header formats (Bearer vs Basic vs token header — varies by provider)

**Low mock-fidelity-risk** (skip E2E):
- Status code checks (`if (response.status === 401)`) — no shape to drift
- Pure client-side logic (e.g., PR number detection from `GITHUB_EVENT_PATH`) — no external API involved

#### 4b. Check existing E2E coverage

Read the existing E2E test files in `e2e/` and list which API interactions they already exercise. A scenario does NOT need a new E2E test if every high-risk interaction is already covered by an existing test.

E2E tests in `e2e/` cover ANY component (CLI, dashboard, shared libraries) and ANY provider (GitHub, Jenkins, GitLab). "No E2E infrastructure exists for this component" and "this provider isn't set up yet" are NOT valid reasons to skip E2E.

#### 4c. Decision

Add or extend an E2E test if the scenario introduces a **new high-risk API interaction** not covered by existing E2E tests. When multiple scenarios share the same new interaction, group them into a single E2E test.

If adding an E2E test, invoke `/create-e2e-test <description>` to follow the project's E2E conventions (provider-specific helpers, credential guards, assertion style, cleanup).

Run `make e2e-test` to verify. Do NOT wait until all scenarios are done — add E2E tests incrementally as high-risk interactions are introduced.

### Loop

Return to Step 1 for the next remaining scenario. Continue until all scenarios are implemented and committed.

## Phase 3: Verification

When all scenarios are done:

1. Invoke `/verify-milestone $1`. Fix any failures before proceeding.
2. Report to the user:
   - What was implemented (scenario list with commit hashes)
   - What was verified (verification report summary)
   - Any deviations from the manifest

## Constraints

- You MUST adhere to all functional and non-functional requirements.
- One concern per change. You MUST NOT allow scope drift.
- You MUST NOT modify existing files in `docs/scenarios/` without confirmation from the user.
