/**
 * Artifact JSON ingestion for the quarantine dashboard.
 *
 * Parses quarantine result JSON from GitHub Artifacts, validates against
 * the test-result schema, and upserts into SQLite. Keyed by run_id for
 * idempotency.
 */

import Ajv2020 from "ajv/dist/2020"
import addFormats from "ajv-formats"
import type { Database as RawDatabase } from "better-sqlite3"
import schema from "../../../schemas/test-result.schema.json" with { type: "json" }
import type { Database } from "./db.server.js"
import { testRuns } from "./db.server.js"

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
  suite_name: string
  run_id: string
  repo: string
  branch: string
  commit_sha: string
  pr_number: number | null
  timestamp: string
  cli_version: string
  config: {
    retry_count: number
  }
  summary: {
    total: number
    passed: number
    failed: number
    skipped: number
    quarantined: number
    flaky_detected: number
    unresolved?: number
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
  error?: string
  rerun_exit_code?: number
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
  unresolvedTests: number
}

/**
 * The required prefix for GitHub Artifacts that contain quarantine results.
 * CI workflows must name artifacts `quarantine-results-{suite-name}-{run_id}`.
 * See contracts.md §16 (Artifact naming convention).
 */
export const ARTIFACT_PREFIX = "quarantine-results"

const ajv = new Ajv2020()
addFormats(ajv)
const validate = ajv.compile(schema)

/**
 * Pure: filters artifacts to only those created after lastSynced.
 * If lastSynced is null, returns all artifacts (first sync).
 */
export function filterArtifactsSince(artifacts: Artifact[], lastSynced: string | null): Artifact[] {
  if (lastSynced === null) return artifacts
  return artifacts.filter((a) => a.created_at > lastSynced)
}

/**
 * Pure: sorts artifacts by created_at in ascending order (oldest first).
 */
export function sortArtifactsChronologically(artifacts: Artifact[]): Artifact[] {
  return [...artifacts].sort((a, b) =>
    a.created_at < b.created_at ? -1 : a.created_at > b.created_at ? 1 : 0,
  )
}

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
    unresolvedTests: result.summary.unresolved ?? 0,
  }
}

/**
 * Pure: maps an original_status value to the last_run_status enum.
 */
export function mapOriginalStatus(originalStatus: string | null): "failing" | "passing" | null {
  if (originalStatus === "failed") return "failing"
  if (originalStatus === "passed") return "passing"
  return null
}

/**
 * Pure: builds a GitHub issue URL from owner, repo, and issue number.
 */
export function buildIssueUrl(owner: string, repo: string, issueNumber: number): string {
  return `https://github.com/${owner}/${repo}/issues/${issueNumber}`
}

/**
 * I/O: upserts a test run into SQLite, keyed by run_id for idempotency.
 * Returns true if a new row was inserted, false if the run_id already existed.
 */
export async function upsertTestRun(
  db: Database,
  projectId: number,
  result: TestResult,
): Promise<boolean> {
  const record = buildTestRunRecord(result, projectId)
  const existing = await db.query(testRuns).where({ run_id: record.runId }).first()
  if (existing) return false

  await db.create(testRuns, {
    project_id: record.projectId,
    run_id: record.runId,
    branch: record.branch,
    commit_sha: record.commitSha,
    pr_number: record.prNumber ?? undefined,
    timestamp: record.timestamp,
    total_tests: record.totalTests,
    passed_tests: record.passedTests,
    failed_tests: record.failedTests,
    flaky_tests: record.flakyTests,
    unresolved_tests: record.unresolvedTests,
  })
  return true
}

/**
 * I/O: upserts a quarantined test entry into SQLite using INSERT OR IGNORE + UPDATE.
 * - INSERT OR IGNORE preserves the original quarantined_at on conflict.
 * - UPDATE always refreshes last_run_status to the latest value.
 */
export function upsertQuarantinedTest(
  raw: RawDatabase,
  projectId: number,
  owner: string,
  repo: string,
  entry: TestEntry,
  timestamp: string,
): void {
  const lastRunStatus = mapOriginalStatus(entry.original_status)
  const issueUrl =
    entry.issue_number != null ? buildIssueUrl(owner, repo, entry.issue_number) : null

  raw
    .prepare(
      `INSERT OR IGNORE INTO quarantined_tests
         (project_id, test_id, name, issue_number, issue_url, quarantined_at, last_run_status)
       VALUES (?, ?, ?, ?, ?, ?, ?)`,
    )
    .run(
      projectId,
      entry.test_id,
      entry.name,
      entry.issue_number,
      issueUrl,
      timestamp,
      lastRunStatus,
    )

  raw
    .prepare(
      `UPDATE quarantined_tests
       SET last_run_status = ?
       WHERE project_id = ? AND test_id = ?`,
    )
    .run(lastRunStatus, projectId, entry.test_id)
}

/**
 * I/O: increments flaky_count by 1, updates last_flaky_at to timestamp, and sets
 * last_run_status to 'passing' for the given test. A flaky test always ultimately
 * passed on retry, so last_run_status is hardcoded to 'passing'.
 */
export function incrementFlakyCount(
  raw: RawDatabase,
  projectId: number,
  testId: string,
  timestamp: string,
): void {
  raw
    .prepare(
      `UPDATE quarantined_tests
       SET flaky_count = flaky_count + 1, last_flaky_at = ?, last_run_status = 'passing'
       WHERE project_id = ? AND test_id = ?`,
    )
    .run(timestamp, projectId, testId)
}

/**
 * I/O Orchestrator: parses and validates a JSON string, then upserts into SQLite.
 * If parsing or validation fails, logs a warning and returns 'skipped'.
 * Never throws — callers can process remaining artifacts safely.
 *
 * @param warn - injectable logger, defaults to console.warn (inject in tests)
 */
export async function ingestArtifact(
  db: Database,
  raw: RawDatabase,
  owner: string,
  repo: string,
  artifactName: string,
  jsonString: string,
  projectId: number,
  warn: (msg: string) => void = console.warn,
): Promise<"ingested" | "skipped"> {
  let parsed: unknown
  try {
    parsed = JSON.parse(jsonString)
  } catch {
    warn(
      `[ingest] WARNING: skipping artifact ${artifactName} for ${owner}/${repo}: validation failed`,
    )
    return "skipped"
  }

  const { valid } = validateTestResult(parsed)
  if (!valid) {
    warn(
      `[ingest] WARNING: skipping artifact ${artifactName} for ${owner}/${repo}: validation failed`,
    )
    return "skipped"
  }

  try {
    const result = parsed as TestResult
    const isNew = await upsertTestRun(db, projectId, result)
    if (isNew) {
      for (const entry of result.tests) {
        if (entry.status === "quarantined") {
          upsertQuarantinedTest(raw, projectId, owner, repo, entry, result.timestamp)
        } else if (entry.status === "flaky") {
          // A flaky entry means the test was just detected this run. Ensure the
          // quarantined_tests row exists before incrementing — for Jest/Vitest the
          // test was excluded from execution on all prior runs and never appeared
          // as "quarantined" in any result, so the row may not exist yet.
          upsertQuarantinedTest(raw, projectId, owner, repo, entry, result.timestamp)
          incrementFlakyCount(raw, projectId, entry.test_id, result.timestamp)
        }
      }
    }
  } catch {
    warn(
      `[ingest] WARNING: skipping artifact ${artifactName} for ${owner}/${repo}: validation failed`,
    )
    return "skipped"
  }
  return "ingested"
}
