---
name: create-user-scenario
description: Author Given/When/Then scenarios for a feature or edge case. Reads existing scenarios and specs to match style, numbering, and intended behavior. Use when adding new acceptance scenarios to docs/scenarios/.
argument-hint: "[feature or edge case description]"
disable-model-invocation: false
user-invocable: true
allowed-tools: Read, Grep, Glob, Write, Edit, AskUserQuestion
---

Author Given/When/Then scenarios for: "$1"

## Step 0 — Gather context

If no argument was provided (empty `$1`), use AskUserQuestion to ask:
- "What feature or edge case should the scenarios cover?"

## Step 1 — Read existing scenarios and specs

1. Read `docs/scenarios/index.md` to understand the scenario index and file organization.
2. Glob `docs/scenarios/v1/*.md` and read the file most relevant to the requested feature to match style, heading format, and numbering conventions.
3. Find the highest scenario number across ALL scenario files (Grep for `### Scenario \d+`) to determine the next available number.
4. Read the relevant spec(s) in `docs/specs/` that define the behavior being covered. Scenarios MUST reflect specified behavior, not invented behavior.

## Step 2 — Clarify scope

Use AskUserQuestion to confirm:
- Which scenario file the new scenarios belong in (existing file or new file)
- Whether the scope is clear enough to write scenarios, or if ambiguities need resolution first

If the spec is ambiguous about the behavior, stop and surface the ambiguity. Do NOT invent behavior.

## Step 3 — Determine milestone tag

Read `docs/milestones/index.md` to find the milestone that covers this feature. Every scenario heading MUST include a milestone tag like `[M3]`.

If the feature doesn't clearly map to a milestone, use AskUserQuestion to ask which milestone tag to use.

## Step 4 — Write scenarios

Write scenarios that follow these rules:

- **Heading format:** `### Scenario N: <title> [MX]` — every scenario is individually linkable.
- **Risk line:** Immediately after the heading, include `**Risk:**` stating the failure mode this scenario prevents once tests are written.
- **Structure:** Follow the exact Given/When/Then format used in existing files.
- **Self-contained:** Each scenario fully describes its preconditions. A reader should not need to look at other scenarios.
- **Specific:** Use exact output strings, exit codes, field values, and error messages where the spec defines them.
- **One behavior each.** Do not combine multiple behaviors in one scenario.
- **Separator:** Use `---` between scenarios.

### Placement

- New scenarios go at the END of the appropriate existing file, or in a new file if the topic is distinct from all existing files.
- MUST NOT modify existing scenarios without explicit confirmation from the user.

## Step 5 — Verify

After writing:

1. Confirm no scenario number collisions (Grep all scenario files for the numbers used).
2. Confirm every scenario has a `**Risk:**` line.
3. Confirm the milestone tag matches a real milestone in `docs/milestones/index.md`.
4. Confirm the Given/When/Then structure matches the style of existing scenarios in the same file.

## Step 6 — Update index

If scenarios were added to an existing file, check whether `docs/scenarios/index.md` needs its scenario count or range updated. If a new file was created, add it to the index.

## Step 7 — Report

Print a summary:
- File written/updated
- Scenario numbers and titles added
- Milestone tag used
- Number of new scenarios
