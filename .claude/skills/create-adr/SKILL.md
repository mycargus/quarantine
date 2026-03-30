---
name: create-adr
description: Propose a new Architecture Decision Record. Reads existing ADRs for numbering and conflict detection, drafts the ADR following project conventions, and checks for scope boundary impacts. Use when a design decision needs to be documented.
argument-hint: "[decision title]"
disable-model-invocation: false
user-invocable: true
allowed-tools: Read, Grep, Glob, Write, Edit, AskUserQuestion
---

Propose a new ADR for: "$1"

## Step 0 — Gather context

If no argument was provided (empty `$1`), use AskUserQuestion to ask:
- "What decision needs to be documented? (Use imperative form, e.g., 'Use X for Y')"

## Step 1 — Read existing ADRs

1. Glob `docs/adr/*.md` and determine the next available ADR number.
2. Read 3-4 existing ADRs to understand the structure, tone, and level of detail used in this project.
3. Note all existing decisions — you will need these for conflict detection.

## Step 2 — Check for conflicts

Use `/review-adr` to check whether the proposed decision conflicts with any existing ADR. If a conflict exists, stop and surface it to the user before proceeding. Present:
- Which ADR conflicts
- What the existing ADR decided
- How the proposed decision conflicts
- Options: abandon, supersede the old ADR, or adjust the proposal

## Step 3 — Clarify the decision

Use AskUserQuestion to confirm understanding. Ask about whichever of these are unclear:
- What problem does this decision solve? (Context)
- What alternatives were considered? (At least 2)
- What are the key trade-offs?

If the decision affects v1 scope boundaries (see CLAUDE.md Boundaries section), flag this explicitly — scope changes require discussion.

## Step 4 — Draft the ADR

Write the ADR to `docs/adr/{NNN}-{slug}.md` following the structure of existing ADRs:

```markdown
# ADR-{NNN}: {Title}

**Status:** Proposed
**Date:** {today's date, YYYY-MM-DD}

## Context

{Why is this decision needed? What problem does it solve? Include enough
background for a reader unfamiliar with the discussion.}

## Decision

{What is the decision? Use clear, direct language. Bold the key statement.
Include specific technical details — file paths, API names, library choices.}

## Alternatives Considered

- **{Alternative 1}.** {Why rejected — be specific about the trade-off.}
- **{Alternative 2}.** {Why rejected.}

## Consequences

**Positive:**

- (+) {Benefit}
- (+) {Benefit}

**Negative:**

- (-) {Trade-off or cost}
- (-) {Trade-off or cost}
```

### Rules

- Status MUST be `Proposed`. Only the user can accept an ADR.
- Title MUST be short and imperative (e.g., "Use X for Y", "Require Z").
- Context MUST explain the problem, not just the solution.
- Alternatives MUST include at least 2 rejected options with specific reasons.
- Consequences MUST list both positives and negatives. No decision is free.
- MUST NOT implement anything based on this ADR — it is a proposal only.

## Step 5 — Verify

After writing:

1. Confirm the ADR number does not collide with an existing file.
2. Confirm the Status is `Proposed`.
3. Confirm Alternatives Considered has at least 2 entries.
4. Confirm Consequences has both Positive and Negative sections.

## Step 6 — Report

Print a summary:
- ADR number and title
- File path
- Status
- Number of alternatives considered
- Whether any scope boundary impacts were flagged
- Reminder: "This ADR is Proposed. Accept it before implementing."
