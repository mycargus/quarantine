# ADR-003: Framework-Adaptive Quarantine Mechanism

**Status:** Amended (2nd amendment)
**Date:** 2026-03-17 (originally accepted 2026-03-14, first amendment 2026-03-17)

## Context

Need a mechanism to quarantine flaky tests so they stop blocking CI. Options: (A) code modification -- edit source to add @skip annotations; (B) external exclusion list -- manifest file that a test runner plugin reads; (C) wrapper/shim -- CLI wraps the test runner, filters results post-execution; (D) pre-execution exclusion -- CLI constructs the test command to skip quarantined tests entirely using framework-specific exclusion flags.

After implementing the first amendment (pure pre-execution exclusion), we discovered that RSpec has no viable pre-execution exclusion mechanism without code changes. `--tag ~quarantine` requires adding tags to source code. File-level exclusion over-excludes (quarantining one test removes the entire file). `rspec_junit_formatter` does not include line numbers in JUnit XML, so `file:line` targeting is impossible.

## Decision

Framework-adaptive quarantine enforcement. The mechanism varies by framework:

- **Jest:** Pre-execution exclusion via `--testPathIgnorePatterns` and `--testNamePattern` (inverted). Quarantined tests never execute.
- **Vitest:** Pre-execution exclusion via `--exclude` patterns. Quarantined tests never execute.
- **RSpec:** Post-execution result filtering. Quarantined tests run normally and produce JUnit XML entries. The CLI checks failures against the quarantine list and suppresses them from the exit code. No JUnit XML rewriting -- the original XML is untouched.

In all cases:
- The CLI reads the quarantine list from GitHub before execution.
- The exit code reflects only non-quarantined, non-flaky failures. Exit 0 if all failures are quarantined or flaky.
- PR comments, GitHub Issues, and results artifacts report quarantined test status to incentivize developers to fix, skip, or remove flaky tests.
- There is no JUnit XML rewriting in any framework.

The philosophy: automatic pre-execution exclusion is a convenience applied where it is cheap and clean. The core mechanism is the quarantine list + exit code suppression + developer notifications. The goal is not to hide flaky tests forever, but to unblock CI while pushing developers to take action.

v2 adds a code sync adapter (automated PRs to add framework-specific skip markers in source) as a complementary feature, which also enables RSpec pre-execution exclusion via tags.

## Alternatives Considered

- **Code modification (A):** Invasive approach that requires understanding every framework's skip syntax and creates noisy diffs in version control.
- **External exclusion list (B):** Requires a test runner plugin per framework, demanding significant integration work for each supported ecosystem.
- **Post-execution suppression via JUnit XML rewriting (C, original decision):** Requires XML manipulation logic that adds complexity and fragility. Rejected.
- **Pure pre-execution exclusion for all frameworks (D, first amendment):** Clean for Jest and Vitest, but forces bad tradeoffs for RSpec (file-level over-exclusion or code changes). Replaced by this framework-adaptive approach.

Both A and B were rejected for v1 due to high adoption friction. C was rejected due to XML rewriting complexity. D was refined into the current hybrid approach because not all frameworks support clean pre-execution exclusion.

## Consequences

**Positive:**
- Zero code changes to the test suite.
- Zero test runner plugins required.
- Jest and Vitest quarantined tests do not execute at all, saving CI time.
- RSpec quarantined tests run but do not break the build -- no over-exclusion, no code changes required.
- No JUnit XML rewriting logic needed in any framework.
- Developer notifications (PR comments, issues) create pressure to fix flaky tests rather than letting them rot in quarantine.
- User's test command is augmented transparently -- quarantine is still just a prefix.

**Negative:**
- RSpec quarantined tests still consume CI time (they execute even though their failures are suppressed). Mitigated by v2 code sync adapter which enables tag-based exclusion.
- Requires framework-specific knowledge: exclusion flags for Jest/Vitest, result filtering logic for RSpec. Manageable for three frameworks but scales less well than a framework-agnostic approach.
- Quarantined tests are invisible to developers reading source code. Mitigated by PR comments, GitHub Issues, and the v2 code sync adapter.
- Re-running individual tests requires knowing framework-specific invocation patterns. Mitigated by a baked-in rerun command registry for common frameworks and user-overridable config. v1 frameworks (RSpec, Jest, Vitest) with key caveats:
  - **rspec:** `rspec_junit_formatter` does NOT include line numbers in JUnit XML, so `rspec {file}:{line}` is unworkable. The CLI uses `rspec -e "{name}"` with the test description from the `name` attribute instead. Note: this may match multiple tests with similar names.
  - **jest:** Rerun via `jest --testNamePattern "{name}"`. Requires `jest-junit` for JUnit XML output.
  - **vitest:** Rerun via `vitest run --reporter=junit {test_file} -t "{test_name}"`. Vitest has built-in JUnit XML support via `--reporter=junit` (no third-party package needed). Uses the same `-t` flag as Jest for filtering by test name.
  - [v2+] **pytest:** JUnit XML classnames use dots (e.g., `tests.test_payment`) which must be converted to path format (`tests/test_payment.py::test_name`). The CLI transforms dot-separated classnames to file paths and appends `.py`.
  - [v2+] **go test, maven:** Standard rerun patterns apply.
