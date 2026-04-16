import { describe } from "riteway"
import { checkRateLimit, parseLinkHeader } from "./github-client.server.js"

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
    given: "remaining is 200 of 1000 (exactly 20%, at the threshold)",
    should: "return null (threshold is exclusive: < 0.2, not <= 0.2)",
    actual: checkRateLimit(200, 1000, "core"),
    expected: null,
  })

  assert({
    given: "remaining is 250 of 1000 (25%, above 20% threshold)",
    should: "return null (no warning)",
    actual: checkRateLimit(250, 1000, "core"),
    expected: null,
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

describe("parseLinkHeader()", async (assert) => {
  assert({
    given: "a header with next and last links",
    should: "return the next URL",
    actual: parseLinkHeader(
      '<https://api.github.com/app/installations?page=2&per_page=100>; rel="next", <https://api.github.com/app/installations?page=5&per_page=100>; rel="last"',
    ),
    expected: "https://api.github.com/app/installations?page=2&per_page=100",
  })

  assert({
    given: "a header with only a next link",
    should: "return the next URL",
    actual: parseLinkHeader(
      '<https://api.github.com/repos/owner/repo/actions/artifacts?page=3&per_page=30>; rel="next"',
    ),
    expected: "https://api.github.com/repos/owner/repo/actions/artifacts?page=3&per_page=30",
  })

  assert({
    given: "a header with only a rel=last link (no next)",
    should: "return null",
    actual: parseLinkHeader(
      '<https://api.github.com/app/installations?page=5&per_page=100>; rel="last"',
    ),
    expected: null,
  })

  assert({
    given: "a null header",
    should: "return null",
    actual: parseLinkHeader(null),
    expected: null,
  })

  assert({
    given: "an empty string header",
    should: "return null",
    actual: parseLinkHeader(""),
    expected: null,
  })

  assert({
    given: "a malformed header with no angle brackets or rel",
    should: "return null",
    actual: parseLinkHeader("this is not a valid link header"),
    expected: null,
  })
})
