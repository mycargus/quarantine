# ADR-003: Wrapper/Shim Quarantine Mechanism

**Status:** Accepted
**Date:** 2026-03-14

## Context

Need a mechanism to quarantine flaky tests so they stop blocking CI. Options: (A) code modification -- edit source to add @skip annotations; (B) external exclusion list -- manifest file that a test runner plugin reads; (C) wrapper/shim -- CLI wraps the test runner, filters results post-execution.

## Decision

Option C -- wrapper/shim for v1. The CLI wraps the user's test command (`quarantine run --junitxml="results/*.xml" -- jest --ci --reporters=default --reporters=jest-junit`), reads the quarantine list from GitHub, runs tests as-is, parses results (merging multiple XML files if a glob pattern is provided), suppresses quarantined failures (rewrites them as skips in JUnit XML), and determines the exit code.

v2 adds a code sync adapter (automated PRs to add framework-specific skip markers in source) as a complementary feature.

## Alternatives Considered

- **Code modification (A):** Invasive approach that requires understanding every framework's skip syntax and creates noisy diffs in version control.
- **External exclusion list (B):** Requires a test runner plugin per framework, demanding significant integration work for each supported ecosystem.

Both were rejected for v1 due to high adoption friction.

## Consequences

**Positive:**
- Zero code changes to the test suite.
- Zero test runner plugins required.
- Works with any framework that emits JUnit XML.
- User's test command is untouched -- quarantine is just a prefix.

**Negative:**
- Quarantined tests are invisible to developers reading source code. Mitigated by the v2 code sync adapter.
- Re-running individual tests requires knowing framework-specific invocation patterns. Mitigated by a baked-in rerun command registry for common frameworks and user-overridable config. v1 frameworks (RSpec, Jest, Vitest) with key caveats:
  - **rspec:** `rspec_junit_formatter` does NOT include line numbers in JUnit XML, so `rspec {file}:{line}` is unworkable. The CLI uses `rspec -e "{name}"` with the test description from the `name` attribute instead. Note: this may match multiple tests with similar names.
  - **jest:** Rerun via `jest --testNamePattern "{name}"`. Requires `jest-junit` for JUnit XML output.
  - **vitest:** Rerun via `vitest run --reporter=junit {test_file} -t "{test_name}"`. Vitest has built-in JUnit XML support via `--reporter=junit` (no third-party package needed). Uses the same `-t` flag as Jest for filtering by test name.
  - [v2+] **pytest:** JUnit XML classnames use dots (e.g., `tests.test_payment`) which must be converted to path format (`tests/test_payment.py::test_name`). The CLI transforms dot-separated classnames to file paths and appends `.py`.
  - [v2+] **go test, maven:** Standard rerun patterns apply.
