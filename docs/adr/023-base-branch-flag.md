# ADR-023: `--base-branch` Flag for Non-GHA CI Systems

**Status:** Accepted (v2)
**Date:** 2026-03-27

## Context

ADR-022 introduces new-test detection: the CLI skips GitHub Issue creation for flaky tests that are new to the PR (file or test case not on the base branch). This relies on `GITHUB_BASE_REF`, an environment variable automatically set by GitHub Actions for `pull_request` and `pull_request_target` events.

Non-GitHub-Actions CI systems (Jenkins, CircleCI, GitLab CI, Buildkite, etc.) do not set `GITHUB_BASE_REF`. Without it, the diff check cannot run and the CLI falls back to treating all flaky tests as pre-existing — creating GitHub Issues even for tests that only exist in the PR. This is the safe default (per ADR-022), but it means non-GHA users permanently miss the new-test detection benefit.

## Decision

v2 adds a `--base-branch` CLI flag that provides the base branch name when `GITHUB_BASE_REF` is not set.

**Resolution order** for the base branch reference:

1. `--base-branch` flag (highest priority — explicit user override)
2. `GITHUB_BASE_REF` environment variable (GitHub Actions auto-detection)
3. Unset → diff check skipped → safe default (create issue)

**Usage:**

```bash
# GitHub Actions (no flag needed — GITHUB_BASE_REF is set automatically)
quarantine run --retries 3 -- jest --ci

# Jenkins / CircleCI / GitLab CI
quarantine run --retries 3 --pr 42 --base-branch main -- jest --ci
```

**Config equivalent (v2):** `quarantine.yml` gains an optional `base_branch` field for teams that always use the same base branch:

```yaml
base_branch: main
```

CLI flag overrides the config value. Both override `GITHUB_BASE_REF`. Note: GitHub Actions users should not normally use `--base-branch` or `base_branch` — `GITHUB_BASE_REF` is set automatically and is always correct for the triggering event. The flag exists for CI systems that lack an equivalent.

## Why v2

v1 targets GitHub Actions as the only fully-supported CI provider (ADR-013). The `GITHUB_BASE_REF` auto-detection covers the v1 target environment. Non-GHA CI systems work in v1 with degraded behavior (all flaky tests create issues) — functional but noisier. `--base-branch` is a refinement that makes quarantine equally effective across CI providers.

## Alternatives Considered

- **Auto-detect base branch from git (e.g., `git merge-base HEAD origin/main`):** Assumes the base branch is `main`, which is not always true (`master`, `develop`, release branches). Unreliable without explicit configuration. Rejected.
- **Require `GITHUB_BASE_REF` to be manually set in non-GHA CI:** Forces users to learn a GitHub-Actions-specific env var convention. The `--base-branch` flag is more discoverable and idiomatic for CLI tools. Rejected.
- **Always skip issue creation in PR builds:** Avoids the problem entirely but loses the ability to create issues for genuinely pre-existing flaky tests triggered by a PR. Rejected per ADR-022.

## Consequences

- (+) Non-GHA CI systems get the same new-test detection as GitHub Actions.
- (+) `--base-branch` is self-documenting in CI scripts — clearer than setting an env var.
- (+) Config-file `base_branch` avoids repeating the flag in every CI command.
- (-) Another CLI flag and config field to document and maintain.
- (-) Users on non-GHA CI must know to add the flag to get optimal behavior. Mitigated by documentation and by the v1 safe default (issue is created, which is visible and correctable).
