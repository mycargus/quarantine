import { unlinkSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway"
import {
  getLastPulledAt,
  getLastSynced,
  getProjects,
  initDb,
  updateLastPulledAt,
  updateLastSynced,
  upsertProject,
  upsertSuiteState,
} from "./db.server.js"

describe("initDb()", async (assert) => {
  const { db, raw } = initDb(":memory:")

  const tables = raw
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
    const { raw: fileRaw } = initDb(tmpPath)
    assert({
      given: "a file-based database",
      should: "enable WAL journal mode",
      actual: (fileRaw.pragma("journal_mode") as { journal_mode: string }[])[0]?.journal_mode,
      expected: "wal",
    })
    fileRaw.close()
  } finally {
    unlinkSync(tmpPath)
  }

  assert({
    given: "the same owner/repo pair inserted twice via raw SQL",
    should: "throw a UNIQUE constraint error on the second insert",
    actual: (() => {
      try {
        raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("test", "repo")
        raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("test", "repo")
        return false
      } catch {
        return true
      }
    })(),
    expected: true,
  })

  void db
})

describe("upsertProject()", async (assert) => {
  const { db } = initDb(":memory:")

  const firstId = await upsertProject(db, "mycargus", "my-app")

  assert({
    given: "a new owner/repo pair",
    should: "return a positive integer id",
    actual: typeof firstId === "number" && firstId > 0,
    expected: true,
  })

  const secondId = await upsertProject(db, "mycargus", "my-app")

  assert({
    given: "the same owner/repo inserted twice",
    should: "return the same id on the second call",
    actual: secondId,
    expected: firstId,
  })

  const otherId = await upsertProject(db, "acme", "payments-service")

  assert({
    given: "two different owner/repo pairs",
    should: "return distinct positive integer IDs",
    actual: otherId !== firstId && otherId > 0,
    expected: true,
  })
})

describe("getLastSynced()", async (assert) => {
  const { db } = initDb(":memory:")
  const projectId = await upsertProject(db, "acme", "widget")

  assert({
    given: "a project that has never been synced",
    should: "return null",
    actual: await getLastSynced(db, projectId),
    expected: null,
  })

  await updateLastSynced(db, projectId, "2026-03-15T14:00:00Z")

  assert({
    given: "a project after updateLastSynced sets a timestamp",
    should: "return that timestamp",
    actual: await getLastSynced(db, projectId),
    expected: "2026-03-15T14:00:00Z",
  })

  assert({
    given: "a projectId that does not exist in the database",
    should: "return null",
    actual: await getLastSynced(db, 99999),
    expected: null,
  })
})

describe("getProjects()", async (assert) => {
  assert({
    given: "an empty repos config",
    should: "return an empty array",
    actual: await getProjects(initDb(":memory:").db, []),
    expected: [],
  })

  const { db: dbNeverSynced } = initDb(":memory:")

  assert({
    given: "a config with 1 repo that has never been ingested (not in projects table)",
    should: "return testRunCount 0 and lastSynced null",
    actual: await getProjects(dbNeverSynced, [{ owner: "acme", repo: "payments-service" }]),
    expected: [{ owner: "acme", repo: "payments-service", testRunCount: 0, lastSynced: null }],
  })

  const { db: dbWithRuns, raw: rawWithRuns } = initDb(":memory:")
  const projectId = await upsertProject(dbWithRuns, "mycargus", "my-app")
  await updateLastSynced(dbWithRuns, projectId, "2025-01-15T10:30:00Z")
  for (let i = 0; i < 3; i++) {
    rawWithRuns
      .prepare(
        "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
      )
      .run(projectId, `run-${i}`, "main", `sha-${i}`, "2025-01-15T10:00:00Z", 10, 9, 0, 1)
  }

  assert({
    given: "a config with 1 repo that has 3 test runs and a last_synced timestamp",
    should: "return testRunCount 3 and the correct lastSynced value",
    actual: await getProjects(dbWithRuns, [{ owner: "mycargus", repo: "my-app" }]),
    expected: [
      { owner: "mycargus", repo: "my-app", testRunCount: 3, lastSynced: "2025-01-15T10:30:00Z" },
    ],
  })

  const { db: dbMixed, raw: rawMixed } = initDb(":memory:")
  const syncedId = await upsertProject(dbMixed, "mycargus", "my-app")
  await updateLastSynced(dbMixed, syncedId, "2025-01-15T10:30:00Z")
  for (let i = 0; i < 5; i++) {
    rawMixed
      .prepare(
        "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
      )
      .run(syncedId, `run-${i}`, "main", `sha-${i}`, "2025-01-15T10:00:00Z", 10, 9, 0, 1)
  }

  assert({
    given: "a config with 2 repos (one synced with 5 runs, one never synced)",
    should: "return both repos in config order with correct counts",
    actual: await getProjects(dbMixed, [
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
  const { db: dbBoth, raw: rawBoth } = initDb(":memory:")
  const idA = await upsertProject(dbBoth, "acme", "alpha")
  const idB = await upsertProject(dbBoth, "acme", "beta")
  await updateLastSynced(dbBoth, idA, "2025-01-01T00:00:00Z")
  await updateLastSynced(dbBoth, idB, "2025-01-02T00:00:00Z")
  rawBoth
    .prepare(
      "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
    )
    .run(idA, "run-a", "main", "sha-a", "2025-01-01T00:00:00Z", 1, 1, 0, 0)
  rawBoth
    .prepare(
      "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
    )
    .run(idB, "run-b1", "main", "sha-b1", "2025-01-02T00:00:00Z", 2, 2, 0, 0)
  rawBoth
    .prepare(
      "INSERT INTO test_runs (project_id, run_id, branch, commit_sha, timestamp, total_tests, passed_tests, failed_tests, flaky_tests) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
    )
    .run(idB, "run-b2", "main", "sha-b2", "2025-01-02T01:00:00Z", 2, 2, 0, 0)

  assert({
    given:
      "2 repos both present in DB with distinct run counts, queried in reverse DB insertion order",
    should: "return results matching config order, not DB insertion order",
    actual: await getProjects(dbBoth, [
      { owner: "acme", repo: "beta" },
      { owner: "acme", repo: "alpha" },
    ]),
    expected: [
      { owner: "acme", repo: "beta", testRunCount: 2, lastSynced: "2025-01-02T00:00:00Z" },
      { owner: "acme", repo: "alpha", testRunCount: 1, lastSynced: "2025-01-01T00:00:00Z" },
    ],
  })

  // D2: project in DB with last_synced but zero test runs
  const { db: dbZeroRuns } = initDb(":memory:")
  const zeroId = await upsertProject(dbZeroRuns, "acme", "empty-repo")
  await updateLastSynced(dbZeroRuns, zeroId, "2025-06-01T00:00:00Z")

  assert({
    given: "a repo in projects table with last_synced set but no test_runs",
    should: "return testRunCount 0 and lastSynced with the actual timestamp",
    actual: await getProjects(dbZeroRuns, [{ owner: "acme", repo: "empty-repo" }]),
    expected: [
      { owner: "acme", repo: "empty-repo", testRunCount: 0, lastSynced: "2025-06-01T00:00:00Z" },
    ],
  })

  // D3: duplicate repos in config
  const { db: dbDup } = initDb(":memory:")
  await upsertProject(dbDup, "acme", "dup-repo")

  assert({
    given: "a config array containing the same owner/repo twice",
    should: "return two entries for that repo (map preserves duplicates)",
    actual: await getProjects(dbDup, [
      { owner: "acme", repo: "dup-repo" },
      { owner: "acme", repo: "dup-repo" },
    ]),
    expected: [
      { owner: "acme", repo: "dup-repo", testRunCount: 0, lastSynced: null },
      { owner: "acme", repo: "dup-repo", testRunCount: 0, lastSynced: null },
    ],
  })

  // D4: case sensitivity — owner/repo match is case-sensitive
  const { db: dbCase } = initDb(":memory:")
  await upsertProject(dbCase, "acme", "my-repo")

  assert({
    given: "a repo stored as lowercase in DB but queried with different case in config",
    should: "return testRunCount 0 and lastSynced null (case-sensitive match)",
    actual: await getProjects(dbCase, [{ owner: "ACME", repo: "my-repo" }]),
    expected: [{ owner: "ACME", repo: "my-repo", testRunCount: 0, lastSynced: null }],
  })
})

describe("getLastPulledAt() and updateLastPulledAt()", async (assert) => {
  const { db } = initDb(":memory:")
  const projectId = await upsertProject(db, "acme", "widget-pull")

  assert({
    given: "a new project that has never been pulled",
    should: "return null",
    actual: await getLastPulledAt(db, projectId),
    expected: null,
  })

  await updateLastPulledAt(db, projectId, "2026-03-28T10:06:00Z")

  assert({
    given: "after calling updateLastPulledAt with a timestamp",
    should: "return that timestamp via getLastPulledAt",
    actual: await getLastPulledAt(db, projectId),
    expected: "2026-03-28T10:06:00Z",
  })

  await updateLastPulledAt(db, projectId, "2026-03-28T11:00:00Z")

  assert({
    given: "after calling updateLastPulledAt a second time with a newer timestamp",
    should: "last call wins — return the newer timestamp",
    actual: await getLastPulledAt(db, projectId),
    expected: "2026-03-28T11:00:00Z",
  })
})

describe("initDb() last_pulled_at migration idempotency", async (assert) => {
  const tmpPath = join(tmpdir(), `quarantine-debounce-test-${Date.now()}.db`)
  try {
    const { raw: raw1 } = initDb(tmpPath)
    raw1.close()
    let errorMsg: string | null = null
    try {
      const { raw: raw2 } = initDb(tmpPath)
      raw2.close()
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

describe("upsertSuiteState()", async (assert) => {
  {
    const { raw } = initDb(":memory:")
    const projectId = raw
      .prepare("INSERT INTO projects (owner, repo) VALUES (?, ?) RETURNING id")
      .get("acme", "widget") as { id: number }

    // First upsert: establish the row
    upsertSuiteState(raw, projectId.id, "suite-A", 3, '{"tests":["a","b","c"]}', "2026-01-01T00:00:00Z")

    const after1 = raw
      .prepare("SELECT quarantined_count, state_json FROM quarantine_state WHERE project_id = ? AND suite_name = ?")
      .get(projectId.id, "suite-A") as { quarantined_count: number; state_json: string }

    assert({
      given: "a first call to upsertSuiteState",
      should: "store quarantined_count 3",
      actual: after1.quarantined_count,
      expected: 3,
    })

    assert({
      given: "a first call to upsertSuiteState",
      should: "store the provided state_json",
      actual: after1.state_json,
      expected: '{"tests":["a","b","c"]}',
    })

    // Second upsert with the same project_id + suite_name but different values
    upsertSuiteState(raw, projectId.id, "suite-A", 7, '{"tests":["x","y","z","w"]}', "2026-02-01T00:00:00Z")

    const after2 = raw
      .prepare("SELECT quarantined_count, state_json FROM quarantine_state WHERE project_id = ? AND suite_name = ?")
      .get(projectId.id, "suite-A") as { quarantined_count: number; state_json: string }

    // Mutation guard: if quarantined_count = excluded.state_json and state_json = excluded.quarantined_count
    // were swapped, quarantined_count would be the JSON string and state_json would be the integer.
    assert({
      given: "a second call to upsertSuiteState with quarantined_count=7 and a new state_json",
      should: "update quarantined_count to 7 (not the state_json value)",
      actual: after2.quarantined_count,
      expected: 7,
    })

    assert({
      given: "a second call to upsertSuiteState with quarantined_count=7 and a new state_json",
      should: "update state_json to the new JSON string (not the quarantined_count value)",
      actual: after2.state_json,
      expected: '{"tests":["x","y","z","w"]}',
    })
  }
})

describe("updateLastSynced()", async (assert) => {
  const { db } = initDb(":memory:")
  const projectId = await upsertProject(db, "acme", "payments")

  await updateLastSynced(db, projectId, "2026-03-15T10:00:00Z")

  assert({
    given: "a first call to updateLastSynced",
    should: "set the last_synced timestamp",
    actual: await getLastSynced(db, projectId),
    expected: "2026-03-15T10:00:00Z",
  })

  await updateLastSynced(db, projectId, "2026-03-15T16:00:00Z")

  assert({
    given: "a second call to updateLastSynced with a newer timestamp",
    should: "update last_synced to the newer timestamp",
    actual: await getLastSynced(db, projectId),
    expected: "2026-03-15T16:00:00Z",
  })

  await updateLastSynced(db, projectId, "2026-03-15T08:00:00Z")

  assert({
    given: "a call to updateLastSynced with an older timestamp",
    should: "overwrite last_synced with the older value (no guard at DB layer)",
    actual: await getLastSynced(db, projectId),
    expected: "2026-03-15T08:00:00Z",
  })

  const otherProjectId = await upsertProject(db, "acme", "other")
  await updateLastSynced(db, 99999, "2026-01-01T00:00:00Z")

  assert({
    given: "a non-existent projectId",
    should: "not throw and not affect any existing project",
    actual: await getLastSynced(db, otherProjectId),
    expected: null,
  })
})
