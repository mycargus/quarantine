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

// --- SearchOpenIssue ---

func TestSearchOpenIssueFound(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 1,
				"items": []map[string]interface{}{
					{"number": 42, "html_url": "https://github.com/my-org/my-project/issues/42"},
				},
			})
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	number, url, found, err := c.SearchOpenIssue(
		context.Background(), "abc123",
	)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /search/issues returns 200 with total_count=1 and one item",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns total_count=1 and one item",
		Should:   "return found=true",
		Actual:   found,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /search/issues returns issue number 42",
		Should:   "return issue number 42",
		Actual:   number,
		Expected: 42,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GET /search/issues returns html_url for issue 42",
		Should:   "return the issue URL",
		Actual:   url,
		Expected: "https://github.com/my-org/my-project/issues/42",
	})
}

func TestSearchOpenIssueNotFound(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	number, url, found, err := c.SearchOpenIssue(
		context.Background(), "abc123",
	)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /search/issues returns 200 with total_count=0 and empty items",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns total_count=0 and empty items",
		Should:   "return found=false",
		Actual:   found,
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /search/issues returns no issues",
		Should:   "return issue number 0",
		Actual:   number,
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GET /search/issues returns no issues",
		Should:   "return empty URL",
		Actual:   url,
		Expected: "",
	})
}

// TestSearchOpenIssueTotalCountZeroNotFound kills the condition mutation
// `TotalCount > 0 && len(Items) > 0` → `||`. With `||`, a response where
// total_count=0 but items is non-empty would return found=true. This test
// ensures that total_count=0 always means not found.
func TestSearchOpenIssueTotalCountZeroNotFound(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// total_count=0 but items has one entry (inconsistent GitHub response)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items": []map[string]interface{}{
					{
						"number":   7,
						"html_url": "https://github.com/my-org/my-project/issues/7",
					},
				},
			})
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, found, err := c.SearchOpenIssue(context.Background(), "abc123")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /search/issues returns total_count=0 but non-empty items",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns total_count=0 but non-empty items",
		Should:   "return found=false because total_count=0",
		Actual:   found,
		Expected: false,
	})
}

// TestSearchOpenIssueItemsEmptyNotFound kills the condition mutation
// `TotalCount > 0 && len(Items) > 0` → `||`. With `||`, a response where
// total_count>0 but items is empty would return found=true. This test
// ensures that empty items always means not found.
func TestSearchOpenIssueItemsEmptyNotFound(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// total_count=1 but items is empty (inconsistent GitHub response)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 1,
				"items":       []interface{}{},
			})
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, found, err := c.SearchOpenIssue(context.Background(), "abc123")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /search/issues returns total_count=1 but empty items slice",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns total_count=1 but empty items slice",
		Should:   "return found=false because items is empty",
		Actual:   found,
		Expected: false,
	})
}

func TestSearchOpenIssueNonOKStatus(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, _, err := c.SearchOpenIssue(context.Background(), "abc123")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /search/issues returns 403",
		Should:   "return an APIError with status 403",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 403,
		Expected: true,
	})
}

// --- CreateIssue ---

func TestCreateIssueSuccess201(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   101,
				"html_url": "https://github.com/my-org/my-project/issues/101",
			})
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	number, url, err := c.CreateIssue(
		context.Background(), "Flaky test", "body text", []string{"quarantine"},
	)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "POST /issues returns 201 with valid JSON",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "POST /issues returns issue number 101",
		Should:   "return issue number 101",
		Actual:   number,
		Expected: 101,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "POST /issues returns html_url for issue 101",
		Should:   "return the issue URL",
		Actual:   url,
		Expected: "https://github.com/my-org/my-project/issues/101",
	})
}

func TestCreateIssueSuccess200(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   202,
				"html_url": "https://github.com/my-org/my-project/issues/202",
			})
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	number, url, err := c.CreateIssue(
		context.Background(), "Flaky test", "body text", []string{"quarantine"},
	)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "POST /issues returns 200 with valid JSON",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "POST /issues returns 200 with issue number 202",
		Should:   "return issue number 202",
		Actual:   number,
		Expected: 202,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "POST /issues returns 200 with html_url for issue 202",
		Should:   "return the issue URL",
		Actual:   url,
		Expected: "https://github.com/my-org/my-project/issues/202",
	})
}

func TestCreateIssueNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, err := c.CreateIssue(
		context.Background(), "Flaky test", "body text", []string{"quarantine"},
	)

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "POST /issues returns 422",
		Should:   "return an APIError with status 422",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 422,
		Expected: true,
	})
}

// --- ListPRComments ---

func TestListPRCommentsSuccess(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1001, "body": "first comment"},
				{"id": 1002, "body": "second comment"},
			})
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	comments, err := c.ListPRComments(context.Background(), 7)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /issues/7/comments returns 200 with 2 comments",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /issues/7/comments returns 2 comments",
		Should:   "return 2 comments",
		Actual:   len(comments),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[int64]{
		Given:    "GET /issues/7/comments returns first comment with id=1001",
		Should:   "return first comment ID as 1001",
		Actual:   comments[0].ID,
		Expected: 1001,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GET /issues/7/comments returns first comment with body 'first comment'",
		Should:   "return first comment body",
		Actual:   comments[0].Body,
		Expected: "first comment",
	})
}

func TestListPRCommentsNonOKStatus(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, err := c.ListPRComments(context.Background(), 7)

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /issues/7/comments returns 404",
		Should:   "return an APIError with status 404",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 404,
		Expected: true,
	})
}

// --- CreatePRComment ---

func TestCreatePRCommentSuccess201(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.CreatePRComment(context.Background(), 7, "hello from quarantine")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "POST /issues/7/comments returns 201",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
}

func TestCreatePRCommentSuccess200(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.CreatePRComment(context.Background(), 7, "hello from quarantine")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "POST /issues/7/comments returns 200",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
}

func TestCreatePRCommentNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.CreatePRComment(context.Background(), 7, "hello from quarantine")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "POST /issues/7/comments returns 403",
		Should:   "return an APIError with status 403",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 403,
		Expected: true,
	})
}

// --- UpdatePRComment ---

func TestUpdatePRCommentSuccess(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.UpdatePRComment(context.Background(), 9001, "updated body")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "PATCH /issues/comments/9001 returns 200",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
}

func TestUpdatePRCommentNonOKStatus(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
	)
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.UpdatePRComment(context.Background(), 9001, "updated body")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PATCH /issues/comments/9001 returns 404",
		Should:   "return an APIError with status 404",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 404,
		Expected: true,
	})
}
