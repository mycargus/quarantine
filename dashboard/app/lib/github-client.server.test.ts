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
})
