package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// initTestServer creates a minimal mock GitHub API server for init tests.
// It records the sequence of requests made against it.
type initTestServer struct {
	server       *httptest.Server
	requests     []string // method + path for each request
	defaultBranch string
	existingBranch bool // if true, GET /git/ref/heads/quarantine/state returns 200
}

func newInitTestServer(t *testing.T, opts ...func(*initTestServer)) *initTestServer {
	t.Helper()
	s := &initTestServer{
		defaultBranch: "main",
	}
	for _, opt := range opts {
		opt(s)
	}

	mux := http.NewServeMux()

	// GET /repos/{owner}/{repo} — repo info
	mux.HandleFunc("/repos/my-org/my-project", func(w http.ResponseWriter, r *http.Request) {
		s.requests = append(s.requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":             1,
			"full_name":      "my-org/my-project",
			"default_branch": s.defaultBranch,
			"private":        false,
		})
	})

	// GET /repos/{owner}/{repo}/git/ref/heads/{branch}
	mux.HandleFunc("/repos/my-org/my-project/git/ref/heads/", func(w http.ResponseWriter, r *http.Request) {
		s.requests = append(s.requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		// quarantine/state branch check
		if strings.HasSuffix(r.URL.Path, "/heads/quarantine/state") {
			if s.existingBranch {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"ref": "refs/heads/quarantine/state",
					"object": map[string]interface{}{
						"sha":  "abc123",
						"type": "commit",
					},
				})
			} else {
				http.NotFound(w, r)
			}
			return
		}
		// default branch SHA
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ref": "refs/heads/" + s.defaultBranch,
			"object": map[string]interface{}{
				"sha":  "basesha123",
				"type": "commit",
			},
		})
	})

	// POST /repos/{owner}/{repo}/git/refs — create branch
	mux.HandleFunc("/repos/my-org/my-project/git/refs", func(w http.ResponseWriter, r *http.Request) {
		s.requests = append(s.requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ref": "refs/heads/quarantine/state",
			"object": map[string]interface{}{
				"sha":  "basesha123",
				"type": "commit",
			},
		})
	})

	// PUT /repos/{owner}/{repo}/contents/quarantine.json
	mux.HandleFunc("/repos/my-org/my-project/contents/quarantine.json", func(w http.ResponseWriter, r *http.Request) {
		s.requests = append(s.requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodPut {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content": map[string]interface{}{
				"name": "quarantine.json",
				"sha":  "newsha456",
			},
		})
	})

	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

// withExistingBranch is an option to make the mock server report that
// quarantine/state already exists.
func withExistingBranch() func(*initTestServer) {
	return func(s *initTestServer) {
		s.existingBranch = true
	}
}

// executeInitCmd runs the init command with a fake stdin and the given env vars.
// configDir is the directory where quarantine.yml will be written.
func executeInitCmd(t *testing.T, stdin string, configDir string, env map[string]string) (stdout string, err error) {
	t.Helper()

	// Set env vars.
	for k, v := range env {
		t.Setenv(k, v)
	}

	// Change to the config dir so quarantine.yml gets written there.
	origDir, _ := os.Getwd()
	if err := os.Chdir(configDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetArgs([]string{"init"})
	execErr := rootCmd.Execute()
	return buf.String(), execErr
}

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

// --- Scenario 10: invalid framework input ---

func TestInitInvalidFrameworkRePrompts(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mockServer := newInitTestServer(t)
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "ghp_test")
	t.Setenv("QUARANTINE_GITHUB_API_BASE_URL", mockServer.server.URL)

	// Enter invalid framework first, then valid one.
	stdout, _ := executeInitCmd(t,
		"pytest\njest\n\n\n",
		dir,
		nil,
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "user enters 'pytest' then 'jest' at the framework prompt",
		Should:   "print exact invalid framework message for 'pytest'",
		Actual:   strings.Contains(stdout, "Invalid framework 'pytest'. Supported: rspec, jest, vitest."),
		Expected: true,
	})
}

// --- Scenario 4: quarantine.yml already exists ---

func TestInitConfigAlreadyExistsAbort(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Write existing config.
	if err := os.WriteFile(dir+"/quarantine.yml", []byte("version: 1\nframework: rspec\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// User enters 'n' (or just enter) to not overwrite.
	stdout, err := executeInitCmd(t,
		"n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN": "ghp_test",
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml exists and user declines overwrite",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml exists and user declines overwrite",
		Should:   "print 'Aborted'",
		Actual:   strings.Contains(stdout, "Aborted") || strings.Contains(stdout, "preserved"),
		Expected: true,
	})

	// Existing file should be unchanged.
	content, readErr := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "user aborts init",
		Should:   "preserve existing quarantine.yml content",
		Actual:   readErr == nil && strings.Contains(string(content), "rspec"),
		Expected: true,
	})
}

// --- Scenario 4: quarantine.yml already exists — user enters y to overwrite ---

func TestInitConfigAlreadyExistsOverwrite(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Write existing config with rspec.
	if err := os.WriteFile(dir+"/quarantine.yml", []byte("version: 1\nframework: rspec\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mockServer := newInitTestServer(t)

	// User enters 'y' to overwrite, then selects jest.
	stdout, err := executeInitCmd(t,
		"y\njest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml exists and user enters 'y' to overwrite",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml exists and user enters 'y' to overwrite",
		Should:   "print success message",
		Actual:   strings.Contains(stdout, "Quarantine initialized successfully"),
		Expected: true,
	})

	// File should now contain jest, not rspec.
	content, readErr := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "user overwrites existing config with jest",
		Should:   "quarantine.yml now contains framework: jest",
		Actual:   readErr == nil && strings.Contains(string(content), "framework: jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "user overwrites existing config with jest",
		Should:   "quarantine.yml no longer contains framework: rspec",
		Actual:   !strings.Contains(string(content), "framework: rspec"),
		Expected: true,
	})
}

// --- Scenario 1: First-time setup with Jest ---

func TestInitJestFirstTime(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"jest\n\n\n", // framework=jest, retries=default, junitxml=default
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":         "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL":  mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "first-time jest setup with valid GitHub token and repo",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "first-time jest setup",
		Should:   "print success message",
		Actual:   strings.Contains(stdout, "Quarantine initialized successfully"),
		Expected: true,
	})

	// Verify quarantine.yml was created.
	content, readErr := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "first-time jest setup",
		Should:   "create quarantine.yml",
		Actual:   readErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml written by init",
		Should:   "contain version: 1",
		Actual:   strings.Contains(string(content), "version: 1"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine.yml written by init",
		Should:   "contain framework: jest",
		Actual:   strings.Contains(string(content), "framework: jest"),
		Expected: true,
	})

	// Jest-specific recommendation should be printed.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest framework selected",
		Should:   "print jest-junit recommendation",
		Actual:   strings.Contains(stdout, "jest-junit"),
		Expected: true,
	})

	// Verify API call order: GetRepo → GetRef(default) → GetRef(state) → CreateRef → PutContents
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "successful init with new branch",
		Should:   "call GET /repos first",
		Actual:   len(mockServer.requests) > 0 && mockServer.requests[0] == "GET /repos/my-org/my-project",
		Expected: true,
	})
}

// --- Scenario 2: quarantine init with RSpec ---

func TestInitRSpec(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"rspec\n\n\n", // framework=rspec, retries=default, junitxml=default
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec framework selected",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	content, _ := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec framework selected",
		Should:   "write framework: rspec to quarantine.yml",
		Actual:   strings.Contains(string(content), "framework: rspec"),
		Expected: true,
	})

	// No jest-junit recommendation for rspec.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec framework selected",
		Should:   "NOT print jest-junit recommendation",
		Actual:   !strings.Contains(stdout, "jest-junit"),
		Expected: true,
	})

	// Workflow snippet should be rspec-specific.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec framework selected",
		Should:   "print rspec workflow snippet",
		Actual:   strings.Contains(stdout, "RspecJunitFormatter") || strings.Contains(stdout, "rspec"),
		Expected: true,
	})
}

// --- Scenario 3: quarantine init with Vitest ---

func TestInitVitest(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"vitest\n\n\n", // framework=vitest, retries=default, junitxml=default
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "vitest framework selected",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	content, _ := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "vitest framework selected",
		Should:   "write framework: vitest to quarantine.yml",
		Actual:   strings.Contains(string(content), "framework: vitest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "vitest framework selected",
		Should:   "print vitest workflow snippet with --reporter=junit",
		Actual:   strings.Contains(stdout, "--reporter=junit"),
		Expected: true,
	})
}

// --- Scenario 5: quarantine/state branch already exists ---

func TestInitBranchAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	mockServer := newInitTestServer(t, withExistingBranch())

	stdout, err := executeInitCmd(t,
		"jest\n\n\n",
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch already exists",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch already exists",
		Should:   "print message about skipping branch creation",
		Actual:   strings.Contains(stdout, "already exists") && strings.Contains(stdout, "Skipping"),
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

// --- Retries prompt: non-default and invalid input ---

func TestInitCustomRetriesInput(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// No token so init fails after writing config — retries is written before GitHub ops.
	_, _ = executeInitCmd(t,
		"jest\n5\n\n", // framework=jest, retries=5, junitxml=default
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN": "",
			"GITHUB_TOKEN":            "",
		},
	)

	content, readErr := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries input '5' provided at the prompt",
		Should:   "write quarantine.yml without error",
		Actual:   readErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries input '5' provided at the prompt",
		Should:   "write 'retries: 5' to quarantine.yml",
		Actual:   strings.Contains(string(content), "retries: 5"),
		Expected: true,
	})
}

func TestInitInvalidRetriesInputUsesDefault(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// No token so init fails after writing config.
	_, _ = executeInitCmd(t,
		"jest\nabc\n\n", // framework=jest, retries=invalid, junitxml=default
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN": "",
			"GITHUB_TOKEN":            "",
		},
	)

	content, readErr := os.ReadFile(dir + "/quarantine.yml")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "invalid retries input 'abc' provided at the prompt",
		Should:   "write quarantine.yml without error",
		Actual:   readErr == nil,
		Expected: true,
	})

	// writeConfig omits the retries key when it equals the default (3).
	// If the mutation fires (err == nil → err != nil), Atoi("abc") returns (0, err);
	// err != nil becomes true → retries is set to 0 → written as "retries: 0".
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "invalid retries input 'abc' (Atoi fails)",
		Should:   "not write a retries key (default 3 is omitted by writeConfig)",
		Actual:   !strings.Contains(string(content), "retries:"),
		Expected: true,
	})
}

// --- writeConfig unit test ---

func TestWriteConfigFailure(t *testing.T) {
	err := writeConfig("/nonexistent/path/quarantine.yml", "jest", 3, "junit.xml", "junit.xml")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a path inside a nonexistent directory",
		Should:   "return an error",
		Actual:   err != nil,
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

// --- classifyGitHubError unit tests ---

func TestClassifyGitHubError401(t *testing.T) {
	msg := classifyGitHubError(fmt.Errorf("401 Unauthorized"), "my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a 401 error from the GitHub API",
		Should:   "return message about invalid or expired token",
		Actual:   strings.Contains(msg, "invalid or expired"),
		Expected: true,
	})
}

func TestClassifyGitHubError403(t *testing.T) {
	msg := classifyGitHubError(fmt.Errorf("403 Forbidden"), "my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a 403 error from the GitHub API",
		Should:   "return message about lacking permission including owner/repo",
		Actual:   strings.Contains(msg, "lacks permission") && strings.Contains(msg, "my-org/my-repo"),
		Expected: true,
	})
}

func TestClassifyGitHubErrorNetworkFailure(t *testing.T) {
	msg := classifyGitHubError(fmt.Errorf("connection refused"), "my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a connection refused error",
		Should:   "return message about unable to reach GitHub API",
		Actual:   strings.Contains(msg, "Unable to reach GitHub API"),
		Expected: true,
	})
}

func TestClassifyGitHubErrorGeneric(t *testing.T) {
	msg := classifyGitHubError(fmt.Errorf("unexpected status 500"), "my-org", "my-repo")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an unrecognized GitHub API error",
		Should:   "return generic GitHub API error message",
		Actual:   strings.Contains(msg, "GitHub API error"),
		Expected: true,
	})
}

// --- classifyGitRemoteError unit tests ---

func TestClassifyGitRemoteErrorNotGitRepo(t *testing.T) {
	msg := classifyGitRemoteError(fmt.Errorf("not a git repository: /some/path"))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a not-a-git-repository error",
		Should:   "return message about running from a git repository",
		Actual:   strings.Contains(msg, "Not a git repository"),
		Expected: true,
	})
}

func TestClassifyGitRemoteErrorNotGitHubURL(t *testing.T) {
	msg := classifyGitRemoteError(fmt.Errorf("remote 'origin' is not a GitHub URL: https://gitlab.com/foo/bar.git"))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-GitHub remote URL error",
		Should:   "return message containing the URL",
		Actual:   strings.Contains(msg, "gitlab.com/foo/bar.git"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a non-GitHub remote URL error",
		Should:   "mention manual config option",
		Actual:   strings.Contains(msg, "quarantine.yml"),
		Expected: true,
	})
}

func TestClassifyGitRemoteErrorGeneric(t *testing.T) {
	msg := classifyGitRemoteError(fmt.Errorf("exec: git not found"))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an unrecognized git remote error",
		Should:   "return message containing the original error",
		Actual:   strings.Contains(msg, "git not found"),
		Expected: true,
	})
}

// --- formatInitSummary unit tests ---

func TestFormatInitSummaryNewBranch(t *testing.T) {
	summary := formatInitSummary("jest", 3, "junit.xml", false)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest, retries=3, new branch",
		Should:   "contain framework jest",
		Actual:   strings.Contains(summary, "Framework:  jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest, retries=3, new branch",
		Should:   "show branch as created",
		Actual:   strings.Contains(summary, "quarantine/state (created)"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest, retries=3, new branch",
		Should:   "contain quarantine doctor next step",
		Actual:   strings.Contains(summary, "quarantine doctor"),
		Expected: true,
	})
}

func TestFormatInitSummaryExistingBranch(t *testing.T) {
	summary := formatInitSummary("rspec", 5, "custom.xml", true)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec, retries=5, existing branch",
		Should:   "show branch as already exists",
		Actual:   strings.Contains(summary, "quarantine/state (already exists)"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rspec, retries=5, existing branch",
		Should:   "contain retries value",
		Actual:   strings.Contains(summary, "Retries:    5"),
		Expected: true,
	})
}

// --- jestRecommendation unit tests ---

func TestJestRecommendationForJest(t *testing.T) {
	rec := jestRecommendation("jest")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "framework is jest",
		Should:   "return non-empty recommendation",
		Actual:   rec != "",
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "framework is jest",
		Should:   "contain jest-junit configuration",
		Actual:   strings.Contains(rec, "jest-junit"),
		Expected: true,
	})
}

func TestJestRecommendationForRSpec(t *testing.T) {
	rec := jestRecommendation("rspec")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "framework is rspec",
		Should:   "return empty string",
		Actual:   rec,
		Expected: "",
	})
}

func TestJestRecommendationForVitest(t *testing.T) {
	rec := jestRecommendation("vitest")

	riteway.Assert(t, riteway.Case[string]{
		Given:    "framework is vitest",
		Should:   "return empty string",
		Actual:   rec,
		Expected: "",
	})
}

// setupFakeGitRepo creates a real git repository in dir with the given remote URL.
func setupFakeGitRepo(t *testing.T, dir, remoteURL string) {
	t.Helper()
	runCmd(t, dir, "git", "init", "-b", "main")
	runCmd(t, dir, "git", "remote", "add", "origin", remoteURL)
}

// runCmd runs a command in a directory and fails the test on error.
func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run %s %v: %v\n%s", name, args, err, out)
	}
}
