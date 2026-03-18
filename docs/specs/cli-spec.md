# CLI Interface Specification

> Last updated: 2026-03-17
>
> This is the primary implementation reference for CLI development. Every
> command, subcommand, flag, argument, exit code, and output format is defined
> here. An implementer should be able to code against this document without
> re-reading the architecture doc.

## Commands Overview

```
quarantine init
quarantine run [flags] -- <test command>
quarantine doctor [flags]
quarantine version
```

---

## `quarantine init`

Initialize quarantine for a repository. This command is required before
`quarantine run` can be used.

### Behavior

1. **Interactive prompts** (in order):
   - **Framework** (required, no default): `Which test framework? [rspec/jest/vitest]`
     Validates that the answer is one of `rspec`, `jest`, `vitest`. Repeats the
     prompt on invalid input.
   - **Retries** (default: `3`): `How many retries for failing tests? [3]`
     Validates range 1--10. Accepts enter for default.
   - **JUnit XML path** (default: framework-specific):
     `Path/glob for JUnit XML output? [{default}]`
     Framework-specific defaults:
     | Framework | Default          |
     |-----------|------------------|
     | `jest`    | `junit.xml`      |
     | `rspec`   | `rspec.xml`      |
     | `vitest`  | `junit-report.xml` |
     Accepts enter for default.

2. **Create `quarantine.yml`** in the current directory with the values from
   the prompts. Includes `version: 1` and omits fields that match defaults
   (except `framework`, which is always written).

3. **Validate GitHub token:**
   - Check `QUARANTINE_GITHUB_TOKEN` env var, then fall back to `GITHUB_TOKEN`.
   - If neither is set, print error and exit 2.
   - Make an authenticated API call to verify the token is valid.

4. **Test repository read/write access:**
   - Detect `github.owner` and `github.repo` from the git remote.
   - Verify the token has `repo` scope (or sufficient permissions) by reading
     the repository metadata via the GitHub API.

5. **Create `quarantine/state` branch** with an empty `quarantine.json`:
   ```json
   {
     "version": 1,
     "updated_at": "<current ISO 8601 timestamp>",
     "tests": {}
   }
   ```
   - If the branch already exists, print a warning and skip creation.
   - Uses the GitHub Refs API to create the branch and Contents API to commit
     the initial `quarantine.json`.

6. **Jest-specific guidance:** If `framework` is `jest`, print a recommendation
   for `jest-junit` configuration that produces clean `test_id` output:
   ```
   Recommended jest-junit configuration (in jest.config.js or package.json):

     "jest-junit": {
       "classNameTemplate": "{classname}",
       "titleTemplate": "{title}",
       "ancestorSeparator": " > ",
       "addFileAttribute": "true"
     }

   This produces well-structured JUnit XML for quarantine's test identification.
   ```

7. **Print summary** of everything done, plus next steps:
   ```
   Quarantine initialized successfully.

     Config:     quarantine.yml (created)
     Framework:  jest
     Retries:    3
     JUnit XML:  junit.xml
     Branch:     quarantine/state (created)

   Next steps:
     1. Add quarantine to your CI workflow:

        - name: Run tests
          run: quarantine run -- jest --ci --reporters=default --reporters=jest-junit
          env:
            QUARANTINE_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

        - name: Upload quarantine results
          if: always()
          uses: actions/upload-artifact@v4
          with:
            name: quarantine-results-${{ github.run_id }}
            path: .quarantine/results.json

     2. Run `quarantine doctor` to verify your configuration.
   ```

### Flags

None. Interactive only in v1. `--yes` (accept defaults, non-interactive) is
deferred to v2.

### Exit Codes

| Code | Meaning                                   |
|------|-------------------------------------------|
| 0    | Initialization completed successfully     |
| 2    | Initialization failed (no token, API error, permissions error) |

### Environment Variables

| Variable                  | Purpose                         |
|---------------------------|---------------------------------|
| `QUARANTINE_GITHUB_TOKEN` | Preferred GitHub token          |
| `GITHUB_TOKEN`            | Fallback GitHub token           |

### Example

```bash
$ quarantine init
Which test framework? [rspec/jest/vitest] jest
How many retries for failing tests? [3]
Path/glob for JUnit XML output? [junit.xml]

Validating GitHub token... OK
Testing repository access (mycargus/my-app)... OK
Creating quarantine/state branch... OK

Quarantine initialized successfully.
  ...
```

---

## `quarantine run [flags] -- <test command>`

Run tests with quarantine detection and enforcement. This is the primary
command, intended to wrap the user's existing test command in CI.

### Prerequisites

Requires prior `quarantine init`. The CLI checks for the existence of the
`quarantine/state` branch. If the branch does not exist, the CLI prints the
following message and exits 2:

```
Error: Quarantine is not initialized for this repository. Run 'quarantine init' first.
```

### Execution Flow

1. **Read config:** Load `quarantine.yml` (or path from `--config`). Merge
   CLI flag overrides (flags take precedence over config values).

2. **Read quarantine state:** Fetch `quarantine.json` from the
   `quarantine/state` branch via the GitHub Contents API.
   - On failure: attempt to read from GitHub Actions cache (fallback).
   - On total failure: log warning, proceed with empty quarantine list
     (degraded mode). If `--strict` is set, exit 2 instead.

3. **Batch check issue status:** Use the GitHub Search API to find all closed
   issues with the `quarantine` label for this repository in a single API call.
   Compare against `quarantine.json` to identify unquarantined tests.

4. **Remove unquarantined tests:** For any quarantined test whose issue is now
   closed, remove it from the in-memory quarantine list. These tests will run
   normally. The removal is written back to `quarantine.json` in step 10.

5. **Apply exclude patterns:** Filter the quarantine list against `exclude`
   patterns from config and `--exclude` flags. Excluded tests are ignored
   entirely by quarantine (no retry, no quarantine, no issue).

6. **Construct test command with exclusions:** Augment the user's test command
   with framework-specific flags to exclude quarantined tests from execution.
   See [Framework-Specific Exclusion Flags](#framework-specific-exclusion-flags).

7. **Execute test command:** Run the augmented command. Capture the exit code.

8. **Parse JUnit XML:** Resolve the `junitxml` glob pattern. Parse all matching
   XML files. Merge results from multiple files (parallel runners).
   - If no files match: log error suggesting `--junitxml`, exit with the test
     runner's exit code.
   - If some files are malformed: parse what is possible, log warnings for
     failures, proceed.

9. **Retry failures:** For each failed test that is NOT in the quarantine list
   and NOT matched by exclude patterns:
   - Re-run the test individually up to N times (from `--retries` or config).
   - Use the framework-specific rerun command (see
     [Framework-Specific Rerun Commands](#framework-specific-rerun-commands)).
   - If the test passes on any retry: classify as **flaky**.
   - If the test fails all retries: classify as **genuine failure**.

10. **Update quarantine state:** For newly detected flaky tests, add entries to
    `quarantine.json`. For unquarantined tests (step 4), remove entries. Write
    the updated file to the `quarantine/state` branch using SHA-based
    compare-and-swap. Retry up to 3 times on 409 conflict.
    - On failure: log warning, proceed (degraded mode). If `--strict`, exit 2.

11. **Create GitHub Issues:** For each newly detected flaky test, create a
    GitHub Issue. Uses check-before-create with deterministic labels to avoid
    duplicates.
    - On failure: log warning, proceed (degraded mode). If `--strict`, exit 2.

12. **Post/update PR comment:** If a PR number is available (from
    `GITHUB_EVENT_PATH` or `--pr`) and `notifications.github_pr_comment` is
    `true`, post or update a PR comment. See
    [PR Comment Template](#pr-comment-template).
    - On failure: log warning, proceed (degraded mode). If `--strict`, exit 2.

13. **Write results to disk:** Write structured JSON results to
    `.quarantine/results.json` for artifact upload by the CI workflow.

14. **Determine exit code:** Based on test results only (not quarantine
    infrastructure status, unless `--strict`).

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--retries N` | int | From config, or `3` | Number of retries for failing tests. Range: 1--10. Overrides the `retries` value in `quarantine.yml`. |
| `--config PATH` | string | `quarantine.yml` | Path to the configuration file. |
| `--junitxml GLOB` | string | From config | Glob pattern for JUnit XML output files. Overrides the `junitxml` value in `quarantine.yml`. |
| `--dry-run` | bool | `false` | Show what would happen without making changes. Runs the test command and parses XML, but does not update `quarantine.json`, create issues, or post PR comments. Prints a summary of what would have been done. |
| `--verbose` | bool | `false` | Detailed output. Adds: API call details, retry attempts with timing, config resolution trace, quarantine list contents. |
| `--quiet` | bool | `false` | Minimal output. Suppresses success messages and summaries. Only shows warnings and errors. Mutually exclusive with `--verbose` (error if both set). |
| `--strict` | bool | `false` | Exit 2 on infrastructure errors instead of degraded mode. Useful for debugging and verifying setup. |
| `--pr N` | int | Auto-detected | Override PR number. Auto-detected from `GITHUB_EVENT_PATH` when running in GitHub Actions. Required for PR comments outside of GitHub Actions. |
| `--exclude PATTERN` | string | None | Additional exclude patterns. Repeatable. Merged with `exclude` patterns from config. Patterns match against `test_id` (see [Exclude Pattern Matching](#exclude-pattern-matching)). |

### Arguments

Everything after `--` is treated as the test command and its arguments. The
CLI executes this command as a subprocess.

```
quarantine run [flags] -- <test command and arguments>
```

The `--` separator is required. If omitted, the CLI prints a usage error
and exits 2.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0    | All tests passed. Includes: no failures, only flaky failures (quarantined), or degraded mode where the test suite itself passed. |
| 1    | Real test failures exist. At least one non-flaky, non-quarantined test failed after all retries. Exit code 1 exclusively means "your tests failed." |
| 2    | Quarantine error. Not initialized, invalid flags/config, or infrastructure failure with `--strict` set. Never used for test failures. |

**Guiding principle:** The exit code always reflects the test results, not
quarantine infrastructure status. Quarantine infrastructure failures result in
warnings (stderr), not non-zero exit codes -- unless `--strict` is set.

### Environment Variables

| Variable                  | Purpose                                          |
|---------------------------|--------------------------------------------------|
| `QUARANTINE_GITHUB_TOKEN` | Preferred GitHub token for API access             |
| `GITHUB_TOKEN`            | Fallback GitHub token                             |
| `GITHUB_ACTIONS`          | When set, emit `::warning` annotations for GHA    |
| `GITHUB_EVENT_PATH`       | Path to the event payload JSON (for PR number auto-detection) |

### Examples

**Basic usage:**
```bash
quarantine run -- jest --ci --reporters=default --reporters=jest-junit
```

**With flag overrides:**
```bash
quarantine run --retries 5 --junitxml "results/*.xml" --verbose -- rspec --format RspecJunitFormatter --out rspec.xml
```

**Dry run:**
```bash
quarantine run --dry-run -- vitest run --reporter=junit
```

**Strict mode (fail on infrastructure errors):**
```bash
quarantine run --strict -- jest --ci
```

**With additional exclusions:**
```bash
quarantine run --exclude "test/integration/**" --exclude "**::SlowServiceTest::*" -- jest --ci
```

---

## `quarantine doctor [flags]`

Validate `quarantine.yml` configuration. Reads and validates all fields against
the schema, prints the resolved configuration (including auto-detected values),
and reports errors and warnings.

### Behavior

1. Read `quarantine.yml` (or path from `--config`).
2. Validate every field against the rules defined in the
   [config schema](config-schema.md).
3. Resolve auto-detected values (`github.owner`, `github.repo` from git
   remote; framework-specific `junitxml` default).
4. Check forward-compatible fields (`issue_tracker`, `labels`, `notifications`)
   against v1 restrictions.
5. Check for GitHub token in environment variables (warning if missing).
6. Print results.

### Output Format

**On success (exit 0):**
```
quarantine.yml is valid.

Resolved configuration:
  version:               1
  framework:             jest
  retries:               3
  junitxml:              junit.xml
  github.owner:          mycargus (auto-detected)
  github.repo:           my-app (auto-detected)
  issue_tracker:         github
  labels:                [quarantine]
  notifications:
    github_pr_comment:   true
  storage.branch:        quarantine/state
  exclude:               (none)
  rerun_command:          (auto-detected for jest)

Warnings:
  (none)
```

**On failure (exit 2):**
```
quarantine.yml has errors.

Errors:
  - Missing required field 'framework' in quarantine.yml.
  - Invalid retries value: 15. Must be between 1 and 10.

Warnings:
  - GitHub token not found in environment. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.
  - Unknown field 'timeout' in quarantine.yml. This field will be ignored.
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config PATH` | string | `quarantine.yml` | Path to the configuration file. |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0    | Configuration is valid (warnings may still be present) |
| 2    | Configuration has errors |

### Example

```bash
$ quarantine doctor
quarantine.yml is valid.

Resolved configuration:
  version:               1
  framework:             rspec
  retries:               3
  ...
```

---

## `quarantine version`

Print the CLI version and exit.

### Output

```
quarantine v{version}
```

Printed to stdout. The format is the literal string `quarantine v` followed by
the semantic version (e.g., `quarantine v0.1.0`).

### Flags

None.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0    | Always |

### Example

```bash
$ quarantine version
quarantine v0.1.0
```

---

## Exit Code Summary

| Code | Meaning | Commands |
|------|---------|----------|
| 0 | Success | All commands |
| 1 | Real test failures | `run` only |
| 2 | Quarantine error | All commands |

Exit code 1 exclusively means "your tests failed." It is never used for
quarantine infrastructure errors, invalid configuration, or usage errors.

Exit code 2 means a quarantine-related error: not initialized, bad flags,
invalid config, or (with `--strict`) infrastructure failure. Cobra's default
behavior of exit 2 for usage errors is preserved via `cmd.SetFlagErrorFunc()`.

---

## Output Format

### General Principles

- **Human-readable by default.** No `--json` flag in v1 (deferred to v2).
- **Warnings to stderr,** prefixed with `[quarantine] WARNING:`.
- **Errors to stderr,** prefixed with `[quarantine] ERROR:`.
- **Informational output to stdout.**
- **GitHub Actions annotations** when `GITHUB_ACTIONS` env var is set:
  `::warning::` annotations appear as yellow warning banners on the workflow
  run summary page. Emitted in addition to stderr warnings, not instead of.

### `--verbose` Output

Adds the following to the default output:

- API call details: endpoint, method, response status, response time.
- Retry attempts: test name, attempt number, result, duration.
- Config resolution: which values came from config, flags, or defaults.
- Quarantine list: full list of quarantined test IDs at the start of the run.
- Timing: total wall-clock time for the run, time per phase.

### `--quiet` Output

Suppresses:

- Success messages ("All tests passed", summary tables).
- Informational messages ("Reading quarantine state...", "Posting PR comment...").

Preserves:

- All warnings (`[quarantine] WARNING:` lines).
- All errors (`[quarantine] ERROR:` lines).
- The test runner's own stdout/stderr (passed through unmodified).

### `--verbose` and `--quiet` Interaction

These flags are mutually exclusive. If both are set, the CLI prints an error
and exits 2:

```
Error: --verbose and --quiet are mutually exclusive.
```

### Standard Run Output (Default)

```
[quarantine] Reading quarantine state... OK (3 tests quarantined)
[quarantine] Checking issue status... OK (1 unquarantined)
[quarantine] Excluding 2 quarantined tests from execution.
[quarantine] Running: jest --ci --reporters=default --reporters=jest-junit --testPathIgnorePatterns="..." ...

  <test runner output appears here, unmodified>

[quarantine] Parsing JUnit XML: junit.xml (150 tests, 2 failures)
[quarantine] Retrying: UserService > should handle timeout (attempt 1/3)... PASSED (flaky)
[quarantine] Retrying: PaymentService > should process refund (attempt 1/3)... FAILED
[quarantine] Retrying: PaymentService > should process refund (attempt 2/3)... FAILED
[quarantine] Retrying: PaymentService > should process refund (attempt 3/3)... FAILED

[quarantine] Results:
  Total:           150
  Passed:          147
  Failed:          1 (genuine)
  Flaky detected:  1 (quarantined)
  Quarantined:     2 (excluded from execution)
  Unquarantined:   1

[quarantine] Created issue #43: Flaky test: UserService > should handle timeout
[quarantine] Updated quarantine state (3 tests quarantined).
[quarantine] Posted PR comment on #99.
[quarantine] Results written to .quarantine/results.json
```

### Degraded Mode Output

When infrastructure errors occur but the CLI continues in degraded mode:

```
[quarantine] WARNING: Could not read quarantine state from GitHub (HTTP 503). Trying Actions cache...
[quarantine] WARNING: Actions cache miss. Running without quarantine state (all tests will execute).
[quarantine] Running: jest --ci ...

  <test runner output>

[quarantine] WARNING: Could not update quarantine state (HTTP 503). Changes will be retried on next run.
[quarantine] WARNING: Running in degraded mode. 2 infrastructure errors occurred.
```

When `GITHUB_ACTIONS` is set, the same warnings also emit GHA annotations:

```
::warning::Could not read quarantine state from GitHub (HTTP 503). Running in degraded mode.
```

---

## `test_id` Format

The `test_id` is a human-readable composite string constructed from JUnit XML
attributes:

```
file_path::classname::name
```

The `::` delimiter was chosen because it does not appear in JUnit XML output
from any v1-supported framework.

### Components

| Component    | Source                                         |
|--------------|------------------------------------------------|
| `file_path`  | See framework-specific extraction below        |
| `classname`  | `classname` attribute of `<testcase>`          |
| `name`       | `name` attribute of `<testcase>`               |

### Framework-Specific `file_path` Extraction

| Framework | `file_path` source | Notes |
|-----------|--------------------|-------|
| `jest`    | `<testsuite>` context or `file` attribute if `addFileAttribute=true` in jest-junit config | Recommend configuring `addFileAttribute: "true"` for reliable file path extraction. |
| `rspec`   | `file` attribute on `<testcase>` (emitted by rspec_junit_formatter) | Always present. Example: `./spec/models/user_spec.rb` |
| `vitest`  | `<testsuite name>` attribute (equals the relative file path) | Always present. Example: `src/utils/__tests__/math.test.ts` |

### Examples

| Framework | `test_id` |
|-----------|-----------|
| Jest      | `__tests__/addition.test.js::addition positive numbers::should add up` |
| RSpec     | `./spec/models/user_spec.rb::spec.models.user_spec::User#valid? returns true for valid attributes` |
| Vitest    | `src/utils/__tests__/math.test.ts::src/utils/__tests__/math.test.ts::math > add > should add positive numbers` |

### Storage

The raw `test_id` string is stored in `quarantine.json` as the key for each
quarantined test entry, and in test result artifacts alongside the individual
components (`file_path`, `classname`, `name`).

The dashboard stores a SHA-256 hash of `test_id` internally for efficient
indexing. The hash is not part of the cross-system contract.

---

## Framework-Specific Exclusion Flags

When quarantined tests exist, the CLI augments the user's test command to
exclude them before execution. Quarantined tests never run and produce no JUnit
XML entries.

### Jest

Jest does not have a single flag to exclude tests by name with exact matching.
The CLI uses a combination approach:

- **`--testPathIgnorePatterns`**: Exclude entire test files if all tests in the
  file are quarantined. Accepts a regex pattern.
  ```
  jest --ci --testPathIgnorePatterns="__tests__/flaky.test.js"
  ```

- **`--testNamePattern`** (inverted via regex): When only some tests in a file
  are quarantined, use a negative lookahead regex to skip specific test names.
  ```
  jest --ci --testNamePattern="^(?!.*should handle timeout).*$"
  ```

- **Combined**: When quarantined tests span multiple files and partial files:
  ```
  jest --ci --testPathIgnorePatterns="__tests__/all-flaky.test.js" --testNamePattern="^(?!.*(should handle timeout|should retry on error)).*$"
  ```

**Caveat:** `--testNamePattern` uses regex matching against the full test name
(all describe blocks + test name). Special regex characters in test names must
be escaped. The CLI handles this automatically.

### RSpec

RSpec supports tag-based exclusion and pattern-based filtering:

- **Tag exclusion** (preferred): Requires adding a `quarantine` tag to tests,
  which is a code modification approach not used in v1.

- **Pattern exclusion via `-e` negation**: RSpec does not natively support
  negative `-e` patterns. Instead, the CLI constructs a list of spec files and
  descriptions to run, excluding quarantined ones.

- **File-level exclusion**: If all tests in a spec file are quarantined, exclude
  the file entirely by omitting it from the file list passed to rspec.
  ```
  rspec spec/models/user_spec.rb spec/services/payment_spec.rb
  ```
  (omitting `spec/services/flaky_spec.rb` which has all tests quarantined)

- **Individual test exclusion**: When only some tests in a file are quarantined,
  pass specific line numbers or use `--example` with a negative pattern if
  supported by the RSpec version. As a practical approach, the CLI runs the
  full file and relies on post-execution filtering for partial-file
  quarantined tests.

**Implementation note:** RSpec exclusion is the most complex of the three
frameworks. The implementation should start with file-level exclusion (simpler)
and add individual test exclusion in a follow-up.

### Vitest

Vitest has built-in exclusion support:

- **`--exclude`**: Exclude test files by glob pattern.
  ```
  vitest run --exclude="src/utils/__tests__/flaky.test.ts"
  ```

- **Name filtering via `-t`** (inverted): Similar to Jest, use a regex pattern
  with negative lookahead.
  ```
  vitest run -t "^(?!.*should handle timeout).*$"
  ```

- **Combined**:
  ```
  vitest run --exclude="src/flaky.test.ts" -t "^(?!.*should handle timeout).*$"
  ```

---

## Framework-Specific Rerun Commands

When a test fails, the CLI re-runs it individually to determine if it is flaky.
The rerun uses framework-specific commands.

### Auto-Detected Rerun Commands

| Framework | Rerun command template                                    |
|-----------|-----------------------------------------------------------|
| `jest`    | `jest --testNamePattern "{name}"`                         |
| `rspec`   | `rspec -e "{name}"`                                      |
| `vitest`  | `vitest run --reporter=junit {file} -t "{name}"`         |

### Placeholder Variables

| Placeholder   | Description                                    |
|---------------|------------------------------------------------|
| `{name}`      | Test name from JUnit XML `name` attribute      |
| `{classname}` | Class name from JUnit XML `classname` attribute|
| `{file}`      | File path extracted from the test identifier   |

### Override

The `rerun_command` field in `quarantine.yml` overrides the auto-detected
command. See [config-schema.md](config-schema.md#rerun_command).

### Framework Caveats

- **Jest:** Requires `jest-junit` for JUnit XML output. The rerun command
  does not include `--reporters` flags -- it runs the test for pass/fail
  determination only, not XML output.
- **RSpec:** `rspec -e "{name}"` matches against `full_description`, which may
  match multiple tests with similar names. This is a known limitation;
  rspec_junit_formatter does not include line numbers in its XML output.
- **Vitest:** Built-in JUnit XML support via `--reporter=junit`. The rerun
  command includes the reporter flag so the CLI can parse rerun results.

---

## Exclude Pattern Matching

Exclude patterns (from `quarantine.yml` `exclude` field and `--exclude` flags)
are matched against the `test_id`, which has the format:

```
file_path::classname::name
```

Standard glob syntax is supported:

| Pattern | Meaning |
|---------|---------|
| `*`     | Match any characters within a single segment |
| `**`    | Match across path separators (for the `file_path` portion) |
| `?`     | Match a single character |

### Examples

```yaml
exclude:
  - "test/integration/**"           # Exclude all integration tests by file path
  - "**::SlowServiceTest::*"        # Exclude all tests in a specific class
  - "**::*timeout*"                  # Exclude tests with "timeout" in the name
```

### Merge Behavior

`--exclude` flags are merged with `exclude` patterns from config. The combined
set of patterns is used for matching. There is no deduplication -- overlapping
patterns are harmless.

---

## PR Comment Template

The CLI posts or updates a single comment per PR. The comment is identified by
a hidden HTML marker so the CLI updates the existing comment instead of creating
duplicates.

### Template

```markdown
<!-- quarantine-bot -->
## Quarantine Summary

| Metric | Count |
|--------|-------|
| Tests run | {total} |
| Passed | {passed} |
| Failed (genuine) | {failed} |
| Flaky (newly detected) | {flaky_detected} |
| Quarantined (excluded) | {quarantined_excluded} |
| Unquarantined | {unquarantined} |

{flaky_section}

{quarantined_section}

{unquarantined_section}

{failures_section}

---
<sub>Posted by [quarantine](https://github.com/mycargus/quarantine) v{version} | {result_emoji} Build {result}</sub>
```

### Conditional Sections

Each section is included only when relevant (non-zero count).

**`{flaky_section}` -- New flaky tests detected:**

```markdown
### New Flaky Tests Detected

| Test | Issue |
|------|-------|
| `UserService > should handle timeout` | [#43](https://github.com/org/repo/issues/43) |
| `PaymentService > should validate card` | [#44](https://github.com/org/repo/issues/44) |

These tests failed initially but passed on retry. They have been quarantined
and will be excluded from future runs until their issues are resolved.
```

**`{quarantined_section}` -- Already-quarantined tests:**

```markdown
### Quarantined Tests (Excluded)

{count} quarantined tests were excluded from this run:

<details>
<summary>View excluded tests</summary>

| Test | Issue | Quarantined since |
|------|-------|-------------------|
| `DatabaseTest > should handle connection loss` | [#38](https://github.com/org/repo/issues/38) | 2026-03-10 |
| `CacheTest > should expire entries` | [#39](https://github.com/org/repo/issues/39) | 2026-03-12 |

</details>
```

**`{unquarantined_section}` -- Unquarantined tests (issues closed):**

```markdown
### Unquarantined Tests

The following tests have been unquarantined because their tracking issues were closed:

| Test | Issue |
|------|-------|
| `AuthService > should refresh token` | [#35](https://github.com/org/repo/issues/35) (closed) |

These tests are now running normally again. If they fail, they will be treated
as genuine failures.
```

**`{failures_section}` -- Real failures:**

```markdown
### Real Failures

The following tests failed and are NOT flaky (failed all {retries} retries):

| Test | Failure message |
|------|-----------------|
| `PaymentService > should process refund` | `AssertionError: expected 200, got 500` |
```

### Comment Identification

The hidden HTML marker `<!-- quarantine-bot -->` must be the first line of the
comment body. The CLI searches existing PR comments for this marker to find
the comment to update. If found, the comment is updated via PATCH. If not
found, a new comment is created via POST.

### Result Indicators

| Build result | `{result_emoji}` | `{result}` |
|-------------|-------------------|------------|
| All passed  | (checkmark)       | `passed`   |
| Flaky only  | (warning)         | `passed (flaky tests detected)` |
| Failures    | (cross)           | `failed`   |
| Degraded    | (warning)         | `passed (degraded mode)` |

---

## Config Resolution Order

When a value can come from multiple sources, the following precedence applies
(highest to lowest):

1. **CLI flags** (`--retries`, `--junitxml`, `--config`, etc.)
2. **`quarantine.yml`** values
3. **Auto-detected values** (`github.owner`, `github.repo` from git remote)
4. **Built-in defaults** (`retries: 3`, framework-specific `junitxml`, etc.)

---

## Global Behavior

### Cobra CLI Framework

The CLI uses the `cobra` Go library for command-line parsing. Specific
configuration:

- Override cobra's default exit-2 for usage errors via `cmd.SetFlagErrorFunc()`
  to ensure exit code 2 is used consistently for all quarantine errors.
- Unknown flags produce a usage error (exit 2).
- Missing required arguments produce a usage error (exit 2).

### Signal Handling

- The CLI forwards `SIGINT` and `SIGTERM` to the child test process.
- If the child process is killed, the CLI exits with exit code 2.
- On interrupt, the CLI does not attempt to update quarantine state or post
  PR comments.

### Filesystem Output

The CLI writes results to `.quarantine/results.json` in the current working
directory by default. This directory is created if it does not exist. The path
is configurable via the `--output` flag:

```
quarantine run --output path/to/results.json -- jest --ci
```

The results file follows the test result JSON schema defined in
`schemas/test-result.schema.json`.

The CI workflow is responsible for uploading this file as a GitHub Artifact
using `actions/upload-artifact`. The CLI does not interact with the Artifacts
REST API. Artifact naming conventions and dashboard discovery patterns are
defined during M6 (dashboard implementation).

---

*References: [config-schema.md](config-schema.md), [architecture.md](../planning/architecture.md),
[ADR-003](../adr/003-quarantine-mechanism.md), [ADR-010](../adr/010-config-format.md),
[ADR-016](../adr/016-v1-framework-scope.md).*
