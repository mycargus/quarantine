# ADR-030: Framework-Agnostic Design (Supersedes ADR-016)

**Status:** Accepted
**Date:** 2026-04-10

## Context

ADR-016 scoped v1 to three test frameworks (RSpec, Jest, Vitest) and gated CLI
functionality on a `framework` field in `quarantine.yml`. The `framework` field
bundles three concerns: a default JUnit XML path, a default rerun command, and a
default exclusion mechanism. In practice, users with custom setups (pnpm, bun,
monorepos, custom wrappers) override every value it provides.

The multi-suite support plan (`docs/plans/multi-suite-support.md`) replaces the
single `framework` field with named test suites, each specifying its own
`command`, `junitxml`, and `rerun_command`. Quarantine does not need to know the
framework --- it needs concrete values.

## Decision

Remove the `framework` field from the configuration. Quarantine is
framework-agnostic: any test runner that produces JUnit XML works without code
changes. Known-runner defaults (Jest, RSpec, Vitest) are advisory only ---
suggested during `quarantine suite add` as a convenience, not enforced.

This supersedes ADR-016. The supported-frameworks list no longer gates
functionality. The "v1 framework scope" concept is replaced by "any framework
that produces JUnit XML."

## Alternatives Considered

- **Keep `framework` as optional metadata:** Adds no value. The suite name
  (e.g., "backend", "frontend") is more meaningful than the framework name for
  analytics and display.
- **Amend ADR-016 instead of superseding:** The core decision of ADR-016
  (limit to three frameworks) is reversed, not refined. A supersession is more
  accurate.

## Consequences

- (+) Any test runner that produces JUnit XML works on day one.
- (+) No code changes needed to support new frameworks (pytest, Go, etc.).
- (+) Removes the leaky `framework` abstraction from config and code.
- (+) `quarantine init` no longer asks "which framework?" --- replaced by
  `quarantine suite add` which asks for concrete values.
- (-) No framework-specific validation (e.g., checking jest-junit is installed).
  This moves to advisory checks during `quarantine suite add` for known runners.
- (-) Results schema loses the `framework` field. Dashboard analytics that
  previously grouped by framework must use suite name instead.
