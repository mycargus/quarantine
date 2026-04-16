import { generateJWT } from "./jwt.server.js"

export interface InstallationToken {
  token: string
  expiresAt: Date
}

export interface TokenProviderOptions {
  clientID: string
  privateKeyPEM: string
  baseUrl?: string
  warn?: (message: string) => void
}

export class InstallationTokenProvider {
  private clientID: string
  private privateKeyPEM: string
  private baseUrl: string
  private warn: (message: string) => void
  private cache: Map<number, InstallationToken> = new Map()
  private pending: Map<number, Promise<InstallationToken>> = new Map()

  constructor(options: TokenProviderOptions) {
    this.clientID = options.clientID
    this.privateKeyPEM = options.privateKeyPEM
    this.baseUrl = options.baseUrl ?? "https://api.github.com"
    this.warn = options.warn ?? console.warn
  }

  async getToken(installationId: number): Promise<InstallationToken> {
    const REFRESH_THRESHOLD_MS = 5 * 60 * 1000
    const cached = this.cache.get(installationId)
    if (cached && cached.expiresAt.getTime() - Date.now() > REFRESH_THRESHOLD_MS) {
      return cached
    }

    const inflight = this.pending.get(installationId)
    if (inflight) {
      return inflight
    }

    const exchangePromise = this.exchange(installationId)
    this.pending.set(installationId, exchangePromise)
    try {
      const token = await exchangePromise
      return token
    } finally {
      this.pending.delete(installationId)
    }
  }

  private async exchange(installationId: number): Promise<InstallationToken> {
    try {
      const jwt = generateJWT(this.clientID, this.privateKeyPEM, new Date())
      const url = `${this.baseUrl}/app/installations/${installationId}/access_tokens`
      const response = await fetch(url, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${jwt}`,
          Accept: "application/vnd.github+json",
        },
      })
      if (!response.ok) {
        throw new Error(`Token exchange failed: ${response.status}`)
      }
      const data = await response.json()
      const token: InstallationToken = {
        token: data.token,
        expiresAt: new Date(data.expires_at),
      }
      this.cache.set(installationId, token)
      return token
    } catch (error) {
      const detail = (error as Error).message
      const message = `[quarantine] Installation token exchange failed for installation ${installationId}: ${detail}`
      this.warn(message)
      throw new Error(`Installation token exchange failed: ${detail}`)
    }
  }
}
