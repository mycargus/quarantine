# ADR-025: Schema Validation Strategy

**Status:** Accepted
**Date:** 2026-03-30

## Context

Three JSON schemas exist in `schemas/`:

| Schema | Producer | Consumer |
|--------|----------|----------|
| `test-result.schema.json` | CLI (Go) | Dashboard (ajv at runtime) |
| `quarantine-state.schema.json` | CLI (Go) | CLI (Go) |
| `quarantine-config.schema.json` | User (YAML) | CLI (Go code validation) |

The `schemas-validate` Makefile target is a TODO stub that prints a message and
exits successfully. `test-all` includes it, so CI appears to validate schemas
but does not. Twelve golden fixtures exist in `testdata/` but no test reads
them.

A planning exercise (now removed) analyzed the gap and found:

1. **`test-result` is the only cross-component schema.** The CLI produces JSON
   that the dashboard validates at runtime via ajv. If Go output drifts from
   the schema, production breaks. No CI test catches this today.

2. **`quarantine-state` is single-component.** The CLI is both producer and
   consumer. It trusts its own output. Schema drift means the schema is wrong,
   not the code. However, the schema marks `issue_number` and `issue_url` as
   required while the Go struct uses `omitempty` — a real conflict (see
   [Bug](#bug-issue_number-and-issue_url-required-vs-omitempty)).

3. **`quarantine-config` is a documentation artifact.** Config is YAML
   validated by Go code (`config.Validate()`), not the JSON schema. Adding
   schema validation would require YAML-to-JSON conversion for no practical
   benefit.

4. **No negative schema tests exist.** Nothing proves the schemas actually
   reject invalid data (missing required fields, wrong enum values, wrong
   types).

## Decision

**Validate Go marshal output against `test-result.schema.json` in Go tests.**
This is the one contract where producer and consumer are different components.
A Go test marshals a `Result` via `result.Build*()`, then validates the JSON
against the schema using `santhosh-tekuri/jsonschema/v6`. This runs in
`cli-test`, not a separate CI job.

**Add minimal negative schema regression tests.** A small JS test suite (ajv)
proves each schema rejects known-invalid data: missing required fields, invalid
enum values, wrong types. These are schema correctness tests, not per-fixture
validation.

**Fix the `issue_number`/`issue_url` required-vs-omitempty conflict** by
making both fields optional in `quarantine-state.schema.json`. The Go struct is
correct: entries exist without an issue when issue creation hasn't happened yet
or failed in degraded mode (see bug section below).

**Do not validate `quarantine-state` or `quarantine-config` marshal output
against schemas.** These schemas are single-component documentation. Testing
that Go output matches them provides no cross-component safety.

**Delete the `schemas-validate` Makefile stub.** It lies — CI reports success
with no validation. The Go schema test (`cli-test`) and negative JS tests
replace it.

**Do not extract dashboard inline fixtures.** The inline `validFixture` objects
in dashboard tests work fine. DRY-ness across test files is not a correctness
concern.

**Do not create a `/create-schema-test` skill or update milestone/CLAUDE.md
docs.** The pattern is simple enough to not need a skill, and documentation
updates are disproportionate to the scope.

## Bug: `issue_number` and `issue_url` required vs omitempty

`quarantine-state.schema.json` marks `issue_number` and `issue_url` as
required. The Go `Entry` struct uses `*int` with `omitempty` for
`IssueNumber` and `string` with `omitempty` for `IssueURL`. This is
intentional in Go: a newly detected flaky test is written to state *before*
issue creation, and if issue creation fails (degraded mode), the entry persists
without an issue number. This follows the "never break the build" principle.

**Fix:** Remove `issue_number` and `issue_url` from the `required` array in
the schema. The Go struct is the source of truth for this single-component
contract. See scenario 66 in `docs/scenarios/v1/10-github-api-edge-cases.md`
which already describes this behavior.

## Alternatives Considered

- **Validate all three schemas in Go and JS.** Rejected. Two of three schemas
  are single-component documentation. Testing Go output against its own
  documentation schema is circular — if the schema drifts, the schema is wrong.

- **Per-fixture validation (every fixture file against its schema).** Rejected.
  Fixtures are test data. If a fixture doesn't match the schema, the test using
  it will fail. Validating test data is a test of a test.

- **Do nothing.** Rejected. The `test-result` cross-component gap is real. The
  dashboard could reject CLI output at runtime with no CI warning.

## Consequences

**Positive:**

- (+) The one cross-component contract (`test-result`) is validated in CI.
  Schema drift between Go output and dashboard expectations is caught before
  merge.
- (+) Negative tests prove schemas reject invalid data, preventing schema
  weakening (e.g., accidentally removing a `required` field).
- (+) The `omitempty` bug is fixed, aligning the schema with actual runtime
  behavior.
- (+) The lying `schemas-validate` stub is removed.

**Negative:**

- (-) `quarantine-state` and `quarantine-config` schemas remain unvalidated
  against Go output. Acceptable because both are single-component.
- (-) Adding `santhosh-tekuri/jsonschema/v6` as a Go test dependency increases
  `go.sum` size.
