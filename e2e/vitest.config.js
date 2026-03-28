import { existsSync, readFileSync } from "node:fs"
import { join } from "node:path"
import { defineConfig } from "vitest/config"

// Load e2e/.env when present. CI env vars always take precedence.
const envFile = join(import.meta.dirname, ".env")
if (existsSync(envFile)) {
  for (const line of readFileSync(envFile, "utf8").split("\n")) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith("#")) continue
    const eq = trimmed.indexOf("=")
    if (eq === -1) continue
    const key = trimmed.slice(0, eq).trim()
    const raw = trimmed.slice(eq + 1).trim()
    // Strip optional surrounding quotes.
    const value = raw.replace(/^(['"])(.*)\1$/, "$2")
    if (key && !(key in process.env)) {
      process.env[key] = value
    }
  }
}

export default defineConfig({
  test: {
    testTimeout: 120_000,
    // Run test files sequentially — all e2e suites share the same GitHub
    // repo state and would race if run in parallel.
    fileParallelism: false,
  },
})
