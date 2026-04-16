export function resolvePrivateKey(
  envKey: string | undefined,
  envPath: string | undefined,
  readFile: (path: string) => string,
): string {
  if (envKey !== undefined) {
    return envKey
  }
  if (envPath !== undefined) {
    return readFile(envPath)
  }
  throw new Error("No private key configured")
}
