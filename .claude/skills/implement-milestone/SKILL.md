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

Rules:
- ONE scenario per `/mikey:tdd` invocation. Do NOT batch.
- Start with integration or e2e tests — they catch real issues faster than unit tests and drive better design.
- Add unit tests for pure functions extracted during the Refactor step.

### Step 2 — Validate

Invoke `/mikey:testify <test-file-path> --with-design`.

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

Read `docs/specs/test-strategy.md` to understand the E2E test layer: E2E tests exercise the compiled binary against real external dependencies (real GitHub API, real git repo). They catch issues that mocks cannot — API behavior changes, response format drift, auth edge cases.

After each scenario, evaluate whether it warrants an E2E test:
- Does the scenario exercise a new external integration (GitHub API endpoint, git operation, file I/O)?
- Does it cover a flow where mock fidelity is a concern (e.g., API response shapes, auth, rate limits)?
- Is there already an E2E test covering this flow?

If yes to the first two and no to the third, add or extend an E2E test in `e2e/`. Run `make e2e-test` to verify. Do NOT wait until all scenarios are done — run E2E tests incrementally as they become relevant.

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
