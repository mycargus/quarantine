export function resolvePrivateKey(
  envKey: string | undefined,
  _envPath: string | undefined,
  _readFile: (path: string) => string,
): string {
  if (envKey !== undefined) {
    return envKey
  }
  throw new Error("No private key configured")
}
