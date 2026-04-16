# ADR-035: Exclude Quarantined Tests via Framework-Native Filtering When Supported

**Status:** Proposed
**Date:** 2026-04-15

## Context

ADR-034 established post-execution reclassification as the universal correctness
guarantee: quarantined test failures are suppressed from the exit code regardless
of framework. However, reclassification does not save CI time or money — the
tests still execute.

ADR-031 established that quarantine never modifies the test command. This rules
out the old approach from ADR-003 where the CLI appended framework-specific
exclusion flags (`--testPathIgnorePatterns`, `--testNamePattern`) to the command.

Many test frameworks have native mechanisms that can filter tests by tag or
metadata without requiring command modification:

- **RSpec:** `--tag ~quarantine` excludes tests annotated with `:quarantine`.
  The filter is part of the user's suite command; the tag is metadata on the test.
- **pytest:** `-m "not quarantine"` excludes tests marked with
  `@pytest.mark.quarantine`. Same pattern: filter in command, mark on test.
- **Go:** `go test -run` patterns or build tags can exclude specific tests.

Some frameworks lack a clean tag-based exclusion mechanism (Jest and Vitest have
no tag system — only `test.skip()` which permanently alters test behavior for
all contexts, including local development). These frameworks would continue to
rely on ADR-034's post-execution reclassification until a suitable mechanism
exists.

## Decision

**When a test framework supports tag-based filtering, quarantined tests should
carry a framework-native tag (e.g., `:quarantine` for RSpec, `@pytest.mark.quarantine`
for pytest) that the test runner's own filtering mechanism uses to exclude them
from execution.**

The key properties of this approach:

1. **The suite command is configured once by the user** to exclude the quarantine
   tag (e.g., `rspec --tag ~quarantine`). Quarantine does not modify the command
   at runtime. This is consistent with ADR-031.

2. **The quarantine state on the `quarantine/state` branch remains the single
   source of truth.** Tags in source are a derived optimization — a signal to the
   test runner. If the tag is absent or stale, ADR-034's post-execution
   reclassification ensures correctness.

3. **Tags must not alter test behavior outside of quarantine-aware runs.** A
   developer running `rspec` without `--tag ~quarantine` should still execute the
   test. This rules out `test.skip()` / `xit()` as a tagging mechanism — those
   skip the test unconditionally for everyone, including developers debugging
   locally.

4. **ADR-034 remains the universal fallback.** Even with tag-based exclusion,
   post-execution reclassification still runs to handle: frameworks without tag
   support, stale tags, and partial-file exclusion gaps.

**How tags are applied to tests (the delivery mechanism) is deliberately not
decided here.** Options include automated PRs, a CLI sync command, CI-time
transient injection, or manual tagging by developers. Each has different
trade-offs around permissions, latency, and workflow integration. The delivery
mechanism should be decided in a separate ADR or during milestone planning.

## Alternatives Considered

- **CLI appends framework-specific exclusion flags to the test command.**
  Rejected: violates ADR-031 (commands never modified). Breaks wrapper scripts
  that don't forward unknown flags.

- **Rely solely on post-execution reclassification (ADR-034) indefinitely.**
  Correctness is preserved but CI time and cost are not optimized. Quarantined
  tests that consistently fail consume the same resources as healthy tests.
  Acceptable as the v1 steady state but not as the long-term design.

- **Use `test.skip()` / `xit()` as the framework-native marker for Jest/Vitest.**
  Rejected: these unconditionally skip the test for everyone, including
  developers running tests locally. A developer who wants to debug a quarantined
  test cannot run it without undoing the skip. This conflates "excluded in CI
  because flaky" with "disabled everywhere."

- **Generate a wrapper config file (e.g., Jest config that filters by name).**
  Amounts to command augmentation via side-channel. Fragments the user's test
  configuration. Framework-specific config generation is brittle across
  framework versions. Rejected.

## Consequences

**Positive:**

- (+) Quarantined tests do not execute in CI — reduces CI time and compute cost
  for frameworks that support tag-based filtering.
- (+) The suite command is unchanged at runtime; the user configures the tag
  filter once.
- (+) Tags in source make quarantine state visible to developers reading tests.
- (+) Does not affect local development — tests are only excluded when the
  runner is invoked with the tag filter.
- (+) Layered with ADR-034: correctness is guaranteed regardless of tag state.

**Negative:**

- (-) Not all frameworks have usable tag mechanisms. Jest and Vitest lack native
  tag filtering, so they cannot benefit from this approach without a future
  framework-level solution. They continue to rely on ADR-034.
- (-) Requires a delivery mechanism (separate decision) to apply and remove tags.
  Until that mechanism exists, tags would need to be applied manually or via
  external tooling.
- (-) Introduces a potential sync gap between quarantine state and source tags.
  ADR-034 covers the gap, but users may be confused if a test is quarantined in
  state but not yet tagged (or vice versa).
- (-) v2 scope — not implemented in v1.
