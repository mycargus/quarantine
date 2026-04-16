export function resolvePrivateKey(
  envKey: string | undefined,
  envPath: string | undefined,
  readFile: (path: string) => string,
): string {
  if (envPath !== undefined) {
    try {
      return readFile(envPath)
    } catch {
      throw new Error(`Private key file not found: ${envPath}`)
    }
  }
  if (envKey !== undefined) {
    return envKey
  }
  throw new Error("No private key configured")
}
