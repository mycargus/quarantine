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

E2E tests exercise the compiled binary against real external dependencies (real GitHub API, real git repo). They catch issues that mocks cannot: API query format correctness, response shape drift, pagination behavior, auth edge cases, and eventual consistency.

After completing each scenario's commit, determine whether it needs E2E coverage.

#### 4a. Inventory the scenario's external API interactions

List every external API call the scenario exercises. For each call, note:
- The HTTP method and endpoint pattern (e.g., `GET /search/issues?q=...`)
- What the integration test mocks for this call (canned response)
- Whether the real API could behave differently from the mock in ways that matter

**Examples of high mock-fidelity-risk interactions:**
- Search API queries — the query string format (`label:X is:open repo:O/R`) must match what GitHub actually indexes. A mock always returns what you tell it; the real API may not find the issue if the query is wrong.
- Pagination — `per_page=100` and link-header pagination. Mocks typically return one page; real API may paginate differently.
- PATCH to update a comment — depends on finding the correct `comment_id` from a prior GET. Mocks hardcode IDs; real API returns dynamic IDs.
- Sequential state — second run depends on state created by first run (e.g., issue exists, comment exists). Mocks simulate this; real API has propagation delays.

**Examples of low mock-fidelity-risk interactions (skip E2E):**
- Simple POST with a known request body (e.g., create issue) — already covered by existing E2E happy path.
- Error handling for status codes (e.g., 410 Gone) — the code checks `resp.StatusCode`; there is no mock-vs-real divergence risk.
- Pure client-side logic (e.g., PR number detection from `GITHUB_EVENT_PATH`) — no external API involved.

#### 4b. Check existing E2E coverage

Read the existing E2E test files in `e2e/` and list which API interactions they already exercise. A scenario does NOT need a new E2E test if every high-risk interaction is already covered by an existing test.

#### 4c. Decision

Add or extend an E2E test if the scenario introduces a **new high-risk API interaction** not covered by existing E2E tests. When multiple scenarios share the same new interaction, group them into a single E2E test.

If adding an E2E test, follow the patterns in the existing `e2e/` tests:
- Use retry loops with delays for GitHub API propagation (search index, CDN)
- Clean up created resources (issues, comments) in `afterEach`
- Use the `riteway` assertion style
- Commit the E2E test in the same commit as the scenario, or as a separate commit if it covers multiple scenarios

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
