export const DEBOUNCE_MS = 5 * 60 * 1000 // 5 minutes

/**
 * Pure: returns true if an on-demand pull should be triggered for a repo.
 * Pull is needed when lastPulledAt is null (never pulled) or was more than
 * debounceMs ago (stale). Strict greater-than: at exactly debounceMs, not yet stale.
 */
export function shouldPull(lastPulledAt: string | null, now: Date, debounceMs: number): boolean {
  if (lastPulledAt === null) return true
  const elapsed = now.getTime() - new Date(lastPulledAt).getTime()
  return elapsed > debounceMs
}
