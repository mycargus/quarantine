export type OAuthEnvInput = {
  clientId?: string
  clientSecret?: string
  origin?: string
}

export type OAuthConfig = {
  clientId: string
  clientSecret: string
  origin: string
}

const ENV_NAMES: Record<keyof OAuthEnvInput, string> = {
  clientId: "QUARANTINE_APP_CLIENT_ID",
  clientSecret: "QUARANTINE_APP_CLIENT_SECRET",
  origin: "QUARANTINE_APP_ORIGIN",
}

export function validateOAuthEnv(env: OAuthEnvInput): OAuthConfig {
  const missing: string[] = []

  for (const key of Object.keys(ENV_NAMES) as (keyof OAuthEnvInput)[]) {
    if (env[key] === undefined) {
      missing.push(ENV_NAMES[key])
    }
  }

  if (missing.length === 1) {
    throw new Error(`Missing required environment variable: ${missing[0]}`)
  }

  if (missing.length > 1) {
    throw new Error(`Missing required environment variables: ${missing.join(", ")}`)
  }

  return {
    clientId: env.clientId as string,
    clientSecret: env.clientSecret as string,
    origin: env.origin as string,
  }
}
