---
name: create-milestone
description: Generate a milestone manifest file — a lightweight routing document that points agents to source docs
argument-hint: "<milestone-number> [--plan <path>] | --validate [N]"
model: sonnet
effort: max
disable-model-invocation: false
user-invocable: true
allowed-tools: Read, Grep, Glob, Write, Bash, Agent, AskUserQuestion
---

## Mode detection

If `$1` is `--validate` or starts with `--validate`, run **validation mode** (see [Validation Mode](#validation-mode) at the end). Otherwise, generate a new manifest for M$1 at `docs/milestones/m$1.md`.

### Plan-assisted mode

If `--plan <path>` is provided, read the plan file first. Use its milestone
section (matching M$1) as the primary source for scope, acceptance criteria,
and scenario outlines — instead of relying solely on `docs/milestones/index.md`.
The plan file is produced by `/plan` and has a defined structure with
`## Milestones` containing per-milestone scope, criteria, and scenario outlines.

When a plan is provided:
- Step 1a uses the plan's milestone section for scope and acceptance criteria.
- Step 1b uses the plan's scenario outlines to locate or create scenarios.
- Steps 2-6 proceed normally (the plan supplements, not replaces, the
  standard manifest generation flow).

If the plan's milestone section conflicts with `index.md`, prefer the plan
(it is the more recent, reviewed artifact). Flag the discrepancy in the
report (step 7).

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

### 1. Gather milestone definition and scenarios

Use the Agent tool (subagent_type: Explore) to run steps 1a–1c in parallel, keeping the heavy file reads out of main context:

**1a. Milestone definition:** Read `docs/milestones/index.md` and extract for M$1: title, scope, acceptance criteria, exclusions, and dependencies on prior milestones.

**1b. Scenarios:** Read `docs/scenarios/index.md` and find ALL scenarios tagged with M$1. Group by scenario file. Note number ranges and topics. Check whether any scenarios in other files are partially tagged with M$1 (some files span multiple milestones — include only the M$1 scenarios).

**1c. Relevant ADRs:** Read `docs/adr/*.md` and identify only the ADRs directly relevant to this milestone's scope.

### 2. Clarify ambiguities

Before proceeding, review the gathered data for issues. Use AskUserQuestion if ANY of the following apply:

- The milestone definition in `index.md` is vague about scope boundaries (what's in vs. out)
- Scenarios don't clearly map to acceptance criteria (gaps or overlaps)
- Prior milestone dependencies appear unmet or unclear
- The milestone scope could reasonably be interpreted in multiple ways

Do NOT proceed past this step with unresolved ambiguities — they propagate into the manifest and mislead implementing agents.

### 3. Identify relevant source docs

Based on the milestone scope, determine which of these source docs are relevant:

| Source doc | When to include |
|------------|-----------------|
| `docs/specs/cli-spec.md` | Any milestone that implements CLI commands |
| `docs/specs/config-schema.md` | M1, or any milestone touching config |
| `docs/specs/github-api-inventory.md` | Any milestone making GitHub API calls |
| `docs/specs/error-handling.md` | Any milestone with error handling requirements |
| `docs/specs/sequence-diagrams.md` | Any milestone with multi-step flows |
| `docs/specs/functional-requirements.md` | Always — every milestone traces to functional requirements |
| `docs/specs/non-functional-requirements.md` | Always — every milestone traces to NFRs |
| `docs/specs/test-strategy.md` | If the milestone has special testing concerns |
| `docs/adr/*.md` | Only ADRs directly relevant to this milestone's decisions |

### 4. Verify anchors and gather invariants

Use the Agent tool (subagent_type: Explore) to run these in parallel:

**4a. Anchor verification:** For every link you plan to include in the manifest, verify the target anchor exists in the target file. Use Grep to search for the heading text. If an anchor does not exist, find the correct anchor or report it as missing.

**4b. Verification invariants:** Read the relevant sections of these docs to find concrete, testable invariants:
- `docs/specs/sequence-diagrams.md` — endpoint call order, branching logic
- `docs/specs/functional-requirements.md` — specific FR IDs that map to this milestone
- `docs/specs/non-functional-requirements.md` — specific NFR IDs that map to this milestone
- `docs/specs/error-handling.md` — exit codes, degraded mode behavior
- `docs/specs/cli-spec.md` — output format, flag behavior

If any anchors are missing with no clear alternative, use AskUserQuestion to ask which section to reference.

Verification MUST contain specific invariants, NOT vague statements. Good: "MUST call endpoints in this order: GET /repos, GET /git/ref". Bad: "MUST match the spec".

### 5. Generate the manifest

Write the file to `docs/milestones/m$1.md` using this exact structure:

```markdown
---
status: planned
---

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

From [milestones.md](index.md#[anchor]):

[Numbered list — copied from milestones.md since these are the contract.
Each criterion MUST include the FR/NFR IDs it satisfies in parentheses.
Read docs/specs/functional-requirements.md and
docs/specs/non-functional-requirements.md to find matching IDs.
Example: "1. `quarantine init` creates a valid quarantine.yml... (FR-1.4.1, FR-1.11.1)"]

Requirements: [functional](../specs/functional-requirements.md),
[non-functional](../specs/non-functional-requirements.md).

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
in [functional-requirements.md](../specs/functional-requirements.md)
and [NFR-X.Y.Z] in [non-functional-requirements.md](../specs/non-functional-requirements.md).

**Build:** `make cli-build && make cli-test && make cli-lint` MUST pass.
```

### 6. Validate the result

After writing, verify:
- Line count is between 60 and 110 lines
- No content is duplicated from source docs (only scope and acceptance criteria are inline)
- Every link has a specific anchor that exists in the target file
- MUST/MUST NOT language is used for all constraints
- The six sections appear in exact order: Workflow, Scope, Acceptance criteria, Specifications, Acceptance scenarios, Verification

### 7. Report

Print a summary:
- Milestone title
- Number of acceptance criteria
- Number of scenario files referenced (and total scenario count)
- Number of specification links
- Number of verification invariants
- Final line count

---

## Validation Mode

Triggered by `/create-milestone --validate` or `/create-milestone --validate N`.

- `--validate` (no number): validate ALL manifests in `docs/milestones/m*.md`
- `--validate N`: validate only `docs/milestones/mN.md`

### Validation checks

For each manifest file, run these checks and report pass/fail:

1. **Line count.** SHOULD NOT exceed 100 lines.
2. **Section order.** MUST contain exactly these 6 `##` sections in order:
   Workflow, Scope, Acceptance criteria, Specifications, Acceptance scenarios,
   Verification. No extra `##` sections allowed.
3. **Workflow template.** Workflow section MUST match the standard 5-step
   template (steps 1-5 from the generation template). Custom workflows fail.
4. **RFC 2119 language.** Scope and Verification sections MUST contain at least
   one MUST or MUST NOT. Soft language ("try to", "ideally", "should consider")
   MUST NOT appear anywhere in the manifest.
5. **Anchor verification.** For every markdown link in the Specifications table
   and Verification section, verify the target anchor exists in the target file.
   Use Grep to check for the heading text that would produce the slug. Report
   broken links.
6. **FR/NFR traceability.** Acceptance criteria MUST contain at least one
   `(FR-X.Y.Z)` or `(NFR-X.Y.Z)` reference. Verify each referenced ID exists
   in `functional-requirements.md` or `non-functional-requirements.md`.
7. **Scenario coverage.** Every scenario tagged with this milestone's `[MN]`
   tag in `docs/scenarios/v1/*.md` MUST appear in the Acceptance scenarios
   table (directly or within a range). Report any missing scenarios.
8. **No content duplication.** Flag any block of 3+ consecutive lines in the
   Scope section that appears verbatim in a source doc (cli-spec, milestones,
   error-handling, etc.). Scope should summarize, not copy.

### Validation output

For each manifest, print:

```
M{N}: {title}
  ✓ Line count: {count} (60-110)
  ✗ Anchor broken: {link} → {file} has no heading matching "{slug}"
  ✓ Sections: 6/6 in order
  ...
  Result: PASS | FAIL ({count} issues)
```

At the end, print a summary: `{pass}/{total} manifests passed validation.`
