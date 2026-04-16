import { route } from "remix/fetch-router/routes"

export const routes = route({
  home: "/",
  health: "/health",
  projectDetail: "/projects/:owner/:repo",
})
