export function resolvePrivateKey(
  envKey: string | undefined,
  envPath: string | undefined,
  readFile: (path: string) => string,
): string {
  if (envKey !== undefined) {
    return envKey
  }
  if (envPath !== undefined) {
    try {
      return readFile(envPath)
    } catch {
      throw new Error(`Private key file not found: ${envPath}`)
    }
  }
  throw new Error("No private key configured")
}
