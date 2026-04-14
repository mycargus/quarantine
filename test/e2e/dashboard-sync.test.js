/**
 * E2E test: Dashboard sync against real GitHub Artifacts
 *
 * Exercises the full data pipeline: GitHub Artifacts API → artifact download →
 * ZIP extraction → SQLite ingest → route rendering.
 *
 * The dashboard HTTP server is spawned as a child process using tsx so the
 * TypeScript source is executed with the correct JSX runtime (remix/component).
 * Each test gets an isolated temp config and SQLite database.
 *
 * Required env vars:
 *   QUARANTINE_GITHUB_TOKEN   — PAT with repo scope
 *   QUARANTINE_TEST_OWNER     — GitHub org or user owning the fixture repo
 *   QUARANTINE_TEST_REPO      — repo that has quarantine-results-* artifacts
 *
 * The fixture repo must have at least one quarantine-results-* artifact. The
 * existing quarantine-test-fixture used by CLI E2E tests qualifies if it has
 * been run; the `mycargus/quarantine-app-test-fixture` repo is the canonical
 * fixture for dashboard E2E.
 */

import { mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { assert } from "riteway/vitest"
import { afterEach, beforeEach, describe, test } from "vitest"

const token = process.env.QUARANTINE_GITHUB_TOKEN
const owner = process.env.QUARANTINE_TEST_OWNER
const repo = process.env.QUARANTINE_TEST_REPO

if (!token || !owner || !repo) {
  throw new Error(
    "Dashboard E2E tests require QUARANTINE_GITHUB_TOKEN, QUARANTINE_TEST_OWNER, and QUARANTINE_TEST_REPO. See test/e2e/.env.example.",
  )
}

const DASHBOARD_DIR = resolve(new URL("../../dashboard", import.meta.url).pathname)

// --- Test infrastructure ---

/**
 * Picks an ephemeral port in the range 14000–15000. Using a fixed-but-unique
 * port avoids collisions between parallel test processes without requiring a
 * port-allocation service.
 */
function pickPort() {
  return 14000 + Math.floor(Math.random() * 1000)
}

/**
 * Writes a minimal dashboard.yml config pointing at the fixture repo.
 */
function writeConfig(configPath, repoOwner, repoName) {
  writeFileSync(
    configPath,
    `source: manual\nrepos:\n  - owner: ${repoOwner}\n    repo: ${repoName}\n`,
    "utf8",
  )
}

/**
 * Starts the dashboard HTTP server as a child process and returns the
 * {process, port, tempDir} handle. The server runs with `node --import tsx/esm
 * app/server.ts` from the dashboard directory so the TypeScript source and
 * custom JSX runtime are handled by tsx, matching production behaviour.
 */
async function startServer(tempDir, port) {
  const { spawn } = await import("node:child_process")

  const configPath = join(tempDir, "dashboard.yml")
  const dbPath = join(tempDir, "quarantine.db")
  writeConfig(configPath, owner, repo)

  const server = spawn("node", ["--import", "tsx/esm", "app/server.ts"], {
    cwd: DASHBOARD_DIR,
    env: {
      ...process.env,
      PORT: String(port),
      DASHBOARD_CONFIG: configPath,
      DATABASE_URL: dbPath,
      QUARANTINE_GITHUB_TOKEN: token,
    },
    stdio: ["ignore", "pipe", "pipe"],
  })

  // Capture stderr for debugging if a test fails.
  let stderr = ""
  server.stderr?.on("data", (d) => {
    stderr += d.toString()
  })

  return { server, port, configPath, dbPath, tempDir, getStderr: () => stderr }
}

/**
 * Polls the given URL until it returns a non-5xx response or the timeout elapses.
 * Returns the final response, or throws if the server never became ready.
 */
async function waitForServer(baseUrl, timeoutMs = 10_000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`${baseUrl}/`, { signal: AbortSignal.timeout(1000) })
      if (res.status < 500) return res
    } catch {
      // server not ready yet; wait a bit
    }
    await new Promise((r) => setTimeout(r, 200))
  }
  throw new Error(`Server at ${baseUrl} did not become ready within ${timeoutMs}ms`)
}

/**
 * Kills the server process and waits for it to exit.
 */
async function stopServer(handle) {
  handle.server.kill("SIGTERM")
  await new Promise((r) => {
    handle.server.once("exit", r)
    // Force-kill after 5s if SIGTERM is ignored.
    setTimeout(() => {
      handle.server.kill("SIGKILL")
      r()
    }, 5000)
  })
}

// --- Tests ---

describe("dashboard E2E — sync happy path", () => {
  let handle
  let baseUrl

  beforeEach(async () => {
    const tempDir = mkdtempSync(join(tmpdir(), "dash-e2e-"))
    const port = pickPort()
    handle = await startServer(tempDir, port)
    baseUrl = `http://localhost:${port}`
  })

  afterEach(async () => {
    if (handle) await stopServer(handle)
    try {
      rmSync(handle.tempDir, { recursive: true, force: true })
    } catch {
      // ignore cleanup errors
    }
  })

  test("GET / triggers sync and returns the fixture project in the response", {
    timeout: 120_000,
  }, async () => {
    // GET / triggers sync on first request; await the full response.
    const response = await waitForServer(baseUrl)
    const html = await response.text()

    assert({
      given: "a GET / request to a fresh dashboard pointed at the fixture repo",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a GET / response after sync",
      should: "include 'Quarantine Dashboard' in the HTML",
      actual: html.includes("Quarantine Dashboard"),
      expected: true,
    })

    assert({
      given: "a GET / response after syncing the fixture repo",
      should: "include the fixture repo owner in the rendered HTML",
      actual: html.includes(owner),
      expected: true,
    })

    assert({
      given: "a GET / response after syncing the fixture repo",
      should: "include the fixture repo name in the rendered HTML",
      actual: html.includes(repo),
      expected: true,
    })
  })

  test("GET /projects/:owner/:repo returns the project detail page after sync", {
    timeout: 120_000,
  }, async () => {
    // Trigger sync via home route first.
    await waitForServer(baseUrl)

    const detailUrl = `${baseUrl}/projects/${owner}/${repo}`
    const response = await fetch(detailUrl)
    const html = await response.text()

    assert({
      given: `a GET /projects/${owner}/${repo} after the home route has synced`,
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: `a GET /projects/${owner}/${repo}`,
      should: "include the repo name in the page title area",
      actual: html.includes(`${owner}/${repo}`),
      expected: true,
    })
  })
})

describe("dashboard E2E — idempotent re-sync produces stable results", () => {
  test("two independent syncs of the same fixture repo produce identical rendered HTML", {
    timeout: 240_000,
  }, async () => {
    // Run two isolated server instances with separate SQLite databases but
    // the same GitHub fixture config. Both sync fresh on first request.
    // The rendered output should be identical (same artifacts ingested).

    async function syncAndCapture(suffix) {
      const tempDir = mkdtempSync(join(tmpdir(), `dash-e2e-idem-${suffix}-`))
      const port = pickPort()
      const handle = await startServer(tempDir, port)
      const baseUrl = `http://localhost:${port}`
      try {
        const response = await waitForServer(baseUrl)
        const html = await response.text()
        return { html, status: response.status }
      } finally {
        await stopServer(handle)
        try {
          rmSync(tempDir, { recursive: true, force: true })
        } catch {
          // ignore
        }
      }
    }

    const first = await syncAndCapture("a")
    const second = await syncAndCapture("b")

    assert({
      given: "the first sync of the fixture repo",
      should: "return HTTP 200",
      actual: first.status,
      expected: 200,
    })

    assert({
      given: "the second sync of the fixture repo",
      should: "return HTTP 200",
      actual: second.status,
      expected: 200,
    })

    assert({
      given: "two independent syncs of the same fixture repo",
      should: "produce HTML with the same fixture repo reference",
      actual: first.html.includes(owner) && second.html.includes(owner),
      expected: true,
    })

    // The quarantined counts rendered in the overview section should be
    // identical across both syncs, confirming ingestion is idempotent.
    const quarantineCountPattern = /Total quarantined tests.*?(\d+)/s
    const firstMatch = first.html.match(quarantineCountPattern)
    const secondMatch = second.html.match(quarantineCountPattern)

    assert({
      given: "two independent syncs of the same fixture repo",
      should: "render the same total quarantined test count",
      actual: firstMatch?.[1] === secondMatch?.[1],
      expected: true,
    })
  })
})
