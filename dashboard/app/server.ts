import * as http from "node:http"
import { createRouter } from "remix/fetch-router"
import { createRequestListener } from "remix/node-fetch-server"
import { home } from "./controllers/home.js"
import { routes } from "./routes.js"

const router = createRouter()

router.map(routes, {
  actions: {
    home,
  },
})

const port = Number(process.env.PORT ?? 3000)
const server = http.createServer(createRequestListener((req) => router.fetch(req)))

server.listen(port, () => {
  console.log(`Quarantine dashboard running at http://localhost:${port}`)
})
