package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	ghclient "github.com/mycargus/quarantine/internal/github"
)

// --- SearchClosedIssues ---

func TestSearchClosedIssuesReturnsClosedIssueNumbers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count":        2,
			"incomplete_results": false,
			"items": []map[string]interface{}{
				{"number": 42},
				{"number": 99},
			},
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	numbers, truncated, total, err := c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /search/issues returns 200 with 2 closed issues",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /search/issues returns 2 closed issues",
		Should:   "return 2 issue numbers",
		Actual:   len(numbers),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /search/issues returns issues 42 and 99",
		Should:   "return issue number 42 first",
		Actual:   numbers[0],
		Expected: 42,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns total_count=2 (not truncated)",
		Should:   "return truncated=false",
		Actual:   truncated,
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /search/issues returns total_count=2",
		Should:   "return total=2",
		Actual:   total,
		Expected: 2,
	})
}

func TestSearchClosedIssuesEmptyResultReturnsEmptySlice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count":        0,
			"incomplete_results": false,
			"items":              []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	numbers, truncated, total, err := c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /search/issues returns 0 closed issues",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /search/issues returns 0 issues",
		Should:   "return empty slice",
		Actual:   len(numbers),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns 0 issues",
		Should:   "return truncated=false",
		Actual:   truncated,
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /search/issues returns total_count=0",
		Should:   "return total=0",
		Actual:   total,
		Expected: 0,
	})
}

func TestSearchClosedIssuesTruncatedWhenTotalExceedsLimit(t *testing.T) {
	// Return total_count > 1000 but only 100 items (one page).
	items := make([]map[string]interface{}, 100)
	for i := 0; i < 100; i++ {
		items[i] = map[string]interface{}{"number": i + 1}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count":        1500,
			"incomplete_results": false,
			"items":              items,
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	numbers, truncated, total, err := c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /search/issues returns total_count=1500 (exceeds 1000 limit)",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns total_count=1500 (exceeds 1000 limit)",
		Should:   "return truncated=true",
		Actual:   truncated,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /search/issues returns total_count=1500",
		Should:   "return total=1500",
		Actual:   total,
		Expected: 1500,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns items from one page (100 items)",
		Should:   "return non-empty numbers slice",
		Actual:   len(numbers) > 0,
		Expected: true,
	})
}

func TestSearchClosedIssuesBuildsCorrectQueryString(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0,
			"items":       []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	// Verify the search query includes required filters.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "SearchClosedIssues called with label 'quarantine'",
		Should:   "request the /search/issues endpoint",
		Actual:   len(capturedURL) > 0 && contains(capturedURL, "/search/issues"),
		Expected: true,
	})
}

func TestSearchClosedIssues403ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, _, err := c.SearchClosedIssues(context.Background(), "quarantine")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns 403",
		Should:   "return an APIError with status 403",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 403,
		Expected: true,
	})
}

func TestSearchClosedIssuesPaginatesMultiplePages(t *testing.T) {
	// Simulate 2 pages: first page has 100 items, second page has 50 items.
	page1Items := make([]map[string]interface{}, 100)
	for i := 0; i < 100; i++ {
		page1Items[i] = map[string]interface{}{"number": i + 1}
	}
	page2Items := make([]map[string]interface{}, 50)
	for i := 0; i < 50; i++ {
		page2Items[i] = map[string]interface{}{"number": i + 101}
	}

	pageCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.Header().Set("Content-Type", "application/json")
		if pageCount == 1 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 150,
				"items":       page1Items,
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 150,
				"items":       page2Items,
			})
		}
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	numbers, truncated, total, err := c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /search/issues paginates 2 pages (100 + 50 items)",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /search/issues returns 150 total issues across 2 pages",
		Should:   "return 150 issue numbers",
		Actual:   len(numbers),
		Expected: 150,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns total_count=150 (within 1000 limit)",
		Should:   "return truncated=false",
		Actual:   truncated,
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /search/issues returns total_count=150",
		Should:   "return total=150",
		Actual:   total,
		Expected: 150,
	})
}

// contains is a helper used in query-string assertions.
func contains(s, substr string) bool {
	return fmt.Sprintf("%s", s) != "" && len(s) >= len(substr) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()
}
