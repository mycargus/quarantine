import * as http from "node:http"
import { createRequestListener } from "remix/node-fetch-server"
import { createApp } from "./app.js"

const router = createApp()
const port = Number(process.env.PORT ?? 3000)
const server = http.createServer(createRequestListener((req) => router.fetch(req)))

server.listen(port, () => {
  console.log(`Quarantine dashboard running at http://localhost:${port}`)
})
