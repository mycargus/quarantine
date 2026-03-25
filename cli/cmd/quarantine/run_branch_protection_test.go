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

	"github.com/mycargus/quarantine/internal/quarantine"
)

// fakeBranchProtectedAPI creates a test server that returns 403 on PUT
// to simulate a branch protection rule blocking the write.
func fakeBranchProtectedAPI(t *testing.T, qs *quarantine.State) *httptest.Server {
	t.Helper()

	var stateContent []byte
	if qs != nil && len(qs.Tests) > 0 {
		var err error
		stateContent, err = qs.Marshal()
		if err != nil {
			t.Fatalf("marshal quarantine state: %v", err)
		}
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			if len(stateContent) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			encoded := base64.StdEncoding.EncodeToString(stateContent)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": encoded,
				"sha":     "state-sha-abc",
			})

		case strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/contents/quarantine.json"):
			// Simulate branch protection — always returns 403.
			w.WriteHeader(http.StatusForbidden)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// --- Scenario 42: Write to protected branch — fallback warning ---

func TestRunWriteToProtectedBranchEmitsWarningAndExits0(t *testing.T) {
	dir := t.TempDir()

	// Start with empty quarantine state — new flaky test will be detected this run.
	qs := quarantine.NewEmptyState()

	// JUnit XML: one test fails initially, passes on retry → flaky.
	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/payment.test.js" tests="1" failures="1">
    <testcase classname="PaymentService" name="should process payment"
              file="src/payment.test.js" time="0.1">
      <failure message="timeout">timeout</failure>
    </testcase>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	// The rerun script exits 0 — test passes on retry (flaky).
	rerunScriptPath := filepath.Join(dir, "rerun")
	if err := os.WriteFile(rerunScriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 1)

	configPath := writeTempConfig(t, fmt.Sprintf(`
version: 1
framework: jest
retries: 1
rerun_command: %s
`, rerunScriptPath))

	server := fakeBranchProtectedAPI(t, qs)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, err := executeRunCmd(t, []string{
		"--config", configPath,
		"--junitxml", xmlPath,
		"--output", resultsPath,
		"--", scriptPath,
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch is protected (PUT returns 403) and a flaky test is detected",
		Should:   "exit with code 0 (protected branch is a warning, not a fatal error)",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch is protected (PUT returns 403)",
		Should:   "log a warning that the branch is protected",
		Actual:   strings.Contains(output, "protected"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine/state branch is protected (PUT returns 403)",
		Should:   "name the specific branch in the warning",
		Actual:   strings.Contains(output, "quarantine/state"),
		Expected: true,
	})
}
