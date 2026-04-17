/**
 * Interface test: Scenario 48 — User permission filtering paginates through
 * all accessible repos.
 *
 * When source: github-app and a logged-in user has access to 120 repos all
 * under a single installation delivered over 2 pages of /user/installations/{id}/repositories,
 * GET / must follow the Link rel="next" header and include repos from BOTH pages
 * in the filtered project list.
 *
 * Test layer: Interface — exercises the full GET / route through router.fetch(),
 * uses a local mock GitHub server that returns paginated responses with Link headers.
 */

import { randomUUID } from "node:crypto"
import { unlinkSync, writeFileSync } from "node:fs"
import { createServer } from "node:http"
import type { AddressInfo } from "node:net"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway"
import { createApp } from "../app/app.js"
import { initDb } from "../app/lib/db.server.js"
import { buildSessionCookieWithAccessToken } from "./helpers.js"

/**
 * Starts a local mock HTTP server that serves paginated responses:
 *
 * Installations (2 pages, but only 1 installation total):
 *   GET /user/installations?per_page=100          → [{id:1}] + Link next (page 2)
 *   GET /user/installations?page=2&per_page=100   → [] (no more)
 *
 * Repos for installation 1 (2 pages, 120 repos total):
 *   GET /user/installations/1/repositories?per_page=100         → repos 1001–1100 + Link next
 *   GET /user/installations/1/repositories?page=2&per_page=100  → repos 1101–1120, no Link
 *
 * All other endpoints → {total_count:0, artifacts:[]}
 *
 * The mock server must know its own base URL to construct proper Link headers,
 * so baseUrl is captured after listen() and used to build the Link header values.
 */
function startPaginatedMockServer(): Promise<{
  server: ReturnType<typeof createServer>
  baseUrl: string
  close: () => Promise<void>
}> {
  return new Promise((resolve) => {
    let capturedBaseUrl = ""

    const server = createServer((req, res) => {
      const url = req.url ?? ""

      // ----- GET /user/installations -----
      if (url.startsWith("/user/installations") && !url.includes("/repositories")) {
        const isPage2 = url.includes("page=2")

        if (isPage2) {
          // Page 2: no more installations
          res.writeHead(200, { "Content-Type": "application/json" })
          res.end(JSON.stringify([]))
          return
        }

        // Page 1: installation id=1, plus Link: next pointing to page 2
        const nextUrl = `${capturedBaseUrl}/user/installations?page=2&per_page=100`
        res.writeHead(200, {
          "Content-Type": "application/json",
          Link: `<${nextUrl}>; rel="next"`,
        })
        res.end(JSON.stringify([{ id: 1, account: { login: "acme" } }]))
        return
      }

      // ----- GET /user/installations/1/repositories -----
      if (url.startsWith("/user/installations/1/repositories")) {
        const isPage2 = url.includes("page=2")

        if (isPage2) {
          // Page 2: repos 1101–1120 (20 repos), no Link header
          const repos = Array.from({ length: 20 }, (_, i) => ({
            id: 1101 + i,
            name: `repo-${1101 + i}`,
            owner: { login: "acme" },
          }))
          res.writeHead(200, { "Content-Type": "application/json" })
          res.end(JSON.stringify({ total_count: 20, repositories: repos }))
          return
        }

        // Page 1: repos 1001–1100 (100 repos) + Link: next pointing to page 2
        const repos = Array.from({ length: 100 }, (_, i) => ({
          id: 1001 + i,
          name: `repo-${1001 + i}`,
          owner: { login: "acme" },
        }))
        const nextUrl = `${capturedBaseUrl}/user/installations/1/repositories?page=2&per_page=100`
        res.writeHead(200, {
          "Content-Type": "application/json",
          Link: `<${nextUrl}>; rel="next"`,
        })
        res.end(JSON.stringify({ total_count: 120, repositories: repos }))
        return
      }

      // All other endpoints (artifacts, etc.)
      res.writeHead(200, { "Content-Type": "application/json" })
      res.end(JSON.stringify({ total_count: 0, artifacts: [] }))
    })

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address() as AddressInfo
      capturedBaseUrl = `http://127.0.0.1:${addr.port}`
      resolve({
        server,
        baseUrl: capturedBaseUrl,
        close: () =>
          new Promise((res, rej) => {
            server.close((err) => (err ? rej(err) : res()))
          }),
      })
    })
  })
}

describe("GET / — user permission filtering paginates through all accessible repos", async (assert) => {
  const { baseUrl, close } = await startPaginatedMockServer()

  // Config: source: github-app
  const configPath = join(tmpdir(), `pagination-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")

  // DB: 120 App-discovered projects with github_repo_id 1001–1120
  // plus one extra project (id=1121) that is NOT in the accessible set
  const dbPath = join(tmpdir(), `pagination-db-${randomUUID()}.db`)
  const { raw } = initDb(dbPath)
  raw.prepare("INSERT INTO installations (id, account_login) VALUES (?, ?)").run(1, "acme")
  for (let id = 1001; id <= 1121; id++) {
    raw
      .prepare(
        "INSERT INTO projects (owner, repo, installation_id, github_repo_id) VALUES (?, ?, ?, ?)",
      )
      .run("acme", `repo-${id}`, 1, id)
  }
  raw.close()

  // mockFetchFn: rewrites https://api.github.com → mock server URL
  const mockFetchFn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    return fetch(url.replace("https://api.github.com", baseUrl), init)
  }

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: "test-secret",
    fetchFn: mockFetchFn,
    getInstallationToken: async () => "ghs_pagination_token",
  })

  const cookie = await buildSessionCookieWithAccessToken("ghu_pagination_token")
  const request = new Request("http://localhost/", {
    headers: { Cookie: cookie },
  })

  try {
    const response = await router.fetch(request)
    const html = await response.text()

    assert({
      given:
        "a user with access to 120 repos (2 pages) under installation 1 and 121 App-discovered projects",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a user with access to repos 1001–1120 delivered over 2 pages via Link rel=next",
      should: "include acme/repo-1001 from page 1 of installation repos",
      actual: html.includes("acme/repo-1001"),
      expected: true,
    })

    assert({
      given: "a user with access to repos 1001–1120 delivered over 2 pages via Link rel=next",
      should:
        "include acme/repo-1101 from page 2 of installation repos (proves pagination followed)",
      actual: html.includes("acme/repo-1101"),
      expected: true,
    })

    assert({
      given: "a user with access to repos 1001–1120 delivered over 2 pages via Link rel=next",
      should: "NOT include acme/repo-1121 which is not in the accessible set",
      actual: html.includes("acme/repo-1121"),
      expected: false,
    })
  } finally {
    await close()
    try {
      unlinkSync(configPath)
    } catch {
      /* ignore */
    }
    try {
      unlinkSync(dbPath)
    } catch {
      /* ignore */
    }
  }
})
