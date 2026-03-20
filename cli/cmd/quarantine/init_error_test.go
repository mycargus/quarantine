package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 6: no GitHub token ---

func TestInitNoGitHubToken(t *testing.T) {
	dir := t.TempDir()
	// Create a fake git repo with a GitHub remote.
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	stdout, err := executeInitCmd(t,
		"jest\n\n\n", // framework=jest, retries=default, junitxml=default
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN": "",
			"GITHUB_TOKEN":            "",
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "neither QUARANTINE_GITHUB_TOKEN nor GITHUB_TOKEN set",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "neither QUARANTINE_GITHUB_TOKEN nor GITHUB_TOKEN set",
		Should:   "print error about missing token",
		Actual:   strings.Contains(stdout, "No GitHub token found"),
		Expected: true,
	})

	// Config file should have been created before GitHub operations.
	_, statErr := os.Stat(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "init fails at token validation",
		Should:   "have already created quarantine.yml (config written before GitHub ops)",
		Actual:   statErr == nil,
		Expected: true,
	})
}

// --- Scenario 8: not a git repository ---

func TestInitNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	// No .git directory.

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN": "ghp_test",
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no .git directory in current directory",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no .git directory in current directory",
		Should:   "print error about not being a git repository",
		Actual:   strings.Contains(stdout, "Not a git repository") || strings.Contains(stdout, "not a git repository"),
		Expected: true,
	})
}

// --- Scenario 9: non-GitHub remote ---

func TestInitNonGitHubRemote(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://gitlab.com/my-org/my-project.git")

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN": "ghp_test",
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "origin remote is a GitLab URL",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "origin remote is a GitLab URL",
		Should:   "print error about non-GitHub URL",
		Actual:   strings.Contains(stdout, "not a GitHub URL"),
		Expected: true,
	})
}

// --- Scenario 7b: unauthorized token (401) ---

func TestInitUnauthorizedToken(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/my-org/my-project", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_invalid",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos returns 401",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos returns 401",
		Should:   "print error about invalid or expired token",
		Actual:   strings.Contains(stdout, "invalid or expired"),
		Expected: true,
	})
}

// --- Scenario 7: insufficient token permissions ---

func TestInitForbiddenRepo(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Mock server that returns 403 for repo access.
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/my-org/my-project", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos returns 403",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos returns 403",
		Should:   "print error about insufficient permissions",
		Actual:   strings.Contains(stdout, "GitHub token lacks permission"),
		Expected: true,
	})
}

// --- Scenario 11: GitHub API unreachable ---

// TestInitAPIUnreachable verifies that when the GitHub API is unreachable the
// CLI retries once and then exits 2 with an actionable message.
// Note: this test takes ~2 seconds because doWithRetry sleeps between attempts.
func TestInitAPIUnreachable(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Start a server and close it immediately so every connection attempt
	// gets "connection refused" — simulating a network-level failure.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": serverURL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API server is unreachable (connection refused on both attempts)",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API server is unreachable",
		Should:   "print 'Unable to reach GitHub API'",
		Actual:   strings.Contains(stdout, "Unable to reach GitHub API"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub API server is unreachable",
		Should:   "print 'Check your network connection'",
		Actual:   strings.Contains(stdout, "Check your network connection"),
		Expected: true,
	})
}

// --- GetRef(quarantine/state) returns an unexpected error ---

func TestInitGetRefQuarantineStateError(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/my-org/my-project", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 1, "full_name": "my-org/my-project", "default_branch": "main", "private": false,
		})
	})
	mux.HandleFunc("/repos/my-org/my-project/git/ref/heads/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /git/ref/heads/quarantine/state returns 500",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /git/ref/heads/quarantine/state returns 500",
		Should:   "print error about failed branch status check",
		Actual:   strings.Contains(stdout, "failed to check branch status"),
		Expected: true,
	})
}

// --- GetRef(default branch) returns an unexpected error ---

func TestInitGetDefaultBranchRefError(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/my-org/my-project", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 1, "full_name": "my-org/my-project", "default_branch": "main", "private": false,
		})
	})
	mux.HandleFunc("/repos/my-org/my-project/git/ref/heads/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/heads/quarantine/state") {
			http.NotFound(w, r) // branch doesn't exist — proceed to create it
		} else {
			w.WriteHeader(http.StatusInternalServerError) // default branch SHA lookup fails
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /git/ref/heads/main returns 500",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /git/ref/heads/main returns 500",
		Should:   "print error about failed default branch SHA lookup",
		Actual:   strings.Contains(stdout, "failed to get default branch SHA"),
		Expected: true,
	})
}

// --- CreateRef returns an error ---

func TestInitCreateRefFailure(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/my-org/my-project", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 1, "full_name": "my-org/my-project", "default_branch": "main", "private": false,
		})
	})
	mux.HandleFunc("/repos/my-org/my-project/git/ref/heads/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/heads/quarantine/state") {
			http.NotFound(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ref":    "refs/heads/main",
				"object": map[string]interface{}{"sha": "basesha123", "type": "commit"},
			})
		}
	})
	mux.HandleFunc("/repos/my-org/my-project/git/refs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "POST /git/refs returns 500",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "POST /git/refs returns 500",
		Should:   "print error about failed branch creation",
		Actual:   strings.Contains(stdout, "failed to create branch"),
		Expected: true,
	})
}

// --- PutContents returns an error ---

func TestInitPutContentsFailure(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/my-org/my-project", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 1, "full_name": "my-org/my-project", "default_branch": "main", "private": false,
		})
	})
	mux.HandleFunc("/repos/my-org/my-project/git/ref/heads/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/heads/quarantine/state") {
			http.NotFound(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ref":    "refs/heads/main",
				"object": map[string]interface{}{"sha": "basesha123", "type": "commit"},
			})
		}
	})
	mux.HandleFunc("/repos/my-org/my-project/git/refs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/repos/my-org/my-project/contents/quarantine.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT /contents/quarantine.json returns 409",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT /contents/quarantine.json returns 409",
		Should:   "print error about failed quarantine.json write",
		Actual:   strings.Contains(stdout, "failed to write quarantine.json"),
		Expected: true,
	})
}
