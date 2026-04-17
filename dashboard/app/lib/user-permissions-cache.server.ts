/**
 * LRU cache for user permission results (accessible repo IDs).
 * TTL: 5 minutes per user token.
 * Max size: 1,000 entries with LRU eviction (Map insertion order used).
 */

export interface CacheEntry {
  userRepoIds: Set<number>
  fetchedAt: number // Date.now() timestamp
}

const MAX_ENTRIES = 1000
const TTL_MS = 5 * 60 * 1000 // 5 minutes

const cache = new Map<string, CacheEntry>()

/**
 * Pure: returns cached repo IDs for the given token if the entry exists and
 * is within TTL. Returns null on miss or expiry.
 */
export function getCachedUserRepoIds(userToken: string, now: number): Set<number> | null {
  const entry = cache.get(userToken)
  if (!entry) return null
  if (now - entry.fetchedAt > TTL_MS) {
    cache.delete(userToken)
    return null
  }
  return entry.userRepoIds
}

/**
 * I/O (side effect): stores repo IDs for a user token with a timestamp.
 * Evicts the oldest entry when the cache is at capacity.
 */
export function setCachedUserRepoIds(
  userToken: string,
  userRepoIds: Set<number>,
  now: number,
): void {
  if (cache.size >= MAX_ENTRIES) {
    const oldest = cache.keys().next().value
    if (oldest !== undefined) cache.delete(oldest)
  }
  cache.set(userToken, { userRepoIds, fetchedAt: now })
}

/**
 * Clears all cache entries. Used in tests to prevent cross-test interference.
 */
export function clearUserRepoIdsCache(): void {
  cache.clear()
}
