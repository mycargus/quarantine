# Plan: Multi-Suite Support

## Problem Statement

Quarantine v1 assumes one test framework per repository. Real-world repositories
often have multiple test suites (e.g., RSpec for backend, Jest for frontend,
separate unit and integration suites for the same framework). The current design
has no story for these repositories.

Additionally, the `framework` field is a leaky abstraction. It bundles
framework-specific defaults (junitxml path, rerun command, exclusion flags) behind
a single string, but users with custom setups (pnpm, bun, monorepos, custom
wrappers) override every value it provides. Quarantine does not actually need to
know the framework --- it needs concrete values: the test command, JUnit XML path,
and rerun command.

## Decisions

These decisions were reached through iterative design review. Each is documented
with its rationale.

### D1: `framework` removed from config; `junitxml` and `rerun_command` always required

**Rationale:** The top-level `framework` field bundles three concerns: a default
junitxml path, a default rerun command, and a default exclusion mechanism. As a
top-level field it is a leaky abstraction --- users with custom setups override
every value it provides, and it gates functionality on a supported-frameworks
list.

ADR-030 decided to remove `framework` from the configuration entirely. This plan
honors that decision.

**Decision:** Remove the `framework` field from the config schema. `junitxml` and
`rerun_command` are always required per-suite fields (alongside `command`). There
are no conditional requirements and no runtime defaults resolution.

`quarantine init` detects test frameworks (jest/vitest from `package.json`, rspec
from `Gemfile`) and pre-fills `junitxml` and `rerun_command` with sane defaults
in the generated config. The user edits the YAML to match their setup. The
defaults exist at generation time, not runtime.

- **Always explicit.** Every suite has `command`, `junitxml`, and
  `rerun_command` in the config. No hidden defaults, no conditional logic.
- **`quarantine init` does the work.** Detection fills in sensible values so the
  user starts from a working config, not a blank template.
- **One validation path.** The config parser checks that all three fields are
  present. Period.

**Init defaults (used by `quarantine init` to pre-fill generated config):**

| Detected runner | Pre-filled `junitxml` | Pre-filled `rerun_command` |
|----------------|----------------------|---------------------------|
| jest | `junit.xml` | `["npx", "jest", "--testNamePattern", "{name}"]` |
| rspec | `rspec.xml` | `["bundle", "exec", "rspec", "-e", "{name}"]` |
| vitest | `junit-report.xml` | `["npx", "vitest", "run", "--reporter=junit", "{file}", "-t", "{name}"]` |

**Impact:** ADR-016 (v1 framework scope) is superseded by ADR-030. The CLI no
longer gates functionality on a supported-frameworks list. ADR-030 is honored
as-is --- no amendment needed.

### D2: Single config file with a `test_suites` array

**Rationale:** Compared to one config file per framework:
- Eliminates silent under-coverage (the #1 product risk with multi-file)
- Shared settings are written once, no drift
- One file to find, read, and version-control
- `quarantine doctor` validates everything in one pass

The implementation cost of array parsing and per-suite validation is acceptable.

**Config location:** `.quarantine/config.yml`, always at the repository root.
Quarantine discovers the repo root via git (same mechanism used for git remote
detection). If run from a subdirectory, quarantine walks up to find the repo root.
All quarantine-generated files live in the `.quarantine/` directory --- config,
runtime files, per-suite output. One location, always.

**Example:**

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

  - name: frontend
    command: ["npx", "jest", "--ci"]
    junitxml: "junit.xml"
    rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
    retries: 3

  - name: e2e
    command: ["npx", "playwright", "test"]
    junitxml: "test-results/junit.xml"
    rerun_command: ["npx", "playwright", "test", "{file}"]
    retries: 2
```

**Validation:** An empty `test_suites` array is NOT valid. `quarantine doctor`
reports an error. `quarantine run` errors with:
`"No test suites configured. Edit .quarantine/config.yml to add one."`

### D3: Quarantine wraps the user's existing test command

**Principle:** The `command` field is the user's existing test command as a YAML
array (e.g., `["npx", "jest", "--ci"]`, `["bundle", "exec", "rspec"]`).
Quarantine runs it via `exec.Command(command[0], command[1:]...)`, never modifies
it, never appends flags. Quarantine's only requirement from the test runner is
that the command produces JUnit XML as a side effect (configured in the test
runner's own setup --- jest.config.js, .rspec, vitest.config.ts). This is a
test-runner configuration concern, not a quarantine config concern.

**Rationale:** Users already have a way to run their tests. If quarantine's config
specifies a different command, there are two ways to run tests and they can drift.
By wrapping the existing command, quarantine adopts the user's workflow rather than
replacing it. Appending flags to wrapper commands (`npm test`, `rake spec`) is
fragile --- npm requires `--` to forward flags, rake ignores unknown flags, etc.
Too many things can go wrong.

**Why array format, not string:** A string `command` field requires `sh -c` to
execute, which introduces shell interpretation, process group management
(`Setpgid`, `syscall.Kill(-pgid, ...)`), and platform-dependent behavior (no
Windows, shell metacharacter surprises). The array format uses `exec.Command`
directly --- the same mechanism already used for `rerun_command` --- eliminating
the shell layer entirely. This matches Docker Compose's `command` field
convention. Users who need shell features explicitly write
`command: ["sh", "-c", "npm test && echo done"]`.

CI integration:

```yaml
- run: quarantine run backend
- run: quarantine run frontend
```

**Single-suite convenience:** When exactly one suite is configured,
`quarantine run` (no argument) runs it. When multiple suites are configured and
no name is provided, quarantine errors with a list of available suite names.

**Command execution:** The suite's `command` is executed via
`exec.Command(command[0], command[1:]...)`. No shell is involved. This is the
same execution model used for `rerun_command`, giving both commands identical
signal handling, timeout enforcement, and security properties.

**Rerun command execution:** The suite's `rerun_command` uses the same
`exec.Command` approach as `command`. Placeholder values (`{name}`, `{file}`,
`{classname}`) are substituted within array elements before execution. Because
`exec.Command` is used (no shell), shell metacharacters in test names from JUnit
XML are harmless --- quotes, semicolons, backticks are passed as literal
characters to the test runner.

### D4: `quarantine init` creates config with pre-filled suites; user edits YAML

**Rationale:** A suite definition is ~5 lines of YAML. A CLI command that
accepts the same information via flags (`--name`, `--command`,
`--rerun-command` as comma-separated arrays) is harder to write correctly than
the YAML itself. Comma-separated arrays are ambiguous (what if an argument
contains a comma?), the escaping rules are unspecified, and the rerun-command
template requires framework-specific knowledge that the user must already have.

Instead: `quarantine init` detects test frameworks and creates
`.quarantine/config.yml` with pre-filled suite entries including `junitxml` and
`rerun_command` defaults for each detected framework. The user reviews and edits
the YAML. `quarantine doctor` validates it. This is the product's "zero
friction" path --- the config file IS the interface.

**`quarantine init`:**
- Detects test frameworks (jest/vitest from `package.json`, rspec from `Gemfile`)
- Creates `.quarantine/config.yml` with shared settings and a pre-filled suite
  entry for each detected framework. Each entry includes `name`, `command`,
  `junitxml`, and `rerun_command` with sane defaults from the init defaults
  table (D1). When no framework is detected, a commented example suite is
  written instead.
- Creates `.quarantine/.gitignore`
- Creates the `quarantine/state` branch with an initial commit containing a
  `README.md` (see [State Branch Initial Commit](#state-branch-initial-commit))
- Validates GitHub token
- Prints detection results and guides user to next step:
  ```
  Quarantine initialized.

  Detected test frameworks: jest, rspec
  Pre-filled 2 suite entries in .quarantine/config.yml

  Next step: review .quarantine/config.yml, adjust suite names and commands,
  then run `quarantine doctor` to validate.
  ```
- Init creates pre-filled suite entries for detected frameworks. The user
  adjusts names, commands, and values to match their actual setup.

**`quarantine suite add`:** Deferred to a future version. A suite definition is
~5 lines of YAML (`name`, `command`, `junitxml`, `rerun_command`, `retries`)
--- editing YAML is simpler than constructing a flag-driven CLI command.
Interactive `suite add` with framework detection, prompts, and validation
guidance may be added later when the UX requirements are clearer.

**`quarantine suite list`:**
- Prints configured suites with their key settings (name, command, junitxml,
  rerun_command)

**`quarantine suite remove <name>`:**
- Removes a suite entry from config. Asks for confirmation.
- Does NOT delete the state file on the state branch (preserves history)
- Does NOT delete issues associated with the suite

### D5: Exclusion deferred to v2

**Status:** Deferred. Full exclusion (skipping quarantined tests to save CI time)
is not included in v1. However, v1 provides a composable building block and
visibility into the CI time cost.

**Rationale for deferral:** No test runner (Jest, Vitest, RSpec) natively supports
reading an exclusion file. Implementing exclusion requires custom per-framework
adapters (e.g., a Jest `globalSetup` that reads a file and sets
`testPathIgnorePatterns`, or an RSpec `spec_helper.rb` with metadata-based
filtering). Additionally, file-path-level exclusion is too coarse --- a file with
50 tests and 1 quarantined would skip all 50. Both problems need design work that
is out of scope for v1.

**v1 behavior:** Quarantined tests run normally. Their failures are recognized via
the state file and ignored in the exit code (D18). The build passes. Quarantined
test failures appear in test runner output and consume CI time, but correctness is
not affected.

**v1 mitigation — quarantined files list:** Before executing the suite's
`command`, quarantine writes `.quarantine/<suite>/quarantined-files.txt` --- a
newline-delimited list of quarantined test file paths (deduplicated). This is a
side-effect produced from data already in memory (the state file), not an
integration point. Users who want coarse file-level exclusion can reference it in
their command:

```yaml
# User opts into exclusion by invoking a shell explicitly:
command: ["sh", "-c", "jest --ci --testPathIgnorePatterns $(cat .quarantine/jest/quarantined-files.txt | tr '\\n' '|')"]
```

This is explicitly **user-driven**, not quarantine-managed. Quarantine provides
the data; the user decides whether and how to use it. The trade-off is documented:
file-level exclusion is coarse --- a file with 50 tests and 1 quarantined skips
all 50.

**v1 mitigation — `quarantine status`:** A new command (see
[CLI Changes](#new-commands)) that shows quarantined test counts, estimated CI
time cost, and oldest quarantined tests. This makes the scaling cost visible and
actionable so teams can triage old quarantined tests before the waste becomes
painful.

**v2 scope:** Per-framework exclusion adapters with test-level granularity. These
are **optional accelerators** layered on top of the file-level list --- they add
precision for teams that need it, but the file-level mechanism works for any
framework.

### D6: Separate state file per suite on the state branch

**Rationale:** Three approaches were considered:
1. **Separate files per suite** --- simplest, zero coupling between suites, no CAS
   contention on parallel runs.
2. **Suite name prefix in test ID** --- couples state identity to config naming,
   renaming a suite orphans quarantined tests.
3. **Separate sections in one file** --- CAS contention when parallel suite runs
   update the same file.

Option 1 is the simplest and most reliable. Each `quarantine run <suite>` reads
and writes only its own state file. Suites are fully independent at the state
level.

**State file path:** `.quarantine/<suite-name>/state.json` on the
`quarantine/state` branch. Consistent with the per-suite directory convention.

**State file lifecycle:** Created on the first `quarantine run` for that suite,
not during `init`. If the state file does not exist, quarantine creates it with
an empty test map.

#### State branch initial commit

`quarantine init` creates the `quarantine/state` branch with an initial commit
containing a `README.md`:

```markdown
# Quarantine State

This branch stores quarantine state files. Do not merge this branch.
Managed automatically by the quarantine CLI.
```

This gives the Contents API a commit to work with (an empty branch with no
commits cannot have files written to it) and is human-friendly for anyone who
discovers the branch.

**Test ID uniqueness:** We do not assume test IDs are globally unique across
suites. Scoping state files by suite name eliminates the need for this assumption.

**Dashboard API call impact:** The dashboard reads the state branch via the GitHub
Contents API. With multi-suite, it lists the `.quarantine/` directory (1 call)
then reads each suite's state file (N calls). For a repo with 5 suites on a
5-minute poll, this is ~72 calls/hr --- well within PAT rate limits (5,000/hr)
and `GITHUB_TOKEN` limits (1,000/hr) at v1 scale.

**v2 optimization:** A scheduled GitHub Actions workflow runs
`quarantine state consolidate`, which reads all per-suite state files and writes
a single `state.json` on the state branch. See
[v2: Dashboard State Consolidation](#v2-dashboard-state-consolidation). This
requires no webhook infrastructure — just a CLI command and a workflow file.
All webhooks remain deferred to v3 per ADR-027.

### D7: One PR comment per suite

**Rationale:** A combined comment across suites would require coordination between
independent CI steps (read-modify-write with race conditions, or a separate
aggregation job). Each `quarantine run <suite>` posts/updates its own comment,
identified by `<!-- quarantine:<suite-name> -->`. Independent, reliable, no
coordination required.

**v2 option:** A post-workflow aggregation step could combine suite comments into
a single PR comment after all suite runs complete.

### D8: One issue per flaky test, dedup scoped by suite

**Rationale:** An issue tracks a flaky test, not a suite. The issue body notes
which suite detected the flakiness.

**Dedup label format:** `quarantine:<suite-name>:<test_hash>`, where `test_hash`
is the first 8 hex characters of `SHA-256(test_id)` --- the same hash function
already used by the CLI for issue dedup (`testHash` in `run_notifications.go`).
This is unique per suite per test, human-readable, and deterministic. Example:
`quarantine:backend:a1b2c3d4`. With 8 hex chars (~4 billion values), collision
probability is negligible at v1 scale (50% collision requires ~77,000 tests per
suite via the birthday bound).

**Label length constraint:** GitHub labels have a 50-character maximum. The label
format is `quarantine:` (11 chars) + suite name + `:` (1 char) + hash (8 chars)
= 20 + suite name length. Suite names are constrained to 30 characters maximum
(see [D14](#d14-suite-name-constraints)), keeping the total under 50.

**Known edge case --- overlapping suites:** If the same test file appears in two
suites (due to overlapping `command` configurations), the same flaky test creates
separate GitHub Issues for each suite because dedup labels are scoped by suite
name. This is expected: each suite independently tracks and manages its own
quarantine state. Users who configure overlapping suites accept this tradeoff.
No plans to address this in v1.

### D9: Quarantine must own retries

**Rationale:** Research confirms that Jest, RSpec, and Vitest all report only the
final test result in JUnit XML. If framework-level retries are enabled and a flaky
test passes on retry, the XML shows a pass --- the failure is invisible.
Quarantine cannot detect flakiness from the XML alone.

Additionally, RSpec's community retry gem (rspec-retry) was archived in July 2025.
There is no maintained retry solution for RSpec.

Quarantine's flow must be:
1. Run the test suite (via the suite's `command`)
2. Parse JUnit XML for failures
3. Rerun individual failures using the suite's `rerun_command`
4. Classify: passes on any retry = flaky, fails all retries = genuine failure

**Framework retries silently defeat flakiness detection.** Documentation alone is
insufficient --- users commonly configure retries without realizing the
implication.

#### Enforcement: `quarantine doctor` only

Retry detection is best-effort heuristic matching that produces both false
positives and false negatives (dynamic config, per-test calls, unknown runners).
Therefore it belongs in the diagnostic command (`quarantine doctor`), not in the
runtime path (`quarantine run`). Doctor is where users go to check their setup;
adding heuristic warnings to every CI invocation trains users to ignore
quarantine output.

`quarantine doctor` checks the following locations and emits **warnings** (never
errors):

| Runner | Check | Location |
|--------|-------|----------|
| Jest | `retryTimes` in config | `jest.config.{js,ts,mjs,cjs}`, `package.json` `jest` key |
| Vitest | `retry` in config | `vitest.config.{js,ts,mjs,cjs}` |
| RSpec | `rspec-retry` gem | `Gemfile`, `Gemfile.lock` |

```
Warning: jest.config.js contains 'retryTimes'. Framework-level retries hide
failures from JUnit XML, preventing quarantine from detecting flaky tests.
Remove retryTimes before using quarantine. See: quarantine.dev/guides/disable-retries
```

**Static detection refinement:** To reduce false positives, static detection
uses patterns that distinguish active retries from no-ops:
- **Jest:** Match `retryTimes` but NOT `retryTimes(0)` or `retryTimes( 0 )`.
  Pattern: `/retryTimes\(\s*[1-9]/`. The string `retryTimes(0)` is a no-op
  and should not trigger a warning.
- **Vitest:** Match `retry:` with a non-zero value. Pattern: `/retry:\s*[1-9]/`.
- **RSpec:** Match `rspec-retry` in Gemfile (presence of the gem is sufficient).

**No `retry_detection` config field.** The per-suite `retry_detection: false`
override is removed. Users who see persistent false positives from `doctor`
can ignore the warning --- it does not block anything, and it only appears
when the user explicitly runs `doctor`.

#### Detection scope and limitations

Static detection is best-effort for known runners. It checks config files at the
repo root and common locations. It does NOT:
- Parse JavaScript/TypeScript ASTs (too complex; simple string/regex matching)
- Detect per-test `jest.retryTimes(N)` calls (would require AST parsing)
- Detect retry configuration in monorepo subdirectories
- Check for unknown test runners' retry mechanisms

For unknown runners, quarantine has no static retry detection. Documentation
is the only mitigation.

### D10: `quarantine doctor` validates structure, not content

**Rationale:** Doctor validates schema correctness:
- Required fields are present with correct data types
- `test_suites` is a non-empty array
- Each suite has required fields (`name`, `command`, `junitxml`, `rerun_command`)
- `retries` is a positive integer in range 1--10
- Suite names are unique, match `[a-z0-9][a-z0-9-]*`, max 30 characters
- State branch exists
- GitHub token is present

Doctor is also the **sole checkpoint** for framework-level retry configuration
(D9) --- it checks for Jest `retryTimes`, Vitest `retry`, and RSpec
`rspec-retry` and reports warnings if detected. Retry detection does not run
during `quarantine run` or any other command.

Doctor does NOT validate:
- Whether a test command is executable
- Whether a junitxml glob will match files
- Whether a rerun command template is syntactically valid

### D11: CLI flags override config values

**Rationale:** Like Docker Compose, CLI flags override config file values. This
allows users to work around invalid config without editing the file, and supports
one-off customization:

```bash
quarantine run backend --retries 5 --junitxml "custom/*.xml"
```

Flag overrides apply to the named suite for that invocation only.

### D12: No `exclude` patterns field (test path ignore)

**Rationale:** The original config had an `exclude` field for test path patterns
quarantine should ignore entirely (no retry, no quarantine, no issue). With the
test suite model, the user controls exactly which tests run via the suite's
`command` field. If they don't want integration tests quarantined, they configure
the command to not run them. The `exclude` patterns field is redundant.

### D13: `.quarantine/` always at the repository root

**Rationale:** Quarantine discovers the repo root via git (same mechanism used for
remote detection). If run from a subdirectory, quarantine walks up to find the
repo root and looks for `.quarantine/config.yml` there. If `.quarantine/` is not
at the repo root, quarantine errors.

There is no `--config` flag. The existing `--config PATH` flag (documented in
cli-spec.md and config-schema.md) is removed. There is only ever one config file
per repository, and its location is a fixed convention. This simplifies
discovery, documentation, and tooling. All code, tests, and documentation
referencing `--config` must be updated.

### D14: Suite name constraints

Suite names are used as:
- Directory names on the state branch (`.quarantine/<name>/state.json`)
- Directory names in working tree (`.quarantine/<name>/results.json`)
- PR comment markers (`<!-- quarantine:<name> -->`)
- CLI arguments (`quarantine run <name>`)
- Issue dedup labels (`quarantine:<name>:<hash>`)

**Validation:** `[a-z0-9][a-z0-9-]*`, maximum 30 characters. Must not collide
with CLI subcommands or flags.

**Enforcement:** `quarantine doctor` flags invalid names as errors.
`quarantine run` errors and aborts if it encounters an invalid suite name (with
a message to fix the config).

**Why not truncate:** Truncation creates silent inconsistency between the config
suite name and the label/directory name. Two suites differing only after the 30th
character would collide. Debugging truncated labels is a nightmare. Strict
validation at every entry point prevents the problem entirely.

### D15: Distinguish test command crashes from test failures

`quarantine run` must distinguish between:
1. **Test failures** --- the test command ran successfully, some tests failed.
   JUnit XML was produced. Quarantine processes results normally.
2. **Command crashes** --- the test command exited non-zero for a non-test reason
   (syntax error, missing dependency, configuration issue). No JUnit XML was
   produced.

When the test command exits non-zero and no JUnit XML files match the `junitxml`
glob:

```
Error: test command exited with code 1 but no JUnit XML files found at 'junit.xml'.
This usually means the test runner crashed before producing results.
Check that:
  - Your test command ('npm test') runs successfully outside of quarantine
  - JUnit XML output is configured in your test runner
  - The junitxml path in .quarantine/config.yml matches where your runner writes XML
```

Exit code: 2 (quarantine error, not test failure).

### D16: Rerun command failure handling

When `rerun_command` fails for a specific test (wrong path, missing binary, bad
template expansion, crash), quarantine does NOT abort the entire run. It continues
processing other failed tests. For the specific test whose rerun crashed:

- **Classify as unresolved** --- not flaky (couldn't prove it), not genuine failure
  (couldn't complete retries).
- **Include rerun error details in `results.json`** with status `"unresolved"`,
  an `error` field containing the failure details, and a `rerun_exit_code` field
  containing the actual exit code from the rerun command (for debugging).
- **Report in the PR comment:** "1 test could not be retried --- rerun command
  failed: `<error>`"
- **Do NOT create a flaky-test issue** (the test is not confirmed flaky).
- **Exit code: 2** (infrastructure error). An unresolved test is an infrastructure
  problem (broken rerun command), not a confirmed test failure. Exit code 1 is
  reserved exclusively for "your tests failed" and must not be used when
  quarantine cannot determine whether the test actually fails. The actual rerun
  command's exit code is recorded in `results.json` for debugging.

**Exit code priority** when a run has mixed results:
- Any genuine test failure → exit 1 (test failures take priority)
- Else any unresolved test → exit 2 (infrastructure error)
- Else → exit 0

**Error prefixes for exit code 2 diagnostics:** Exit code 2 covers multiple
failure modes (config error, token error, command crash, rerun crash, timeout,
unresolved test). To make log-based debugging tractable, all exit-2 error
messages use a consistent parseable prefix:

```
Error [config]: test_suites[0].command must be a YAML array, not a string.
Error [rerun]: rerun command failed for 'validates email': exec: 'bundle' not found
Error [timeout]: test command timed out after 30m.
Error [crash]: test command exited with code 1 but no JUnit XML files found.
```

This costs almost nothing to implement and avoids the need for additional exit
codes (which would be a breaking change).

This gives the user visibility without quarantine either swallowing the error or
treating an infrastructure problem as a test failure.

### D17: `quarantine init` is idempotent

Running `quarantine init` on a repo that is already initialized must be safe and
non-destructive. Init checks each artifact independently:

- `.quarantine/config.yml` exists → skip (do NOT overwrite; preserves suites)
- `.quarantine/.gitignore` exists → skip
- `quarantine/state` branch exists → skip (do NOT recreate; preserves state)
- `.quarantine/config.yml` missing but state branch exists → recreate config
  (shared settings only, `test_suites: []`), warn user that suites need to be
  re-added
- State branch missing but config exists → recreate branch with README

Init prints what it did and what it skipped:

```
.quarantine/config.yml already exists — skipping.
.quarantine/.gitignore already exists — skipping.
quarantine/state branch already exists — skipping.
GitHub token validated.

Quarantine is already initialized. Edit .quarantine/config.yml to add test suites.
```

This allows users to recover from partial failures (e.g., accidentally deleted
state branch) without losing their config.

### D18: Quarantined test failure recognition (core logic)

Before executing the test command, quarantine checks issue status and updates the
state file (same logic as `cli-spec.md` steps 3--4):

0. **Batch check issue status:** Use the GitHub Search API to find all closed
   issues with the `quarantine` label for this repository. Compare against the
   suite's state file. For any quarantined test whose issue is now closed, remove
   it from the in-memory quarantine list (unquarantine). The removal is written
   back to the state file after the run completes.

After parsing JUnit XML, quarantine applies this logic to every failed test:

1. **Check if the test ID exists in the suite's state file** (quarantined).
2. **If quarantined:** The failure is expected. Ignore it in the exit code. Update
   `last_failure_at` in the state file. Do not retry.
3. **If NOT quarantined:** This is a new failure. Retry using `rerun_command` up
   to N times:
   - If any retry passes → classify as **flaky**. Add to state file. Create issue.
   - If all retries fail → classify as **genuine failure**. Counts toward exit
     code 1.
   - If rerun command crashes → classify as **unresolved** (D16).

Quarantined tests always run (exclusion is deferred to v2, see D5). When a
quarantined test fails, step 1 catches it and ignores the failure in the exit
code.

The exit code is determined by non-quarantined test results:
- All non-quarantined tests pass → exit 0
- Any non-quarantined test has a genuine failure → exit 1
- Any unresolved test (D16) and no genuine failures → exit 2
- Quarantine infrastructure error → exit 2

### D19: Zero-failure and all-pass run behavior

When all tests pass on the first run (no failures, nothing to retry), quarantine
still:
- Writes `.quarantine/<suite>/results.json` (records the run for dashboard history)
- Updates state file: refreshes `last_failure_at` for any quarantined tests that
  appeared in the XML and failed (none in this case, so no updates)
- Posts/updates PR comment (if enabled, showing "all tests passed")
- Exit code: 0

### D20: (Removed --- exclusion deferred to v2)

Exclusion warning heuristic removed. With exclusion deferred (D5), there is no
`excluded.txt` to warn about. Quarantined tests always run; their failures are
ignored in the exit code (D18).

### D21: Signal handling

Both `command` and `rerun_command` use `exec.Command` with explicit args (no
shell). Signal forwarding uses the existing `runner.Run()` approach: forward
SIGINT/SIGTERM to the child process directly (`cmd.Process.Signal(sig)`).

This is the same signal handling design from the CLI spec (M2), applied uniformly
to both command types. No process group management is needed because there is no
shell intermediary --- `exec.Command` spawns the test runner directly.

**Limitation:** If the test runner spawns child processes that outlive it (e.g.,
background workers), those children may survive quarantine's signal. This matches
the behavior of CI systems (GitHub Actions, Jenkins). No mitigation in v1.

**Platform:** Linux and macOS only (`linux/darwin x amd64/arm64` per the
architecture doc). Windows support is out of scope for v1.

### D22: `--dry-run` semantics

`quarantine run <suite> --dry-run` answers the question: "what WOULD quarantine
do if I ran this for real?" Dry-run does NOT execute the test command or rerun
commands. It analyzes existing JUnit XML output from a previous test run.

**Does execute:**
- Reads state file from state branch
- Reads JUnit XML from the suite's `junitxml` glob
- Prints analysis: which tests are quarantined, which tests failed, which
  failed tests would be retried in a real run

**Does NOT execute:**
- Does not run the test command (the user must run their test suite first)
- Does not retry failed tests
- Does not write state to the state branch
- Does not write `results.json`
- Does not post or update PR comments
- Does not create GitHub issues
- Does not upload artifacts

**When no JUnit XML is found:** If the `junitxml` glob matches no files,
quarantine prints a warning and exits 0:
```
Warning: no JUnit XML files found at 'junit.xml'.
Run your test suite first to produce JUnit XML output, then re-run with --dry-run.
```

**Limitation:** Because dry-run does not execute retries, it cannot classify
failed tests as flaky vs genuine. All non-quarantined failures are reported as
"would be retried." The real exit code depends on retry outcomes and cannot be
predicted.

**Use cases:**
1. **First-time setup validation.** User configured a suite and wants to verify
   quarantine finds the JUnit XML, recognizes the right tests, and identifies
   quarantined tests correctly --- without creating real issues or comments.
   The user runs their test suite normally first, then runs
   `quarantine run <suite> --dry-run` to inspect the results.
2. **Debugging.** User wants to inspect quarantine's view: which tests are
   quarantined, what the state file contains, which tests would be retried.

**Exit code in dry-run:** 0 always. Dry-run is informational; it does not signal
test failures.

### D23: Timeouts

A hanging test command or rerun blocks the CI pipeline indefinitely. Quarantine
sits between the CI job timeout and the test runner; without its own timeout, it
is the process left holding the bag when a test runner stalls.

**Per-suite config:**

```yaml
test_suites:
  - name: backend
    timeout: 30m           # Optional. Default: 30m. Applies to the suite command.
    rerun_timeout: 5m      # Optional. Default: 5m. Applies to each individual rerun.
```

**Timeout behavior (command):** When `timeout` elapses:
1. Send `SIGTERM` to the process (`cmd.Process.Signal(SIGTERM)`)
2. Wait 5 seconds for graceful shutdown
3. Send `SIGKILL` if still running (`cmd.Process.Kill()`)
4. If JUnit XML exists at timeout, process partial results (better than nothing).
   Partial results are treated as a normal run --- any failures found in the XML
   are processed, retried, etc.
5. If no JUnit XML exists, treat as command crash (D15)
6. Exit 2 with diagnostic:
   ```
   Error: test command timed out after 30m.
   Partial results processed: 80 passed, 3 failed (from junit.xml).
   ```

**Timeout behavior (rerun):** When `rerun_timeout` elapses for a specific test:
1. Kill the rerun process (`cmd.Process.Kill()`)
2. Classify the test as **unresolved** (D16) with error:
   `"rerun timed out after 5m"`
3. Continue processing other failed tests (do NOT abort the run)

**Duration format:** Go `time.ParseDuration` format: `30m`, `1h`, `90s`, etc.
`quarantine doctor` validates that values parse correctly.

**CLI flag overrides:** `--timeout` and `--rerun-timeout` override per-suite
config for that invocation (consistent with D11).

---

## Generated Files

All files quarantine generates, across the full lifecycle:

| File | Location | Created by | When | Source-controlled | Purpose |
|------|----------|-----------|------|-------------------|---------|
| `.quarantine/config.yml` | Working tree (repo root) | `quarantine init` (user edits to add suites) | Setup | Yes | Config: shared settings + test suites |
| `.quarantine/.gitignore` | Working tree (repo root) | `quarantine init` | Setup | Yes | Ignores runtime files |
| `.quarantine/<suite>/results.json` | Working tree (repo root), then uploaded as GitHub Artifact by CI | `quarantine run` | After test run | No | Full test results for dashboard ingestion |
| `.quarantine/<suite>/quarantined-files.txt` | Working tree (repo root) | `quarantine run` | Before test command | No | Newline-delimited quarantined file paths (D5 composable building block) |
| `README.md` | `quarantine/state` branch | `quarantine init` | Setup | N/A (separate branch) | Explains the branch's purpose |
| `.quarantine/<suite>/state.json` | `quarantine/state` branch (via Contents API) | `quarantine run` | After test run (created on first run) | N/A (separate branch) | Current quarantine state per suite |

JUnit XML files are produced by the test runner, not by quarantine. Quarantine
reads them but does not generate them.

### Source control

`.quarantine/config.yml` and `.quarantine/.gitignore` are source-controlled.
The state branch `README.md` explains the project's purpose; no separate
readme is needed in the working tree `.quarantine/` directory.

All other files in `.quarantine/` are runtime artifacts. `quarantine init` creates
`.quarantine/.gitignore`:

```gitignore
# Ignore all runtime files. Only config.yml is source-controlled.
*
!.gitignore
!config.yml
```

---

## Artifact Schema

Each `quarantine run <suite>` writes `.quarantine/<suite-name>/results.json`.
The CI workflow uploads this as a GitHub Artifact.

The results JSON retains all fields from the existing
`schemas/test-result.schema.json` with these necessary changes only:

1. **Add `suite_name`** (required string) --- dashboard associates results with
   suites.
2. **Remove `framework` from results** --- `suite_name` replaces its role in
   the results schema. The `framework` field is removed from config entirely
   (D1) and is not propagated to results.
3. **Add `"unresolved"` to test status enum** --- D16 introduces this
   classification.
4. **Add `error` field to test_entry** (optional string) --- D16 requires error
   details for unresolved tests.
5. **Add `rerun_exit_code` field to test_entry** (optional integer) --- D16
   records the actual exit code from the crashed rerun command for debugging.
6. **Remove `config.excluded_patterns` and `config.excluded_count`** --- the
   `--exclude` flag is removed (see D12).

All other fields (`repo`, `branch`, `commit_sha`, `cli_version`, `config`,
`summary`, `original_status`, `retries` array, `failure_message`,
`issue_skipped_reason`) are unchanged.

```json
{
  "version": 1,
  "suite_name": "backend",
  "run_id": "12345",
  "repo": "my-org/my-project",
  "branch": "main",
  "commit_sha": "abc123",
  "timestamp": "2026-04-10T12:00:00Z",
  "cli_version": "1.0.0",
  "config": {
    "retry_count": 3
  },
  "tests": [
    {
      "test_id": "spec/models/user_spec.rb::User::validates email",
      "file_path": "spec/models/user_spec.rb",
      "classname": "User",
      "name": "validates email",
      "status": "flaky",
      "original_status": "failed",
      "duration_ms": 150,
      "retries": [
        { "attempt": 1, "status": "failed", "duration_ms": 140 },
        { "attempt": 2, "status": "passed", "duration_ms": 150 }
      ]
    },
    {
      "test_id": "spec/models/order_spec.rb::Order::calculates total",
      "file_path": "spec/models/order_spec.rb",
      "classname": "Order",
      "name": "calculates total",
      "status": "unresolved",
      "original_status": "failed",
      "duration_ms": 45,
      "error": "rerun command failed: exec: 'bundle': executable file not found in $PATH",
      "rerun_exit_code": 127
    }
  ],
  "summary": {
    "total": 100,
    "passed": 95,
    "failed": 2,
    "skipped": 0,
    "flaky_detected": 3,
    "quarantined": 5
  }
}
```

**Test status values:**

| Status | Meaning |
|--------|---------|
| `passed` | Test passed on first run |
| `failed` | Test failed all retries (genuine failure) |
| `flaky` | Test failed initially but passed on a retry |
| `quarantined` | Test was already quarantined; failure was ignored |
| `unresolved` | Test failed but rerun command crashed (D16); includes `error` field |
| `skipped` | Test was marked as skipped by the test runner |

### Dashboard data sources

The dashboard reads from two sources:

1. **GitHub Artifacts** (results.json per suite) --- test run history, flaky
   detection trends, timing data. This is time-series data: "what happened in
   this run?" Artifacts expire (default 90 days).

2. **State branch** (`quarantine/state`) --- per-suite state files at
   `.quarantine/<suite>/state.json` tracking which tests are currently
   quarantined, their issue numbers, quarantine timestamps. This is the
   authoritative source for current quarantine state.

The dashboard does NOT read `.quarantine/config.yml`. It gets everything it needs
from artifacts (suite name, test results) and the state branch (quarantine state,
issue links).

**State branch enumeration:** The dashboard lists the `.quarantine/` directory on
the state branch, discovers suite subdirectories, and reads each `state.json`.
If `.quarantine/` does not exist yet (no suites have run), the dashboard handles
the 404 gracefully --- there is no state to display. Alternatively, the dashboard
reads the consolidated `state.json` if available (v2, see below).

### Schema transition

Nothing has shipped. There are no users and no production data. The CLI
replaces the old format with the new across two milestones (M9/M10). The
dashboard is implemented against the new schemas only:

- **Results artifact:** Requires `suite_name`. No `framework` field.
- **State branch:** Per-suite files at `.quarantine/<suite>/state.json`. No
  single `quarantine.json` at branch root.
- **Config:** `.quarantine/config.yml` with `test_suites` array. No
  `quarantine.yml`.

---

## Config Schema

### `.quarantine/config.yml`

```yaml
version: 1                           # Required. Integer. Must be 1.

github:                              # Optional. Auto-detected from git remote.
  owner: string                      #   GitHub org or user.
  repo: string                       #   Repository name.

issue_tracker: github                # Optional. Default: github. v1: only github.

labels:                              # Optional. Default: [quarantine].
  - quarantine                       # v1: only ["quarantine"] accepted.

notifications:                       # Optional.
  github_pr_comment: true            # Default: true. Boolean.

storage:                             # Optional.
  branch: quarantine/state           # Default: quarantine/state.

test_suites:                         # Required. Non-empty array.
  - name: string                     # Required. Unique. [a-z0-9][a-z0-9-]*, max 30.
    command: [string]                 # Required. YAML array. User's existing test
                                     #   command as explicit args. Executed via
                                     #   exec.Command(command[0], command[1:]...).
                                     #   Quarantine never modifies this command.
    junitxml: string                  # Required. Glob for JUnit XML output.
    rerun_command: [string]           # Required. YAML array. Template with
                                     #   {name}, {classname}, {file} placeholders.
                                     #   Executed via exec.Command (no shell).
    retries: integer                  # Optional. Default: 3. Range: 1-10.
    timeout: duration                 # Optional. Default: 30m. Go duration format.
                                     #   Applies to the suite's command.
    rerun_timeout: duration           # Optional. Default: 5m. Go duration format.
                                     #   Applies to each individual rerun.
```

### Per-suite state file on `quarantine/state` branch

Path: `.quarantine/<suite-name>/state.json`

The per-suite state file retains all fields from the existing
`quarantine-state.schema.json`. The `suite` field adopts a new semantic meaning:
it now refers to the quarantine suite name (matching the suite's `name` in
config) rather than the JUnit classname/describe block. `last_flaky_at` was
renamed to `last_failure_at` (implemented in M9) because D18 updates this
timestamp on every quarantined test failure, not only on new flaky detections.

```json
{
  "version": 1,
  "updated_at": "2026-04-10T12:00:00Z",
  "tests": {
    "file_path::classname::name": {
      "test_id": "file_path::classname::name",
      "file_path": "spec/models/user_spec.rb",
      "classname": "User",
      "name": "validates email format",
      "suite": "backend",
      "first_flaky_at": "2026-04-01T08:00:00Z",
      "last_failure_at": "2026-04-10T12:00:00Z",
      "flaky_count": 3,
      "quarantined_at": "2026-04-01T08:00:00Z",
      "quarantined_by": "cli-auto",
      "issue_number": 42,
      "issue_url": "https://github.com/my-org/my-project/issues/42"
    }
  }
}
```

---

## CLI Changes

### Modified commands

**`quarantine init`**
- Idempotent (D17): checks each artifact and creates only what is missing. Never
  overwrites existing config or recreates existing branch.
- Creates `.quarantine/` directory at the repo root (if missing)
- Creates `.quarantine/config.yml` with shared settings and pre-filled suite
  entries for detected frameworks (if missing)
- Creates `.quarantine/.gitignore` (if missing)
- Creates the `quarantine/state` branch with initial commit (`README.md`)
  (if missing)
- Validates GitHub token
- Detects test frameworks (package.json, Gemfile)
- Prints what was created, what was skipped, and guides user to edit config
- Does NOT create suites, does NOT prompt for suite configuration
- Exit codes: 0 = success, 2 = failure (token invalid, branch creation failed)

**`quarantine run [suite-name] [flags]`**

**Major execution model change:** This replaces the existing
`quarantine run [flags] -- <test command>` syntax and the entire execution flow
in `cli-spec.md` steps 1--14. The old model constructed and augmented the test
command (appending framework-specific exclusion flags). The new model wraps the
user's command unmodified --- quarantine never touches the command string. The
test command is no longer passed on the command line; it comes from the suite's
`command` field in config. The `--` separator is removed. All code, tests, and
documentation referencing the old syntax must be updated.

- Discovers repo root via git, reads `.quarantine/config.yml`
- Validates suite name (max 30 chars, pattern match); errors and aborts if invalid
- Looks up suite by name in config
- If no name provided and exactly one suite: runs it
- If no name provided and multiple suites: errors with available names
- If no suites configured: errors with
  `"No test suites configured. Edit .quarantine/config.yml to add one."`
- Reads suite's state file (`.quarantine/<suite>/state.json`) from state branch;
  creates with empty test map if it does not exist (first run)
- Batch checks issue status and removes unquarantined tests from in-memory
  state (D18 step 0)
- Executes the suite's `command` via `exec.Command(command[0], command[1:]...)`
  (never modified, never appended to). Signal forwarding per D21.
- If command exits non-zero and no JUnit XML found: errors with diagnostic (D15)
- Parses JUnit XML from the suite's `junitxml` glob
- For each failed test: checks if quarantined (D18). Quarantined failures are
  ignored. Non-quarantined failures are retried.
- If rerun command fails for a test: classifies as unresolved (D16), exit 2
- Writes updated state to `.quarantine/<suite>/state.json` on state branch (CAS)
- Writes `.quarantine/<suite>/results.json` (always, even on zero failures — D19)
- Posts/updates PR comment with `<!-- quarantine:<suite-name> -->` marker
- Creates issues for newly detected flaky tests (label:
  `quarantine:<suite-name>:<test_hash>`)
- Flag overrides: `--retries`, `--junitxml`, `--rerun-command`, `--timeout`,
  `--rerun-timeout`, `--verbose`, `--quiet`, `--dry-run`, `--strict`, `--pr`
  (semantics unchanged from `cli-spec.md` except for new timeout flags per D23)
- The `--output` flag is removed. Results are always written to
  `.quarantine/<suite>/results.json`.

**`quarantine doctor [flags]`**
- Discovers repo root via git, reads `.quarantine/config.yml`
- Validates schema:
  - Required fields present with correct data types
  - `test_suites` is non-empty (error if empty)
  - Suite names are unique, match `[a-z0-9][a-z0-9-]*`, max 30 characters
  - `junitxml` and `rerun_command` are present for every suite
  - `retries` in range 1--10 (if provided)
  - `timeout` and `rerun_timeout` are valid Go duration strings (if provided)
- Checks for framework-level retry configuration; warns if detected (D9)
- Checks state branch exists
- Checks GitHub token is present in environment
- Prints resolved config (including auto-detected github.owner/repo)
- Exit 0 if valid, exit 2 if invalid

### New commands

**`quarantine suite add`:** Deferred to a future version (see D4). Users add
suites by editing `.quarantine/config.yml` directly. A suite definition is ~5
lines of YAML. Interactive `suite add` with framework detection and validation
guidance may be added later.

**`quarantine suite list`**
- Discovers repo root, reads config
- Prints configured suites with key settings (name, command, junitxml)

**`quarantine suite remove <name>`**
- Discovers repo root, reads config
- Prints ramifications before asking for confirmation:
  ```
  Removing suite 'backend':
    - The 'backend' entry will be removed from .quarantine/config.yml
    - The state file (.quarantine/backend/state.json) on the quarantine/state
      branch will NOT be deleted — quarantined tests remain quarantined
    - GitHub issues for this suite's flaky tests will remain open but will no
      longer be updated by quarantine
    - If CI still runs `quarantine run backend`, it will error because the suite
      no longer exists in config — update your CI workflow first

  Are you sure? [y/N]
  ```
- Does NOT delete the state file on the state branch (preserves history)
- Does NOT delete or close issues associated with the suite

**`quarantine status [suite-name]`**
- Discovers repo root via git, reads config and state
- If suite name provided: shows status for that suite
- If no suite name and single suite: shows that suite
- If no suite name and multiple suites: shows summary for all suites
- Reads state file from state branch and recent artifacts for duration data
- Output:
  ```
  Suite: backend
  Quarantined tests: 12
  Avg quarantined test duration: 4.2s (from last 10 runs)
  Estimated CI time per run on quarantined tests: ~50s

  Oldest quarantined (consider closing if fixed):
    spec/models/user_spec.rb::validates email  (#42, 45 days, last failed 44 days ago)
    spec/models/order_spec.rb::calculates total (#51, 30 days, last failed 29 days ago)
  ```
- Duration data comes from the `duration_ms` field in recent results artifacts.
  If no artifacts are available, the duration line is omitted.
- Exit codes: 0 = success, 2 = config/state error

**`quarantine version`**
- Unchanged

---

## Testing Strategy

### Unit tests

| What | Why |
|------|-----|
| Config parser: `test_suites` array with validation | Core schema change; every field, every error case |
| Config parser: empty `test_suites` rejects as invalid | Prevents unconfigured state |
| Suite selection: single suite no-arg, multi-suite with name, missing name error | Branch logic that affects every run |
| Suite name validation: `[a-z0-9][a-z0-9-]*`, max 30 chars | Names used in file paths, HTML, CLI args, labels |
| Suite name too long: error and abort, not truncate | Strict enforcement |
| State file read/write at `.quarantine/<suite>/state.json` | Path convention, JSON structure, CAS SHA handling |
| State file creation on first run (file does not exist) | Bootstrap path |
| Rerun command template expansion via exec.Command with array (no shell) | Shell metacharacters in test names are harmless |
| Command execution via `exec.Command(command[0], command[1:]...)` (unmodified) | Verify quarantine never appends to the command |
| Config validation: junitxml always required | No conditional logic (D1) |
| Config validation: rerun_command always required | No conditional logic (D1) |
| Command crash detection: non-zero exit + no JUnit XML | Error message and exit code 2 |
| Rerun command failure: classify as unresolved, record rerun_exit_code, continue run | D16 handling |
| Unresolved tests contribute exit 2, not exit 1 | D16 exit code semantics |
| Exit code priority: 1 (genuine failures) > 2 (unresolved/infra) > 0 | Mixed result aggregation |
| Issue status batch check removes unquarantined tests from state | D18 step 0 |
| Quarantined test failure ignored in exit code | D18 core logic |
| Non-quarantined failure retried, classified flaky/genuine/unresolved | D18 classification |
| Zero-failure run still writes results.json and updates state | D19 behavior |
| CLI flag overrides on `quarantine run` | Flag values override per-suite config |
| Framework detection (package.json, Gemfile) | Advisory detection for init |
| JUnit XML prerequisite checking (jest-junit, rspec_junit_formatter) | Suite add warns when missing |
| Framework retry detection: Jest retryTimes in jest.config.* | D9 doctor enforcement |
| Framework retry detection: Vitest retry in vitest.config.* | D9 doctor enforcement |
| Framework retry detection: rspec-retry in Gemfile/Gemfile.lock | D9 doctor enforcement |
| Framework retry detection: absent config = no warning | No false positives |
| Framework retry detection: `retryTimes(0)` = no warning | D9 refined regex avoids false positive on no-op |
| Timeout: `timeout` field parsed as Go duration | D23 validation |
| Timeout: `rerun_timeout` field parsed as Go duration | D23 validation |
| Timeout: invalid duration string rejected by config parser | D23 validation |
| Timeout: command killed after timeout, partial XML processed | D23 behavior |
| Timeout: command killed after timeout, no XML, exit 2 | D23 + D15 interaction |
| Timeout: rerun killed after rerun_timeout, test classified unresolved | D23 + D16 interaction |
| Quarantined files list: `.quarantine/<suite>/quarantined-files.txt` written before command | D5 composable building block |
| Quarantined files list: empty when no quarantined tests | D5 edge case |
| Quarantined files list: deduplicated file paths | D5 correctness |
| Init pre-fills junitxml + rerun_command for detected frameworks | D1 init defaults |
| Init writes commented example when no framework detected | D1 fallback |
| Config validation: `command` must be a YAML array (non-empty) | Array format enforcement |
| Config validation: `rerun_command` must be a YAML array (non-empty) | Array format enforcement |
| Issue dedup label: `quarantine:<suite>:<hash>` under 50 chars | Label format, length constraint |
| Results JSON includes `suite_name` field | Dashboard association |
| Repo root discovery from subdirectory | Git-based walk-up |

### Interface tests

| What | Why |
|------|-----|
| `quarantine init` creates `.quarantine/config.yml`, `.gitignore`, state branch with README | End-to-end init flow |
| `quarantine init` is idempotent: re-running skips existing artifacts | D17 safety |
| `quarantine init` recreates missing artifacts without overwriting existing | D17 recovery |
| `quarantine init` detects frameworks and pre-fills suite entries | Init pre-fill |
| `quarantine doctor` warns on detected framework retries (does not error) | D9 doctor-only enforcement |
| `quarantine doctor` no retry warning when config absent | D9 no false positives |
| `quarantine suite remove` modifies config correctly | Removal without breaking YAML structure |
| `quarantine run <suite>` first run creates state file on state branch | Bootstrap path |
| `quarantine run <suite>` runs command, writes results | Happy path |
| `quarantine run <suite>` command crash: non-zero exit, no XML, exit 2 with diagnostic | Crash detection |
| `quarantine run <suite>` rerun command failure: test classified unresolved, exit 2 | D16 |
| `quarantine run <suite>` rerun failure + genuine failure: exit 1 (priority) | D16 exit code priority |
| `quarantine run <suite>` quarantined test failures ignored in exit code | D18 |
| `quarantine run <suite>` issue status check unquarantines tests with closed issues | D18 step 0 |
| `quarantine run <suite> --dry-run` does not execute test command | D22 |
| `quarantine run <suite> --dry-run` warns when no JUnit XML found | D22 missing XML |
| `quarantine run <suite>` zero failures: results.json still written | D19 |
| `quarantine run <suite>` writes quarantined-files.txt before executing command | D5 building block |
| `quarantine run <suite>` command timeout: SIGTERM then SIGKILL, partial results | D23 |
| `quarantine run <suite>` rerun timeout: test classified unresolved, run continues | D23 |
| `quarantine run <suite> --timeout 1s` overrides suite config | D23 flag override |
| `quarantine status <suite>` shows quarantine count and duration estimate | D5 visibility |
| `quarantine status` with no suites errors with guidance | Empty config path |
| `quarantine run` with single suite (no arg) | Convenience path |
| `quarantine run` with multiple suites (no arg) errors | Error path |
| `quarantine run` with no suites errors with guidance | Empty config path |
| `quarantine run` with invalid suite name in config errors | Strict validation |
| `quarantine run` from subdirectory discovers repo root | Path discovery |
| `quarantine doctor` validates valid and invalid configs | Schema validation |
| `quarantine doctor` errors on empty test_suites | Validation |
| `quarantine doctor` errors on suite name over 30 chars | Length enforcement |
| Parallel suite runs write to separate state files without conflict | CAS independence |
| CLI flag overrides applied correctly during run | Override precedence |

### Contract tests

| What | Why |
|------|-----|
| GitHub Contents API: create branch with initial README commit | Init creates state branch |
| GitHub Contents API: read/write state files at `.quarantine/<suite>/state.json` | Verify nested path interactions |
| GitHub Contents API: create file on first run (state file doesn't exist) | Bootstrap path |
| GitHub Contents API: list `.quarantine/` directory on state branch | Dashboard enumeration |
| GitHub Contents API: list returns 404 when `.quarantine/` doesn't exist yet | Dashboard graceful handling |
| GitHub Issues API: issue creation with suite-scoped dedup label | `quarantine:<suite>:<hash>` label |
| GitHub PR Comments API: per-suite comment marker | `<!-- quarantine:<suite-name> -->` marker |

### E2E tests

| What | Why |
|------|-----|
| `quarantine init` + edit config + `quarantine run` full flow | Complete happy path against real GitHub repo |
| Multi-suite: two suites configured, both run, separate state files | Multi-suite independence |
| Suite removal doesn't affect other suites' state | Independence |
| PR comments: one per suite on same PR | Both comments appear, no interference |
| Issue creation from different suites | Dedup works per suite, labels are correct |
| Artifact upload: per-suite results.json with suite_name field | Dashboard ingestion |
| Config with init-generated defaults + `quarantine run` in non-TTY | CI bootstrap path |
| `quarantine status` shows data from real artifacts | Status visibility |

---

## User Documentation

### Must write

| Document | Content |
|----------|---------|
| Quick start guide | `quarantine init` -> review/edit config -> `quarantine doctor` -> CI integration |
| CI integration guide | Per-CI-provider examples (GitHub Actions, GitLab CI, CircleCI) showing multi-suite setup |
| Config reference | `.quarantine/config.yml` schema with all fields and examples |
| CLI reference | All commands, subcommands, flags, exit codes |
| Framework setup guides | Per-runner guides (jest, rspec, vitest) with: how to configure JUnit XML output, recommended rerun command, and **disable framework retries** guidance |
| Disable framework retries guide | Why quarantine must own retries, how to disable per framework (Jest `retryTimes`, Vitest `retry` config), what happens if you don't |

### Must update

| Document | Change |
|----------|--------|
| `docs/specs/config-schema.md` | Replace flat schema with `test_suites` array schema; `.quarantine/config.yml` location; remove `framework` field entirely; no `exclude` field; remove `--config` flag documentation; add `timeout`, `rerun_timeout` fields; `junitxml` and `rerun_command` always required |
| `docs/specs/cli-spec.md` | Update init, run, doctor; add suite subcommands (list, remove; add deferred); add `quarantine status`; per-suite output dirs; repo root discovery; command crash handling; timeout enforcement (D23); remove `quarantine run -- <cmd>` syntax (replaced by `quarantine run [suite-name]`); remove `--config` flag; remove `--exclude` flag; remove `--output` flag (results always at `.quarantine/<suite>/results.json`); add `--timeout`, `--rerun-timeout` flags; replace entire execution flow (steps 1--14) with new wrap-don't-augment model; both `command` and `rerun_command` as YAML arrays executed via `exec.Command` (no shell); update all CI workflow examples to new syntax; update exit code table (unresolved → exit 2, not exit 1); add error prefix convention for exit-2 diagnostics |
| `docs/specs/architecture.md` | Update sequence diagram, config location, per-suite state files at `.quarantine/<suite>/state.json`, artifact schema with `suite_name`, remove `framework` field |
| `docs/specs/contracts.md` | Update PR comment marker (`<!-- quarantine:<suite-name> -->` replaces `<!-- quarantine-bot -->`), issue dedup label format (`quarantine:<suite>:<hash>` replaces `quarantine:{hash}`), state file location, results artifact schema changes (add `suite_name`, `rerun_exit_code`; remove `framework`), config file location and name |
| `schemas/quarantine-state.schema.json` | Per-suite file at `.quarantine/<suite>/state.json`; `last_flaky_at` → `last_failure_at` (done, M9); update `suite` field description to quarantine suite name semantic |
| `schemas/test-result.schema.json` | Add `suite_name`; remove `framework`; add `"unresolved"` to status enum; add `error` field to test_entry; add `rerun_exit_code` field to test_entry; remove `config.excluded_patterns` and `config.excluded_count`; remove `--output` flag reference |
| `docs/adr/010-config-format.md` | Amend: `.quarantine/config.yml`, `test_suites` array, `junitxml` and `rerun_command` always required, no `--config` flag |
| `docs/adr/016-v1-framework-scope.md` | Already superseded by ADR-030 (framework-agnostic design) |
| `docs/adr/030-framework-agnostic-design.md` | No amendment needed. Plan aligns with ADR-030 as accepted (`framework` removed from config entirely). |
| `docs/adr/020-test-id-construction.md` | Amend: test IDs scoped per suite state file, no global uniqueness assumption |
| `docs/adr/027-v2-webhooks-deferred.md` | Amendment reverted: all webhooks remain deferred to v3. State consolidation uses scheduled Action + CLI command. |
| `docs/milestones/m11.md` | Remove multi-framework numbered list from auto-detection; adjust scope |
| `docs/plans/auto-detect-framework.md` | Remove Case B (multi-framework selection); detection is advisory for init |
| `docs/milestones/index.md` | Replace "Phase 5 addition" webhook reference with Phase 6 M9/M10 milestones; renumber Phase 6 → Phase 7 |
| `docs/plans/webhooks.md` | Update status: all webhooks deferred to v3. State consolidation section updated to use scheduled Action. |

---

## Impact on Existing Milestones

M1--M8 are verified but not shipped. There are zero users and no production data.

### Migration strategy: single milestone, atomic commits

There are zero users and no production data. The convergence struct pattern
(supporting both old and new config formats simultaneously) was designed to
protect backwards compatibility --- but there is nothing to protect. Maintaining
dual code paths during migration adds bug surface without reducing risk.

Instead, two milestones (M9, M10) replace the old format with the new.
Ordering safety comes from atomic commits within each milestone, and a hard
boundary between them: M9 changes the irreducibly coupled core (config,
execution model, state paths, notifications); M10 adds features on top of
that stable surface (new commands, timeouts, dashboard, schemas). Each commit
compiles, tests pass, and git bisect works. No deprecation warnings, no dual
config paths, no deletion pass.

**Why two milestones, not one?** The execution flow in `runRun()` is tightly
coupled: `cfg.Framework` flows into exclusion args, state path, rerun command,
result metadata, PR comment marker, and issue label format --- six downstream
concerns. These must change together (M9). But new commands (`suite list`,
`suite remove`, `quarantine status`), timeout enforcement, dashboard migration,
and schema file updates are additive --- they consume M9's output but cannot
break it. Keeping them in M10 limits blast radius: a bug in `suite remove`
cannot cascade into the core run path.

### Migration impact matrix

68+ Go files reference `quarantine.yml`, `framework`, `quarantine-bot`,
`quarantine.json`, `BuildExclusionArgs`, `RerunCommand`, or `SplitShellArgs`
(verified against the M1--M8 codebase as of 2026-04-11). The matrix below
categorizes files by the nature of the change required.

| Category | Files affected | Change type |
|----------|---------------|-------------|
| Config parsing (`config.go`, `config_test.go`, schema types) | ~6 source + tests | **Rewrite:** remove top-level `Framework`, `Exclude`, `RerunCommand`; add `TestSuites []SuiteConfig`; `junitxml` and `rerun_command` always required; new parse + validate |
| Config validation (`doctor.go`, `doctor_*_test.go`) | ~4 source + tests | **Rewrite:** suite names, durations, all fields always required, retry detection (D9 doctor-only) |
| Command interface (`run.go`, Cobra setup, `init.go`) | ~5 source + tests | **Rewrite:** `runRun` reads suite from config (no `--` separator, no CLI args for test command); new `init` flow; suite selection logic |
| Suite subcommands | New files | **New:** `suite_list.go`, `suite_remove.go`, `status.go` + tests (`suite add` deferred) |
| Runner (`runner.go`, `exclusion.go`, `exclude_pattern.go`) | ~3 source + tests | **Simplify:** delete exclusion logic, delete `Framework` type, `RerunCommand` takes `[]string` directly |
| State I/O (`state.go`, CAS logic) | ~3 source + tests | **Parameterize:** state path `.quarantine/<suite>/state.json` |
| Notifications (`run_notifications.go`, issues, PR comments) | ~4 source + tests | **Parameterize:** comment marker `<!-- quarantine:<suite> -->`, label `quarantine:<suite>:<hash>` |
| JUnit XML parsing | ~3 source + tests | **Unchanged:** parsing logic is framework-agnostic already |
| Rerun command (template logic in `runner.go`) | ~2 source + tests | **Minor:** takes `[]string` array directly, add `rerun_timeout`; `SplitShellArgs` likely unused |
| Result schema (`result.go`, `result_test.go`) | ~3 source + tests | **Moderate:** add `suite_name`, `error`, `rerun_exit_code`, `"unresolved"` status; remove `framework` |
| Dashboard ingest | ~4 source + tests | **Rewrite:** enumerate per-suite state files, new results schema |
| Test fixtures (~43 test files) | ~43 files | **Rewrite fixtures:** config YAML from flat to `test_suites` array; mock API paths from `quarantine.json` to `.quarantine/<suite>/state.json`; CLI args from `["--", "jest"]` to `["backend"]`; assertion strings for markers and labels |

**Atomic commit sequence:** Each commit keeps tests green. The commits are
ordered to minimize blast radius:

**M9 commits:**

1. **Add `TestSuites` to Config** (additive --- `Framework` field still exists,
   all tests compile). `junitxml` and `rerun_command` always required. New
   parsing and validation tests.
2. **Refactor `runRun`** to read suite fields instead of top-level
   `cfg.Framework` / `cfg.RerunCommand`. Update test fixtures from
   `quarantine.yml` to `.quarantine/config.yml` with `test_suites`.
3. **Per-suite state paths** (`.quarantine/<suite>/state.json`). Update mock
   API endpoints in tests.
4. **Per-suite notifications** (comment marker, issue label format). Update
   assertion strings in tests.
5. **Rewrite init.** New flow: create `.quarantine/` directory, `config.yml`
   with pre-filled suite entries for detected frameworks, `.gitignore`, state
   branch with `README.md`. New test suite.
6. **Delete old code.** Remove top-level `Framework` field, `validFrameworks`
   map, exclusion logic (`exclusion.go`, `exclude_pattern.go`), `--` separator
   handling, `--config` flag, `--output` flag, `--exclude` flag,
   `SplitShellArgs` (if unused). Delete tests for removed features.

**M10 commits:**

7. **New commands.** `suite list`, `suite remove`, `quarantine status`. Tests.
8. **Timeout enforcement.** `timeout` and `rerun_timeout` config fields,
   SIGTERM/SIGKILL logic, per-rerun timeout with unresolved classification.
9. **Quarantined files list.** Write `.quarantine/<suite>/quarantined-files.txt`
   before command execution. Tests.
10. **Error prefixes.** Consistent parseable prefix for all exit-2 diagnostics.
11. **Dashboard + schemas.** Enumerate per-suite state files. Results schema
    adds `suite_name`, removes `framework`. Schema files updated.

### M9: Core Conversion

The irreducibly coupled core — config, execution model, state paths, and
notifications must change together because `cfg.Framework` flows into six
downstream concerns. These changes form one milestone because separating them
requires a translation layer that adds bug surface without reducing risk.

**Dependencies:** M1--M8 verified (current state).

**Scope:**
- New config format: `.quarantine/config.yml` with `test_suites` array
- `framework` removed from config; `junitxml` and `rerun_command` always
  required (D1)
- `quarantine run [suite-name]` reads command from config (no `--` separator)
- `command` and `rerun_command` as YAML arrays executed via `exec.Command`
- Suite selection: single suite no-arg, multi-suite name required
- Per-suite state files at `.quarantine/<suite>/state.json`
- Per-suite results at `.quarantine/<suite>/results.json`
- Per-suite PR comments (`<!-- quarantine:<suite> -->` marker)
- Per-suite issue dedup (`quarantine:<suite>:<hash>` label)
- Command crash detection (D15)
- Rerun failure handling with `"unresolved"` status (D16)
- New `quarantine init` flow with framework detection (pre-fills config, D4)
- `quarantine doctor` updated: suite validation, retry detection (D9)
- Delete: exclusion logic, `quarantine.yml` support, `--config` flag,
  `--exclude` flag, `--output` flag, `--` separator syntax, `Framework` type

**Does NOT include:**
- New CLI commands (M10)
- Timeout enforcement (M10)
- `quarantined-files.txt` generation (M10)
- Dashboard migration (M10)
- Schema file updates (M10)
- Full exclusion (deferred to v2)
- Combined PR comments (deferred to v2)
- `quarantine suite add` (deferred; users edit YAML, D4)

**Demoable artifact:** Single suite configured, runs, state on branch, PR
comment, issue created — on a real GitHub repo.

### M10: Additive Features

New commands, timeout enforcement, and downstream consumers. These are built
on M9's stable surface and cannot cascade failures into the core execution
path.

**Dependencies:** M9 complete.

**Scope:**
- `suite list`, `suite remove` commands
- `quarantine status` command
- Timeout enforcement: `timeout` and `rerun_timeout` (D23)
- Writes `.quarantine/<suite>/quarantined-files.txt` before command (D5)
- Error prefix convention for exit-2 diagnostics (D16)
- Dashboard: enumerate per-suite state files, new result schema
- Schema files updated

**Demoable artifact:** Two suites configured, `quarantine status` shows both,
timeouts enforced, dashboard reads per-suite state.

### Existing milestones: unchanged

M1--M8 retain their current scope and implementation. M9/M10 refactor on top.

| Milestone | Relationship to M9/M10 |
|-----------|--------------------------|
| M1 | M9: Config rewritten. Init rewritten. |
| M2 | M9: `runRun` refactored. `-- <cmd>` syntax removed. |
| M3 | M9: Rerun command from per-suite config array. |
| M4 | M9: State files at `.quarantine/<suite>/state.json`. |
| M5 | M9: PR comment marker and issue dedup label format updated. |
| M6 | M10: Dashboard enumerates per-suite state files. |
| M7 | M10: Dashboard per-suite and cross-suite views. |
| M8 | M10: Error handling and degraded mode per suite. Error prefixes for exit 2. |
| M11 | M9: Scope reduced: detection pre-fills config. Init detects and suggests. |
| M12--M16 | Unchanged. State consolidation uses scheduled Action + CLI command, not webhooks. |

---

## v2+ Future Work

### v2: Dashboard State Consolidation

A new `quarantine state consolidate` CLI command reads all per-suite state files
from the `quarantine/state` branch and writes a single `state.json`. This reduces
dashboard API calls from 1 + N (list + read per suite) to 1 (single read) per
repo per poll interval.

The consolidated file:

```json
{
  "version": 1,
  "consolidated_at": "2026-04-10T12:00:00Z",
  "suites": {
    "backend": { "tests": { ... } },
    "frontend": { "tests": { ... } }
  }
}
```

Per-suite files remain the source of truth (written by the CLI). The consolidated
file is a read-optimized derivative.

**Trigger:** A GitHub Actions workflow runs `quarantine state consolidate` on a
schedule or after CI completes:

```yaml
# .github/workflows/quarantine-consolidate.yml
on:
  schedule:
    - cron: '*/5 * * * *'
  workflow_run:
    workflows: ["CI"]
    types: [completed]

jobs:
  consolidate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ref: quarantine/state
      - name: Install quarantine
        run: curl -sSL https://raw.githubusercontent.com/mycargus/quarantine/main/scripts/install.sh | bash
      - run: quarantine state consolidate
        env:
          QUARANTINE_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

This approach requires no webhook infrastructure (no public endpoint, no webhook
secret, no HMAC verification, no job queue). All webhooks remain deferred to v3
per ADR-027.

### v2: Per-framework exclusion adapters

Optional accelerators that add test-level exclusion precision on top of v1's
file-level `quarantined-files.txt`. Likely approaches:
- **Jest:** Custom `globalSetup` that reads quarantine state and dynamically sets
  `testPathIgnorePatterns` or `testNamePattern` with negative lookahead.
- **Vitest:** Similar to Jest, or `--tagsFilter` if test tagging is viable.
- **RSpec:** `spec_helper.rb` integration using metadata-based filtering
  (`config.filter_run_excluding :quarantined => true`).

Each adapter needs its own design, implementation, and test coverage. See D5.
The file-level mechanism works for any framework; per-framework adapters are an
optimization for teams that need test-level granularity.

### v2: Combined PR comment

After all suite runs in a workflow complete, a post-workflow aggregation step
combines per-suite PR comments into a single comment.

---

## Open Questions

### OQ1: Framework detection scope

Init detects frameworks, but a "test suite" is more than a framework. A repo with
jest might have unit tests and integration tests as separate suites. Detection
identifies frameworks (jest, rspec, vitest), but the user defines suites.

**Decision for v1:** Detection identifies frameworks. Init pre-fills suite
entries with `junitxml` and `rerun_command` defaults for each detected framework
(D1). The user edits the generated config to match their actual setup. Framework
detection is a convenience, not a requirement.

### OQ2: State file cleanup

When a suite is removed via `quarantine suite remove`, its state file remains on
the state branch (preserving history). Over time, removed suites accumulate
orphaned state files.

**Decision for v1:** No automatic cleanup. State files are small (kilobytes).
Manual cleanup is possible via git if needed. Revisit if users report clutter.

### OQ3: Config file format for `command` — RESOLVED

**Decision for v1:** Array form only (executed via `exec.Command`). This
eliminates the shell layer entirely --- no `sh -c`, no process groups, no shell
metacharacter surprises. Matches Docker Compose's `command` field convention.
Users who need shell features explicitly write
`command: ["sh", "-c", "npm test && echo done"]`.

**YAML type coercion:** With array format, the YAML coercion pitfall (bare `yes`
parsed as boolean) applies to individual array elements rather than the whole
command. The config parser validates that `command` and `rerun_command` are YAML
sequences (`!!seq`), not scalars. If a user writes `command: "npm test"` (string
instead of array), the parser rejects it with a diagnostic:

```
Error: test_suites[0].command must be a YAML array, not a string.
  Use: command: ["npm", "test"]
```

This catches the most common mistake (writing a string instead of an array) at
config load time.

### OQ4: Rerun command placeholder semantics

The `rerun_command` template uses `{name}`, `{classname}`, `{file}` placeholders.
Precise definitions:
- `{file}` = `file_path` component from the test ID (from JUnit XML)
- `{name}` = `name` attribute from JUnit XML `<testcase>`
- `{classname}` = `classname` attribute from JUnit XML `<testcase>`

With the array format, placeholders are substituted directly within each array
element via simple string replacement. No `SplitShellArgs` is needed --- the
array already defines the token boundaries. Because `exec.Command` is used (no
shell), shell metacharacters in substituted values are harmless. Needs a spec
with examples covering edge cases (test names with quotes, parentheses, spaces).

### OQ5: Test command detection

`quarantine init` detects frameworks (jest/vitest from `package.json`, rspec from
`Gemfile`) and pre-fills suite entries with `junitxml` and `rerun_command`
defaults for each detected framework (D1). The user edits the generated config
to match their actual setup.

Detection is best-effort and advisory only. It does not gate any functionality.

