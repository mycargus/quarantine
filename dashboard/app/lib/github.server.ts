/**
 * GitHub Artifacts polling for the quarantine dashboard.
 *
 * Polls the GitHub Artifacts API for each configured repository, downloads
 * new quarantine result artifacts, and passes them to the ingestion pipeline.
 * Uses ETag-based conditional requests to avoid re-downloading unchanged data.
 *
 * Implements a circuit breaker: 3 consecutive failures for a repo triggers
 * a 30-minute pause.
 */

// TODO: M6 — Implement artifact polling, ETag handling, circuit breaker.

export interface ArtifactListResponse {
  total_count: number
  artifacts: Artifact[]
}

export interface Artifact {
  id: number
  name: string
  size_in_bytes: number
  archive_download_url: string
  created_at: string
  expires_at: string
}

export interface PollState {
  lastEtag: string | null
  consecutiveFailures: number
  pausedUntil: string | null
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
  // TODO: M6 — List artifacts with ETag, filter by name prefix, download new ones.
  return []
}
