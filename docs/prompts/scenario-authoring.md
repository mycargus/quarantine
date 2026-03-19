# Author scenarios for <feature or edge case>

Use this prompt when writing new Given/When/Then scenarios.

## Instructions

1. In the prompt content below, replace `<feature or edge case>` with a short description of what you're adding scenarios for.
2. Copy + Paste the prompt into a fresh Claude Code session.

## Prompt

```
Author scenarios for <feature or edge case>

Before writing any scenarios, read the relevant existing scenario file(s) in `docs/scenarios/` to match style and numbering, and read the relevant spec in `docs/specs/` to ensure scenarios reflect the intended behavior. Confirm the scenario number does not collide with an existing one.

Then write scenarios that:
- Use a `### Scenario N: <title> [MX]` heading so every scenario is individually linkable from other docs.
- Include a `**Risk:**` line immediately after the heading that states the failure mode or incorrect behavior this scenario prevents once tests are written for it.
- Follow the exact Given/When/Then structure used in the file.
- Are self-contained — each scenario fully describes its preconditions.
- Are specific — use exact output strings, exit codes, and field values where the spec defines them.
- Cover one behavior each. Do not combine multiple behaviors in one scenario.

Constraints:
- You MUST NOT modify existing scenarios without confirmation from me.
- New scenarios go at the end of the appropriate file, or in a new file if the topic is distinct.
- If the spec is ambiguous, you MUST flag it before writing — do not invent behavior.
```
