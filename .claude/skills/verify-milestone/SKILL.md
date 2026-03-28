---
name: verify-milestone
description: Verify a milestone's implementation against its manifest — build, acceptance criteria, flow invariants, and scenario coverage
argument-hint: "[milestone-number]"
disable-model-invocation: false
allowed-tools: Read, Grep, Glob, Bash, Edit
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

### 7. Check E2E coverage

Determine whether the milestone's scenarios require E2E tests beyond what already exists. E2E tests verify that real external APIs behave as integration test mocks assume. They catch mock-fidelity drift: response shape changes, header behavior, redirect chains, pagination, auth edge cases, and eventual consistency.

#### 7a. Inventory external API interactions

Read the milestone's acceptance scenarios and the **production source code**. List every external API call the production code path exercises — not just what tests call. If a function accepts an injected dependency (e.g., `fetchFn`) for testing but uses real HTTP in production, that IS an external API interaction. Dependency injection makes unit testing possible; it does not eliminate mock-fidelity risk.

For each interaction, note the HTTP method, endpoint, and provider (e.g., `GitHub: GET /repos/.../actions/artifacts`, `Jenkins: GET /job/.../api/json`).

Classify each interaction as **high-risk** or **low-risk** for mock-vs-real divergence:

**High-risk** (always need E2E):
- Response shapes the code destructures (e.g., `data.artifacts`, `response.headers.get("etag")`)
- Conditional request round-trips (ETag/If-None-Match, Last-Modified/If-Modified-Since)
- Redirect chains (302 → blob storage download)
- Search/query API formats (query string must match what the provider indexes)
- Pagination (code assumes single page with `per_page=100` or similar)
- Sequential state (second call depends on state from first call)
- Auth header formats (Bearer vs Basic vs token header — varies by provider)

**Low-risk** (skip — mock fidelity is sufficient):
- Status code checks (`if (response.status === 401)`) — no shape to drift
- Pure client-side logic (e.g., parsing `GITHUB_EVENT_PATH`) — no external API involved

#### 7b. Check existing E2E tests

Read the E2E test files in `e2e/`. List which high-risk API interactions they already exercise.

E2E tests in `e2e/` cover ANY component (CLI, dashboard, shared libraries) and ANY provider (GitHub, Jenkins, GitLab). "No E2E infrastructure exists for this component" and "this provider isn't set up yet" are NOT valid reasons to mark E2E as PASS.

#### 7c. Identify gaps

For each high-risk interaction from 7a that is NOT covered by an existing E2E test in 7b, report it as a gap. Group related gaps (e.g., "Scenarios 27 and 49 both need sequential-state E2E tests"). For each gap, note which provider is involved.

Mark as PASS if all high-risk interactions are covered, or FAIL with the list of uncovered interactions.

### 8. Generate report

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

### E2E Coverage
- [x] All high-risk API interactions covered by e2e/ tests
- [ ] Missing E2E coverage — <list of uncovered high-risk interactions>

### Summary
X/Y checks passed. Z issues found.
```

Rules for the report:
- Use `[x]` for PASS, `[ ]` for FAIL
- Every FAIL line MUST include a reason after an em dash
- The Summary line MUST count ALL checks across all sections (build + criteria + invariants + scenario files + e2e coverage)
- If all checks pass, end with: "Milestone M$1 is verified."
- If any checks fail, end with: "Milestone M$1 has Z unresolved issues."

### 9. Mark milestone as verified

If ALL checks passed (no failures in any section), update the manifest's frontmatter status from `planned` to `verified`:

Use the Edit tool to change:
```
status: planned
```
to:
```
status: verified
```

in `docs/milestones/m$1.md`.

If any checks failed, do NOT update the status. The manifest stays `planned` until all issues are resolved and verification passes.
