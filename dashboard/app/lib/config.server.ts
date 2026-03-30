import { readFileSync } from "node:fs"
import { load } from "js-yaml"
import { array, object, parseSafe, string } from "remix/data-schema"

const RepoConfigSchema = object({ owner: string() }, { unknownKeys: "passthrough" })

const DashboardConfigSchema = object(
  {
    source: string(),
    repos: array(RepoConfigSchema),
  },
  { unknownKeys: "passthrough" },
)

export type RepoConfig = {
  owner: string
  repo: string
}

export type DashboardConfig = {
  source: "manual"
  repos: RepoConfig[]
  poll_interval: number
}

export function parseConfig(yaml: string): DashboardConfig {
  const raw = load(yaml)
  const result = parseSafe(DashboardConfigSchema, raw)

  if (!result.success) {
    const firstIssue = result.issues[0]
    const field = firstIssue?.path?.[0] ?? "unknown"
    throw new Error(`Invalid config: missing or invalid field "${String(field)}"`)
  }

  const { source, repos } = result.value
  const poll_interval = (raw as Record<string, unknown>)?.poll_interval

  if (!source) {
    throw new Error('Invalid config: missing or invalid field "source"')
  }

  return {
    source: source as "manual",
    repos: repos as RepoConfig[],
    poll_interval: typeof poll_interval === "number" ? poll_interval : 300,
  }
}

export function loadConfig(filePath: string): DashboardConfig {
  const yaml = readFileSync(filePath, "utf8")
  return parseConfig(yaml)
}
