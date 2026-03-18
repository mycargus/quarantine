---
name: milestone
description: Set the current milestone context to scope work and enable guardrails
argument-hint: "[milestone-number]"
disable-model-invocation: true
allowed-tools: Read, Edit
---

Set the active milestone to M$1.

## Steps

1. Read `docs/planning/milestones.md` to get the full milestone definition for M$1, including its scope, acceptance criteria, and phase.

2. Read `CLAUDE.md` to check if there is already a "Current Milestone" section.

3. Update `CLAUDE.md` to add or replace a "Current Milestone" section right after the "## v1 Scope" section with:

```
## Current Milestone

**M$1** — [milestone title from milestones.md]

Scope:
[bullet points from the milestone definition]

Acceptance criteria:
[acceptance criteria from the milestone definition]

Files in scope for this milestone:
[list the specific directories/files this milestone touches based on the milestone definition]

Do not modify files outside this milestone's scope without explicit approval.
```

Use these mappings to determine files in scope:
- M1 (CLI scaffolding + init): `cli/cmd/`, `cli/internal/config/`, `cli/internal/github/`, `cli/internal/git/`
- M2 (Test execution + XML parsing): `cli/cmd/`, `cli/internal/parser/`, `cli/internal/runner/`, `testdata/`
- M3 (Flaky detection + retry): `cli/internal/runner/`, `cli/internal/parser/`
- M4 (Quarantine state + exclusion): `cli/internal/github/`, `cli/internal/quarantine/`
- M5 (Issues + PR comments + artifacts): `cli/internal/github/`
- M6 (Dashboard ingestion): `dashboard/`
- M7 (Dashboard UI): `dashboard/`
- M8 (Polish): all files

4. Confirm the milestone was set and summarize what's in scope.
