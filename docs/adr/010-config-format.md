# ADR-010: YAML Configuration Format

**Status:** Amended
**Date:** 2026-03-17 (originally accepted 2026-03-14)

## Context

The CLI needs a configuration file in the repo root. Need to choose a format and define the schema. The config must support v1 features while being forward-compatible with v2 expansions (additional issue trackers, notification channels, label customization) without breaking schema changes.

## Decision

YAML. File: `quarantine.yml` in repo root. Created interactively by `quarantine init`.

Example:

```yaml
version: 1
framework: jest                     # Required. Set by `quarantine init`.
                                    # v1: rspec, jest, vitest
                                    # v2+: pytest, go, maven
retries: 3                          # Optional. Default: 3. Range: 1-10.

junitxml: "results/*.xml"           # Optional. Default: framework-specific.
                                    # Glob pattern for JUnit XML output files.

github:
  owner: my-org                     # Optional. Auto-detected from git remote.
  repo: my-project                  # Optional. Auto-detected from git remote.

issue_tracker: github               # Optional. Default: github.
                                    # v1: only "github" accepted.
                                    # v2+: jira, linear, etc.

labels:                             # Optional. Default: [quarantine].
  - quarantine                      # v1: only ["quarantine"] accepted.
                                    # v2+: custom labels.

notifications:                      # Optional. Default: { github_pr_comment: true }.
  github_pr_comment: true           # v1: only valid key. true/false.
                                    # v2+: adds slack, email, etc.
  # slack:                          # v2+ only
  #   webhook_url: https://...
  #   threshold: 10

storage:
  branch: quarantine/state          # Optional. Default: quarantine/state.

exclude:                            # Optional. Default: none.
  - "test/integration/**"           # Patterns for tests quarantine ignores
  - "TestSlowService"               # entirely (no retry, no quarantine, no issue).

rerun_command: ""                   # Optional. Override auto-detected rerun
                                    # command template. Uses {name}, {classname},
                                    # {file} placeholders.
```

Key changes from original decision:

1. **`framework` is required.** No auto-detection. Set interactively by `quarantine init`.
2. **`storage.backend` removed.** v1 only supports branch storage. The `actions-cache` fallback is an internal degraded-mode detail, not a user-facing config option.
3. **Forward-compatible fields added:** `issue_tracker`, `labels`, and `notifications` exist in the schema with v1-restricted values. See ADR-021 for the pattern.
4. **`exclude` field added** for v1. Quarantine ignores matched tests entirely -- no retry, no quarantine, no issue creation.
5. **`quarantine init` creates the config** interactively, prompting for framework, retries, and JUnit XML path.

CLI also supports a `quarantine validate` command to verify config and print resolved values (including auto-detected fields like `github.owner` and `github.repo`).

## Alternatives Considered

- **JSON:** No comments (significant drawback for config files), familiar but more verbose (braces, quotes). `yq` validates YAML equivalently to `jq` for JSON, and `quarantine validate` provides built-in validation.
- **TOML:** Good format, supports comments, but less familiar in CI ecosystem (CI configs are predominantly YAML). Lower tooling availability.
- **No config file (all CLI flags):** Works for simple cases but does not scale for project-level defaults. CLI flags still override config file values.

## Consequences

- (+) Comments allow documenting why settings are configured a certain way.
- (+) Familiar to anyone who writes CI configs (GitHub Actions, GitLab CI, etc.).
- (+) Less verbose than JSON.
- (+) Well-supported in Go (gopkg.in/yaml.v3).
- (+) Forward-compatible fields allow v2 to expand allowed values without schema changes.
- (+) `quarantine init` ensures the config is valid from the start.
- (-) Indentation sensitivity can cause subtle errors. Mitigated by `quarantine validate` command.
- (-) YAML has well-known footguns (the Norway problem, implicit type coercion). Mitigated by strict parsing mode and limited schema.
- (-) Forward-compatible fields with restricted values may confuse users who try to set v2 values. Mitigated by clear error messages (e.g., "jira is not supported in v1. Supported values: github").
