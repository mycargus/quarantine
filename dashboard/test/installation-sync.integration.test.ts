/**
 * Interface test for syncInstallations().
 *
 * Exercises the installation sync through its public API with a local HTTP
 * server standing in for GitHub's /app/installations and
 * /installation/repositories endpoints. Uses a real SQLite database.
 */

import { createServer, type IncomingMessage, type Server } from "node:http"
import { describe } from "riteway"
import { initDb } from "../app/lib/db.server.js"
import {
  syncInstallations,
  startGitHubAppMode,
  type SyncDeps,
} from "../app/lib/installation-sync.server.js"

interface RequestEntry {
  method: string
  path: string
}

interface MockRoute {
  status: number
  body: unknown
  headers?: Record<string, string>
}

function startMockServer(
  routes: Record<string, MockRoute>,
): Promise<{ url: string; server: Server; requestLog: RequestEntry[] }> {
  return new Promise((resolve) => {
    const requestLog: RequestEntry[] = []

    const server = createServer((req: IncomingMessage, res) => {
      const parsedUrl = new URL(req.url ?? "/", "http://localhost")
      const path = parsedUrl.pathname + parsedUrl.search

      requestLog.push({
        method: req.method ?? "GET",
        path,
      })

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
      resolve({ url: `http://127.0.0.1:${port}`, server, requestLog })
    })
  })
}

function closeServer(server: Server): Promise<void> {
  return new Promise((resolve) => {
    server.close(() => resolve())
  })
}

describe("syncInstallations() — single page, no Link header", async (assert) => {
  const installations = [
    { id: 1, account: { login: "org1", id: 101 }, suspended_at: null, app_id: 10 },
    { id: 2, account: { login: "org2", id: 102 }, suspended_at: null, app_id: 10 },
    { id: 3, account: { login: "org3", id: 103 }, suspended_at: null, app_id: 10 },
  ]

  const emptyRepos = { total_count: 0, repositories: [] }

  const routes: Record<string, MockRoute> = {
    "/app/installations?per_page=100": {
      status: 200,
      body: installations,
      // No Link header — single page
    },
    "/installation/repositories?per_page=100": {
      status: 200,
      body: emptyRepos,
    },
  }

  const { url, server, requestLog } = await startMockServer(routes)
  const { raw } = initDb(":memory:")

  try {
    const logs: string[] = []
    const deps: SyncDeps = {
      fetchFn: fetch,
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async (id: number) => `mock-token-${id}`,
      log: (msg: string) => logs.push(msg),
    }

    await syncInstallations(raw, deps)

    const installationRequests = requestLog.filter(
      (r) => r.path === "/app/installations?per_page=100",
    )

    assert({
      given: "a single-page installations response with no Link header",
      should: "call GET /app/installations exactly once",
      actual: installationRequests.length,
      expected: 1,
    })

    const anySecondPage = requestLog.some(
      (r) => r.path.includes("/app/installations") && r.path.includes("page=2"),
    )

    assert({
      given: "a single-page installations response with no Link header",
      should: "not request a second page",
      actual: anySecondPage,
      expected: false,
    })

    const rows = raw.prepare("SELECT id, account_login, suspended_at, removed_at FROM installations ORDER BY id").all() as Array<{
      id: number
      account_login: string
      suspended_at: string | null
      removed_at: string | null
    }>

    assert({
      given: "3 installations in the API response",
      should: "upsert all 3 installations into the database",
      actual: rows.length,
      expected: 3,
    })

    assert({
      given: "3 installations in the API response",
      should: "store the correct account logins",
      actual: rows.map((r) => r.account_login),
      expected: ["org1", "org2", "org3"],
    })

    assert({
      given: "syncInstallations completing successfully",
      should: "not throw an error",
      actual: true,
      expected: true,
    })
  } finally {
    raw.close()
    await closeServer(server)
  }
})

describe("startGitHubAppMode() — zero installations at startup", async (assert) => {
  const routes: Record<string, MockRoute> = {
    "/app/installations?per_page=100": {
      status: 200,
      body: [],
      // No Link header — empty single page
    },
  }

  const { url, server } = await startMockServer(routes)

  try {
    const logs: string[] = []

    const { raw } = await startGitHubAppMode({
      dbPath: ":memory:",
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async () => "mock-installation-token",
      log: (msg: string) => logs.push(msg),
    })

    try {
      // Verify installations table has 0 rows
      const installationRows = raw
        .prepare("SELECT id FROM installations")
        .all()

      assert({
        given: "zero installations returned by the API",
        should: "leave the installations table empty",
        actual: installationRows.length,
        expected: 0,
      })

      // Verify projects table has 0 rows with installation_id IS NOT NULL
      const projectRows = raw
        .prepare("SELECT owner, repo FROM projects WHERE installation_id IS NOT NULL")
        .all()

      assert({
        given: "zero installations returned by the API",
        should: "leave the projects table empty",
        actual: projectRows.length,
        expected: 0,
      })

      // Verify function completed without error (reaching here is the proof)
      assert({
        given: "zero installations returned by the API",
        should: "complete startup without error",
        actual: true,
        expected: true,
      })
    } finally {
      raw.close()
    }
  } finally {
    await closeServer(server)
  }
})

describe("syncInstallations() — installation with zero repositories", async (assert) => {
  const installations = [
    { id: 1, account: { login: "acme", id: 100 }, suspended_at: null, app_id: 10 },
  ]

  const emptyRepos = { total_count: 0, repositories: [] }

  const routes: Record<string, MockRoute> = {
    "/app/installations?per_page=100": {
      status: 200,
      body: installations,
    },
    "/installation/repositories?per_page=100": {
      status: 200,
      body: emptyRepos,
    },
  }

  const { url, server } = await startMockServer(routes)
  const { raw } = initDb(":memory:")

  try {
    const logs: string[] = []
    const deps: SyncDeps = {
      fetchFn: fetch,
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async (id: number) => `mock-token-${id}`,
      log: (msg: string) => logs.push(msg),
    }

    await syncInstallations(raw, deps)

    // Verify the installation is upserted
    const installationRows = raw
      .prepare("SELECT id, account_login FROM installations ORDER BY id")
      .all() as Array<{ id: number; account_login: string }>

    assert({
      given: "one installation with zero repositories",
      should: "upsert the installation into the installations table",
      actual: installationRows.length,
      expected: 1,
    })

    assert({
      given: "one installation with account login 'acme'",
      should: "store the correct account login",
      actual: installationRows[0]?.account_login,
      expected: "acme",
    })

    // Verify no projects are linked to that installation
    const projectRows = raw
      .prepare("SELECT owner, repo FROM projects WHERE installation_id = ?")
      .all(1) as Array<{ owner: string; repo: string }>

    assert({
      given: "an installation whose repo list is empty",
      should: "have zero project rows linked to that installation",
      actual: projectRows.length,
      expected: 0,
    })
  } finally {
    raw.close()
    await closeServer(server)
  }
})

describe("startGitHubAppMode() — startup sync", async (assert) => {
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

  const { url, server, requestLog } = await startMockServer(routes)

  try {
    const logs: string[] = []

    const { raw } = await startGitHubAppMode({
      dbPath: ":memory:",
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async () => "mock-installation-token",
      log: (msg: string) => logs.push(msg),
    })

    try {
      // Verify installations table has 1 row
      const installationRows = raw
        .prepare("SELECT id, account_login FROM installations ORDER BY id")
        .all() as Array<{ id: number; account_login: string }>

      assert({
        given: "1 installation in the API response",
        should: "populate the installations table with 1 row",
        actual: installationRows.length,
        expected: 1,
      })

      assert({
        given: "an installation with account login 'acme'",
        should: "store the correct account login",
        actual: installationRows[0]?.account_login,
        expected: "acme",
      })

      // Verify projects table has 2 rows with correct installation_id
      const projectRows = raw
        .prepare(
          "SELECT owner, repo, installation_id FROM projects ORDER BY repo",
        )
        .all() as Array<{ owner: string; repo: string; installation_id: number }>

      assert({
        given: "2 repos for the installation",
        should: "populate the projects table with 2 rows",
        actual: projectRows.length,
        expected: 2,
      })

      assert({
        given: "2 repos for installation id 1",
        should: "set installation_id on each project row",
        actual: projectRows.map((r) => r.installation_id),
        expected: [1, 1],
      })

      assert({
        given: "repos named 'api' and 'web'",
        should: "store the correct repo names",
        actual: projectRows.map((r) => `${r.owner}/${r.repo}`),
        expected: ["acme/api", "acme/web"],
      })

      // Verify request order: installations first, then repos
      const relevantRequests = requestLog.map((r) => r.path)

      assert({
        given: "startup sync completing",
        should: "call /app/installations before /installation/repositories",
        actual: relevantRequests,
        expected: [
          "/app/installations?per_page=100",
          "/installation/repositories?per_page=100",
        ],
      })
    } finally {
      raw.close()
    }
  } finally {
    await closeServer(server)
  }
})

describe("syncInstallations() — API returns 500", async (assert) => {
  const routes: Record<string, MockRoute> = {
    "/app/installations?per_page=100": {
      status: 500,
      body: { message: "Internal Server Error" },
    },
  }

  const { url, server } = await startMockServer(routes)
  const { raw } = initDb(":memory:")

  try {
    // Seed pre-existing data to verify it survives the failure
    raw
      .prepare(
        "INSERT INTO installations (id, account_login, suspended_at) VALUES (?, ?, ?)",
      )
      .run(99, "existing-org", null)
    raw
      .prepare(
        "INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)",
      )
      .run("existing-org", "existing-repo", 99)

    const logs: string[] = []
    const deps: SyncDeps = {
      fetchFn: fetch,
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async (id: number) => `mock-token-${id}`,
      log: (msg: string) => logs.push(msg),
    }

    let thrownError: Error | null = null
    try {
      await syncInstallations(raw, deps)
    } catch (err) {
      thrownError = err instanceof Error ? err : new Error(String(err))
    }

    assert({
      given: "GET /app/installations returns 500",
      should: "not throw an error",
      actual: thrownError,
      expected: null,
    })

    // Verify installations table still has the pre-existing row
    const installationRows = raw
      .prepare("SELECT id, account_login FROM installations")
      .all() as Array<{ id: number; account_login: string }>

    assert({
      given: "GET /app/installations returns 500",
      should: "leave the pre-existing installation unchanged",
      actual: installationRows,
      expected: [{ id: 99, account_login: "existing-org" }],
    })

    // Verify projects table still has the pre-existing row
    const projectRows = raw
      .prepare("SELECT owner, repo, installation_id FROM projects")
      .all() as Array<{ owner: string; repo: string; installation_id: number }>

    assert({
      given: "GET /app/installations returns 500",
      should: "leave the pre-existing project unchanged",
      actual: projectRows,
      expected: [
        { owner: "existing-org", repo: "existing-repo", installation_id: 99 },
      ],
    })

    // Verify the error was logged
    const hasErrorLog = logs.some((msg) => /error/i.test(msg))

    assert({
      given: "GET /app/installations returns 500",
      should: "log an error message",
      actual: hasErrorLog,
      expected: true,
    })
  } finally {
    raw.close()
    await closeServer(server)
  }
})
