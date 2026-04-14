/**
 * GitHub Contents API integration for reading quarantine state files
 * from the quarantine/state branch.
 *
 * Implements the I/O shell for listing suite directories and reading
 * per-suite state.json files. Pure logic (base64 decode + JSON parse)
 * is extracted into a separate function.
 */

export interface QuarantineStateEntry {
  test_id: string
  file_path: string
  classname: string
  name: string
  suite: string
  first_flaky_at: string
  last_failure_at: string
  flaky_count: number
  quarantined_at: string
  quarantined_by: string
  issue_number?: number
  issue_url?: string
}

export interface QuarantineState {
  version: 1
  updated_at: string
  tests: Record<string, QuarantineStateEntry>
}

interface ContentsEntry {
  name: string
  path: string
  type: "dir" | "file"
  sha: string
}

interface ContentsFileResponse {
  content: string
  sha: string
}

/**
 * Pure: decodes a base64-encoded string and parses it as JSON.
 */
function decodeBase64Json(encoded: string): unknown {
  return JSON.parse(Buffer.from(encoded, "base64").toString("utf8"))
}

/**
 * I/O: lists suite names from .quarantine/ directory on the state branch.
 * Returns [] when directory doesn't exist (404) — not an error.
 */
export async function listStateSuites(
  owner: string,
  repo: string,
  token: string,
  branch: string,
  fetchFn: typeof fetch = fetch,
): Promise<string[]> {
  const url = `https://api.github.com/repos/${owner}/${repo}/contents/.quarantine?ref=${encodeURIComponent(branch)}`
  const response = await fetchFn(url, {
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/vnd.github+json",
    },
  })

  if (response.status === 404) {
    return []
  }

  // Non-404 errors (e.g., 401, 500) propagate to the caller.
  // syncRepo wraps all calls in try/catch — never throws to the user.
  if (!response.ok) {
    throw new Error(`GitHub API error listing .quarantine/: ${response.status}`)
  }

  const entries = (await response.json()) as ContentsEntry[]
  return entries.filter((e) => e.type === "dir").map((e) => e.name)
}

/**
 * I/O: reads and parses .quarantine/{suite}/state.json from the state branch.
 * Returns null when the file doesn't exist (404).
 */
export async function readSuiteState(
  owner: string,
  repo: string,
  token: string,
  branch: string,
  suiteName: string,
  fetchFn: typeof fetch = fetch,
): Promise<QuarantineState | null> {
  const url = `https://api.github.com/repos/${owner}/${repo}/contents/.quarantine/${encodeURIComponent(suiteName)}/state.json?ref=${encodeURIComponent(branch)}`
  const response = await fetchFn(url, {
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/vnd.github+json",
    },
  })

  if (response.status === 404) {
    return null
  }

  const file = (await response.json()) as ContentsFileResponse
  return decodeBase64Json(file.content) as QuarantineState
}
