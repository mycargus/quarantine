---
name: milestone
description: Set the current milestone context to scope work and enable guardrails
argument-hint: "[milestone-number]"
disable-model-invocation: true
allowed-tools: Read, Edit
---

Set the active milestone to M$1.

## Steps

1. Read `docs/pre-implementation-tasks.md` to get the full milestone definition for M$1, including its scope, acceptance criteria, and phase.

2. Read `CLAUDE.md` to check if there is already a "Current Milestone" section.

3. Update `CLAUDE.md` to add or replace a "Current Milestone" section right after the "## v1 Scope" section with:

```
## Current Milestone

**M$1** — [milestone title from pre-implementation-tasks.md]

Scope:
[bullet points from the milestone definition]

Acceptance criteria:
[acceptance criteria from the milestone definition]

Files in scope for this milestone:
[list the specific directories/files this milestone touches based on the milestone definition]

Do not modify files outside this milestone's scope without explicit approval.
```

Use these mappings to determine files in scope:
- M1 (CLI core): `cli/cmd/`, `cli/internal/parser/`, `cli/internal/config/`, `cli/internal/runner/`, `testdata/`
- M2 (Flaky detection): `cli/internal/parser/`, `cli/internal/runner/`, `cli/internal/quarantine/`
- M3 (GitHub state): `cli/internal/github/`, `cli/internal/quarantine/`, `schemas/quarantine-state.schema.json`
- M4 (Issues + PR comments): `cli/internal/github/`
- M5 (Artifacts): `cli/internal/github/`, `schemas/test-result.schema.json`
- M6 (Dashboard ingestion): `dashboard/`
- M7 (Dashboard UI): `dashboard/`
- M8 (Polish): all files

4. Confirm the milestone was set and summarize what's in scope.
