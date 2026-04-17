/**
 * Interface test: Scenario 33 — Artifact polling uses installation tokens.
 *
 * When `source: github-app` and a project row with installation_id exists,
 * GET / must use an installation token (not a PAT) on the GitHub Artifacts API.
 *
 * Test layer: Interface — exercises the full GET / route through router.fetch(),
 * intercepts the outbound HTTP artifacts request via a local mock server to
 * observe the Authorization header.
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

const TEST_SESSION_SECRET = "test-secret"

async function buildSessionCookie(): Promise<string> {
  const cookie = createCookie("__session", {
    httpOnly: true,
    secure: true,
    sameSite: "Lax" as const,
    maxAge: 28800,
    secrets: [TEST_SESSION_SECRET],
  })
  const session = createSession()
  session.set("userId" as never, "test-user" as never)
  const serializedData = JSON.stringify({ i: session.id, d: session.data })
  return cookie.serialize(serializedData)
}

/**
 * Starts a local mock HTTP server. Returns: the server, its base URL, and
 * a getter for the last Authorization header received on the artifacts endpoint.
 */
function startMockGitHubServer(): Promise<{
  server: ReturnType<typeof createServer>
  baseUrl: string
  getLastAuthorizationHeader: () => string | null
  close: () => Promise<void>
}> {
  return new Promise((resolve) => {
    let lastAuthorizationHeader: string | null = null

    const server = createServer((req, res) => {
      // Capture Authorization header for any request to the artifacts endpoint
      if (req.url?.includes("/actions/artifacts")) {
        lastAuthorizationHeader = req.headers.authorization ?? null
        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(JSON.stringify({ total_count: 0, artifacts: [] }))
        return
      }
      // All other endpoints return 404
      res.writeHead(404)
      res.end()
    })

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address() as AddressInfo
      const baseUrl = `http://127.0.0.1:${addr.port}`
      resolve({
        server,
        baseUrl,
        getLastAuthorizationHeader: () => lastAuthorizationHeader,
        close: () =>
          new Promise((res, rej) => {
            server.close((err) => (err ? rej(err) : res()))
          }),
      })
    })
  })
}

describe("GET / — github-app source without getInstallationToken still returns 200", async (assert) => {
  const configPath = join(tmpdir(), `app-mode-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")
  const dbPath = join(tmpdir(), `app-mode-db-${randomUUID()}.db`)
  const { raw } = initDb(dbPath)
  raw.prepare("INSERT INTO installations (id, account_login) VALUES (?, ?)").run(1, "acme")
  raw
    .prepare("INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)")
    .run("acme", "payments", 1)
  raw.close()

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    // getInstallationToken intentionally omitted
  })

  const cookie = await buildSessionCookie()
  try {
    const response = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )

    assert({
      given: "github-app mode configured but getInstallationToken not provided",
      should: "return HTTP 200 (graceful degradation — sync skipped, not a crash)",
      actual: response.status,
      expected: 200,
    })
  } finally {
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

describe("GET / — github-app source with getInstallationToken that throws still returns 200", async (assert) => {
  const configPath = join(tmpdir(), `app-mode-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")
  const dbPath = join(tmpdir(), `app-mode-db-${randomUUID()}.db`)
  const { raw } = initDb(dbPath)
  raw.prepare("INSERT INTO installations (id, account_login) VALUES (?, ?)").run(99, "acme")
  raw
    .prepare("INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)")
    .run("acme", "api", 99)
  raw.close()

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    getInstallationToken: async () => {
      throw new Error("token exchange failed: 500")
    },
  })

  const cookie = await buildSessionCookie()
  try {
    const response = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )

    assert({
      given: "github-app mode and getInstallationToken throws for a project",
      should: "return HTTP 200 (graceful degradation — sync skipped per FR-1.14.5, not 500)",
      actual: response.status,
      expected: 200,
    })
  } finally {
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

describe("GET / — github-app source uses installation token for artifact polling", async (assert) => {
  const { baseUrl, getLastAuthorizationHeader, close } = await startMockGitHubServer()

  // Temp config: source: github-app
  const configPath = join(tmpdir(), `app-mode-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")

  // Temp DB path
  const dbPath = join(tmpdir(), `app-mode-db-${randomUUID()}.db`)

  // Seed DB: one installation row (id=42) and one project referencing it
  const { raw } = initDb(dbPath)
  raw.prepare("INSERT INTO installations (id, account_login) VALUES (?, ?)").run(42, "acme")
  raw
    .prepare("INSERT INTO projects (owner, repo, installation_id) VALUES (?, ?, ?)")
    .run("acme", "payments", 42)
  raw.close()

  // Mock fetchFn that rewrites github.com → mock server
  const mockFetchFn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    const rewritten = url.replace("https://api.github.com", baseUrl)
    return fetch(rewritten, init)
  }

  // Mock getInstallationToken: returns known token for installation 42
  const getInstallationToken = async (installationId: number): Promise<string> => {
    if (installationId === 42) return "ghs_install_token_42"
    throw new Error(`Unexpected installation ID: ${installationId}`)
  }

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    fetchFn: mockFetchFn,
    getInstallationToken,
  })

  const cookie = await buildSessionCookie()
  const request = new Request("http://localhost/", {
    headers: { Cookie: cookie },
  })

  try {
    const response = await router.fetch(request)

    assert({
      given: "a github-app config with a project having installation_id 42",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a github-app config with a project having installation_id 42",
      should: "use the installation token in the Authorization header for artifacts API calls",
      actual: getLastAuthorizationHeader(),
      expected: "Bearer ghs_install_token_42",
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
