import { createVerify, generateKeyPairSync } from "node:crypto"
import { describe } from "riteway"
import { generateJWT } from "./jwt.server.js"

const { privateKey, publicKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
  publicKeyEncoding: { type: "spki", format: "pem" },
  privateKeyEncoding: { type: "pkcs8", format: "pem" },
})

const clientID = "Iv1.abc123def456"
const now = new Date("2026-03-15T12:00:00.000Z")
const nowUnix = Math.floor(now.getTime() / 1000)

function decodeJWTPart(part: string): Record<string, unknown> {
  const padded = part.replace(/-/g, "+").replace(/_/g, "/")
  return JSON.parse(Buffer.from(padded, "base64").toString("utf-8"))
}

describe("generateJWT()", async (assert) => {
  const jwt = generateJWT(clientID, privateKey, now)
  const parts = jwt.split(".")
  const header = decodeJWTPart(parts[0])
  const payload = decodeJWTPart(parts[1])

  assert({
    given: "a valid client ID and RSA private key",
    should: "return a JWT with three dot-separated parts",
    actual: parts.length,
    expected: 3,
  })

  assert({
    given: "a valid client ID and RSA private key",
    should: "set the header alg to RS256",
    actual: header.alg,
    expected: "RS256",
  })

  assert({
    given: "a valid client ID and RSA private key",
    should: "set the header typ to JWT",
    actual: header.typ,
    expected: "JWT",
  })

  assert({
    given: "a valid client ID and RSA private key",
    should: "set iss equal to the client ID",
    actual: payload.iss,
    expected: clientID,
  })

  assert({
    given: "a valid client ID and RSA private key",
    should: "set iat to now minus 60 seconds",
    actual: payload.iat,
    expected: nowUnix - 60,
  })

  assert({
    given: "a valid client ID and RSA private key",
    should: "set exp to now plus 9 minutes (540 seconds)",
    actual: payload.exp,
    expected: nowUnix + 540,
  })

  // Verify the signature is valid using the public key
  const signatureInput = `${parts[0]}.${parts[1]}`
  const signature = Buffer.from(parts[2].replace(/-/g, "+").replace(/_/g, "/"), "base64")
  const verifier = createVerify("RSA-SHA256")
  verifier.update(signatureInput)
  const isValid = verifier.verify(publicKey, signature)

  assert({
    given: "a valid client ID and RSA private key",
    should: "produce a signature verifiable with the corresponding public key",
    actual: isValid,
    expected: true,
  })
})

const thrownMessage = (fn: () => unknown): string | null => {
  try {
    fn()
    return null
  } catch (e) {
    return e instanceof Error ? e.message : String(e)
  }
}

describe("generateJWT() — invalid private key", async (assert) => {
  assert({
    given: "a public key instead of a private key",
    should: "throw an error mentioning public key",
    actual: thrownMessage(() => generateJWT(clientID, publicKey, now)),
    expected: "Expected an RSA private key, got a public key",
  })

  assert({
    given: "a malformed string that is not a PEM key",
    should: "throw a descriptive error about the invalid key",
    actual: thrownMessage(() => generateJWT(clientID, "not-a-pem", now)),
    expected: "Invalid private key: not a valid PEM-encoded RSA private key",
  })

  assert({
    given: "an empty string as the private key",
    should: "throw an error that the key must not be empty",
    actual: thrownMessage(() => generateJWT(clientID, "", now)),
    expected: "Private key must not be empty",
  })
})
