import { unlinkSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway/esm"
import { initDb, upsertProject } from "./db.server.js"

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
