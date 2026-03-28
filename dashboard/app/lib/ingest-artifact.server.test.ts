import { describe } from "riteway/esm"
import { initDb } from "./db.server.js"
import { ingestArtifact } from "./ingest.server.js"
import type { TestResult } from "./ingest.server.js"

const validFixture: TestResult = {
  version: 1,
  run_id: "fixture-jest-flaky",
  repo: "mycargus/my-app",
  branch: "main",
  commit_sha: "aaa1234567890def1234567890abcdef12345678",
  pr_number: 99,
  timestamp: "2026-03-15T14:22:15Z",
  cli_version: "0.1.0",
  framework: "jest",
  config: {
    retry_count: 3,
  },
  summary: {
    total: 4,
    passed: 3,
    failed: 0,
    skipped: 0,
    quarantined: 0,
    flaky_detected: 1,
  },
  tests: [
    {
      test_id:
        "__tests__/services/user.test.js::UserService createUser::should create user with valid data",
      file_path: "__tests__/services/user.test.js",
      classname: "UserService createUser",
      name: "should create user with valid data",
      status: "passed",
      original_status: null,
      duration_ms: 156,
      failure_message: null,
      issue_number: null,
    },
  ],
}

const invalidFixture = { ...validFixture, run_id: undefined }

describe("ingestArtifact()", async (assert) => {
  {
    // Test 1: valid JSON → ingested, row exists
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const projectId = project.id
    const warnings: string[] = []
    const warn = (msg: string) => {
      warnings.push(msg)
    }

    const result = ingestArtifact(
      db,
      "mycargus",
      "my-app",
      "quarantine-results-100",
      JSON.stringify(validFixture),
      projectId,
      warn,
    )

    const row = db
      .prepare("SELECT run_id FROM test_runs WHERE run_id = ?")
      .get("fixture-jest-flaky") as { run_id: string } | undefined

    assert({
      given: "a valid JSON artifact",
      should: "return 'ingested'",
      actual: result,
      expected: "ingested",
    })

    assert({
      given: "a valid JSON artifact",
      should: "insert a row into test_runs",
      actual: row?.run_id,
      expected: "fixture-jest-flaky",
    })

    assert({
      given: "a valid JSON artifact",
      should: "not emit any warnings",
      actual: warnings.length,
      expected: 0,
    })
  }

  {
    // Test 2: invalid JSON (missing run_id) → skipped, warns, no row
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const projectId = project.id
    const warnings: string[] = []
    const warn = (msg: string) => {
      warnings.push(msg)
    }

    const result = ingestArtifact(
      db,
      "mycargus",
      "my-app",
      "quarantine-results-101",
      JSON.stringify(invalidFixture),
      projectId,
      warn,
    )

    const rows = db.prepare("SELECT * FROM test_runs").all()

    assert({
      given: "a JSON artifact missing the required run_id field",
      should: "return 'skipped'",
      actual: result,
      expected: "skipped",
    })

    assert({
      given: "a JSON artifact missing the required run_id field",
      should: "emit exactly one warning",
      actual: warnings.length,
      expected: 1,
    })

    assert({
      given: "a JSON artifact missing the required run_id field",
      should: "warn with the exact message format",
      actual: warnings[0],
      expected:
        "[ingest] WARNING: skipping artifact quarantine-results-101 for mycargus/my-app: validation failed",
    })

    assert({
      given: "a JSON artifact missing the required run_id field",
      should: "not insert any row into test_runs",
      actual: rows.length,
      expected: 0,
    })
  }

  {
    // Test 3: syntactically malformed JSON → skipped, warns, no row
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const projectId = project.id
    const warnings: string[] = []
    const warn = (msg: string) => {
      warnings.push(msg)
    }

    const result = ingestArtifact(
      db,
      "mycargus",
      "my-app",
      "quarantine-results-101",
      "not-json",
      projectId,
      warn,
    )

    const rows = db.prepare("SELECT * FROM test_runs").all()

    assert({
      given: "a syntactically malformed JSON string",
      should: "return 'skipped'",
      actual: result,
      expected: "skipped",
    })

    assert({
      given: "a syntactically malformed JSON string",
      should: "emit exactly one warning",
      actual: warnings.length,
      expected: 1,
    })

    assert({
      given: "a syntactically malformed JSON string",
      should: "warn with the exact message format",
      actual: warnings[0],
      expected:
        "[ingest] WARNING: skipping artifact quarantine-results-101 for mycargus/my-app: validation failed",
    })

    assert({
      given: "a syntactically malformed JSON string",
      should: "not insert any row into test_runs",
      actual: rows.length,
      expected: 0,
    })
  }

  {
    // G1: idempotency — same run_id ingested twice
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const projectId = project.id
    const warn = (_msg: string) => {}

    ingestArtifact(
      db,
      "mycargus",
      "my-app",
      "quarantine-results-100",
      JSON.stringify(validFixture),
      projectId,
      warn,
    )
    const secondResult = ingestArtifact(
      db,
      "mycargus",
      "my-app",
      "quarantine-results-100",
      JSON.stringify(validFixture),
      projectId,
      warn,
    )
    const rowCount = (
      db.prepare("SELECT COUNT(*) as count FROM test_runs").get() as { count: number }
    ).count

    assert({
      given: "a valid artifact ingested twice with the same run_id",
      should: "return 'ingested' on the second call (idempotent)",
      actual: secondResult,
      expected: "ingested",
    })

    assert({
      given: "a valid artifact ingested twice with the same run_id",
      should: "store exactly one row",
      actual: rowCount,
      expected: 1,
    })
  }

  {
    // G2: JSON primitive (valid JSON, schema-invalid)
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const warnings: string[] = []

    assert({
      given: "a JSON string whose top-level value is null",
      should: "return 'skipped' and emit a warning",
      actual: (() => {
        const r = ingestArtifact(
          db,
          "mycargus",
          "my-app",
          "quarantine-results-101",
          "null",
          project.id,
          (m) => warnings.push(m),
        )
        return { result: r, warned: warnings.length === 1 }
      })(),
      expected: { result: "skipped", warned: true },
    })
  }

  {
    // G3: empty string input
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const warnings: string[] = []

    assert({
      given: "an empty string (artifact download returned empty body)",
      should: "return 'skipped' and emit a warning",
      actual: (() => {
        const r = ingestArtifact(
          db,
          "mycargus",
          "my-app",
          "quarantine-results-101",
          "",
          project.id,
          (m) => warnings.push(m),
        )
        return { result: r, warned: warnings.length === 1 }
      })(),
      expected: { result: "skipped", warned: true },
    })
  }

  {
    // G4: unsupported framework enum value
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const warnings: string[] = []
    const badFramework = { ...validFixture, framework: "mocha" }

    assert({
      given: "an artifact with an unsupported framework value",
      should: "return 'skipped' and emit a warning",
      actual: (() => {
        const r = ingestArtifact(
          db,
          "mycargus",
          "my-app",
          "quarantine-results-101",
          JSON.stringify(badFramework),
          project.id,
          (m) => warnings.push(m),
        )
        return { result: r, warned: warnings.length === 1 }
      })(),
      expected: { result: "skipped", warned: true },
    })
  }

  {
    // G5: pr_number = 0 (below schema minimum of 1)
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const warnings: string[] = []
    const badPr = { ...validFixture, run_id: "run-pr-0", pr_number: 0 }

    assert({
      given: "an artifact where pr_number is 0 (below schema minimum)",
      should: "return 'skipped' and emit a warning",
      actual: (() => {
        const r = ingestArtifact(
          db,
          "mycargus",
          "my-app",
          "quarantine-results-101",
          JSON.stringify(badPr),
          project.id,
          (m) => warnings.push(m),
        )
        return { result: r, warned: warnings.length === 1 }
      })(),
      expected: { result: "skipped", warned: true },
    })
  }

  {
    // G6: valid artifact with pr_number: null
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const noPrFixture: TestResult = { ...validFixture, run_id: "run-no-pr", pr_number: null }

    const result = ingestArtifact(
      db,
      "mycargus",
      "my-app",
      "quarantine-results-103",
      JSON.stringify(noPrFixture),
      project.id,
      (_m) => {},
    )
    const row = db.prepare("SELECT pr_number FROM test_runs WHERE run_id = ?").get("run-no-pr") as
      | { pr_number: number | null }
      | undefined

    assert({
      given: "a valid artifact where pr_number is null",
      should: "return 'ingested' and store a row with null pr_number",
      actual: { result, prNumber: row?.pr_number },
      expected: { result: "ingested", prNumber: null },
    })
  }

  {
    // G7: valid artifact with empty tests array
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const emptyTests: TestResult = { ...validFixture, run_id: "run-empty-tests", tests: [] }

    assert({
      given: "a valid artifact with an empty tests array",
      should: "return 'ingested' and insert one row",
      actual: ingestArtifact(
        db,
        "mycargus",
        "my-app",
        "quarantine-results-104",
        JSON.stringify(emptyTests),
        project.id,
        (_m) => {},
      ),
      expected: "ingested",
    })
  }

  {
    // G8: invalid projectId → upsertTestRun fails; must not throw, must return 'skipped'
    const db = initDb(":memory:")
    const warnings: string[] = []

    assert({
      given: "a valid artifact but a projectId that does not exist in projects (FK violation)",
      should: "return 'skipped' and emit a warning (never throws)",
      actual: (() => {
        const r = ingestArtifact(
          db,
          "mycargus",
          "my-app",
          "quarantine-results-101",
          JSON.stringify(validFixture),
          99999,
          (m) => warnings.push(m),
        )
        return { result: r, warned: warnings.length === 1 }
      })(),
      expected: { result: "skipped", warned: true },
    })
  }

  {
    // Test 4: three artifacts in sequence — artifact 101 (invalid) does not stop processing
    const db = initDb(":memory:")
    db.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
    const project = db
      .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
      .get("mycargus", "my-app") as { id: number }
    const projectId = project.id
    const warnings: string[] = []
    const warn = (msg: string) => {
      warnings.push(msg)
    }

    const fixture100: TestResult = { ...validFixture, run_id: "run-100" }
    const fixture102: TestResult = { ...validFixture, run_id: "run-102" }

    const results = [
      ingestArtifact(
        db,
        "mycargus",
        "my-app",
        "quarantine-results-100",
        JSON.stringify(fixture100),
        projectId,
        warn,
      ),
      ingestArtifact(
        db,
        "mycargus",
        "my-app",
        "quarantine-results-101",
        JSON.stringify(invalidFixture),
        projectId,
        warn,
      ),
      ingestArtifact(
        db,
        "mycargus",
        "my-app",
        "quarantine-results-102",
        JSON.stringify(fixture102),
        projectId,
        warn,
      ),
    ]

    const rowCount = (
      db.prepare("SELECT COUNT(*) as count FROM test_runs").get() as { count: number }
    ).count

    assert({
      given: "three artifacts where artifact 101 is invalid",
      should: "return ingested, skipped, ingested in order",
      actual: results,
      expected: ["ingested", "skipped", "ingested"],
    })

    assert({
      given: "three artifacts where artifact 101 is invalid",
      should: "insert exactly 2 rows into test_runs (artifacts 100 and 102)",
      actual: rowCount,
      expected: 2,
    })

    assert({
      given: "three artifacts where artifact 101 is invalid",
      should: "emit exactly one warning (for artifact 101 only)",
      actual: warnings.length,
      expected: 1,
    })
  }
})
