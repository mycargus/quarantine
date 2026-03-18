# ADR-020: Test ID Construction Strategy

**Status:** Accepted
**Date:** 2026-03-17

## Context

Quarantine needs a stable, unique identifier for each test to track flakiness across runs, manage quarantine state, create issues, and correlate data between the CLI and dashboard. The identifier must be:

- **Stable:** The same test produces the same ID across runs.
- **Unique:** No two distinct tests share an ID within a repository.
- **Human-readable:** Developers can look at an ID and know which test it refers to.
- **Cross-framework:** Works for all v1 frameworks (Jest, RSpec, Vitest) and extensible to v2 frameworks.
- **Derivable from JUnit XML:** The ID must be constructable from data available in JUnit XML output, since that is the CLI's sole interface with test results.

Research found that there is no official JUnit XML schema. Framework implementations vary in how they populate `classname`, `name`, and `file` attributes. See `docs/research/junit-xml-research.md` for full findings.

## Decision

The `test_id` is a human-readable composite string: `file_path::classname::name`, constructed from JUnit XML `<testcase>` attributes. The `::` delimiter was chosen because it does not appear in any v1 framework's JUnit XML output.

Raw components (`file_path`, `classname`, `name`) are stored alongside `test_id` in both `quarantine.json` and test result artifacts. This allows the dashboard and CLI to reconstruct or re-derive IDs if the construction logic changes.

### Framework-Specific Construction

- **Jest (via `jest-junit`):** `classname` and `name` are configurable via `jest-junit` options. The CLI recommends specific `jest-junit` configuration during `quarantine init` to ensure stable, parseable output. Without recommended config, `classname` defaults may be inconsistent across Jest versions.
- **RSpec (via `rspec_junit_formatter`):** `classname` is typically the file path with dots, `name` is the full test description. Line numbers are NOT included in JUnit XML, so they cannot be part of the ID.
- **Vitest:** Built-in `--reporter=junit` support. `classname` is the file path, `name` is the test description.

### Dashboard Indexing

The dashboard stores a SHA-256 hash of `test_id` internally for efficient database indexing. The hash is an internal implementation detail of the dashboard and is NOT part of the cross-system contract (not stored in `quarantine.json` or result artifacts).

## Alternatives Considered

- **Hash-only ID (SHA-256 of test attributes):** Compact and unique, but not human-readable. Developers cannot look at a quarantine list and know which tests are quarantined without a lookup table. Rejected for poor developer experience.
- **Framework-native ID (e.g., RSpec's `./spec/foo_spec.rb[1:2:3]`):** Not cross-framework, not derivable from JUnit XML for all frameworks, and ties the ID format to a specific framework's internals. Rejected for lack of portability.
- **`classname.name` (dot-separated):** Dots appear in classnames across all frameworks, making parsing ambiguous. Rejected for delimiter collision.
- **`classname#name` (hash-separated):** `#` appears in some RSpec test names. Rejected for delimiter collision.

## Consequences

- (+) Human-readable -- developers can identify the test from the ID alone.
- (+) `::` delimiter is unambiguous across all v1 (and known v2) frameworks.
- (+) Storing raw components alongside the composite ID allows for future ID format evolution without data migration.
- (+) SHA-256 indexing in the dashboard provides efficient lookups without exposing implementation details to the contract.
- (-) Jest requires specific `jest-junit` configuration for stable output. Mitigated by recommending config during `quarantine init` and documenting in setup guides.
- (-) If a test file is renamed, its `test_id` changes, breaking quarantine continuity. The old quarantine entry becomes orphaned. Mitigated by the unquarantine-on-issue-close mechanism (orphaned entries are cleaned up when their issues are closed).
- (-) Parameterized tests (Jest `test.each`, RSpec shared examples) may produce multiple test IDs for what a developer considers "one test." This is acceptable -- each parameterized variant is tracked independently.
