/**
 * Interface tests for installation and repo removal in syncInstallations().
 *
 * Verifies that removing a repo from an installation (S30) and uninstalling
 * an app entirely (S46) both preserve historical test data while correctly
 * updating installation_id linkage.
 */

import { createServer, type IncomingMessage, type Server } from "node:http"
import { describe } from "riteway"
import { initDb } from "../app/lib/db.server.js"
import { type SyncDeps, syncInstallations } from "../app/lib/installation-sync.server.js"

interface MockRoute {
  status: number
  body: unknown
  headers?: Record<string, string>
}

function startMockServer(
  routes: Record<string, MockRoute>,
): Promise<{ url: string; server: Server }> {
  return new Promise((resolve) => {
    const server = createServer((req: IncomingMessage, res) => {
      const parsedUrl = new URL(req.url ?? "/", "http://localhost")
      const path = parsedUrl.pathname + parsedUrl.search

      const route = routes[path]
      if (route) {
        const headers: Record<string, string> = {
          "Content-Type": "application/json",
          ...(route.headers ?? {}),
        }
        res.writeHead(route.status, headers)
        res.end(JSON.stringify(route.body))
      } else {
        res.writeHead(404, { "Content-Type": "application/json" })
        res.end(JSON.stringify({ message: "Not Found" }))
      }
    })

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address()
      const port = typeof addr === "object" && addr !== null ? addr.port : 0
      resolve({ url: `http://127.0.0.1:${port}`, server })
    })
  })
}

function closeServer(server: Server): Promise<void> {
  return new Promise((resolve) => {
    server.close(() => resolve())
  })
}

describe("syncInstallations() -- repo removed from installation preserves historical data", async (assert) => {
  // Pre-seed: 1 installation with 3 projects, one has test data
  const { raw } = initDb(":memory:")

  raw
    .prepare("INSERT INTO installations (id, account_login, suspended_at) VALUES (?, ?, ?)")
    .run(1, "acme", null)

  const _p1 = raw
    .prepare("INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)")
    .run("acme", "api", 1).lastInsertRowid
  const _p2 = raw
    .prepare("INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)")
    .run("acme", "web", 1).lastInsertRowid
  const p3 = raw
    .prepare("INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)")
    .run("acme", "mobile", 1).lastInsertRowid

  // Seed test data for the project that will be removed
  raw
    .prepare(
      "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests, unresolved_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
    )
    .run(p3, "run-1", "main", "abc123", "2026-04-01T00:00:00Z", 10, 9, 0, 1, 0)
  raw
    .prepare(
      "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at) VALUES (?, ?, ?, ?)",
    )
    .run(p3, "test-1", "flaky checkout test", "2026-04-01T00:00:00Z")

  // Mock: installation still exists, but only 2 repos returned (mobile removed)
  const installations = [
    { id: 1, account: { login: "acme", id: 100 }, suspended_at: null, app_id: 10 },
  ]

  const repoPayload = {
    total_count: 2,
    repositories: [
      { owner: { login: "acme" }, name: "api", full_name: "acme/api" },
      { owner: { login: "acme" }, name: "web", full_name: "acme/web" },
    ],
  }

  const routes: Record<string, MockRoute> = {
    "/app/installations?per_page=100": {
      status: 200,
      body: installations,
    },
    "/installation/repositories?per_page=100": {
      status: 200,
      body: repoPayload,
    },
  }

  const { url, server } = await startMockServer(routes)

  try {
    const deps: SyncDeps = {
      fetchFn: fetch,
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async (id: number) => `mock-token-${id}`,
      log: () => {},
    }

    await syncInstallations(raw, deps)

    // Verify "acme/api" still linked
    const apiProject = raw
      .prepare("SELECT installation_id FROM projects WHERE owner = ? AND repo = ?")
      .get("acme", "api") as { installation_id: number | null }

    assert({
      given: "a repo still present in the installation",
      should: "keep installation_id for acme/api",
      actual: apiProject.installation_id,
      expected: 1,
    })

    // Verify "acme/web" still linked
    const webProject = raw
      .prepare("SELECT installation_id FROM projects WHERE owner = ? AND repo = ?")
      .get("acme", "web") as { installation_id: number | null }

    assert({
      given: "a repo still present in the installation",
      should: "keep installation_id for acme/web",
      actual: webProject.installation_id,
      expected: 1,
    })

    // Verify "acme/mobile" has installation_id = NULL
    const mobileProject = raw
      .prepare("SELECT installation_id FROM projects WHERE owner = ? AND repo = ?")
      .get("acme", "mobile") as { installation_id: number | null }

    assert({
      given: "a repo removed from the installation",
      should: "set installation_id to NULL for acme/mobile",
      actual: mobileProject.installation_id,
      expected: null,
    })

    // Verify test_runs for acme/mobile still exist
    const testRunCount = raw
      .prepare("SELECT COUNT(*) as count FROM test_runs WHERE project_id = ?")
      .get(p3) as { count: number }

    assert({
      given: "a repo removed from the installation with historical test runs",
      should: "preserve the test_runs data",
      actual: testRunCount.count,
      expected: 1,
    })

    // Verify quarantined_tests for acme/mobile still exist
    const qtCount = raw
      .prepare("SELECT COUNT(*) as count FROM quarantined_tests WHERE project_id = ?")
      .get(p3) as { count: number }

    assert({
      given: "a repo removed from the installation with quarantined tests",
      should: "preserve the quarantined_tests data",
      actual: qtCount.count,
      expected: 1,
    })
  } finally {
    raw.close()
    await closeServer(server)
  }
})

describe("syncInstallations() -- uninstalled app detected and marked as removed", async (assert) => {
  // Pre-seed: 2 installations, each with 1 project, org2 has test data
  const { raw } = initDb(":memory:")

  raw
    .prepare("INSERT INTO installations (id, account_login, suspended_at) VALUES (?, ?, ?)")
    .run(1, "org1", null)
  raw
    .prepare("INSERT INTO installations (id, account_login, suspended_at) VALUES (?, ?, ?)")
    .run(2, "org2", null)

  raw
    .prepare("INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)")
    .run("org1", "repo1", 1)
  const p2 = raw
    .prepare("INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)")
    .run("org2", "repo2", 2).lastInsertRowid

  // Seed test data for org2/repo2
  raw
    .prepare(
      "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests, unresolved_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
    )
    .run(p2, "run-2", "main", "def456", "2026-04-02T00:00:00Z", 20, 18, 1, 1, 0)
  raw
    .prepare(
      "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at) VALUES (?, ?, ?, ?)",
    )
    .run(p2, "test-2", "flaky payment test", "2026-04-02T00:00:00Z")

  // Mock: only installation 1 returned (org2 uninstalled)
  const installations = [
    { id: 1, account: { login: "org1", id: 101 }, suspended_at: null, app_id: 10 },
  ]

  const repoPayload = {
    total_count: 1,
    repositories: [{ owner: { login: "org1" }, name: "repo1", full_name: "org1/repo1" }],
  }

  const routes: Record<string, MockRoute> = {
    "/app/installations?per_page=100": {
      status: 200,
      body: installations,
    },
    "/installation/repositories?per_page=100": {
      status: 200,
      body: repoPayload,
    },
  }

  const { url, server } = await startMockServer(routes)

  try {
    const deps: SyncDeps = {
      fetchFn: fetch,
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async (id: number) => `mock-token-${id}`,
      log: () => {},
    }

    await syncInstallations(raw, deps)

    // Verify installation 1 still active
    const inst1 = raw.prepare("SELECT removed_at FROM installations WHERE id = ?").get(1) as {
      removed_at: string | null
    }

    assert({
      given: "an installation still returned by the API",
      should: "keep removed_at NULL for installation 1",
      actual: inst1.removed_at,
      expected: null,
    })

    // Verify installation 2 marked as removed
    const inst2 = raw.prepare("SELECT removed_at FROM installations WHERE id = ?").get(2) as {
      removed_at: string | null
    }

    assert({
      given: "an installation no longer returned by the API",
      should: "set removed_at to a non-NULL timestamp for installation 2",
      actual: inst2.removed_at !== null,
      expected: true,
    })

    // Verify org1/repo1 still linked
    const repo1 = raw
      .prepare("SELECT installation_id FROM projects WHERE owner = ? AND repo = ?")
      .get("org1", "repo1") as { installation_id: number | null }

    assert({
      given: "a project linked to an active installation",
      should: "keep installation_id for org1/repo1",
      actual: repo1.installation_id,
      expected: 1,
    })

    // Verify org2/repo2 has installation_id = NULL
    const repo2 = raw
      .prepare("SELECT installation_id FROM projects WHERE owner = ? AND repo = ?")
      .get("org2", "repo2") as { installation_id: number | null }

    assert({
      given: "a project linked to a removed installation",
      should: "set installation_id to NULL for org2/repo2",
      actual: repo2.installation_id,
      expected: null,
    })

    // Verify test_runs for org2/repo2 still exist
    const testRunCount = raw
      .prepare("SELECT COUNT(*) as count FROM test_runs WHERE project_id = ?")
      .get(p2) as { count: number }

    assert({
      given: "a removed installation with historical test runs",
      should: "preserve the test_runs data",
      actual: testRunCount.count,
      expected: 1,
    })

    // Verify quarantined_tests for org2/repo2 still exist
    const qtCount = raw
      .prepare("SELECT COUNT(*) as count FROM quarantined_tests WHERE project_id = ?")
      .get(p2) as { count: number }

    assert({
      given: "a removed installation with quarantined tests",
      should: "preserve the quarantined_tests data",
      actual: qtCount.count,
      expected: 1,
    })
  } finally {
    raw.close()
    await closeServer(server)
  }
})

describe("syncInstallations() -- previously removed installation reappears after reinstall", async (assert) => {
  // Pre-seed: 1 installation that was previously removed, 1 project with cleared installation_id
  const { raw } = initDb(":memory:")

  raw
    .prepare(
      "INSERT INTO installations (id, account_login, suspended_at, removed_at) VALUES (?, ?, ?, ?)",
    )
    .run(1, "acme", null, "2026-03-15T00:00:00Z")
  raw
    .prepare("INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)")
    .run("acme", "api", null)

  // Mock: installation id=1 is back (reinstalled)
  const installations = [
    { id: 1, account: { login: "acme", id: 100 }, suspended_at: null, app_id: 10 },
  ]

  const repoPayload = {
    total_count: 1,
    repositories: [{ owner: { login: "acme" }, name: "api", full_name: "acme/api" }],
  }

  const routes: Record<string, MockRoute> = {
    "/app/installations?per_page=100": {
      status: 200,
      body: installations,
    },
    "/installation/repositories?per_page=100": {
      status: 200,
      body: repoPayload,
    },
  }

  const { url, server } = await startMockServer(routes)

  try {
    const deps: SyncDeps = {
      fetchFn: fetch,
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async (id: number) => `mock-token-${id}`,
      log: () => {},
    }

    await syncInstallations(raw, deps)

    // Verify installation id=1 has removed_at cleared
    const inst = raw.prepare("SELECT removed_at FROM installations WHERE id = ?").get(1) as {
      removed_at: string | null
    }

    assert({
      given: "a previously removed installation that reappears in the API response",
      should: "clear removed_at to NULL",
      actual: inst.removed_at,
      expected: null,
    })

    // Verify project "acme/api" is re-linked to installation 1
    const project = raw
      .prepare("SELECT installation_id FROM projects WHERE owner = ? AND repo = ?")
      .get("acme", "api") as { installation_id: number | null }

    assert({
      given: "a project whose installation was previously removed and is now reinstalled",
      should: "re-link installation_id to 1",
      actual: project.installation_id,
      expected: 1,
    })
  } finally {
    raw.close()
    await closeServer(server)
  }
})
