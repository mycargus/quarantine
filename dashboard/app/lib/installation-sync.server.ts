export type AppCredentialInput = {
  clientId?: string
  privateKey?: string
}

export type AppCredentials = {
  clientId: string
  privateKey: string
}

const CREDENTIAL_ENV_NAMES: Record<keyof AppCredentialInput, string> = {
  clientId: "QUARANTINE_APP_CLIENT_ID",
  privateKey: "QUARANTINE_APP_PRIVATE_KEY",
}

export function validateAppCredentials(env: AppCredentialInput): AppCredentials {
  const missing: string[] = []
  const blank: string[] = []

  for (const key of Object.keys(CREDENTIAL_ENV_NAMES) as (keyof AppCredentialInput)[]) {
    const value = env[key]
    if (value === undefined) {
      missing.push(CREDENTIAL_ENV_NAMES[key])
    } else if (value.trim() === "") {
      blank.push(CREDENTIAL_ENV_NAMES[key])
    }
  }

  if (blank.length > 0) {
    throw new Error(
      blank.map((name) => `${name} is set but blank`).join("; "),
    )
  }

  if (missing.length === 1) {
    throw new Error(`Missing required environment variable: ${missing[0]}`)
  }

  if (missing.length > 1) {
    throw new Error(`Missing required environment variables: ${missing.join(", ")}`)
  }

  return {
    clientId: env.clientId as string,
    privateKey: env.privateKey as string,
  }
}

export function shouldSyncInstallations(
  lastSyncedAt: Date | null,
  now: Date,
  intervalMs: number,
): boolean {
  if (lastSyncedAt === null) return true
  return now.getTime() - lastSyncedAt.getTime() > intervalMs
}
