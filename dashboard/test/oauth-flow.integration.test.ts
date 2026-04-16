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

describe("User rate limit — 301st authenticated request returns 429 with Retry-After", async (assert) => {
  const fixedNow = 1_000_000
  const { router, sessionCookie, cleanup } = createTestApp({
    repos: [],
    clock: () => fixedNow,
  })
  const cookie = await sessionCookie()
  try {
    // Send 300 authenticated requests — cycle through IPs to avoid IP rate limit (20/min)
    for (let i = 0; i < 300; i++) {
      const ip = `10.0.0.${i % 16}`
      const res = await router.fetch(
        new Request("http://localhost/health", {
          headers: {
            Cookie: cookie,
            "X-Forwarded-For": ip,
          },
        }),
      )
      if (res.status === 429) {
        assert({
          given: `authenticated request ${i + 1} of 300 within the user rate limit`,
          should: "not be rate limited",
          actual: res.status,
          expected: 200,
        })
        return
      }
    }

    // Request #301 should be rate limited by the user limiter
    const response = await router.fetch(
      new Request("http://localhost/health", {
        headers: {
          Cookie: cookie,
          "X-Forwarded-For": "10.0.0.0",
        },
      }),
    )

    assert({
      given: "301st authenticated request from the same user within 1-minute window",
      should: "return HTTP 429",
      actual: response.status,
      expected: 429,
    })

    const retryAfter = response.headers.get("Retry-After")

    assert({
      given: "301st authenticated request from the same user within 1-minute window",
      should: "include a Retry-After header with seconds until window resets",
      actual: retryAfter !== null && Number(retryAfter) > 0,
      expected: true,
    })
  } finally {
    cleanup()
  }
})

describe("GET /auth/github/callback — valid code and state completes login", async (assert) => {
  const { router, cleanup } = createTestApp({
    repos: [],
    oauthClientId: "test-client-id",
    oauthClientSecret: "test-secret",
    oauthOrigin: "http://localhost:3000",
  })
  try {
    // Step 1: Start the login flow to get PKCE state in the session
    const loginResponse = await router.fetch(new Request("http://localhost/auth/login"))
    const location = loginResponse.headers.get("Location") ?? ""
    const locationUrl = new URL(location)
    const state = locationUrl.searchParams.get("state") ?? ""

    // Step 2: Mock GitHub's token exchange and user profile endpoints
    const originalFetch = globalThis.fetch
    const restoreFetch = () => {
      globalThis.fetch = originalFetch
    }
    globalThis.fetch = async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
      if (url.startsWith("https://github.com/login/oauth/access_token")) {
        return new Response(
          JSON.stringify({
            access_token: "gho_fake_token_123",
            token_type: "bearer",
            scope: "read:user,user:email",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        )
      }
      if (url.startsWith("https://api.github.com/user/emails")) {
        return new Response(
          JSON.stringify([
            { email: "octocat@github.com", primary: true, verified: true, visibility: "public" },
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } },
        )
      }
      if (url.startsWith("https://api.github.com/user")) {
        return new Response(
          JSON.stringify({
            id: 42,
            login: "octocat",
            name: "The Octocat",
            email: null,
            avatar_url: "https://github.com/images/error/octocat_happy.gif",
            html_url: "https://github.com/octocat",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        )
      }
      return new Response("Not found", { status: 404 })
    }

    try {
      // Step 3: Call the callback endpoint with the session cookie from login
      const loginCookies = loginResponse.headers
        .getSetCookie()
        .map((c) => c.split(";")[0])
        .join("; ")
      const callbackResponse = await router.fetch(
        new Request(`http://localhost/auth/github/callback?code=fake-code&state=${state}`, {
          headers: { Cookie: loginCookies },
        }),
      )

      assert({
        given: "a valid authorization code and state from GitHub OAuth",
        should: "return HTTP 302 redirect",
        actual: callbackResponse.status,
        expected: 302,
      })

      const redirectLocation = callbackResponse.headers.get("Location") ?? ""

      assert({
        given: "a valid authorization code and state from GitHub OAuth",
        should: "redirect to /",
        actual: new URL(redirectLocation, "http://localhost").pathname,
        expected: "/",
      })

      const setCookie = callbackResponse.headers.get("Set-Cookie") ?? ""

      assert({
        given: "a valid authorization code and state from GitHub OAuth",
        should: "set a session cookie",
        actual: setCookie !== "",
        expected: true,
      })

      const lowerCookie = setCookie.toLowerCase()

      assert({
        given: "a valid authorization code and state from GitHub OAuth",
        should: "set HttpOnly on the session cookie",
        actual: lowerCookie.includes("httponly"),
        expected: true,
      })

      assert({
        given: "a valid authorization code and state from GitHub OAuth",
        should: "set Secure on the session cookie",
        actual: lowerCookie.includes("secure"),
        expected: true,
      })

      assert({
        given: "a valid authorization code and state from GitHub OAuth",
        should: "set SameSite=Lax on the session cookie",
        actual: lowerCookie.includes("samesite=lax"),
        expected: true,
      })

      assert({
        given: "a valid authorization code and state from GitHub OAuth",
        should: "set Max-Age=28800 on the session cookie",
        actual: lowerCookie.includes("max-age=28800"),
        expected: true,
      })

      // Step 4: Verify the session cookie grants access to a protected route
      const callbackCookies = callbackResponse.headers
        .getSetCookie()
        .map((c) => c.split(";")[0])
        .join("; ")
      const authenticatedResponse = await router.fetch(
        new Request("http://localhost/", { headers: { Cookie: callbackCookies } }),
      )

      assert({
        given: "the session cookie from a successful OAuth callback",
        should: "grant access to the protected home route (HTTP 200)",
        actual: authenticatedResponse.status,
        expected: 200,
      })
    } finally {
      restoreFetch()
    }
  } finally {
    cleanup()
  }
})

describe("GET /auth/github/callback — invalid authorization code returns error", async (assert) => {
  const { router, cleanup } = createTestApp({
    repos: [],
    oauthClientId: "test-client-id",
    oauthClientSecret: "test-secret",
    oauthOrigin: "http://localhost:3000",
  })
  try {
    // Step 1: Start the login flow to get PKCE state in the session
    const loginResponse = await router.fetch(new Request("http://localhost/auth/login"))
    const location = loginResponse.headers.get("Location") ?? ""
    const locationUrl = new URL(location)
    const state = locationUrl.searchParams.get("state") ?? ""

    // Step 2: Mock GitHub's token exchange to return an error (invalid code)
    const originalFetch = globalThis.fetch
    const restoreFetch = () => {
      globalThis.fetch = originalFetch
    }
    globalThis.fetch = async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
      if (url.startsWith("https://github.com/login/oauth/access_token")) {
        return new Response(
          JSON.stringify({
            error: "bad_verification_code",
            error_description: "The code passed is incorrect or expired.",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        )
      }
      return new Response("Not found", { status: 404 })
    }

    try {
      // Step 3: Call the callback endpoint with the session cookie from login
      const loginCookies = loginResponse.headers
        .getSetCookie()
        .map((c) => c.split(";")[0])
        .join("; ")
      const callbackResponse = await router.fetch(
        new Request(`http://localhost/auth/github/callback?code=invalid-code&state=${state}`, {
          headers: { Cookie: loginCookies },
        }),
      )

      assert({
        given: "an invalid authorization code on OAuth callback",
        should: "return an HTTP error status (4xx)",
        actual: callbackResponse.status >= 400 && callbackResponse.status < 500,
        expected: true,
      })

      // Verify the session cookie from the error response does NOT grant
      // access to protected routes (no userId was set in the session).
      const callbackCookies = callbackResponse.headers
        .getSetCookie()
        .map((c) => c.split(";")[0])
        .join("; ")
      const protectedResponse = await router.fetch(
        new Request("http://localhost/", { headers: { Cookie: callbackCookies } }),
      )

      assert({
        given: "the session cookie from a failed OAuth callback",
        should: "not grant access to protected routes (HTTP 401)",
        actual: protectedResponse.status,
        expected: 401,
      })
    } finally {
      restoreFetch()
    }
  } finally {
    cleanup()
  }
})

describe("IP rate limit — counter resets after the 60-second window expires", async (assert) => {
  let fakeNow = 1_000_000
  const clock = () => fakeNow
  const { router, cleanup } = createTestApp({
    repos: [],
    clock,
  })
  try {
    const clientIp = "198.51.100.7"

    // Exhaust the 20-request limit within the current window
    for (let i = 0; i < 20; i++) {
      await router.fetch(
        new Request("http://localhost/health", {
          headers: { "X-Forwarded-For": clientIp },
        }),
      )
    }

    // Confirm request #21 is rate limited
    const blockedResponse = await router.fetch(
      new Request("http://localhost/health", {
        headers: { "X-Forwarded-For": clientIp },
      }),
    )

    assert({
      given: "21st request within the same 1-minute window",
      should: "return HTTP 429",
      actual: blockedResponse.status,
      expected: 429,
    })

    // Advance the clock past the 60-second window boundary
    fakeNow += 61_000

    // Send a new request after the window has expired
    const resetResponse = await router.fetch(
      new Request("http://localhost/health", {
        headers: { "X-Forwarded-For": clientIp },
      }),
    )

    assert({
      given: "a request after the 60-second rate limit window has expired",
      should: "not return 429 because the counter has reset",
      actual: resetResponse.status !== 429,
      expected: true,
    })

    assert({
      given: "a request after the 60-second rate limit window has expired",
      should: "return HTTP 200",
      actual: resetResponse.status,
      expected: 200,
    })
  } finally {
    cleanup()
  }
})

describe("GET /auth/github/callback — successful login logs auth event", async (assert) => {
  const { router, cleanup } = createTestApp({
    repos: [],
    oauthClientId: "test-client-id",
    oauthClientSecret: "test-secret",
    oauthOrigin: "http://localhost:3000",
  })
  try {
    // Step 1: Start the login flow to get PKCE state in the session
    const loginResponse = await router.fetch(new Request("http://localhost/auth/login"))
    const location = loginResponse.headers.get("Location") ?? ""
    const locationUrl = new URL(location)
    const state = locationUrl.searchParams.get("state") ?? ""

    // Step 2: Mock GitHub's token exchange and user profile endpoints
    const originalFetch = globalThis.fetch
    const restoreFetch = () => {
      globalThis.fetch = originalFetch
    }
    globalThis.fetch = async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url
      if (url.startsWith("https://github.com/login/oauth/access_token")) {
        return new Response(
          JSON.stringify({
            access_token: "ghu_mock_secret_token_abc123",
            token_type: "bearer",
            scope: "read:user,user:email",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        )
      }
      if (url.startsWith("https://api.github.com/user/emails")) {
        return new Response(
          JSON.stringify([
            { email: "octocat@github.com", primary: true, verified: true, visibility: "public" },
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } },
        )
      }
      if (url.startsWith("https://api.github.com/user")) {
        return new Response(
          JSON.stringify({
            id: 42,
            login: "octocat",
            name: "The Octocat",
            email: null,
            avatar_url: "https://github.com/images/error/octocat_happy.gif",
            html_url: "https://github.com/octocat",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        )
      }
      return new Response("Not found", { status: 404 })
    }

    // Step 3: Capture console.log output during the callback
    const logOutput: string[] = []
    const originalLog = console.log
    const restoreLog = () => {
      console.log = originalLog
    }
    console.log = (...args: unknown[]) => {
      logOutput.push(args.map(String).join(" "))
    }

    try {
      const loginCookies = loginResponse.headers
        .getSetCookie()
        .map((c) => c.split(";")[0])
        .join("; ")
      await router.fetch(
        new Request(`http://localhost/auth/github/callback?code=fake-code&state=${state}`, {
          headers: { Cookie: loginCookies },
        }),
      )

      const allLogs = logOutput.join("\n")

      assert({
        given: "a successful OAuth login callback",
        should: "log an auth event containing the event type 'login'",
        actual: allLogs.includes("login"),
        expected: true,
      })

      assert({
        given: "a successful OAuth login callback",
        should: "log an auth event containing the GitHub user ID",
        actual: allLogs.includes("octocat"),
        expected: true,
      })

      assert({
        given: "a successful OAuth login callback",
        should: "log an auth event containing an ISO 8601 timestamp",
        actual: /\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(allLogs),
        expected: true,
      })

      assert({
        given: "a successful OAuth login callback",
        should: "not include the access token (ghu_ prefix) in log output",
        actual: allLogs.includes("ghu_"),
        expected: false,
      })
    } finally {
      restoreLog()
      restoreFetch()
    }
  } finally {
    cleanup()
  }
})
