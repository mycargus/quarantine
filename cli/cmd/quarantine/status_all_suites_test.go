package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// makeStateJSON creates a quarantine state JSON with n test entries.
func makeStateJSON(t *testing.T, suiteName string, count int) string {
	t.Helper()
	tests := make(map[string]interface{}, count)
	for i := 1; i <= count; i++ {
		id := fmt.Sprintf("test/%s/%d", suiteName, i)
		tests[id] = map[string]interface{}{
			"test_id":         id,
			"file_path":       "test/file.go",
			"classname":       "X",
			"name":            fmt.Sprintf("test%d", i),
			"suite":           suiteName,
			"first_flaky_at":  "2026-01-01T00:00:00Z",
			"last_failure_at": "2026-01-02T00:00:00Z",
			"flaky_count":     1,
			"quarantined_at":  "2026-01-01T00:00:00Z",
			"quarantined_by":  "cli-auto",
		}
	}
	state := map[string]interface{}{
		"version":    1,
		"updated_at": "2026-04-14T00:00:00Z",
		"tests":      tests,
	}
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("makeStateJSON: %v", err)
	}
	return string(b)
}

// --- Scenario 130: quarantine status (no suite name) ---

func TestStatusAllSuitesShowsSummary(t *testing.T) {
	backendStateBytes := []byte(makeStateJSON(t, "backend", 5))
	frontendStateBytes := []byte(makeStateJSON(t, "frontend", 2))

	backendEncoded := base64.StdEncoding.EncodeToString(backendStateBytes)
	frontendEncoded := base64.StdEncoding.EncodeToString(frontendStateBytes)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/") && strings.Contains(r.URL.RawQuery, "ref=quarantine") {
			switch {
			case strings.Contains(r.URL.Path, "backend"):
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"content": backendEncoded,
					"sha":     "backend-sha",
				})
			case strings.Contains(r.URL.Path, "frontend"):
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"content": frontendEncoded,
					"sha":     "frontend-sha",
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	writeTempConfig(t, `
version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "--only-failures"]
    retries: 3
  - name: frontend
    command: ["pnpm", "vitest"]
    junitxml: "vitest.xml"
    rerun_command: ["pnpm", "vitest", "--reporter=verbose"]
    retries: 2
`)

	stdout, err := executeStatusCmd(t, []string{}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend (5 quarantined) and frontend (2 quarantined) suites, no suite name arg",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend (5 quarantined) and frontend (2 quarantined) suites, no suite name arg",
		Should:   "contain 'backend' suite name in the output",
		Actual:   strings.Contains(stdout, "backend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend (5 quarantined) and frontend (2 quarantined) suites, no suite name arg",
		Should:   "contain 'frontend' suite name in the output",
		Actual:   strings.Contains(stdout, "frontend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend (5 quarantined) and frontend (2 quarantined) suites, no suite name arg",
		Should:   "contain 'Total' row in the output",
		Actual:   strings.Contains(stdout, "Total"),
		Expected: true,
	})
}

// TestStatusAllSuitesStateFetchError verifies that when one suite's state file
// returns 404, that suite appears with count 0 and the run still exits 0.
func TestStatusAllSuitesStateFetchError(t *testing.T) {
	frontendStateBytes := []byte(makeStateJSON(t, "frontend", 2))
	frontendEncoded := base64.StdEncoding.EncodeToString(frontendStateBytes)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/") && strings.Contains(r.URL.RawQuery, "ref=quarantine") {
			switch {
			case strings.Contains(r.URL.Path, "backend"):
				// Backend state file returns 404 — simulates missing/expired state.
				w.WriteHeader(http.StatusNotFound)
			case strings.Contains(r.URL.Path, "frontend"):
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"content": frontendEncoded,
					"sha":     "frontend-sha",
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	writeTempConfig(t, `
version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: backend
    command: ["bundle", "exec", "rspec"]
    junitxml: "rspec.xml"
    rerun_command: ["bundle", "exec", "rspec", "--only-failures"]
    retries: 3
  - name: frontend
    command: ["pnpm", "vitest"]
    junitxml: "vitest.xml"
    rerun_command: ["pnpm", "vitest", "--reporter=verbose"]
    retries: 2
`)

	stdout, err := executeStatusCmd(t, []string{}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend state returns 404 and frontend state has 2 quarantined tests, no suite name arg",
		Should:   "exit without error (code 0) — state fetch errors are non-fatal",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend state returns 404 and frontend state has 2 quarantined tests",
		Should:   "still include 'backend' in the output with count 0",
		Actual:   strings.Contains(stdout, "backend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend state returns 404 and frontend state has 2 quarantined tests",
		Should:   "include 'frontend' in the output",
		Actual:   strings.Contains(stdout, "frontend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend state returns 404 and frontend state has 2 quarantined tests",
		Should:   "include 'Total' row",
		Actual:   strings.Contains(stdout, "Total"),
		Expected: true,
	})
}
