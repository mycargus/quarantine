export function shouldSyncInstallations(
  lastSyncedAt: Date | null,
  now: Date,
  intervalMs: number,
): boolean {
  if (lastSyncedAt === null) return true
  return now.getTime() - lastSyncedAt.getTime() > intervalMs
}
