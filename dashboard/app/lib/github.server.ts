/**
 * GitHub Artifacts polling for the quarantine dashboard.
 *
 * Polls the GitHub Artifacts API for each configured repository, downloads
 * new quarantine result artifacts, and passes them to the ingestion pipeline.
 * Uses ETag-based conditional requests to avoid re-downloading unchanged data.
 */

import AdmZip from "adm-zip"
import type { Artifact, ListArtifactsResult } from "./ingest.server.js"

export type { Artifact, ListArtifactsResult }

export interface ArtifactListResponse {
  total_count: number
  artifacts: Artifact[]
}

export interface PollState {
  lastEtag: string | null
  consecutiveFailures: number
  pausedUntil: string | null
}

/**
 * I/O: lists artifacts for a repo with conditional ETag request.
 * fetchFn is injected for testing.
 */
export async function listArtifacts(
  owner: string,
  repo: string,
  token: string,
  etag: string | null,
  fetchFn: typeof fetch = fetch,
): Promise<ListArtifactsResult> {
  const url = `https://api.github.com/repos/${owner}/${repo}/actions/artifacts?per_page=100`
  const headers: Record<string, string> = {
    Authorization: `Bearer ${token}`,
    Accept: "application/vnd.github+json",
  }
  if (etag !== null) {
    headers["If-None-Match"] = etag
  }

  const response = await fetchFn(url, { headers })

  if (response.status === 304) {
    return { artifacts: [], etag: null, notModified: true }
  }

  if (!response.ok) {
    throw new Error(`GitHub API error: ${response.status}`)
  }

  const data = (await response.json()) as ArtifactListResponse
  const responseEtag = response.headers.get("etag")

  return {
    artifacts: data.artifacts,
    etag: responseEtag,
    notModified: false,
  }
}

/**
 * I/O: downloads an artifact zip and extracts the first JSON file as a string.
 * fetchFn is injected for testing.
 */
export async function downloadAndExtractJson(
  downloadUrl: string,
  token: string,
  fetchFn: typeof fetch = fetch,
): Promise<string> {
  const response = await fetchFn(downloadUrl, {
    headers: { Authorization: `Bearer ${token}` },
  })
  const buffer = Buffer.from(await response.arrayBuffer())
  const zip = new AdmZip(buffer)
  const entries = zip.getEntries()
  if (entries.length === 0) {
    throw new Error("Artifact zip contains no entries")
  }
  return entries[0].getData().toString("utf8")
}

/**
 * Placeholder: poll artifacts for a repository.
 */
export async function pollArtifacts(
  _owner: string,
  _repo: string,
  _token: string,
  _state: PollState,
): Promise<Artifact[]> {
  return []
}
