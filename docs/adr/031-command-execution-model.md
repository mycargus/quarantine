# ADR-031: Command Execution Model — Wrap Unmodified, No Shell, Explicit Timeouts

**Status:** Proposed
**Date:** 2026-04-11

## Context

The multi-suite support plan replaces `quarantine run -- <test command>` (where
the test command was constructed and augmented by quarantine) with
`quarantine run [suite-name]` (where the command comes from the suite's
`command` field in config and is run unmodified).

This introduces several coupled decisions about how quarantine executes test
commands, handles failures, and enforces timeouts. These decisions interact
strongly: the choice to use `exec.Command` (no shell) determines how signals
propagate, how shell metacharacters are handled, and how timeouts can be
reliably enforced.

The old model appended framework-specific exclusion flags to the test command.
This was fragile: npm requires `--` to forward flags, rake ignores unknown
flags, and wrapper scripts may not pass flags through. Users who relied on
quarantine to construct the command had two sources of truth for how tests
run — their CI config and quarantine's flag logic.

Three distinct failure modes require explicit handling:

1. **Command crash:** The test command exits non-zero but produces no JUnit XML.
   This typically means the test runner crashed before starting (syntax error,
   missing dependency). Without disambiguation, quarantine would treat this as
   "all tests failed," which is misleading and may suppress the real error.

2. **Rerun failure:** The `rerun_command` fails for a specific test (binary not
   found, bad template expansion, crash). If quarantine aborts the entire run,
   results from other tests are lost. If it treats the test as a genuine
   failure, it may send a false signal on exit code.

3. **Timeout:** A hanging test command blocks the CI pipeline indefinitely.
   Without quarantine-level timeouts, a stalled test runner holds the CI job
   until the job's own timeout fires — often 6+ hours, wasting capacity.

## Decision

### 1. Execute via `exec.Command` — no shell, array format

Both `command` and `rerun_command` in the suite config are YAML arrays. They
are executed via `exec.Command(args[0], args[1:]...)` with no shell
intermediary. Signal forwarding uses `cmd.Process.Signal(sig)` directly.

**Why array format:** A string `command` field would require `sh -c` to
execute, introducing shell interpretation (quoting, metacharacters, `&&`/`||`
operators, variable expansion), process group management (`Setpgid`,
`syscall.Kill(-pgid, ...)`), and platform-dependent behavior (no Windows, shell
differences between `/bin/sh` versions). The array format eliminates all of this.
Users who need shell features explicitly write
`command: ["sh", "-c", "npm test && echo done"]`.

Rerun command placeholder substitution (`{name}`, `{file}`, `{classname}`)
is performed via simple string replacement within each array element before
`exec.Command` is called. Because no shell is involved, shell metacharacters
in test names from JUnit XML (quotes, semicolons, backticks) are passed as
literal characters to the test runner — harmless.

### 2. Wrap the command unmodified — quarantine never appends flags

The suite's `command` field is the user's existing test command. Quarantine
runs it exactly as configured and never appends flags or modifies arguments.
Quarantine's sole requirement is that the command produces JUnit XML as a
side effect (configured in the test runner's own setup: `jest.config.js`,
`.rspec`, `vitest.config.ts`).

### 3. Command crash detection

When the test command exits non-zero **and** no JUnit XML files match the
`junitxml` glob:

- Classify as a **command crash** (the runner failed to start or aborted
  before producing output), not as test failures.
- Print a diagnostic error:
  ```
  Error [crash]: test command exited with code 1 but no JUnit XML files found at 'junit.xml'.
  This usually means the test runner crashed before producing results.
  Check that:
    - Your test command ('npm test') runs successfully outside of quarantine
    - JUnit XML output is configured in your test runner
    - the junitxml path in .quarantine/config.yml matches where your runner writes XML
  ```
- Exit with code **2** (quarantine infrastructure error, not test failure).

When the test command exits non-zero **and** JUnit XML is present (even
partial), quarantine processes the XML normally. Partial results from a timed-
out run are processed rather than discarded.

### 4. Rerun failure handling — "unresolved" classification

When `rerun_command` fails for a specific test (wrong binary path, bad template
expansion, crash, or timeout):

- **Do NOT abort the run.** Continue processing other failed tests.
- Classify the specific test as **unresolved**: not flaky (couldn't prove it),
  not genuine failure (couldn't complete retries).
- Record in `results.json`:
  - `status: "unresolved"`
  - `error`: human-readable description of the rerun failure
  - `rerun_exit_code`: the actual exit code from the crashed rerun process
- Report in the PR comment: `"1 test could not be retried — rerun command failed: <error>"`
- Do NOT create a flaky-test issue (the test is not confirmed flaky).
- Exit with code **2** (infrastructure error).

**Exit code priority when results are mixed:**
1. Any genuine test failure → exit **1** (test failures take priority)
2. Any unresolved test (no genuine failures) → exit **2** (infrastructure error)
3. All non-quarantined tests pass → exit **0**

### 5. Per-suite timeout enforcement

Each suite has two optional duration fields (Go `time.ParseDuration` format):

- `timeout` (default 30m): applies to the suite's `command`.
- `rerun_timeout` (default 5m): applies to each individual rerun invocation.

**Command timeout behavior:**
1. `SIGTERM` sent to the process.
2. 5-second grace period for shutdown.
3. `SIGKILL` if still running.
4. If JUnit XML exists at timeout, process partial results (better than none).
5. If no JUnit XML exists, treat as command crash (see §3 above).
6. Exit 2 with diagnostic: `Error [timeout]: test command timed out after 30m.`

**Rerun timeout behavior:**
1. Kill the rerun process (`SIGKILL`).
2. Classify the test as unresolved with `error: "rerun timed out after 5m"`.
3. Continue processing remaining failed tests.

CLI flags `--timeout` and `--rerun-timeout` override per-suite config for that
invocation (consistent with the flag-override-config principle from ADR-010).

### 6. Error prefix convention for exit-2 diagnostics

All exit-2 error messages use a consistent parseable prefix to distinguish
failure modes in CI logs:

```
Error [config]: ...
Error [crash]: ...
Error [timeout]: ...
Error [rerun]: ...
```

## Alternatives Considered

- **String `command` field executed via `sh -c`.** Allows shell features
  (`&&`, pipes, variable expansion) without the user having to write
  `["sh", "-c", "..."]`. Rejected because it requires process group
  management for signal forwarding, introduces platform-dependent shell
  behavior, and creates a security surface for shell metacharacters in test
  names during rerun command expansion.

- **Abort the entire run on rerun failure.** Simpler error handling — the run
  either succeeds or fails, no "unresolved" intermediate state. Rejected
  because it discards results from tests that were successfully processed
  before the failure, and forces the user to re-run the entire suite to
  find out the status of other tests.

- **Treat rerun failure as a genuine test failure (exit 1).** Conflates an
  infrastructure problem (broken rerun command) with actual test failures.
  Exit code 1 must mean "your tests failed" — using it for a broken binary
  path misleads developers and may trigger incorrect escalation. Rejected.

- **Rely on the CI job's timeout rather than quarantine-level timeouts.**
  CI job timeouts are typically 6+ hours and not configurable per test
  suite. Quarantine sits between CI and the test runner; without its own
  timeout, a stalled test runner holds the job indefinitely. Rejected.

- **SIGKILL immediately on timeout without SIGTERM.** Faster but denies the
  test runner the opportunity to write partial JUnit XML and do cleanup.
  Rejected — the 5-second SIGTERM window has negligible cost and may recover
  usable partial results.

## Consequences

**Positive:**

- (+) No shell layer means no platform-dependent behavior, no process group
  complexity, no metacharacter surprises in rerun command substitution.
- (+) Wrapping the user's existing command eliminates the two-source-of-truth
  problem (quarantine's flags vs. the user's CI script).
- (+) Command crash detection gives users actionable diagnostics when their
  test runner fails to start, rather than a misleading "all tests failed" signal.
- (+) "Unresolved" classification prevents infrastructure errors from being
  reported as test failures, keeping exit code 1 semantically clean.
- (+) Per-suite timeouts prevent runaway test processes from blocking CI
  pipelines for hours.
- (+) Parseable error prefixes make CI log parsing tractable without adding
  additional exit codes (which would be a breaking change).

**Negative:**

- (-) Users who rely on shell features in their test command must add
  `["sh", "-c", "..."]` explicitly. This is a mild ergonomic cost.
- (-) The "unresolved" state is a new concept that users must learn. The PR
  comment and results.json clarify it, but it adds cognitive load.
- (-) Partial XML processing on timeout may produce confusing results if the
  test runner wrote an incomplete XML file. Mitigated by the diagnostic
  message noting that results are partial.
- (-) Process group isolation is not implemented (if the test runner spawns
  children, those children survive quarantine's signal). This matches CI
  system behavior and is a known limitation documented for v2.
