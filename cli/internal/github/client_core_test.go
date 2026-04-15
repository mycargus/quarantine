package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	riteway "github.com/mycargus/riteway-golang"

	ghclient "github.com/mycargus/quarantine/cli/internal/github"
)

// newTestClient creates a Client pointed at the given test server URL.
func newTestClient(t *testing.T, serverURL string) *ghclient.Client {
	t.Helper()
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "ghp_test")
	t.Setenv("QUARANTINE_GITHUB_API_BASE_URL", serverURL)
	c, err := ghclient.NewClient("my-org", "my-project")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetRetryDelay(0)
	return c
}

// --- Default field values from NewClient ---

func TestNewClientHasDefaultRetryDelay(t *testing.T) {
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "ghp_test")

	c, err := ghclient.NewClient("owner", "repo")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a freshly created client",
		Should:   "have default retry delay of 2 seconds",
		Actual:   c.RetryDelay(),
		Expected: 2 * time.Second,
	})
}

// --- Token resolution via NewClient ---

func TestNewClientUsesQUARANTINE_GITHUB_TOKEN(t *testing.T) {
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "ghp_primary")
	t.Setenv("GITHUB_TOKEN", "")

	_, err := ghclient.NewClient("owner", "repo")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "QUARANTINE_GITHUB_TOKEN is set and GITHUB_TOKEN is empty",
		Should:   "create client without error",
		Actual:   err,
		Expected: nil,
	})
}

func TestNewClientFallsBackToGITHUB_TOKEN(t *testing.T) {
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "ghp_fallback")

	_, err := ghclient.NewClient("owner", "repo")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "QUARANTINE_GITHUB_TOKEN is empty and GITHUB_TOKEN is set",
		Should:   "create client without error (fallback to GITHUB_TOKEN)",
		Actual:   err,
		Expected: nil,
	})
}

func TestNewClientSucceedsWhenBothTokensSet(t *testing.T) {
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "ghp_primary")
	t.Setenv("GITHUB_TOKEN", "ghp_fallback")

	_, err := ghclient.NewClient("owner", "repo")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "both QUARANTINE_GITHUB_TOKEN and GITHUB_TOKEN are set",
		Should:   "create client without error",
		Actual:   err,
		Expected: nil,
	})
}

func TestNewClientReturnsErrorWhenNoToken(t *testing.T) {
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	_, err := ghclient.NewClient("owner", "repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "neither QUARANTINE_GITHUB_TOKEN nor GITHUB_TOKEN is set",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- Constant value tests ---

// TestClientSendsCorrectUserAgentHeader verifies the userAgent constant is sent
// in every request. Kills mutation: `userAgent = "quarantine-cli/0.1.0"` → `""`.
func TestClientSendsCorrectUserAgentHeader(t *testing.T) {
	var capturedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":1,"full_name":"my-org/my-project","default_branch":"main","private":false}`)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	_, _ = c.GetRepo(context.Background())

	riteway.Assert(t, riteway.Case[string]{
		Given:    "any API request",
		Should:   "set User-Agent to 'quarantine-cli/0.1.0'",
		Actual:   capturedUA,
		Expected: "quarantine-cli/0.1.0",
	})
}

// TestClientUsesDefaultBaseURLWhenNoOverride verifies the defaultBaseURL constant
// is "https://api.github.com". When QUARANTINE_GITHUB_API_BASE_URL is unset,
// requests should target api.github.com.
// Kills mutation: `defaultBaseURL = "https://api.github.com"` → `""`.
func TestClientUsesDefaultBaseURLWhenNoOverride(t *testing.T) {
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "ghp_test")
	t.Setenv("QUARANTINE_GITHUB_API_BASE_URL", "") // explicitly clear override

	c, err := ghclient.NewClient("my-org", "my-project")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// Make an API call — it will fail (no real GitHub), but the error must NOT
	// be "unsupported protocol scheme", which is what Go returns when the URL
	// has no scheme (i.e. when defaultBaseURL = "").
	// With correct defaultBaseURL, the request goes to https://api.github.com
	// and fails with an auth or connection error (not a scheme error).
	_, err = c.GetRepo(context.Background())

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "QUARANTINE_GITHUB_API_BASE_URL is unset and defaultBaseURL = 'https://api.github.com'",
		Should:   "NOT get 'unsupported protocol scheme' error (proving a valid base URL was used)",
		Actual:   err != nil && !strings.Contains(err.Error(), "unsupported protocol scheme"),
		Expected: true,
	})
}

// --- PutContents SHA conditional ---

// TestPutContentsSHAIncludedWhenNonEmpty verifies that SHA is included in the
// request body when non-empty (line 140: sha != "").
// Kills mutation: `sha != ""` → `sha == ""`.
func TestPutContentsSHAIncludedWhenNonEmpty(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.PutContents(context.Background(), "quarantine.json", "init", []byte("{}"), "existing-sha")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "PutContents called with a non-empty SHA",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[interface{}]{
		Given:    "PutContents called with sha='existing-sha'",
		Should:   "include 'sha' field in request body",
		Actual:   capturedBody["sha"],
		Expected: "existing-sha",
	})
}

// TestPutContentsSHAOmittedWhenEmpty verifies that SHA is omitted when empty.
func TestPutContentsSHAOmittedWhenEmpty(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server.URL)
	err := c.PutContents(context.Background(), "quarantine.json", "init", []byte("{}"), "")

	riteway.Assert(t, riteway.Case[error]{
		Given:    "PutContents called with empty SHA (new file creation)",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})

	_, hasSHA := capturedBody["sha"]
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PutContents called with empty SHA",
		Should:   "NOT include 'sha' field in request body",
		Actual:   hasSHA,
		Expected: false,
	})
}
