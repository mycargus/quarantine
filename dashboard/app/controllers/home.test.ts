import { unlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import AdmZip from "adm-zip"
import { describe } from "riteway"
import { initDb } from "../lib/db.server.js"
import { home } from "./home.js"

type FetchFn = typeof fetch

function makeZipBuffer(jsonContent: string): Buffer {
  const zip = new AdmZip()
  zip.addFile("results.json", Buffer.from(jsonContent, "utf8"))
  return zip.toBuffer()
}

function toArrayBuffer(buf: Buffer): ArrayBuffer {
  return buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength) as ArrayBuffer
}

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

async function bodyText(response: Response): Promise<string> {
  return new Response(response.body).text()
}

describe("home() — missing config", async (assert) => {
  const original = process.env.DASHBOARD_CONFIG
  process.env.DASHBOARD_CONFIG = "/nonexistent/dashboard.yml"

  try {
    const response = await home()
    const html = await bodyText(response)

    assert({
      given: "a config path that does not exist",
      should: "return HTTP 500",
      actual: response.status,
      expected: 500,
    })

    assert({
      given: "a config path that does not exist",
      should: "include 'Configuration Error' in the body",
      actual: html.includes("Configuration Error"),
      expected: true,
    })

    assert({
      given: "a config path that does not exist",
      should: "include the missing file path in the body",
      actual: html.includes("/nonexistent/dashboard.yml"),
      expected: true,
    })
  } finally {
    if (original === undefined) {
      delete process.env.DASHBOARD_CONFIG
    } else {
      process.env.DASHBOARD_CONFIG = original
    }
  }
})

describe("home() — invalid config", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-test-bad-${Date.now()}.yml`)
  writeFileSync(configPath, "not: valid\nconfig: true", "utf8")
  const original = process.env.DASHBOARD_CONFIG
  process.env.DASHBOARD_CONFIG = configPath

  try {
    const response = await home()
    const html = await bodyText(response)

    assert({
      given: "a config file with invalid content (missing source/repos)",
      should: "return HTTP 500",
      actual: response.status,
      expected: 500,
    })

    assert({
      given: "a config file with invalid content",
      should: "include 'Configuration Error' in the body",
      actual: html.includes("Configuration Error"),
      expected: true,
    })
  } finally {
    if (original === undefined) {
      delete process.env.DASHBOARD_CONFIG
    } else {
      process.env.DASHBOARD_CONFIG = original
    }
    unlinkSync(configPath)
  }
})

describe("home() — valid config, empty repos", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-test-ok-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-test-${Date.now()}.db`)
  writeFileSync(configPath, "source: manual\nrepos: []", "utf8")

  const origConfig = process.env.DASHBOARD_CONFIG
  const origDb = process.env.DATABASE_URL
  process.env.DASHBOARD_CONFIG = configPath
  process.env.DATABASE_URL = dbPath

  try {
    const response = await home()
    const html = await bodyText(response)

    assert({
      given: "a valid config with empty repos list",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a valid config with empty repos list",
      should: "return HTML with content-type header",
      actual: response.headers.get("Content-Type"),
      expected: "text/html; charset=utf-8",
    })

    assert({
      given: "a valid config with empty repos list",
      should: "include the page title",
      actual: html.includes("Quarantine Dashboard"),
      expected: true,
    })

    assert({
      given: "a valid config with empty repos list",
      should: "include the Projects heading",
      actual: html.includes("Projects"),
      expected: true,
    })
  } finally {
    if (origConfig === undefined) {
      delete process.env.DASHBOARD_CONFIG
    } else {
      process.env.DASHBOARD_CONFIG = origConfig
    }
    if (origDb === undefined) {
      delete process.env.DATABASE_URL
    } else {
      process.env.DATABASE_URL = origDb
    }
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})

describe("home() — org overview: total quarantined count", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-overview-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-overview-db-${Date.now()}.db`)
  writeFileSync(
    configPath,
    "source: manual\nrepos:\n  - owner: acme\n    repo: payments-service\n  - owner: acme\n    repo: frontend",
    "utf8",
  )

  const origConfig = process.env.DASHBOARD_CONFIG
  const origDb = process.env.DATABASE_URL
  process.env.DASHBOARD_CONFIG = configPath
  process.env.DATABASE_URL = dbPath

  // Seed the DB before the handler runs (WAL mode supports concurrent connections).
  const { raw } = initDb(dbPath)
  const stmt = raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)")
  stmt.run("acme", "payments-service")
  stmt.run("acme", "frontend")
  const ids = raw.prepare("SELECT id, owner, repo FROM projects").all() as {
    id: number
    owner: string
    repo: string
  }[]
  const pid1 = ids.find((r) => r.repo === "payments-service")!.id
  const pid2 = ids.find((r) => r.repo === "frontend")!.id
  raw
    .prepare(
      "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
    )
    .run(
      pid1,
      "t1",
      "payment test",
      "2026-03-01T00:00:00Z",
      "https://github.com/acme/payments-service/issues/1",
    )
  raw
    .prepare(
      "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
    )
    .run(
      pid2,
      "f1",
      "nav test",
      "2026-03-02T00:00:00Z",
      "https://github.com/acme/frontend/issues/1",
    )
  raw.close()

  try {
    const response = await home()
    const html = await bodyText(response)

    assert({
      given: "2 repos with 1 quarantined test each",
      should: "include the total quarantined count wrapped in <strong>",
      actual: html.includes("<strong>2</strong>"),
      expected: true,
    })

    assert({
      given: "2 repos with quarantined tests",
      should: "include the payments-service repo name",
      actual: html.includes("payments-service"),
      expected: true,
    })

    assert({
      given: "2 repos with quarantined tests",
      should: "include the frontend repo name",
      actual: html.includes("frontend"),
      expected: true,
    })

    assert({
      given: "2 repos with quarantined tests",
      should: "include a link to each project details page",
      actual:
        html.includes("/projects/acme/payments-service") &&
        html.includes("/projects/acme/frontend"),
      expected: true,
    })

    assert({
      given: "a repo with a recently quarantined test",
      should: "include the most recently quarantined test name",
      actual: html.includes("nav test"),
      expected: true,
    })
  } finally {
    if (origConfig === undefined) {
      delete process.env.DASHBOARD_CONFIG
    } else {
      process.env.DASHBOARD_CONFIG = origConfig
    }
    if (origDb === undefined) {
      delete process.env.DATABASE_URL
    } else {
      process.env.DATABASE_URL = origDb
    }
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})

describe("home() — sync failure does not produce HTTP 500", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-sync-fail-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-sync-fail-db-${Date.now()}.db`)
  writeFileSync(configPath, "source: manual\nrepos:\n  - owner: mycargus\n    repo: my-app", "utf8")

  const origConfig = process.env.DASHBOARD_CONFIG
  const origDb = process.env.DATABASE_URL
  const origToken = process.env.QUARANTINE_GITHUB_TOKEN
  process.env.DASHBOARD_CONFIG = configPath
  process.env.DATABASE_URL = dbPath
  process.env.QUARANTINE_GITHUB_TOKEN = "fake-token"

  try {
    const response = await home({
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
    if (origConfig === undefined) delete process.env.DASHBOARD_CONFIG
    else process.env.DASHBOARD_CONFIG = origConfig
    if (origDb === undefined) delete process.env.DATABASE_URL
    else process.env.DATABASE_URL = origDb
    if (origToken === undefined) delete process.env.QUARANTINE_GITHUB_TOKEN
    else process.env.QUARANTINE_GITHUB_TOKEN = origToken
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

  const origConfig = process.env.DASHBOARD_CONFIG
  const origDb = process.env.DATABASE_URL
  const origToken = process.env.QUARANTINE_GITHUB_TOKEN
  const origGithubToken = process.env.GITHUB_TOKEN
  process.env.DASHBOARD_CONFIG = configPath
  process.env.DATABASE_URL = dbPath
  delete process.env.QUARANTINE_GITHUB_TOKEN
  delete process.env.GITHUB_TOKEN

  try {
    const response = await home()

    assert({
      given: "no QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN set",
      should: "return HTTP 200 without attempting sync",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "no token set",
      should: "return HTML content-type",
      actual: response.headers.get("Content-Type"),
      expected: "text/html; charset=utf-8",
    })
  } finally {
    if (origConfig === undefined) delete process.env.DASHBOARD_CONFIG
    else process.env.DASHBOARD_CONFIG = origConfig
    if (origDb === undefined) delete process.env.DATABASE_URL
    else process.env.DATABASE_URL = origDb
    if (origToken === undefined) delete process.env.QUARANTINE_GITHUB_TOKEN
    else process.env.QUARANTINE_GITHUB_TOKEN = origToken
    if (origGithubToken === undefined) delete process.env.GITHUB_TOKEN
    else process.env.GITHUB_TOKEN = origGithubToken
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

  const origConfig = process.env.DASHBOARD_CONFIG
  const origDb = process.env.DATABASE_URL
  const origToken = process.env.QUARANTINE_GITHUB_TOKEN
  process.env.DASHBOARD_CONFIG = configPath
  process.env.DATABASE_URL = dbPath
  delete process.env.QUARANTINE_GITHUB_TOKEN

  try {
    const response = await home()
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
    if (origConfig === undefined) delete process.env.DASHBOARD_CONFIG
    else process.env.DASHBOARD_CONFIG = origConfig
    if (origDb === undefined) delete process.env.DATABASE_URL
    else process.env.DATABASE_URL = origDb
    if (origToken === undefined) delete process.env.QUARANTINE_GITHUB_TOKEN
    else process.env.QUARANTINE_GITHUB_TOKEN = origToken
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {}
  }
})

describe("home() — GITHUB_TOKEN fallback triggers sync when QUARANTINE_GITHUB_TOKEN absent", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-gh-token-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-gh-token-db-${Date.now()}.db`)
  writeFileSync(configPath, "source: manual\nrepos:\n  - owner: mycargus\n    repo: my-app", "utf8")

  const origConfig = process.env.DASHBOARD_CONFIG
  const origDb = process.env.DATABASE_URL
  const origToken = process.env.QUARANTINE_GITHUB_TOKEN
  const origGhToken = process.env.GITHUB_TOKEN
  process.env.DASHBOARD_CONFIG = configPath
  process.env.DATABASE_URL = dbPath
  delete process.env.QUARANTINE_GITHUB_TOKEN
  process.env.GITHUB_TOKEN = "fallback-token"

  try {
    const response = await home({
      fetchFn: makeSucceedingFetch("mycargus", "my-app", "run-fallback-token"),
    })

    // Verify that sync was triggered by checking DB state: test_runs should have the ingested row
    const { raw } = initDb(dbPath)
    const row = raw
      .prepare("SELECT run_id FROM test_runs WHERE run_id = ?")
      .get("run-fallback-token") as { run_id: string } | undefined
    raw.close()

    assert({
      given: "only GITHUB_TOKEN is set (QUARANTINE_GITHUB_TOKEN absent)",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "only GITHUB_TOKEN is set",
      should: "ingest the artifact into test_runs (sync was triggered with the fallback token)",
      actual: row?.run_id,
      expected: "run-fallback-token",
    })
  } finally {
    if (origConfig === undefined) delete process.env.DASHBOARD_CONFIG
    else process.env.DASHBOARD_CONFIG = origConfig
    if (origDb === undefined) delete process.env.DATABASE_URL
    else process.env.DATABASE_URL = origDb
    if (origToken === undefined) delete process.env.QUARANTINE_GITHUB_TOKEN
    else process.env.QUARANTINE_GITHUB_TOKEN = origToken
    if (origGhToken === undefined) delete process.env.GITHUB_TOKEN
    else process.env.GITHUB_TOKEN = origGhToken
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

  const origConfig = process.env.DASHBOARD_CONFIG
  const origDb = process.env.DATABASE_URL
  const origToken = process.env.QUARANTINE_GITHUB_TOKEN
  process.env.DASHBOARD_CONFIG = configPath
  process.env.DATABASE_URL = dbPath
  process.env.QUARANTINE_GITHUB_TOKEN = "fake-token"

  // fetchFn: mycargus/my-app list call throws; acme/payments-service succeeds
  const multiRepoFetch: FetchFn = (async (url: string | URL | Request, init?: RequestInit) => {
    const urlStr = typeof url === "string" ? url : url instanceof URL ? url.href : url.url
    if (urlStr.includes("/mycargus/my-app/actions/artifacts")) {
      throw new Error("my-app network error")
    }
    return makeSucceedingFetch("acme", "payments-service", "run-payments-service")(url, init)
  }) as unknown as FetchFn

  try {
    const response = await home({ fetchFn: multiRepoFetch })
    const html = await bodyText(response)

    // Verify payments-service data was ingested despite my-app failure
    const { raw } = initDb(dbPath)
    const row = raw
      .prepare("SELECT run_id FROM test_runs WHERE run_id = ?")
      .get("run-payments-service") as { run_id: string } | undefined
    raw.close()

    assert({
      given: "two repos where the first repo sync throws a network error",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "two repos where the first repo sync throws",
      should: "still ingest data from the second repo into test_runs",
      actual: row?.run_id,
      expected: "run-payments-service",
    })

    assert({
      given: "two repos where the first repo sync throws",
      should: "still include the second repo name in the rendered HTML",
      actual: html.includes("payments-service"),
      expected: true,
    })
  } finally {
    if (origConfig === undefined) delete process.env.DASHBOARD_CONFIG
    else process.env.DASHBOARD_CONFIG = origConfig
    if (origDb === undefined) delete process.env.DATABASE_URL
    else process.env.DATABASE_URL = origDb
    if (origToken === undefined) delete process.env.QUARANTINE_GITHUB_TOKEN
    else process.env.QUARANTINE_GITHUB_TOKEN = origToken
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {}
  }
})
