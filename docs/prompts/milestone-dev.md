# Implement milestone N

Use this prompt when implementing predefined milestones.

## Instructions

1. In the prompt content below, replace `N` with the number for the milestone you want to implement.
2. Copy + Paste the prompt into a fresh Claude Code session.

## Prompt

```
Implement milestone N.

Before writing any code, read `docs/milestones/m{N}.md` in the current context and answer the following:
- Do the acceptance scenarios cover all acceptance criteria? If no, you MUST flag any gaps.
- Are there ambiguities or contradictions? If so, you MUST stop and ask.

Then work through the manifest ONE SCENARIO AT A TIME:

1. TDD. `/mikey:tdd --validate <scenario-file>#<scenario-number>`
   - ONE scenario per invocation. Do NOT batch multiple scenarios.
   - Batching defeats the Red-Green-Refactor discipline of TDD.
   - Start with integration or e2e tests. They catch real issues faster
     than unit tests and drive better design. Add unit tests for pure
     functions extracted during the Refactor step.
2. Validate. `/mikey:testify <path> --with-design` — you MUST fix all issues before moving on.
3. Commit. Every chunk of work MUST be committed with a passing build:
   - `make cli-build && make cli-test && make cli-lint` (CLI milestones)
   - `make dash-build && make dash-test && make dash-lint` (dashboard milestones)
   - Commit message: `milestone {N}: <description of what changed>`
   - Each commit is a safe rollback point. Never accumulate uncommitted work
     across multiple scenarios.
4. E2E. Run `make e2e-test` after the first scenario that touches GitHub API
   integration. Do NOT wait until the end — e2e tests catch issues that
   unit and integration tests miss (API caching, shell execution, real
   network behavior).
5. Verify. `/verify-milestone {N}` when all scenarios are done. You MUST fix failures before reporting completion.
6. Report. Summarize what was implemented, what was verified, and any deviations from the manifest.

Constraints:
- You MUST adhere to all functional and non-functional requirements.
- One concern per change. You MUST NOT allow scope drift.
- You MUST NOT modify existing files in `docs/scenarios/` without confirmation from me.
```