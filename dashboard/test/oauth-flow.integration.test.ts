/**
 * Interface tests for the OAuth flow and related auth endpoints.
 *
 * Tests exercise the full request -> response path via router.fetch() with
 * no external GitHub API calls (token: "" prevents sync).
 */

import { describe } from "riteway"
import { createTestApp } from "./helpers.js"

describe("GET /health — no authentication provided", async (assert) => {
  const { router, cleanup } = createTestApp({ repos: [] })
  try {
    const response = await router.fetch(new Request("http://localhost/health"))

    assert({
      given: "no authentication",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })
  } finally {
    cleanup()
  }
})

describe("GET / — no session cookie (unauthenticated)", async (assert) => {
  const { router, cleanup } = createTestApp({ repos: [] })
  try {
    const response = await router.fetch(new Request("http://localhost/"))

    assert({
      given: "no session cookie on GET /",
      should: "return HTTP 401",
      actual: response.status,
      expected: 401,
    })
  } finally {
    cleanup()
  }
})

describe("GET / — valid session cookie (authenticated)", async (assert) => {
  const { router, sessionCookie, cleanup } = createTestApp({ repos: [] })
  const cookie = await sessionCookie()
  try {
    const response = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: cookie } }),
    )

    assert({
      given: "a valid session cookie on GET /",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })
  } finally {
    cleanup()
  }
})

describe("GET /auth/login — redirects to GitHub OAuth", async (assert) => {
  const { router, cleanup } = createTestApp({
    repos: [],
    oauthClientId: "test-client-id",
    oauthClientSecret: "test-secret",
    oauthOrigin: "http://localhost:3000",
  })
  try {
    const response = await router.fetch(new Request("http://localhost/auth/login"))

    assert({
      given: "a GET /auth/login request",
      should: "return HTTP 302 redirect",
      actual: response.status,
      expected: 302,
    })

    const location = response.headers.get("Location") ?? ""
    const locationUrl = new URL(location)

    assert({
      given: "a GET /auth/login request",
      should: "redirect to GitHub OAuth authorize endpoint",
      actual: locationUrl.origin + locationUrl.pathname,
      expected: "https://github.com/login/oauth/authorize",
    })

    assert({
      given: "a GET /auth/login request",
      should: "include client_id=test-client-id in redirect URL",
      actual: locationUrl.searchParams.get("client_id"),
      expected: "test-client-id",
    })

    assert({
      given: "a GET /auth/login request",
      should: "include redirect_uri containing /auth/github/callback",
      actual:
        locationUrl.searchParams.get("redirect_uri")?.includes("/auth/github/callback") ?? false,
      expected: true,
    })
  } finally {
    cleanup()
  }
})

describe("GET /auth/logout — authenticated user", async (assert) => {
  const { router, sessionCookie, cleanup } = createTestApp({
    repos: [],
    oauthClientId: "test-client-id",
    oauthClientSecret: "test-secret",
    oauthOrigin: "http://localhost:3000",
  })
  const cookie = await sessionCookie()
  try {
    // Step 1: Logout with a valid session cookie
    const logoutResponse = await router.fetch(
      new Request("http://localhost/auth/logout", { headers: { Cookie: cookie } }),
    )

    assert({
      given: "an authenticated user requesting GET /auth/logout",
      should: "return HTTP 302 redirect",
      actual: logoutResponse.status,
      expected: 302,
    })

    assert({
      given: "an authenticated user requesting GET /auth/logout",
      should: "redirect to /auth/login",
      actual: new URL(logoutResponse.headers.get("Location") ?? "", "http://localhost").pathname,
      expected: "/auth/login",
    })

    // Step 2: Use the Set-Cookie from logout response for next request
    const clearedCookie = logoutResponse.headers.get("Set-Cookie") ?? ""

    assert({
      given: "an authenticated user requesting GET /auth/logout",
      should: "return a Set-Cookie header to clear the session",
      actual: clearedCookie !== "",
      expected: true,
    })

    // Step 3: Try accessing a protected route with the cleared cookie
    const protectedResponse = await router.fetch(
      new Request("http://localhost/", { headers: { Cookie: clearedCookie } }),
    )

    assert({
      given: "a request to a protected route after logout",
      should: "return HTTP 401",
      actual: protectedResponse.status,
      expected: 401,
    })
  } finally {
    cleanup()
  }
})

describe("Session cookie — secure attributes", async (assert) => {
  const { router, sessionCookie, cleanup } = createTestApp({
    repos: [],
    oauthClientId: "test-client-id",
    oauthClientSecret: "test-secret",
    oauthOrigin: "http://localhost:3000",
  })
  const cookie = await sessionCookie()
  try {
    const response = await router.fetch(
      new Request("http://localhost/auth/logout", { headers: { Cookie: cookie } }),
    )
    const setCookie = response.headers.get("Set-Cookie") ?? ""
    const lower = setCookie.toLowerCase()

    assert({
      given: "a response that sets the session cookie",
      should: "include the HttpOnly attribute",
      actual: lower.includes("httponly"),
      expected: true,
    })

    assert({
      given: "a response that sets the session cookie",
      should: "include the Secure attribute",
      actual: lower.includes("secure"),
      expected: true,
    })

    assert({
      given: "a response that sets the session cookie",
      should: "include SameSite=Lax",
      actual: lower.includes("samesite=lax"),
      expected: true,
    })

    assert({
      given: "a response that sets the session cookie",
      should: "include Max-Age=28800",
      actual: lower.includes("max-age=28800"),
      expected: true,
    })
  } finally {
    cleanup()
  }
})

describe("GET /auth/logout — no session cookie (expired or absent)", async (assert) => {
  const { router, cleanup } = createTestApp({ repos: [] })
  try {
    const response = await router.fetch(new Request("http://localhost/auth/logout"))

    assert({
      given: "no session cookie on GET /auth/logout",
      should: "return HTTP 302 redirect",
      actual: response.status,
      expected: 302,
    })

    assert({
      given: "no session cookie on GET /auth/logout",
      should: "redirect to /auth/login",
      actual: new URL(response.headers.get("Location") ?? "", "http://localhost").pathname,
      expected: "/auth/login",
    })
  } finally {
    cleanup()
  }
})

describe("IP rate limit — 21st request returns 429 with Retry-After", async (assert) => {
  const fixedNow = 1_000_000
  const { router, cleanup } = createTestApp({
    repos: [],
    clock: () => fixedNow,
  })
  try {
    const clientIp = "203.0.113.42"

    // Send 20 requests — all should succeed (not 429)
    for (let i = 0; i < 20; i++) {
      const res = await router.fetch(
        new Request("http://localhost/health", {
          headers: { "X-Forwarded-For": clientIp },
        }),
      )
      if (res.status === 429) {
        assert({
          given: `request ${i + 1} of 20 within the rate limit`,
          should: "not be rate limited",
          actual: res.status,
          expected: 200,
        })
        return
      }
    }

    // Request #21 should be rate limited
    const response = await router.fetch(
      new Request("http://localhost/health", {
        headers: { "X-Forwarded-For": clientIp },
      }),
    )

    assert({
      given: "21st request from the same IP within 1-minute window",
      should: "return HTTP 429",
      actual: response.status,
      expected: 429,
    })

    const retryAfter = response.headers.get("Retry-After")

    assert({
      given: "21st request from the same IP within 1-minute window",
      should: "include a Retry-After header with seconds until window resets",
      actual: retryAfter !== null && Number(retryAfter) > 0,
      expected: true,
    })
  } finally {
    cleanup()
  }
})
