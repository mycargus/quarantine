package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qstate "github.com/mycargus/quarantine/cli/internal/quarantine"
	riteway "github.com/mycargus/riteway-golang"
)

// --- Unit test: quarantinedFilePaths pure function (Scenario 135) ---

func TestQuarantinedFilePaths(t *testing.T) {
	t.Run("deduplicates and sorts file paths", func(t *testing.T) {
		state := qstate.NewEmptyState()
		state.AddTest(qstate.Entry{
			TestID:   "spec/models/user_spec.rb::User::validates email",
			FilePath: "spec/models/user_spec.rb",
		})
		state.AddTest(qstate.Entry{
			TestID:   "spec/models/user_spec.rb::User::validates password",
			FilePath: "spec/models/user_spec.rb",
		})
		state.AddTest(qstate.Entry{
			TestID:   "spec/services/payment_spec.rb::Payment::charges card",
			FilePath: "spec/services/payment_spec.rb",
		})

		paths := quarantinedFilePaths(state)

		riteway.Assert(t, riteway.Case[int]{
			Given:    "3 quarantined tests across 2 files",
			Should:   "return 2 deduplicated file paths",
			Actual:   len(paths),
			Expected: 2,
		})

		riteway.Assert(t, riteway.Case[string]{
			Given:    "3 quarantined tests across 2 files",
			Should:   "return first path as spec/models/user_spec.rb (sorted)",
			Actual:   paths[0],
			Expected: "spec/models/user_spec.rb",
		})

		riteway.Assert(t, riteway.Case[string]{
			Given:    "3 quarantined tests across 2 files",
			Should:   "return second path as spec/services/payment_spec.rb (sorted)",
			Actual:   paths[1],
			Expected: "spec/services/payment_spec.rb",
		})
	})

	t.Run("returns empty slice for empty state", func(t *testing.T) {
		state := qstate.NewEmptyState()

		paths := quarantinedFilePaths(state)

		riteway.Assert(t, riteway.Case[int]{
			Given:    "an empty quarantine state",
			Should:   "return an empty slice",
			Actual:   len(paths),
			Expected: 0,
		})
	})

	t.Run("filters out empty file_path values", func(t *testing.T) {
		state := qstate.NewEmptyState()
		state.AddTest(qstate.Entry{
			TestID:   "spec/models/user_spec.rb::User::validates email",
			FilePath: "spec/models/user_spec.rb",
		})
		state.AddTest(qstate.Entry{
			TestID:   "::some::test",
			FilePath: "", // zero-value — should not appear in output
		})

		paths := quarantinedFilePaths(state)

		riteway.Assert(t, riteway.Case[int]{
			Given:    "state with one valid file_path and one empty file_path",
			Should:   "return only 1 path (empty filtered out)",
			Actual:   len(paths),
			Expected: 1,
		})

		riteway.Assert(t, riteway.Case[string]{
			Given:    "state with one valid file_path and one empty file_path",
			Should:   "return the non-empty file path",
			Actual:   paths[0],
			Expected: "spec/models/user_spec.rb",
		})
	})
}

// --- Integration test: quarantined-files.txt written before command runs (Scenario 135) ---

func TestRunWritesQuarantinedFilesTxtBeforeCommand(t *testing.T) {
	dir := t.TempDir()

	// State JSON with 3 tests across 2 files.
	stateJSON := `{
  "version": 1,
  "updated_at": "2026-04-14T10:00:00Z",
  "tests": {
    "spec/models/user_spec.rb::User::validates email": {
      "test_id": "spec/models/user_spec.rb::User::validates email",
      "file_path": "spec/models/user_spec.rb",
      "classname": "User",
      "name": "validates email",
      "suite": "backend",
      "first_flaky_at": "2026-04-01T10:00:00Z",
      "last_failure_at": "2026-04-13T10:00:00Z",
      "flaky_count": 3,
      "quarantined_at": "2026-04-01T10:00:00Z",
      "quarantined_by": "cli-auto"
    },
    "spec/models/user_spec.rb::User::validates password": {
      "test_id": "spec/models/user_spec.rb::User::validates password",
      "file_path": "spec/models/user_spec.rb",
      "classname": "User",
      "name": "validates password",
      "suite": "backend",
      "first_flaky_at": "2026-04-01T10:00:00Z",
      "last_failure_at": "2026-04-13T10:00:00Z",
      "flaky_count": 2,
      "quarantined_at": "2026-04-01T10:00:00Z",
      "quarantined_by": "cli-auto"
    },
    "spec/services/payment_spec.rb::Payment::charges card": {
      "test_id": "spec/services/payment_spec.rb::Payment::charges card",
      "file_path": "spec/services/payment_spec.rb",
      "classname": "Payment",
      "name": "charges card",
      "suite": "backend",
      "first_flaky_at": "2026-04-05T10:00:00Z",
      "last_failure_at": "2026-04-13T10:00:00Z",
      "flaky_count": 1,
      "quarantined_at": "2026-04-05T10:00:00Z",
      "quarantined_by": "cli-auto"
    }
  }
}`

	encodedState := base64.StdEncoding.EncodeToString([]byte(stateJSON))

	// Sentinel file: the fake binary writes quarantined-files.txt content here
	// if the file exists when the binary runs. This proves the file was written
	// BEFORE the command executed.
	sentinelPath := filepath.Join(dir, "quarantined-files-at-run-time.txt")
	quarantinedFilesPath := filepath.Join(dir, ".quarantine", "backend", "quarantined-files.txt")

	// XML output path (must be absolute for the config).
	xmlPath := filepath.Join(dir, "rspec.xml")

	// Fake binary: reads quarantined-files.txt if it exists, writes its content
	// to the sentinel file, then writes a valid empty JUnit XML and exits 0.
	fakeBin := filepath.Join(dir, "fake-rspec")
	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="rspec" tests="0" skipped="0" failures="0" errors="0" time="0.1">
</testsuite>`
	// The script reads the quarantined-files.txt relative to CWD (which is dir).
	script := fmt.Sprintf(`#!/bin/sh
if [ -f .quarantine/backend/quarantined-files.txt ]; then
  cp .quarantine/backend/quarantined-files.txt %s
fi
cat > %s << 'XMLEOF'
%s
XMLEOF
exit 0
`, sentinelPath, xmlPath, emptyXML)
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake-rspec: %v", err)
	}

	// Mock GitHub API server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		// Branch exists check.
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		// State file GET — return the backend state with 3 tests.
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/.quarantine/backend/state.json"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encodedState,
				"sha":     "state-sha-abc",
			})

		// Search issues — return empty (no existing issues).
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		// State file PUT — accept state update.
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"content":{"sha":"newsha123"}}`)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Write suite config.
	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configContent := fmt.Sprintf(`version: 1
github:
  owner: testowner
  repo: testrepo
test_suites:
  - name: backend
    command: ["%s"]
    junitxml: "%s"
    rerun_command: ["true"]
    retries: 1
`, fakeBin, xmlPath)
	if err := os.WriteFile(filepath.Join(suiteConfigDir, "config.yml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}

	chdirTest(t, dir)

	_, err := executeRunCmd(t, []string{"backend"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_BASE_REF":                "main",
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend suite with 3 quarantined tests across 2 files",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	// The file must exist at the expected path.
	fileContent, readErr := os.ReadFile(quarantinedFilesPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend suite with 3 quarantined tests across 2 files",
		Should:   "write .quarantine/backend/quarantined-files.txt",
		Actual:   readErr == nil,
		Expected: true,
	})

	// File must contain both file paths.
	content := string(fileContent)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantined-files.txt was written",
		Should:   "contain spec/models/user_spec.rb",
		Actual:   strings.Contains(content, "spec/models/user_spec.rb"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantined-files.txt was written",
		Should:   "contain spec/services/payment_spec.rb",
		Actual:   strings.Contains(content, "spec/services/payment_spec.rb"),
		Expected: true,
	})

	// File must have exactly 2 lines (deduplicated).
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	riteway.Assert(t, riteway.Case[int]{
		Given:    "3 tests across 2 files",
		Should:   "quarantined-files.txt has exactly 2 lines (deduplicated)",
		Actual:   len(lines),
		Expected: 2,
	})

	// KEY ASSERTION: The file must have existed BEFORE the command ran.
	// The fake binary copies quarantined-files.txt to the sentinel if it exists.
	sentinelContent, sentinelErr := os.ReadFile(sentinelPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantined-files.txt is written before command execution",
		Should:   "fake binary finds the file and writes it to sentinel",
		Actual:   sentinelErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "sentinel file written by fake binary at run time",
		Should:   "contain spec/models/user_spec.rb (file was present before command ran)",
		Actual:   strings.Contains(string(sentinelContent), "spec/models/user_spec.rb"),
		Expected: true,
	})
}
