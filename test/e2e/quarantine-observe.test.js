/**
 * E2E test: observe real quarantine CI output
 *
 * Observes the quarantine-test-fixture CI's real output: quarantine state,
 * GitHub Issues, Search API shape, artifact contents, and PR comments.
 * No puppeteering — the fixture CI runs on a daily schedule and produces
 * real data that these tests read.
 *
 * Required env vars:
 *   QUARANTINE_GITHUB_TOKEN  — PAT with repo scope
 *   QUARANTINE_TEST_OWNER    — GitHub org or user (e.g. "mycargus")
 *   QUARANTINE_TEST_REPO     — repository name (e.g. "quarantine-test-fixture")
 */

import AdmZip from "adm-zip"
import { assert } from "riteway/vitest"
import { describe, test } from "vitest"

const SUITE_NAME = "jest-tests"
const STATE_BRANCH = "quarantine/state"
const STATE_PATH = `.quarantine/${SUITE_NAME}/state.json`
const ARTIFACT_PREFIX = `quarantine-results-${SUITE_NAME}-`
const ISSUE_LABEL_PREFIX = `quarantine:${SUITE_NAME}:`
const PR_COMMENT_MARKER = `<!-- quarantine:${SUITE_NAME} -->`
const KNOWN_FLAKY = ["PaymentService", "AuthService", "CacheService"]
const KNOWN_STABLE = "DatabaseService"

const token = process.env.QUARANTINE_GITHUB_TOKEN
const owner = process.env.QUARANTINE_TEST_OWNER
const repo = process.env.QUARANTINE_TEST_REPO

// --- Credential guard ---

if (!token) throw new Error("QUARANTINE_GITHUB_TOKEN is required")
if (!owner) throw new Error("QUARANTINE_TEST_OWNER is required")
if (!repo) throw new Error("QUARANTINE_TEST_REPO is required")

// --- GitHub API helpers ---

async function ghRequest(method, path) {
  return fetch(`https://api.github.com/repos/${owner}/${repo}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/vnd.github+json",
      "X-GitHub-Api-Version": "2022-11-28",
    },
  })
}

// --- Tests ---

describe(`quarantine observe — ${owner}/${repo} suite: ${SUITE_NAME}`, () => {
  test("quarantine state has known flaky tests", { timeout: 120_000 }, async () => {
    const res = await ghRequest(
      "GET",
      `/contents/${encodeURIComponent(STATE_PATH)}?ref=${STATE_BRANCH}`,
    )

    assert({
      given: `GET /contents/${STATE_PATH}?ref=${STATE_BRANCH}`,
      should: "return HTTP 200",
      actual: res.status,
      expected: 200,
    })

    const envelope = await res.json()
    const content = JSON.parse(Buffer.from(envelope.content, "base64").toString("utf8"))

    assert({
      given: "the state file content",
      should: "have version 1",
      actual: content.version,
      expected: 1,
    })

    assert({
      given: "the state file content",
      should: "have a non-empty tests object",
      actual: typeof content.tests === "object" && Object.keys(content.tests).length > 0,
      expected: true,
    })

    const testIds = Object.keys(content.tests)

    const allFlaky = testIds.every((id) => KNOWN_FLAKY.some((cls) => id.includes(cls)))

    assert({
      given: "all quarantined test IDs",
      should: "only contain known flaky classnames (PaymentService, AuthService, CacheService)",
      actual: allFlaky,
      expected: true,
    })

    const hasStable = testIds.some((id) => id.includes(KNOWN_STABLE))

    assert({
      given: "quarantined test IDs",
      should: "not include stable test (DatabaseService)",
      actual: hasStable,
      expected: false,
    })
  })

  test("GitHub Issues exist with correct format and recent dates", {
    timeout: 120_000,
  }, async () => {
    const res = await ghRequest("GET", `/issues?labels=quarantine&state=open&per_page=100`)

    assert({
      given: "GET /issues?labels=quarantine&state=open",
      should: "return HTTP 200",
      actual: res.status,
      expected: 200,
    })

    const issues = await res.json()

    assert({
      given: "open issues with quarantine label",
      should: "have at least one issue",
      actual: issues.length >= 1,
      expected: true,
    })

    const allTitlesMatch = issues.every((issue) => /^\[Quarantine\] .+$/.test(issue.title))

    assert({
      given: "open quarantine issue titles",
      should: 'all match "[Quarantine] <name>" format',
      actual: allTitlesMatch,
      expected: true,
    })

    const allHaveDedupLabel = issues.every((issue) =>
      issue.labels.some(
        (label) =>
          label.name.startsWith(ISSUE_LABEL_PREFIX) &&
          /^quarantine:jest-tests:[0-9a-f]{8}$/.test(label.name),
      ),
    )

    assert({
      given: "open quarantine issues",
      should: `all have a dedup label matching ${ISSUE_LABEL_PREFIX}<8 hex chars>`,
      actual: allHaveDedupLabel,
      expected: true,
    })

    const cutoff = Date.now() - 48 * 60 * 60 * 1000
    const allRecent = issues.every((issue) => new Date(issue.created_at).getTime() > cutoff)

    assert({
      given: "open quarantine issue creation dates",
      should: "all be within the last 48 hours (proves cycle ran recently)",
      actual: allRecent,
      expected: true,
    })
  })

  test("Search API returns closed quarantine issues", { timeout: 120_000 }, async () => {
    const query = encodeURIComponent(`repo:${owner}/${repo} is:issue is:closed label:quarantine`)
    const res = await fetch(`https://api.github.com/search/issues?q=${query}`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
      },
    })

    assert({
      given: "GET /search/issues?q=repo:...+is:closed+label:quarantine",
      should: "return HTTP 200",
      actual: res.status,
      expected: 200,
    })

    const data = await res.json()

    assert({
      given: "search results",
      should: "have total_count as a number",
      actual: typeof data.total_count,
      expected: "number",
    })

    assert({
      given: "search results",
      should: "have total_count >= 1 (closed issues exist from the daily cycle)",
      actual: data.total_count >= 1,
      expected: true,
    })

    assert({
      given: "search results",
      should: "have items as an array",
      actual: Array.isArray(data.items),
      expected: true,
    })

    assert({
      given: "search results items[0]",
      should: "have number as a number",
      actual: typeof data.items[0].number,
      expected: "number",
    })
  })

  test("artifact exists with valid results", { timeout: 120_000 }, async () => {
    const listRes = await ghRequest("GET", "/actions/artifacts?per_page=100")

    assert({
      given: "GET /actions/artifacts",
      should: "return HTTP 200",
      actual: listRes.status,
      expected: 200,
    })

    const { artifacts } = await listRes.json()

    const artifact = artifacts.find((a) => a.name.startsWith(ARTIFACT_PREFIX))

    assert({
      given: `artifacts list`,
      should: `include an artifact with name starting with "${ARTIFACT_PREFIX}"`,
      actual: artifact !== undefined,
      expected: true,
    })

    // Download the artifact ZIP (follows the 302 redirect automatically)
    const zipRes = await fetch(artifact.archive_download_url, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
      redirect: "follow",
    })

    assert({
      given: "artifact archive_download_url request",
      should: "return a successful response",
      actual: zipRes.ok,
      expected: true,
    })

    const zipBuffer = Buffer.from(await zipRes.arrayBuffer())
    const zip = new AdmZip(zipBuffer)
    const entry = zip.getEntry("results.json")

    assert({
      given: "artifact ZIP contents",
      should: 'contain a "results.json" entry',
      actual: entry !== null,
      expected: true,
    })

    const results = JSON.parse(zip.readAsText(entry))

    assert({
      given: "results.json",
      should: `have suite_name "${SUITE_NAME}"`,
      actual: results.suite_name,
      expected: SUITE_NAME,
    })

    assert({
      given: "results.json",
      should: "have version 1",
      actual: results.version,
      expected: 1,
    })

    assert({
      given: "results.json",
      should: "have a non-empty tests array",
      actual: Array.isArray(results.tests) && results.tests.length > 0,
      expected: true,
    })

    const stableTests = results.tests.filter((t) => t.classname?.includes(KNOWN_STABLE))

    assert({
      given: "results.json tests",
      should: `include at least one ${KNOWN_STABLE} test with status "passed"`,
      actual: stableTests.length > 0 && stableTests.every((t) => t.status === "passed"),
      expected: true,
    })
  })

  test("PR comment exists on proxy issue", { timeout: 120_000 }, async () => {
    // Find the open e2e-pr-proxy issue
    const issueRes = await ghRequest("GET", "/issues?labels=e2e-pr-proxy&state=open&per_page=1")

    assert({
      given: "GET /issues?labels=e2e-pr-proxy&state=open",
      should: "return HTTP 200",
      actual: issueRes.status,
      expected: 200,
    })

    const proxyIssues = await issueRes.json()

    assert({
      given: "issues with e2e-pr-proxy label",
      should: "have at least one open issue",
      actual: proxyIssues.length >= 1,
      expected: true,
    })

    const proxyNumber = proxyIssues[0].number

    // List comments on the proxy issue
    const commentsRes = await ghRequest("GET", `/issues/${proxyNumber}/comments?per_page=100`)

    assert({
      given: `GET /issues/${proxyNumber}/comments`,
      should: "return HTTP 200",
      actual: commentsRes.status,
      expected: 200,
    })

    const comments = await commentsRes.json()

    const markerComment = comments.find((c) => c.body.startsWith(PR_COMMENT_MARKER))

    assert({
      given: `comments on proxy issue #${proxyNumber}`,
      should: `include a comment starting with "${PR_COMMENT_MARKER}"`,
      actual: markerComment !== undefined,
      expected: true,
    })

    const mentionsFlaky = KNOWN_FLAKY.some((cls) => markerComment.body.includes(cls))

    assert({
      given: "PR comment body",
      should: "mention at least one known flaky test classname",
      actual: mentionsFlaky,
      expected: true,
    })
  })
})
