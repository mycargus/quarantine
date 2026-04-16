import { createGitHubAuthProvider, startExternalAuth } from "@remix-run/auth"
import { requireAuth } from "@remix-run/auth-middleware"
import { createRouter } from "remix/fetch-router"
import { home } from "./controllers/home.js"
import { project } from "./controllers/project.js"
import { createSessionMiddleware } from "./lib/session.server.js"
import { routes } from "./routes.js"

export interface AppOptions {
  configPath?: string
  dbPath?: string
  token?: string
  fetchFn?: typeof fetch
  sessionSecret?: string
  oauthClientId?: string
  oauthClientSecret?: string
  oauthOrigin?: string
}

const DEFAULT_SECRET = "dev-secret-change-in-production"

export function createApp(opts: AppOptions = {}) {
  const { session, auth } = createSessionMiddleware(opts.sessionSecret ?? DEFAULT_SECRET)

  const githubProvider =
    opts.oauthClientId && opts.oauthClientSecret && opts.oauthOrigin
      ? createGitHubAuthProvider({
          clientId: opts.oauthClientId,
          clientSecret: opts.oauthClientSecret,
          redirectUri: `${opts.oauthOrigin}/auth/github/callback`,
        })
      : null

  const router = createRouter({ middleware: [session, auth] })
  router.map(routes, {
    actions: {
      home: {
        middleware: [requireAuth()],
        handler: () => home(opts),
      },
      health: () => new Response("ok", { status: 200 }),
      authLogin: (ctx) => startExternalAuth(githubProvider!, ctx),
      projectDetail: {
        middleware: [requireAuth()],
        handler: (ctx) => project(ctx.params.owner, ctx.params.repo, ctx.request.url, opts.dbPath),
      },
    },
  })
  return router
}
