import { unlinkSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway"
import { initDb } from "../lib/db.server.js"
import { bodyText } from "../test-helpers.js"
import { project } from "./project.js"

function makeDbWithFilterTests(dbPath: string): void {
  const { raw } = initDb(dbPath)
  const projectId = raw
    .prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)")
    .run("acme", "filter-repo").lastInsertRowid as number

  const tests = [
    {
      test_id: "suite::timeout-db",
      name: "timeout when DB is slow",
      last_run_status: "failing",
      quarantined_at: "2026-01-01T00:00:00Z",
    },
    {
      test_id: "suite::timeout-network",
      name: "timeout on network",
      last_run_status: "passing",
      quarantined_at: "2026-01-02T00:00:00Z",
    },
    {
      test_id: "suite::payment",
      name: "should process payment",
      last_run_status: "failing",
      quarantined_at: "2026-01-03T00:00:00Z",
    },
    {
      test_id: "suite::validate",
      name: "should validate card",
      last_run_status: "passing",
      quarantined_at: "2026-01-04T00:00:00Z",
    },
    {
      test_id: "suite::unknown",
      name: "unknown status test",
      last_run_status: null,
      quarantined_at: "2026-01-05T00:00:00Z",
    },
  ]

  for (const t of tests) {
    raw
      .prepare(
        `INSERT INTO quarantined_tests
          (project_id, test_id, name, quarantined_at, last_run_status)
         VALUES (?, ?, ?, ?, ?)`,
      )
      .run(projectId, t.test_id, t.name, t.quarantined_at, t.last_run_status)
  }

  raw.close()
}

describe("project() — search filter: search=timeout", async (assert) => {
  const dbPath = join(tmpdir(), `project-test-search-${Date.now()}.db`)
  makeDbWithFilterTests(dbPath)

  try {
    const url = "http://localhost/acme/filter-repo?search=timeout"
    const response = await project("acme", "filter-repo", url, dbPath)
    const html = await bodyText(response)

    assert({
      given: "a project with 5 tests and search=timeout in the URL",
      should: "include tests whose name contains 'timeout'",
      actual: html.includes("timeout when DB is slow") && html.includes("timeout on network"),
      expected: true,
    })

    assert({
      given: "a project with 5 tests and search=timeout in the URL",
      should: "exclude tests whose name does not contain 'timeout'",
      actual: html.includes("should process payment"),
      expected: false,
    })

    assert({
      given: "a project with 5 tests and search=timeout in the URL",
      should: "show the filtered count phrase 'Showing 2 of 5 quarantined tests'",
      actual: html.includes("Showing 2 of 5 quarantined tests"),
      expected: true,
    })
  } finally {
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})

describe("project() — status filter: status=failing", async (assert) => {
  const dbPath = join(tmpdir(), `project-test-status-${Date.now()}.db`)
  makeDbWithFilterTests(dbPath)

  try {
    const url = "http://localhost/acme/filter-repo?status=failing"
    const response = await project("acme", "filter-repo", url, dbPath)
    const html = await bodyText(response)

    assert({
      given: "a project with 5 tests and status=failing in the URL",
      should: "include only tests with lastRunStatus failing",
      actual: html.includes("timeout when DB is slow") && html.includes("should process payment"),
      expected: true,
    })

    assert({
      given: "a project with 5 tests and status=failing in the URL",
      should: "exclude passing tests",
      actual: html.includes("should validate card"),
      expected: false,
    })
  } finally {
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})

describe("project() — status filter: status=passing", async (assert) => {
  const dbPath = join(tmpdir(), `project-test-passing-${Date.now()}.db`)
  makeDbWithFilterTests(dbPath)

  try {
    const url = "http://localhost/acme/filter-repo?status=passing"
    const response = await project("acme", "filter-repo", url, dbPath)
    const html = await bodyText(response)

    assert({
      given: "a project with 5 tests and status=passing in the URL",
      should: "include only tests with lastRunStatus passing",
      actual: html.includes("timeout on network") && html.includes("should validate card"),
      expected: true,
    })

    assert({
      given: "a project with 5 tests and status=passing in the URL",
      should: "exclude failing tests",
      actual: html.includes("timeout when DB is slow"),
      expected: false,
    })
  } finally {
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})

describe("project() — combined filters: search=timeout&status=failing", async (assert) => {
  const dbPath = join(tmpdir(), `project-test-combined-${Date.now()}.db`)
  makeDbWithFilterTests(dbPath)

  try {
    const url = "http://localhost/acme/filter-repo?search=timeout&status=failing"
    const response = await project("acme", "filter-repo", url, dbPath)
    const html = await bodyText(response)

    assert({
      given: "a project with search=timeout and status=failing",
      should: "include only 'timeout when DB is slow' (matches both filters)",
      actual: html.includes("timeout when DB is slow"),
      expected: true,
    })

    assert({
      given: "a project with search=timeout and status=failing",
      should: "exclude 'timeout on network' (matches search but not status)",
      actual: html.includes("timeout on network"),
      expected: false,
    })

    assert({
      given: "a project with search=timeout and status=failing",
      should: "show the filtered count phrase 'Showing 1 of 5 quarantined tests'",
      actual: html.includes("Showing 1 of 5 quarantined tests"),
      expected: true,
    })
  } finally {
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})

describe("project() — no filters: shows all tests with total count", async (assert) => {
  const dbPath = join(tmpdir(), `project-test-nofilter-${Date.now()}.db`)
  makeDbWithFilterTests(dbPath)

  try {
    const url = "http://localhost/acme/filter-repo"
    const response = await project("acme", "filter-repo", url, dbPath)
    const html = await bodyText(response)

    assert({
      given: "a project with 5 tests and no filters",
      should: "show all 5 tests",
      actual:
        html.includes("timeout when DB is slow") &&
        html.includes("timeout on network") &&
        html.includes("should process payment") &&
        html.includes("should validate card") &&
        html.includes("unknown status test"),
      expected: true,
    })

    assert({
      given: "a project with 5 tests and no filters",
      should: "show the count phrase 'Showing 5 of 5 quarantined tests'",
      actual: html.includes("Showing 5 of 5 quarantined tests"),
      expected: true,
    })
  } finally {
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})

describe("project() — date range filter: from and until", async (assert) => {
  const dbPath = join(tmpdir(), `project-test-daterange-${Date.now()}.db`)
  makeDbWithFilterTests(dbPath)

  try {
    // fixture has quarantined_at dates: 2026-01-01, 2026-01-02, 2026-01-03, 2026-01-04, 2026-01-05
    const url = "http://localhost/acme/filter-repo?from=2026-01-02&until=2026-01-04"
    const response = await project("acme", "filter-repo", url, dbPath)
    const html = await bodyText(response)

    assert({
      given: "from=2026-01-02 and until=2026-01-04",
      should: "include tests quarantined within the range",
      actual:
        html.includes("timeout on network") &&
        html.includes("should process payment") &&
        html.includes("should validate card"),
      expected: true,
    })

    assert({
      given: "from=2026-01-02 and until=2026-01-04",
      should: "exclude tests outside the range (quarantined before or after)",
      actual: html.includes("timeout when DB is slow") || html.includes("unknown status test"),
      expected: false,
    })

    assert({
      given: "from=2026-01-02 and until=2026-01-04 (3 of 5 tests match)",
      should: "show the filtered count phrase 'Showing 3 of 5 quarantined tests'",
      actual: html.includes("Showing 3 of 5 quarantined tests"),
      expected: true,
    })
  } finally {
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})
