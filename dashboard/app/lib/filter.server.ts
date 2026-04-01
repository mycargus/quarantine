import type { QuarantinedTestDetail } from "./db.server.js"

/**
 * Pure: filter tests by case-insensitive substring match on name OR test_id.
 * An empty query returns all tests.
 */
export function filterBySearch(
  tests: QuarantinedTestDetail[],
  query: string,
): QuarantinedTestDetail[] {
  if (query === "") return tests
  const lower = query.toLowerCase()
  return tests.filter(
    (t) => t.name.toLowerCase().includes(lower) || t.testId.toLowerCase().includes(lower),
  )
}

/**
 * Pure: filter tests by last_run_status.
 * A null status returns all tests (no filter applied).
 */
export function filterByStatus(
  tests: QuarantinedTestDetail[],
  status: "passing" | "failing" | null,
): QuarantinedTestDetail[] {
  if (status === null) return tests
  return tests.filter((t) => t.lastRunStatus === status)
}

/**
 * Pure: filter tests by quarantine date range (inclusive on both ends).
 * Compares the YYYY-MM-DD portion of quarantinedAt against from/until.
 * Null bounds are unbounded (no lower or upper limit).
 */
export function filterByDateRange(
  tests: QuarantinedTestDetail[],
  from: string | null,
  until: string | null,
): QuarantinedTestDetail[] {
  if (from === null && until === null) return tests
  const fromDate = from?.slice(0, 10) ?? null
  const untilDate = until?.slice(0, 10) ?? null
  return tests.filter((t) => {
    const date = t.quarantinedAt.slice(0, 10)
    if (fromDate !== null && date < fromDate) return false
    if (untilDate !== null && date > untilDate) return false
    return true
  })
}

/**
 * Pure: apply search, status, and date range filters (all AND logic).
 * from and until default to null (unbounded).
 */
export function applyFilters(
  tests: QuarantinedTestDetail[],
  query: string,
  status: "passing" | "failing" | null,
  from: string | null = null,
  until: string | null = null,
): QuarantinedTestDetail[] {
  return filterByDateRange(filterByStatus(filterBySearch(tests, query), status), from, until)
}
