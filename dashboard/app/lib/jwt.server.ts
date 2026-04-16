import { createSign } from "node:crypto"

function base64url(data: string): string {
  return Buffer.from(data).toString("base64url")
}

export function generateJWT(clientID: string, privateKeyPEM: string, now: Date): string {
  if (clientID === "") {
    throw new Error("Client ID must not be empty")
  }
  if (privateKeyPEM === "") {
    throw new Error("Private key must not be empty")
  }
  if (privateKeyPEM.includes("PUBLIC KEY")) {
    throw new Error("Expected an RSA private key, got a public key")
  }

  const header = JSON.stringify({ alg: "RS256", typ: "JWT" })
  const nowSeconds = Math.floor(now.getTime() / 1000)
  const payload = JSON.stringify({
    iss: clientID,
    iat: nowSeconds - 60,
    exp: nowSeconds + 540,
  })

  const signingInput = `${base64url(header)}.${base64url(payload)}`

  const signer = createSign("RSA-SHA256")
  signer.update(signingInput)

  try {
    const signature = signer.sign(privateKeyPEM, "base64url")
    return `${signingInput}.${signature}`
  } catch {
    throw new Error("Invalid private key: not a valid PEM-encoded RSA private key")
  }
}
