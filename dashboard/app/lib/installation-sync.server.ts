export function shouldSyncInstallations(
  lastSyncedAt: Date | null,
  _now: Date,
  _intervalMs: number,
): boolean {
  return lastSyncedAt === null
}
