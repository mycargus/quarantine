package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// initTestServer creates a minimal mock GitHub API server for init tests.
type initTestServer struct {
	server         *httptest.Server
	defaultBranch  string
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
		// default branch SHA — reject empty branch name so that tests can verify
		// the fallback to "main" fires when defaultBranch is "".
		branchName := strings.TrimPrefix(r.URL.Path, "/repos/my-org/my-project/git/ref/heads/")
		if branchName == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ref": "refs/heads/" + branchName,
			"object": map[string]interface{}{
				"sha":  "basesha123",
				"type": "commit",
			},
		})
	})

	// POST /repos/{owner}/{repo}/git/refs — create branch
	mux.HandleFunc("/repos/my-org/my-project/git/refs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		// Validate that a non-empty SHA was provided, mirroring real GitHub API behavior.
		var body map[string]interface{}
		if decErr := json.NewDecoder(r.Body).Decode(&body); decErr != nil || body["sha"] == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "sha is required"})
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

func withEmptyDefaultBranch() func(*initTestServer) {
	return func(s *initTestServer) {
		s.defaultBranch = ""
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
