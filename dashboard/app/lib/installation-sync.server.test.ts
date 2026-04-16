import { describe } from "riteway"
import { shouldSyncInstallations } from "./installation-sync.server.js"

const now = new Date("2026-04-16T12:00:00.000Z")
const FIFTEEN_MIN = 900_000

describe("shouldSyncInstallations()", async (assert) => {
  assert({
    given: "lastSyncedAt is null (no sync has ever occurred)",
    should: "return true",
    actual: shouldSyncInstallations(null, now, FIFTEEN_MIN),
    expected: true,
  })

  {
    const fiveMinAgo = new Date(now.getTime() - 5 * 60_000)
    assert({
      given: "lastSyncedAt is 5 minutes ago (within 15 min interval)",
      should: "return false",
      actual: shouldSyncInstallations(fiveMinAgo, now, FIFTEEN_MIN),
      expected: false,
    })
  }
})
