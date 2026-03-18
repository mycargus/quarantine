---
name: sync-docs
description: Scan for inconsistencies between code and documentation
argument-hint: "[scope]"
disable-model-invocation: false
allowed-tools: Read, Grep, Glob
---

Scan for inconsistencies between the codebase and documentation.

Scope: $1 (default: all) — valid values: 'all', 'architecture', 'requirements', 'scenarios', 'adrs'

## Steps

1. Read the current state of the codebase:
   - Check what files/directories exist under `cli/` and `dashboard/`
   - Check `schemas/` for JSON schema files
   - Check `CLAUDE.md` for the current milestone
   - Check `quarantine.yml` example or schema if it exists

2. Based on the scope argument, read the relevant docs:
   - `all` or `architecture`: Read `docs/planning/architecture.md`
   - `all` or `requirements`: Read `docs/planning/functional-requirements.md`
   - `all` or `scenarios`: Read `docs/scenarios/index.md` and relevant section files in `docs/scenarios/v1/`
   - `all` or `adrs`: Read all files in `docs/adr/`

3. Check for these categories of inconsistency:

**Stale references:**
- File paths mentioned in docs that don't exist in the codebase
- Commands or CLI flags documented but not implemented (or implemented differently)
- Config fields documented but not in the actual schema/parser
- API endpoints documented but not in the code

**Drift:**
- Data model in code differs from architecture.md section 5 schemas
- Behavior in code differs from user scenario expectations
- Error handling in code differs from docs/error-handling.md (if it exists)

**Missing documentation:**
- Code that implements undocumented behavior
- New files/modules not referenced in architecture.md
- New config fields not in the schema documentation

**Version label issues:**
- v2+ features that have been implemented (docs should be updated)
- v1 features documented but not yet implemented (expected if in-progress)

4. Report findings grouped by severity:

```
## Documentation Sync Report

### Stale (docs reference things that don't match code)
- [file:line] — [what's wrong]

### Drift (code behavior differs from docs)
- [file:line] — [what's wrong]

### Undocumented (code exists without docs)
- [file] — [what needs documenting]

### Summary
- X stale references
- X behavioral drifts
- X undocumented items
```

5. Ask the user if they want to fix any of the issues found. Do NOT auto-fix — inconsistencies may be intentional (e.g., work in progress).
