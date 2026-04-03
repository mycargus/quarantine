import { unlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway"
import { bodyText, makeZipBuffer, toArrayBuffer } from "../test-helpers.js"
import { home } from "./home.js"

type FetchFn = typeof fetch

function makeArtifactJson(runId: string, owner: string, repo: string): string {
  return JSON.stringify({
    version: 1,
    run_id: runId,
    repo: `${owner}/${repo}`,
    branch: "main",
    commit_sha: "aaa1234567890def1234567890abcdef12345678",
    pr_number: null,
    timestamp: "2026-02-10T14:00:00Z",
    cli_version: "0.1.0",
    framework: "jest",
    config: { retry_count: 3 },
    summary: { total: 1, passed: 1, failed: 0, skipped: 0, quarantined: 0, flaky_detected: 0 },
    tests: [],
  })
}

function makeSucceedingFetch(owner: string, repo: string, runId: string): FetchFn {
  return (async (url: string | URL | Request, _init?: RequestInit) => {
    const urlStr = typeof url === "string" ? url : url instanceof URL ? url.href : url.url
    if (urlStr.includes("/actions/artifacts?")) {
      return {
        ok: true,
        status: 200,
        headers: { get: (k: string) => (k === "etag" ? '"etag-v1"' : null) },
        json: async () => ({
          artifacts: [
            {
              id: 1,
              name: "quarantine-results-1",
              archive_download_url: `https://api.github.com/repos/${owner}/${repo}/actions/artifacts/1/zip`,
              created_at: "2026-02-10T14:00:00Z",
              expires_at: "2026-03-10T14:00:00Z",
            },
          ],
        }),
      }
    }
    if (urlStr.includes("/artifacts/1/zip")) {
      const zipBuf = makeZipBuffer(makeArtifactJson(runId, owner, repo))
      return {
        ok: true,
        status: 200,
        arrayBuffer: async () => toArrayBuffer(zipBuf),
        headers: { get: () => null },
      }
    }
    throw new Error(`Unexpected fetch call: ${urlStr}`)
  }) as unknown as FetchFn
}

describe("home() — sync failure does not produce HTTP 500", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-sync-fail-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-sync-fail-db-${Date.now()}.db`)
  writeFileSync(configPath, "source: manual\nrepos:\n  - owner: mycargus\n    repo: my-app", "utf8")

  try {
    const response = await home({
      configPath,
      dbPath,
      token: "fake-token",
      fetchFn: (async () => {
        throw new Error("network unreachable")
      }) as unknown as FetchFn,
    })

    assert({
      given: "a sync that fails with a network error",
      should: "return HTTP 200 (not 500)",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a sync that fails with a network error",
      should: "return HTML content-type",
      actual: response.headers.get("Content-Type"),
      expected: "text/html; charset=utf-8",
    })
  } finally {
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {
      // may not exist
    }
  }
})

describe("home() — no token skips sync and renders HTTP 200", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-no-token-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-no-token-db-${Date.now()}.db`)
  writeFileSync(configPath, "source: manual\nrepos:\n  - owner: mycargus\n    repo: my-app", "utf8")

  try {
    const response = await home({ configPath, dbPath })

    assert({
      given: "no token provided",
      should: "return HTTP 200 without attempting sync",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "no token provided",
      should: "return HTML content-type",
      actual: response.headers.get("Content-Type"),
      expected: "text/html; charset=utf-8",
    })
  } finally {
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {
      // may not exist
    }
  }
})

describe("home() — corrupt DB file produces HTTP 500 with error body", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-corrupt-config-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-corrupt-db-${Date.now()}.db`)
  writeFileSync(configPath, "source: manual\nrepos:\n  - owner: mycargus\n    repo: my-app", "utf8")
  // Write non-SQLite bytes so better-sqlite3 throws on open
  writeFileSync(dbPath, "not a sqlite database", "utf8")

  try {
    const response = await home({ configPath, dbPath })
    const html = await bodyText(response)

    assert({
      given: "a database file that is not a valid SQLite file",
      should: "return HTTP 500 (not crash with unhandled rejection)",
      actual: response.status,
      expected: 500,
    })

    assert({
      given: "a database file that is not a valid SQLite file",
      should: "include 'Internal error' or 'Configuration Error' in the body",
      actual: html.includes("Internal error") || html.includes("Configuration Error"),
      expected: true,
    })
  } finally {
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {}
  }
})

describe("home() — token provided triggers sync", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-gh-token-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-gh-token-db-${Date.now()}.db`)
  writeFileSync(configPath, "source: manual\nrepos:\n  - owner: mycargus\n    repo: my-app", "utf8")

  try {
    const response = await home({
      configPath,
      dbPath,
      token: "fallback-token",
      fetchFn: makeSucceedingFetch("mycargus", "my-app", "run-fallback-token"),
    })

    assert({
      given: "a token is provided",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a token is provided",
      should: "return HTML content-type",
      actual: response.headers.get("Content-Type"),
      expected: "text/html; charset=utf-8",
    })
  } finally {
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {}
  }
})

describe("home() — multi-repo: first repo sync failure does not abort second repo", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-multi-repo-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-multi-repo-db-${Date.now()}.db`)
  writeFileSync(
    configPath,
    "source: manual\nrepos:\n  - owner: mycargus\n    repo: my-app\n  - owner: acme\n    repo: payments-service",
    "utf8",
  )

  // fetchFn: mycargus/my-app list call throws; acme/payments-service succeeds
  const multiRepoFetch: FetchFn = (async (url: string | URL | Request, init?: RequestInit) => {
    const urlStr = typeof url === "string" ? url : url instanceof URL ? url.href : url.url
    if (urlStr.includes("/mycargus/my-app/actions/artifacts")) {
      throw new Error("my-app network error")
    }
    return makeSucceedingFetch("acme", "payments-service", "run-payments-service")(url, init)
  }) as unknown as FetchFn

  try {
    const response = await home({
      configPath,
      dbPath,
      token: "fake-token",
      fetchFn: multiRepoFetch,
    })
    const html = await bodyText(response)

    assert({
      given: "two repos where the first repo sync throws a network error",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "two repos where the first repo sync throws",
      should: "still include the second repo name in the rendered HTML",
      actual: html.includes("payments-service"),
      expected: true,
    })
  } finally {
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {}
  }
})
