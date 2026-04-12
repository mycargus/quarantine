/**
 * Negative schema regression tests (ADR-025)
 *
 * Proves that test-result.schema.json rejects known-invalid data:
 * missing required fields, invalid enum values, wrong types.
 *
 * These are schema correctness tests — they verify the schema itself enforces
 * its own constraints, not that any specific fixture is valid.
 *
 * Uses ajv with draft 2020-12 support (the schemas use
 * "$schema": "https://json-schema.org/draft/2020-12/schema").
 */

import { readFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import Ajv from "ajv/dist/2020.js"
import { assert } from "riteway/vitest"
import { describe, test } from "vitest"

const __dirname = dirname(fileURLToPath(import.meta.url))
const schemasDir = join(__dirname, "..", "..", "schemas")

function loadSchema(name) {
  const raw = readFileSync(join(schemasDir, name), "utf8")
  return JSON.parse(raw)
}

// Build an ajv instance. Format validation is not enabled (ajv-formats not
// installed) — date-time format is not asserted in these negative tests.
function buildAjv() {
  return new Ajv({ strict: false, allErrors: true })
}

// Minimal valid test-result document that passes schema validation.
function validResult(overrides = {}) {
  return {
    version: 1,
    run_id: "run-123",
    repo: "owner/repo",
    branch: "main",
    commit_sha: "abc123",
    timestamp: "2026-01-15T10:00:00Z",
    cli_version: "0.1.0",
    suite_name: "jest",
    config: { retry_count: 3 },
    summary: {
      total: 0,
      passed: 0,
      failed: 0,
      skipped: 0,
      quarantined: 0,
      flaky_detected: 0,
      unresolved: 0,
    },
    tests: [],
    ...overrides,
  }
}

describe("test-result.schema.json — negative regression", () => {
  const schema = loadSchema("test-result.schema.json")
  const ajv = buildAjv()
  const validate = ajv.compile(schema)

  function rejects(doc) {
    return !validate(doc)
  }

  function accepts(doc) {
    return validate(doc)
  }

  test("valid baseline document passes schema", () => {
    assert({
      given: "a minimal valid test-result document",
      should: "pass schema validation",
      actual: accepts(validResult()),
      expected: true,
    })
  })

  test("missing required field 'version' is rejected", () => {
    const doc = validResult()
    delete doc.version
    assert({
      given: "test-result missing 'version'",
      should: "fail schema validation",
      actual: rejects(doc),
      expected: true,
    })
  })

  test("missing required field 'run_id' is rejected", () => {
    const doc = validResult()
    delete doc.run_id
    assert({
      given: "test-result missing 'run_id'",
      should: "fail schema validation",
      actual: rejects(doc),
      expected: true,
    })
  })

  test("missing required field 'summary' is rejected", () => {
    const doc = validResult()
    delete doc.summary
    assert({
      given: "test-result missing 'summary'",
      should: "fail schema validation",
      actual: rejects(doc),
      expected: true,
    })
  })

  test("missing required field 'suite_name' is rejected", () => {
    const doc = validResult()
    delete doc.suite_name
    assert({
      given: "test-result missing 'suite_name'",
      should: "fail schema validation",
      actual: rejects(doc),
      expected: true,
    })
  })

  test("version value 2 is rejected", () => {
    assert({
      given: "test-result with version: 2",
      should: "fail schema validation (const must be 1)",
      actual: rejects(validResult({ version: 2 })),
      expected: true,
    })
  })

  test("repo without slash is rejected", () => {
    assert({
      given: "test-result with repo: 'noslash' (no owner/repo format)",
      should: "fail schema validation",
      actual: rejects(validResult({ repo: "noslash" })),
      expected: true,
    })
  })

  test("test entry with invalid status enum is rejected", () => {
    const doc = validResult({
      tests: [
        {
          test_id: "src/foo.test.ts::Suite::test",
          file_path: "src/foo.test.ts",
          classname: "Suite",
          name: "test",
          status: "error", // not in enum — Scenario 72 regression
          duration_ms: 100,
        },
      ],
    })
    assert({
      given: "test entry with status: 'error' (not in schema enum)",
      should: "fail schema validation (Scenario 72 regression)",
      actual: rejects(doc),
      expected: true,
    })
  })

  test("test entry with status 'failed' is accepted", () => {
    const doc = validResult({
      tests: [
        {
          test_id: "src/foo.test.ts::Suite::test",
          file_path: "src/foo.test.ts",
          classname: "Suite",
          name: "test",
          status: "failed",
          duration_ms: 100,
        },
      ],
    })
    assert({
      given: "test entry with status: 'failed'",
      should: "pass schema validation",
      actual: accepts(doc),
      expected: true,
    })
  })

  test("config with retry_count below minimum is rejected", () => {
    assert({
      given: "test-result with config.retry_count: 0 (below minimum of 1)",
      should: "fail schema validation",
      actual: rejects(validResult({ config: { retry_count: 0 } })),
      expected: true,
    })
  })

  test("additional properties at root level are rejected", () => {
    assert({
      given: "test-result with an unknown top-level field",
      should: "fail schema validation (additionalProperties: false)",
      actual: rejects(validResult({ unknown_field: "value" })),
      expected: true,
    })
  })
})

// --- quarantine-state.schema.json ---

// Minimal valid quarantine-state entry (no issue fields — pre-issue-creation).
function validEntry(overrides = {}) {
  return {
    test_id: "src/auth.test.ts::AuthService::logs in",
    file_path: "src/auth.test.ts",
    classname: "AuthService",
    name: "logs in",
    suite: "AuthService",
    first_flaky_at: "2026-01-01T00:00:00Z",
    last_failure_at: "2026-01-01T00:00:00Z",
    flaky_count: 1,
    quarantined_at: "2026-01-01T00:00:00Z",
    quarantined_by: "cli-auto",
    ...overrides,
  }
}

function validStateDoc(testEntries = {}) {
  return {
    version: 1,
    updated_at: "2026-01-01T00:00:00Z",
    tests: testEntries,
  }
}

describe("quarantine-state.schema.json — negative regression", () => {
  const schema = loadSchema("quarantine-state.schema.json")
  const ajv = buildAjv()
  const validate = ajv.compile(schema)

  function rejectsState(doc) {
    return !validate(doc)
  }

  function acceptsState(doc) {
    return validate(doc)
  }

  test("valid state with no entries is accepted", () => {
    assert({
      given: "a quarantine-state with an empty tests map",
      should: "pass schema validation",
      actual: acceptsState(validStateDoc()),
      expected: true,
    })
  })

  test("valid entry without issue fields is accepted (Scenario 66)", () => {
    assert({
      given: "a quarantine-state entry with no issue_number or issue_url (pre-issue-creation)",
      should:
        "pass schema validation (dependentRequired — neither field is independently required)",
      actual: acceptsState(
        validStateDoc({ "src/auth.test.ts::AuthService::logs in": validEntry() }),
      ),
      expected: true,
    })
  })

  test("entry with issue_number but no issue_url is rejected (dependentRequired)", () => {
    assert({
      given: "a quarantine-state entry with issue_number but no issue_url",
      should: "fail schema validation (dependentRequired: issue_number requires issue_url)",
      actual: rejectsState(
        validStateDoc({
          "src/auth.test.ts::AuthService::logs in": validEntry({ issue_number: 42 }),
        }),
      ),
      expected: true,
    })
  })

  test("entry with issue_url but no issue_number is rejected (dependentRequired)", () => {
    assert({
      given: "a quarantine-state entry with issue_url but no issue_number",
      should: "fail schema validation (dependentRequired: issue_url requires issue_number)",
      actual: rejectsState(
        validStateDoc({
          "src/auth.test.ts::AuthService::logs in": validEntry({
            issue_url: "https://github.com/owner/repo/issues/42",
          }),
        }),
      ),
      expected: true,
    })
  })

  test("missing required field 'version' is rejected", () => {
    const doc = validStateDoc()
    delete doc.version
    assert({
      given: "a quarantine-state missing 'version'",
      should: "fail schema validation",
      actual: rejectsState(doc),
      expected: true,
    })
  })

  test("missing required field 'tests' is rejected", () => {
    const doc = validStateDoc()
    delete doc.tests
    assert({
      given: "a quarantine-state missing 'tests'",
      should: "fail schema validation",
      actual: rejectsState(doc),
      expected: true,
    })
  })
})
