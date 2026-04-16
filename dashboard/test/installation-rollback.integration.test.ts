/**
 * Interface test for transaction rollback in syncInstallations().
 *
 * Verifies that when an upsert fails mid-transaction (simulated via a SQLite
 * BEFORE INSERT trigger), the entire transaction is rolled back, preserving
 * pre-sync database state. Uses a real SQLite database and a local HTTP server
 * standing in for GitHub's API.
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

describe("syncInstallations() -- partial upsert failure triggers full transaction rollback", async (assert) => {
  const { raw } = initDb(":memory:")

  // Pre-seed: 1 existing installation and 1 existing project
  raw
    .prepare(
      "INSERT INTO installations (id, account_login, suspended_at) VALUES (?, ?, ?)",
    )
    .run(99, "pre-existing", null)
  raw
    .prepare(
      "INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)",
    )
    .run("pre-existing", "old-repo", 99)

  // Create a SQLite trigger that raises an error when installation id=2 is inserted.
  // This simulates a constraint violation mid-transaction. The trigger fires on
  // INSERT (which is what ON CONFLICT ... DO UPDATE triggers for new rows).
  raw.exec(`
    CREATE TRIGGER force_error_on_install_2
    BEFORE INSERT ON installations
    WHEN NEW.id = 2
    BEGIN
      SELECT RAISE(ABORT, 'simulated constraint violation');
    END;
  `)

  // Mock server: 2 installations (id=1 succeeds, id=2 blocked by trigger)
  const installations = [
    { id: 1, account: { login: "org-one" }, suspended_at: null },
    { id: 2, account: { login: "org-two" }, suspended_at: null },
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

  try {
    const logs: string[] = []
    const deps: SyncDeps = {
      fetchFn: fetch,
      baseUrl: url,
      jwtToken: "mock-jwt-token",
      getInstallationToken: async (id: number) => `mock-token-${id}`,
      log: (msg: string) => logs.push(msg),
    }

    // syncInstallations should not throw
    let thrownError: Error | null = null
    try {
      await syncInstallations(raw, deps)
    } catch (err) {
      thrownError = err instanceof Error ? err : new Error(String(err))
    }

    assert({
      given: "a partial upsert failure inside the transaction",
      should: "not throw an error from syncInstallations",
      actual: thrownError,
      expected: null,
    })

    // Pre-existing installation id=99 should still be present (rollback preserved it)
    const preExistingInstallation = raw
      .prepare("SELECT id, account_login FROM installations WHERE id = ?")
      .get(99) as { id: number; account_login: string } | undefined

    assert({
      given: "a partial upsert failure that triggers rollback",
      should: "preserve the pre-existing installation (id=99)",
      actual: preExistingInstallation,
      expected: { id: 99, account_login: "pre-existing" },
    })

    // Installation id=1 should NOT exist (the entire transaction was rolled back)
    const rolledBackInstallation = raw
      .prepare("SELECT id FROM installations WHERE id = ?")
      .get(1) as { id: number } | undefined

    assert({
      given: "a partial upsert failure that triggers rollback",
      should: "not have installation id=1 (rolled back)",
      actual: rolledBackInstallation,
      expected: undefined,
    })

    // Pre-existing project should still have installation_id=99 (unchanged)
    const preExistingProject = raw
      .prepare(
        "SELECT owner, repo, installation_id FROM projects WHERE owner = ? AND repo = ?",
      )
      .get("pre-existing", "old-repo") as {
      owner: string
      repo: string
      installation_id: number | null
    }

    assert({
      given: "a partial upsert failure that triggers rollback",
      should: "preserve the pre-existing project with installation_id=99",
      actual: preExistingProject,
      expected: {
        owner: "pre-existing",
        repo: "old-repo",
        installation_id: 99,
      },
    })

    // Error should have been logged
    const hasErrorLog = logs.some((msg) => /error/i.test(msg))

    assert({
      given: "a partial upsert failure inside the transaction",
      should: "log an error message",
      actual: hasErrorLog,
      expected: true,
    })
  } finally {
    raw.close()
    await closeServer(server)
  }
})
