/**
 * Interface test: Scenario 36 — Manual-mode repos continue using PATs.
 *
 * When `source: github-app`, the dashboard must poll BOTH:
 *   - App-discovered projects (installation_id IS NOT NULL) using installation tokens
 *   - Manually-configured projects (installation_id IS NULL) using the PAT from
 *     QUARANTINE_GITHUB_TOKEN / token option
 *
 * Test layer: Interface — exercises GET / through router.fetch(), uses a local
 * mock server that records the Authorization header per repo path segment, and
 * observes which token was used for each repo's artifacts API call.
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
import { createTestApp } from "./helpers.js"

const TEST_SESSION_SECRET = "test-secret"

/**
 * Starts a mock GitHub API server that records the Authorization header per
 * repo (extracted from the URL path segment after /repos/owner/).
 * Returns a getter that takes a "owner/repo" string and returns the last
 * Authorization header seen for that repo's artifacts request.
 */
function startMockGitHubServer(): Promise<{
  server: ReturnType<typeof createServer>
  baseUrl: string
  getAuthHeaderForRepo: (ownerRepo: string) => string | null
  close: () => Promise<void>
}> {
  return new Promise((resolve) => {
    const authHeadersByRepo = new Map<string, string>()

    const server = createServer((req, res) => {
      // Match: /repos/:owner/:repo/actions/artifacts
      const match = req.url?.match(/^\/repos\/([^/]+)\/([^/]+)\/actions\/artifacts/)
      if (match) {
        const key = `${match[1]}/${match[2]}`
        authHeadersByRepo.set(key, req.headers.authorization ?? "")
        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(JSON.stringify({ total_count: 0, artifacts: [] }))
        return
      }
      res.writeHead(404)
      res.end()
    })

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address() as AddressInfo
      const baseUrl = `http://127.0.0.1:${addr.port}`
      resolve({
        server,
        baseUrl,
        getAuthHeaderForRepo: (ownerRepo: string) => authHeadersByRepo.get(ownerRepo) ?? null,
        close: () =>
          new Promise((res, rej) => {
            server.close((err) => (err ? rej(err) : res()))
          }),
      })
    })
  })
}

describe("GET / — github-app mode polls app-discovered repos with installation tokens and manual repos with PAT", async (assert) => {
  const { baseUrl, getAuthHeaderForRepo, close } = await startMockGitHubServer()

  // Config: source: github-app (no repos array — it's ignored in app mode)
  const configPath = join(tmpdir(), `mixed-auth-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")

  const dbPath = join(tmpdir(), `mixed-auth-db-${randomUUID()}.db`)

  // Seed DB:
  //   - acme/app-repo: installation_id = 10, github_repo_id = 501 (App-discovered)
  //   - acme/manual-repo: installation_id = NULL (manually configured before App)
  const { raw } = initDb(dbPath)
  raw.prepare("INSERT INTO installations (id, account_login) VALUES (?, ?)").run(10, "acme")
  raw
    .prepare(
      "INSERT INTO projects (owner, repo, installation_id, github_repo_id) VALUES (?, ?, ?, ?)",
    )
    .run("acme", "app-repo", 10, 501)
  raw
    .prepare(
      "INSERT INTO projects (owner, repo, installation_id, github_repo_id) VALUES (?, ?, ?, ?)",
    )
    .run("acme", "manual-repo", null, null)
  raw.close()

  const mockFetchFn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    const rewritten = url.replace("https://api.github.com", baseUrl)
    return fetch(rewritten, init)
  }

  const getInstallationToken = async (installationId: number): Promise<string> => {
    if (installationId === 10) return "ghs_install_token"
    throw new Error(`Unexpected installation ID: ${installationId}`)
  }

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    fetchFn: mockFetchFn,
    getInstallationToken,
    token: "ghp_pat_token",
  })

  const { sessionCookie } = createTestApp()
  const cookie = await sessionCookie()

  try {
    const response = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )

    assert({
      given:
        "github-app mode with one app-discovered repo (installation_id=10) and one manual repo (installation_id=NULL)",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "acme/app-repo has installation_id = 10 which resolves to ghs_install_token",
      should: "use the installation token in the Authorization header for acme/app-repo",
      actual: getAuthHeaderForRepo("acme/app-repo"),
      expected: "Bearer ghs_install_token",
    })

    assert({
      given: "acme/manual-repo has installation_id = NULL and a PAT is configured",
      should: "use the PAT in the Authorization header for acme/manual-repo",
      actual: getAuthHeaderForRepo("acme/manual-repo"),
      expected: "Bearer ghp_pat_token",
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

describe("GET / — github-app mode with no PAT does not poll manual repos", async (assert) => {
  const { baseUrl, getAuthHeaderForRepo, close } = await startMockGitHubServer()

  const configPath = join(tmpdir(), `mixed-auth-config-${randomUUID()}.yml`)
  writeFileSync(configPath, "source: github-app\npoll_interval: 300", "utf8")
  const dbPath = join(tmpdir(), `mixed-auth-db-${randomUUID()}.db`)
  const { raw } = initDb(dbPath)
  raw
    .prepare(
      "INSERT INTO projects (owner, repo, installation_id, github_repo_id) VALUES (?, ?, ?, ?)",
    )
    .run("acme", "manual-repo", null, null)
  raw.close()

  const mockFetchFn: typeof fetch = (input, init) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
    return fetch(url.replace("https://api.github.com", baseUrl), init)
  }

  const router = createApp({
    configPath,
    dbPath,
    sessionSecret: TEST_SESSION_SECRET,
    fetchFn: mockFetchFn,
    getInstallationToken: async () => "ghs_token",
    // token intentionally omitted — no PAT configured
  })

  const { sessionCookie } = createTestApp()
  const cookie = await sessionCookie()

  try {
    const response = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )

    assert({
      given: "github-app mode with no PAT configured and a manual repo in the DB",
      should: "return HTTP 200 (skip is silent, not an error)",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "github-app mode with no PAT configured and a manual repo in the DB",
      should: "NOT poll the manual repo (no Authorization header recorded for acme/manual-repo)",
      actual: getAuthHeaderForRepo("acme/manual-repo"),
      expected: null,
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
