import { describe } from "riteway"
import {
  getProjectByOwnerRepo,
  getProjectQuarantinedTests,
  getProjectTrend,
  initDb,
  upsertProject,
} from "./db.server.js"

describe("getProjectByOwnerRepo()", async (assert) => {
  {
    const handle = initDb(":memory:")

    assert({
      given: "a project that does not exist in the database",
      should: "return null",
      actual: await getProjectByOwnerRepo(handle, "acme", "missing"),
      expected: null,
    })
  }

  {
    const handle = initDb(":memory:")
    await upsertProject(handle.db, "acme", "payments-service")

    const result = await getProjectByOwnerRepo(handle, "acme", "payments-service")

    assert({
      given: "a project that exists in the database",
      should: "return owner 'acme'",
      actual: result?.owner,
      expected: "acme",
    })

    assert({
      given: "a project that exists in the database",
      should: "return repo 'payments-service'",
      actual: result?.repo,
      expected: "payments-service",
    })

    assert({
      given: "a project that exists in the database",
      should: "return a positive integer id",
      actual: typeof result?.id === "number" && (result?.id ?? 0) > 0,
      expected: true,
    })
  }
})

describe("getProjectQuarantinedTests()", async (assert) => {
  {
    const handle = initDb(":memory:")
    await upsertProject(handle.db, "acme", "payments-service")

    assert({
      given: "a project with no quarantined tests",
      should: "return an empty array",
      actual: await getProjectQuarantinedTests(handle, "acme", "payments-service"),
      expected: [],
    })
  }

  {
    const handle = initDb(":memory:")
    const projectId = await upsertProject(handle.db, "acme", "payments-service")

    handle.raw
      .prepare(
        `INSERT INTO quarantined_tests
          (project_id, test_id, name, issue_number, issue_url, quarantined_at, last_flaky_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        projectId,
        "test-1",
        "should process payment",
        42,
        "https://github.com/acme/payments-service/issues/42",
        "2026-01-10T10:00:00Z",
        "2026-03-20T08:00:00Z",
      )

    handle.raw
      .prepare(
        `INSERT INTO quarantined_tests
          (project_id, test_id, name, issue_number, issue_url, quarantined_at, last_flaky_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        projectId,
        "test-2",
        "should refund payment",
        43,
        "https://github.com/acme/payments-service/issues/43",
        "2026-02-01T09:00:00Z",
        null,
      )

    handle.raw
      .prepare(
        `INSERT INTO quarantined_tests
          (project_id, test_id, name, issue_number, issue_url, quarantined_at, last_flaky_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        projectId,
        "test-3",
        "should validate card",
        null,
        null,
        "2026-03-01T12:00:00Z",
        "2026-03-15T07:00:00Z",
      )

    const results = await getProjectQuarantinedTests(handle, "acme", "payments-service")

    assert({
      given: "a project with 3 quarantined tests",
      should: "return all 3 tests",
      actual: results.length,
      expected: 3,
    })

    assert({
      given: "a project with 3 quarantined tests",
      should: "include the test name for the first test",
      actual: results.some((t) => t.name === "should process payment"),
      expected: true,
    })

    const paymentTest = results.find((r) => r.name === "should process payment")
    const refundTest = results.find((r) => r.name === "should refund payment")
    const validateTest = results.find((r) => r.name === "should validate card")

    assert({
      given: "the 'should process payment' quarantined test",
      should: "return issueNumber 42",
      actual: paymentTest?.issueNumber,
      expected: 42,
    })

    assert({
      given: "the 'should process payment' quarantined test",
      should: "return non-null issueUrl",
      actual: paymentTest?.issueUrl !== null && paymentTest?.issueUrl !== undefined,
      expected: true,
    })

    assert({
      given: "the 'should validate card' quarantined test with null issue fields",
      should: "return null for issueNumber",
      actual: validateTest?.issueNumber,
      expected: null,
    })

    assert({
      given: "the 'should validate card' quarantined test with null issue fields",
      should: "return null for issueUrl",
      actual: validateTest?.issueUrl,
      expected: null,
    })

    assert({
      given: "the 'should process payment' quarantined test with non-null last_flaky_at",
      should: "return the lastFlakyAt timestamp",
      actual: paymentTest?.lastFlakyAt,
      expected: "2026-03-20T08:00:00Z",
    })

    assert({
      given: "the 'should refund payment' quarantined test with null last_flaky_at",
      should: "return null for lastFlakyAt",
      actual: refundTest?.lastFlakyAt,
      expected: null,
    })

    assert({
      given: "the 'should process payment' quarantined test",
      should: "return quarantinedAt '2026-01-10T10:00:00Z'",
      actual: paymentTest?.quarantinedAt,
      expected: "2026-01-10T10:00:00Z",
    })
  }

  {
    const handle = initDb(":memory:")
    await upsertProject(handle.db, "acme", "payments-service")
    await upsertProject(handle.db, "acme", "other-service")
    const otherId = handle.raw
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("acme", "other-service") as { id: number }

    handle.raw
      .prepare(
        `INSERT INTO quarantined_tests
          (project_id, test_id, name, quarantined_at)
         VALUES (?, ?, ?, ?)`,
      )
      .run(otherId.id, "other-test", "should belong to other service", "2026-01-01T00:00:00Z")

    const results = await getProjectQuarantinedTests(handle, "acme", "payments-service")

    assert({
      given: "quarantined tests for a different project",
      should: "not be included in results for the queried project",
      actual: results.length,
      expected: 0,
    })
  }

  {
    const handle = initDb(":memory:")

    const results = await getProjectQuarantinedTests(handle, "acme", "nonexistent")

    assert({
      given: "a project that does not exist in the database",
      should: "return an empty array",
      actual: results,
      expected: [],
    })
  }
})

describe("getProjectTrend()", async (assert) => {
  {
    const handle = initDb(":memory:")
    await upsertProject(handle.db, "acme", "payments-service")

    assert({
      given: "a project with no test runs",
      should: "return an empty array",
      actual: await getProjectTrend(handle, "acme", "payments-service"),
      expected: [],
    })
  }

  {
    const handle = initDb(":memory:")
    const projectId = await upsertProject(handle.db, "acme", "payments-service")

    // Insert 7 days of test runs with varying flaky counts
    const days = [
      { date: "2026-03-01", flaky: 2 },
      { date: "2026-03-02", flaky: 1 },
      { date: "2026-03-03", flaky: 3 },
      { date: "2026-03-04", flaky: 0 },
      { date: "2026-03-05", flaky: 2 },
      { date: "2026-03-06", flaky: 1 },
      { date: "2026-03-07", flaky: 4 },
    ]

    for (let i = 0; i < days.length; i++) {
      const day = days[i]
      handle.raw
        .prepare(
          `INSERT INTO test_runs
            (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests)
           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        )
        .run(
          projectId,
          `run-${i}`,
          "main",
          `sha-${i}`,
          `${day.date}T10:00:00Z`,
          100,
          100 - day.flaky,
          0,
          day.flaky,
        )
    }

    const trend = await getProjectTrend(handle, "acme", "payments-service")

    assert({
      given: "a project with test runs over 7 distinct days",
      should: "return 7 trend data points",
      actual: trend.length,
      expected: 7,
    })

    assert({
      given: "7 days of test run data",
      should: "return data ordered by date ascending",
      actual: trend[0].date,
      expected: "2026-03-01",
    })

    assert({
      given: "7 days of test run data",
      should: "return the last date as the final entry",
      actual: trend[6].date,
      expected: "2026-03-07",
    })

    assert({
      given: "a day with 2 flaky tests",
      should: "return flakyCount of 2 for that day",
      actual: trend[0].flakyCount,
      expected: 2,
    })

    assert({
      given: "a day with 4 flaky tests",
      should: "return flakyCount of 4 for that day",
      actual: trend[6].flakyCount,
      expected: 4,
    })
  }

  {
    const handle = initDb(":memory:")
    const projectId = await upsertProject(handle.db, "acme", "payments-service")

    // Two runs on the same day — expect summed flaky count
    handle.raw
      .prepare(
        `INSERT INTO test_runs
          (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(projectId, "run-a", "main", "sha-a", "2026-03-10T08:00:00Z", 50, 48, 0, 2)

    handle.raw
      .prepare(
        `INSERT INTO test_runs
          (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(projectId, "run-b", "main", "sha-b", "2026-03-10T14:00:00Z", 50, 47, 0, 3)

    const trend = await getProjectTrend(handle, "acme", "payments-service")

    assert({
      given: "two test runs on the same day",
      should: "aggregate into a single row with summed flakyCount",
      actual: trend.length,
      expected: 1,
    })

    assert({
      given: "two test runs on the same day with 2 and 3 flaky tests respectively",
      should: "return the sum (5) as the flakyCount",
      actual: trend[0].flakyCount,
      expected: 5,
    })
  }

  {
    const handle = initDb(":memory:")

    assert({
      given: "a project that does not exist in the database",
      should: "return an empty array",
      actual: await getProjectTrend(handle, "acme", "nonexistent"),
      expected: [],
    })
  }

  // Sparse data: gap days produce no row (not a zero row)
  {
    const handle = initDb(":memory:")
    const projectId = await upsertProject(handle.db, "acme", "payments-service")

    // Insert runs on March 1 and March 3 — no run on March 2
    handle.raw
      .prepare(
        `INSERT INTO test_runs
          (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(projectId, "sparse-1", "main", "sha-1", "2026-03-01T10:00:00Z", 10, 9, 0, 1)

    handle.raw
      .prepare(
        `INSERT INTO test_runs
          (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(projectId, "sparse-3", "main", "sha-3", "2026-03-03T10:00:00Z", 10, 8, 0, 2)

    const trend = await getProjectTrend(handle, "acme", "payments-service")

    assert({
      given: "runs on March 1 and March 3 with no run on March 2",
      should: "return exactly 2 data points (gap day is absent, not a zero row)",
      actual: trend.length,
      expected: 2,
    })

    assert({
      given: "runs on March 1 and March 3 with no run on March 2",
      should: "not include March 2 in the result",
      actual: trend.some((p) => p.date === "2026-03-02"),
      expected: false,
    })
  }
})
