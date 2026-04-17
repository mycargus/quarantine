/**
 * Interface test: Scenario 171 — Expired user access token on GET /user/installations returns 401.
 *
 * When source: github-app and a logged-in user's access token is expired (GitHub
 * returns 401 on /user/installations), the dashboard must return HTTP 401 to the
 * browser, prompting the user to re-authenticate via OAuth.
 *
 * A 500 from /user/installations must still degrade gracefully (HTTP 200 with empty list).
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
import { describe } from "riteway"
import { createApp } from "../app/app.js"
import { initDb } from "../app/lib/db.server.js"
import { buildSessionCookieWithAccessToken } from "./helpers.js"

const TEST_SESSION_SECRET = "test-secret"

/**
 * Starts a local mock HTTP server that always returns the given status on all routes.
 */
function startMockServerWithStatus(status: number): Promise<{
  url: string
  close: () => Promise<void>
}> {
  return new Promise((resolve) => {
    const server = createServer((_req, res) => {
      res.writeHead(status, { "Content-Type": "application/json" })
      res.end(JSON.stringify({ message: "mock response" }))
    })
    server.listen(0, "127.0.0.1", () => {
      const addr = server.address() as AddressInfo
      resolve({
        url: `http://127.0.0.1:${addr.port}`,
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
function setupAppModeFixture() {
  const configPath = join(tmpdir(), `auth-error-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")
  const dbPath = join(tmpdir(), `auth-error-db-${randomUUID()}.db`)
  const { raw } = initDb(dbPath)
  raw.prepare("INSERT INTO installations (id, account_login) VALUES (?, ?)").run(1, "acme")
  raw
    .prepare(
      "INSERT INTO projects (owner, repo, installation_id, github_repo_id) VALUES (?, ?, ?, ?)",
    )
    .run("acme", "repo-101", 1, 101)
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

describe("GET / — expired access token (GitHub returns 401 on /user/installations) returns HTTP 401", async (assert) => {
  const { configPath, dbPath, cleanup } = setupAppModeFixture()
  const mockServer = await startMockServerWithStatus(401)

  const mockFetchFn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    return fetch(url.replace("https://api.github.com", mockServer.url), init)
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
      given:
        "a logged-in user with an expired access token and GitHub returns 401 on /user/installations",
      should: "return HTTP 401 to the browser (treat as invalid session)",
      actual: response.status,
      expected: 401,
    })
  } finally {
    await mockServer.close()
    cleanup()
  }
})

describe("GET / — GitHub returns 500 on /user/installations degrades gracefully (HTTP 200)", async (assert) => {
  const { configPath, dbPath, cleanup } = setupAppModeFixture()
  const mockServer = await startMockServerWithStatus(500)

  const mockFetchFn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    return fetch(url.replace("https://api.github.com", mockServer.url), init)
  }

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    fetchFn: mockFetchFn,
    getInstallationToken: async () => "ghs_token",
  })

  const cookie = await buildSessionCookieWithAccessToken("ghu_valid_token")
  try {
    const response = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )

    assert({
      given: "a logged-in user and GitHub returns 500 on /user/installations",
      should: "return HTTP 200 (graceful degradation — non-auth errors produce empty list)",
      actual: response.status,
      expected: 200,
    })
  } finally {
    await mockServer.close()
    cleanup()
  }
})
