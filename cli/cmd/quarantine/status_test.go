package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	riteway "github.com/mycargus/riteway-golang"
)

// Note: pure function tests (averageDurationMs, formatDuration, daysBetween,
// computeStatusText, parseResultsJSON, extractResultsFromZip) are in
// status_pure_test.go.

// executeStatusCmd is a test helper that runs the status command with given args,
// capturing output and returning the exit error.
func executeStatusCmd(t *testing.T, args []string, env map[string]string) (stdout string, exitErr error) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	rootCmd := newRootCmd()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"status"}, args...))
	exitErr = rootCmd.Execute()
	return buf.String(), exitErr
}

// makeArtifactZip creates an in-memory ZIP containing results.json with the
// given test entries. Each entry has the given durationMs.
func makeArtifactZip(t *testing.T, tests []map[string]interface{}) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("results.json")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	results := map[string]interface{}{
		"version":    1,
		"suite_name": "backend",
		"tests":      tests,
	}
	if err := json.NewEncoder(f).Encode(results); err != nil {
		t.Fatalf("zip encode: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// --- Scenario 128: quarantine status backend ---

func TestStatusShowsSuiteStatusWithDuration(t *testing.T) {
	// "now" for the test: 2026-04-14T10:00:00Z
	// Tests quarantined 45, 30, 3 days ago respectively.
	// Last failed 2, 29, 3 days ago.
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	quarantinedAt45 := now.AddDate(0, 0, -45).Format(time.RFC3339) // 2026-02-28
	quarantinedAt30 := now.AddDate(0, 0, -30).Format(time.RFC3339) // 2026-03-15
	quarantinedAt3 := now.AddDate(0, 0, -3).Format(time.RFC3339)   // 2026-04-11

	lastFailure2 := now.AddDate(0, 0, -2).Format(time.RFC3339)  // 2026-04-12
	lastFailure29 := now.AddDate(0, 0, -29).Format(time.RFC3339) // 2026-03-16
	lastFailure3 := now.AddDate(0, 0, -3).Format(time.RFC3339)   // 2026-04-11

	stateJSON := map[string]interface{}{
		"version":    1,
		"updated_at": "2026-04-14T10:00:00Z",
		"tests": map[string]interface{}{
			"spec/models/user_spec.rb::User::validates email": map[string]interface{}{
				"test_id":        "spec/models/user_spec.rb::User::validates email",
				"file_path":      "spec/models/user_spec.rb",
				"classname":      "User",
				"name":           "validates email",
				"suite":          "backend",
				"first_flaky_at": quarantinedAt45,
				"last_failure_at": lastFailure2,
				"flaky_count":    5,
				"quarantined_at": quarantinedAt45,
				"quarantined_by": "cli-auto",
				"issue_number":   42,
				"issue_url":      "https://github.com/testowner/testrepo/issues/42",
			},
			"spec/models/order_spec.rb::Order::calculates total": map[string]interface{}{
				"test_id":        "spec/models/order_spec.rb::Order::calculates total",
				"file_path":      "spec/models/order_spec.rb",
				"classname":      "Order",
				"name":           "calculates total",
				"suite":          "backend",
				"first_flaky_at": quarantinedAt30,
				"last_failure_at": lastFailure29,
				"flaky_count":    3,
				"quarantined_at": quarantinedAt30,
				"quarantined_by": "cli-auto",
				"issue_number":   51,
				"issue_url":      "https://github.com/testowner/testrepo/issues/51",
			},
			"spec/services/payment_spec.rb::Payment::retries charge": map[string]interface{}{
				"test_id":        "spec/services/payment_spec.rb::Payment::retries charge",
				"file_path":      "spec/services/payment_spec.rb",
				"classname":      "Payment",
				"name":           "retries charge",
				"suite":          "backend",
				"first_flaky_at": quarantinedAt3,
				"last_failure_at": lastFailure3,
				"flaky_count":    2,
				"quarantined_at": quarantinedAt3,
				"quarantined_by": "cli-auto",
				"issue_number":   63,
				"issue_url":      "https://github.com/testowner/testrepo/issues/63",
			},
		},
	}

	stateBytes, _ := json.Marshal(stateJSON)
	stateEncoded := base64.StdEncoding.EncodeToString(stateBytes)

	// Each artifact has 3 quarantined tests each with 4200ms duration.
	artifactTests := []map[string]interface{}{
		{"test_id": "spec/models/user_spec.rb::User::validates email", "file_path": "spec/models/user_spec.rb", "classname": "User", "name": "validates email", "status": "quarantined", "duration_ms": 4200},
		{"test_id": "spec/models/order_spec.rb::Order::calculates total", "file_path": "spec/models/order_spec.rb", "classname": "Order", "name": "calculates total", "status": "quarantined", "duration_ms": 4200},
		{"test_id": "spec/services/payment_spec.rb::Payment::retries charge", "file_path": "spec/services/payment_spec.rb", "classname": "Payment", "name": "retries charge", "status": "quarantined", "duration_ms": 4200},
	}
	zipBytes := makeArtifactZip(t, artifactTests)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/") && strings.Contains(r.URL.RawQuery, "ref=quarantine"):
			// State file read.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": stateEncoded,
				"sha":     "state-sha-abc",
			})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/actions/artifacts"):
			// List artifacts — return 10 artifacts all pointing to same download URL.
			artifacts := make([]map[string]interface{}, 10)
			for i := range artifacts {
				artifacts[i] = map[string]interface{}{
					"id":                    i + 1,
					"name":                  "quarantine-results-backend",
					"archive_download_url":  fmt.Sprintf("%s/download/artifact/%d", server.URL, i+1),
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 10,
				"artifacts":   artifacts,
			})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/download/artifact/"):
			// Return ZIP bytes.
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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
`)

	stdout, err := executeStatusCmd(t, []string{"backend"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite with 3 quarantined tests and avg 4200ms duration per test",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite with 3 quarantined tests",
		Should:   "print suite name",
		Actual:   strings.Contains(stdout, "Suite: backend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite with 3 quarantined tests",
		Should:   "print quarantined test count",
		Actual:   strings.Contains(stdout, "Quarantined tests: 3"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite with avg 4200ms duration per quarantined test over last 10 runs",
		Should:   "print avg quarantined test duration as 4.2s",
		Actual:   strings.Contains(stdout, "Avg quarantined test duration: 4.2s"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "3 quarantined tests each averaging 4.2s",
		Should:   "print estimated CI time as ~12.6s",
		Actual:   strings.Contains(stdout, "Estimated CI time per run on quarantined tests: ~12.6s"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "validates email test quarantined 45 days ago",
		Should:   "print validates email test name",
		Actual:   strings.Contains(stdout, "validates email"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "validates email test with issue #42",
		Should:   "print issue number #42",
		Actual:   strings.Contains(stdout, "#42"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "validates email test quarantined 45 days ago",
		Should:   "print quarantine age 45 days",
		Actual:   strings.Contains(stdout, "45 days"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "calculates total test quarantined 30 days ago",
		Should:   "print calculates total test name",
		Actual:   strings.Contains(stdout, "calculates total"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "calculates total test with issue #51",
		Should:   "print issue number #51",
		Actual:   strings.Contains(stdout, "#51"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "calculates total test quarantined 30 days ago",
		Should:   "print quarantine age 30 days",
		Actual:   strings.Contains(stdout, "30 days"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries charge test quarantined 3 days ago",
		Should:   "print retries charge test name",
		Actual:   strings.Contains(stdout, "retries charge"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries charge test with issue #63",
		Should:   "print issue number #63",
		Actual:   strings.Contains(stdout, "#63"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "retries charge test quarantined 3 days ago",
		Should:   "print quarantine age 3 days",
		Actual:   strings.Contains(stdout, "3 days"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "user_spec.rb test last failed 2 days ago",
		Should:   "print 'last failed 2 days ago' for validates email",
		Actual:   strings.Contains(stdout, "last failed 2 days ago"),
		Expected: true,
	})
}

// Pure function tests are in status_pure_test.go.
