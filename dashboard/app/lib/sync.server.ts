/**
 * On-demand sync orchestrator for the quarantine dashboard.
 *
 * Wires together shouldPull, listArtifacts, downloadAndExtractJson,
 * ingestArtifact, and the DB update functions. Never throws — on any
 * error it logs a warning and returns so the caller can still render.
 */

import {
  type DbHandle,
  getLastPulledAt,
  updateLastPulledAt,
  upsertProject,
  upsertSuiteState,
} from "./db.server.js"
import { DEBOUNCE_MS, shouldPull } from "./debounce.server.js"
import { downloadAndExtractJson, listArtifacts } from "./github.server.js"
import {
  ARTIFACT_PREFIX,
  filterArtifactsByPrefix,
  ingestArtifact,
  sortArtifactsChronologically,
} from "./ingest.server.js"
import { listStateSuites, readSuiteState } from "./state-sync.server.js"

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
    const filtered = filterArtifactsByPrefix(artifacts, ARTIFACT_PREFIX)
    const sorted = sortArtifactsChronologically(filtered)

    for (const artifact of sorted) {
      const jsonString = await downloadAndExtractJson(artifact.archive_download_url, token, fetchFn)
      await ingestArtifact(
        handle.db,
        handle.raw,
        owner,
        repo,
        artifact.name,
        jsonString,
        projectId,
        warn,
      )
    }

    const suites = await listStateSuites(owner, repo, token, "quarantine/state", fetchFn)
    for (const suite of suites) {
      const state = await readSuiteState(owner, repo, token, "quarantine/state", suite, fetchFn)
      if (state !== null) {
        upsertSuiteState(
          handle.raw,
          projectId,
          suite,
          Object.keys(state.tests).length,
          JSON.stringify(state),
          now.toISOString(),
        )
      }
    }

    await updateLastPulledAt(handle.db, projectId, now.toISOString())
  } catch (e) {
    warn(
      `[sync] WARNING: sync failed for ${owner}/${repo}: ${e instanceof Error ? e.message : String(e)}`,
    )
  }
}
