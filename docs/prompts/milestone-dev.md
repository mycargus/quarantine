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

Then work through the manifest one scenario at a time:

1. TDD. `/mikey:tdd --validate <scenario-file>`
2. Validate. `/mikey:testify <path> --with-design` — you MUST fix all issues before moving on.
3. Verify. `/verify-milestone {N}` when all scenarios are done. You MUST fix failures before reporting completion.
4. Report. Summarize what was implemented, what was verified, and any deviations from the manifest.

Constraints:
- You MUST adhere to all functional and non-functional requirements.
- One concern per change. You MUST NOT allow scope drift.
- You MUST NOT modify existing files in `docs/scenarios/` without confirmation from me.
```