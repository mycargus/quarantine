package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
		Actual:   fmt.Sprintf("%s", warningMsg) != "" && containsStr(warningMsg, "50"),
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

// containsStr checks if s contains substr.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
