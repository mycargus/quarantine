/**
 * Artifact JSON ingestion for the quarantine dashboard.
 *
 * Parses quarantine result JSON from GitHub Artifacts, validates against
 * the test-result schema, and upserts into SQLite. Keyed by run_id for
 * idempotency.
 */

import addFormats from "ajv-formats"
import Ajv2020 from "ajv/dist/2020"
import type { Database } from "better-sqlite3"
import schema from "../../../schemas/test-result.schema.json" with { type: "json" }

export interface Artifact {
  id: number
  name: string
  archive_download_url: string
  created_at: string
  expires_at: string
}

export interface ListArtifactsResult {
  artifacts: Artifact[]
  etag: string | null
  notModified: boolean
}

export interface TestResult {
  version: number
  run_id: string
  repo: string
  branch: string
  commit_sha: string
  pr_number: number | null
  timestamp: string
  cli_version: string
  framework: string
  config: {
    retry_count: number
    excluded_patterns?: string[]
    excluded_count?: number
  }
  summary: {
    total: number
    passed: number
    failed: number
    skipped: number
    quarantined: number
    flaky_detected: number
  }
  tests: TestEntry[]
}

export interface TestEntry {
  test_id: string
  file_path: string
  classname: string
  name: string
  status: string
  original_status: string | null
  duration_ms: number
  failure_message: string | null
  issue_number: number | null
}

export interface TestRunRecord {
  projectId: number
  runId: string
  branch: string
  commitSha: string
  prNumber: number | null
  timestamp: string
  totalTests: number
  passedTests: number
  failedTests: number
  flakyTests: number
}

const ajv = new Ajv2020()
addFormats(ajv)
const validate = ajv.compile(schema)

/**
 * Pure: filters artifacts by name prefix.
 */
export function filterArtifactsByPrefix(artifacts: Artifact[], prefix: string): Artifact[] {
  return artifacts.filter((a) => a.name.startsWith(prefix))
}

/**
 * Pure: validates a test result against the JSON schema.
 */
export function validateTestResult(data: unknown): { valid: boolean; errors: string[] } {
  const valid = validate(data)
  if (valid) {
    return { valid: true, errors: [] }
  }
  const errors = (validate.errors ?? []).map((e) => {
    const field =
      e.instancePath.replace(/^\//, "") || ((e.params?.missingProperty as string | undefined) ?? "")
    return field ? `${field}: ${e.message}` : (e.message ?? "unknown error")
  })
  return { valid: false, errors }
}

/**
 * Pure: maps a TestResult and projectId to a TestRunRecord.
 */
export function buildTestRunRecord(result: TestResult, projectId: number): TestRunRecord {
  return {
    projectId,
    runId: result.run_id,
    branch: result.branch,
    commitSha: result.commit_sha,
    prNumber: result.pr_number,
    timestamp: result.timestamp,
    totalTests: result.summary.total,
    passedTests: result.summary.passed,
    failedTests: result.summary.failed,
    flakyTests: result.summary.flaky_detected,
  }
}

/**
 * I/O: upserts a test run into SQLite, keyed by run_id for idempotency.
 */
export function upsertTestRun(db: Database, projectId: number, result: TestResult): void {
  const record = buildTestRunRecord(result, projectId)
  db.prepare(`
    INSERT OR IGNORE INTO test_runs
      (project_id, run_id, branch, commit_sha, pr_number, timestamp, total_tests, passed_tests, failed_tests, flaky_tests)
    VALUES
      (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
  `).run(
    record.projectId,
    record.runId,
    record.branch,
    record.commitSha,
    record.prNumber,
    record.timestamp,
    record.totalTests,
    record.passedTests,
    record.failedTests,
    record.flakyTests,
  )
}

/**
 * Placeholder: ingest a test result artifact into the database.
 */
export async function ingestResult(_result: TestResult): Promise<void> {
  // TODO: orchestrate validation + upsert
}
