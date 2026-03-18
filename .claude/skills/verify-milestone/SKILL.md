---
name: verify-milestone
description: Verify a milestone's implementation against its manifest — build, acceptance criteria, flow invariants, and scenario coverage
argument-hint: "[milestone-number]"
disable-model-invocation: false
allowed-tools: Read, Grep, Glob, Bash
---

Verify M$1 implementation against its manifest at `docs/milestones/m$1.md`.

This is a post-implementation audit. It checks whether code and tests satisfy the manifest's contract. It does NOT implement anything or re-run `/mikey:tdd`.

## Steps

### 1. Read the manifest

Read `docs/milestones/m$1.md`. Extract:
- Acceptance criteria (numbered list with FR/NFR IDs)
- Acceptance scenarios (file references and scenario ranges)
- Verification section (build commands, flow invariants, requirement IDs)
- Scope section (MUST implement items — these tell you where to look for code)

If the manifest does not exist, stop and report: "No manifest found at docs/milestones/m$1.md".

### 2. Determine build commands

Use the milestone number to select build commands:
- **M1–M5, M8 (CLI):** `make cli-build && make cli-test && make cli-lint`
- **M6–M7 (Dashboard):** `cd dashboard && pnpm build && pnpm test && pnpm lint`

If the manifest's Verification section specifies different build commands, use those instead.

### 3. Run build/test/lint

Run the build commands from Step 2. Capture output.

If ANY command fails, report the failure and STOP. Do not proceed to audit steps — there is no point auditing code that does not compile or pass tests.

Record test count from test output if available.

### 4. Check acceptance criteria

For each numbered criterion in the manifest's Acceptance Criteria section:

1. **Criteria referencing commands** (e.g., "`quarantine init` creates..."): Use Grep and Glob to verify the command is implemented — find the cobra command registration, the Run function, and confirm it is not a TODO stub.

2. **Criteria referencing test coverage** (e.g., "Unit tests cover: config parsing..."): Use Glob to find test files (`*_test.go` or `*.test.ts`) covering the listed concerns. Read relevant test files to confirm they test the specific things listed.

3. **Criteria with FR/NFR IDs** (e.g., "(FR-1.4.1, FR-1.11.1)"): Read the requirement from `docs/planning/functional-requirements.md` or `docs/planning/non-functional-requirements.md`. Then read the implementation code and verify it addresses the requirement. Check for completeness — partial implementations count as failures.

4. **Criteria referencing build** (e.g., "`make cli-build` produces a binary"): Already verified in Step 3 — mark as passed if build succeeded.

Mark each criterion as PASS or FAIL. For failures, note specifically what is missing or incomplete.

### 5. Check flow invariants

For each MUST statement in the manifest's Verification section under "Flow correctness":

1. Read the referenced sequence diagram or spec section.
2. Read the implementation code that performs the flow.
3. Verify the invariant holds by checking the code path. For example:
   - "MUST call endpoints in this order" — check the function calls are sequenced correctly
   - "MUST exit 2 on failure" — check error handling returns the correct exit code
   - "MUST skip X if Y" — check the conditional logic exists

Mark each invariant as PASS or FAIL. For failures, cite the code location where the violation occurs.

### 6. Check scenario coverage

For each scenario file listed in the manifest's Acceptance Scenarios section:

1. Read the scenario file to get individual scenario numbers and titles.
2. Use Grep and Glob to find test files that correspond to those scenarios. Look for:
   - Test names or comments referencing scenario numbers (e.g., "Scenario 1", "S1")
   - Test names matching scenario titles or descriptions
   - Test files in the same package as the code being tested
3. For each scenario, determine if a corresponding test exists.
4. Count covered vs total scenarios per file.

Mark each scenario file as PASS (all scenarios covered) or PARTIAL (with count and list of missing scenarios).

### 7. Generate report

Print a structured report in this exact format:

```
## M$1 Verification Report

### Build
- [x] <command 1> — passed
- [x] <command 2> — passed (N tests)
- [x] <command 3> — passed

### Acceptance Criteria
- [x] 1. <criterion summary>... (FR-X.Y.Z)
- [ ] 2. <criterion summary>... (FR-X.Y.Z) — MISSING: <what's missing>

### Flow Invariants
- [x] <invariant description>
- [ ] <invariant description> — VIOLATION: <what's wrong at file:line>

### Scenario Coverage
- [x] Scenarios N–M (filename.md) — X/X covered
- [ ] Scenarios N–M (filename.md) — X/Y covered, missing: Scenario Z (title)

### Summary
X/Y checks passed. Z issues found.
```

Rules for the report:
- Use `[x]` for PASS, `[ ]` for FAIL
- Every FAIL line MUST include a reason after an em dash
- The Summary line MUST count ALL checks across all sections (build + criteria + invariants + scenario files)
- If all checks pass, end with: "Milestone M$1 is verified."
- If any checks fail, end with: "Milestone M$1 has Z unresolved issues."
