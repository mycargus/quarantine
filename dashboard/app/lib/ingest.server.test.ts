import { describe } from "riteway"
import { initDb } from "./db.server.js"
import type { Artifact, TestResult } from "./ingest.server.js"
import {
  buildIssueUrl,
  buildTestRunRecord,
  filterArtifactsByPrefix,
  filterArtifactsSince,
  mapOriginalStatus,
  sortArtifactsChronologically,
  upsertTestRun,
  validateTestResult,
} from "./ingest.server.js"

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

describe("filterArtifactsByPrefix()", async (assert) => {
  const artifacts: Artifact[] = [
    {
      id: 1,
      name: "quarantine-results-run-123",
      archive_download_url: "https://example.com/1",
      created_at: "2026-03-15T14:00:00Z",
      expires_at: "2026-04-15T14:00:00Z",
    },
    {
      id: 2,
      name: "coverage-report",
      archive_download_url: "https://example.com/2",
      created_at: "2026-03-15T14:00:00Z",
      expires_at: "2026-04-15T14:00:00Z",
    },
    {
      id: 3,
      name: "quarantine-results-run-456",
      archive_download_url: "https://example.com/3",
      created_at: "2026-03-15T14:00:00Z",
      expires_at: "2026-04-15T14:00:00Z",
    },
  ]

  assert({
    given: "a list of artifacts with mixed names",
    should: "return only artifacts matching the prefix",
    actual: filterArtifactsByPrefix(artifacts, "quarantine-results").map((a) => a.id),
    expected: [1, 3],
  })

  assert({
    given: "a list of artifacts where none match the prefix",
    should: "return an empty array",
    actual: filterArtifactsByPrefix(
      [
        {
          id: 2,
          name: "coverage-report",
          archive_download_url: "u",
          created_at: "t",
          expires_at: "t",
        },
      ],
      "quarantine-results",
    ),
    expected: [],
  })

  assert({
    given: "an empty artifact array",
    should: "return an empty array",
    actual: filterArtifactsByPrefix([], "quarantine-results"),
    expected: [],
  })

  assert({
    given: "an empty prefix string",
    should: "return all artifacts (startsWith('') is always true)",
    actual: filterArtifactsByPrefix(artifacts, "").map((a) => a.id),
    expected: [1, 2, 3],
  })
})

describe("validateTestResult()", async (assert) => {
  assert({
    given: "a valid test result fixture",
    should: "return { valid: true, errors: [] }",
    actual: validateTestResult(validFixture),
    expected: { valid: true, errors: [] },
  })

  assert({
    given: "an object missing run_id",
    should: "return valid: false with an error mentioning run_id",
    actual: (() => {
      const result = validateTestResult({ ...validFixture, run_id: undefined })
      return { valid: result.valid, hasError: result.errors.some((e) => e.includes("run_id")) }
    })(),
    expected: { valid: false, hasError: true },
  })

  assert({
    given: "an object with version set to a string instead of integer",
    should: "return valid: false",
    actual: validateTestResult({ ...validFixture, version: "1" as unknown as number }).valid,
    expected: false,
  })

  assert({
    given: "null",
    should: "return { valid: false } with at least one error",
    actual: (() => {
      const r = validateTestResult(null)
      return { valid: r.valid, hasErrors: r.errors.length > 0 }
    })(),
    expected: { valid: false, hasErrors: true },
  })

  assert({
    given: "an empty object",
    should: "return valid: false with at least one error",
    actual: (() => {
      const r = validateTestResult({})
      return { valid: r.valid, hasErrors: r.errors.length > 0 }
    })(),
    expected: { valid: false, hasErrors: true },
  })

  assert({
    given: "a missing required field",
    should: "return an error string in the format containing the field name",
    actual: validateTestResult({ ...validFixture, run_id: undefined }).errors.some((e) =>
      e.includes("run_id"),
    ),
    expected: true,
  })
})

describe("buildTestRunRecord()", async (assert) => {
  assert({
    given: "a valid TestResult and a projectId",
    should: "map all fields to a TestRunRecord",
    actual: buildTestRunRecord(validFixture, 42),
    expected: {
      projectId: 42,
      runId: "fixture-jest-flaky",
      branch: "main",
      commitSha: "aaa1234567890def1234567890abcdef12345678",
      prNumber: 99,
      timestamp: "2026-03-15T14:22:15Z",
      totalTests: 4,
      passedTests: 3,
      failedTests: 0,
      flakyTests: 1,
    },
  })

  assert({
    given: "a TestResult with null pr_number",
    should: "map prNumber as null",
    actual: buildTestRunRecord({ ...validFixture, pr_number: null }, 1).prNumber,
    expected: null,
  })
})

describe("filterArtifactsSince()", async (assert) => {
  const artifacts: Artifact[] = [
    {
      id: 1,
      name: "quarantine-results-run-1",
      archive_download_url: "https://example.com/1",
      created_at: "2026-03-15T10:00:00Z",
      expires_at: "2026-04-15T10:00:00Z",
    },
    {
      id: 2,
      name: "quarantine-results-run-2",
      archive_download_url: "https://example.com/2",
      created_at: "2026-03-15T12:00:00Z",
      expires_at: "2026-04-15T12:00:00Z",
    },
    {
      id: 3,
      name: "quarantine-results-run-3",
      archive_download_url: "https://example.com/3",
      created_at: "2026-03-15T14:00:00Z",
      expires_at: "2026-04-15T14:00:00Z",
    },
  ]

  assert({
    given: "lastSynced is null",
    should: "return all artifacts (first sync)",
    actual: filterArtifactsSince(artifacts, null).map((a) => a.id),
    expected: [1, 2, 3],
  })

  assert({
    given: "lastSynced filters to newer artifacts",
    should: "return only artifacts created after lastSynced",
    actual: filterArtifactsSince(artifacts, "2026-03-15T11:00:00Z").map((a) => a.id),
    expected: [2, 3],
  })

  assert({
    given: "an artifact with created_at exactly equal to lastSynced",
    should: "exclude that artifact (strict greater-than)",
    actual: filterArtifactsSince(artifacts, "2026-03-15T12:00:00Z").map((a) => a.id),
    expected: [3],
  })

  assert({
    given: "an empty artifact array",
    should: "return an empty array",
    actual: filterArtifactsSince([], "2026-03-15T12:00:00Z"),
    expected: [],
  })

  assert({
    given: "lastSynced after all artifact timestamps",
    should: "return an empty array",
    actual: filterArtifactsSince(artifacts, "2026-03-15T15:00:00Z").map((a) => a.id),
    expected: [],
  })
})

describe("sortArtifactsChronologically()", async (assert) => {
  const artifactA: Artifact = {
    id: 1,
    name: "run-1",
    archive_download_url: "https://example.com/1",
    created_at: "2026-03-15T10:00:00Z",
    expires_at: "2026-04-15T10:00:00Z",
  }
  const artifactB: Artifact = {
    id: 2,
    name: "run-2",
    archive_download_url: "https://example.com/2",
    created_at: "2026-03-15T12:00:00Z",
    expires_at: "2026-04-15T12:00:00Z",
  }
  const artifactC: Artifact = {
    id: 3,
    name: "run-3",
    archive_download_url: "https://example.com/3",
    created_at: "2026-03-15T14:00:00Z",
    expires_at: "2026-04-15T14:00:00Z",
  }

  assert({
    given: "artifacts in reverse chronological order",
    should: "sort them oldest-first",
    actual: sortArtifactsChronologically([artifactC, artifactA, artifactB]).map((a) => a.id),
    expected: [1, 2, 3],
  })

  assert({
    given: "artifacts already in chronological order",
    should: "return them in the same order",
    actual: sortArtifactsChronologically([artifactA, artifactB, artifactC]).map((a) => a.id),
    expected: [1, 2, 3],
  })

  assert({
    given: "a single artifact",
    should: "return it unchanged",
    actual: sortArtifactsChronologically([artifactB]).map((a) => a.id),
    expected: [2],
  })

  assert({
    given: "the original array",
    should: "not be mutated by sorting",
    actual: (() => {
      const input = [artifactC, artifactA, artifactB]
      sortArtifactsChronologically(input)
      return input.map((a) => a.id)
    })(),
    expected: [3, 1, 2],
  })

  assert({
    given: "an empty artifact array",
    should: "return an empty array",
    actual: sortArtifactsChronologically([]),
    expected: [],
  })

  assert({
    given: "two artifacts with identical created_at",
    should: "preserve their relative input order (stable sort)",
    actual: (() => {
      const same = "2026-03-15T12:00:00Z"
      const first: Artifact = {
        id: 10,
        name: "first",
        archive_download_url: "u",
        created_at: same,
        expires_at: "t",
      }
      const second: Artifact = {
        id: 11,
        name: "second",
        archive_download_url: "u",
        created_at: same,
        expires_at: "t",
      }
      return sortArtifactsChronologically([first, second]).map((a) => a.id)
    })(),
    expected: [10, 11],
  })
})

describe("upsertTestRun()", async (assert) => {
  const { db, raw } = initDb(":memory:")

  // Insert a project first so the foreign key is satisfied
  raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)").run("mycargus", "my-app")
  const project = raw
    .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
    .get("mycargus", "my-app") as { id: number }
  const projectId = project.id

  await upsertTestRun(db, projectId, validFixture)
  const row = raw.prepare("SELECT * FROM test_runs WHERE run_id = ?").get("fixture-jest-flaky") as
    | Record<string, unknown>
    | undefined

  assert({
    given: "a valid TestResult inserted into an empty DB",
    should: "create a row with the correct run_id",
    actual: row?.run_id,
    expected: "fixture-jest-flaky",
  })

  assert({
    given: "a valid TestResult inserted into an empty DB",
    should: "store branch correctly",
    actual: row?.branch,
    expected: "main",
  })

  // Second upsert with same run_id — should be idempotent
  await upsertTestRun(db, projectId, validFixture)
  const allRows = raw.prepare("SELECT * FROM test_runs WHERE run_id = ?").all("fixture-jest-flaky")

  assert({
    given: "the same TestResult upserted twice",
    should: "result in only one row (idempotent)",
    actual: allRows.length,
    expected: 1,
  })

  const nullPrFixture = { ...validFixture, run_id: "fixture-null-pr", pr_number: null }
  await upsertTestRun(db, projectId, nullPrFixture)
  const nullPrRow = raw
    .prepare("SELECT pr_number FROM test_runs WHERE run_id = ?")
    .get("fixture-null-pr") as { pr_number: number | null } | undefined

  assert({
    given: "a TestResult with null pr_number",
    should: "store NULL in the pr_number column",
    actual: nullPrRow?.pr_number,
    expected: null,
  })
})

describe("mapOriginalStatus()", async (assert) => {
  assert({
    given: 'original_status "failed"',
    should: 'return "failing"',
    actual: mapOriginalStatus("failed"),
    expected: "failing",
  })

  assert({
    given: 'original_status "passed"',
    should: 'return "passing"',
    actual: mapOriginalStatus("passed"),
    expected: "passing",
  })

  assert({
    given: "original_status null",
    should: "return null",
    actual: mapOriginalStatus(null),
    expected: null,
  })

  assert({
    given: 'an unknown original_status string "skipped"',
    should: "return null",
    actual: mapOriginalStatus("skipped"),
    expected: null,
  })
})

describe("buildIssueUrl()", async (assert) => {
  assert({
    given: "owner mycargus, repo my-app, issue_number 42",
    should: "return the correct GitHub issue URL",
    actual: buildIssueUrl("mycargus", "my-app", 42),
    expected: "https://github.com/mycargus/my-app/issues/42",
  })

  assert({
    given: "issue_number 0 (falsy but valid)",
    should: "still return a correct URL with /issues/0",
    actual: buildIssueUrl("owner", "repo", 0),
    expected: "https://github.com/owner/repo/issues/0",
  })
})

describe("filterArtifactsSince() — empty string lastSynced", async (assert) => {
  const artifacts: Artifact[] = [
    {
      id: 1,
      name: "run-1",
      archive_download_url: "u",
      created_at: "2026-03-15T10:00:00Z",
      expires_at: "t",
    },
    {
      id: 2,
      name: "run-2",
      archive_download_url: "u",
      created_at: "2026-03-15T12:00:00Z",
      expires_at: "t",
    },
  ]

  assert({
    given: "lastSynced is an empty string",
    should: "return all artifacts (empty string sorts before all ISO timestamps)",
    actual: filterArtifactsSince(artifacts, "").map((a) => a.id),
    expected: [1, 2],
  })
})
