/**
 * Interface test for InstallationTokenProvider.getToken() — concurrency.
 *
 * Tests pending-promise coalescing for both success and error cases.
 * Uses delayed mock servers to ensure concurrent callers overlap.
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

function closeServer(server: Server): Promise<void> {
  return new Promise((resolve) => {
    server.close(() => resolve())
  })
}

function startMockGitHubWithDelay(delayMs: number): Promise<{
  server: Server
  port: number
  captured: CapturedRequest[]
}> {
  const expiresAt = new Date(Date.now() + 30 * 60 * 1000).toISOString()
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

        setTimeout(() => {
          res.writeHead(200, { "Content-Type": "application/json" })
          res.end(
            JSON.stringify({
              token: "ghs_coalesced",
              expires_at: expiresAt,
              permissions: {},
            }),
          )
        }, delayMs)
      })
    })

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address()
      const port = typeof addr === "object" && addr !== null ? addr.port : 0
      resolve({ server, port, captured })
    })
  })
}

function startMockGitHubPerInstallation(delayMs: number): Promise<{
  server: Server
  port: number
  captured: CapturedRequest[]
}> {
  const expiresAt = new Date(Date.now() + 30 * 60 * 1000).toISOString()
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

        const installId = req.url?.match(/installations\/(\d+)/)?.[1] ?? "unknown"

        setTimeout(() => {
          res.writeHead(200, { "Content-Type": "application/json" })
          res.end(
            JSON.stringify({
              token: `ghs_token_${installId}`,
              expires_at: expiresAt,
              permissions: {},
            }),
          )
        }, delayMs)
      })
    })

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address()
      const port = typeof addr === "object" && addr !== null ? addr.port : 0
      resolve({ server, port, captured })
    })
  })
}

function startMockGitHubErrorWithDelay(
  statusCode: number,
  delayMs: number,
): Promise<{ server: Server; port: number; captured: CapturedRequest[] }> {
  return new Promise((resolve) => {
    const captured: CapturedRequest[] = []
    const server = createServer((req: IncomingMessage, res) => {
      let body = ""
      req.on("data", (chunk: Buffer) => {
        body += chunk.toString()
      })
      req.on("end", () => {
        captured.push({ method: req.method, url: req.url, headers: req.headers, body })
        setTimeout(() => {
          res.writeHead(statusCode, { "Content-Type": "application/json" })
          res.end(JSON.stringify({ message: "Internal Server Error" }))
        }, delayMs)
      })
    })
    server.listen(0, "127.0.0.1", () => {
      const addr = server.address()
      const port = typeof addr === "object" && addr !== null ? addr.port : 0
      resolve({ server, port, captured })
    })
  })
}

describe("InstallationTokenProvider.getToken() — concurrent coalescing", async (assert) => {
  const { privateKey } = generateKeyPairSync("rsa", {
    modulusLength: 2048,
    publicKeyEncoding: { type: "spki", format: "pem" },
    privateKeyEncoding: { type: "pkcs8", format: "pem" },
  })

  const { server, port, captured } = await startMockGitHubWithDelay(50)

  try {
    const provider = new InstallationTokenProvider({
      clientID: "Iv1.abc123",
      privateKeyPEM: privateKey as string,
      baseUrl: `http://127.0.0.1:${port}`,
    })

    const [first, second] = await Promise.all([provider.getToken(12345), provider.getToken(12345)])

    assert({
      given: "two concurrent getToken calls with no cached token",
      should: "make exactly one HTTP request",
      actual: captured.length,
      expected: 1,
    })

    assert({
      given: "two concurrent getToken calls with no cached token",
      should: "return the same token to both callers",
      actual: first.token === second.token,
      expected: true,
    })

    assert({
      given: "two concurrent getToken calls with no cached token",
      should: "return the same expiresAt to both callers",
      actual: first.expiresAt.getTime() === second.expiresAt.getTime(),
      expected: true,
    })

    // Verify the token is cached for subsequent calls
    const third = await provider.getToken(12345)

    assert({
      given: "a subsequent call after concurrent coalescing",
      should: "still make only one HTTP request total (served from cache)",
      actual: captured.length,
      expected: 1,
    })

    assert({
      given: "a subsequent call after concurrent coalescing",
      should: "return the same cached token",
      actual: third.token,
      expected: first.token,
    })
  } finally {
    await closeServer(server)
  }
})

describe("InstallationTokenProvider.getToken() — concurrent error propagation", async (assert) => {
  const { privateKey } = generateKeyPairSync("rsa", {
    modulusLength: 2048,
    publicKeyEncoding: { type: "spki", format: "pem" },
    privateKeyEncoding: { type: "pkcs8", format: "pem" },
  })

  const { server, port, captured } = await startMockGitHubErrorWithDelay(500, 50)

  try {
    const warnings: string[] = []
    const provider = new InstallationTokenProvider({
      clientID: "Iv1.abc123",
      privateKeyPEM: privateKey as string,
      baseUrl: `http://127.0.0.1:${port}`,
      warn: (msg: string) => warnings.push(msg),
    })

    const [first, second] = await Promise.allSettled([
      provider.getToken(12345),
      provider.getToken(12345),
    ])

    assert({
      given: "two concurrent getToken calls when exchange returns 500",
      should: "reject the first caller",
      actual: first.status,
      expected: "rejected",
    })

    assert({
      given: "two concurrent getToken calls when exchange returns 500",
      should: "reject the second caller",
      actual: second.status,
      expected: "rejected",
    })

    const firstMsg = first.status === "rejected" ? (first.reason as Error).message : ""
    const secondMsg = second.status === "rejected" ? (second.reason as Error).message : ""

    assert({
      given: "two concurrent getToken calls when exchange returns 500",
      should: "propagate the same error message to both callers",
      actual: firstMsg === secondMsg && firstMsg.includes("500"),
      expected: true,
    })

    assert({
      given: "two concurrent getToken calls when exchange returns 500",
      should: "make only one HTTP request (coalesced)",
      actual: captured.length,
      expected: 1,
    })
  } finally {
    await closeServer(server)
  }
})

describe("InstallationTokenProvider.getToken() — different installations run independently", async (assert) => {
  const { privateKey } = generateKeyPairSync("rsa", {
    modulusLength: 2048,
    publicKeyEncoding: { type: "spki", format: "pem" },
    privateKeyEncoding: { type: "pkcs8", format: "pem" },
  })

  const { server, port, captured } = await startMockGitHubPerInstallation(50)

  try {
    const provider = new InstallationTokenProvider({
      clientID: "Iv1.abc123",
      privateKeyPEM: privateKey as string,
      baseUrl: `http://127.0.0.1:${port}`,
    })

    const [tokenA, tokenB] = await Promise.all([provider.getToken(111), provider.getToken(222)])

    assert({
      given: "concurrent getToken calls for two different installation IDs",
      should: "make two separate HTTP requests",
      actual: captured.length,
      expected: 2,
    })

    const requestUrls = captured.map((r) => r.url).sort()

    assert({
      given: "concurrent getToken calls for installation 111 and 222",
      should: "send a request for installation 111",
      actual: requestUrls.some((u) => u?.includes("/installations/111/")),
      expected: true,
    })

    assert({
      given: "concurrent getToken calls for installation 111 and 222",
      should: "send a request for installation 222",
      actual: requestUrls.some((u) => u?.includes("/installations/222/")),
      expected: true,
    })

    assert({
      given: "concurrent getToken calls for two different installation IDs",
      should: "return a different token for each installation",
      actual: tokenA.token !== tokenB.token,
      expected: true,
    })

    assert({
      given: "concurrent getToken calls for installation 111",
      should: "return the token for installation 111",
      actual: tokenA.token,
      expected: "ghs_token_111",
    })

    assert({
      given: "concurrent getToken calls for installation 222",
      should: "return the token for installation 222",
      actual: tokenB.token,
      expected: "ghs_token_222",
    })
  } finally {
    await closeServer(server)
  }
})
