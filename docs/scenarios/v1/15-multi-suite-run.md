# Multi-Suite Run

### Scenario 117: quarantine run executes suite command unmodified via exec.Command [M9]

**Risk:** Quarantine appends framework-specific flags or modifies the user's
command, breaking wrapper scripts (npm, rake, Makefile targets) that do not
pass through unknown flags.

**Given** `.quarantine/config.yml` with one suite:
```yaml
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec", "--format", "progress"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
    retries: 3
```

**When** CI executes `quarantine run backend`

**Then** quarantine:
1. Reads the suite's `command` field from config.
2. Executes `bundle exec rspec --format progress` via
   `exec.Command("bundle", "exec", "rspec", "--format", "progress")` —
   exactly as configured, with no modifications, no appended flags, and
   no shell intermediary.
3. The process receives SIGINT/SIGTERM via `cmd.Process.Signal(sig)` directly
   (no process group management needed — no shell layer).
4. The test runner produces `rspec.xml`.
5. Quarantine parses the XML, processes results, and exits 0 (all tests pass).

The `-- ` separator syntax is **not** accepted. Running
`quarantine run -- bundle exec rspec` exits 2 with a usage error.

---

### Scenario 118: quarantine run with single configured suite and no name argument runs it [M9]

**Risk:** When only one suite is configured, requiring the user to always type
the suite name adds friction and causes `quarantine run` to fail in CI
workflows written before the suite was named.

**Given** `.quarantine/config.yml` with exactly one suite named `unit`

**When** CI executes `quarantine run` (no suite name argument)

**Then** quarantine detects that exactly one suite is configured, selects it
automatically, and runs it — identical to `quarantine run unit`. Exits 0 if
all tests pass.

No suite-selection prompt or warning is printed; the single-suite shortcut
is silent.

---

### Scenario 119: quarantine run with multiple suites and no name argument exits 2 [M9]

**Risk:** Without a required suite name argument for multi-suite repos, quarantine
silently runs the wrong suite or picks an arbitrary one, producing misleading
results.

**Given** `.quarantine/config.yml` with two suites named `backend` and `frontend`

**When** the developer runs `quarantine run` (no suite name)

**Then** the CLI exits with code 2 and prints:
```
Error [config]: Multiple test suites configured. Specify a suite name:

  quarantine run backend
  quarantine run frontend

Run `quarantine suite list` to see all configured suites.
```

No test command is executed.

---

### Scenario 120: quarantine run with no suites configured exits 2 with guidance [M9]

**Risk:** After a fresh `quarantine init` with no framework detected, `quarantine run`
produces an obscure error rather than guiding the user to add a suite definition.

**Given** `.quarantine/config.yml` exists with an empty `test_suites` array
(or a `test_suites` key with only comments)

**When** the developer runs `quarantine run`

**Then** the CLI exits with code 2 and prints:
```
Error [config]: No test suites configured. Edit .quarantine/config.yml to add one.
```

No test command is executed. `quarantine doctor` also reports this as an error
(not a warning).

---

### Scenario 121: quarantine run detects flaky test and creates per-suite state file and PR comment [M9]

**Risk:** The CLI writes to a shared `quarantine.json` rather than a suite-scoped
state file, causing concurrent suite runs to contend and overwrite each other's
state.

**Given** `.quarantine/config.yml` with a `backend` suite

**And** the `quarantine/state` branch exists but `.quarantine/backend/state.json`
does not yet exist (first run for this suite)

**And** CI is running on a PR and `GITHUB_BASE_REF` is set

**When** CI executes `quarantine run backend` and the test
`spec/models/user_spec.rb::User::validates email` fails initially but passes
on retry

**Then** quarantine:
1. Detects the test as flaky.
2. Creates `.quarantine/backend/state.json` on the `quarantine/state` branch
   with the new entry (the file did not exist; it is created, not updated).
3. Creates a GitHub Issue with label `quarantine:backend:<test_hash>` (8 hex
   chars), where `<test_hash>` = first 8 chars of SHA-256 of the test ID.
4. Posts a PR comment containing `<!-- quarantine:backend -->` as the HTML
   marker (not `<!-- quarantine-bot -->`).
5. Writes `.quarantine/backend/results.json` with `suite_name: "backend"`.
6. Exits with code 0 (flaky = not a real failure).

---

### Scenario 122: quarantine run when test command exits non-zero and no JUnit XML found [M9]

**Risk:** A test runner crash (syntax error, missing dependency, misconfigured
JUnit reporter) is misinterpreted as "all tests failed," masking the real
problem and producing no actionable diagnostic.

**Given** `.quarantine/config.yml` with a `frontend` suite:
```yaml
  - name: frontend
    command: ["npx", "jest", "--ci"]
    junitxml: "junit.xml"
    rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
```

**And** Jest is not installed (or fails to start due to a syntax error in
`jest.config.js`)

**When** CI executes `quarantine run frontend`

**Then** the test command exits non-zero and no `junit.xml` file is produced.
Quarantine:
1. Detects the non-zero exit AND the absence of any files matching `junit.xml`.
2. Classifies this as a **command crash**, not test failures.
3. Prints to stderr:
   ```
   Error [crash]: test command exited with code 1 but no JUnit XML files found at 'junit.xml'.
   This usually means the test runner crashed before producing results.
   Check that:
     - Your test command ('npx jest --ci') runs successfully outside of quarantine
     - JUnit XML output is configured in your test runner
     - the junitxml path in .quarantine/config.yml matches where your runner writes XML
   ```
4. Exits with code **2** (quarantine infrastructure error — not test failures,
   not exit 1).

No state file is written. No GitHub Issue is created. No PR comment is posted.

---

### Scenario 123: quarantine run rerun failure classifies test as unresolved and continues [M9]

**Risk:** When `rerun_command` fails for one test, the entire run aborts,
losing results from the other tests that processed successfully — and
misclassifying an infrastructure failure as a test failure.

**Given** `.quarantine/config.yml` with a `backend` suite where
`rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]`

**And** `bundle` is not available in the CI environment (returns exit 127)

**And** two tests fail the initial run:
- `spec/models/user_spec.rb::User::validates email` — initially fails
- `spec/services/payment_spec.rb::Payment::charges card` — initially fails

**When** CI executes `quarantine run backend`

**Then** quarantine:
1. Parses JUnit XML: 2 failures found.
2. Attempts to rerun `spec/models/user_spec.rb::User::validates email` via
   `bundle exec rspec -e "validates email"` → rerun exits 127, `bundle` not found.
3. Classifies `User::validates email` as **unresolved** (not flaky, not genuine
   failure). Continues — does NOT abort.
4. Attempts to rerun `Payment::charges card` → same failure (exit 127).
5. Classifies `Payment::charges card` as **unresolved**.
6. Writes `.quarantine/backend/results.json` with both tests at
   `status: "unresolved"`, `error: "rerun command failed: exec: 'bundle': executable file not found in $PATH"`,
   and `rerun_exit_code: 127`.
7. Prints to stderr:
   ```
   Error [rerun]: rerun command failed for 'validates email': exec: 'bundle' not found in $PATH
   Error [rerun]: rerun command failed for 'charges card': exec: 'bundle' not found in $PATH
   2 tests could not be retried — rerun command failed.
   ```
8. Posts PR comment noting "2 tests could not be retried."
9. Does NOT create GitHub Issues (tests not confirmed flaky).
10. Exits with code **2** (infrastructure error; no genuine failures).

---

### Scenario 124: quarantine run with genuine failures and rerun failures exits 1 [M9]

**Risk:** When genuine test failures and rerun infrastructure failures both occur,
the CLI exits 2 (infrastructure error) instead of 1 (test failure), masking real
failures from the CI system and allowing broken builds to pass.

**Given** `.quarantine/config.yml` with a `backend` suite

**And** three tests fail the initial run:
- `UserSpec::validates email` — retries pass → classified as **flaky**
- `PaymentSpec::charges card` — rerun command crashes → classified as **unresolved**
- `OrderSpec::calculates total` — fails all 3 retries → classified as **genuine failure**

**When** CI executes `quarantine run backend`

**Then** quarantine:
1. Processes all three tests to completion.
2. Classifies results: 1 flaky, 1 unresolved, 1 genuine failure.
3. Applies exit code priority: genuine failure (exit 1) > unresolved (exit 2) > pass (exit 0).
4. Exits with code **1** — the genuine failure takes priority over the
   unresolved infrastructure error.

The PR comment and results.json accurately report all three outcomes.

---

### Scenario 125: two parallel suite runs operate on separate state files without contention [M9]

**Risk:** Parallel CI steps running `quarantine run backend` and
`quarantine run frontend` contend on the same state file, causing CAS conflicts
and lost quarantine state.

**Given** `.quarantine/config.yml` with `backend` and `frontend` suites

**And** both suites have existing state files on the `quarantine/state` branch:
- `.quarantine/backend/state.json` with 2 quarantined tests
- `.quarantine/frontend/state.json` with 1 quarantined test

**When** two CI jobs run concurrently:
- Job A: `quarantine run backend` (detects a new flaky test in backend)
- Job B: `quarantine run frontend` (detects a new flaky test in frontend)

**Then**:
1. Job A reads and writes only `.quarantine/backend/state.json`. Its CAS
   operation uses the SHA of the backend file.
2. Job B reads and writes only `.quarantine/frontend/state.json`. Its CAS
   operation uses the SHA of the frontend file.
3. The two CAS operations operate on different files — there is **no conflict**
   between them. Neither job waits for or is blocked by the other.
4. After both jobs complete:
   - `.quarantine/backend/state.json` contains 3 quarantined tests.
   - `.quarantine/frontend/state.json` contains 2 quarantined tests.
5. Each job posts its own PR comment:
   - Job A: `<!-- quarantine:backend -->`
   - Job B: `<!-- quarantine:frontend -->`
   Both comments coexist on the PR with no interference.

---

### Scenario 137: zero-failure run still writes results.json and posts PR comment [M9]

**Risk:** When all tests pass, quarantine skips writing `results.json` or posting
a PR comment, leaving the dashboard with gaps in run history and developers with
no confirmation that quarantine ran successfully.

**Given** `.quarantine/config.yml` with a `backend` suite

**And** `.quarantine/backend/state.json` contains 2 quarantined tests

**And** CI runs on a PR

**When** CI executes `quarantine run backend` and all tests pass on the first run
(no failures, nothing to retry)

**Then** quarantine:
1. Reads state file — 2 quarantined tests found.
2. Executes the suite's `command` — all tests pass.
3. Parses JUnit XML — no failures detected.
4. **Still writes** `.quarantine/backend/results.json` (records the run for
   dashboard history, even with zero failures).
5. **Still posts/updates** the PR comment (`<!-- quarantine:backend -->`) showing
   "All tests passed (2 quarantined tests did not run)".
6. Does NOT update the state file (no changes to quarantine state).
7. Does NOT create any GitHub Issues.
8. Exits with code **0**.

---

### Scenario 138: quarantine run --dry-run analyzes existing JUnit XML without executing anything [M9]

**Risk:** A developer who wants to verify quarantine's configuration before
committing to a real CI run has no way to inspect what quarantine would do without
actually running tests and creating issues or PR comments.

**Given** `.quarantine/config.yml` with a `backend` suite and `junitxml: "rspec.xml"`

**And** `.quarantine/backend/state.json` on the state branch contains 1 quarantined
test: `spec/models/user_spec.rb::User::validates email`

**And** the developer previously ran their test suite and `rspec.xml` exists with
3 failures (including the quarantined test and 2 non-quarantined failures)

**When** the developer runs `quarantine run backend --dry-run`

**Then** quarantine:
1. Reads the state file from the state branch.
2. Reads `rspec.xml` from disk (the existing file — quarantine does NOT run the
   test command).
3. Prints an analysis:
   ```
   Dry run — no changes will be made.

   State: 1 quarantined test found.
   JUnit XML: rspec.xml (3 failures, 47 passed)

   Quarantined failures (would be ignored):
     spec/models/user_spec.rb::User::validates email   (#42)

   Non-quarantined failures (would be retried):
     spec/models/order_spec.rb::Order::calculates total
     spec/services/payment_spec.rb::Payment::charges card

   Note: dry run cannot classify flaky vs genuine — retries are not executed.
   ```
4. Does **NOT** execute the test command.
5. Does **NOT** retry any failures.
6. Does **NOT** write or update `.quarantine/backend/state.json`.
7. Does **NOT** write `.quarantine/backend/results.json`.
8. Does **NOT** post or update any PR comment.
9. Does **NOT** create any GitHub Issues.
10. Exits with code **0** (dry-run is always informational).

**And when** no `rspec.xml` exists at the `junitxml` path:

The CLI prints a warning and exits 0:
```
Warning: no JUnit XML files found at 'rspec.xml'.
Run your test suite first to produce JUnit XML output, then re-run with --dry-run.
```

---

### Scenario 148: previously quarantined test executes and fails all retries — failure suppressed, exit 0 [M9]

**Risk:** A test that was quarantined before this run fails deterministically, breaks the build with exit 1, and defeats the purpose of quarantine — which is to prevent known-flaky tests from blocking CI.

**Given** `.quarantine/config.yml` with a `jest-tests` suite

**And** `.quarantine/jest-tests/state.json` on the `quarantine/state` branch contains:
```json
{
  "tests": {
    "src/payment.test.js::PaymentService::should handle charge timeout": {
      "test_id": "src/payment.test.js::PaymentService::should handle charge timeout",
      "issue_number": 42,
      "quarantined_by": "auto"
    }
  }
}
```

**And** GitHub Issue #42 is still open

**When** CI executes `quarantine run jest-tests` and `should handle charge timeout` fails on the initial run and on all 3 retries (it is consistently failing today)

**Then** quarantine:
1. Reads state — finds `should handle charge timeout` quarantined with issue #42 open.
2. Executes the suite command unmodified.
3. Parses JUnit XML — `should handle charge timeout` has `status: "failed"`.
4. Retries the failing test 3 times — it fails every attempt.
5. After retries, **reclassifies** `should handle charge timeout` from `"failed"` to
   `"quarantined"` in the result, setting `original_status: "failed"`.
6. Updates `last_failure_at` and increments `flaky_count` in the state entry.
7. Writes `.quarantine/jest-tests/results.json` with:
   - `status: "quarantined"`, `original_status: "failed"` for the reclassified test
   - `summary.quarantined: 1`, `summary.failed: 0`
8. Posts or updates the PR comment showing the quarantined test.
9. Exits with code **0** — the quarantined failure does not break the build.

**Note:** This replaces the RSpec-specific Scenario 22 behavior. In suite mode, all frameworks use post-execution reclassification rather than framework-specific exclusion flags.

---

### Scenario 149: previously quarantined test executes and passes — reclassified quarantined with original_status: passed [M9]

**Risk:** A quarantined test that happens to pass in this run is silently treated as a clean non-quarantined pass, misrepresenting that the test is still under quarantine management and still has an open issue to resolve.

**Given** `.quarantine/config.yml` with a `jest-tests` suite

**And** `.quarantine/jest-tests/state.json` contains an entry for
`src/auth.test.js::AuthService::should validate expired token` with
`issue_number: 43` (issue open)

**When** CI executes `quarantine run jest-tests` and `should validate expired token`
passes on the initial run (no retries triggered)

**Then** quarantine:
1. Reads state — finds the test quarantined with issue #43 open.
2. Executes the suite command unmodified.
3. Parses JUnit XML — `should validate expired token` has `status: "passed"`.
4. No retries triggered (test passed).
5. **Reclassifies** `should validate expired token` from `"passed"` to
   `"quarantined"` in the result, setting `original_status: "passed"`.
6. Writes `.quarantine/jest-tests/results.json` with:
   - `status: "quarantined"`, `original_status: "passed"` for the test
   - `summary.quarantined: 1`, `summary.passed: 0` (the pass is absorbed into quarantined)
7. Exits with code **0**.

The test remains quarantined until a human closes Issue #43. The `original_status: "passed"` signal informs the developer that the test is stable now and the issue can be reviewed for closure.

---

### Scenario 150: previously quarantined test is flaky again — reclassified quarantined with original_status: flaky [M9]

**Risk:** A quarantined test that fails initially but passes on retry is double-counted as both a newly detected flaky test and a quarantined one, creating a duplicate GitHub Issue and overcounting in the summary.

**Given** `.quarantine/config.yml` with a `jest-tests` suite

**And** `.quarantine/jest-tests/state.json` contains an entry for
`src/cache.test.js::CacheService::should handle eviction under load` with
`issue_number: 44` (issue open) and `flaky_count: 2`

**When** CI executes `quarantine run jest-tests` and `should handle eviction under load`
fails on the initial run but passes on retry 1

**Then** quarantine:
1. Reads state — finds the test quarantined with issue #44 open.
2. Executes the suite command unmodified.
3. Parses JUnit XML — the test has `status: "failed"`.
4. Retries — passes on attempt 1. `BuildWithRetries` classifies the test as `"flaky"`.
5. `addNewFlakyTests` recognizes the test is **already in state**, increments its
   `flaky_count` to 3, and updates `last_failure_at`. It does NOT create a new
   state entry (no duplicate).
6. **Reclassifies** the test from `"flaky"` to `"quarantined"` in the result,
   setting `original_status: "flaky"`.
7. Writes `.quarantine/jest-tests/results.json` with:
   - `status: "quarantined"`, `original_status: "flaky"` for the test
   - `summary.quarantined: 1`, `summary.flaky_detected: 0`
8. Does NOT create a duplicate GitHub Issue (test already has issue #44).
9. Exits with code **0**.

---

### Scenario 151: quarantined failure and genuine failure in the same run — genuine failure drives exit 1 [M9]

**Risk:** A quarantined test failure masks a genuine failure: the quarantine suppression exits 0 when there is also a real broken test, hiding the regression from the developer.

**Given** `.quarantine/config.yml` with a `jest-tests` suite

**And** `.quarantine/jest-tests/state.json` contains an entry for
`src/payment.test.js::PaymentService::should process refund` with
`issue_number: 45` (issue open)

**And** `src/checkout.test.js::CheckoutService::should apply discount` is NOT
quarantined and has a genuine bug

**When** CI executes `quarantine run jest-tests` and:
- `should process refund` fails all 3 retries (quarantined, known flaky)
- `should apply discount` fails all 3 retries (genuine failure, not quarantined)

**Then** quarantine:
1. Reads state — `should process refund` is quarantined (issue #45 open).
   `should apply discount` is not in state.
2. Executes the suite command unmodified. Both tests fail.
3. Retries both. Both fail all retries.
4. `BuildWithRetries` classifies both as `"failed"`.
5. Reclassifies `should process refund` to `"quarantined"` (original_status: "failed").
   `should apply discount` remains `"failed"`.
6. Writes `.quarantine/jest-tests/results.json` with:
   - `summary.quarantined: 1`, `summary.failed: 1`
7. Creates a GitHub Issue for `should apply discount` (newly detected genuine failure
   that may become flaky on a future run — but that's handled by retry logic separately).
   Actually: `should apply discount` failed all retries so it is a genuine failure,
   NOT classified as flaky. No issue is created for genuine failures.
8. Posts PR comment showing 1 quarantined test and 1 genuine failure.
9. Exits with code **1** — the genuine failure drives the exit code.

---

### Scenario 152: quarantined test is skipped — skipped status is preserved, not reclassified [M9]

**Risk:** A quarantined test that is skipped (e.g., marked `xit` or `pending` in the test file) is reclassified to `"quarantined"` and removed from the skipped count, producing inaccurate summary totals and masking that the test runner skipped it.

**Given** `.quarantine/config.yml` with a `jest-tests` suite

**And** `.quarantine/jest-tests/state.json` contains an entry for
`src/migration.test.js::MigrationService::should migrate v2 schema` with
`issue_number: 46` (issue open)

**And** the test is marked `xit(...)` (pending) in the source file, so Jest
reports it as skipped in JUnit XML

**When** CI executes `quarantine run jest-tests`

**Then** quarantine:
1. Reads state — test is quarantined with issue #46 open.
2. Executes the suite command unmodified.
3. Parses JUnit XML — `should migrate v2 schema` has `status: "skipped"`.
4. **Does NOT reclassify** the test — `"skipped"` is preserved as-is.
   Only `"failed"`, `"flaky"`, and `"passed"` outcomes are reclassified to
   `"quarantined"` when the test is in the quarantine state.
5. Writes `.quarantine/jest-tests/results.json` with:
   - `status: "skipped"` (unchanged) for the test
   - `summary.skipped: 1`, `summary.quarantined: 0` (the quarantined count reflects
     tests that actually ran and were reclassified)
6. Exits with code **0**.

---

### Scenario 153: all tests in the suite are quarantined and all fail — exit 0 with warning [M9]

**Risk:** When every test in a run is quarantined and fails, reclassification correctly exits 0, but without a warning the developer has no signal that the entire suite was silently suppressed — a quarantine state that has grown to cover all tests is itself a problem worth surfacing.

**Given** `.quarantine/config.yml` with a `jest-tests` suite

**And** `.quarantine/jest-tests/state.json` contains entries for all three tests in
the suite: `src/a.test.js::A::test1`, `src/b.test.js::B::test2`,
`src/c.test.js::C::test3` — all with open issues

**When** CI executes `quarantine run jest-tests` and all three tests fail on the
initial run and fail all 3 retries

**Then** quarantine:
1. Reads state — all three tests are quarantined.
2. Executes suite command. All three tests fail.
3. Parses JUnit XML — three failures.
4. Retries all three — all fail every retry. `BuildWithRetries` classifies all as
   `"failed"`.
5. Reclassifies all three to `"quarantined"` (original_status: "failed" for each).
6. Detects that `summary.quarantined == summary.total` (all tests are quarantined).
7. Prints warning to stderr:
   ```
   [quarantine] WARNING: All tests are quarantined. The entire test suite was skipped. Review and close resolved quarantine issues.
   ```
8. Writes `.quarantine/jest-tests/results.json` with `summary.quarantined: 3`,
   `summary.failed: 0`.
9. Exits with code **0** (all failures were suppressed — no genuine failures remain).

---

### Scenario 154: quarantined failure and unresolved test with no genuine failure — exit 2 [M9]

**Risk:** Reclassifying a quarantined failure to `"quarantined"` accidentally masks a concurrent infrastructure error (unresolved rerun), causing the CI step to exit 0 when it should exit 2 to signal that the rerun command is broken.

**Given** `.quarantine/config.yml` with a `backend` suite

**And** `.quarantine/backend/state.json` contains an entry for
`spec/models/user_spec.rb::User::validates email` with `issue_number: 50` (issue open)

**And** the rerun command (`bundle exec rspec -e "{name}"`) crashes with exit 127
(binary not found) for every retry

**When** CI executes `quarantine run backend` and two tests fail initially:
- `spec/models/user_spec.rb::User::validates email` (quarantined) — fails initial
  run, rerun crashes → `"unresolved"`
- `spec/services/order_spec.rb::Order::calculates total` (not quarantined) — fails
  initial run, rerun crashes → `"unresolved"`

**Then** quarantine:
1. Reads state — `validates email` is quarantined (issue #50 open).
   `calculates total` is not quarantined.
2. Executes suite command. Both tests fail.
3. Retries both — rerun crashes for both. Both classified as `"unresolved"`.
4. `ReclassifyQuarantinedTests` checks `validates email` — status is `"unresolved"`,
   which is **not reclassified** (only `"failed"`, `"flaky"`, `"passed"` are
   reclassified). Leaves it as `"unresolved"`.
5. Writes `.quarantine/backend/results.json` with:
   - `summary.unresolved: 2`, `summary.quarantined: 0`, `summary.failed: 0`
6. Exits with code **2** — unresolved tests drive exit 2; reclassification does not
   suppress infrastructure errors.

---

### Scenario 155: newly detected flaky test and pre-existing quarantined test in the same run [M9]

**Risk:** The pre-run snapshot that prevents newly detected flaky tests from being reclassified is missed or applied incorrectly, causing a new flaky test to silently become `"quarantined"` on its first detection rather than `"flaky"` — hiding the new discovery from the developer.

**Given** `.quarantine/config.yml` with a `jest-tests` suite

**And** `.quarantine/jest-tests/state.json` contains an entry for
`src/payment.test.js::PaymentService::should process refund` with
`issue_number: 51` (issue open, quarantined before this run)

**And** `src/auth.test.js::AuthService::should validate session` is NOT in the
quarantine state (first time it has been observed as flaky)

**When** CI executes `quarantine run jest-tests` and:
- `should process refund` (pre-existing quarantined) fails all 3 retries
- `should validate session` (newly flaky) fails the initial run but passes on retry 1

**Then** quarantine:
1. Reads state — `should process refund` is quarantined. Snapshots pre-run IDs:
   `{"src/payment.test.js::PaymentService::should process refund": true}`.
2. Executes suite command.
3. Parses JUnit XML — both tests show `"failed"`.
4. Retries both. `should process refund` fails all 3 retries (classified `"failed"`).
   `should validate session` passes on retry 1 (classified `"flaky"`).
5. `addNewFlakyTests` adds `should validate session` to the quarantine state (it is
   newly flaky). `should process refund` is already in state — its `flaky_count` and
   `last_failure_at` are updated.
6. `ReclassifyQuarantinedTests` uses the **pre-run snapshot**:
   - `should process refund` is in the snapshot → reclassified to `"quarantined"`
     (original_status: "failed").
   - `should validate session` is **NOT** in the snapshot (it was added to state
     during step 5, after the snapshot was taken) → stays `"flaky"`.
7. Writes `.quarantine/jest-tests/results.json` with:
   - `should process refund`: `status: "quarantined"`, `original_status: "failed"`
   - `should validate session`: `status: "flaky"` (visible as a new discovery)
   - `summary.quarantined: 1`, `summary.flaky_detected: 1`, `summary.failed: 0`
8. Creates a GitHub Issue for `should validate session` (newly detected flaky).
   Does NOT create a duplicate issue for `should process refund` (already quarantined).
9. Exits with code **0**.

---

### Scenario 156: multiple quarantined tests with mixed execution outcomes — all reclassified, exit 0 [M9]

**Risk:** Reclassification only handles one execution outcome (e.g., `"failed"`) and silently leaves quarantined tests with `"passed"` or `"flaky"` outcomes with incorrect statuses and summary counts, producing misleading results.

**Given** `.quarantine/config.yml` with a `jest-tests` suite

**And** `.quarantine/jest-tests/state.json` contains entries for three tests, all
with open issues:
- `src/a.test.js::A::test1` (quarantined)
- `src/b.test.js::B::test2` (quarantined)
- `src/c.test.js::C::test3` (quarantined)

**When** CI executes `quarantine run jest-tests` and:
- `test1` passes on the initial run (no retry)
- `test2` fails the initial run but passes on retry 1 (flaky)
- `test3` fails the initial run and all 3 retries (consistently failing today)

**Then** quarantine:
1. Reads state — all three tests are quarantined.
2. Executes suite command.
3. `BuildWithRetries` classifies: `test1` → `"passed"`, `test2` → `"flaky"`,
   `test3` → `"failed"`.
4. `ReclassifyQuarantinedTests` reclassifies all three:
   - `test1`: `status: "quarantined"`, `original_status: "passed"`
   - `test2`: `status: "quarantined"`, `original_status: "flaky"`
   - `test3`: `status: "quarantined"`, `original_status: "failed"`
5. Writes `.quarantine/jest-tests/results.json` with:
   - `summary.quarantined: 3`, `summary.passed: 0`, `summary.flaky_detected: 0`,
     `summary.failed: 0`
6. Exits with code **0** — all three failures/outcomes are suppressed because all
   three tests are known-quarantined.
