/**
 * Interface test: Scenario 35 — User with no accessible repos sees empty list.
 *
 * When source: github-app and a logged-in user's access token returns 1
 * installation from GET /user/installations but 0 repos from
 * GET /user/installations/{id}/repositories, GET / must display an empty
 * project list (HTTP 200) — not an error page.
 *
 * Test layer: Interface — exercises the full GET / route through router.fetch(),
 * uses a local mock GitHub server that returns zero repositories.
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

const TEST_SESSION_SECRET = "test-secret"

describe("GET / — user with zero accessible repos sees empty list (not an error)", async (assert) => {
  // Mock GitHub server: returns 1 installation but 0 repos for that installation
  const srv = await new Promise<{
    url: string
    server: ReturnType<typeof createServer>
    close: () => Promise<void>
  }>((resolve) => {
    const s = createServer((req, res) => {
      if (req.url?.startsWith("/user/installations") && !req.url.includes("/repositories")) {
        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(JSON.stringify([{ id: 1, account: { login: "acme" } }]))
        return
      }
      if (req.url?.includes("/user/installations/1/repositories")) {
        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(JSON.stringify({ total_count: 0, repositories: [] }))
        return
      }
      res.writeHead(200, { "Content-Type": "application/json" })
      res.end(JSON.stringify({ total_count: 0, artifacts: [] }))
    })
    s.listen(0, "127.0.0.1", () => {
      const addr = s.address() as AddressInfo
      resolve({
        url: `http://127.0.0.1:${addr.port}`,
        server: s,
        close: () => new Promise((res, rej) => s.close((err) => (err ? rej(err) : res()))),
      })
    })
  })

  // Temp config: source: github-app
  const configPath = join(tmpdir(), `permissions-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")

  // Temp DB: 3 App-discovered projects (ids 201, 202, 203), installation_id=1
  const dbPath = join(tmpdir(), `permissions-db-${randomUUID()}.db`)
  const { raw } = initDb(dbPath)
  raw.prepare("INSERT INTO installations (id, account_login) VALUES (?, ?)").run(1, "acme")
  for (const id of [201, 202, 203]) {
    raw
      .prepare(
        "INSERT INTO projects (owner, repo, installation_id, github_repo_id) VALUES (?, ?, ?, ?)",
      )
      .run("acme", `proj-${id}`, 1, id)
  }
  raw.close()

  const mockFetchFn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    return fetch(url.replace("https://api.github.com", srv.url), init)
  }

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    fetchFn: mockFetchFn,
    getInstallationToken: async () => "ghs_token",
  })

  const cookie = await buildSessionCookieWithAccessToken("ghu_limited_token")
  try {
    const response = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )
    const html = await response.text()

    assert({
      given: "installation exists with 3 app-discovered projects but user has 0 accessible repos",
      should: "return HTTP 200 (not an error page)",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "installation exists with 3 app-discovered projects but user has 0 accessible repos",
      should: "NOT display acme/proj-201 in the HTML",
      actual: html.includes("acme/proj-201"),
      expected: false,
    })

    assert({
      given: "installation exists with 3 app-discovered projects but user has 0 accessible repos",
      should: "NOT display acme/proj-202 in the HTML",
      actual: html.includes("acme/proj-202"),
      expected: false,
    })

    assert({
      given: "installation exists with 3 app-discovered projects but user has 0 accessible repos",
      should: "NOT display acme/proj-203 in the HTML",
      actual: html.includes("acme/proj-203"),
      expected: false,
    })

    assert({
      given: "installation exists with 3 app-discovered projects but user has 0 accessible repos",
      should: "render the page title (Quarantine Dashboard) confirming a normal page render",
      actual: html.includes("Quarantine Dashboard"),
      expected: true,
    })
  } finally {
    await srv.close()
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
