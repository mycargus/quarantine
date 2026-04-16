import { randomUUID } from "node:crypto"
import { unlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { createSession } from "@remix-run/session"
import { createCookie } from "remix/cookie"
import { createApp } from "../app/app.js"
import { initDb } from "../app/lib/db.server.js"

type RepoEntry = { owner: string; repo: string }

const TEST_SESSION_SECRET = "test-secret"

function writeTempConfig(repos: RepoEntry[]): string {
  const configPath = join(tmpdir(), `dashboard-iface-config-${randomUUID()}.yml`)
  if (repos.length === 0) {
    writeFileSync(configPath, "source: manual\nrepos: []", "utf8")
  } else {
    const repoList = repos.map((r) => `  - owner: ${r.owner}\n    repo: ${r.repo}`).join("\n")
    writeFileSync(configPath, `source: manual\nrepos:\n${repoList}`, "utf8")
  }
  return configPath
}

function createTempDbPath(): string {
  return join(tmpdir(), `dashboard-iface-db-${randomUUID()}.db`)
}

export interface TestApp {
  router: ReturnType<typeof createApp>
  dbPath: string
  configPath: string
  cleanup: () => void
  sessionCookie: () => Promise<string>
}

/**
 * Builds a signed session cookie header value for use in authenticated test
 * requests. Uses the same cookie name and secret as the test app.
 */
async function buildSessionCookie(): Promise<string> {
  const cookie = createCookie("__session", {
    httpOnly: true,
    sameSite: "Lax" as const,
    secrets: [TEST_SESSION_SECRET],
  })
  const session = createSession()
  session.set("userId" as never, "test-user" as never)
  const serializedData = JSON.stringify({ i: session.id, d: session.data })
  return cookie.serialize(serializedData)
}

export function createTestApp(opts: { repos?: RepoEntry[] } = {}): TestApp {
  const configPath = writeTempConfig(opts.repos ?? [])
  const dbPath = createTempDbPath()
  const router = createApp({
    configPath,
    dbPath,
    token: "",
    sessionSecret: TEST_SESSION_SECRET,
  })
  return {
    router,
    dbPath,
    configPath,
    sessionCookie: buildSessionCookie,
    cleanup() {
      try {
        unlinkSync(configPath)
      } catch {
        // ignore
      }
      try {
        unlinkSync(dbPath)
      } catch {
        // ignore
      }
    },
  }
}

export interface TestSeed {
  owner: string
  repo: string
  tests?: Array<{ testId: string; name: string; quarantinedAt: string; issueUrl?: string }>
}

/**
 * Seeds a SQLite database with projects and quarantined test rows for Interface
 * tests that need pre-populated data. Returns the DB handle for additional
 * setup if needed; caller must call raw.close() when done seeding.
 */
export function seedTestDb(dbPath: string, seeds: TestSeed[]): void {
  const { raw } = initDb(dbPath)
  for (const { owner, repo, tests = [] } of seeds) {
    const projectId = raw
      .prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)")
      .run(owner, repo).lastInsertRowid as number
    for (const test of tests) {
      raw
        .prepare(
          `INSERT INTO quarantined_tests
             (project_id, test_id, name, quarantined_at, issue_url)
           VALUES (?, ?, ?, ?, ?)`,
        )
        .run(projectId, test.testId, test.name, test.quarantinedAt, test.issueUrl ?? null)
    }
  }
  raw.close()
}
