/**
 * Interface test for syncInstallations() pagination behavior.
 *
 * Verifies that syncInstallations correctly follows Link header pagination
 * across multiple installations with different repo counts. Uses a custom
 * HTTP server handler that inspects the Authorization header to differentiate
 * responses per installation.
 */

import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http"
import { describe } from "riteway"
import { initDb } from "../app/lib/db.server.js"
import { type SyncDeps, syncInstallations } from "../app/lib/installation-sync.server.js"

interface RequestEntry {
  method: string
  path: string
}

type RequestHandler = (req: IncomingMessage, res: ServerResponse) => void

function startCustomServer(
  handler: RequestHandler,
): Promise<{ url: string; server: Server; requestLog: RequestEntry[] }> {
  return new Promise((resolve) => {
    const requestLog: RequestEntry[] = []

    const server = createServer((req: IncomingMessage, res: ServerResponse) => {
      const parsedUrl = new URL(req.url ?? "/", "http://localhost")
      const path = parsedUrl.pathname + parsedUrl.search

      requestLog.push({
        method: req.method ?? "GET",
        path,
      })

      handler(req, res)
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

describe("syncInstallations() — paginates through all installations and repos", async (assert) => {
  function createPaginationHandler(baseUrl: string): RequestHandler {
    return (req: IncomingMessage, res: ServerResponse) => {
      const url = new URL(req.url ?? "/", baseUrl)
      const path = url.pathname
      const auth = req.headers.authorization ?? ""

      if (path === "/app/installations") {
        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(
          JSON.stringify([
            { id: 1, account: { login: "org1", id: 100 }, suspended_at: null },
            { id: 2, account: { login: "org2", id: 200 }, suspended_at: null },
          ]),
        )
      } else if (path === "/installation/repositories" && auth === "token mock-token-1") {
        const page = url.searchParams.get("page") ?? "1"
        if (page === "1") {
          const repos = Array.from({ length: 100 }, (_, i) => ({
            owner: { login: "org1" },
            name: `repo-${i + 1}`,
            full_name: `org1/repo-${i + 1}`,
          }))
          res.writeHead(200, {
            "Content-Type": "application/json",
            Link: `<${baseUrl}/installation/repositories?per_page=100&page=2>; rel="next"`,
          })
          res.end(JSON.stringify({ total_count: 110, repositories: repos }))
        } else {
          const repos = Array.from({ length: 10 }, (_, i) => ({
            owner: { login: "org1" },
            name: `repo-${101 + i}`,
            full_name: `org1/repo-${101 + i}`,
          }))
          res.writeHead(200, { "Content-Type": "application/json" })
          res.end(JSON.stringify({ total_count: 110, repositories: repos }))
        }
      } else if (path === "/installation/repositories" && auth === "token mock-token-2") {
        const repos = Array.from({ length: 40 }, (_, i) => ({
          owner: { login: "org2" },
          name: `repo-${i + 1}`,
          full_name: `org2/repo-${i + 1}`,
        }))
        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(JSON.stringify({ total_count: 40, repositories: repos }))
      } else {
        res.writeHead(404, { "Content-Type": "application/json" })
        res.end(JSON.stringify({ message: "Not Found" }))
      }
    }
  }

  // Must create server first to know the baseUrl for Link headers
  let handler: RequestHandler = (_req, res) => {
    res.writeHead(500)
    res.end()
  }
  const { url, server, requestLog } = await startCustomServer((req, res) => handler(req, res))
  handler = createPaginationHandler(url)

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

    // Verify installations table
    const installationRows = raw
      .prepare("SELECT id, account_login FROM installations ORDER BY id")
      .all() as Array<{ id: number; account_login: string }>

    assert({
      given: "2 installations across paginated responses",
      should: "store both installations in the database",
      actual: installationRows.length,
      expected: 2,
    })

    assert({
      given: "installations with logins org1 and org2",
      should: "store the correct account logins",
      actual: installationRows.map((r) => r.account_login),
      expected: ["org1", "org2"],
    })

    // Verify total project count
    const totalProjects = raw.prepare("SELECT COUNT(*) as count FROM projects").get() as {
      count: number
    }

    assert({
      given: "110 repos for installation 1 and 40 repos for installation 2",
      should: "store all 150 projects in the database",
      actual: totalProjects.count,
      expected: 150,
    })

    // Verify projects per installation
    const inst1Projects = raw
      .prepare("SELECT COUNT(*) as count FROM projects WHERE installation_id = ?")
      .get(1) as { count: number }

    assert({
      given: "installation 1 with 110 repos across 2 pages",
      should: "store 110 projects for installation 1",
      actual: inst1Projects.count,
      expected: 110,
    })

    const inst2Projects = raw
      .prepare("SELECT COUNT(*) as count FROM projects WHERE installation_id = ?")
      .get(2) as { count: number }

    assert({
      given: "installation 2 with 40 repos on a single page",
      should: "store 40 projects for installation 2",
      actual: inst2Projects.count,
      expected: 40,
    })

    // Verify that page 2 was actually fetched for installation 1
    const page2Requests = requestLog.filter(
      (r) => r.path.includes("page=2") && r.path.includes("/installation/repositories"),
    )

    assert({
      given: "installation 1 repos spanning 2 pages with a Link header",
      should: "fetch page 2 of the repos list",
      actual: page2Requests.length,
      expected: 1,
    })
  } finally {
    raw.close()
    await closeServer(server)
  }
})
