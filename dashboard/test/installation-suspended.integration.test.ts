/**
 * Interface tests for suspended_at handling in syncInstallations().
 *
 * Verifies that syncInstallations correctly stores and clears the
 * suspended_at timestamp from the GitHub API response.
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

describe("syncInstallations() — suspended installation stores suspended_at", async (assert) => {
  const installations = [
    {
      id: 1,
      account: { login: "acme", id: 100 },
      suspended_at: "2026-03-15T10:00:00Z",
      app_id: 10,
    },
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
    const deps: SyncDeps = {
      fetchFn: fetch,
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async (id: number) => `mock-token-${id}`,
      log: () => {},
    }

    await syncInstallations(raw, deps)

    const row = raw
      .prepare("SELECT id, account_login, suspended_at FROM installations WHERE id = ?")
      .get(1) as { id: number; account_login: string; suspended_at: string | null } | undefined

    assert({
      given: "an installation with suspended_at '2026-03-15T10:00:00Z'",
      should: "store the suspended_at timestamp in the database",
      actual: row?.suspended_at,
      expected: "2026-03-15T10:00:00Z",
    })
  } finally {
    raw.close()
    await closeServer(server)
  }
})

describe("syncInstallations() — suspended installation becomes active again", async (assert) => {
  const installations = [
    {
      id: 1,
      account: { login: "acme", id: 100 },
      suspended_at: null,
      app_id: 10,
    },
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
    // Pre-seed: installation exists with a non-null suspended_at
    raw
      .prepare("INSERT INTO installations (id, account_login, suspended_at) VALUES (?, ?, ?)")
      .run(1, "acme", "2026-03-15T10:00:00Z")

    // Verify the pre-seed worked
    const before = raw.prepare("SELECT suspended_at FROM installations WHERE id = ?").get(1) as {
      suspended_at: string | null
    }

    assert({
      given: "a pre-seeded installation with suspended_at set",
      should: "have suspended_at before sync",
      actual: before.suspended_at,
      expected: "2026-03-15T10:00:00Z",
    })

    const deps: SyncDeps = {
      fetchFn: fetch,
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async (id: number) => `mock-token-${id}`,
      log: () => {},
    }

    await syncInstallations(raw, deps)

    const row = raw
      .prepare("SELECT id, account_login, suspended_at FROM installations WHERE id = ?")
      .get(1) as { id: number; account_login: string; suspended_at: string | null } | undefined

    assert({
      given: "a previously-suspended installation now returned with suspended_at: null",
      should: "update suspended_at to NULL in the database",
      actual: row?.suspended_at,
      expected: null,
    })
  } finally {
    raw.close()
    await closeServer(server)
  }
})
