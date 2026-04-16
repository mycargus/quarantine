/**
 * Outputs a valid signed session cookie header value to stdout for E2E test use.
 *
 * Usage:
 *   SESSION_SECRET=<secret> node --import tsx/esm scripts/e2e-session-cookie.ts
 *
 * Prints only the "name=value" portion (Cookie header format, not Set-Cookie),
 * so the caller can pass it directly as: Cookie: <output>
 */
import { createSession } from "@remix-run/session"
import { createCookie } from "remix/cookie"

const secret = process.env.SESSION_SECRET ?? "dev-secret-change-in-production"

const cookie = createCookie("__session", {
  httpOnly: true,
  secure: true,
  sameSite: "Lax" as const,
  maxAge: 28800,
  secrets: [secret],
})

const session = createSession()
session.set("userId" as never, "e2e-test-user" as never)
const serializedData = JSON.stringify({ i: session.id, d: session.data })
const setCookieHeader = await cookie.serialize(serializedData)

// Output only the "name=value" segment — the Set-Cookie header includes
// attributes like HttpOnly, Secure, SameSite that we don't want in the
// Cookie request header.
process.stdout.write(setCookieHeader.split(";")[0])
