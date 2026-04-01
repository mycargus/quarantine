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
 * Pure: apply both search and status filters (AND logic).
 */
export function applyFilters(
  tests: QuarantinedTestDetail[],
  query: string,
  status: "passing" | "failing" | null,
): QuarantinedTestDetail[] {
  return filterByStatus(filterBySearch(tests, query), status)
}
