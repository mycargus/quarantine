### Scenario 77: Go Result marshal output conforms to test-result schema [M8]

**Risk:** The CLI produces `results.json` that the dashboard validates at runtime via ajv. If Go struct changes cause the marshaled JSON to drift from `test-result.schema.json`, the dashboard silently rejects results with no CI warning (ADR-025).

**Given** the CLI builds a `Result` from parser output containing:
- A passing test
- A failing test
- A flaky test (failed initial run, passed on retry)

**When** the CLI marshals the `Result` via `result.BuildAt()` (or `result.BuildAtWithRetries()` for the retry case)

**Then** the marshaled JSON validates against `schemas/test-result.schema.json` using a JSON Schema draft 2020-12 validator. All required fields are present, all enum values (`passed`, `failed`, `flaky`) are valid, and `additionalProperties: false` is not violated.

---

### Scenario 78: Schemas reject known-invalid data [M8]

**Risk:** A schema could be weakened (e.g., a `required` field accidentally removed, an enum value added) without detection, allowing invalid data to pass validation silently.

**Given** three JSON schemas in `schemas/`:
- `test-result.schema.json`
- `quarantine-state.schema.json`
- `quarantine-config.schema.json`

**When** each schema is used to validate known-invalid data:
1. A test result missing `run_id` (a required field)
2. A quarantine state entry missing `test_id` (a required field)
3. A quarantine config with `framework: "mocha"` (not in the enum)

**Then** the validator rejects all three inputs with errors identifying the specific violation. These are regression tests — if a future schema change causes any to pass, the test fails.

---

### Scenario 79: quarantine-state schema allows entries without issue fields [M8]

**Risk:** `quarantine-state.schema.json` marks `issue_number` and `issue_url` as required, but the CLI writes entries before issue creation and persists them without issue fields when issue creation fails in degraded mode. The schema contradicts the "never break the build" principle (ADR-025, scenario 66).

**Given** `quarantine-state.schema.json` has been updated per ADR-025 to make `issue_number` and `issue_url` optional

**When** a quarantine state entry is validated that has all fields except `issue_number` and `issue_url`

**Then** the validator accepts the entry as valid. This confirms the schema fix aligns with the Go struct's `omitempty` behavior and degraded mode semantics.

---
