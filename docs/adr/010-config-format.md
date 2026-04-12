# ADR-010: YAML Configuration Format

**Status:** Amended (2026-04-11: multi-suite config model)
**Date:** 2026-03-17 (originally accepted 2026-03-14)

## Context

The CLI needs a configuration file in the repo root. Need to choose a format and define the schema. The config must support v1 features while being forward-compatible with v2 expansions (additional issue trackers, notification channels, label customization) without breaking schema changes.

## Decision

YAML. File: `.quarantine/config.yml` at the repository root. Quarantine discovers
the repo root via git; if run from a subdirectory, it walks up to find the repo
root. Created by `quarantine init` with pre-filled suite entries for detected
frameworks.

### Amendment: Multi-Suite Config Model (2026-04-11)

The original flat schema (`quarantine.yml` with a single `framework` field) is
replaced by `.quarantine/config.yml` with a `test_suites` array. This amendment
is driven by the multi-suite support plan (`docs/plans/multi-suite-support.md`)
and ADR-030 (framework-agnostic design, which supersedes ADR-016).

**Key changes from prior amendment:**

1. **File location changed** from `quarantine.yml` (repo root) to
   `.quarantine/config.yml` (repo root). All quarantine-generated files live
   in `.quarantine/`.
2. **`framework` field removed.** Superseded by ADR-030. Quarantine is
   framework-agnostic.
3. **`exclude` field removed.** With the test suite model, the user controls
   which tests run via the suite's `command` field (see plan decision D12).
4. **`rerun_command` moved** from a top-level optional override to a required
   per-suite field.
5. **`test_suites` array added** (required, non-empty). Each suite has `name`,
   `command`, `junitxml`, `rerun_command` (all required), plus optional
   `retries`, `timeout`, `rerun_timeout`.
6. **`--config` flag removed.** There is only ever one config file per
   repository at a fixed, conventional location.
7. **Suite name constraints:** `[a-z0-9][a-z0-9-]*`, maximum 30 characters.
   Names are used in file paths, HTML markers, CLI arguments, and GitHub labels.
8. **`command` and `rerun_command` are YAML arrays**, executed via
   `exec.Command` (no shell). See ADR-031.
9. **`timeout` and `rerun_timeout` fields added** (optional, Go duration
   format). See ADR-033.

Example:

```yaml
version: 1

github:
  owner: my-org                     # Optional. Auto-detected from git remote.
  repo: my-project                  # Optional. Auto-detected from git remote.

issue_tracker: github               # Optional. Default: github.
                                    # v1: only "github" accepted.

labels:                             # Optional. Default: [quarantine].
  - quarantine                      # v1: only ["quarantine"] accepted.

notifications:                      # Optional.
  github_pr_comment: true           # Default: true. Boolean.

storage:
  branch: quarantine/state          # Optional. Default: quarantine/state.

test_suites:                        # Required. Non-empty array.
  - name: backend                   # Required. Unique. [a-z0-9][a-z0-9-]*, max 30.
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"           # Required. Glob for JUnit XML output.
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
    retries: 3                      # Optional. Default: 3. Range: 1-10.
    timeout: 30m                    # Optional. Default: 30m.
    rerun_timeout: 5m               # Optional. Default: 5m.

  - name: frontend
    command: ["npx", "jest", "--ci"]
    junitxml: "junit.xml"
    rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
    retries: 3
```

**Validation rules (enforced by `quarantine doctor` and `quarantine run`):**

- `test_suites` must be a non-empty array.
- Each suite must have `name`, `command`, `junitxml`, and `rerun_command`.
- `command` and `rerun_command` must be YAML arrays (not strings).
- Suite names must be unique, match `[a-z0-9][a-z0-9-]*`, max 30 characters.
- `retries` must be an integer in range 1--10.
- `timeout` and `rerun_timeout` must be valid Go duration strings.
- An empty `test_suites` array is rejected:
  `"No test suites configured. Edit .quarantine/config.yml to add one."`

**`quarantine init` behavior:**

- Detects test frameworks (jest/vitest from `package.json`, rspec from
  `Gemfile`) and pre-fills suite entries with `junitxml` and `rerun_command`
  defaults.
- Creates `.quarantine/config.yml`, `.quarantine/.gitignore`, and the
  `quarantine/state` branch with a `README.md`.
- Idempotent: re-running skips existing artifacts without overwriting.

CLI flags (`--retries`, `--junitxml`, `--rerun-command`, `--timeout`,
`--rerun-timeout`) override per-suite config values for that invocation only.

## Alternatives Considered

- **JSON:** No comments (significant drawback for config files), familiar but more verbose (braces, quotes). `yq` validates YAML equivalently to `jq` for JSON, and `quarantine doctor` provides built-in validation.
- **TOML:** Good format, supports comments, but less familiar in CI ecosystem (CI configs are predominantly YAML). Lower tooling availability.
- **No config file (all CLI flags):** Works for simple cases but does not scale for project-level defaults. CLI flags still override config file values.
- **One config file per suite:** Eliminates silent under-coverage (the #1
  product risk), causes shared settings drift, requires finding multiple
  files. Rejected in favor of single file with array.
- **String `command` field with `sh -c`:** Introduces shell interpretation,
  process groups, platform-dependent behavior. Rejected in favor of YAML
  array with `exec.Command`.

## Consequences

- (+) Comments allow documenting why settings are configured a certain way.
- (+) Familiar to anyone who writes CI configs (GitHub Actions, GitLab CI, etc.).
- (+) Less verbose than JSON.
- (+) Well-supported in Go (gopkg.in/yaml.v3).
- (+) Forward-compatible fields allow v2 to expand allowed values without schema changes.
- (+) `quarantine init` ensures the config is valid from the start.
- (+) Single file with `test_suites` array eliminates silent under-coverage.
- (+) Framework-agnostic: any test runner that produces JUnit XML works.
- (-) Indentation sensitivity can cause subtle errors. Mitigated by `quarantine doctor` command.
- (-) YAML has well-known footguns (the Norway problem, implicit type coercion). Mitigated by strict parsing mode and limited schema.
- (-) Array parsing and per-suite validation adds implementation complexity. Acceptable for the multi-suite use case.
