package github_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	ghclient "github.com/mycargus/quarantine/internal/github"
)

// --- GetContents ---

func TestGetContentsSuccess(t *testing.T) {
	rawContent := []byte(`{"version":1,"tests":{}}`)
	encoded := base64.StdEncoding.EncodeToString(rawContent)
	sha := "abc123sha"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content": encoded + "\n",
			"sha":     sha,
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	content, gotSHA, err := c.GetContents(context.Background(), "quarantine.json", "quarantine/state")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /contents returns 200 with base64-encoded content",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GET /contents returns 200 with base64-encoded content",
		Should:   "return decoded content",
		Actual:   string(content),
		Expected: string(rawContent),
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GET /contents returns 200 with sha abc123sha",
		Should:   "return the SHA",
		Actual:   gotSHA,
		Expected: sha,
	})
}

func TestGetContents404ReturnsEmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	content, sha, err := c.GetContents(context.Background(), "quarantine.json", "quarantine/state")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /contents returns 404",
		Should:   "return no error (treat as empty state)",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /contents returns 404",
		Should:   "return empty content",
		Actual:   len(content),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GET /contents returns 404",
		Should:   "return empty sha",
		Actual:   sha,
		Expected: "",
	})
}

func TestGetContents404OnMissingBranchReturnsNotInitializedError(t *testing.T) {
	// When the ref (branch) does not exist, GitHub returns 404 on the contents
	// endpoint with a specific message. We distinguish branch-not-found from
	// file-not-found by checking for the BranchNotFound error type.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "No commit found for the ref quarantine/state",
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, err := c.GetContents(context.Background(), "quarantine.json", "quarantine/state")

	// A 404 with a ref-not-found message should be treated as "not initialized"
	// and return a BranchNotFoundError.
	var branchErr *ghclient.BranchNotFoundError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /contents returns 404 with 'No commit found for the ref' message",
		Should:   "return a BranchNotFoundError",
		Actual:   errors.As(err, &branchErr),
		Expected: true,
	})
}

func TestGetContentsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, err := c.GetContents(context.Background(), "quarantine.json", "quarantine/state")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /contents returns 401",
		Should:   "return an APIError with status 401",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 401,
		Expected: true,
	})
}

// --- UpdateContents ---

func TestUpdateContentsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.UpdateContents(context.Background(), "quarantine.json", "quarantine/state", "update state", []byte(`{}`), "abc123")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "PUT /contents returns 200",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
}

func TestUpdateContentsCreatedSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.UpdateContents(context.Background(), "quarantine.json", "quarantine/state", "init state", []byte(`{}`), "")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "PUT /contents returns 201",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
}

func TestUpdateContents409ReturnsConflictError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.UpdateContents(context.Background(), "quarantine.json", "quarantine/state", "update", []byte(`{}`), "stale-sha")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT /contents returns 409 (CAS conflict)",
		Should:   "return an APIError with status 409",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 409,
		Expected: true,
	})
}

func TestUpdateContents422ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.UpdateContents(context.Background(), "quarantine.json", "quarantine/state", "update", []byte(`{}`), "sha")

	var apiErr *ghclient.APIError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT /contents returns 422",
		Should:   "return an APIError with status 422",
		Actual:   errors.As(err, &apiErr) && apiErr.StatusCode == 422,
		Expected: true,
	})
}


func TestGetContentsInvalidBase64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content": "!!!not-valid-base64!!!",
			"sha":     "abc123",
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, err := c.GetContents(context.Background(), "quarantine.json", "quarantine/state")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /contents returns 200 with invalid base64 content",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- Additional mutation-coverage tests ---

// TestGetContentsBase64WithNewlinesShouldDecode verifies that the '\n' stripping
// (line 97) uses '\n' not ' '. GitHub wraps base64 in newlines every ~60 chars.
// Kills mutation: `"\n"` → `" "`.
func TestGetContentsBase64WithNewlinesShouldDecode(t *testing.T) {
	rawContent := []byte(`{"version":1,"tests":{}}`)
	// Encode and inject newlines at arbitrary positions (as GitHub does).
	full := base64.StdEncoding.EncodeToString(rawContent)
	// Insert '\n' after every 20 chars to simulate GitHub's line-wrapping.
	var wrapped strings.Builder
	for i, ch := range full {
		if i > 0 && i%20 == 0 {
			wrapped.WriteRune('\n')
		}
		wrapped.WriteRune(ch)
	}
	wrapped.WriteRune('\n')

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content": wrapped.String(),
			"sha":     "abc",
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	content, _, err := c.GetContents(context.Background(), "quarantine.json", "branch")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "GET /contents returns base64 with embedded newlines (GitHub's format)",
		Should:   "decode successfully (newlines are stripped before base64 decode)",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "GET /contents returns base64 with embedded newlines",
		Should:   "return the correctly decoded content",
		Actual:   string(content),
		Expected: string(rawContent),
	})
}

// TestGetContents404BranchNotFoundIncludesRefName verifies that BranchNotFoundError
// contains the actual ref name (line 110: Branch: ref, not Branch: "").
// Kills mutation: `BranchNotFoundError{Branch: ref}` → `BranchNotFoundError{Branch: ""}`.
func TestGetContents404BranchNotFoundIncludesRefName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "No commit found for the ref quarantine/state",
		})
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, err := c.GetContents(context.Background(), "quarantine.json", "quarantine/state")

	var branchErr *ghclient.BranchNotFoundError
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /contents returns 404 with 'No commit found for the ref'",
		Should:   "return a BranchNotFoundError with the ref name 'quarantine/state'",
		Actual:   errors.As(err, &branchErr) && branchErr.Branch == "quarantine/state",
		Expected: true,
	})
}

// TestUpdateContentsSHAIncludedWhenNonEmpty verifies that the SHA is included
// in the request body when non-empty (line 41: sha != "").
// Kills mutation: `sha != ""` → `sha == ""`.
func TestUpdateContentsSHAIncludedWhenNonEmpty(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.UpdateContents(context.Background(), "quarantine.json", "quarantine/state", "update", []byte("{}"), "existing-sha")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "UpdateContents called with a non-empty SHA",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[interface{}]{
		Given:    "UpdateContents called with sha='existing-sha'",
		Should:   "include 'sha' field in request body",
		Actual:   capturedBody["sha"],
		Expected: "existing-sha",
	})
}

// TestUpdateContentsSHAOmittedWhenEmpty verifies that SHA is NOT included when empty
// (line 41: sha != "" guard). Kills the mutation's counterpart.
func TestUpdateContentsSHAOmittedWhenEmpty(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.UpdateContents(context.Background(), "quarantine.json", "quarantine/state", "create", []byte("{}"), "")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "UpdateContents called with empty SHA",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	_, hasSHA := capturedBody["sha"]
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "UpdateContents called with empty sha",
		Should:   "NOT include 'sha' field in request body (file creation, not update)",
		Actual:   hasSHA,
		Expected: false,
	})
}

// TestGetContentsPathWithLeadingSlashStripped verifies that leading '/' is stripped
// from paths before constructing the API URL (lines 50, 78: TrimPrefix).
// Kills mutation: remove TrimPrefix call.
func TestGetContentsPathWithLeadingSlashStripped(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _, _ = c.GetContents(context.Background(), "/quarantine.json", "branch")

	// Without TrimPrefix, the URL would be "...//quarantine.json?ref=..."
	// With TrimPrefix, it's ".../quarantine.json?ref=..."
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GetContents called with path '/quarantine.json' (leading slash)",
		Should:   "construct URL without double slash (leading slash stripped)",
		Actual:   !strings.Contains(capturedPath, "//"),
		Expected: true,
	})
}

// TestUpdateContentsPathWithLeadingSlashStripped verifies that leading '/' is
// stripped from paths in UpdateContents (line 50: TrimPrefix).
// Kills mutation: remove TrimPrefix → path includes leading '/' → double slash in URL.
func TestUpdateContentsPathWithLeadingSlashStripped(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_ = c.UpdateContents(context.Background(), "/quarantine.json", "quarantine/state", "update", []byte("{}"), "sha")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "UpdateContents called with path '/quarantine.json' (leading slash)",
		Should:   "construct URL without double slash (leading slash stripped)",
		Actual:   !strings.Contains(capturedPath, "//"),
		Expected: true,
	})
}
