package github_test

import (
	"context"
	"encoding/json"
	"errors"
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

// TestSearchClosedIssuesFetchesExactlyMaxPages verifies that exactly maxSearchPages
// (10) requests are made when every page returns a full 100 items.
// Kills mutations: `page <= maxSearchPages` → `< maxSearchPages` (9 pages),
// and `maxSearchPages = 10` → `11` (11 pages = one extra request).
func TestSearchClosedIssuesFetchesExactlyMaxPages(t *testing.T) {
	items100 := make([]map[string]interface{}, 100)
	for i := range items100 {
		items100[i] = map[string]interface{}{"number": i + 1}
	}

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 1000,
			"items":       items100,
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	numbers, _, _, err := c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "every page returns 100 items",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "10 pages × 100 items each (maxSearchPages=10)",
		Should:   "make exactly 10 HTTP requests",
		Actual:   requestCount,
		Expected: 10,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "10 pages × 100 items each",
		Should:   "return 1000 issue numbers",
		Actual:   len(numbers),
		Expected: 1000,
	})
}

// TestSearchClosedIssuesNotTruncatedAtExactly1000 verifies the boundary:
// total_count == 1000 is NOT truncated (> 1000, not >= 1000).
// Kills mutation: `seenTotal > 1000` → `seenTotal >= 1000`.
func TestSearchClosedIssuesNotTruncatedAtExactly1000(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 1000,
			"items":       []map[string]interface{}{{"number": 1}},
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, truncated, total, err := c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "total_count exactly equals 1000",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "total_count=1000",
		Should:   "return total=1000",
		Actual:   total,
		Expected: 1000,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "total_count=1000 (exactly at limit, not above)",
		Should:   "return truncated=false (> 1000, not >= 1000)",
		Actual:   truncated,
		Expected: false,
	})
}

// TestSearchClosedIssuesTruncatedAtJustAbove1000 verifies the other side:
// total_count == 1001 IS truncated.
// Kills mutation: `seenTotal > 1000` → `seenTotal > 1001`.
func TestSearchClosedIssuesTruncatedAtJustAbove1000(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 1001,
			"items":       []map[string]interface{}{{"number": 1}},
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, truncated, _, err := c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "total_count=1001 (just above limit)",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "total_count=1001 (just above 1000)",
		Should:   "return truncated=true",
		Actual:   truncated,
		Expected: true,
	})
}

// TestSearchClosedIssuesUsesPerPage100 verifies the per_page query parameter.
// Kills mutation: `"per_page": {"100"}` → `{"99"}`.
func TestSearchClosedIssuesUsesPerPage100(t *testing.T) {
	var capturedPerPage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPerPage = r.URL.Query().Get("per_page")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_count": 0,
			"items":       []interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, _, _ = c.SearchClosedIssues(context.Background(), "quarantine")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "SearchClosedIssues called",
		Should:   "request per_page=100 (not 99 or other value)",
		Actual:   capturedPerPage,
		Expected: "100",
	})
}
