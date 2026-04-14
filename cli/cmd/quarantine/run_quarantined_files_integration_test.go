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

	riteway "github.com/mycargus/riteway-golang"
)

// --- Integration test: quarantined-files.txt written before command runs (Scenario 135) ---

func TestRunWritesQuarantinedFilesTxtBeforeCommand(t *testing.T) {
	dir := t.TempDir()

	stateJSON := `{
  "version": 1,
  "updated_at": "2026-04-14T10:00:00Z",
  "tests": {
    "spec/models/user_spec.rb::User::validates email": {
      "test_id": "spec/models/user_spec.rb::User::validates email",
      "file_path": "spec/models/user_spec.rb",
      "classname": "User", "name": "validates email",
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
      "classname": "User", "name": "validates password",
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
      "classname": "Payment", "name": "charges card",
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
	sentinelPath := filepath.Join(dir, "quarantined-files-at-run-time.txt")
	quarantinedFilesPath := filepath.Join(dir, ".quarantine", "backend", "quarantined-files.txt")
	xmlPath := filepath.Join(dir, "rspec.xml")

	fakeBin := filepath.Join(dir, "fake-rspec")
	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="rspec" tests="0" skipped="0" failures="0" errors="0" time="0.1">
</testsuite>`
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/.quarantine/backend/state.json"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"content": encodedState, "sha": "state-sha-abc"})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"total_count": 0, "items": []interface{}{}})
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"content":{"sha":"newsha123"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

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

	fileContent, readErr := os.ReadFile(quarantinedFilesPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend suite with 3 quarantined tests across 2 files",
		Should:   "write .quarantine/backend/quarantined-files.txt",
		Actual:   readErr == nil,
		Expected: true,
	})

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

	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	riteway.Assert(t, riteway.Case[int]{
		Given:    "3 tests across 2 files",
		Should:   "quarantined-files.txt has exactly 2 lines (deduplicated)",
		Actual:   len(lines),
		Expected: 2,
	})

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

// --- Integration test: quarantined-files.txt written as empty file when no quarantined tests (Scenario 136) ---

func TestRunWritesEmptyQuarantinedFilesTxtWhenNoTests(t *testing.T) {
	dir := t.TempDir()

	stateJSON := `{"version":1,"updated_at":"2026-04-14T10:00:00Z","tests":{}}`
	encodedState := base64.StdEncoding.EncodeToString([]byte(stateJSON))

	sentinelPath := filepath.Join(dir, "sentinel.txt")
	quarantinedFilesPath := filepath.Join(dir, ".quarantine", "backend", "quarantined-files.txt")
	xmlPath := filepath.Join(dir, "rspec.xml")

	fakeBin := filepath.Join(dir, "fake-rspec-empty")
	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="rspec" tests="0" skipped="0" failures="0" errors="0" time="0.1">
</testsuite>`
	script := fmt.Sprintf(`#!/bin/sh
if [ -f .quarantine/backend/quarantined-files.txt ]; then
  echo "exists" > %s
else
  echo "not-exists" > %s
fi
cat > %s << 'XMLEOF'
%s
XMLEOF
exit 0
`, sentinelPath, sentinelPath, xmlPath, emptyXML)
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake-rspec-empty: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/.quarantine/backend/state.json"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"content": encodedState, "sha": "state-sha-abc"})
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"total_count": 0, "items": []interface{}{}})
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"content":{"sha":"newsha123"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

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
		Given:    "backend suite with empty tests map",
		Should:   "exit without error (code 0)",
		Actual:   err == nil,
		Expected: true,
	})

	// Fatal if file absent — prevents panic on info.Size().
	info, statErr := os.Stat(quarantinedFilesPath)
	if statErr != nil {
		t.Fatalf("expected .quarantine/backend/quarantined-files.txt to exist: %v", statErr)
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend suite with zero quarantined tests",
		Should:   "write .quarantine/backend/quarantined-files.txt (file exists)",
		Actual:   statErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int64]{
		Given:    "backend suite with empty tests map and quarantined-files.txt present",
		Should:   "quarantined-files.txt has zero bytes",
		Actual:   info.Size(),
		Expected: 0,
	})

	sentinelContent, sentinelErr := os.ReadFile(sentinelPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantined-files.txt written before command runs",
		Should:   "fake binary reports file exists at run time",
		Actual:   sentinelErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "sentinel written by fake binary",
		Should:   "equal 'exists' exactly (file was present before command ran, not 'not-exists')",
		Actual:   strings.TrimSpace(string(sentinelContent)) == "exists",
		Expected: true,
	})
}

// --- Integration test: quarantined-files.txt written as empty file in degraded mode (I3 — design gap) ---

func TestRunWritesEmptyQuarantinedFilesTxtInDegradedMode(t *testing.T) {
	dir := t.TempDir()

	quarantinedFilesPath := filepath.Join(dir, ".quarantine", "backend", "quarantined-files.txt")
	xmlPath := filepath.Join(dir, "rspec.xml")

	fakeBin := filepath.Join(dir, "fake-rspec-degraded")
	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="rspec" tests="0" skipped="0" failures="0" errors="0" time="0.1">
</testsuite>`
	script := fmt.Sprintf(`#!/bin/sh
cat > %s << 'XMLEOF'
%s
XMLEOF
exit 0
`, xmlPath, emptyXML)
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake-rspec-degraded: %v", err)
	}

	// Server returns 404 for state — simulates degraded mode (state unavailable).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"total_count": 0, "items": []interface{}{}})
		default:
			// State file GET returns 404 → degraded mode.
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

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

	_, _ = executeRunCmd(t, []string{"backend"}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
		"GITHUB_BASE_REF":                "main",
	})

	// Even in degraded mode, the file must exist (empty) so CI scripts don't fail.
	info, statErr := os.Stat(quarantinedFilesPath)
	if statErr != nil {
		t.Fatalf("expected .quarantine/backend/quarantined-files.txt to exist in degraded mode: %v", statErr)
	}

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "state fetch returns 404 (degraded mode)",
		Should:   "still write .quarantine/backend/quarantined-files.txt (file exists)",
		Actual:   statErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int64]{
		Given:    "state fetch returns 404 (degraded mode — no quarantine state available)",
		Should:   "quarantined-files.txt has zero bytes (empty, not missing)",
		Actual:   info.Size(),
		Expected: 0,
	})
}
