import { unlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway"
import { home } from "./home.js"

async function bodyText(response: Response): Promise<string> {
  return new Response(response.body).text()
}

describe("home() — missing config", async (assert) => {
  const original = process.env.DASHBOARD_CONFIG
  process.env.DASHBOARD_CONFIG = "/nonexistent/dashboard.yml"

  try {
    const response = await home()
    const html = await bodyText(response)

    assert({
      given: "a config path that does not exist",
      should: "return HTTP 500",
      actual: response.status,
      expected: 500,
    })

    assert({
      given: "a config path that does not exist",
      should: "include 'Configuration Error' in the body",
      actual: html.includes("Configuration Error"),
      expected: true,
    })

    assert({
      given: "a config path that does not exist",
      should: "include the missing file path in the body",
      actual: html.includes("/nonexistent/dashboard.yml"),
      expected: true,
    })
  } finally {
    if (original === undefined) {
      delete process.env.DASHBOARD_CONFIG
    } else {
      process.env.DASHBOARD_CONFIG = original
    }
  }
})

describe("home() — invalid config", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-test-bad-${Date.now()}.yml`)
  writeFileSync(configPath, "not: valid\nconfig: true", "utf8")
  const original = process.env.DASHBOARD_CONFIG
  process.env.DASHBOARD_CONFIG = configPath

  try {
    const response = await home()
    const html = await bodyText(response)

    assert({
      given: "a config file with invalid content (missing source/repos)",
      should: "return HTTP 500",
      actual: response.status,
      expected: 500,
    })

    assert({
      given: "a config file with invalid content",
      should: "include 'Configuration Error' in the body",
      actual: html.includes("Configuration Error"),
      expected: true,
    })
  } finally {
    if (original === undefined) {
      delete process.env.DASHBOARD_CONFIG
    } else {
      process.env.DASHBOARD_CONFIG = original
    }
    unlinkSync(configPath)
  }
})

describe("home() — valid config, empty repos", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-test-ok-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-test-${Date.now()}.db`)
  writeFileSync(configPath, "source: manual\nrepos: []", "utf8")

  const origConfig = process.env.DASHBOARD_CONFIG
  const origDb = process.env.DATABASE_URL
  process.env.DASHBOARD_CONFIG = configPath
  process.env.DATABASE_URL = dbPath

  try {
    const response = await home()
    const html = await bodyText(response)

    assert({
      given: "a valid config with empty repos list",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a valid config with empty repos list",
      should: "return HTML with content-type header",
      actual: response.headers.get("Content-Type"),
      expected: "text/html; charset=utf-8",
    })

    assert({
      given: "a valid config with empty repos list",
      should: "include the page title",
      actual: html.includes("Quarantine Dashboard"),
      expected: true,
    })

    assert({
      given: "a valid config with empty repos list",
      should: "include the Projects heading",
      actual: html.includes("Projects"),
      expected: true,
    })
  } finally {
    if (origConfig === undefined) {
      delete process.env.DASHBOARD_CONFIG
    } else {
      process.env.DASHBOARD_CONFIG = origConfig
    }
    if (origDb === undefined) {
      delete process.env.DATABASE_URL
    } else {
      process.env.DATABASE_URL = origDb
    }
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})
