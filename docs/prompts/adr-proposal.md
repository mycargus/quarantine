# Propose ADR: <decision title>

Use this prompt when proposing a new Architecture Decision Record.

## Instructions

1. In the prompt content below, replace `<decision title>` with a short imperative description of the decision (e.g., "Use X for Y").
2. Copy + Paste the prompt into a fresh Claude Code session.

## Prompt

```
Propose ADR: <decision title>

Before drafting, read `docs/adr/` to find the next available ADR number and understand existing decisions. Then use `/review-adr "<decision title>"` to check whether the proposed decision conflicts with any existing ADR. If a conflict exists, you MUST stop and surface it before proceeding.

Then draft the ADR following the structure of existing ADRs in `docs/adr/`:
- Title: Short, imperative (e.g., "Use X for Y")
- Status: Proposed
- Context: Why is this decision needed? What problem does it solve?
- Decision: What is the decision?
- Consequences: What are the trade-offs? What does this enable or constrain?

Constraints:
- You MUST NOT implement anything based on a proposed ADR until it is accepted.
- If the decision affects v1 scope boundaries, you MUST flag it explicitly — scope changes require discussion.
```
