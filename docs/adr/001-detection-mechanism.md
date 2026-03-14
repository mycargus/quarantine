# ADR-001: Flaky Test Detection via Re-run Failures

**Status:** Accepted
**Date:** 2026-03-14

## Context

Need a mechanism to identify flaky (non-deterministic) tests in CI. Options were: (A) re-run failures -- when a test fails, re-run it N times, if it passes on retry it is flaky; (B) historical analysis -- track results over time, flag tests that flip-flop; (C) both.

## Decision

Option A -- re-run failures. Default 3 retries, configurable in quarantine.yml.

## Alternatives Considered

- **Historical analysis (B):** Requires multiple builds before detection, adds infrastructure complexity. Detection is not immediate and depends on accumulated data.
- **Combined approach (C):** More comprehensive but over-engineered for v1. Historical analysis will be layered on in v2 using data that accumulates naturally in the dashboard.

## Consequences

**Positive:**
- Simplest integration -- detection is self-contained in a single CI run.
- Immediate results, no cold-start period.
- No server dependency for detection.

**Negative:**
- Will not catch tests that flake infrequently (only on different runs, not within a single run).
- Retries add time to CI runs. Mitigated by only retrying failures, not the full suite.
