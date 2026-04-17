/**
 * Interface test: Scenario 172 — User permission results are cached for 5 minutes per user.
 *
 * When source: github-app and a logged-in user loads the dashboard, their accessible
 * repo IDs are fetched via /user/installations. On a second request within 5 minutes,
 * the cache is hit and no additional /user/installations call is made.
 *
 * Test layer: Interface — exercises the full GET / route through router.fetch(),
 * uses a local mock GitHub server that counts requests per access token.
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
import { clearUserRepoIdsCache } from "../app/lib/user-permissions-cache.server.js"
import { buildSessionCookieWithAccessToken } from "./helpers.js"

const TEST_SESSION_SECRET = "test-secret"
const USER_TOKEN = "ghu_user42_token"

/**
 * Starts a mock server that counts /user/installations requests per token.
 * Returns installation id=1 with repo id=101 accessible.
 */
function startCountingMockServer(): Promise<{
  baseUrl: string
  getCallCount: (token: string) => number
  close: () => Promise<void>
}> {
  const callCounts = new Map<string, number>()

  return new Promise((resolve) => {
    const server = createServer((req, res) => {
      const authHeader = req.headers.authorization ?? ""
      const token = authHeader.replace(/^Bearer /, "")

      if (req.url?.startsWith("/user/installations") && !req.url.includes("/repositories")) {
        // Track call count per token
        callCounts.set(token, (callCounts.get(token) ?? 0) + 1)

        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(JSON.stringify([{ id: 1, account: { login: "acme" } }]))
        return
      }

      if (req.url?.startsWith("/user/installations/1/repositories")) {
        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(
          JSON.stringify({
            total_count: 1,
            repositories: [{ id: 101, name: "repo-101", owner: { login: "acme" } }],
          }),
        )
        return
      }

      // Artifacts and other endpoints return empty
      res.writeHead(200, { "Content-Type": "application/json" })
      res.end(JSON.stringify({ total_count: 0, artifacts: [] }))
    })

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address() as AddressInfo
      const baseUrl = `http://127.0.0.1:${addr.port}`
      resolve({
        baseUrl,
        getCallCount: (token: string) => callCounts.get(token) ?? 0,
        close: () =>
          new Promise((res, rej) => {
            server.close((err) => (err ? rej(err) : res()))
          }),
      })
    })
  })
}

/**
 * Writes a github-app config and seeds DB with 1 installation + 1 project.
 */
function setupAppModeFixture() {
  const configPath = join(tmpdir(), `cache-test-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")

  const dbPath = join(tmpdir(), `cache-test-db-${randomUUID()}.db`)
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

describe("GET / — user permissions cache: second request within 5 minutes does not re-call /user/installations", async (assert) => {
  // Clear cache before test to avoid interference from other tests
  clearUserRepoIdsCache()

  const { baseUrl, getCallCount, close } = await startCountingMockServer()
  const { configPath, dbPath, cleanup } = setupAppModeFixture()

  // Injectable clock starting at T=0
  let nowMs = 0
  const clock = () => nowMs

  const mockFetchFn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    const rewritten = url.replace("https://api.github.com", baseUrl)
    return fetch(rewritten, init)
  }

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    fetchFn: mockFetchFn,
    getInstallationToken: async () => "ghs_install_token",
    now: clock,
  })

  const cookie = await buildSessionCookieWithAccessToken(USER_TOKEN)

  try {
    // First request at T=0
    nowMs = 0
    const response1 = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )
    await response1.text() // drain the body

    assert({
      given: "a first GET / request at T=0 (cache miss)",
      should: "return HTTP 200",
      actual: response1.status,
      expected: 200,
    })

    assert({
      given: "a first GET / request at T=0 (cache miss)",
      should: "call /user/installations exactly once for user token",
      actual: getCallCount(USER_TOKEN),
      expected: 1,
    })

    // Second request at T=1 minute (well within 5-minute TTL)
    nowMs = 60 * 1000 // 1 minute later
    const response2 = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )
    await response2.text() // drain the body

    assert({
      given: "a second GET / request at T=1min (cache hit, within 5min TTL)",
      should: "return HTTP 200",
      actual: response2.status,
      expected: 200,
    })

    assert({
      given: "a second GET / request at T=1min (cache hit, within 5min TTL)",
      should: "NOT call /user/installations again (call count stays at 1)",
      actual: getCallCount(USER_TOKEN),
      expected: 1,
    })

    // Third request at T=6 minutes (past 5-minute TTL — cache expired)
    nowMs = 6 * 60 * 1000 // 6 minutes later
    const response3 = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )
    await response3.text() // drain the body

    assert({
      given: "a third GET / request at T=6min (past 5min TTL — cache expired)",
      should: "return HTTP 200",
      actual: response3.status,
      expected: 200,
    })

    assert({
      given: "a third GET / request at T=6min (past 5min TTL — cache expired)",
      should: "call /user/installations again (call count increments to 2)",
      actual: getCallCount(USER_TOKEN),
      expected: 2,
    })
  } finally {
    clearUserRepoIdsCache()
    await close()
    cleanup()
  }
})
