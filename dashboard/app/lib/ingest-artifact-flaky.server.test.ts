/**
 * Integration tests for ingestArtifact() — flaky detection handling (Scenario 89).
 *
 * Verifies that test entries with status "flaky" increment flaky_count and update
 * last_flaky_at and last_run_status to "passing", while entries with status
 * "quarantined" leave flaky_count and last_flaky_at unchanged.
 */

import { describe } from "riteway"
import { initDb } from "./db.server.js"
import type { TestResult } from "./ingest.server.js"
import { incrementFlakyCount, ingestArtifact } from "./ingest.server.js"

const flakyFixture: TestResult = {
  version: 1,
  run_id: "run-sc89",
  repo: "mycargus/my-app",
  branch: "main",
  commit_sha: "abc1234567890def1234567890abcdef12345678",
  pr_number: null,
  timestamp: "2026-03-15T14:00:00Z",
  cli_version: "0.1.0",
  framework: "rspec",
  config: {
    retry_count: 3,
  },
  summary: {
    total: 2,
    passed: 0,
    failed: 0,
    skipped: 0,
    quarantined: 1,
    flaky_detected: 1,
  },
  tests: [
    {
      test_id: "spec/payments_spec.rb::PaymentsService::processes_payment",
      file_path: "spec/payments_spec.rb",
      classname: "PaymentsService",
      name: "processes_payment",
      status: "flaky",
      original_status: null,
      duration_ms: 150,
      failure_message: null,
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
  ],
}

describe("ingestArtifact() — flaky entry increments flaky_count and updates last_flaky_at (Scenario 89)", async (assert) => {
  const { db, raw } = initDb(":memory:")
  raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
  const project = raw
    .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
    .get("mycargus", "my-app") as { id: number }
  const projectId = project.id

  // Pre-insert existing quarantined_tests rows for both tests with flaky_count: 2
  raw
    .prepare(
      `INSERT INTO quarantined_tests
           (project_id, test_id, name, issue_number, issue_url, quarantined_at, flaky_count, last_flaky_at, last_run_status)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    )
    .run(
      projectId,
      "spec/payments_spec.rb::PaymentsService::processes_payment",
      "processes_payment",
      42,
      "https://github.com/mycargus/my-app/issues/42",
      "2026-01-10T08:00:00Z",
      2,
      "2026-02-20T10:00:00Z",
      "failing",
    )

  raw
    .prepare(
      `INSERT INTO quarantined_tests
           (project_id, test_id, name, issue_number, issue_url, quarantined_at, flaky_count, last_flaky_at, last_run_status)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    )
    .run(
      projectId,
      "spec/auth_spec.rb::Auth::login",
      "login",
      43,
      "https://github.com/mycargus/my-app/issues/43",
      "2026-01-11T08:00:00Z",
      2,
      "2026-02-20T10:00:00Z",
      "failing",
    )

  await ingestArtifact(
    db,
    raw,
    "mycargus",
    "my-app",
    "quarantine-results-run-sc89",
    JSON.stringify(flakyFixture),
    projectId,
  )

  const paymentsRow = raw
    .prepare(
      "SELECT flaky_count, last_flaky_at, last_run_status FROM quarantined_tests WHERE project_id = ? AND test_id = ?",
    )
    .get(projectId, "spec/payments_spec.rb::PaymentsService::processes_payment") as {
    flaky_count: number
    last_flaky_at: string | null
    last_run_status: string | null
  }

  const loginRow = raw
    .prepare(
      "SELECT flaky_count, last_flaky_at, last_run_status FROM quarantined_tests WHERE project_id = ? AND test_id = ?",
    )
    .get(projectId, "spec/auth_spec.rb::Auth::login") as {
    flaky_count: number
    last_flaky_at: string | null
    last_run_status: string | null
  }

  assert({
    given:
      "a pre-existing processes_payment row with flaky_count 2 and a new artifact entry with status 'flaky'",
    should: "increment flaky_count to 3",
    actual: paymentsRow.flaky_count,
    expected: 3,
  })

  assert({
    given:
      "a pre-existing processes_payment row with last_flaky_at '2026-02-20T10:00:00Z' and a new artifact entry with status 'flaky' and timestamp '2026-03-15T14:00:00Z'",
    should: "update last_flaky_at to the artifact timestamp '2026-03-15T14:00:00Z'",
    actual: paymentsRow.last_flaky_at,
    expected: "2026-03-15T14:00:00Z",
  })

  assert({
    given: "a flaky test entry (failed initially, passed on retry)",
    should: "set last_run_status to 'passing'",
    actual: paymentsRow.last_run_status,
    expected: "passing",
  })

  assert({
    given:
      "a pre-existing login row with flaky_count 2 and a new artifact entry with status 'quarantined' (not flaky)",
    should: "leave flaky_count unchanged at 2",
    actual: loginRow.flaky_count,
    expected: 2,
  })

  assert({
    given:
      "a pre-existing login row with last_flaky_at '2026-02-20T10:00:00Z' and a new artifact entry with status 'quarantined' (not flaky)",
    should: "leave last_flaky_at unchanged at '2026-02-20T10:00:00Z'",
    actual: loginRow.last_flaky_at,
    expected: "2026-02-20T10:00:00Z",
  })
})

describe("ingestArtifact() — flaky entry idempotency: same run_id ingested twice does not double-increment", async (assert) => {
  const { db, raw } = initDb(":memory:")
  raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
  const project = raw
    .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
    .get("mycargus", "my-app") as { id: number }
  const projectId = project.id

  raw
    .prepare(
      `INSERT INTO quarantined_tests
           (project_id, test_id, name, issue_number, issue_url, quarantined_at, flaky_count, last_flaky_at, last_run_status)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    )
    .run(
      projectId,
      "spec/payments_spec.rb::PaymentsService::processes_payment",
      "processes_payment",
      42,
      "https://github.com/mycargus/my-app/issues/42",
      "2026-01-10T08:00:00Z",
      2,
      "2026-02-20T10:00:00Z",
      "failing",
    )

  const json = JSON.stringify(flakyFixture)
  await ingestArtifact(
    db,
    raw,
    "mycargus",
    "my-app",
    "quarantine-results-run-sc89-idem",
    json,
    projectId,
  )
  await ingestArtifact(
    db,
    raw,
    "mycargus",
    "my-app",
    "quarantine-results-run-sc89-idem",
    json,
    projectId,
  )

  const row = raw
    .prepare("SELECT flaky_count FROM quarantined_tests WHERE project_id = ? AND test_id = ?")
    .get(projectId, "spec/payments_spec.rb::PaymentsService::processes_payment") as {
    flaky_count: number
  }

  assert({
    given: "the same artifact with a flaky entry ingested twice (same run_id)",
    should: "increment flaky_count only once — duplicate run_id is not reprocessed",
    actual: row.flaky_count,
    expected: 3,
  })
})

describe("incrementFlakyCount() — no-op when test_id does not exist in quarantined_tests", async (assert) => {
  const { raw } = initDb(":memory:")
  raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
  const project = raw
    .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
    .get("mycargus", "my-app") as { id: number }
  const projectId = project.id

  incrementFlakyCount(
    raw,
    projectId,
    "spec/nonexistent_spec.rb::Unknown::test",
    "2026-03-15T14:00:00Z",
  )

  const count = (
    raw
      .prepare("SELECT COUNT(*) as count FROM quarantined_tests WHERE project_id = ?")
      .get(projectId) as { count: number }
  ).count

  assert({
    given: "a test_id that does not exist in quarantined_tests",
    should: "silently do nothing — UPDATE affects 0 rows, no new row is created",
    actual: count,
    expected: 0,
  })
})

describe("ingestArtifact() — flaky entry increments flaky_count from 0 (first flaky detection)", async (assert) => {
  const { db, raw } = initDb(":memory:")
  raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
  const project = raw
    .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
    .get("mycargus", "my-app") as { id: number }
  const projectId = project.id

  raw
    .prepare(
      `INSERT INTO quarantined_tests
           (project_id, test_id, name, issue_number, issue_url, quarantined_at, flaky_count, last_flaky_at, last_run_status)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    )
    .run(
      projectId,
      "spec/payments_spec.rb::PaymentsService::processes_payment",
      "processes_payment",
      42,
      "https://github.com/mycargus/my-app/issues/42",
      "2026-01-10T08:00:00Z",
      0,
      null,
      "failing",
    )

  const firstFlakyFixture: TestResult = {
    ...flakyFixture,
    run_id: "run-first-flaky",
    tests: [flakyFixture.tests[0]],
  }

  await ingestArtifact(
    db,
    raw,
    "mycargus",
    "my-app",
    "quarantine-results-first-flaky",
    JSON.stringify(firstFlakyFixture),
    projectId,
  )

  const row = raw
    .prepare(
      "SELECT flaky_count, last_flaky_at FROM quarantined_tests WHERE project_id = ? AND test_id = ?",
    )
    .get(projectId, "spec/payments_spec.rb::PaymentsService::processes_payment") as {
    flaky_count: number
    last_flaky_at: string | null
  }

  assert({
    given:
      "a quarantined_tests row with flaky_count 0 and a new artifact entry with status 'flaky'",
    should: "increment flaky_count from 0 to 1",
    actual: row.flaky_count,
    expected: 1,
  })

  assert({
    given:
      "a quarantined_tests row with last_flaky_at null and a new artifact entry with status 'flaky'",
    should: "set last_flaky_at to the artifact timestamp",
    actual: row.last_flaky_at,
    expected: "2026-03-15T14:00:00Z",
  })
})
