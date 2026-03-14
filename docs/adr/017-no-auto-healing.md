# ADR-017: No Auto-Healing of Flaky Tests

**Status:** Accepted
**Date:** 2026-03-14

## Context

When a test is quarantined, should the system periodically check if the test has become stable and automatically unquarantine it? Or should unquarantine only happen when a human closes the associated issue?

## Decision

Quarantine does NOT attempt to heal or auto-unquarantine flaky tests. The tool detects and quarantines only. The sole signal to unquarantine a test is closure of its associated GitHub Issue by a human. Already-quarantined tests are NOT re-run during CI builds -- they are immediately suppressed.

Rationale:

- "Healing" (fixing the root cause of flakiness) is a fundamentally different problem from "managing" (keeping builds green while flakiness exists). This tool manages.
- Flaky tests are non-deterministic by definition. A test passing N times in a row does not prove it is fixed -- it may flake on the N+1th run. Only a human reviewing the root cause can determine if the fix is real.
- Re-running quarantined tests wastes CI time for no actionable signal.
- Requiring human issue closure creates accountability -- someone must own the fix and explicitly declare it done.
- The v2 code sync adapter (automated PRs with skip markers) reinforces this: quarantined tests are visibly marked in code until a human removes the marker.

## Alternatives Considered

- **Auto-unquarantine after N consecutive passes:** Rejected because flaky tests can pass many times before failing again. This would create a cycle of quarantine -> auto-unquarantine -> flake again -> quarantine, wasting CI resources and creating noise.
- **Re-run quarantined tests as a "health check" (but still suppress failures):** Rejected because it adds CI time with no user benefit. If the test passes, nothing changes (still quarantined until issue closed). If it fails, nothing changes (still quarantined). The information gain is zero.
- **Hybrid: auto-unquarantine + re-quarantine on next flake:** Rejected because the oscillation creates confusing state changes and noisy issue/PR activity.

## Consequences

- (+) Simple, predictable behavior -- quarantined tests stay quarantined until a human acts.
- (+) No wasted CI time re-running quarantined tests.
- (+) Clear accountability -- issue closure is an explicit human decision.
- (+) No oscillation between quarantined/unquarantined states.
- (-) A fixed test remains quarantined until someone remembers to close the issue. Mitigated by: dashboard visibility of quarantine duration, and future notifications when tests have been quarantined for unusually long periods (v2+).
- (-) No automated signal that a quarantined test might be fixed. Mitigated by: a developer can close the issue at any time, and the test will be unquarantined on the next CI run.
