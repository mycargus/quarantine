export type AppCredentialInput = {
  clientId?: string
  privateKey?: string
}

export type AppCredentials = {
  clientId: string
  privateKey: string
}

const CREDENTIAL_ENV_NAMES: Record<keyof AppCredentialInput, string> = {
  clientId: "QUARANTINE_APP_CLIENT_ID",
  privateKey: "QUARANTINE_APP_PRIVATE_KEY",
}

export function validateAppCredentials(env: AppCredentialInput): AppCredentials {
  const missing: string[] = []
  const blank: string[] = []

  for (const key of Object.keys(CREDENTIAL_ENV_NAMES) as (keyof AppCredentialInput)[]) {
    const value = env[key]
    if (value === undefined) {
      missing.push(CREDENTIAL_ENV_NAMES[key])
    } else if (value.trim() === "") {
      blank.push(CREDENTIAL_ENV_NAMES[key])
    }
  }

  if (blank.length > 0) {
    throw new Error(blank.map((name) => `${name} is set but blank`).join("; "))
  }

  if (missing.length === 1) {
    throw new Error(`Missing required environment variable: ${missing[0]}`)
  }

  if (missing.length > 1) {
    throw new Error(`Missing required environment variables: ${missing.join(", ")}`)
  }

  return {
    clientId: env.clientId as string,
    privateKey: env.privateKey as string,
  }
}

export function shouldSyncInstallations(
  lastSyncedAt: Date | null,
  now: Date,
  intervalMs: number,
): boolean {
  if (lastSyncedAt === null) return true
  return now.getTime() - lastSyncedAt.getTime() > intervalMs
}

export interface SyncDeps {
  fetchFn: typeof fetch
  baseUrl: string
  jwtToken: string
  getInstallationToken: (installationId: number) => Promise<string>
  log: (msg: string) => void
}

interface GitHubInstallation {
  id: number
  account: { login: string }
  suspended_at: string | null
}

interface GitHubRepo {
  id: number
  owner: { login: string }
  name: string
}

interface GitHubReposResponse {
  total_count: number
  repositories: GitHubRepo[]
}

import type { Database as RawDatabase } from "better-sqlite3"
import { initDb } from "./db.server.js"
import { parseLinkHeader } from "./github-client.server.js"

export interface StartupDeps {
  dbPath: string
  baseUrl: string
  jwtToken: string
  getInstallationToken: (installationId: number) => Promise<string>
  fetchFn?: typeof fetch
  log?: (msg: string) => void
}

export async function startGitHubAppMode(deps: StartupDeps): Promise<{ raw: RawDatabase }> {
  const { raw } = initDb(deps.dbPath)

  await syncInstallations(raw, {
    fetchFn: deps.fetchFn ?? fetch,
    baseUrl: deps.baseUrl,
    jwtToken: deps.jwtToken,
    getInstallationToken: deps.getInstallationToken,
    log: deps.log ?? console.log,
  })

  return { raw }
}

export async function syncInstallations(raw: RawDatabase, deps: SyncDeps): Promise<void> {
  try {
    // 1. Fetch all installations (with pagination)
    const allInstallations: GitHubInstallation[] = []
    let installationsUrl: string | null = `${deps.baseUrl}/app/installations?per_page=100`

    while (installationsUrl) {
      const response = await deps.fetchFn(installationsUrl, {
        headers: {
          Authorization: `Bearer ${deps.jwtToken}`,
          Accept: "application/vnd.github+json",
        },
      })

      const pageInstallations = (await response.json()) as GitHubInstallation[]
      allInstallations.push(...pageInstallations)

      const linkHeader = response.headers.get("link")
      installationsUrl = parseLinkHeader(linkHeader)
    }

    deps.log(`Discovered ${allInstallations.length} installations`)

    // 2. For each installation, fetch repos
    const installationRepos = new Map<number, GitHubRepo[]>()
    for (const inst of allInstallations) {
      const token = await deps.getInstallationToken(inst.id)
      const repos: GitHubRepo[] = []
      let reposUrl: string | null = `${deps.baseUrl}/installation/repositories?per_page=100`

      while (reposUrl) {
        const repoResponse = await deps.fetchFn(reposUrl, {
          headers: {
            Authorization: `token ${token}`,
            Accept: "application/vnd.github+json",
          },
        })

        const repoData = (await repoResponse.json()) as GitHubReposResponse
        repos.push(...repoData.repositories)

        const linkHeader = repoResponse.headers.get("link")
        reposUrl = parseLinkHeader(linkHeader)
      }

      installationRepos.set(inst.id, repos)
    }

    // 3. Wrap all DB operations in a single transaction
    raw.exec("BEGIN")
    try {
      const seenInstallationIds = new Set<number>()

      // Upsert installations
      const upsertInstallation = raw.prepare(
        `INSERT INTO installations (id, account_login, suspended_at, removed_at)
         VALUES (?, ?, ?, NULL)
         ON CONFLICT(id) DO UPDATE SET
           account_login = excluded.account_login,
           suspended_at = excluded.suspended_at,
           removed_at = NULL`,
      )

      for (const inst of allInstallations) {
        upsertInstallation.run(inst.id, inst.account.login, inst.suspended_at)
        seenInstallationIds.add(inst.id)
      }

      // Upsert repos
      const upsertProject = raw.prepare(
        `INSERT INTO projects (owner, repo, installation_id, github_repo_id)
         VALUES (?, ?, ?, ?)
         ON CONFLICT(owner, repo) DO UPDATE SET
           installation_id = excluded.installation_id,
           github_repo_id = excluded.github_repo_id`,
      )

      for (const [installationId, repos] of installationRepos) {
        for (const repo of repos) {
          upsertProject.run(repo.owner.login, repo.name, installationId, repo.id)
        }
      }

      // Detect repos removed from active installations
      const selectLinkedProjects = raw.prepare(
        "SELECT id, owner, repo FROM projects WHERE installation_id = ?",
      )
      const clearProjectInstallation = raw.prepare(
        "UPDATE projects SET installation_id = NULL WHERE id = ?",
      )

      for (const [installationId, repos] of installationRepos) {
        const repoNames = new Set(repos.map((r) => `${r.owner.login}/${r.name}`))
        const linkedProjects = selectLinkedProjects.all(installationId) as Array<{
          id: number
          owner: string
          repo: string
        }>

        for (const proj of linkedProjects) {
          if (!repoNames.has(`${proj.owner}/${proj.repo}`)) {
            clearProjectInstallation.run(proj.id)
          }
        }
      }

      // Mark removed installations
      const existingInstallations = raw
        .prepare("SELECT id FROM installations WHERE removed_at IS NULL")
        .all() as Array<{ id: number }>

      const now = new Date().toISOString()
      const markRemoved = raw.prepare("UPDATE installations SET removed_at = ? WHERE id = ?")
      const clearInstallationId = raw.prepare(
        "UPDATE projects SET installation_id = NULL WHERE installation_id = ?",
      )

      for (const existing of existingInstallations) {
        if (!seenInstallationIds.has(existing.id)) {
          markRemoved.run(now, existing.id)
          clearInstallationId.run(existing.id)
        }
      }

      raw.exec("COMMIT")
    } catch (txErr) {
      raw.exec("ROLLBACK")
      throw txErr
    }

    deps.log("Installation sync complete")
  } catch (err) {
    deps.log(`Installation sync error: ${err instanceof Error ? err.message : String(err)}`)
  }
}

export interface DiscoveryLoopDeps {
  syncFn: () => Promise<void>
  intervalMs: number
  shutdownSignals?: string[]
  log?: (msg: string) => void
  terminate?: (code: number) => void
}

export interface DiscoveryLoopResult {
  cleanup: () => void
}

export function startDiscoveryLoop(deps: DiscoveryLoopDeps): DiscoveryLoopResult {
  const log = deps.log ?? console.log
  const terminate = deps.terminate ?? process.exit
  const signals = deps.shutdownSignals ?? ["SIGTERM", "SIGINT"]

  const interval = setInterval(async () => {
    log("Discovery loop: starting sync")
    await deps.syncFn()
    log("Discovery loop: sync complete")
  }, deps.intervalMs)

  const onSignal = () => {
    log("Discovery loop: received shutdown signal, clearing interval")
    clearInterval(interval)
    cleanup()
    terminate(0)
  }

  for (const sig of signals) {
    process.on(sig, onSignal)
  }

  function cleanup() {
    clearInterval(interval)
    for (const sig of signals) {
      process.removeListener(sig, onSignal)
    }
  }

  return { cleanup }
}

export async function startupSyncWithTimeout(
  syncFn: () => Promise<void>,
  timeoutMs: number,
  log: (msg: string) => void,
  terminate: (code: number) => void,
): Promise<void> {
  let timedOut = false
  const timer = setTimeout(() => {
    timedOut = true
    log(`Startup sync timed out after ${timeoutMs}ms`)
    terminate(1)
  }, timeoutMs)

  try {
    await syncFn()
  } finally {
    clearTimeout(timer)
  }

  if (timedOut) {
    throw new Error("Startup sync timed out")
  }
}
