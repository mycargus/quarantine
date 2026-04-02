/**
 * Integration tests for ingestArtifact() — quarantined_tests population (Scenario 87).
 *
 * Verifies that quarantined test entries in the artifact JSON are upserted into
 * the quarantined_tests table with correct field mappings.
 */

import { describe } from "riteway"
import { initDb } from "./db.server.js"
import type { TestResult } from "./ingest.server.js"
import { ingestArtifact } from "./ingest.server.js"

const quarantineFixture: TestResult = {
  version: 1,
  run_id: "run-abc123",
  repo: "mycargus/my-app",
  branch: "main",
  commit_sha: "abc1234567890def1234567890abcdef12345678",
  pr_number: null,
  timestamp: "2026-02-10T14:00:00Z",
  cli_version: "0.1.0",
  framework: "rspec",
  config: {
    retry_count: 3,
  },
  summary: {
    total: 3,
    passed: 0,
    failed: 0,
    skipped: 0,
    quarantined: 3,
    flaky_detected: 0,
  },
  tests: [
    {
      test_id: "spec/payments_spec.rb::PaymentsService::processes_payment",
      file_path: "spec/payments_spec.rb",
      classname: "PaymentsService",
      name: "processes_payment",
      status: "quarantined",
      original_status: "failed",
      duration_ms: 120,
      failure_message: "expected true, got false",
      issue_number: 42,
    },
    {
      test_id: "spec/auth_spec.rb::Auth::login",
      file_path: "spec/auth_spec.rb",
      classname: "Auth",
      name: "login",
      status: "quarantined",
      original_status: "passed",
      duration_ms: 80,
      failure_message: null,
      issue_number: 43,
    },
    {
      test_id: "spec/cart_spec.rb::Cart::checkout",
      file_path: "spec/cart_spec.rb",
      classname: "Cart",
      name: "checkout",
      status: "quarantined",
      original_status: null,
      duration_ms: 0,
      failure_message: null,
      issue_number: 44,
    },
  ],
}

describe("ingestArtifact() with quarantined test entries", async (assert) => {
  const { db, raw } = initDb(":memory:")
  raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
  const project = raw
    .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
    .get("mycargus", "my-app") as { id: number }
  const projectId = project.id

  const result = await ingestArtifact(
    db,
    raw,
    "mycargus",
    "my-app",
    "quarantine-results-run-abc123",
    JSON.stringify(quarantineFixture),
    projectId,
  )

  const qtRows = raw
    .prepare("SELECT * FROM quarantined_tests WHERE project_id = ? ORDER BY test_id")
    .all(projectId) as {
    test_id: string
    name: string
    issue_number: number | null
    issue_url: string | null
    quarantined_at: string
    last_run_status: string | null
  }[]

  const testRunRow = raw
    .prepare("SELECT run_id FROM test_runs WHERE run_id = ?")
    .get("run-abc123") as { run_id: string } | undefined

  assert({
    given: "an artifact with 3 quarantined test entries",
    should: "return 'ingested'",
    actual: result,
    expected: "ingested",
  })

  assert({
    given: "an artifact with 3 quarantined test entries",
    should: "upsert all 3 entries into quarantined_tests",
    actual: qtRows.length,
    expected: 3,
  })

  assert({
    given: "an artifact with 3 quarantined test entries",
    should: "set quarantined_at to the artifact timestamp for all entries",
    actual: qtRows.map((r) => r.quarantined_at),
    expected: ["2026-02-10T14:00:00Z", "2026-02-10T14:00:00Z", "2026-02-10T14:00:00Z"],
  })

  assert({
    given: 'the entry with original_status "failed"',
    should: 'have last_run_status "failing"',
    actual: qtRows.find(
      (r) => r.test_id === "spec/payments_spec.rb::PaymentsService::processes_payment",
    )?.last_run_status,
    expected: "failing",
  })

  assert({
    given: 'the entry with original_status "passed"',
    should: 'have last_run_status "passing"',
    actual: qtRows.find((r) => r.test_id === "spec/auth_spec.rb::Auth::login")?.last_run_status,
    expected: "passing",
  })

  assert({
    given: "the entry with original_status null",
    should: "have last_run_status null",
    actual: qtRows.find((r) => r.test_id === "spec/cart_spec.rb::Cart::checkout")?.last_run_status,
    expected: null,
  })

  assert({
    given: "the entry with issue_number 42",
    should: "have issue_url https://github.com/mycargus/my-app/issues/42",
    actual: qtRows.find(
      (r) => r.test_id === "spec/payments_spec.rb::PaymentsService::processes_payment",
    )?.issue_url,
    expected: "https://github.com/mycargus/my-app/issues/42",
  })

  assert({
    given: "the entry with issue_number 43",
    should: "have issue_url https://github.com/mycargus/my-app/issues/43",
    actual: qtRows.find((r) => r.test_id === "spec/auth_spec.rb::Auth::login")?.issue_url,
    expected: "https://github.com/mycargus/my-app/issues/43",
  })

  assert({
    given: "the entry with issue_number 44",
    should: "have issue_url https://github.com/mycargus/my-app/issues/44",
    actual: qtRows.find((r) => r.test_id === "spec/cart_spec.rb::Cart::checkout")?.issue_url,
    expected: "https://github.com/mycargus/my-app/issues/44",
  })

  assert({
    given: "an artifact with 3 quarantined test entries",
    should: "also insert a test_runs row for run-abc123 (existing behavior unchanged)",
    actual: testRunRow?.run_id,
    expected: "run-abc123",
  })
})

describe("ingestArtifact() — artifact with no quarantined test entries", async (assert) => {
  const { db, raw } = initDb(":memory:")
  raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
  const project = raw
    .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
    .get("mycargus", "my-app") as { id: number }
  const projectId = project.id

  const noQuarantineFixture: TestResult = {
    ...quarantineFixture,
    run_id: "run-no-quarantined",
    summary: {
      total: 1,
      passed: 1,
      failed: 0,
      skipped: 0,
      quarantined: 0,
      flaky_detected: 0,
    },
    tests: [
      {
        test_id: "spec/other_spec.rb::Other::passes",
        file_path: "spec/other_spec.rb",
        classname: "Other",
        name: "passes",
        status: "passed",
        original_status: null,
        duration_ms: 50,
        failure_message: null,
        issue_number: null,
      },
    ],
  }

  const result = await ingestArtifact(
    db,
    raw,
    "mycargus",
    "my-app",
    "quarantine-results-no-quarantined",
    JSON.stringify(noQuarantineFixture),
    projectId,
  )

  const qtCount = (
    raw
      .prepare("SELECT COUNT(*) as count FROM quarantined_tests WHERE project_id = ?")
      .get(projectId) as {
      count: number
    }
  ).count
  const trRow = raw
    .prepare("SELECT run_id FROM test_runs WHERE run_id = ?")
    .get("run-no-quarantined") as { run_id: string } | undefined

  assert({
    given: "an artifact where all tests have status 'passed' (none quarantined)",
    should: "return 'ingested'",
    actual: result,
    expected: "ingested",
  })

  assert({
    given: "an artifact where all tests have status 'passed' (none quarantined)",
    should: "write zero quarantined_tests rows",
    actual: qtCount,
    expected: 0,
  })

  assert({
    given: "an artifact where all tests have status 'passed' (none quarantined)",
    should: "still insert a test_runs row",
    actual: trRow?.run_id,
    expected: "run-no-quarantined",
  })
})

describe("ingestArtifact() — idempotency: same artifact ingested twice", async (assert) => {
  const { db, raw } = initDb(":memory:")
  raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
  const project = raw
    .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
    .get("mycargus", "my-app") as { id: number }
  const projectId = project.id

  const fixture: TestResult = {
    ...quarantineFixture,
    run_id: "run-idempotent",
    tests: [quarantineFixture.tests[0]],
  }
  const json = JSON.stringify(fixture)

  const first = await ingestArtifact(
    db,
    raw,
    "mycargus",
    "my-app",
    "quarantine-results-idem",
    json,
    projectId,
  )
  const second = await ingestArtifact(
    db,
    raw,
    "mycargus",
    "my-app",
    "quarantine-results-idem",
    json,
    projectId,
  )

  const trCount = (
    raw
      .prepare("SELECT COUNT(*) as count FROM test_runs WHERE run_id = ?")
      .get("run-idempotent") as {
      count: number
    }
  ).count
  const qtCount = (
    raw
      .prepare("SELECT COUNT(*) as count FROM quarantined_tests WHERE project_id = ?")
      .get(projectId) as {
      count: number
    }
  ).count

  assert({
    given: "the same artifact ingested twice",
    should: "return 'ingested' on both calls",
    actual: [first, second],
    expected: ["ingested", "ingested"],
  })

  assert({
    given: "the same artifact ingested twice",
    should: "result in exactly one test_runs row",
    actual: trCount,
    expected: 1,
  })

  assert({
    given: "the same artifact ingested twice",
    should: "result in exactly one quarantined_tests row per test entry",
    actual: qtCount,
    expected: 1,
  })
})
