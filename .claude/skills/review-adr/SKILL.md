---
name: review-adr
description: Check if a proposed change contradicts any existing Architecture Decision Record
argument-hint: "[change-description]"
disable-model-invocation: false
allowed-tools: Read, Grep
---

Review whether the proposed change contradicts any existing ADRs: "$1"

## Steps

1. Read ALL ADR files in `docs/adr/` (001 through 018). For each ADR, extract the core decision and key constraints.

2. Analyze whether the proposed change contradicts, weakens, or conflicts with any existing ADR decision. Check specifically for:
   - Direct contradictions (the change does something an ADR explicitly decided against)
   - Scope violations (the change adds something deferred to v2+ per an ADR)
   - Architectural violations (the change breaks a design principle from an ADR)
   - Implicit conflicts (the change is technically compatible but undermines the reasoning behind an ADR)

3. Also read `CLAUDE.md` and check the "What NOT to Do" section for conflicts.

4. Report findings in this format:

**If no conflicts found:**
```
No ADR conflicts detected. The proposed change is compatible with all 18 existing decisions.
```

**If conflicts found:**
```
## ADR Conflicts Detected

### Conflict with ADR-NNN: [title]
- **ADR says:** [what the ADR decided]
- **Proposed change:** [how it conflicts]
- **Severity:** [Direct contradiction / Scope violation / Implicit conflict]
- **Options:**
  1. Abandon the proposed change
  2. Update ADR-NNN with new decision and rationale
  3. [Any other resolution]
```

5. If conflicts are found, ask the user how they want to proceed before making any changes. If they want to update an ADR, write the updated ADR with the new decision, marking the old decision as superseded and documenting why.
