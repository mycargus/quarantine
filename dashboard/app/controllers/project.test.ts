import { unlinkSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway"
import { initDb } from "../lib/db.server.js"
import { project } from "./project.js"

async function bodyText(response: Response): Promise<string> {
  return new Response(response.body).text()
}

describe("project() — project not found", async (assert) => {
  const dbPath = join(tmpdir(), `project-test-notfound-${Date.now()}.db`)
  const origDb = process.env.DATABASE_URL
  process.env.DATABASE_URL = dbPath

  try {
    const response = await project("acme", "missing-repo")
    const html = await bodyText(response)

    assert({
      given: "a request for a project that does not exist",
      should: "return HTTP 404",
      actual: response.status,
      expected: 404,
    })

    assert({
      given: "a request for a project that does not exist",
      should: "return HTML content-type",
      actual: response.headers.get("Content-Type"),
      expected: "text/html; charset=utf-8",
    })

    assert({
      given: "a request for a project that does not exist",
      should: "include 'Not Found' in the body",
      actual: html.includes("Not Found"),
      expected: true,
    })
  } finally {
    if (origDb === undefined) {
      delete process.env.DATABASE_URL
    } else {
      process.env.DATABASE_URL = origDb
    }
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})

describe("project() — project exists with 3 quarantined tests", async (assert) => {
  const dbPath = join(tmpdir(), `project-test-detail-${Date.now()}.db`)
  const origDb = process.env.DATABASE_URL
  process.env.DATABASE_URL = dbPath

  const { raw } = initDb(dbPath)
  const projectId = raw
    .prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)")
    .run("acme", "payments-service").lastInsertRowid as number

  raw
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

  raw
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

  raw
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

  // Insert trend data: 3 days of test runs
  raw
    .prepare(
      `INSERT INTO test_runs
        (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    )
    .run(projectId, "run-1", "main", "sha-1", "2026-03-18T10:00:00Z", 50, 48, 0, 2)

  raw
    .prepare(
      `INSERT INTO test_runs
        (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    )
    .run(projectId, "run-2", "main", "sha-2", "2026-03-19T10:00:00Z", 50, 49, 0, 1)

  raw
    .prepare(
      `INSERT INTO test_runs
        (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    )
    .run(projectId, "run-3", "main", "sha-3", "2026-03-20T10:00:00Z", 50, 47, 0, 3)

  raw.close()

  try {
    const response = await project("acme", "payments-service")
    const html = await bodyText(response)

    assert({
      given: "a project with 3 quarantined tests",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a project with 3 quarantined tests",
      should: "return HTML content-type",
      actual: response.headers.get("Content-Type"),
      expected: "text/html; charset=utf-8",
    })

    assert({
      given: "a project with 3 quarantined tests",
      should: "include the repository name in the page",
      actual: html.includes("acme/payments-service"),
      expected: true,
    })

    assert({
      given: "a project with 3 quarantined tests",
      should: "include the first test name",
      actual: html.includes("should process payment"),
      expected: true,
    })

    assert({
      given: "a project with 3 quarantined tests",
      should: "include the second test name",
      actual: html.includes("should refund payment"),
      expected: true,
    })

    assert({
      given: "a project with 3 quarantined tests",
      should: "include the third test name",
      actual: html.includes("should validate card"),
      expected: true,
    })

    assert({
      given: "a quarantined test with a date first quarantined",
      should: "include the quarantinedAt date in the page",
      actual: html.includes("2026-01-10"),
      expected: true,
    })

    assert({
      given: "a quarantined test with a lastFlakyAt date",
      should: "include the lastFlakyAt date in the page",
      actual: html.includes("2026-03-20"),
      expected: true,
    })

    assert({
      given: "a quarantined test with null last_flaky_at",
      should: "show 'Never' for that test",
      actual: html.includes("Never"),
      expected: true,
    })

    assert({
      given: "a quarantined test with issue_number and issue_url",
      should: "render a link with the issue number",
      actual: html.includes('href="https://github.com/acme/payments-service/issues/42"'),
      expected: true,
    })

    assert({
      given: "a quarantined test with null issue_url",
      should: "render '—' in the issue column",
      actual: html.includes("—"),
      expected: true,
    })

    assert({
      given: "a project with trend data",
      should: "include trend dates in the page",
      actual:
        html.includes("2026-03-18") && html.includes("2026-03-19") && html.includes("2026-03-20"),
      expected: true,
    })

    assert({
      given: "a project with trend data",
      should: "render each date adjacent to its flaky count in a table row",
      actual:
        html.includes("2026-03-18</td><td>2</td>") &&
        html.includes("2026-03-19</td><td>1</td>") &&
        html.includes("2026-03-20</td><td>3</td>"),
      expected: true,
    })
  } finally {
    if (origDb === undefined) {
      delete process.env.DATABASE_URL
    } else {
      process.env.DATABASE_URL = origDb
    }
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})
