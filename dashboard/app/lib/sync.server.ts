/**
 * On-demand sync orchestrator for the quarantine dashboard.
 *
 * Wires together shouldPull, listArtifacts, downloadAndExtractJson,
 * ingestArtifact, and the DB update functions. Never throws — on any
 * error it logs a warning and returns so the caller can still render.
 */

import { type DbHandle, getLastPulledAt, updateLastPulledAt, upsertProject } from "./db.server.js"
import { DEBOUNCE_MS, shouldPull } from "./debounce.server.js"
import { downloadAndExtractJson, listArtifacts } from "./github.server.js"
import {
  filterArtifactsByPrefix,
  ingestArtifact,
  sortArtifactsChronologically,
} from "./ingest.server.js"

/**
 * Orchestrator: syncs a single repo from GitHub Artifacts into SQLite.
 * - Returns early (without fetching) when debounce window has not elapsed.
 * - Never throws; errors are passed to warn.
 */
export async function syncRepo(
  owner: string,
  repo: string,
  token: string,
  handle: DbHandle,
  now: Date,
  fetchFn: typeof fetch = fetch,
  warn: (msg: string) => void = console.warn,
): Promise<void> {
  try {
    const projectId = await upsertProject(handle.db, owner, repo)
    const lastPulledAt = await getLastPulledAt(handle.db, projectId)

    if (!shouldPull(lastPulledAt, now, DEBOUNCE_MS)) {
      return
    }

    const { artifacts } = await listArtifacts(owner, repo, token, null, fetchFn)
    const filtered = filterArtifactsByPrefix(artifacts, "quarantine-results")
    const sorted = sortArtifactsChronologically(filtered)

    for (const artifact of sorted) {
      const jsonString = await downloadAndExtractJson(artifact.archive_download_url, token, fetchFn)
      await ingestArtifact(handle.db, owner, repo, artifact.name, jsonString, projectId, warn)
    }

    await updateLastPulledAt(handle.db, projectId, now.toISOString())
  } catch (e) {
    warn(
      `[sync] WARNING: sync failed for ${owner}/${repo}: ${e instanceof Error ? e.message : String(e)}`,
    )
  }
}
