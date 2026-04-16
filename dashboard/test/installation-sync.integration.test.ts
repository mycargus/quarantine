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
import { syncInstallations, type SyncDeps } from "../app/lib/installation-sync.server.js"

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
