/**
 * Unit tests for user-permissions-cache module.
 * Tests cache hit, TTL expiry, and LRU eviction at capacity.
 */

import { describe } from "riteway"
import {
  clearUserRepoIdsCache,
  getCachedUserRepoIds,
  setCachedUserRepoIds,
} from "./user-permissions-cache.server.js"

const TTL_MS = 5 * 60 * 1000
const MAX_ENTRIES = 1000

describe("getCachedUserRepoIds — miss when cache is empty", async (assert) => {
  clearUserRepoIdsCache()
  assert({
    given: "no entry in cache for token",
    should: "return null",
    actual: getCachedUserRepoIds("ghu_token_a", 0),
    expected: null,
  })
})

describe("getCachedUserRepoIds — hit within TTL", async (assert) => {
  clearUserRepoIdsCache()
  const ids = new Set([1, 2, 3])
  setCachedUserRepoIds("ghu_token_b", ids, 0)
  assert({
    given: "a valid cache entry and now is within 5-minute TTL",
    should: "return the cached Set",
    actual: getCachedUserRepoIds("ghu_token_b", TTL_MS - 1),
    expected: ids,
  })
})

describe("getCachedUserRepoIds — miss after TTL expires", async (assert) => {
  clearUserRepoIdsCache()
  const ids = new Set([4, 5])
  setCachedUserRepoIds("ghu_token_c", ids, 0)
  assert({
    given: "a cache entry and now is past the 5-minute TTL",
    should: "return null (entry expired)",
    actual: getCachedUserRepoIds("ghu_token_c", TTL_MS + 1),
    expected: null,
  })
})

describe("setCachedUserRepoIds — LRU eviction at MAX_ENTRIES capacity", async (assert) => {
  clearUserRepoIdsCache()
  // Fill cache to capacity with tokens "t0" through "t999"
  for (let i = 0; i < MAX_ENTRIES; i++) {
    setCachedUserRepoIds(`t${i}`, new Set([i]), 0)
  }
  // Insert one more entry — should evict "t0" (oldest)
  setCachedUserRepoIds("t_new", new Set([9999]), 0)

  assert({
    given: "cache at MAX_ENTRIES capacity when a new entry is inserted",
    should: "evict the oldest entry (t0)",
    actual: getCachedUserRepoIds("t0", 0),
    expected: null,
  })

  assert({
    given: "cache at MAX_ENTRIES capacity when a new entry is inserted",
    should: "keep the new entry (t_new)",
    actual: getCachedUserRepoIds("t_new", 0),
    expected: new Set([9999]),
  })
})
