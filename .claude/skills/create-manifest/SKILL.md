---
name: create-manifest
description: Generate a milestone manifest file — a lightweight routing document that points agents to source docs
argument-hint: "[milestone-number]"
disable-model-invocation: false
allowed-tools: Read, Grep, Glob, Write, Bash
---

Generate a milestone manifest for M$1 at `docs/milestones/m$1.md`.

Milestone manifests are routing tables, NOT content copies. They point agents to existing source docs with specific anchors. The only inline content allowed is scope boundaries (MUST/MUST NOT) and acceptance criteria summaries. Everything else MUST be a link.

## Design Principles

These principles are grounded in published research on AI agent context engineering:

1. **Target 80-100 lines.** If the manifest exceeds 100 lines, you are duplicating content instead of referencing it. Cut ruthlessly. When context files grow long, "critical rules get buried in the middle, exactly where models pay the least attention" — the lost-in-the-middle phenomenon. ([Codified Context Infrastructure, arXiv:2602.20478](https://arxiv.org/html/2602.20478v1))

2. **Route, don't copy.** Manifests are routing tables that point to source docs, not duplicates of them. "An agent can treat each [duplicate] instance as a separate concept. It may update one copy and overlook the others, causing the logic to drift apart." ([Faros AI — AI-generated code and the DRY principle](https://www.faros.ai/blog/ai-generated-code-and-the-dry-principle)). The only inline content allowed is scope boundaries and acceptance criteria.

3. **Use RFC 2119 language.** MUST, MUST NOT, SHALL, SHOULD — no ambiguous phrasing like "try to" or "ideally". Agents fill ambiguity gaps with hallucination. Explicit prohibitions prevent agents from adding "helpful" features outside scope. ([deliberate.codes — Writing specs for AI coding agents](https://deliberate.codes/blog/2026/writing-specs-for-ai-coding-agents/))

4. **Section order is load-bearing.** The order below exploits attention patterns — Workflow and Scope get highest attention at the top, Verification is a checklist used last. "Find the smallest set of high-signal tokens that maximize the likelihood of your desired outcome." ([Anthropic — Effective context engineering for AI agents](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents))

5. **Every reference MUST include a specific anchor**, not just a file path. `[cli-spec.md#quarantine-run](../cli-spec.md#quarantine-run)` not just `cli-spec.md`. This follows the Codified Context paper's "trigger table" pattern — precise routing from work context to the specific knowledge needed, not a pointer to an entire document.

6. **Specifications table says "load on demand".** The agent MUST NOT read all spec docs upfront — only when implementing a specific piece. Anthropic recommends agents "maintain lightweight identifiers (file paths, stored queries, web links) and retrieve context dynamically" rather than loading everything into the prompt.

## Steps

### 1. Gather milestone definition

Read `docs/planning/milestones.md` and extract for M$1:
- Title
- Scope (what it builds)
- Acceptance criteria
- Exclusions (what it does NOT build)
- Dependencies on prior milestones

### 2. Gather scenarios

Read `docs/scenarios/index.md` and find ALL scenarios tagged with M$1. Group them by scenario file. Note the scenario number ranges and topics.

Also check whether any scenarios in other files are partially tagged with M$1 (some files span multiple milestones — include only the M$1 scenarios from those files).

### 3. Identify relevant source docs

Based on the milestone scope, determine which of these source docs are relevant:

| Source doc | When to include |
|------------|-----------------|
| `docs/specs/cli-spec.md` | Any milestone that implements CLI commands |
| `docs/specs/config-schema.md` | M1, or any milestone touching config |
| `docs/specs/github-api-inventory.md` | Any milestone making GitHub API calls |
| `docs/specs/error-handling.md` | Any milestone with error handling requirements |
| `docs/specs/sequence-diagrams.md` | Any milestone with multi-step flows |
| `docs/planning/functional-requirements.md` | Always — every milestone traces to functional requirements |
| `docs/planning/non-functional-requirements.md` | Always — every milestone traces to NFRs |
| `docs/specs/test-strategy.md` | If the milestone has special testing concerns |
| `docs/adr/*.md` | Only ADRs directly relevant to this milestone's decisions |

### 4. Verify anchors exist

For every link you plan to include in the manifest, verify the target anchor exists in the target file. Use Grep to search for the heading text. If an anchor does not exist, find the correct anchor or reference the file without a broken anchor.

### 5. Identify verification invariants

Read the relevant sections of these docs to find concrete, testable invariants for the Verification section:
- `docs/specs/sequence-diagrams.md` — endpoint call order, branching logic
- `docs/planning/functional-requirements.md` — specific FR IDs that map to this milestone
- `docs/planning/non-functional-requirements.md` — specific NFR IDs that map to this milestone
- `docs/specs/error-handling.md` — exit codes, degraded mode behavior
- `docs/specs/cli-spec.md` — output format, flag behavior

Verification MUST contain specific invariants, NOT vague statements. Good: "MUST call endpoints in this order: GET /repos, GET /git/ref". Bad: "MUST match the spec".

### 6. Generate the manifest

Write the file to `docs/milestones/m$1.md` using this exact structure:

```markdown
# M$1: [Title from milestones.md]

## Workflow

1. Read this manifest to understand scope and constraints.
2. Load the [acceptance scenarios](#acceptance-scenarios) for the current command.
3. Use `/mikey:tdd <scenario-file>` to implement against those scenarios.
4. After implementation, verify against [acceptance criteria](#acceptance-criteria)
   and [flow verification](#verification).
5. Use `/mikey:testify` to validate test quality.

## Scope

**MUST implement:**
- [bullet list from milestones.md scope, with package paths where known]

**MUST NOT implement:**
- [bullet list of exclusions — things adjacent to this milestone that are
  out of scope, with brief reason if not obvious]

[Note any already-implemented items from prior milestones if relevant]

## Acceptance criteria

From [milestones.md](../planning/milestones.md#[anchor]):

[Numbered list — copied from milestones.md since these are the contract.
Each criterion MUST include the FR/NFR IDs it satisfies in parentheses.
Read docs/planning/functional-requirements.md and
docs/planning/non-functional-requirements.md to find matching IDs.
Example: "1. `quarantine init` creates a valid quarantine.yml... (FR-1.4.1, FR-1.11.1)"]

Requirements: [functional](../planning/functional-requirements.md),
[non-functional](../planning/non-functional-requirements.md).

## Specifications

Load these on demand — do not read all upfront:

| What | Reference |
|------|-----------|
| [description of what this spec covers] | [doc.md#anchor](../doc.md#anchor) |
[one row per relevant spec, with specific anchors]

## Acceptance scenarios

Use these with `/mikey:tdd`:

| Scenarios | File | Topic |
|-----------|------|-------|
| [range] | [file.md](../scenarios/v1/file.md) | [brief topic description] |
[one row per scenario file, grouped by file]

## Verification

**Flow correctness:** The implementation MUST match the sequence in
[sequence-diagrams.md#anchor](../specs/sequence-diagrams.md#anchor).
Specifically:
- [concrete invariant with MUST]
- [concrete invariant with MUST]

**Requirements:** The implementation MUST satisfy [FR-X.Y.Z]
in [functional-requirements.md](../planning/functional-requirements.md)
and [NFR-X.Y.Z] in [non-functional-requirements.md](../planning/non-functional-requirements.md).

**Build:** `make cli-build && make cli-test && make cli-lint` MUST pass.
```

### 7. Validate the result

After writing, verify:
- Line count is between 60 and 110 lines
- No content is duplicated from source docs (only scope and acceptance criteria are inline)
- Every link has a specific anchor that exists in the target file
- MUST/MUST NOT language is used for all constraints
- The six sections appear in exact order: Workflow, Scope, Acceptance criteria, Specifications, Acceptance scenarios, Verification

### 8. Report

Print a summary:
- Milestone title
- Number of acceptance criteria
- Number of scenario files referenced (and total scenario count)
- Number of specification links
- Number of verification invariants
- Final line count
