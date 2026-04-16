import { createRouter } from "remix/fetch-router"
import { home } from "./controllers/home.js"
import { project } from "./controllers/project.js"
import { routes } from "./routes.js"

export interface AppOptions {
  configPath?: string
  dbPath?: string
  token?: string
  fetchFn?: typeof fetch
}

export function createApp(opts: AppOptions = {}) {
  const router = createRouter()
  router.map(routes, {
    actions: {
      home: () => home(opts),
      health: () => new Response("ok", { status: 200 }),
      projectDetail: (ctx) =>
        project(ctx.params.owner, ctx.params.repo, ctx.request.url, opts.dbPath),
    },
  })
  return router
}
