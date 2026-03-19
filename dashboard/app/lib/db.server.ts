/**
 * SQLite database operations for the quarantine dashboard.
 *
 * Uses better-sqlite3 in WAL mode for concurrent read access during writes.
 * Schema and migrations will be implemented in M6.
 */

// TODO: M6 — Initialize better-sqlite3, create schema, implement queries.

export interface Project {
  id: number
  owner: string
  repo: string
  lastSynced: string | null
  testRunCount: number
}

export interface TestRun {
  id: number
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

/**
 * Placeholder: get all configured projects with run counts.
 */
export function getProjects(): Project[] {
  // TODO: M6 — Query SQLite for projects with aggregated test run counts.
  return []
}

/**
 * Placeholder: get test runs for a project.
 */
export function getTestRuns(_projectId: number): TestRun[] {
  // TODO: M6 — Query SQLite for test runs ordered by timestamp desc.
  return []
}
