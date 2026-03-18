# ADR-019: Required Initialization via `quarantine init`

**Status:** Accepted
**Date:** 2026-03-17

## Context

There is a tension between zero-friction adoption and reliable first-run UX. The architecture doc (ADR-006) originally implied that the `quarantine/state` branch would be created on first run of `quarantine run`. However, a first run in CI involves multiple potential failure points: GitHub token permissions may be insufficient, the branch may fail to create, or the config may be missing or malformed. Surfacing these errors during a real CI run -- when a developer is waiting for test results -- creates a frustrating and hard-to-debug experience.

Additionally, the config file (`quarantine.yml`) requires decisions about framework, JUnit XML path, and retry count. Without an interactive setup step, users must manually create this file by reading documentation, increasing the chance of misconfiguration.

## Decision

`quarantine init` is required before `quarantine run`. Running `quarantine run` without prior initialization exits with code 2 and the message: `"Quarantine is not initialized for this repository. Run 'quarantine init' first."`

`quarantine init` is interactive and performs the following:

1. Prompts for framework (rspec, jest, vitest), retries, and JUnit XML output path.
2. Writes `quarantine.yml` to the repo root with the provided values.
3. Validates the GitHub token (`QUARANTINE_GITHUB_TOKEN` or `GITHUB_TOKEN`) by testing repo access.
4. Creates the `quarantine/state` branch with an empty `quarantine.json` if it does not already exist.
5. Prints a summary of what was configured and next steps (adding the workflow snippet).

A `--yes` flag for non-interactive mode (accept defaults) is deferred to v2.

## Alternatives Considered

- **Implicit initialization on first `quarantine run`:** Lower adoption friction (one step instead of two), but permission errors and branch creation failures surface during real CI runs. Developers must debug infrastructure issues while waiting for test results. The error messages are ambiguous (is the failure from the test suite or from quarantine setup?).
- **Implicit initialization with `quarantine doctor` as a separate step:** Relies on users remembering to run validate. Does not create the branch or config file.
- **Config file only, no init command:** Users manually create `quarantine.yml` and the branch. Higher error rate, no validation feedback, no guided setup.

## Consequences

- (+) First CI run is reliable -- all infrastructure (config, permissions, branch) is validated before tests ever run.
- (+) Clear separation between setup errors (during `init`, at a developer's terminal) and runtime errors (during `run`, in CI).
- (+) Interactive prompts guide users through configuration, reducing misconfiguration.
- (+) Permission errors surface immediately with clear diagnostic messages, not buried in CI logs.
- (-) Adoption requires two steps (init + workflow update) instead of one. This adds friction but is a one-time cost per repository.
- (-) No non-interactive mode in v1. CI-as-code setups that want to automate initialization must wait for the `--yes` flag in v2.
- (-) Overrides the architecture doc's original implicit branch creation behavior. Documentation must be updated to reflect this change.
