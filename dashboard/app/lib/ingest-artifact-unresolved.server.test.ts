/**
 * Interface tests for ingestArtifact() — unresolved test handling (Scenario 140).
 *
 * Verifies that test entries with status "unresolved" are:
 * - stored with their status preserved (unresolved_tests count in test_runs)
 * - NOT added to quarantined_tests
 * - NOT counted as flaky detections
 * - NOT causing the artifact to be skipped or rejected
 */

import { describe } from "riteway"
import { initDb } from "./db.server.js"
import type { TestResult } from "./ingest.server.js"
import { ingestArtifact, validateTestResult } from "./ingest.server.js"

const unresolvedFixture: TestResult = {
  version: 1,
  run_id: "run-xyz789",
  repo: "mycargus/my-app",
  branch: "main",
  commit_sha: "abc1234567890def1234567890abcdef12345678",
  pr_number: null,
  timestamp: "2026-04-10T14:00:00Z",
  cli_version: "0.1.0",
  suite_name: "backend",
  config: {
    retry_count: 3,
  },
  summary: {
    total: 3,
    passed: 0,
    failed: 0,
    skipped: 0,
    quarantined: 0,
    flaky_detected: 1,
    unresolved: 2,
  },
  tests: [
    {
      test_id: "spec/models/user_spec.rb::User::validates email",
      file_path: "spec/models/user_spec.rb",
      classname: "User",
      name: "validates email",
      status: "flaky",
      original_status: null,
      duration_ms: 120,
      failure_message: null,
      issue_number: 42,
    },
    {
      test_id: "spec/models/order_spec.rb::Order::ships on time",
      file_path: "spec/models/order_spec.rb",
      classname: "Order",
      name: "ships on time",
      status: "unresolved",
      original_status: null,
      duration_ms: 300000,
      failure_message: null,
      issue_number: null,
      error: "rerun timed out after 5m",
    },
    {
      test_id: "spec/services/payment_spec.rb::Payment::charges card",
      file_path: "spec/services/payment_spec.rb",
      classname: "Payment",
      name: "charges card",
      status: "unresolved",
      original_status: null,
      duration_ms: 500,
      failure_message: null,
      issue_number: null,
      error: "rerun command failed: exec: 'bundle': executable file not found in $PATH",
      rerun_exit_code: 127,
    },
  ],
}

describe("validateTestResult() — unresolved status", async (assert) => {
  assert({
    given: "an artifact with unresolved test entries and summary.unresolved field",
    should: "pass JSON Schema validation",
    actual: validateTestResult(unresolvedFixture),
    expected: { valid: true, errors: [] },
  })
})

describe("ingestArtifact() — unresolved tests (Scenario 140)", async (assert) => {
  const { db, raw } = initDb(":memory:")

  raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
  const project = raw
    .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
    .get("mycargus", "my-app") as { id: number }
  const projectId = project.id

  const warnings: string[] = []
  const result = await ingestArtifact(
    db,
    raw,
    "mycargus",
    "my-app",
    "quarantine-results-backend-run-xyz789",
    JSON.stringify(unresolvedFixture),
    projectId,
    (msg) => warnings.push(msg),
  )

  const testRunRow = raw.prepare("SELECT * FROM test_runs WHERE run_id = ?").get("run-xyz789") as
    | Record<string, unknown>
    | undefined

  const quarantinedRows = raw
    .prepare("SELECT test_id FROM quarantined_tests WHERE project_id = ?")
    .all(projectId) as { test_id: string }[]

  assert({
    given: "an artifact with 1 flaky + 2 unresolved tests",
    should: 'return "ingested" (not skipped or rejected)',
    actual: result,
    expected: "ingested",
  })

  assert({
    given: "an artifact with 1 flaky + 2 unresolved tests",
    should: "store the test run row in the database",
    actual: testRunRow?.run_id,
    expected: "run-xyz789",
  })

  assert({
    given: "an artifact with 1 flaky + 2 unresolved tests",
    should: "store unresolved_tests count as 2 in the test_runs row",
    actual: testRunRow?.unresolved_tests,
    expected: 2,
  })

  assert({
    given: "an artifact with 1 flaky + 2 unresolved tests",
    should: "store flaky_tests count as 1 (only the flaky test, not unresolved)",
    actual: testRunRow?.flaky_tests,
    expected: 1,
  })

  assert({
    given: "an artifact with 2 unresolved tests",
    should: "NOT add unresolved tests to quarantined_tests",
    actual: quarantinedRows.some(
      (r) =>
        r.test_id === "spec/models/order_spec.rb::Order::ships on time" ||
        r.test_id === "spec/services/payment_spec.rb::Payment::charges card",
    ),
    expected: false,
  })

  assert({
    given: "an artifact with 1 flaky test",
    should: "add only the flaky test to quarantined_tests",
    actual: quarantinedRows.map((r) => r.test_id),
    expected: ["spec/models/user_spec.rb::User::validates email"],
  })

  assert({
    given: "a valid artifact with unresolved tests",
    should: "emit no warnings",
    actual: warnings,
    expected: [],
  })
})
