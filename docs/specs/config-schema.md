# `.quarantine/config.yml` Configuration Schema

> Last updated: 2026-04-11 (multi-suite rewrite; supersedes quarantine.yml)
>
> Canonical reference for every field in `.quarantine/config.yml`. This file is
> created by `quarantine init` and lives in the `.quarantine/` directory at the
> repository root. Quarantine discovers the repo root via git; running from a
> subdirectory is supported.

## Complete Schema

```yaml
version: 1

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

test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
    retries: 3
    timeout: 30m
    rerun_timeout: 5m

  - name: frontend
    command: ["npx", "jest", "--ci"]
    junitxml: "junit.xml"
    rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
    retries: 3
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
- Values greater than `1` produce an error:
  `"Unsupported config version: {value}. This version of the CLI supports version 1."`

**Behavior when omitted:**
- Error: `"Missing required field 'version' in .quarantine/config.yml."`

---

### `github`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | Object                         |
| Required    | No                             |
| Default     | Auto-detected from git remote  |

GitHub repository coordinates. Both subfields are auto-detected from the `origin`
git remote if not specified.

**Subfields:**

| Field   | Type   | Required | Default           |
|---------|--------|----------|-------------------|
| `owner` | String | No       | Auto-detected     |
| `repo`  | String | No       | Auto-detected     |

Auto-detection parses the `origin` remote URL and extracts `owner` and `repo`.
Supports both HTTPS (`https://github.com/owner/repo.git`) and SSH
(`git@github.com:owner/repo.git`) formats. If auto-detection fails, quarantine
errors with an actionable message.

**Validation rules:**
- If specified, both `owner` and `repo` must be non-empty strings.
- Partial specification (only `owner`, or only `repo`) is an error.

---

### `issue_tracker`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | String                         |
| Required    | No                             |
| Default     | `"github"`                     |
| Valid values| `"github"` (v1 only)           |

Which system to use for flaky test issue tracking.

**Validation rules:**
- v1: only `"github"` is accepted. Any other value produces:
  `"jira is not supported in v1. Supported values: github"`

---

### `labels`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | Array of strings               |
| Required    | No                             |
| Default     | `["quarantine"]`               |

Labels applied to created GitHub Issues.

**Validation rules:**
- v1: only `["quarantine"]` is accepted. Custom labels not supported.
- Used as the base label in the issue dedup label pair: `["quarantine",
  "quarantine:<suite-name>:<hash>"]`.

---

### `notifications`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | Object                         |
| Required    | No                             |
| Default     | `{ github_pr_comment: true }`  |

**Subfields:**

| Field                | Type    | Required | Default |
|----------------------|---------|----------|---------|
| `github_pr_comment`  | Boolean | No       | `true`  |

When `github_pr_comment: false`, no PR comment is posted or updated. The
`quarantine run` flag `--pr` is still accepted but has no effect on PR comment
creation.

---

### `storage`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | Object                         |
| Required    | No                             |
| Default     | `{ branch: "quarantine/state"}`|

**Subfields:**

| Field    | Type   | Required | Default             |
|----------|--------|----------|---------------------|
| `branch` | String | No       | `"quarantine/state"`|

The Git branch on which per-suite state files are stored. See [Per-suite state
files](#per-suite-state-files).

---

### `test_suites`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | Array of suite objects         |
| Required    | Yes                            |
| Default     | None (must be specified)       |

The list of test suite configurations. Must be non-empty. Each entry fully
describes one suite's command, JUnit XML output location, and rerun behavior.

**Validation rules:**
- Must be a non-empty array. An empty `test_suites` is invalid:
  `"No test suites configured. Edit .quarantine/config.yml to add one."`
- Suite names must be unique within the file.
- Detected by `quarantine doctor`; also checked at the start of `quarantine run`.

---

#### Suite fields

##### `name`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | String                         |
| Required    | Yes                            |
| Pattern     | `[a-z0-9][a-z0-9-]*`          |
| Max length  | 30 characters                  |

The suite identifier. Used as:
- Directory name on the state branch (`.quarantine/<name>/state.json`)
- Directory name in the working tree (`.quarantine/<name>/results.json`,
  `.quarantine/<name>/quarantined-files.txt`)
- PR comment marker (`<!-- quarantine:<name> -->`)
- CLI argument (`quarantine run <name>`)
- Issue dedup label component (`quarantine:<name>:<hash>`)

**Validation rules:**
- Must match `[a-z0-9][a-z0-9-]*` (lowercase alphanumeric, hyphens allowed,
  must start with alphanumeric).
- Maximum 30 characters (keeps GitHub label `quarantine:<name>:<hash>` under
  the 50-character limit).
- Must be unique across all suites in the file.
- If invalid, `quarantine run` errors and aborts (does NOT truncate).

---

##### `command`

| Property    | Value                                     |
|-------------|-------------------------------------------|
| Type        | Array of strings (YAML sequence)          |
| Required    | Yes                                       |

The user's existing test command as an explicit argument array. Quarantine
executes it via `exec.Command(command[0], command[1:]...)` without modification.
No shell is involved.

**Examples:**
```yaml
command: ["bundle", "exec", "rspec"]
command: ["npx", "jest", "--ci"]
command: ["npx", "vitest", "run", "--reporter=junit"]
command: ["sh", "-c", "npm test && echo done"]  # explicit shell when needed
```

**Validation rules:**
- Must be a YAML sequence (array), not a string. A string value is rejected:
  `"Error [config]: test_suites[0].command must be a YAML array, not a string.
  Use: command: ["bundle", "exec", "rspec"]"`
- Must be non-empty.

**Why array, not string:** A string command requires `sh -c`, which introduces
shell interpretation, process group management, and platform differences. The
array format uses `exec.Command` directly — identical to how `rerun_command` is
executed — and is safe against shell metacharacters in test names.

---

##### `junitxml`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | String (glob pattern)          |
| Required    | Yes                            |

A glob pattern for the JUnit XML output file(s) produced by the suite's
`command`. Quarantine reads the XML after the command completes.

**Examples:**
```yaml
junitxml: "rspec.xml"
junitxml: "junit.xml"
junitxml: "junit-report.xml"
junitxml: "test-results/junit.xml"
junitxml: "results/*.xml"              # multiple XML files (parallel runners)
```

**Validation rules:**
- Must be present. No default.
- Glob is resolved relative to the repository root.
- If no files match the glob after the command completes, and the command
  exited non-zero, this is a command crash (see [Error handling](#error-handling)).

JUnit XML is produced by the test runner, not by quarantine. Quarantine reads
it but does not generate it.

---

##### `rerun_command`

| Property    | Value                                     |
|-------------|-------------------------------------------|
| Type        | Array of strings (YAML sequence)          |
| Required    | Yes                                       |

The command to re-run a single failing test. Executed via `exec.Command` (no
shell). Supports placeholders substituted from JUnit XML:

| Placeholder   | Source                                       |
|---------------|----------------------------------------------|
| `{name}`      | `name` attribute from JUnit `<testcase>`     |
| `{classname}` | `classname` attribute from JUnit `<testcase>`|
| `{file}`      | `file_path` component of the test ID         |

Placeholders are substituted within each array element via simple string
replacement before `exec.Command` is called. No shell is involved, so
shell metacharacters in test names are literal and harmless.

**Init defaults** (pre-filled by `quarantine init` for known runners):

| Detected runner | Pre-filled `rerun_command` |
|-----------------|---------------------------|
| jest            | `["npx", "jest", "--testNamePattern", "{name}"]` |
| rspec           | `["bundle", "exec", "rspec", "-e", "{name}"]` |
| vitest          | `["npx", "vitest", "run", "--reporter=junit", "{file}", "-t", "{name}"]` |

**Validation rules:**
- Must be a YAML sequence (array), not a string. Same rejection as `command`.
- Must be non-empty.

---

##### `retries`

| Property    | Value          |
|-------------|----------------|
| Type        | Integer        |
| Required    | No             |
| Default     | `3`            |
| Range       | `1` – `10`     |

Number of times to re-run a failing test before classifying it as a genuine
failure. A test that passes on any retry is classified as flaky.

**Validation rules:**
- Must be an integer.
- Must be in range 1–10.
- Out-of-range values are rejected by `quarantine doctor` and `quarantine run`.

---

##### `timeout`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | Duration string                |
| Required    | No                             |
| Default     | `"30m"`                        |

Maximum time to wait for the suite's `command` to complete. Uses Go's
`time.ParseDuration` format: `30m`, `1h`, `90s`, `2h30m`.

When the timeout elapses:
1. `SIGTERM` is sent to the process.
2. Quarantine waits 5 seconds for graceful shutdown.
3. `SIGKILL` is sent if the process is still running.
4. If JUnit XML exists at kill time, partial results are processed.
5. If no JUnit XML exists, treated as command crash (exit 2).

**Validation rules:**
- Must be a valid Go duration string. Invalid values are rejected by
  `quarantine doctor`.
- Override with `--timeout` CLI flag for a single invocation.

---

##### `rerun_timeout`

| Property    | Value                          |
|-------------|--------------------------------|
| Type        | Duration string                |
| Required    | No                             |
| Default     | `"5m"`                         |

Maximum time to wait for each individual `rerun_command` invocation. When this
elapses, the rerun process is killed (`SIGKILL`) and the test is classified as
`"unresolved"`. The run continues processing other tests.

**Validation rules:**
- Same as `timeout`: must be a valid Go duration string.
- Override with `--rerun-timeout` CLI flag.

---

## Per-suite state files

Each suite maintains its own state file at `.quarantine/<suite-name>/state.json`
on the `quarantine/state` branch (or whichever branch `storage.branch` specifies).
State files are created on the first `quarantine run` for each suite — not during
`quarantine init`.

Test IDs are unique within a suite's state file. Two suites may independently
track the same test ID. See ADR-032.

---

## Generated files

`quarantine init` creates:
- `.quarantine/config.yml` — the config file (source-controlled)
- `.quarantine/.gitignore` — ignores all runtime files except `config.yml`

The `.gitignore` contents:
```gitignore
# Ignore all runtime files. Only config.yml is source-controlled.
*
!.gitignore
!config.yml
```

`quarantine run` generates in the working tree (NOT source-controlled):
- `.quarantine/<suite>/results.json` — full test run results for dashboard
- `.quarantine/<suite>/quarantined-files.txt` — deduplicated quarantined file
  paths (written before the test command runs)

`quarantine run` generates on the state branch (NOT in working tree):
- `.quarantine/<suite>/state.json` — current quarantine state per suite

---

## Error handling

### Command crash detection

When the suite's `command` exits non-zero AND no files match the `junitxml`
glob:

```
Error [crash]: test command exited with code 1 but no JUnit XML files found at 'junit.xml'.
This usually means the test runner crashed before producing results.
Check that:
  - Your test command ('npx jest --ci') runs successfully outside of quarantine
  - JUnit XML output is configured in your test runner
  - the junitxml path in .quarantine/config.yml matches where your runner writes XML
```

Exit code: `2`.

### Rerun failure

When `rerun_command` fails for a specific test, that test is classified as
`"unresolved"`. The run continues. See FR-1.1.9.

### Config validation errors

All config errors use the `Error [config]:` prefix:
```
Error [config]: test_suites[0].command must be a YAML array, not a string.
Error [config]: suite "My Backend!": name must match [a-z0-9][a-z0-9-]* (max 30 chars)
Error [config]: test_suites is required and must be non-empty
```

---

## CLI flags that override suite config

These flags override the corresponding per-suite config value for a single
`quarantine run` invocation. The config file is not modified.

| Flag               | Overrides          |
|--------------------|-------------------|
| `--retries N`      | `retries`         |
| `--junitxml GLOB`  | `junitxml`        |
| `--timeout DUR`    | `timeout`         |
| `--rerun-timeout DUR` | `rerun_timeout` |

---

## `quarantine init` behavior

`quarantine init` creates `.quarantine/config.yml` with:
1. Shared settings (`version`, `github`, `issue_tracker`, `labels`,
   `notifications`, `storage`) with auto-detected `github.owner`/`repo`.
2. Pre-filled `test_suites` entries for each detected framework (jest/vitest
   from `package.json`, rspec from `Gemfile`).
3. When no framework is detected: a commented example suite entry.

Init is idempotent: re-running skips existing artifacts without overwriting.
See NFR-2.2.4.

`quarantine doctor` validates the config and prints resolved values including
auto-detected fields.

---

## Removed fields (previously in `quarantine.yml`)

The following fields existed in the old `quarantine.yml` format and are **not
present** in `.quarantine/config.yml`:

| Removed field  | Reason |
|----------------|--------|
| `framework`    | Removed per ADR-030 (framework-agnostic design). Suite's `command` replaces the role of framework-specific defaults. |
| `exclude`      | Removed per plan D12. Suite's `command` controls which tests run. |
| `rerun_command` (top-level) | Moved to per-suite required field. |
| `junitxml` (top-level) | Moved to per-suite required field. |
| `retries` (top-level) | Moved to per-suite optional field. |

The `--config PATH` flag is also removed. There is only one config file per
repository, at `.quarantine/config.yml`.
