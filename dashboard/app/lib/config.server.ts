import { readFileSync } from "node:fs"
import { load } from "js-yaml"

export interface RepoConfig {
  owner: string
  repo: string
}

export interface DashboardConfig {
  source: "manual"
  repos: RepoConfig[]
  poll_interval: number
}

export function parseConfig(yaml: string): DashboardConfig {
  const raw = load(yaml) as Record<string, unknown>

  if (!raw.source) {
    throw new Error("Missing required field: source")
  }

  if (!raw.repos) {
    throw new Error("Missing required field: repos")
  }

  return {
    source: raw.source as "manual",
    repos: raw.repos as RepoConfig[],
    poll_interval: typeof raw.poll_interval === "number" ? raw.poll_interval : 300,
  }
}

export function loadConfig(filePath: string): DashboardConfig {
  const yaml = readFileSync(filePath, "utf8")
  return parseConfig(yaml)
}
