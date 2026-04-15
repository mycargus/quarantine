# ADR-033: Quarantine Owns Retries

**Status:** Accepted
**Date:** 2026-04-14

## Context

Flaky test detection requires seeing both the failure and the subsequent pass.
Test frameworks (Jest, RSpec, Vitest) all support built-in retry mechanisms, but
they report only the **final** test result in JUnit XML. If a flaky test fails
once then passes on a framework-level retry, the XML shows a pass --- the
failure is invisible. Quarantine cannot detect flakiness from the XML alone.

Additionally, RSpec's community retry gem (`rspec-retry`) was archived in July
2025. There is no maintained retry solution for RSpec.

## Decision

Quarantine owns the retry loop. The flow is:

1. Run the test suite (via the suite's `command`)
2. Parse JUnit XML for failures
3. Rerun individual failures using the suite's `rerun_command`
4. Classify: passes on any retry = flaky, fails all retries = genuine failure

Framework-level retries silently defeat flakiness detection. Documentation alone
is insufficient --- users commonly configure retries without realizing the
implication.

### Enforcement: `quarantine doctor` only

Retry detection is a best-effort heuristic (static string/regex matching) that
produces both false positives and false negatives (dynamic config, per-test
calls, unknown runners). Therefore it belongs in the diagnostic command
(`quarantine doctor`), not in the runtime path (`quarantine run`).

`quarantine doctor` checks these locations and emits **warnings** (never errors):

| Runner | Check | Location |
|--------|-------|----------|
| Jest | `retryTimes` in config | `jest.config.{js,ts,mjs}`, top-level `*.test.{js,ts}` |
| Vitest | `retry` in config | `vitest.config.{js,ts,mjs}` |
| RSpec | `rspec-retry` gem | `Gemfile`, `Gemfile.lock` |

Static detection refinements to reduce false positives:
- **Jest:** `/retryTimes\(\s*[1-9]/` --- `retryTimes(0)` is a no-op, not warned.
- **Vitest:** `/retry:\s*[1-9]/` --- only non-zero values warned.
- **RSpec:** Presence of `rspec-retry` in Gemfile is sufficient.

### Detection scope and limitations

Static detection is best-effort for known runners. It does NOT:
- Parse JavaScript/TypeScript ASTs
- Detect per-test `jest.retryTimes(N)` calls
- Detect retry config in monorepo subdirectories
- Check unknown test runners' retry mechanisms

For unknown runners, documentation is the only mitigation.

## Alternatives Considered

- **Trust framework retries and parse retry metadata from XML:** JUnit XML has
  no standard retry metadata. Jest, RSpec, and Vitest all report only the final
  result. There is nothing to parse.
- **Block `quarantine run` when framework retries are detected:** Heuristic
  detection has false positives (dynamic config, per-test calls). Blocking CI
  runs on a heuristic is too aggressive. Warnings in `doctor` strike the right
  balance.
- **Add a `retry_detection: false` config field per suite:** Adds config surface
  area for a diagnostic-only feature. Users who see persistent false positives
  can ignore the warning --- it does not block anything.

## Consequences

- (+) Quarantine sees every failure and every retry result, enabling accurate
  flaky detection.
- (+) Works with any test runner that produces JUnit XML, regardless of its
  retry capabilities.
- (+) `doctor` warnings catch the most common misconfiguration before CI runs.
- (-) Users must disable framework-level retries. This is a setup requirement
  documented in `quarantine init` output and `quarantine doctor` warnings.
- (-) Static detection cannot catch all retry configurations. Dynamic config
  and per-test retries are invisible to heuristics.
