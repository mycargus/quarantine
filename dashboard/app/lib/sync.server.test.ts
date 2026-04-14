import { describe } from "riteway"
import { makeZipBuffer, toArrayBuffer } from "../test-helpers.js"
import { initDb } from "./db.server.js"
import { syncRepo } from "./sync.server.js"

// Minimal QuarantineState fixture for state enumeration tests
const makeStateJson = (suiteName: string, testCount: number): string => {
  const tests: Record<string, object> = {}
  for (let i = 0; i < testCount; i++) {
    const id = `test/file${i}.test.ts::Suite::test ${i}`
    tests[id] = {
      test_id: id,
      file_path: `test/file${i}.test.ts`,
      classname: "Suite",
      name: `test ${i}`,
      suite: suiteName,
      first_flaky_at: "2026-04-01T00:00:00Z",
      last_failure_at: "2026-04-13T00:00:00Z",
      flaky_count: 1,
      quarantined_at: "2026-04-01T00:00:00Z",
      quarantined_by: "cli-auto",
    }
  }
  return JSON.stringify({ version: 1, updated_at: "2026-04-14T10:00:00Z", tests })
}

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

// Creates a fake fetchFn serving a 3-artifact list, ZIP downloads, and a 404
// for the state branch .quarantine/ directory (no state enumeration needed).
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

    // State branch directory listing — return 404 so state enumeration is a no-op
    if (urlStr.includes("contents/.quarantine?")) {
      return {
        ok: false,
        status: 404,
        json: async () => ({ message: "Not Found" }),
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

describe("syncRepo() — state enumeration: backend (3 tests) + frontend (1 test)", async (assert) => {
  const handle = initDb(":memory:")
  const owner = "mycargus"
  const repo = "my-app"
  const now = new Date("2026-04-14T12:00:00Z")

  const directoryListing = [
    { name: "backend", path: ".quarantine/backend", type: "dir", sha: "sha-backend" },
    { name: "frontend", path: ".quarantine/frontend", type: "dir", sha: "sha-frontend" },
  ]

  const stateByUrl: Record<string, string> = {
    ".quarantine/backend/state.json": makeStateJson("backend", 3),
    ".quarantine/frontend/state.json": makeStateJson("frontend", 1),
  }

  const fetchFn: FetchFn = (async (url: string | URL | Request) => {
    const urlStr = typeof url === "string" ? url : url instanceof URL ? url.href : url.url

    // Artifact list — return empty so artifact loop does nothing
    if (urlStr.includes("/actions/artifacts?")) {
      return {
        ok: true,
        status: 200,
        headers: { get: () => null },
        json: async () => ({ artifacts: [] }),
      }
    }

    // Directory listing for .quarantine/
    if (urlStr.includes("contents/.quarantine?")) {
      return {
        ok: true,
        status: 200,
        json: async () => directoryListing,
      }
    }

    // State file reads
    for (const [pathFragment, stateJson] of Object.entries(stateByUrl)) {
      if (urlStr.includes(pathFragment)) {
        const encoded = Buffer.from(stateJson).toString("base64")
        return {
          ok: true,
          status: 200,
          json: async () => ({ content: encoded, sha: "state-sha" }),
        }
      }
    }

    throw new Error(`Unexpected fetch call in state enum test: ${urlStr}`)
  }) as unknown as FetchFn

  await syncRepo(owner, repo, "fake-token", handle, now, fetchFn)

  const rows = handle.raw
    .prepare(
      "SELECT suite_name, quarantined_count FROM quarantine_state WHERE project_id = (SELECT id FROM projects WHERE owner = ? AND repo = ?) ORDER BY suite_name",
    )
    .all(owner, repo) as { suite_name: string; quarantined_count: number }[]

  assert({
    given: "a sync with backend (3 tests) and frontend (1 test) suites on the state branch",
    should: "store 2 quarantine_state rows (one per suite)",
    actual: rows.length,
    expected: 2,
  })

  assert({
    given: "a sync with backend (3 tests) and frontend (1 test) suites on the state branch",
    should: "record 3 quarantined tests for backend suite",
    actual: rows.find((r) => r.suite_name === "backend")?.quarantined_count,
    expected: 3,
  })

  assert({
    given: "a sync with backend (3 tests) and frontend (1 test) suites on the state branch",
    should: "record 1 quarantined test for frontend suite",
    actual: rows.find((r) => r.suite_name === "frontend")?.quarantined_count,
    expected: 1,
  })
})
