export interface RateLimitWarning {
  shouldWarn: boolean
  remaining: number
  limit: number
  resource: string
}

export function checkRateLimit(
  remaining: number | null,
  limit: number | null,
  resource: string,
): RateLimitWarning | null {
  if (remaining !== null && limit !== null && remaining / limit < 0.2) {
    return { shouldWarn: true, remaining, limit, resource }
  }
  return null
}
