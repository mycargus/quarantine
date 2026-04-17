/**
 * Interface test: Scenario 34 — User sees only repos they have access to.
 *
 * When source: github-app and a logged-in user has an access token, GET /
 * must fetch the user's accessible repos via /user/installations and
 * /user/installations/{id}/repositories, intersect with App-discovered
 * projects, and display only the 3 repos the user can access.
 *
 * Test layer: Interface — exercises the full GET / route through router.fetch(),
 * uses a local mock GitHub server for the user-permissions API calls.
 */

import { randomUUID } from "node:crypto"
import { unlinkSync, writeFileSync } from "node:fs"
import { createServer } from "node:http"
import type { AddressInfo } from "node:net"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { createSession } from "@remix-run/session"
import { createCookie } from "remix/cookie"
import { describe } from "riteway"
import { createApp } from "../app/app.js"
import { initDb } from "../app/lib/db.server.js"
import { buildSessionCookieWithAccessToken } from "./helpers.js"

const TEST_SESSION_SECRET = "test-secret"

/**
 * Starts a local mock HTTP server that handles user-permissions API calls.
 * Returns installation list for installation id=1 with 3 repos (ids 101–103).
 */
function startMockPermissionsServer(): Promise<{
  server: ReturnType<typeof createServer>
  baseUrl: string
  close: () => Promise<void>
}> {
  return new Promise((resolve) => {
    const server = createServer((req, res) => {
      if (req.url?.startsWith("/user/installations") && !req.url.includes("/repositories")) {
        // GET /user/installations?per_page=100
        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(JSON.stringify([{ id: 1, account: { login: "acme" } }]))
        return
      }

      if (req.url?.startsWith("/user/installations/1/repositories")) {
        // GET /user/installations/1/repositories?per_page=100
        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(
          JSON.stringify({
            total_count: 3,
            repositories: [
              { id: 101, name: "repo-101", owner: { login: "acme" } },
              { id: 102, name: "repo-102", owner: { login: "acme" } },
              { id: 103, name: "repo-103", owner: { login: "acme" } },
            ],
          }),
        )
        return
      }

      // All other endpoints (artifacts, etc.) return empty result
      res.writeHead(200, { "Content-Type": "application/json" })
      res.end(JSON.stringify({ total_count: 0, artifacts: [] }))
    })

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address() as AddressInfo
      const baseUrl = `http://127.0.0.1:${addr.port}`
      resolve({
        server,
        baseUrl,
        close: () =>
          new Promise((res, rej) => {
            server.close((err) => (err ? rej(err) : res()))
          }),
      })
    })
  })
}

/**
 * Helper: writes a temp github-app config, seeds one installation + one project,
 * and returns {configPath, dbPath, cleanup}.
 */
function setupAppModeFixture(installationId = 1, githubRepoId = 101) {
  const configPath = join(tmpdir(), `permissions-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")
  const dbPath = join(tmpdir(), `permissions-db-${randomUUID()}.db`)
  const { raw } = initDb(dbPath)
  raw
    .prepare("INSERT INTO installations (id, account_login) VALUES (?, ?)")
    .run(installationId, "acme")
  raw
    .prepare(
      "INSERT INTO projects (owner, repo, installation_id, github_repo_id) VALUES (?, ?, ?, ?)",
    )
    .run("acme", `repo-${githubRepoId}`, installationId, githubRepoId)
  raw.close()
  return {
    configPath,
    dbPath,
    cleanup() {
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
    },
  }
}

describe("GET / — 401 from /user/installations returns HTTP 401 (expired token, not degradation)", async (assert) => {
  const { configPath, dbPath, cleanup } = setupAppModeFixture()

  // Mock server always returns 401 on /user/installations
  const server = await new Promise<{
    url: string
    server: ReturnType<typeof createServer>
    close: () => Promise<void>
  }>((resolve) => {
    const srv = createServer((_req, res) => {
      res.writeHead(401, { "Content-Type": "application/json" })
      res.end(JSON.stringify({ message: "Bad credentials" }))
    })
    srv.listen(0, "127.0.0.1", () => {
      const addr = srv.address() as AddressInfo
      resolve({
        url: `http://127.0.0.1:${addr.port}`,
        server: srv,
        close: () => new Promise((res, rej) => srv.close((err) => (err ? rej(err) : res()))),
      })
    })
  })

  const mockFetchFn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    return fetch(url.replace("https://api.github.com", server.url), init)
  }

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    fetchFn: mockFetchFn,
    getInstallationToken: async () => "ghs_token",
  })

  const cookie = await buildSessionCookieWithAccessToken("ghu_expired_token")
  try {
    const response = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )

    assert({
      given: "GET /user/installations returns 401 (expired token)",
      should: "return HTTP 401 to the browser (treat as invalid session, prompt re-auth)",
      actual: response.status,
      expected: 401,
    })
  } finally {
    await server.close()
    cleanup()
  }
})

describe("GET / — empty installations from /user/installations shows empty project list", async (assert) => {
  const { configPath, dbPath, cleanup } = setupAppModeFixture()

  const srv = await new Promise<{ url: string; close: () => Promise<void> }>((resolve) => {
    const s = createServer((_req, res) => {
      res.writeHead(200, { "Content-Type": "application/json" })
      // /user/installations returns empty array
      res.end(JSON.stringify([]))
    })
    s.listen(0, "127.0.0.1", () => {
      const addr = s.address() as AddressInfo
      resolve({
        url: `http://127.0.0.1:${addr.port}`,
        close: () => new Promise((res, rej) => s.close((err) => (err ? rej(err) : res()))),
      })
    })
  })

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

  const cookie = await buildSessionCookieWithAccessToken("ghu_test_token")
  try {
    const response = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )
    const html = await response.text()

    assert({
      given: "GET /user/installations returns empty array",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "GET /user/installations returns empty array",
      should: "NOT display acme/repo-101 (zero accessible repos)",
      actual: html.includes("acme/repo-101"),
      expected: false,
    })
  } finally {
    await srv.close()
    cleanup()
  }
})

describe("GET / — github-app mode with no userAccessToken shows empty project list", async (assert) => {
  const { configPath, dbPath, cleanup } = setupAppModeFixture()

  // Build session cookie WITHOUT accessToken
  const cookieNoToken = await (async () => {
    const cookie = createCookie("__session", {
      httpOnly: true,
      secure: true,
      sameSite: "Lax" as const,
      maxAge: 28800,
      secrets: [TEST_SESSION_SECRET],
    })
    const session = createSession()
    session.set("userId" as never, "test-user" as never)
    // accessToken intentionally NOT set
    return cookie.serialize(JSON.stringify({ i: session.id, d: session.data }))
  })()

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    getInstallationToken: async () => "ghs_token",
  })

  try {
    const response = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookieNoToken } }),
    )
    const html = await response.text()

    assert({
      given: "github-app mode with session that has no accessToken",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "github-app mode with session that has no accessToken",
      should: "NOT display acme/repo-101 (no user token → empty repos list)",
      actual: html.includes("acme/repo-101"),
      expected: false,
    })
  } finally {
    cleanup()
  }
})

describe("GET / — user sees only repos they have access to", async (assert) => {
  const { baseUrl, close } = await startMockPermissionsServer()

  // Temp config: source: github-app
  const configPath = join(tmpdir(), `permissions-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")

  // Temp DB: seed 10 App-discovered projects (ids 101–110), all under installation 1
  const dbPath = join(tmpdir(), `permissions-db-${randomUUID()}.db`)
  const { raw } = initDb(dbPath)
  raw.prepare("INSERT INTO installations (id, account_login) VALUES (?, ?)").run(1, "acme")
  for (let i = 101; i <= 110; i++) {
    raw
      .prepare(
        "INSERT INTO projects (owner, repo, installation_id, github_repo_id) VALUES (?, ?, ?, ?)",
      )
      .run("acme", `repo-${i}`, 1, i)
  }
  raw.close()

  // Mock fetchFn: rewrites https://api.github.com → mock server URL
  const mockFetchFn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    const rewritten = url.replace("https://api.github.com", baseUrl)
    return fetch(rewritten, init)
  }

  const getInstallationToken = async (_installationId: number): Promise<string> =>
    "ghs_test_install_token"

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    fetchFn: mockFetchFn,
    getInstallationToken,
  })

  const cookie = await buildSessionCookieWithAccessToken("ghu_test_token")
  const request = new Request("http://localhost/", {
    headers: { Cookie: cookie },
  })

  let html = ""
  try {
    const response = await router.fetch(request)

    assert({
      given: "a github-app config with 10 app-discovered projects and user access to repos 101–103",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    html = await response.text()

    assert({
      given: "a github-app config with 10 app-discovered projects and user access to repos 101–103",
      should: "display acme/repo-101 in the HTML",
      actual: html.includes("acme/repo-101"),
      expected: true,
    })

    assert({
      given: "a github-app config with 10 app-discovered projects and user access to repos 101–103",
      should: "display acme/repo-102 in the HTML",
      actual: html.includes("acme/repo-102"),
      expected: true,
    })

    assert({
      given: "a github-app config with 10 app-discovered projects and user access to repos 101–103",
      should: "display acme/repo-103 in the HTML",
      actual: html.includes("acme/repo-103"),
      expected: true,
    })

    assert({
      given: "a github-app config with 10 app-discovered projects and user access to repos 101–103",
      should: "NOT display acme/repo-104 in the HTML",
      actual: html.includes("acme/repo-104"),
      expected: false,
    })

    assert({
      given: "a github-app config with 10 app-discovered projects and user access to repos 101–103",
      should: "NOT display acme/repo-105 in the HTML",
      actual: html.includes("acme/repo-105"),
      expected: false,
    })

    assert({
      given: "a github-app config with 10 app-discovered projects and user access to repos 101–103",
      should: "NOT display acme/repo-110 in the HTML",
      actual: html.includes("acme/repo-110"),
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
