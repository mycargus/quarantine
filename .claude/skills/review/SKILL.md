---
name: review
description: Run four review agents (acceptance test, architecture, UX, security) against a plan file. Returns structured verdicts with blockers and observations. Use when a plan needs review or re-review, or when the user says "review the plan", "run reviewers", or "critique this plan".
argument-hint: "<path-to-plan-file>"
model: opus
effort: max
disable-model-invocation: false
user-invocable: true
allowed-tools: Read, Grep, Glob, Agent, AskUserQuestion
---

Review the plan file: "$1"

## Constraints

1. **Read-only.** This skill reviews — it does not modify the plan or any
   other file.
2. **All four agents run in parallel.** Do not run them sequentially.
3. **Verdicts are structured.** Each reviewer returns approve or needs-revision
   with categorized issues.

## Step 1: Locate the plan

If `$1` is a file path, read it directly. If `$1` is a plan name or
description, search `docs/plans/` for a matching file.

If no argument is provided, use AskUserQuestion to ask which plan to review.

Read the plan file. Extract:
- The source references (ADR, companion spec, etc.)
- The milestones, acceptance criteria, and scenario outlines
- Any previous review summary (to detect re-review context)

## Step 2: Load source material

Read all documents referenced by the plan's "Source" and "Spec references"
sections. The reviewers need this context to evaluate the plan against the
specs.

## Step 3: Detect re-review context

If the plan file contains a `## Review summary` section with previous
verdicts, this is a re-review. Include the previous findings in each
reviewer's prompt so they can verify fixes.

## Step 4: Dispatch reviewers

Launch four review agents in parallel using the Agent tool. Pass each
reviewer the full plan content AND the relevant source material content.

Each reviewer prompt MUST include:
- The complete plan file content
- Relevant spec/ADR content
- Instructions to read additional files if needed (Glob, Grep, Read available)
- If re-review: the previous findings to verify

### Reviewer definitions

**Acceptance test critic:**
- Do acceptance criteria have clear pass/fail conditions?
- Are scenarios traceable to requirements? Any requirement without a scenario?
- Are error paths covered?
- Are edge cases covered? (boundary values, empty states, concurrent scenarios)
- Is there any acceptance criterion that is vague or untestable?
- Doc-only requirements should be explicitly marked as "verified by doc review"

**Architecture critic:**
- Does the milestone grouping respect dependencies?
- Are scope boundaries clean? Any cross-cutting concerns that span milestones?
- Do milestones align with existing ADRs?
- Are source documents (ADR, specs) internally consistent?
- Are there any architectural concerns from the requirements?

**UX critic:**
- Will the implementation produce good error messages and user experience?
- Are onboarding scenarios covered?
- Is the user's perspective represented in acceptance criteria?
- Are error messages actionable — can a developer fix the problem from the
  message alone?
- Are there confusing interactions between configuration options?

**Security critic:**
- Are there token/credential handling risks?
- Input validation gaps?
- Privilege escalation paths?
- Are security requirements (SEC-*) covered by acceptance criteria and scenarios?
- Is the attack surface documented?
- Are rate limiting and caching schemes sound?
- Any OWASP Top 10 concerns?

Each reviewer MUST return:
- **Verdict:** `approve` or `needs-revision`
- **Issues** (if needs-revision): Each categorized as `blocker` or `observation`
- Concise bullet points, not paragraphs

## Step 5: Compile results

After all four agents return, compile the results into a summary table:

```
| Reviewer | Verdict | Blockers | Observations |
|----------|---------|----------|-------------|
| Acceptance test | ... | N | N |
| Architecture | ... | N | N |
| UX | ... | N | N |
| Security | ... | N | N |
```

If any reviewer returned `needs-revision`:
- List all blocker issues with their reviewer attribution
- List all observations separately
- State clearly: "N blockers must be addressed before the plan can be approved."

If all reviewers returned `approve`:
- List any observations for consideration
- State: "All reviewers approve. The plan is ready for implementation."

## Step 6: Report

Print the summary table and all issues to the user. Do NOT modify the plan
file — that is the user's or `/plan`'s responsibility.

If this is a re-review, highlight which previous blockers are now resolved
and which remain.
