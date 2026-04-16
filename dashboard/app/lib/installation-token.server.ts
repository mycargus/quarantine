import { generateJWT } from "./jwt.server.js"

export interface InstallationToken {
  token: string
  expiresAt: Date
}

export interface TokenProviderOptions {
  clientID: string
  privateKeyPEM: string
  baseUrl?: string
}

export class InstallationTokenProvider {
  private clientID: string
  private privateKeyPEM: string
  private baseUrl: string

  constructor(options: TokenProviderOptions) {
    this.clientID = options.clientID
    this.privateKeyPEM = options.privateKeyPEM
    this.baseUrl = options.baseUrl ?? "https://api.github.com"
  }

  async getToken(installationId: number): Promise<InstallationToken> {
    const jwt = generateJWT(this.clientID, this.privateKeyPEM, new Date())
    const url = `${this.baseUrl}/app/installations/${installationId}/access_tokens`
    const response = await fetch(url, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${jwt}`,
        Accept: "application/vnd.github+json",
      },
    })
    const data = await response.json()
    return {
      token: data.token,
      expiresAt: new Date(data.expires_at),
    }
  }
}
