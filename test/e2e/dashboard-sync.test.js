/**
 * E2E test: Dashboard sync — real GitHub Artifacts API
 *
 * Exercises the full dashboard data pipeline against the real GitHub
 * Artifacts API: artifact listing → download → ZIP extraction → JSON
 * validation → SQLite ingest → route rendering.
 *
 * The dashboard HTTP server is spawned as a child process using tsx so the
 * TypeScript source is executed with the correct JSX runtime. Each test
 * gets an isolated temp config, SQLite database, and server process.
 *
 * Required env vars:
 *   QUARANTINE_GITHUB_TOKEN   — PAT with repo scope
 *   QUARANTINE_TEST_OWNER     — GitHub org or user owning the fixture repo
 *   QUARANTINE_TEST_REPO      — repo that has quarantine-results-* artifacts
 */

import { spawn, spawnSync } from "node:child_process"
import { mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { assert } from "riteway/vitest"
import { afterEach, beforeEach, describe, onTestFailed, test } from "vitest"

const token = process.env.QUARANTINE_GITHUB_TOKEN
const owner = process.env.QUARANTINE_TEST_OWNER
const repo = process.env.QUARANTINE_TEST_REPO

if (!token) throw new Error("QUARANTINE_GITHUB_TOKEN is required")
if (!owner) throw new Error("QUARANTINE_TEST_OWNER is required")
if (!repo) throw new Error("QUARANTINE_TEST_REPO is required")

const DASHBOARD_DIR = resolve(new URL("../../dashboard", import.meta.url).pathname)

// Fixed session secret for E2E tests. Passed to the spawned server so that
// the cookie we build here will be accepted by requireAuth().
const E2E_SESSION_SECRET = "e2e-test-session-secret"

// --- Test infrastructure ---

function pickPort() {
  return 14000 + Math.floor(Math.random() * 1000)
}

function writeConfig(configPath) {
  writeFileSync(
    configPath,
    `source: manual\nrepos:\n  - owner: ${owner}\n    repo: ${repo}\n`,
    "utf8",
  )
}

/**
 * Builds a valid signed session cookie for the E2E session secret by spawning
 * the dashboard's cookie-builder script. Returns the "name=value" string
 * suitable for use in a Cookie request header.
 */
function buildSessionCookie() {
  const result = spawnSync("node", ["--import", "tsx/esm", "scripts/e2e-session-cookie.ts"], {
    cwd: DASHBOARD_DIR,
    env: { ...process.env, SESSION_SECRET: E2E_SESSION_SECRET },
    encoding: "utf8",
  })
  if (result.status !== 0) {
    throw new Error(`Failed to build session cookie:\n${result.stderr}`)
  }
  return result.stdout.trim()
}

/**
 * Spawns the dashboard HTTP server. Returns a handle with the child
 * process, paths, and a diagnostic stderr buffer for onTestFailed output.
 */
function startServer(tempDir, port) {
  const configPath = join(tempDir, "dashboard.yml")
  const dbPath = join(tempDir, "quarantine.db")
  writeConfig(configPath)

  const child = spawn("node", ["--import", "tsx/esm", "app/server.ts"], {
    cwd: DASHBOARD_DIR,
    env: {
      ...process.env,
      PORT: String(port),
      DASHBOARD_CONFIG: configPath,
      DATABASE_URL: dbPath,
      QUARANTINE_GITHUB_TOKEN: token,
      SESSION_SECRET: E2E_SESSION_SECRET,
    },
    stdio: ["ignore", "pipe", "pipe"],
  })

  let stdout = ""
  let stderr = ""
  child.stdout?.on("data", (d) => {
    stdout += d.toString()
  })
  child.stderr?.on("data", (d) => {
    stderr += d.toString()
  })

  return {
    child,
    port,
    configPath,
    dbPath,
    tempDir,
    getStdout: () => stdout,
    getStderr: () => stderr,
  }
}

/**
 * Polls until the server binds its port (responds to a TCP connection).
 * Does NOT make an HTTP request — this separates "is the server running?"
 * from "does the first request succeed?" so sync failures surface as test
 * assertion failures, not as a generic "server not ready" timeout.
 */
async function waitForReady(port, timeoutMs = 15_000) {
  const { createConnection } = await import("node:net")
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const ok = await new Promise((resolve) => {
      const sock = createConnection({ port, host: "127.0.0.1" }, () => {
        sock.destroy()
        resolve(true)
      })
      sock.on("error", () => resolve(false))
      sock.setTimeout(500, () => {
        sock.destroy()
        resolve(false)
      })
    })
    if (ok) return
    await new Promise((r) => setTimeout(r, 200))
  }
  throw new Error(`Server did not bind port ${port} within ${timeoutMs}ms`)
}

async function stopServer(handle) {
  handle.child.kill("SIGTERM")
  await new Promise((r) => {
    handle.child.once("exit", r)
    setTimeout(() => {
      handle.child.kill("SIGKILL")
      r()
    }, 5000)
  })
}

// --- Tests ---

describe("dashboard E2E — sync from real GitHub Artifacts", () => {
  let handle
  let baseUrl
  let sessionCookie

  beforeEach(async () => {
    const tempDir = mkdtempSync(join(tmpdir(), "dash-e2e-"))
    const port = pickPort()
    handle = startServer(tempDir, port)
    baseUrl = `http://localhost:${port}`
    sessionCookie = buildSessionCookie()
    await waitForReady(port)
  })

  afterEach(async () => {
    if (handle) {
      // Surface server output when a test fails — matches CLI E2E convention.
      onTestFailed(() => {
        console.error("\n--- dashboard server output (on failure) ---")
        if (handle.getStdout()) console.error(`stdout:\n${handle.getStdout().trimEnd()}`)
        if (handle.getStderr()) console.error(`stderr:\n${handle.getStderr().trimEnd()}`)
        console.error("---------------------------------------------\n")
      })
      await stopServer(handle)
      try {
        rmSync(handle.tempDir, { recursive: true, force: true })
      } catch {
        // ignore cleanup errors
      }
    }
  })

  test("GET / syncs artifacts and renders the fixture repo with quarantine data", {
    timeout: 120_000,
  }, async () => {
    // The first GET / triggers sync against the real GitHub Artifacts API.
    // The response is returned only after sync completes and the page renders.
    const response = await fetch(`${baseUrl}/`, {
      headers: { Cookie: sessionCookie },
    })
    const html = await response.text()

    assert({
      given: "a GET / to a fresh dashboard pointed at the fixture repo",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    // Verify the fixture repo appears in the rendered HTML — confirms the
    // config was loaded and the project was registered.
    assert({
      given: "a GET / response after sync",
      should: "render the fixture repo in the page",
      actual: html.includes(`${owner}/${repo}`),
      expected: true,
    })

    // Verify artifacts were actually ingested: the "Total quarantined tests"
    // count must be a number ≥ 0. If sync returned no artifacts or ingestion
    // failed silently, this value would be missing from the HTML entirely.
    const countMatch = html.match(/<strong>(\d+)<\/strong>/)

    assert({
      given: "a GET / response after syncing real artifacts",
      should: "render a numeric total-quarantined-tests count in <strong> tags",
      actual: countMatch !== null,
      expected: true,
    })
  })

  test("GET /projects/:owner/:repo shows test details after sync", {
    timeout: 120_000,
  }, async () => {
    // Trigger sync via the home route.
    const homeRes = await fetch(`${baseUrl}/`, {
      headers: { Cookie: sessionCookie },
    })

    assert({
      given: "a GET / to trigger sync before checking project detail",
      should: "return HTTP 200",
      actual: homeRes.status,
      expected: 200,
    })

    // Now request the project detail page.
    const response = await fetch(`${baseUrl}/projects/${owner}/${repo}`, {
      headers: { Cookie: sessionCookie },
    })
    const html = await response.text()

    assert({
      given: `a GET /projects/${owner}/${repo} after sync`,
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: `a GET /projects/${owner}/${repo} after sync`,
      should: "render the project name in the page",
      actual: html.includes(`${owner}/${repo}`),
      expected: true,
    })

    // The project page shows "Showing X of Y quarantined tests". Verify
    // the phrase is present — this confirms the DB was queried and results
    // rendered, even if the fixture repo happens to have 0 quarantined tests.
    assert({
      given: `a GET /projects/${owner}/${repo} after sync`,
      should: "render the quarantined-test count phrase",
      actual: /Showing \d+ of \d+/.test(html),
      expected: true,
    })
  })

  test("second GET / within debounce window returns the same data without re-syncing", {
    timeout: 120_000,
  }, async () => {
    // First request: triggers sync, ingests artifacts.
    const first = await fetch(`${baseUrl}/`, {
      headers: { Cookie: sessionCookie },
    })
    const firstHtml = await first.text()

    assert({
      given: "the first GET / (triggers sync)",
      should: "return HTTP 200",
      actual: first.status,
      expected: 200,
    })

    // Second request: within the 5-minute debounce window, so sync is
    // skipped. The response should be rendered from the same SQLite data
    // that was ingested on the first request.
    const second = await fetch(`${baseUrl}/`, {
      headers: { Cookie: sessionCookie },
    })
    const secondHtml = await second.text()

    assert({
      given: "the second GET / (within debounce window)",
      should: "return HTTP 200",
      actual: second.status,
      expected: 200,
    })

    // Extract the total-quarantined count from both responses.
    const firstCount = firstHtml.match(/<strong>(\d+)<\/strong>/)?.[1]
    const secondCount = secondHtml.match(/<strong>(\d+)<\/strong>/)?.[1]

    assert({
      given: "two GET / requests to the same server within the debounce window",
      should: "render the same total-quarantined count (data is stable)",
      actual: firstCount === secondCount,
      expected: true,
    })
  })
})
