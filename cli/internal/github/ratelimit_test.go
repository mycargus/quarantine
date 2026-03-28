package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Rate limit tracking ---

func TestRateLimitWarningFiredWhenRemainingBelow10Percent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate 5% remaining: 50 out of 1000.
		w.Header().Set("X-RateLimit-Limit", "1000")
		w.Header().Set("X-RateLimit-Remaining", "50")
		w.Header().Set("X-RateLimit-Reset", "1711000000")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0,
			"items":       []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	var warningMsg string
	c := newTestClient(t, server.URL)
	c.SetRateLimitWarningFunc(func(msg string) {
		warningMsg = msg
	})

	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "API response with X-RateLimit-Remaining=50 (5% of 1000)",
		Should:   "fire the rate limit warning callback",
		Actual:   warningMsg != "",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "API response with X-RateLimit-Remaining=50 (5% of 1000)",
		Should:   "include remaining count in warning message",
		Actual:   strings.Contains(warningMsg, "50"),
		Expected: true,
	})
}

func TestRateLimitWarningNotFiredWhenRemainingAbove10Percent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate 20% remaining: 200 out of 1000.
		w.Header().Set("X-RateLimit-Limit", "1000")
		w.Header().Set("X-RateLimit-Remaining", "200")
		w.Header().Set("X-RateLimit-Reset", "1711000000")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0,
			"items":       []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	warningFired := false
	c := newTestClient(t, server.URL)
	c.SetRateLimitWarningFunc(func(_ string) {
		warningFired = true
	})

	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "API response with X-RateLimit-Remaining=200 (20% of 1000)",
		Should:   "not fire the rate limit warning callback",
		Actual:   warningFired,
		Expected: false,
	})
}

func TestRateLimitWarningNotFiredWhenHeadersMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No rate limit headers.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0,
			"items":       []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	warningFired := false
	c := newTestClient(t, server.URL)
	c.SetRateLimitWarningFunc(func(_ string) {
		warningFired = true
	})

	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "API response with no X-RateLimit headers",
		Should:   "not fire the rate limit warning callback",
		Actual:   warningFired,
		Expected: false,
	})
}

// TestRateLimitWarningNotFiredWhenOnlyLimitHeaderPresent verifies that when
// only one of the two required headers is present, the warning doesn't fire.
// Kills mutation on line 221: `||` → `&&`.
func TestRateLimitWarningNotFiredWhenOnlyLimitHeaderPresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only X-RateLimit-Limit is set; X-RateLimit-Remaining is absent.
		w.Header().Set("X-RateLimit-Limit", "1000")
		// X-RateLimit-Remaining intentionally not set.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0, "items": []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	warningFired := false
	c := newTestClient(t, server.URL)
	c.SetRateLimitWarningFunc(func(_ string) { warningFired = true })
	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "only X-RateLimit-Limit header present, X-RateLimit-Remaining missing",
		Should:   "not fire the rate limit warning",
		Actual:   warningFired,
		Expected: false,
	})
}

// TestRateLimitWarningNotFiredWhenOnlyRemainingHeaderPresent verifies the
// symmetric case: only remaining set, limit missing.
// Also kills mutation on line 221.
func TestRateLimitWarningNotFiredWhenOnlyRemainingHeaderPresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only X-RateLimit-Remaining is set; X-RateLimit-Limit is absent.
		w.Header().Set("X-RateLimit-Remaining", "50")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0, "items": []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	warningFired := false
	c := newTestClient(t, server.URL)
	c.SetRateLimitWarningFunc(func(_ string) { warningFired = true })
	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "only X-RateLimit-Remaining header present, X-RateLimit-Limit missing",
		Should:   "not fire the rate limit warning",
		Actual:   warningFired,
		Expected: false,
	})
}

// TestRateLimitWarningNotFiredWhenLimitIsZero verifies that limit=0 triggers
// the early return (line 225: `err != nil || limit == 0`).
// Kills mutation on line 225: `||` → `&&`.
func TestRateLimitWarningNotFiredWhenLimitIsZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "0")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0, "items": []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	warningFired := false
	c := newTestClient(t, server.URL)
	c.SetRateLimitWarningFunc(func(_ string) { warningFired = true })
	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "X-RateLimit-Limit=0 (division by zero guard)",
		Should:   "not fire the rate limit warning",
		Actual:   warningFired,
		Expected: false,
	})
}

// TestRateLimitWarningNotFiredAtExactly10Percent verifies the boundary.
// With remaining=100 and limit=1000, remaining*10 == limit → not < limit.
// Kills mutation on line 232: `remaining*10 < limit` → `<= limit`.
func TestRateLimitWarningNotFiredAtExactly10Percent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Exactly 10%: 100 out of 1000.
		w.Header().Set("X-RateLimit-Limit", "1000")
		w.Header().Set("X-RateLimit-Remaining", "100")
		w.Header().Set("X-RateLimit-Reset", "1711000000")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0, "items": []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	warningFired := false
	c := newTestClient(t, server.URL)
	c.SetRateLimitWarningFunc(func(_ string) { warningFired = true })
	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "remaining=100 limit=1000 (exactly 10% — not below threshold)",
		Should:   "NOT fire the rate limit warning at exactly the boundary",
		Actual:   warningFired,
		Expected: false,
	})
}

// TestRateLimitWarningFiredJustBelow10Percent verifies the boundary counterpart.
// With remaining=99 and limit=1000, 99*10=990 < 1000 → fires.
func TestRateLimitWarningFiredJustBelow10Percent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "1000")
		w.Header().Set("X-RateLimit-Remaining", "99")
		w.Header().Set("X-RateLimit-Reset", "1711000000")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0, "items": []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	warningFired := false
	c := newTestClient(t, server.URL)
	c.SetRateLimitWarningFunc(func(_ string) { warningFired = true })
	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "remaining=99 limit=1000 (just below 10%)",
		Should:   "fire the rate limit warning",
		Actual:   warningFired,
		Expected: true,
	})
}

// TestRateLimitWarningFormatsResetTimeFromUnixTimestamp verifies that a valid
// Unix timestamp in X-RateLimit-Reset is formatted as HH:MM:SS.
// Kills mutation on line 234: `parseErr == nil` → `parseErr != nil`.
func TestRateLimitWarningFormatsResetTimeFromUnixTimestamp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "1000")
		w.Header().Set("X-RateLimit-Remaining", "50")
		// Unix timestamp 0 = "00:00:00" UTC.
		w.Header().Set("X-RateLimit-Reset", "0")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0, "items": []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	var warningMsg string
	c := newTestClient(t, server.URL)
	c.SetRateLimitWarningFunc(func(msg string) { warningMsg = msg })
	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	// When parseErr == nil (original), resetTime is formatted as "00:00:00".
	// When parseErr != nil (mutation), resetTime falls back to raw "0".
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "X-RateLimit-Reset=0 (valid Unix timestamp)",
		Should:   "format reset time as HH:MM:SS (not the raw '0')",
		Actual:   strings.Contains(warningMsg, ":") && !strings.Contains(warningMsg, "resets at 0 UTC"),
		Expected: true,
	})
}
