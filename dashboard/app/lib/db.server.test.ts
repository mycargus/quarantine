import { unlinkSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway/esm"
import {
  getLastPulledAt,
  getLastSynced,
  getProjects,
  initDb,
  updateLastPulledAt,
  updateLastSynced,
  upsertProject,
} from "./db.server.js"

const throwsSync = (fn: () => unknown): string | null => {
  try {
    fn()
    return null
  } catch (e) {
    return e instanceof Error ? e.message : String(e)
  }
}

describe("initDb()", async (assert) => {
  const db = initDb(":memory:")

  const tables = db
    .prepare("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
    .all() as { name: string }[]
  const tableNames = tables.map((t) => t.name)

  assert({
    given: "an in-memory database",
    should: "create the projects table",
    actual: tableNames.includes("projects"),
    expected: true,
  })

  assert({
    given: "an in-memory database",
    should: "create the test_runs table",
    actual: tableNames.includes("test_runs"),
    expected: true,
  })

  // WAL mode only applies to file-based DBs (not :memory:), so test with a temp file
  const tmpPath = join(tmpdir(), `quarantine-wal-test-${Date.now()}.db`)
  try {
    const fileDb = initDb(tmpPath)
    assert({
      given: "a file-based database",
      should: "enable WAL journal mode",
      actual: (fileDb.pragma("journal_mode") as { journal_mode: string }[])[0]?.journal_mode,
      expected: "wal",
    })
    fileDb.close()
  } finally {
    unlinkSync(tmpPath)
  }

  assert({
    given: "the same owner/repo pair inserted twice via raw SQL",
    should: "throw a UNIQUE constraint error on the second insert",
    actual:
      throwsSync(() => {
        db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("test", "repo")
        db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("test", "repo")
      }) !== null,
    expected: true,
  })
})

describe("upsertProject()", async (assert) => {
  const db = initDb(":memory:")

  const firstId = upsertProject(db, "mycargus", "my-app")

  assert({
    given: "a new owner/repo pair",
    should: "return a positive integer id",
    actual: typeof firstId === "number" && firstId > 0,
    expected: true,
  })

  const secondId = upsertProject(db, "mycargus", "my-app")

  assert({
    given: "the same owner/repo inserted twice",
    should: "return the same id on the second call",
    actual: secondId,
    expected: firstId,
  })

  const otherId = upsertProject(db, "acme", "payments-service")

  assert({
    given: "two different owner/repo pairs",
    should: "return distinct positive integer IDs",
    actual: otherId !== firstId && otherId > 0,
    expected: true,
  })
})

describe("getLastSynced()", async (assert) => {
  const db = initDb(":memory:")
  const projectId = upsertProject(db, "acme", "widget")

  assert({
    given: "a project that has never been synced",
    should: "return null",
    actual: getLastSynced(db, projectId),
    expected: null,
  })

  updateLastSynced(db, projectId, "2026-03-15T14:00:00Z")

  assert({
    given: "a project after updateLastSynced sets a timestamp",
    should: "return that timestamp",
    actual: getLastSynced(db, projectId),
    expected: "2026-03-15T14:00:00Z",
  })

  assert({
    given: "a projectId that does not exist in the database",
    should: "return null",
    actual: getLastSynced(db, 99999),
    expected: null,
  })
})

describe("getProjects()", async (assert) => {
  assert({
    given: "an empty repos config",
    should: "return an empty array",
    actual: getProjects(initDb(":memory:"), []),
    expected: [],
  })

  const dbNeverSynced = initDb(":memory:")

  assert({
    given: "a config with 1 repo that has never been ingested (not in projects table)",
    should: "return testRunCount 0 and lastSynced null",
    actual: getProjects(dbNeverSynced, [{ owner: "acme", repo: "payments-service" }]),
    expected: [{ owner: "acme", repo: "payments-service", testRunCount: 0, lastSynced: null }],
  })

  const dbWithRuns = initDb(":memory:")
  const projectId = upsertProject(dbWithRuns, "mycargus", "my-app")
  updateLastSynced(dbWithRuns, projectId, "2025-01-15T10:30:00Z")
  for (let i = 0; i < 3; i++) {
    dbWithRuns
      .prepare(
        "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
      )
      .run(projectId, `run-${i}`, "main", `sha-${i}`, "2025-01-15T10:00:00Z", 10, 9, 0, 1)
  }

  assert({
    given: "a config with 1 repo that has 3 test runs and a last_synced timestamp",
    should: "return testRunCount 3 and the correct lastSynced value",
    actual: getProjects(dbWithRuns, [{ owner: "mycargus", repo: "my-app" }]),
    expected: [
      { owner: "mycargus", repo: "my-app", testRunCount: 3, lastSynced: "2025-01-15T10:30:00Z" },
    ],
  })

  const dbMixed = initDb(":memory:")
  const syncedId = upsertProject(dbMixed, "mycargus", "my-app")
  updateLastSynced(dbMixed, syncedId, "2025-01-15T10:30:00Z")
  for (let i = 0; i < 5; i++) {
    dbMixed
      .prepare(
        "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
      )
      .run(syncedId, `run-${i}`, "main", `sha-${i}`, "2025-01-15T10:00:00Z", 10, 9, 0, 1)
  }

  assert({
    given: "a config with 2 repos (one synced with 5 runs, one never synced)",
    should: "return both repos in config order with correct counts",
    actual: getProjects(dbMixed, [
      { owner: "mycargus", repo: "my-app" },
      { owner: "acme", repo: "payments-service" },
    ]),
    expected: [
      {
        owner: "mycargus",
        repo: "my-app",
        testRunCount: 5,
        lastSynced: "2025-01-15T10:30:00Z",
      },
      { owner: "acme", repo: "payments-service", testRunCount: 0, lastSynced: null },
    ],
  })

  // D1: config order preserved when both repos exist in DB
  const dbBoth = initDb(":memory:")
  const idA = upsertProject(dbBoth, "acme", "alpha")
  const idB = upsertProject(dbBoth, "acme", "beta")
  updateLastSynced(dbBoth, idA, "2025-01-01T00:00:00Z")
  updateLastSynced(dbBoth, idB, "2025-01-02T00:00:00Z")
  dbBoth
    .prepare(
      "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
    )
    .run(idA, "run-a", "main", "sha-a", "2025-01-01T00:00:00Z", 1, 1, 0, 0)
  dbBoth
    .prepare(
      "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
    )
    .run(idB, "run-b1", "main", "sha-b1", "2025-01-02T00:00:00Z", 2, 2, 0, 0)
  dbBoth
    .prepare(
      "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
    )
    .run(idB, "run-b2", "main", "sha-b2", "2025-01-02T01:00:00Z", 2, 2, 0, 0)

  assert({
    given:
      "2 repos both present in DB with distinct run counts, queried in reverse DB insertion order",
    should: "return results matching config order, not DB insertion order",
    actual: getProjects(dbBoth, [
      { owner: "acme", repo: "beta" },
      { owner: "acme", repo: "alpha" },
    ]),
    expected: [
      { owner: "acme", repo: "beta", testRunCount: 2, lastSynced: "2025-01-02T00:00:00Z" },
      { owner: "acme", repo: "alpha", testRunCount: 1, lastSynced: "2025-01-01T00:00:00Z" },
    ],
  })

  // D2: project in DB with last_synced but zero test runs
  const dbZeroRuns = initDb(":memory:")
  const zeroId = upsertProject(dbZeroRuns, "acme", "empty-repo")
  updateLastSynced(dbZeroRuns, zeroId, "2025-06-01T00:00:00Z")

  assert({
    given: "a repo in projects table with last_synced set but no test_runs",
    should: "return testRunCount 0 and lastSynced with the actual timestamp",
    actual: getProjects(dbZeroRuns, [{ owner: "acme", repo: "empty-repo" }]),
    expected: [
      { owner: "acme", repo: "empty-repo", testRunCount: 0, lastSynced: "2025-06-01T00:00:00Z" },
    ],
  })

  // D3: duplicate repos in config
  const dbDup = initDb(":memory:")
  upsertProject(dbDup, "acme", "dup-repo")

  assert({
    given: "a config array containing the same owner/repo twice",
    should: "return two entries for that repo (map preserves duplicates)",
    actual: getProjects(dbDup, [
      { owner: "acme", repo: "dup-repo" },
      { owner: "acme", repo: "dup-repo" },
    ]),
    expected: [
      { owner: "acme", repo: "dup-repo", testRunCount: 0, lastSynced: null },
      { owner: "acme", repo: "dup-repo", testRunCount: 0, lastSynced: null },
    ],
  })

  // D4: case sensitivity — owner/repo match is case-sensitive
  const dbCase = initDb(":memory:")
  upsertProject(dbCase, "acme", "my-repo")

  assert({
    given: "a repo stored as lowercase in DB but queried with different case in config",
    should: "return testRunCount 0 and lastSynced null (case-sensitive match)",
    actual: getProjects(dbCase, [{ owner: "ACME", repo: "my-repo" }]),
    expected: [{ owner: "ACME", repo: "my-repo", testRunCount: 0, lastSynced: null }],
  })
})

describe("getLastPulledAt() and updateLastPulledAt()", async (assert) => {
  const db = initDb(":memory:")
  const projectId = upsertProject(db, "acme", "widget-pull")

  assert({
    given: "a new project that has never been pulled",
    should: "return null",
    actual: getLastPulledAt(db, projectId),
    expected: null,
  })

  updateLastPulledAt(db, projectId, "2026-03-28T10:06:00Z")

  assert({
    given: "after calling updateLastPulledAt with a timestamp",
    should: "return that timestamp via getLastPulledAt",
    actual: getLastPulledAt(db, projectId),
    expected: "2026-03-28T10:06:00Z",
  })

  updateLastPulledAt(db, projectId, "2026-03-28T11:00:00Z")

  assert({
    given: "after calling updateLastPulledAt a second time with a newer timestamp",
    should: "last call wins — return the newer timestamp",
    actual: getLastPulledAt(db, projectId),
    expected: "2026-03-28T11:00:00Z",
  })
})

describe("initDb() last_pulled_at migration idempotency", async (assert) => {
  const tmpPath = join(tmpdir(), `quarantine-debounce-test-${Date.now()}.db`)
  try {
    const db1 = initDb(tmpPath)
    db1.close()
    // Second call should not throw even though last_pulled_at column already exists
    let errorMsg: string | null = null
    try {
      const db2 = initDb(tmpPath)
      db2.close()
    } catch (e) {
      errorMsg = e instanceof Error ? e.message : String(e)
    }

    assert({
      given:
        "initDb called twice on the same file-based DB (last_pulled_at column already present)",
      should: "not throw an error",
      actual: errorMsg,
      expected: null,
    })
  } finally {
    unlinkSync(tmpPath)
  }
})

describe("updateLastSynced()", async (assert) => {
  const db = initDb(":memory:")
  const projectId = upsertProject(db, "acme", "payments")

  updateLastSynced(db, projectId, "2026-03-15T10:00:00Z")

  assert({
    given: "a first call to updateLastSynced",
    should: "set the last_synced timestamp",
    actual: getLastSynced(db, projectId),
    expected: "2026-03-15T10:00:00Z",
  })

  updateLastSynced(db, projectId, "2026-03-15T16:00:00Z")

  assert({
    given: "a second call to updateLastSynced with a newer timestamp",
    should: "update last_synced to the newer timestamp",
    actual: getLastSynced(db, projectId),
    expected: "2026-03-15T16:00:00Z",
  })

  updateLastSynced(db, projectId, "2026-03-15T08:00:00Z")

  assert({
    given: "a call to updateLastSynced with an older timestamp",
    should: "overwrite last_synced with the older value (no guard at DB layer)",
    actual: getLastSynced(db, projectId),
    expected: "2026-03-15T08:00:00Z",
  })

  const otherProjectId = upsertProject(db, "acme", "other")
  updateLastSynced(db, 99999, "2026-01-01T00:00:00Z")

  assert({
    given: "a non-existent projectId",
    should: "not throw and not affect any existing project",
    actual: getLastSynced(db, otherProjectId),
    expected: null,
  })
})
