/**
 * Interface test for InstallationTokenProvider.getToken().
 *
 * Exercises the token exchange through its public API with a local HTTP
 * server standing in for GitHub's /app/installations/{id}/access_tokens
 * endpoint. Internal dependencies (JWT generation via node:crypto) are real.
 */

import { generateKeyPairSync } from "node:crypto"
import { createServer, type IncomingMessage, type Server } from "node:http"
import { describe } from "riteway"
import { InstallationTokenProvider } from "../app/lib/installation-token.server.js"

interface CapturedRequest {
  method: string | undefined
  url: string | undefined
  headers: Record<string, string | string[] | undefined>
  body: string
}

function startMockGitHub(options: { expiresAt?: string } = {}): Promise<{
  server: Server
  port: number
  captured: CapturedRequest[]
}> {
  const expiresAt = options.expiresAt ?? "2026-03-15T13:00:00Z"
  return new Promise((resolve) => {
    const captured: CapturedRequest[] = []

    const server = createServer((req: IncomingMessage, res) => {
      let body = ""
      req.on("data", (chunk: Buffer) => {
        body += chunk.toString()
      })
      req.on("end", () => {
        captured.push({
          method: req.method,
          url: req.url,
          headers: req.headers,
          body,
        })

        res.writeHead(200, { "Content-Type": "application/json" })
        res.end(
          JSON.stringify({
            token: "ghs_test123",
            expires_at: expiresAt,
            permissions: {},
          }),
        )
      })
    })

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address()
      const port = typeof addr === "object" && addr !== null ? addr.port : 0
      resolve({ server, port, captured })
    })
  })
}

function closeServer(server: Server): Promise<void> {
  return new Promise((resolve) => {
    server.close(() => resolve())
  })
}

describe("InstallationTokenProvider.getToken()", async (assert) => {
  const { privateKey } = generateKeyPairSync("rsa", {
    modulusLength: 2048,
    publicKeyEncoding: { type: "spki", format: "pem" },
    privateKeyEncoding: { type: "pkcs8", format: "pem" },
  })

  const { server, port, captured } = await startMockGitHub()

  try {
    const provider = new InstallationTokenProvider({
      clientID: "Iv1.abc123",
      privateKeyPEM: privateKey as string,
      baseUrl: `http://127.0.0.1:${port}`,
    })

    const result = await provider.getToken(12345)

    assert({
      given: "a valid JWT and installation ID",
      should: "return the token from the response",
      actual: result.token,
      expected: "ghs_test123",
    })

    assert({
      given: "a valid JWT and installation ID",
      should: "return a valid expiresAt Date",
      actual: result.expiresAt instanceof Date && !Number.isNaN(result.expiresAt.getTime()),
      expected: true,
    })

    assert({
      given: "a valid JWT and installation ID",
      should: "send POST to /app/installations/12345/access_tokens",
      actual: `${captured[0].method} ${captured[0].url}`,
      expected: "POST /app/installations/12345/access_tokens",
    })

    assert({
      given: "a valid JWT and installation ID",
      should: "include a Bearer token in the Authorization header",
      actual:
        typeof captured[0].headers.authorization === "string" &&
        captured[0].headers.authorization.startsWith("Bearer "),
      expected: true,
    })

    const requestBody = captured[0].body
    const parsedBody = requestBody === "" ? null : JSON.parse(requestBody)
    const hasPermissions =
      parsedBody !== null && typeof parsedBody === "object" && "permissions" in parsedBody

    assert({
      given: "a valid JWT and installation ID",
      should: "not include a permissions field in the request body",
      actual: hasPermissions,
      expected: false,
    })
  } finally {
    await closeServer(server)
  }
})

describe("InstallationTokenProvider.getToken() — caching", async (assert) => {
  const { privateKey } = generateKeyPairSync("rsa", {
    modulusLength: 2048,
    publicKeyEncoding: { type: "spki", format: "pem" },
    privateKeyEncoding: { type: "pkcs8", format: "pem" },
  })

  const futureExpiry = new Date(Date.now() + 30 * 60 * 1000).toISOString()
  const { server, port, captured } = await startMockGitHub({ expiresAt: futureExpiry })

  try {
    const provider = new InstallationTokenProvider({
      clientID: "Iv1.abc123",
      privateKeyPEM: privateKey as string,
      baseUrl: `http://127.0.0.1:${port}`,
    })

    const first = await provider.getToken(12345)
    const second = await provider.getToken(12345)

    assert({
      given: "two getToken calls for the same installation within validity window",
      should: "make only one HTTP request to the server",
      actual: captured.length,
      expected: 1,
    })

    assert({
      given: "two getToken calls for the same installation within validity window",
      should: "return the same token from both calls",
      actual: second.token,
      expected: first.token,
    })
  } finally {
    await closeServer(server)
  }
})
