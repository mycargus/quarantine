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

// Creates a fake fetchFn serving an artifact list, ZIP downloads, and a 404
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

describe("syncRepo() — state enumeration 404 handling", async (assert) => {
  const handle = initDb(":memory:")
  const owner = "mycargus"
  const repo = "my-app"
  const now = new Date("2026-04-14T12:00:00Z")
  const warnings: string[] = []

  // fetchFn: artifacts list returns one artifact so artifact sync runs normally;
  // the .quarantine/ directory listing returns 404 (no state branch directory).
  const artifacts = [makeArtifact(10, "quarantine-results-10")]
  const fetchFn = makeFakeFetch(artifacts)

  await syncRepo(owner, repo, "fake-token", handle, now, fetchFn, (msg) => warnings.push(msg))

  const projectRow = handle.raw
    .prepare("SELECT last_pulled_at FROM projects WHERE owner = ? AND repo = ?")
    .get(owner, repo) as { last_pulled_at: string | null } | undefined

  assert({
    given: "syncRepo with one artifact and .quarantine/ 404",
    should: "create a project row for the repo",
    actual: projectRow !== undefined,
    expected: true,
  })

  assert({
    given: "syncRepo with one artifact and .quarantine/ 404",
    should: "update last_pulled_at (sync completed fully, not aborted)",
    actual: projectRow?.last_pulled_at,
    expected: now.toISOString(),
  })

  assert({
    given: "syncRepo with one artifact and .quarantine/ 404",
    should: "not emit any warnings (404 is treated as empty, not an error)",
    actual: warnings.length,
    expected: 0,
  })

  const testRunCount = (
    handle.raw.prepare("SELECT COUNT(*) as count FROM test_runs").get() as { count: number }
  ).count

  assert({
    given: "syncRepo with one artifact and .quarantine/ 404",
    should: "still ingest the artifact into test_runs (artifact sync unaffected by 404)",
    actual: testRunCount,
    expected: 1,
  })
})

describe("syncRepo() — partial state: backend has state.json, frontend returns 404", async (assert) => {
  const handle = initDb(":memory:")
  const owner = "mycargus"
  const repo = "my-app"
  const now = new Date("2026-04-14T12:00:00Z")
  const warnings: string[] = []

  // Directory listing discovers both backend/ and frontend/ subdirectories
  const directoryListing = [
    { name: "backend", path: ".quarantine/backend", type: "dir", sha: "sha-backend" },
    { name: "frontend", path: ".quarantine/frontend", type: "dir", sha: "sha-frontend" },
  ]

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

    // backend/state.json — returns 2 quarantined tests
    if (urlStr.includes("backend/state.json")) {
      const encoded = Buffer.from(makeStateJson("backend", 2)).toString("base64")
      return {
        ok: true,
        status: 200,
        json: async () => ({ content: encoded, sha: "state-sha-backend" }),
      }
    }

    // frontend/state.json — returns 404 (file doesn't exist yet)
    if (urlStr.includes("frontend/state.json")) {
      return {
        ok: false,
        status: 404,
        json: async () => ({ message: "Not Found" }),
      }
    }

    throw new Error(`Unexpected fetch call in partial state test: ${urlStr}`)
  }) as unknown as FetchFn

  await syncRepo(owner, repo, "fake-token", handle, now, fetchFn, (msg) => warnings.push(msg))

  const rows = handle.raw
    .prepare(
      "SELECT suite_name, quarantined_count FROM quarantine_state WHERE project_id = (SELECT id FROM projects WHERE owner = ? AND repo = ?) ORDER BY suite_name",
    )
    .all(owner, repo) as { suite_name: string; quarantined_count: number }[]

  assert({
    given: "2 suites listed in .quarantine/, only backend/state.json resolves",
    should: "store exactly 1 quarantine_state row (only backend)",
    actual: rows.length,
    expected: 1,
  })

  assert({
    given: "backend/state.json resolves with 2 quarantined tests",
    should: "record 2 quarantined tests for the backend suite",
    actual: rows.find((r) => r.suite_name === "backend")?.quarantined_count,
    expected: 2,
  })

  assert({
    given: "frontend/state.json returns 404 (suite listed but file absent)",
    should: "not create a quarantine_state row for the frontend suite",
    actual: rows.find((r) => r.suite_name === "frontend"),
    expected: undefined,
  })

  assert({
    given: "frontend/state.json returns 404",
    should: "emit no warnings (404 on a suite state.json is not an error)",
    actual: warnings.length,
    expected: 0,
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
