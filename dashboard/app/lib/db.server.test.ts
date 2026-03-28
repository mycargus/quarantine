import { unlinkSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway/esm"
import { getLastSynced, initDb, updateLastSynced, upsertProject } from "./db.server.js"

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
