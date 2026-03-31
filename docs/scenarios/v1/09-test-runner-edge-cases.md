# Test Runner Edge Cases

### Scenario 52: quarantine run without -- separator [M2]

**Risk:** The CLI misinterprets test command arguments as its own flags, causing unpredictable behavior or silently swallowing test runner options.

**Given** the CLI is configured in CI

**When** the developer runs `quarantine run jest --ci` (missing `--` separator)

**Then** the CLI prints:
```
Error: missing '--' separator. Usage: quarantine run [flags] -- <test command>

Example: quarantine run --retries 3 -- jest --ci
```
Exits with code 2. The test command is NOT executed.

---

### Scenario 53: Test command not found [M2]

**Risk:** A typo in the test command produces exit code 1 (suggesting test failure) instead of exit code 2 with a diagnostic, misleading users about the cause.

**Given** the CLI is configured in CI and `quarantine.yml` is valid

**When** the developer runs `quarantine run -- jset --ci` (typo: `jset` instead
of `jest`) and the command is not found on PATH

**Then** the CLI prints:
```
Error: command not found: "jset". Ensure the test runner is installed and on PATH.
```
Exits with code 2. No tests ran — this is a quarantine error, not a test
failure.

---

### Scenario 54: No JUnit XML produced [M2]

**Risk:** The CLI exits 0 when the test runner crashes before producing XML, falsely indicating all tests passed.

**Given** the CLI is configured in CI with `junitxml: junit.xml` and the test
runner crashes before producing XML output (e.g., segfault, OOM, or the test
command doesn't produce JUnit XML)

**When** the developer runs `quarantine run -- jest --ci` and the test command
exits non-zero, and no file matches the `junit.xml` glob

**Then** the CLI logs:
`[quarantine] WARNING: No JUnit XML found at 'junit.xml'. Cannot determine test
results. Suggest checking --junitxml flag or jest-junit configuration.`
Exits with the test runner's exit code (since the CLI cannot determine whether
the failure was a test failure or infrastructure issue).

---

### Scenario 55: Malformed JUnit XML [M2]

**Risk:** Malformed XML causes the CLI to crash or exit 2, breaking the build due to quarantine infrastructure rather than test results (FR-1.6.3).

**Given** a single JUnit XML file exists but is truncated or contains invalid
XML

**When** the CLI attempts to parse it after the test run

**Then** the CLI logs:
`[quarantine] WARNING: Failed to parse junit.xml: XML syntax error at line 42.
Skipping.`
Treats this as "no XML produced" and exits with the test runner's exit code.

---

### Scenario 56: Multiple XML files, some malformed (parallel runners) [M2]

**Risk:** One corrupt XML file from a crashed shard causes all test results to be discarded, losing valid results from other shards.

**Given** the project uses Jest with `--shard` and produces 4 JUnit XML files.
3 are valid and 1 is truncated.

**When** the CLI parses the XML files matching the glob pattern

**Then** the CLI:
1. Parses all 4 files. Logs a warning for the malformed one:
   `[quarantine] WARNING: Failed to parse results/shard-3.xml: unexpected EOF.
   Skipping.`
2. Merges results from the 3 valid files.
3. Logs: `[quarantine] Parsed 3/4 JUnit XML files. 1 malformed, skipped.`
4. Proceeds with flaky detection and quarantine logic using the partial results.
5. Exits based on the partial results (correct: partial results better than
   none).

---

### Scenario 70: Signal forwarding to child process [M2]

**Risk:** If the CLI does not forward SIGINT/SIGTERM to the child test process, Ctrl-C in a terminal or CI cancellation leaves orphaned test runner processes consuming resources.

**Given** the CLI is running `quarantine run -- jest --ci` and the test suite is
mid-execution

**When** the CLI process receives SIGINT (e.g., Ctrl-C) or SIGTERM (e.g., CI
cancellation)

**Then** the CLI:
1. Forwards the signal to the child test runner process.
2. Waits for the child process to exit.
3. Does not attempt to parse JUnit XML (the run was interrupted).
4. Exits with code 130 (SIGINT) or 143 (SIGTERM), matching standard Unix signal
   exit conventions.

---

### Scenario 71: --retries CLI flag overrides config retries value [M2]

**Risk:** A developer sets `retries: 1` in `quarantine.yml` for normal runs but
wants to run a quick one-shot pass with `--retries 0` (or a different value) in
a specific CI job. If the flag is silently ignored, retry behavior is wrong and
the discrepancy is invisible.

**Given** `quarantine.yml` contains `retries: 1`

**When** the developer runs `quarantine run --retries 3 -- <test command>`

**Then** the CLI uses `retries: 3` for this run (flag wins over config). The
`results.json` artifact reflects `config.retry_count: 3`. Exits with code 0 if
all tests pass.

---

### Scenario 72: JUnit XML `<error>` element produces status not in schema [M2]

**Risk:** A test framework produces `<error>` instead of `<failure>` for a crashed test. The CLI parser maps this to `"error"` status, which is not in the `test-result.schema.json` enum (`passed|failed|skipped|quarantined|flaky`). The dashboard rejects the result at schema validation, silently dropping the entire test run.

**Given** a test suite where one test crashes with an unhandled exception, and
the test runner produces JUnit XML with an `<error>` child element on the
`<testcase>` (instead of `<failure>`)

**When** the CLI parses the JUnit XML

**Then** the CLI maps the `<error>` element to a status value that conforms to
`test-result.schema.json`. The resulting `results.json` passes schema
validation. The dashboard ingests it without rejection.

---

### Scenario 57: All tests in the suite are quarantined — Jest/Vitest [M4]

**Risk:** Excluding all tests causes the test runner to report "no tests found" (exit non-zero), which the CLI misinterprets as a test failure and exits 1.

**Given** `quarantine.json` contains entries for every test in the suite (e.g.,
50 out of 50 tests are quarantined), and all corresponding GitHub Issues are
open. The project uses Jest.

**When** CI executes `quarantine run -- jest --ci ...`

**Then** the CLI constructs exclusion flags that exclude all 50 tests from
execution. The test runner executes 0 tests. Jest exits non-zero with "No tests
found." The CLI detects this condition: the JUnit XML contains zero test cases
(or no XML is produced) and the number of quarantined exclusions equals or
exceeds the expected test count. The CLI treats this as a successful run. Posts a
PR comment:
`All 50 tests in this suite are currently quarantined. No tests were executed.
Consider reviewing quarantined tests and closing resolved issues.`
Results artifact contains `summary.total: 0`, `summary.quarantined: 50`.
Logs to stderr:
`[quarantine] WARNING: All tests excluded by quarantine. No tests executed.`
Exits with code 0.

---

### Scenario 58: All tests in the suite are quarantined — RSpec [M4]

**Risk:** All quarantined RSpec tests run and fail, and the CLI does not suppress all failures, incorrectly exiting 1 and breaking the build.

**Given** `quarantine.json` contains entries for every test in the suite (e.g.,
50 out of 50 tests), all issues open. The project uses RSpec.

**When** CI executes `quarantine run -- rspec ...`

**Then** because RSpec uses post-execution filtering (not pre-execution
exclusion), all 50 tests still run. If any fail, their failures are suppressed
from the exit code (all are quarantined). The CLI posts a PR comment noting all
50 tests are quarantined and suggests reviewing. Exits with code 0.

---

### Scenario 80: Default Jest rerun uses `npx jest` — framework binary not on PATH [M3]

**Risk:** If the CLI reruns failing tests using bare `jest`, the rerun silently
fails when Jest is installed locally (in `node_modules/.bin`) but not globally.
The test is classified as a genuine failure rather than flaky, quarantine never
quarantines it, and the build continues to fail on every run. Discovered during
dogfooding: all flaky tests showed 0 flaky detections because reruns were
failing with "command not found".

**Given** a project using Jest where `jest` is installed as a local
devDependency (in `node_modules/.bin/jest`) but not available as a global
command, and `quarantine.yml` does not define `rerun_command`

**When** a test fails on the initial run and the CLI retries it

**Then** the CLI reruns the test using `npx jest --testNamePattern '<name>'`.
`npx` resolves the local `node_modules/.bin/jest` installation without
requiring any PATH modification. If the test passes on retry, it is classified
as flaky. The `rerun_command` config option overrides this default when set.

---

### Scenario 81: Default Vitest rerun uses `npx vitest` — framework binary not on PATH [M3]

**Risk:** Same as Scenario 80 for Vitest projects. Bare `vitest` fails when
Vitest is a local devDependency, silently preventing flaky detection.

**Given** a project using Vitest where `vitest` is installed as a local
devDependency but not available as a global command, and `quarantine.yml` does
not define `rerun_command`

**When** a test fails on the initial run and the CLI retries it

**Then** the CLI reruns the test using
`npx vitest run --reporter=junit <file> -t '<name>'`.
`npx` resolves the local `node_modules/.bin/vitest` installation without
requiring any PATH modification. If the test passes on retry, it is classified
as flaky. The `rerun_command` config option overrides this default when set.

---

### Scenario 82: Default RSpec rerun uses `bundle exec rspec` — gem not on PATH [M3]

**Risk:** Same failure mode for RSpec projects. Bare `rspec` fails when RSpec
is managed via Bundler (the standard for Ruby projects), silently preventing
flaky detection.

**Given** a project using RSpec where `rspec` is managed by Bundler and not
available as a bare global command, and `quarantine.yml` does not define
`rerun_command`

**When** a test fails on the initial run and the CLI retries it

**Then** the CLI reruns the test using `bundle exec rspec -e '<name>'`.
`bundle exec` invokes the Bundler-managed RSpec without requiring PATH
modification. If the test passes on retry, it is classified as flaky. The
`rerun_command` config option overrides this default when set.

---
