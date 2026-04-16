# ADR-034: Post-Execution Reclassification as Universal Quarantine Fallback

**Status:** Accepted
**Date:** 2026-04-15

## Context

ADR-003 documented a framework-adaptive quarantine mechanism: Jest and Vitest
used pre-execution exclusion (CLI appended `--testPathIgnorePatterns` /
`--testNamePattern` to the test command), while RSpec used post-execution
filtering (quarantined failures suppressed from the exit code after the run).

ADR-031 (command execution model) changed this: suite commands now run
unmodified via `exec.Command`. The CLI never appends flags. The rationale was
that augmenting the user's command breaks wrapper scripts (npm, rake, Makefile
targets) that do not pass unknown flags through, and creates two sources of
truth for how tests run.

This meant the pre-execution exclusion mechanism from ADR-003 was removed in
the M9 core conversion. However, the post-execution suppression for RSpec was
not generalized to replace it. The result was a gap: quarantined tests ran as
before, but their failures were no longer suppressed from the exit code. A test
quarantined in the state file since a previous run could still break the build
with exit 1 — defeating the purpose of quarantine.

This was observed directly in the fixture CI run at
`github.com/mycargus/quarantine-test-fixture` (run #38, 2026-04-15):
`CacheService should handle eviction under load` had been in the quarantine
state since 2026-04-14 with an open issue, yet caused exit 1 because no
suppression was applied.

## Decision

**Apply post-execution reclassification universally in suite mode, regardless
of framework.**

After the test command executes and all retries complete, the CLI cross-references
each test result against the set of test IDs that were quarantined at the start
of the run (the "pre-run snapshot"). For any test in the pre-run snapshot:

- `"failed"` → reclassified to `"quarantined"` (`original_status: "failed"`)
- `"flaky"` → reclassified to `"quarantined"` (`original_status: "flaky"`)
- `"passed"` → reclassified to `"quarantined"` (`original_status: "passed"`)
- `"skipped"` → left unchanged (skipped is orthogonal — the test never ran)
- `"unresolved"` → left unchanged (infrastructure failure, not a test result)

The summary counts are adjusted accordingly (`Failed`/`FlakyDetected`/`Passed`
decremented, `Quarantined` incremented). `resolveExitCode` then sees
`Failed == 0` for runs where all failures were quarantined, and returns 0.

**Pre-run snapshot:** The snapshot of quarantined IDs is taken after
`removeUnquarantinedTests` (closed-issue check) but before `addNewFlakyTests`.
This ensures that tests newly detected as flaky in the current run are NOT
reclassified — their `"flaky"` status is preserved as a visible first-detection
signal. On the next run, they will be in the pre-run snapshot and reclassified.

**Unquarantine path is unchanged:** The only way to remove a test from quarantine
remains closing its GitHub Issue (ADR-017). `removeUnquarantinedTests` handles
this at run start.

**Relationship to pre-execution exclusion:** Post-execution reclassification is
the universal fallback, not a replacement for pre-execution exclusion.
Frameworks that support clean exclusion (Jest via `--testPathIgnorePatterns`,
Vitest via `--exclude`) should still skip quarantined tests before execution
when that capability is added — it saves CI time and money by avoiding
unnecessary test runs. Reclassification guarantees correct exit codes for
frameworks where pre-execution exclusion is impractical (RSpec, custom runners),
and serves as a safety net when exclusion is incomplete (e.g., individual test
exclusion within a shared file). The two mechanisms are complementary layers:
exclusion is the optimization, reclassification is the correctness guarantee.

This supersedes ADR-003's mechanism for the suite mode execution model. ADR-003's
framework-adaptive approach was specific to the old `quarantine run -- <command>`
model where the CLI constructed and augmented the test command. ADR-003's
philosophy — "automatic pre-execution exclusion is a convenience applied where
it is cheap and clean; the core mechanism is the quarantine list + exit code
suppression" — remains the guiding principle. This ADR implements that core
mechanism.

## Alternatives Considered

- **Reinstate pre-execution exclusion for suite mode via `quarantined-files.txt`.**
  Write quarantined file paths before execution; require users to reference the
  file in their test command config. Rejected: requires user-side configuration
  changes (reading and applying the file), the file paths in JUnit XML are often
  classnames rather than real paths (making the file useless without parser
  improvements), and it violates ADR-031's principle that the suite command runs
  unmodified.

- **Restore framework-specific flag injection for known frameworks (Jest/Vitest).**
  Re-implement `--testPathIgnorePatterns` / `--testNamePattern` augmentation for
  known frameworks while keeping RSpec post-execution. Rejected: contradicts
  ADR-031 (commands never modified) and ADR-030 (framework-agnostic design).
  Breaks wrapper scripts that don't forward unknown flags.

- **Do nothing; document the gap as a known limitation.**
  Quarantined tests run and may break the build. Users can work around this by
  closing the issue before the run. Rejected: this is the quarantine tool's core
  promise — quarantined tests must not break the build. A known gap that defeats
  the tool's purpose is not acceptable.

- **Re-run quarantined tests in isolation to confirm they are still flaky before
  suppressing.** Add a dedicated re-run pass for quarantined tests; only suppress
  if they pass on at least one re-run. Rejected: adds CI time and contradicts
  ADR-017 (no auto-healing). A test that fails consistently is still quarantined
  — it may have regressed, but the appropriate response is for a human to close
  the issue and let the build fail, not for quarantine to decide silently.

## Consequences

**Positive:**

- (+) Quarantine's core promise is restored: tests in the quarantine state do not
  break the build, regardless of framework or execution outcome.
- (+) Universal — works identically for Jest, Vitest, RSpec, and any other
  framework that produces JUnit XML.
- (+) No changes to the user's test command or configuration required.
- (+) `original_status` in `results.json` preserves execution signal for the
  dashboard — consumers can see that a quarantined test passed, failed, or was
  flaky without it affecting the CI exit code.
- (+) The pre-run snapshot boundary keeps first-detection semantics clean:
  a newly quarantined test shows as `"flaky"` on the run it was first detected,
  then as `"quarantined"` on all subsequent runs.

**Negative:**

- (-) Quarantined tests still execute and consume CI time. Pre-execution exclusion
  is the intended optimization for frameworks that support it (Jest, Vitest) and
  should be added when feasible — it saves real money on CI bills. This ADR does
  not block that work; it provides the correctness guarantee that exclusion alone
  cannot (RSpec, partial-file cases, custom runners).
- (-) A quarantined test that has genuinely regressed (now deterministically
  broken) will have its failure suppressed until a human closes the issue. The
  `original_status: "failed"` in `results.json` and the `Quarantined` count in
  the PR comment provide a signal, but it is less visible than a build failure.
  Mitigated by the "all tests quarantined" warning when the entire suite is
  suppressed.
- (-) Newly detected flaky tests are visible as `"flaky"` for only one run before
  becoming `"quarantined"`. Developers who don't check the PR comment on the
  detection run may not notice the first-detection event. Mitigated by GitHub
  Issue creation on detection.
