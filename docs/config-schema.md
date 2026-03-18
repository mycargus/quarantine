# quarantine.yml Configuration Schema

> Last updated: 2026-03-17
>
> Canonical reference for every field in `quarantine.yml`. This file is created
> by `quarantine init` and lives in the repository root.

## Complete Schema

```yaml
version: 1
framework: jest
retries: 3
junitxml: "results/*.xml"
github:
  owner: my-org
  repo: my-project
issue_tracker: github
labels:
  - quarantine
notifications:
  github_pr_comment: true
storage:
  branch: quarantine/state
exclude:
  - "test/integration/**"
rerun_command: ""
```

---

## Field Reference

### `version`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | Integer                        |
| Required    | Yes                            |
| Default     | None (must be specified)       |
| Valid values| `1`                            |

Schema version number. Allows the CLI to detect incompatible config files from
future versions.

**Validation rules:**
- Must be present.
- Must be the integer `1`. String values (e.g., `"1"`) are rejected.
- Values greater than `1` produce an error: `"Unsupported config version: {value}. This version of the CLI supports version 1."`

**Behavior when omitted:**
- Error: `"Missing required field 'version' in quarantine.yml."`

---

### `framework`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | String                         |
| Required    | Yes                            |
| Default     | None (must be specified)       |
| Valid values| `rspec`, `jest`, `vitest` (v1) |

The test framework in use. Set by `quarantine init` during interactive setup.
There is no auto-detection; the user must choose explicitly.

The framework determines:
- Default `junitxml` glob pattern (see [`junitxml`](#junitxml)).
- Rerun command construction for individual failing tests.
- Test exclusion flags used to skip quarantined tests.

**Validation rules:**
- Must be present.
- Must be one of the v1-supported values: `rspec`, `jest`, `vitest`.
- Case-sensitive. `Jest` or `JEST` are rejected.
- Values planned for v2+ (`pytest`, `go`, `maven`) produce: `"Framework '{value}' is not supported in this version. Supported frameworks: rspec, jest, vitest."`
- Unrecognized values produce: `"Unknown framework '{value}'. Supported frameworks: rspec, jest, vitest."`

**Behavior when omitted:**
- Error: `"Missing required field 'framework' in quarantine.yml."`

---

### `retries`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | Integer                        |
| Required    | No                             |
| Default     | `3`                            |
| Valid values| `1` through `10`               |

Number of times to re-run a failing test before declaring it a genuine failure.
A test that passes on any retry attempt is classified as flaky.

**Validation rules:**
- Must be an integer.
- Must be in the range 1--10 inclusive.
- Values below 1 produce: `"Invalid retries value: {value}. Must be between 1 and 10."`
- Values above 10 produce: `"Invalid retries value: {value}. Must be between 1 and 10."`
- Non-integer values (e.g., `3.5`, `"three"`) produce: `"Invalid retries value: expected integer, got {type}."`

**Behavior when omitted:**
- Uses default value of `3`.

**CLI override:** `--retries` flag on `quarantine run`.

---

### `junitxml`

| Property    | Value                            |
|-------------|----------------------------------|
| Type        | String (glob pattern)            |
| Required    | No                               |
| Default     | Framework-specific (see below)   |

Glob pattern for locating JUnit XML output files. Supports multiple files for
parallel test runners (e.g., Jest `--shard`, RSpec `parallel_tests`, Vitest
threads).

**Framework-specific defaults:**

| Framework | Default `junitxml` value | Notes |
|-----------|--------------------------|-------|
| `jest`    | `junit.xml`              | Requires `jest-junit` package. Output location depends on `JEST_JUNIT_OUTPUT_DIR` and `JEST_JUNIT_OUTPUT_NAME` env vars. |
| `rspec`   | `rspec.xml`              | Requires `rspec_junit_formatter` gem. Output path set via `--format RspecJunitFormatter --out rspec.xml`. |
| `vitest`  | `junit-report.xml`       | Built-in support via `--reporter=junit`. No third-party package needed. |

**Validation rules:**
- Must be a non-empty string.
- Must be a valid glob pattern (syntax errors produce a parse-time warning).
- The CLI resolves the glob at runtime after the test command completes. If no
  files match, the CLI logs an error suggesting the `--junitxml` flag and exits
  with the test runner's exit code.

**Behavior when omitted:**
- Uses the framework-specific default from the table above.

**CLI override:** `--junitxml` flag on `quarantine run`.

---

### `github`

A nested object for GitHub repository identification.

#### `github.owner`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | String                         |
| Required    | No                             |
| Default     | Auto-detected from git remote  |

The GitHub organization or user that owns the repository.

**Validation rules:**
- Must be a non-empty string if provided.
- Must be a valid GitHub owner name (alphanumeric and hyphens).

**Behavior when omitted:**
- Auto-detected by parsing the `origin` remote URL from the local git config.
  Supports both HTTPS (`https://github.com/owner/repo.git`) and SSH
  (`git@github.com:owner/repo.git`) remote formats.
- If auto-detection fails (e.g., no git remote, non-GitHub remote), the CLI
  logs an error: `"Could not detect GitHub owner from git remote. Set 'github.owner' in quarantine.yml or ensure a GitHub remote is configured."`

#### `github.repo`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | String                         |
| Required    | No                             |
| Default     | Auto-detected from git remote  |

The GitHub repository name (without the owner prefix).

**Validation rules:**
- Must be a non-empty string if provided.
- Must be a valid GitHub repository name.

**Behavior when omitted:**
- Auto-detected from the `origin` remote URL, same as `github.owner`.
- If auto-detection fails, the CLI logs an error: `"Could not detect GitHub repo from git remote. Set 'github.repo' in quarantine.yml or ensure a GitHub remote is configured."`

---

### `issue_tracker`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | String                         |
| Required    | No                             |
| Default     | `github`                       |
| Valid values| `github` (v1)                  |

Specifies the issue tracking system where the CLI creates tickets for flaky
tests.

**Forward compatibility:** This field exists so that `quarantine.yml` files
can be written today and extended in v2+ without breaking changes. v2+ will add
values such as `jira`.

**Validation rules:**
- Must be a string.
- v1 accepts only `github`.
- Any other value produces: `"Unsupported issue_tracker '{value}'. This version supports: github. Jira support is planned for a future release."`

**Behavior when omitted:**
- Uses default value of `github`.

---

### `labels`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | List of strings                |
| Required    | No                             |
| Default     | `["quarantine"]`               |
| Valid values| `["quarantine"]` (v1)          |

Labels applied to GitHub Issues created for flaky tests. Used for deterministic
issue deduplication (the CLI searches for existing issues by these labels plus
the test identifier).

**Forward compatibility:** This field exists so that v2+ can support custom
labels. In v1, only the exact list `["quarantine"]` is accepted.

**Validation rules:**
- Must be a list of strings.
- v1 requires exactly one element: `"quarantine"`.
- An empty list produces: `"Invalid labels: must contain at least one label. Default: ['quarantine']."`
- Additional labels produce: `"Custom labels are not supported in this version. Only ['quarantine'] is accepted."`
- Labels other than `"quarantine"` produce: `"Custom labels are not supported in this version. Only ['quarantine'] is accepted."`

**Behavior when omitted:**
- Uses default value of `["quarantine"]`.

---

### `notifications`

A nested object controlling how the CLI notifies developers about flaky test
results.

**Forward compatibility:** v2+ will add keys such as `slack` and `email`.
In v1, only `github_pr_comment` is accepted.

**Validation rules (top-level):**
- Must be an object (map).
- Any key other than `github_pr_comment` produces: `"Unknown notification channel '{key}'. This version supports: github_pr_comment. Slack and email notifications are planned for a future release."`

**Behavior when omitted:**
- Uses default value of `{ github_pr_comment: true }`.

#### `notifications.github_pr_comment`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | Boolean                        |
| Required    | No                             |
| Default     | `true`                         |
| Valid values| `true`, `false`                |

When `true`, the CLI posts or updates a PR comment summarizing flaky test
findings. Comments are identified by a hidden HTML marker
(`<!-- quarantine-bot -->`) so the CLI updates an existing comment rather than
creating duplicates.

**Validation rules:**
- Must be a boolean.
- Non-boolean values produce: `"Invalid value for notifications.github_pr_comment: expected boolean, got {type}."`

**Behavior when omitted:**
- Uses default value of `true`.

**Behavior when `false`:**
- The CLI does not post or update PR comments. All other behavior (issue
  creation, state updates, artifact upload) is unaffected.

---

### `storage`

A nested object controlling where quarantine state (`quarantine.json`) is
stored.

**Design note:** In an earlier design, `storage.backend` existed with values
`branch` and `actions-cache`. This was removed. v1 only supports branch-based
storage. The Actions cache is used internally as a degraded-mode fallback and
is not a user-facing configuration option.

#### `storage.branch`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | String                         |
| Required    | No                             |
| Default     | `quarantine/state`             |

The Git branch name where `quarantine.json` is stored. This branch is created
by `quarantine init` and is never merged into other branches. The CLI reads and
writes `quarantine.json` on this branch via the GitHub Contents API with
SHA-based compare-and-swap for optimistic concurrency.

**Validation rules:**
- Must be a non-empty string.
- Must be a valid Git branch name (no spaces, no `..`, no leading `/`, etc.).

**Behavior when omitted:**
- Uses default value of `quarantine/state`.

---

### `exclude`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | List of strings (glob patterns)|
| Required    | No                             |
| Default     | None (empty list; no exclusions)|

Patterns for tests that Quarantine ignores entirely. Matched tests are not
retried, not quarantined, and do not trigger issue creation. The behavior is as
if Quarantine is not installed for those tests.

**Pattern matching:**

Patterns are matched against the `test_id`, which has the format:

```
file_path::classname::name
```

The `::` delimiter was chosen because it does not appear in JUnit XML output
from any v1-supported framework. Standard glob syntax is supported:

- `*` matches any characters within a single segment.
- `**` matches across path separators (for the `file_path` portion).
- `?` matches a single character.

**Examples:**

```yaml
exclude:
  - "test/integration/**"           # Exclude all integration tests by file path
  - "**::SlowServiceTest::*"        # Exclude all tests in a specific class
  - "**::*timeout*"                  # Exclude tests with "timeout" in the name
```

**Validation rules:**
- Must be a list of strings.
- Each string must be a valid glob pattern.
- Invalid glob syntax produces a warning at parse time but does not prevent the
  CLI from running.

**Behavior when omitted:**
- No tests are excluded. All tests are eligible for retry and quarantine.

---

### `rerun_command`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | String                         |
| Required    | No                             |
| Default     | `""` (empty; use auto-detected)|

Override the auto-detected rerun command template used to re-run individual
failing tests. This is useful for frameworks not supported in v1 or for
non-standard test runner configurations.

**Placeholder variables:**

| Placeholder   | Description                                    |
|---------------|------------------------------------------------|
| `{name}`      | Test name from JUnit XML `name` attribute      |
| `{classname}` | Class name from JUnit XML `classname` attribute|
| `{file}`      | File path extracted from the test identifier   |

**Example:**

```yaml
rerun_command: "npx jest --testPathPattern={file} --testNamePattern={name}"
```

**Framework-specific auto-detected commands (when `rerun_command` is empty):**

| Framework | Auto-detected rerun command                                |
|-----------|------------------------------------------------------------|
| `jest`    | `jest --testNamePattern "{name}"`                          |
| `rspec`   | `rspec -e "{name}"`                                       |
| `vitest`  | `vitest run --reporter=junit {file} -t "{name}"`          |

**Validation rules:**
- Must be a string.
- When non-empty, the CLI uses this template verbatim instead of the
  framework-specific default.
- The CLI does not validate that the command is executable at config parse time;
  errors surface at runtime when the rerun is attempted.

**Behavior when omitted or empty:**
- Uses the framework-specific auto-detected rerun command.

---

## Authentication

Tokens are **never** stored in `quarantine.yml`. Authentication is handled
exclusively through environment variables:

| Environment variable         | Purpose                                    |
|------------------------------|--------------------------------------------|
| `QUARANTINE_GITHUB_TOKEN`    | Preferred. GitHub PAT for API access.      |
| `GITHUB_TOKEN`               | Fallback. Commonly set in GitHub Actions.  |

The CLI checks `QUARANTINE_GITHUB_TOKEN` first. If not set, it falls back to
`GITHUB_TOKEN`. If neither is set, the CLI cannot perform GitHub operations
(state management, issue creation, PR comments) and runs in degraded mode
(unless `--strict` is set, which causes exit 2).

**Rate limit implications:**

| Token type              | Rate limit         |
|-------------------------|--------------------|
| `GITHUB_TOKEN` (Actions)| 1,000 req/hr/repo |
| Personal Access Token   | 5,000 req/hr       |
| GitHub App (v2+)        | 5,000--12,500 req/hr|

---

## File Location and Discovery

- The CLI looks for `quarantine.yml` in the current working directory (repo root).
- The `--config` flag on `quarantine run` overrides the path.
- `quarantine init` creates the file interactively.
- `quarantine validate` parses and validates the file, printing resolved values
  (including auto-detected `github.owner`, `github.repo`, and
  framework-specific defaults).

---

## Validation Summary

`quarantine validate` checks all of the following and reports errors or
warnings:

| Check                                | Severity |
|--------------------------------------|----------|
| `version` missing or unsupported     | Error    |
| `framework` missing or unsupported   | Error    |
| `retries` out of range               | Error    |
| `junitxml` invalid glob syntax       | Warning  |
| `issue_tracker` unsupported value    | Error    |
| `labels` non-default value           | Error    |
| `notifications` unknown keys         | Error    |
| `storage.branch` empty               | Error    |
| `exclude` invalid glob syntax        | Warning  |
| Unknown top-level keys               | Warning  |
| GitHub token not found in env        | Warning  |
| Git remote not parseable (when `github` omitted) | Warning |

---

## Minimal Valid Configuration

The smallest valid `quarantine.yml`:

```yaml
version: 1
framework: jest
```

All other fields use defaults. The CLI auto-detects `github.owner` and
`github.repo` from the git remote.

---

## Full Example with All Defaults Shown

```yaml
version: 1
framework: jest
retries: 3
junitxml: "junit.xml"          # Framework-specific default for Jest
github:
  owner: my-org                # Auto-detected from git remote
  repo: my-project             # Auto-detected from git remote
issue_tracker: github
labels:
  - quarantine
notifications:
  github_pr_comment: true
storage:
  branch: quarantine/state
exclude: []
rerun_command: ""
```

---

*References: ADR-010 (YAML config format), ADR-016 (v1 framework scope),
ADR-003 (quarantine mechanism), pre-implementation tasks (task 4).*
