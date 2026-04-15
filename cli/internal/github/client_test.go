package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	ghclient "github.com/mycargus/quarantine/cli/internal/github"
)

// --- GetRepo ---

func TestGetRepoSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":             1,
			"full_name":      "my-org/my-project",
			"default_branch": "main",
			"private":        false,
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	info, err := c.GetRepo(context.Background())

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /repos returns 200 with valid JSON",
		Should:   "return repo info without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "the GetRepo response",
		Should:   "parse the default_branch field",
		Actual:   info.DefaultBranch,
		Expected: "main",
	})
}

func TestGetRepo401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, err := c.GetRepo(context.Background())

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos returns 401",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos returns 401",
		Should:   "return an APIError with status 401",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 401,
		Expected: true,
	})
}

func TestGetRepo403(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, err := c.GetRepo(context.Background())

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos returns 403",
		Should:   "return an APIError with status 403",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 403,
		Expected: true,
	})
}

func TestGetRepo404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, err := c.GetRepo(context.Background())

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos returns 404",
		Should:   "return an APIError with status 404",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 404,
		Expected: true,
	})
}

func TestGetRepo500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, err := c.GetRepo(context.Background())

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos returns 500",
		Should:   "return an APIError with status 500",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 500,
		Expected: true,
	})
}

func TestGetRepoInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{bad json`))
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, err := c.GetRepo(context.Background())

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos returns 200 with invalid JSON",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- GetRef ---

func TestGetRef404ReturnsNoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	sha, exists, err := c.GetRef(context.Background(), "quarantine/state")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /git/ref returns 404 (branch does not exist)",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /git/ref returns 404",
		Should:   "return exists=false",
		Actual:   exists,
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GET /git/ref returns 404",
		Should:   "return empty SHA",
		Actual:   sha,
		Expected: "",
	})
}

func TestGetRefSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	sha, exists, err := c.GetRef(context.Background(), "quarantine/state")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /git/ref returns 200 with valid JSON",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /git/ref returns 200",
		Should:   "return exists=true",
		Actual:   exists,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GET /git/ref returns 200 with sha abc123",
		Should:   "return the SHA",
		Actual:   sha,
		Expected: "abc123",
	})
}

func TestGetRefUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, err := c.GetRef(context.Background(), "quarantine/state")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /git/ref returns 500",
		Should:   "return an APIError with status 500",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 500,
		Expected: true,
	})
}

// --- CreateRef ---

func TestCreateRefSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.CreateRef(context.Background(), "quarantine/state", "abc123sha")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "POST /git/refs returns 201",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
}

func TestCreateRefNonCreatedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.CreateRef(context.Background(), "quarantine/state", "abc123sha")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "POST /git/refs returns 409",
		Should:   "return an APIError with status 409",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 409,
		Expected: true,
	})
}

// --- GetRef invalid JSON ---

func TestGetRefInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{bad json`))
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, err := c.GetRef(context.Background(), "quarantine/state")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /git/ref returns 200 with invalid JSON body",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- doWithRetry: both attempts fail ---

// TestGetRepoNetworkErrorRetriesAndFails verifies the doWithRetry behavior when
// both the initial request and the retry fail with a network error.
// Note: this test takes ~2 seconds because doWithRetry sleeps between attempts.
func TestGetRepoNetworkErrorRetriesAndFails(t *testing.T) {
	// Start a server, capture its URL, then close it so every connection
	// attempt gets "connection refused" — simulating a persistent network failure.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	c := newTestClient(t, serverURL)
	_, err := c.GetRepo(context.Background())

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the GitHub API server is unreachable on both attempts",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the GitHub API server is unreachable on both attempts",
		Should:   "return an error containing 'GitHub API request failed'",
		Actual:   strings.Contains(err.Error(), "GitHub API request failed"),
		Expected: true,
	})
}

// --- PutContents ---

func TestPutContentsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.PutContents(context.Background(), "quarantine.json", "init", []byte(`{}`), "")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "PUT /contents returns 201",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
}

func TestPutContentsNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.PutContents(context.Background(), "quarantine.json", "init", []byte(`{}`), "")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT /contents returns 409",
		Should:   "return an APIError with status 409",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 409,
		Expected: true,
	})
}

func TestPutContentsWithoutSHAConflictsWhenFileAlreadyExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.PutContents(context.Background(), "quarantine.json", "init", []byte(`{}`), "")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PutContents called without sha and server responds with 409",
		Should:   "return a conflict error",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 409,
		Expected: true,
	})
}

func TestPutContentsWithValidSHASucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.PutContents(context.Background(), "quarantine.json", "update", []byte(`{}`), "abc123")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "PutContents called with a valid sha and server responds with 200",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
}
