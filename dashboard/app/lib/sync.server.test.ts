import { describe } from "riteway"
import { makeZipBuffer, toArrayBuffer } from "../test-helpers.js"
import { initDb } from "./db.server.js"
import { syncRepo } from "./sync.server.js"

type FetchFn = typeof fetch

const makeArtifactJson = (runId: string) =>
  JSON.stringify({
    version: 1,
    run_id: runId,
    repo: "mycargus/my-app",
    branch: "main",
    commit_sha: "aaa1234567890def1234567890abcdef12345678",
    pr_number: null,
    timestamp: "2026-02-10T14:00:00Z",
    cli_version: "0.1.0",
    suite_name: "unit",
    config: { retry_count: 3 },
    summary: { total: 1, passed: 1, failed: 0, skipped: 0, quarantined: 0, flaky_detected: 0 },
    tests: [],
  })

function makeArtifact(id: number, name: string) {
  return {
    id,
    name,
    archive_download_url: `https://api.github.com/repos/mycargus/my-app/actions/artifacts/${id}/zip`,
    created_at: `2026-02-10T1${id}:00:00Z`,
    expires_at: "2026-03-10T14:00:00Z",
  }
}

// Creates a fake fetchFn serving a 3-artifact list and ZIP downloads.
function makeFakeFetch(artifacts: ReturnType<typeof makeArtifact>[]): FetchFn {
  return (async (url: string | URL | Request, _init?: RequestInit) => {
    const urlStr = typeof url === "string" ? url : url instanceof URL ? url.href : url.url

    if (urlStr.includes("/actions/artifacts?")) {
      return {
        ok: true,
        status: 200,
        headers: { get: (k: string) => (k === "etag" ? '"etag-v1"' : null) },
        json: async () => ({ artifacts }),
      }
    }

    // It's a ZIP download URL — find the artifact by id in the URL.
    const match = urlStr.match(/artifacts\/(\d+)\/zip/)
    if (match) {
      const artifactId = Number(match[1])
      const artifact = artifacts.find((a) => a.id === artifactId)
      const zipBuf = makeZipBuffer(makeArtifactJson(`run-${artifactId}`))
      if (artifact) {
        return {
          ok: true,
          status: 200,
          arrayBuffer: async () => toArrayBuffer(zipBuf),
          headers: { get: () => null },
        }
      }
    }

    throw new Error(`Unexpected fetch call: ${urlStr}`)
  }) as unknown as FetchFn
}

describe("syncRepo() — first call with null last_pulled_at", async (assert) => {
  const handle = initDb(":memory:")
  const owner = "mycargus"
  const repo = "my-app"
  const now = new Date("2026-02-10T15:00:00Z")
  const artifacts = [
    makeArtifact(1, "quarantine-results-1"),
    makeArtifact(2, "quarantine-results-2"),
    makeArtifact(3, "quarantine-results-3"),
  ]
  const fetchFn = makeFakeFetch(artifacts)
  const warnings: string[] = []

  await syncRepo(owner, repo, "fake-token", handle, now, fetchFn, (msg) => warnings.push(msg))

  const testRunRows = handle.raw.prepare("SELECT run_id FROM test_runs ORDER BY run_id").all() as {
    run_id: string
  }[]
  const projectRow = handle.raw
    .prepare("SELECT last_pulled_at FROM projects WHERE owner = ? AND repo = ?")
    .get(owner, repo) as { last_pulled_at: string | null }

  assert({
    given: "a first sync with null last_pulled_at and 3 matching artifacts",
    should: "ingest all 3 artifact run_ids into test_runs",
    actual: testRunRows.map((r) => r.run_id).sort(),
    expected: ["run-1", "run-2", "run-3"],
  })

  assert({
    given: "a successful first sync",
    should: "update last_pulled_at to now.toISOString()",
    actual: projectRow.last_pulled_at,
    expected: now.toISOString(),
  })

  assert({
    given: "a successful first sync",
    should: "produce no warnings",
    actual: warnings.length,
    expected: 0,
  })
})

describe("syncRepo() — second call within 5 minutes does not re-fetch", async (assert) => {
  const handle = initDb(":memory:")
  const owner = "mycargus"
  const repo = "my-app"
  const firstCallTime = new Date("2026-02-10T15:00:00Z")
  const artifacts = [makeArtifact(1, "quarantine-results-1")]

  // First call — should fetch and insert 1 row
  await syncRepo(owner, repo, "fake-token", handle, firstCallTime, makeFakeFetch(artifacts))

  const rowCountAfterFirst = (
    handle.raw.prepare("SELECT COUNT(*) as count FROM test_runs").get() as { count: number }
  ).count

  // Second call within 5 minutes — should not re-fetch or insert new rows
  const secondCallTime = new Date(firstCallTime.getTime() + 2 * 60 * 1000) // 2 minutes later
  await syncRepo(owner, repo, "fake-token", handle, secondCallTime, makeFakeFetch(artifacts))

  const rowCountAfterSecond = (
    handle.raw.prepare("SELECT COUNT(*) as count FROM test_runs").get() as { count: number }
  ).count

  assert({
    given: "a first sync that inserted 1 row",
    should: "have exactly 1 test_runs row after the first call",
    actual: rowCountAfterFirst,
    expected: 1,
  })

  assert({
    given: "a second syncRepo call within 5 minutes of the first",
    should: "not insert any new test_runs rows (sync was not re-triggered)",
    actual: rowCountAfterSecond,
    expected: 1,
  })
})

describe("syncRepo() — fetch throws (network error)", async (assert) => {
  const handle = initDb(":memory:")
  const owner = "mycargus"
  const repo = "my-app"
  const now = new Date("2026-02-10T15:00:00Z")
  const warnings: string[] = []
  const throwingFetch: FetchFn = (async () => {
    throw new Error("network unreachable")
  }) as unknown as FetchFn

  // Must not throw
  await syncRepo(owner, repo, "fake-token", handle, now, throwingFetch, (msg) => warnings.push(msg))

  assert({
    given: "a fetchFn that throws a network error",
    should: "not throw and log at least one warning",
    actual: warnings.length > 0,
    expected: true,
  })
})

describe("syncRepo() — no artifacts match quarantine-results prefix", async (assert) => {
  const handle = initDb(":memory:")
  const owner = "mycargus"
  const repo = "my-app"
  const now = new Date("2026-02-10T15:00:00Z")

  // fetchFn returns only non-matching artifacts (coverage-report)
  const nonMatchingArtifact = {
    id: 99,
    name: "coverage-report",
    archive_download_url: "https://api.github.com/repos/mycargus/my-app/actions/artifacts/99/zip",
    created_at: "2026-02-10T14:00:00Z",
    expires_at: "2026-03-10T14:00:00Z",
  }
  const fetchFn = makeFakeFetch([nonMatchingArtifact])

  await syncRepo(owner, repo, "fake-token", handle, now, fetchFn)

  const rowCount = (
    handle.raw.prepare("SELECT COUNT(*) as count FROM test_runs").get() as { count: number }
  ).count
  const projectRow = handle.raw
    .prepare("SELECT last_pulled_at FROM projects WHERE owner = ? AND repo = ?")
    .get(owner, repo) as { last_pulled_at: string | null }

  assert({
    given: "an artifact list where no artifact matches the quarantine-results prefix",
    should: "insert no test_runs rows",
    actual: rowCount,
    expected: 0,
  })

  assert({
    given: "an artifact list where no artifact matches the quarantine-results prefix",
    should: "still update last_pulled_at to now",
    actual: projectRow.last_pulled_at,
    expected: now.toISOString(),
  })
})

describe("syncRepo() — partial download failure aborts remaining artifacts", async (assert) => {
  const handle = initDb(":memory:")
  const owner = "mycargus"
  const repo = "my-app"
  const now = new Date("2026-02-10T15:00:00Z")
  const warnings: string[] = []

  const artifacts = [
    makeArtifact(1, "quarantine-results-1"),
    makeArtifact(2, "quarantine-results-2"),
    makeArtifact(3, "quarantine-results-3"),
  ]

  // fetchFn: list succeeds; artifact 2 download throws; artifact 3 would succeed
  const partialFailFetch: FetchFn = (async (url: string | URL | Request, init?: RequestInit) => {
    const urlStr = typeof url === "string" ? url : url instanceof URL ? url.href : url.url
    if (urlStr.includes("/actions/artifacts?")) {
      return {
        ok: true,
        status: 200,
        headers: { get: (k: string) => (k === "etag" ? '"etag-v1"' : null) },
        json: async () => ({ artifacts }),
      }
    }
    if (urlStr.includes("artifacts/2/zip")) {
      throw new Error("download failed for artifact 2")
    }
    return makeFakeFetch(artifacts)(url, init)
  }) as unknown as FetchFn

  await syncRepo(owner, repo, "fake-token", handle, now, partialFailFetch, (msg) =>
    warnings.push(msg),
  )

  const rowCount = (
    handle.raw.prepare("SELECT COUNT(*) as count FROM test_runs").get() as { count: number }
  ).count
  const projectRow = handle.raw
    .prepare("SELECT last_pulled_at FROM projects WHERE owner = ? AND repo = ?")
    .get(owner, repo) as { last_pulled_at: string | null }

  assert({
    given: "3 artifacts where the second download throws",
    should: "insert exactly 1 test_runs row (first artifact before the error)",
    actual: rowCount,
    expected: 1,
  })

  assert({
    given: "3 artifacts where the second download throws",
    should: "emit at least one warning",
    actual: warnings.length > 0,
    expected: true,
  })

  assert({
    given: "3 artifacts where the second download throws",
    should: "NOT update last_pulled_at (sync did not complete)",
    actual: projectRow.last_pulled_at,
    expected: null,
  })
})
