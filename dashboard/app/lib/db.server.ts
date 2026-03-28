/**
 * SQLite database operations for the quarantine dashboard.
 *
 * Uses better-sqlite3 in WAL mode for concurrent read access during writes.
 */

import BetterSqlite3 from "better-sqlite3"
import type { Database } from "better-sqlite3"
import type { RepoConfig } from "./config.server.js"

export type { Database }

export interface Project {
  id: number
  owner: string
  repo: string
  lastSynced: string | null
  testRunCount: number
}

export interface ProjectSummary {
  owner: string
  repo: string
  testRunCount: number
  lastSynced: string | null
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
 * Opens (or creates) a SQLite database at dbPath, enables WAL mode, and runs migrations.
 */
export function initDb(dbPath: string): Database {
  const db = new BetterSqlite3(dbPath)
  db.pragma("journal_mode = WAL")

  db.exec(`
    CREATE TABLE IF NOT EXISTS projects (
      id INTEGER PRIMARY KEY,
      owner TEXT NOT NULL,
      repo TEXT NOT NULL,
      last_synced TEXT,
      last_etag TEXT,
      UNIQUE(owner, repo)
    );

    CREATE TABLE IF NOT EXISTS test_runs (
      id INTEGER PRIMARY KEY,
      project_id INTEGER NOT NULL REFERENCES projects(id),
      run_id TEXT NOT NULL UNIQUE,
      branch TEXT NOT NULL,
      commit_sha TEXT NOT NULL,
      pr_number INTEGER,
      timestamp TEXT NOT NULL,
      total_tests INTEGER NOT NULL,
      passed_tests INTEGER NOT NULL,
      failed_tests INTEGER NOT NULL,
      flaky_tests INTEGER NOT NULL
    );
  `)

  return db
}

/**
 * I/O: for each configured repo, returns its test run count and last sync
 * timestamp from SQLite. If a repo has never been ingested, returns
 * testRunCount: 0 and lastSynced: null.
 */
export function getProjects(db: Database, repos: RepoConfig[]): ProjectSummary[] {
  const stmt = db.prepare<[string, string], { last_synced: string | null; run_count: number }>(`
    SELECT p.last_synced, COUNT(tr.id) AS run_count
    FROM projects p
    LEFT JOIN test_runs tr ON tr.project_id = p.id
    WHERE p.owner = ? AND p.repo = ?
    GROUP BY p.id
  `)

  return repos.map((r) => {
    const row = stmt.get(r.owner, r.repo)
    return {
      owner: r.owner,
      repo: r.repo,
      testRunCount: row ? row.run_count : 0,
      lastSynced: row ? row.last_synced : null,
    }
  })
}

/**
 * Insert or ignore a project row, then return its id.
 */
export function upsertProject(db: Database, owner: string, repo: string): number {
  db.prepare("INSERT OR IGNORE INTO projects (owner, repo) VALUES (?, ?)").run(owner, repo)
  const row = db
    .prepare("SELECT id FROM projects WHERE owner = ? AND repo = ?")
    .get(owner, repo) as { id: number }
  return row.id
}

/**
 * I/O: returns the last_synced timestamp for a project, or null if never synced.
 */
export function getLastSynced(db: Database, projectId: number): string | null {
  const row = db.prepare("SELECT last_synced FROM projects WHERE id = ?").get(projectId) as
    | { last_synced: string | null }
    | undefined
  return row?.last_synced ?? null
}

/**
 * I/O: updates the last_synced timestamp for a project to the given ISO 8601 string.
 */
export function updateLastSynced(db: Database, projectId: number, timestamp: string): void {
  db.prepare("UPDATE projects SET last_synced = ? WHERE id = ?").run(timestamp, projectId)
}

/**
 * Get test runs for a project.
 */
export function getTestRuns(_projectId: number): TestRun[] {
  return []
}
