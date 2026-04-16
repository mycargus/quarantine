import { describe } from "riteway"
import { checkRateLimit } from "./github-client.server.js"

describe("checkRateLimit()", async (assert) => {
  const result = checkRateLimit(150, 1000, "core")

  assert({
    given: "remaining is 150 of 1000 (15%, below 20% threshold)",
    should: "return a warning with shouldWarn true",
    actual: result?.shouldWarn,
    expected: true,
  })

  assert({
    given: "remaining is 150 of 1000 (15%, below 20% threshold)",
    should: "include the remaining count",
    actual: result?.remaining,
    expected: 150,
  })

  assert({
    given: "remaining is 150 of 1000 (15%, below 20% threshold)",
    should: "include the limit",
    actual: result?.limit,
    expected: 1000,
  })

  assert({
    given: "remaining is 150 of 1000 (15%, below 20% threshold)",
    should: "include the resource name",
    actual: result?.resource,
    expected: "core",
  })

  assert({
    given: "remaining is 800 of 1000 (80%, above 20% threshold)",
    should: "return null (no warning)",
    actual: checkRateLimit(800, 1000, "core"),
    expected: null,
  })

  assert({
    given: "both rate limit headers are missing (null, null)",
    should: "return null (no warning, no error)",
    actual: checkRateLimit(null, null, "core"),
    expected: null,
  })

  assert({
    given: "only X-RateLimit-Remaining is missing (null, 1000)",
    should: "return null (no warning, no error)",
    actual: checkRateLimit(null, 1000, "core"),
    expected: null,
  })

  assert({
    given: "only X-RateLimit-Limit is missing (150, null)",
    should: "return null (no warning, no error)",
    actual: checkRateLimit(150, null, "core"),
    expected: null,
  })
})
