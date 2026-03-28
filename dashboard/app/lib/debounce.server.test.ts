import { describe } from "riteway/esm"
import { shouldPull } from "./debounce.server.js"

const FIVE_MIN = 5 * 60 * 1000
const now = new Date("2026-03-28T10:06:00.000Z")

describe("shouldPull()", async (assert) => {
  assert({
    given: "lastPulledAt is null (never pulled)",
    should: "return true",
    actual: shouldPull(null, now, FIVE_MIN),
    expected: true,
  })

  assert({
    given: "lastPulledAt is 6 minutes ago (stale)",
    should: "return true",
    actual: shouldPull(new Date(now.getTime() - 6 * 60 * 1000).toISOString(), now, FIVE_MIN),
    expected: true,
  })

  assert({
    given: "lastPulledAt is 2 minutes ago (fresh)",
    should: "return false",
    actual: shouldPull(new Date(now.getTime() - 2 * 60 * 1000).toISOString(), now, FIVE_MIN),
    expected: false,
  })

  assert({
    given: "lastPulledAt is exactly 5 minutes ago",
    should: "return false (strict greater-than, not yet stale)",
    actual: shouldPull(new Date(now.getTime() - FIVE_MIN).toISOString(), now, FIVE_MIN),
    expected: false,
  })

  assert({
    given: "lastPulledAt is 1ms more than 5 minutes ago",
    should: "return true (just over threshold)",
    actual: shouldPull(new Date(now.getTime() - FIVE_MIN - 1).toISOString(), now, FIVE_MIN),
    expected: true,
  })
})
