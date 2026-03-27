# ADR-022: GitHub Issue Creation Scope — PR-Only vs. Existing Tests

**Status:** Accepted
**Date:** 2026-03-25

## Context

When a flaky test is detected in a PR, the CLI creates a GitHub Issue and posts a PR comment. However, the value of a GitHub Issue depends on whether the test represents a persistent problem in the codebase.

Three distinct cases arise:

1. **Test file introduced in the PR:** The flaky test does not yet exist on the base branch. The test may never land in `main` (PR abandoned, reverted, or refactored). A GitHub Issue created here may rot with no clear owner or resolution path. The test is not a codebase-level problem yet — it is the PR author's problem.

2. **New test case added to an existing file:** The test file pre-exists on the base branch, but the specific flaky test case was added in this PR. Same reasoning as case 1 — the test case may never land in `main`. A file-level check alone would miss this and incorrectly create an issue.

3. **Pre-existing test case in a pre-existing file:** The flaky test exists in the shared codebase. Flakiness here is a persistent problem that affects every developer and every CI run. A GitHub Issue is the right tracking mechanism.

A fourth case requires no special handling: **no base ref available** (direct push to main, scheduled run, or non-GHA CI that does not set `GITHUB_BASE_REF`). The diff check cannot run, and the CLI falls back to treating all flaky tests as pre-existing — creating a GitHub Issue. This is the safe default. v2 adds `--base-branch` (ADR-023) so non-GHA CI systems can provide the base ref explicitly.

## Decision

GitHub Issues are only created for flaky tests that already exist on the base branch. The detection is test-level, not file-level.

### Detection: is this test new to the PR?

Two-step check for each flaky test, given `GITHUB_BASE_REF` is set:

**Step 1 — Ensure the base ref is available (shallow clone protection):**

```
git fetch origin ${GITHUB_BASE_REF} --depth=1 2>/dev/null
```

If this fails, fall back to safe default (treat all tests as pre-existing → create issues). GitHub Actions' default `fetch-depth: 1` does not include the base branch, so this fetch is necessary.

**Step 2a — Is the file new?**

```
git diff --name-only --diff-filter=A origin/${GITHUB_BASE_REF}
```

If the flaky test's `file_path` appears in this list, the entire file is new to the PR. All tests in it are new → skip issue creation.

**Step 2b — Is the test case new in a modified file?**

If the file is NOT new (it pre-exists on the base branch), check whether the specific test case was added in this PR:

```
git diff origin/${GITHUB_BASE_REF} -- {file_path}
```

Search the added lines (`+` prefix) for the test name (the `name` attribute from JUnit XML). If the test name appears in an added line → the test case is new to this PR → skip issue creation.

This is a heuristic. It covers the common case (developer adds a new `it('...')` / `test('...')` / `it '...'` block). Edge cases where it may not match:
- Dynamic test names (`test.each`, interpolated strings): test name won't appear literally in source. Falls through to "create issue" — safe default.
- Test was renamed: old name removed, new name added. The new name appears in `+` lines, so it's treated as new — correct behavior (the old test no longer exists).

### Rules

| Context | Test new to PR? | quarantine.json | GitHub Issue | PR Comment |
|---------|----------------|-----------------|--------------|------------|
| PR build | Yes (new file or new test case in diff) | Not updated | Skipped | Posted (warns developer) |
| PR build | No (test pre-existed) | Updated | Created | Posted (references issue) |
| PR build, diff check unavailable | Unknown (fallback) | Updated | Created | Posted (references issue) |
| Non-PR build (main, scheduled) | N/A (no diff check) | Updated | Created | Skipped (no PR number) |

**Fallback:** If `GITHUB_BASE_REF` is not set (and `--base-branch` is not provided — see ADR-023, v2), `git fetch` fails, `git diff` fails, or the diff cannot be computed, the CLI treats the test as pre-existing and creates the issue. Safe default — avoids silently skipping issue creation.

**quarantine.json:** New-to-PR flaky tests are not added to `quarantine.json`. Without a GitHub Issue, there is no mechanism to unquarantine the test (per ADR-017, unquarantine requires closing an issue). The test will be re-detected on every CI run of the same PR, keeping the PR comment current, until the developer fixes it or the file lands in main.

## Alternatives Considered

- **File-level check only (`--diff-filter=A`):** Simple, but misses the common case where a developer adds a new test case to an existing file. The new test case would get a GitHub Issue even though it may never land in main. Rejected — test-level detection is worth the additional complexity.
- **Always create a GitHub Issue:** Simple, but pollutes the issue tracker with tests that may never land in main. A PR for a one-off experiment could generate issues that never get closed. Rejected.
- **Never create a GitHub Issue from a PR build:** Avoids noise, but misses the common case where a PR triggers flakiness in an existing test. Developers would have to wait for a main-branch run to see the issue. Rejected.
- **Use a `--no-issue` flag and leave it to the user:** Puts the burden on CI configuration. Most users won't know to set it, and the default behavior would still be wrong for the PR-only case. Rejected.
- **Create the issue but close it automatically if the PR is never merged:** Requires polling PR state, adds complexity, and GitHub does not support issue auto-close via API in a clean way. Rejected.
- **Compare test IDs against a previous JUnit XML on the base branch:** Accurate but requires storing and retrieving previous run results. Overly complex for v1. Rejected.

## Consequences

- (+) GitHub issue tracker stays clean — issues represent real, persistent problems in the codebase.
- (+) PR authors get immediate feedback (PR comment) without creating noise for maintainers — whether they added a new file or a new test case.
- (+) Developers who merge a flaky test get a GitHub Issue created on the first main-branch run, ensuring the problem is tracked.
- (-) Two-step detection (file check + diff search) is more complex than a simple file existence check.
- (-) Test name search in diff is heuristic — dynamic test names fall through to "create issue." Acceptable as a safe default.
- (-) Requires `git fetch` of the base ref in shallow clones, adding one network call.
- (-) If `GITHUB_BASE_REF` is unavailable outside GitHub Actions, the diff check is skipped and an issue is created (false positive). Acceptable — this only affects non-standard CI setups.
- (-) A new-to-PR flaky test will be re-detected on every CI run of the PR until fixed. This is intentional — the PR comment serves as persistent pressure on the author.
