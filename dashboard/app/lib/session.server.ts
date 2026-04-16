import { auth, createSessionAuthScheme } from "@remix-run/auth-middleware"
import type { Session } from "@remix-run/session"
import { createCookieSessionStorage } from "@remix-run/session/cookie-storage"
import { session } from "@remix-run/session-middleware"
import { createCookie } from "remix/cookie"

const SESSION_KEY = "userId"

export function createSessionMiddleware(secret: string) {
  const cookie = createCookie("__session", {
    httpOnly: true,
    sameSite: "Lax" as const,
    secrets: [secret],
  })

  const sessionStorage = createCookieSessionStorage()

  const sessionScheme = createSessionAuthScheme<string>({
    read(s: Session) {
      return s.get(SESSION_KEY) ?? null
    },
    verify(value: string) {
      return value
    },
  })

  return {
    session: session(cookie, sessionStorage),
    auth: auth({ schemes: [sessionScheme] }),
    cookie,
    sessionStorage,
  }
}
