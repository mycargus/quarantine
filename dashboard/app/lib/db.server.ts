/**
 * SQLite database operations for the quarantine dashboard.
 *
 * Uses remix/data-table with better-sqlite3 in WAL mode.
 * Schema migrations run on startup via raw SQL before the data-table wrapper is applied.
 */

import BetterSqlite3, { type Database as RawDatabase } from "better-sqlite3"
import { column as c, createDatabase, table } from "remix/data-table"
import { createSqliteDatabaseAdapter } from "remix/data-table-sqlite"
import type { RepoConfig } from "./config.server.ts"

export const projects = table({
  name: "projects",
  columns: {
    id: c.integer().primaryKey().autoIncrement(),
    owner: c.text().notNull(),
    repo: c.text().notNull(),
    last_synced: c.text(),
    last_etag: c.text(),
    last_pulled_at: c.text(),
  },
})

export const testRuns = table({
  name: "test_runs",
  columns: {
    id: c.integer().primaryKey().autoIncrement(),
    project_id: c.integer().notNull(),
    run_id: c.text().notNull(),
    branch: c.text().notNull(),
    commit_sha: c.text().notNull(),
    pr_number: c.integer(),
    timestamp: c.text().notNull(),
    total_tests: c.integer().notNull(),
    passed_tests: c.integer().notNull(),
    failed_tests: c.integer().notNull(),
    flaky_tests: c.integer().notNull(),
  },
})

export type Database = ReturnType<typeof createDatabase>

export type DbHandle = {
  db: Database
  raw: RawDatabase
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

function runMigrations(raw: RawDatabase): void {
  raw.pragma("journal_mode = WAL")

  raw.exec(`
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

  try {
    raw.exec("ALTER TABLE projects ADD COLUMN last_pulled_at TEXT")
  } catch {
    // Column already exists — ignore
  }
}

/**
 * Opens (or creates) a SQLite database at dbPath, runs schema migrations,
 * and returns a DbHandle with both the data-table Database and raw better-sqlite3 instance.
 * The raw instance is provided for tests, migrations, and utilities (pragma, close, etc.).
 */
export function initDb(dbPath: string): DbHandle {
  const raw = new BetterSqlite3(dbPath)
  runMigrations(raw)
  return { raw, db: createDatabase(createSqliteDatabaseAdapter(raw)) }
}

/**
 * For each configured repo, returns its test run count and last sync
 * timestamp from SQLite. If a repo has never been ingested, returns
 * testRunCount: 0 and lastSynced: null.
 */
export async function getProjects(db: Database, repos: RepoConfig[]): Promise<ProjectSummary[]> {
  return Promise.all(
    repos.map(async (r) => {
      const row = await db.query(projects).where({ owner: r.owner, repo: r.repo }).first()

      const testRunCount = row ? await db.count(testRuns, { where: { project_id: row.id } }) : 0

      return {
        owner: r.owner,
        repo: r.repo,
        testRunCount,
        lastSynced: row?.last_synced ?? null,
      }
    }),
  )
}

/**
 * Insert or ignore a project row, then return its id.
 */
export async function upsertProject(db: Database, owner: string, repo: string): Promise<number> {
  const existing = await db.query(projects).where({ owner, repo }).first()
  if (existing) return existing.id

  const result = await db.create(projects, { owner, repo }, { returnRow: true })
  return (result as { id: number }).id
}

/**
 * Returns the last_synced timestamp for a project, or null if never synced.
 */
export async function getLastSynced(db: Database, projectId: number): Promise<string | null> {
  const row = await db.find(projects, projectId)
  return row?.last_synced ?? null
}

/**
 * Updates the last_synced timestamp for a project to the given ISO 8601 string.
 * No-op when the projectId does not exist.
 */
export async function updateLastSynced(
  db: Database,
  projectId: number,
  timestamp: string,
): Promise<void> {
  await db.updateMany(projects, { last_synced: timestamp }, { where: { id: projectId } })
}

/**
 * Returns the last_pulled_at timestamp for a project, or null if never pulled.
 */
export async function getLastPulledAt(db: Database, projectId: number): Promise<string | null> {
  const row = await db.find(projects, projectId)
  return row?.last_pulled_at ?? null
}

/**
 * Updates the last_pulled_at timestamp for a project.
 * No-op when the projectId does not exist.
 */
export async function updateLastPulledAt(
  db: Database,
  projectId: number,
  timestamp: string,
): Promise<void> {
  await db.updateMany(projects, { last_pulled_at: timestamp }, { where: { id: projectId } })
}

/**
 * Get test runs for a project.
 */
export async function getTestRuns(_projectId: number): Promise<TestRun[]> {
  return []
}
