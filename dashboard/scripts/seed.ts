/**
 * On-demand seed: pulls quarantine result artifacts from all repos configured
 * in dashboard.yml into the local SQLite database.
 *
 * Run from the dashboard/ directory:
 *   pnpm seed
 *   QUARANTINE_GITHUB_TOKEN=ghp_xxx pnpm seed
 *
 * Bypasses the sync debounce — always fetches regardless of when the last
 * pull happened. Safe to run multiple times; ingestion is idempotent by run_id.
 */

import { loadConfig } from "../app/lib/config.server.js"
import { initDb, updateLastPulledAt, upsertProject } from "../app/lib/db.server.js"
import { downloadAndExtractJson, listArtifacts } from "../app/lib/github.server.js"
import {
  ARTIFACT_PREFIX,
  filterArtifactsByPrefix,
  ingestArtifact,
  sortArtifactsChronologically,
} from "../app/lib/ingest.server.js"

const configPath = process.env.DASHBOARD_CONFIG ?? "./dashboard.yml"
const dbPath = process.env.DATABASE_URL ?? "./quarantine.db"
const token = process.env.QUARANTINE_GITHUB_TOKEN ?? process.env.GITHUB_TOKEN ?? ""

if (!token) {
  console.error("error: set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN")
  process.exit(1)
}

const config = loadConfig(configPath)
const handle = initDb(dbPath)

let totalIngested = 0
let totalSkipped = 0

for (const { owner, repo } of config.repos) {
  console.log(`\n${owner}/${repo}`)

  const projectId = await upsertProject(handle.db, owner, repo)
  const { artifacts } = await listArtifacts(owner, repo, token, null)
  const filtered = filterArtifactsByPrefix(artifacts, ARTIFACT_PREFIX)
  const sorted = sortArtifactsChronologically(filtered)

  console.log(`  ${sorted.length} artifact(s) found`)

  for (const artifact of sorted) {
    const jsonString = await downloadAndExtractJson(artifact.archive_download_url, token)
    const outcome = await ingestArtifact(
      handle.db,
      handle.raw,
      owner,
      repo,
      artifact.name,
      jsonString,
      projectId,
    )
    if (outcome === "ingested") {
      console.log(`  + ${artifact.name}`)
      totalIngested++
    } else {
      console.log(`  = ${artifact.name} (already ingested or skipped)`)
      totalSkipped++
    }
  }

  await updateLastPulledAt(handle.db, projectId, new Date().toISOString())
}

handle.raw.close()
console.log(`\n${totalIngested} ingested, ${totalSkipped} skipped`)
