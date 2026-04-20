---
name: plan
description: Create a structured implementation plan from an ADR, companion spec, or freeform description. Produces a reviewable plan file in docs/plans/ with defined sections that /create-milestone can consume. Use when the user says "plan this", "plan ADR-036", or "make a plan for X".
argument-hint: "<adr-number | description> [--output <path>]"
model: opus
effort: max
disable-model-invocation: false
user-invocable: true
allowed-tools: Read, Grep, Glob, Write, Edit, Bash, Agent, Skill, AskUserQuestion
---

Create a structured implementation plan for: "$1"

## Constraints

1. **Do not implement.** This skill produces a plan file, not code.
2. **Do not invent behavior.** Every requirement in the plan MUST trace to an
   ADR, companion spec, or existing spec document. If behavior is ambiguous,
   ask — do not guess.
3. **The plan is a reviewable artifact.** It must be clear enough that a
   different agent (or human) can implement from it without access to this
   conversation.

## Step 1: Gather source material

Determine the input type from `$1`:

- **ADR number** (e.g., `036`, `ADR-036`): Read `docs/adr/<number>*.md`. Check
  for a companion spec by scanning the ADR for links to `docs/specs/` files.
  Also check `docs/specs/` for files whose names suggest a relationship (e.g.,
  `v2-auth-token-proxy.md` for ADR-036).
- **Freeform description**: Search existing ADRs and specs for relevant material.
  Use AskUserQuestion if the description doesn't clearly map to existing docs.

Also read:
- `docs/milestones/index.md` — to understand the current milestone landscape,
  numbering, and dependency graph
- `docs/specs/functional-requirements.md` — for FR ID matching
- `docs/specs/non-functional-requirements.md` — for NFR ID matching

## Step 2: Extract requirements

From all source material, build a structured requirements list. For each
requirement, capture:

- **ID**: Use the source's own ID if it has one (SEC-1, UX-3, FR-1.4.1). If
  the source has no IDs, assign temporary ones (R-1, R-2, ...).
- **Summary**: One-line description.
- **Source**: File path + section/line reference.
- **Testability**: How would you verify this is implemented correctly? If a
  requirement is not testable, flag it.
- **FR/NFR mapping**: Which FR-X.Y.Z or NFR-X.Y.Z IDs does this satisfy?
  If none exist yet, note "NEW — needs FR/NFR entry."

Present the requirements list to the user. Ask:
- Are any requirements missing?
- Should any be deferred or excluded?
- Are the priorities right?

Wait for confirmation before proceeding.

## Step 3: Group into milestones

Propose how to group the requirements into milestones. Consider:

- **Dependency order**: What must be built before what?
- **Vertical slices**: Each milestone should produce a testable, demonstrable
  increment. Avoid milestones that are purely internal scaffolding.
- **Size**: Target 5-15 requirements per milestone. Split if larger.
- **Existing milestones**: Check `docs/milestones/index.md` for the next
  available milestone number and any dependencies.

For each proposed milestone, draft:

- **Title**
- **Dependencies** on prior milestones
- **Requirements included** (by ID)
- **Requirements excluded** (with reason: deferred, out of scope, etc.)
- **Acceptance criteria** (derived from the requirements — testable, specific,
  with FR/NFR IDs)

Present the grouping to the user. This is the most important review gate —
grouping decisions propagate into everything downstream. Wait for confirmation.

## Step 4: Outline scenarios

For each milestone, outline the Given/When/Then scenarios needed. These are
outlines, not full scenarios — `/create-user-scenario` will flesh them out.

For each requirement in the milestone, propose:
- **Happy path scenario(s)**
- **Error/negative scenario(s)**
- **Edge cases** (if the requirement has boundary conditions)

Map each scenario outline to its source requirement ID.

Present scenario outlines. Ask if any are missing or unnecessary.

## Step 5: Identify spec references

For each milestone, identify which existing spec documents are relevant:

| Spec | Relevant sections |
|------|------------------|
| `cli-spec.md` | Specific `#anchors` |
| `error-handling.md` | Specific `#anchors` |
| etc. | |

Also identify spec updates needed (e.g., "error-handling.md needs a new
Category 4 for dashboard proxy errors" — from the companion spec).

## Step 6: Write the draft plan file

Determine the output path:
- If `--output <path>` was provided, use it.
- Otherwise: `docs/plans/<slug>.md` where `<slug>` is derived from the ADR
  number or description (e.g., `adr-036-token-proxy.md`).

Write the plan file using this exact structure:

```markdown
# Plan: <Title>

> **Status:** draft
> **Created:** <YYYY-MM-DD>
> **Source:** <ADR link and/or description>

## Context

<1-3 paragraphs: what problem this solves and why, derived from the ADR>

## Requirements

| ID | Summary | Source | FR/NFR | Priority |
|----|---------|--------|--------|----------|
| <id> | <one-line> | <file#section> | <FR/NFR IDs> | must/should |

## Milestones

### M<N>: <Title>

**Dependencies:** <prior milestones or "none">

**Scope — included:**
- <requirement IDs and descriptions>

**Scope — excluded:**
- <what's NOT in this milestone, with reason>

**Acceptance criteria:**
1. <testable criterion> (FR-X.Y.Z, NFR-X.Y.Z)
2. ...

**Scenario outlines:**
- <brief Given/When/Then outline> → <requirement ID>
- ...

**Spec references:**
| What | Reference |
|------|-----------|
| <topic> | <file.md#anchor> |

<Repeat ### M<N+1> for additional milestones>

## Spec updates required

- <file.md>: <what needs to change and why>

## Risks and open questions

- <risk or question, with proposed mitigation or owner>

## Review summary

| Reviewer | Verdict | Key findings |
|----------|---------|-------------|
| Acceptance test | approve/needs-revision | <summary> |
| Architecture | approve/needs-revision | <summary> |
| UX | approve/needs-revision | <summary> |
| Security | approve/needs-revision | <summary> |
```

## Step 7: Run review

Invoke `/review <plan-file-path>` to run four review agents (acceptance test,
architecture, UX, security) against the draft plan file. The `/review` skill
is the single source of truth for reviewer definitions.

If any reviewer returns `needs-revision` with blockers:
1. Address all blocker issues in the plan file.
2. Re-invoke `/review <plan-file-path>` (max 2 iterations).
3. If still failing after 2 iterations, present the unresolved issues to the
   user and ask for a decision.

After reviewers pass, update the plan's `## Review summary` section with the
final verdicts and key findings.

## Step 8: Present for approval

Print the plan file path and a summary:
- Number of requirements extracted
- Number of milestones proposed
- Number of scenario outlines
- Review verdicts
- Any unresolved risks or open questions

Ask the user to review the plan file and approve, request changes, or defer.

When approved, update the status line in the plan file from `draft` to
`approved`.

## Integration with /create-milestone

After this skill completes, the user can invoke:
```
/create-milestone <N> --plan docs/plans/<slug>.md
```

`/create-milestone` reads the plan's milestone section for scope, acceptance
criteria, and scenario outlines instead of relying solely on `index.md`. The
user still needs to:
1. Add the milestone definition to `docs/milestones/index.md` (can copy from
   the plan file's milestone section)
2. Create full scenarios via `/create-user-scenario` (using the plan's
   scenario outlines as input)
3. Run `/create-milestone <N>` to build the manifest
