import { describe } from "riteway"
import type { QuarantinedTestDetail } from "./db.server.js"
import { applyFilters, filterByDateRange, filterBySearch, filterByStatus } from "./filter.server.js"

function makeTest(
  overrides: Partial<QuarantinedTestDetail> & { name: string; testId: string },
): QuarantinedTestDetail {
  return {
    quarantinedAt: "2026-01-01T00:00:00Z",
    lastFlakyAt: null,
    issueNumber: null,
    issueUrl: null,
    lastRunStatus: null,
    ...overrides,
  }
}

describe("filterBySearch()", async (assert) => {
  const tests: QuarantinedTestDetail[] = [
    makeTest({ name: "timeout when DB is slow", testId: "suite::timeout-db" }),
    makeTest({ name: "should process payment", testId: "suite::payment" }),
    makeTest({ name: "TIMEOUT on network call", testId: "suite::network-timeout" }),
    makeTest({ name: "should validate card", testId: "suite::validate" }),
    makeTest({ name: "normal test", testId: "suite::timeout-id-match" }),
  ]

  assert({
    given: 'a search query "timeout"',
    should: "return tests whose name contains 'timeout' (case-insensitive)",
    actual: filterBySearch(tests, "timeout").map((t) => t.name),
    expected: ["timeout when DB is slow", "TIMEOUT on network call", "normal test"],
  })

  assert({
    given: "an empty search query",
    should: "return all tests unchanged",
    actual: filterBySearch(tests, "").length,
    expected: tests.length,
  })

  assert({
    given: 'a search query matching only a testId substring "payment"',
    should: "return the test whose testId contains 'payment'",
    actual: filterBySearch(tests, "payment").map((t) => t.name),
    expected: ["should process payment"],
  })

  assert({
    given: "a search query with no matches",
    should: "return an empty array",
    actual: filterBySearch(tests, "zzznomatch"),
    expected: [],
  })
})

describe("filterByStatus()", async (assert) => {
  const tests: QuarantinedTestDetail[] = [
    makeTest({ name: "failing test 1", testId: "t1", lastRunStatus: "failing" }),
    makeTest({ name: "passing test 1", testId: "t2", lastRunStatus: "passing" }),
    makeTest({ name: "failing test 2", testId: "t3", lastRunStatus: "failing" }),
    makeTest({ name: "unknown test", testId: "t4", lastRunStatus: null }),
  ]

  assert({
    given: 'status filter "failing"',
    should: "return only tests with lastRunStatus failing",
    actual: filterByStatus(tests, "failing").map((t) => t.name),
    expected: ["failing test 1", "failing test 2"],
  })

  assert({
    given: 'status filter "passing"',
    should: "return only tests with lastRunStatus passing",
    actual: filterByStatus(tests, "passing").map((t) => t.name),
    expected: ["passing test 1"],
  })

  assert({
    given: "null status filter",
    should: "return all tests unchanged",
    actual: filterByStatus(tests, null).length,
    expected: tests.length,
  })
})

describe("applyFilters()", async (assert) => {
  const tests: QuarantinedTestDetail[] = [
    makeTest({ name: "timeout on DB", testId: "t1", lastRunStatus: "failing" }),
    makeTest({ name: "timeout on network", testId: "t2", lastRunStatus: "passing" }),
    makeTest({ name: "payment timeout", testId: "t3", lastRunStatus: "failing" }),
    makeTest({ name: "should validate card", testId: "t4", lastRunStatus: "failing" }),
  ]

  assert({
    given: 'search "timeout" and status "failing"',
    should: "return only tests matching both filters (AND logic)",
    actual: applyFilters(tests, "timeout", "failing").map((t) => t.name),
    expected: ["timeout on DB", "payment timeout"],
  })

  assert({
    given: "search matches but status does not",
    should: "exclude the test",
    actual: applyFilters(tests, "timeout", "failing")
      .map((t) => t.name)
      .includes("timeout on network"),
    expected: false,
  })

  assert({
    given: "empty search and null status",
    should: "return all tests",
    actual: applyFilters(tests, "", null).length,
    expected: tests.length,
  })

  assert({
    given: 'empty search and status "failing"',
    should: "return only failing tests",
    actual: applyFilters(tests, "", "failing").map((t) => t.name),
    expected: ["timeout on DB", "payment timeout", "should validate card"],
  })

  assert({
    given: "an empty test array with empty query and null status",
    should: "return an empty array",
    actual: applyFilters([], "", null),
    expected: [],
  })

  assert({
    given: 'search "timeout", null status, from "2026-01-02", null until',
    should: "compose search and date range (AND logic)",
    actual: applyFilters(
      [
        makeTest({ name: "timeout early", testId: "e1", quarantinedAt: "2026-01-01T00:00:00Z" }),
        makeTest({ name: "timeout late", testId: "e2", quarantinedAt: "2026-01-03T00:00:00Z" }),
        makeTest({ name: "other late", testId: "e3", quarantinedAt: "2026-01-03T00:00:00Z" }),
      ],
      "timeout",
      null,
      "2026-01-02",
      null,
    ).map((t) => t.name),
    expected: ["timeout late"],
  })
})

describe("filterByDateRange()", async (assert) => {
  const tests: QuarantinedTestDetail[] = [
    makeTest({ name: "jan test", testId: "t1", quarantinedAt: "2026-01-15T00:00:00Z" }),
    makeTest({ name: "feb test", testId: "t2", quarantinedAt: "2026-02-15T00:00:00Z" }),
    makeTest({ name: "mar test", testId: "t3", quarantinedAt: "2026-03-15T00:00:00Z" }),
  ]

  assert({
    given: "null from and null until",
    should: "return all tests unchanged",
    actual: filterByDateRange(tests, null, null).length,
    expected: 3,
  })

  assert({
    given: 'from "2026-02-01" with no until',
    should: "return only tests quarantined on or after Feb 1",
    actual: filterByDateRange(tests, "2026-02-01", null).map((t) => t.name),
    expected: ["feb test", "mar test"],
  })

  assert({
    given: 'until "2026-02-28" with no from',
    should: "return only tests quarantined on or before Feb 28",
    actual: filterByDateRange(tests, null, "2026-02-28").map((t) => t.name),
    expected: ["jan test", "feb test"],
  })

  assert({
    given: 'from "2026-02-01" and until "2026-02-28"',
    should: "return only tests within the date range",
    actual: filterByDateRange(tests, "2026-02-01", "2026-02-28").map((t) => t.name),
    expected: ["feb test"],
  })

  assert({
    given: "from equals the exact quarantine date (inclusive lower bound)",
    should: "include the test quarantined on that date",
    actual: filterByDateRange(tests, "2026-02-15", null).map((t) => t.name),
    expected: ["feb test", "mar test"],
  })

  assert({
    given: "until equals the exact quarantine date (inclusive upper bound)",
    should: "include the test quarantined on that date",
    actual: filterByDateRange(tests, null, "2026-02-15").map((t) => t.name),
    expected: ["jan test", "feb test"],
  })

  assert({
    given: "a range that excludes all tests",
    should: "return an empty array",
    actual: filterByDateRange(tests, "2026-06-01", null).map((t) => t.name),
    expected: [],
  })

  assert({
    given: "an empty test array",
    should: "return an empty array",
    actual: filterByDateRange([], "2026-01-01", "2026-12-31"),
    expected: [],
  })

  assert({
    given: 'a non-ISO from date string "March 15, 2026"',
    should:
      "exclude all tests (documents implicit contract: from/until must be ISO YYYY-MM-DD format)",
    actual: filterByDateRange(
      [makeTest({ name: "mar test", testId: "t1", quarantinedAt: "2026-03-15T00:00:00Z" })],
      "March 15, 2026",
      null,
    ).map((t) => t.name),
    expected: [],
  })
})
