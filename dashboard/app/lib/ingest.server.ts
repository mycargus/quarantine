/**
 * Artifact JSON ingestion for the quarantine dashboard.
 *
 * Parses quarantine result JSON from GitHub Artifacts, validates against
 * the test-result schema, and upserts into SQLite. Keyed by run_id for
 * idempotency.
 */

// TODO: M6 — Implement JSON parsing, schema validation (ajv), SQLite upsert.

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

/**
 * Placeholder: ingest a test result artifact into the database.
 */
export async function ingestResult(_result: TestResult): Promise<void> {
  // TODO: M6 — Validate against schema, upsert test run and results into SQLite.
}
