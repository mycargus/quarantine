package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mycargus/quarantine/cli/internal/result"
	riteway "github.com/mycargus/riteway-golang"
)

// extractExitCode resolves the numeric exit code from a quarantine CLI error.
// Fails the test if the error is non-nil but not an exitCodeError — this surfaces
// unexpected error types rather than silently mapping them to code 2.
func extractExitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	if code, ok := err.(exitCodeError); ok {
		return int(code)
	}
	t.Fatalf("unexpected error type %T (not exitCodeError): %v", err, err)
	return -1
}

// fakeSuiteCrashAPI creates a minimal test server for crash/unresolved scenarios.
// It handles branch check and returns 404 for state reads; tracks issue creation.
func fakeSuiteCrashAPI(t *testing.T, issueCreated *bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusNotFound)

		case strings.Contains(r.URL.Path, "/search/issues"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 0,
				"items":       []interface{}{},
			})

		case strings.Contains(r.URL.Path, "/comments"):
			if r.Method == "GET" {
				_ = json.NewEncoder(w).Encode([]interface{}{})
			} else {
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprint(w, `{"id":1,"body":"comment"}`)
			}

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			if issueCreated != nil {
				*issueCreated = true
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"number":1,"html_url":"https://github.com/owner/repo/issues/1"}`)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// writeSuiteConfig writes a .quarantine/config.yml with the given suite definition
// and returns the config file path.
func writeSuiteConfig(t *testing.T, dir, suiteName, command, junitxml, rerunCommand string) string {
	t.Helper()
	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := fmt.Sprintf(`version: 1
test_suites:
  - name: %s
    command: ["%s"]
    junitxml: "%s"
    rerun_command: ["%s", "--testNamePattern", "{name}"]
    retries: 1
`, suiteName, command, junitxml, rerunCommand)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}
	return configPath
}

// --- Scenario 122: command crash detection ---

func TestRunSuiteCommandCrashExitsTwo(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")

	// A script that exits non-zero but writes NO JUnit XML.
	crashScript := filepath.Join(dir, "fake-jest-crash")
	script := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(crashScript, []byte(script), 0755); err != nil {
		t.Fatalf("write crash script: %v", err)
	}

	configPath := writeSuiteConfig(t, dir, "frontend", crashScript, xmlPath, "npx jest")

	issueCreated := false
	server := fakeSuiteCrashAPI(t, &issueCreated)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, exitErr := executeRunCmd(t, []string{
		"--config", configPath,
		"--output", resultsPath,
		"frontend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	exitCode := extractExitCode(t, exitErr)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a frontend suite whose command exits non-zero and produces no JUnit XML",
		Should:   "exit with code 2 (command crash — infrastructure error)",
		Actual:   exitCode,
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a frontend suite whose command exits non-zero and produces no JUnit XML",
		Should:   "print an error mentioning 'crash'",
		Actual:   strings.Contains(output, "crash"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a frontend suite command crash",
		Should:   "print the JUnit XML path in the error message",
		Actual:   strings.Contains(output, "junit.xml"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a frontend suite command crash",
		Should:   "not create a GitHub Issue",
		Actual:   issueCreated,
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a frontend suite command crash",
		Should:   "not write results.json",
		Actual:   func() bool { _, err := os.Stat(resultsPath); return err != nil }(),
		Expected: true,
	})
}

// --- Scenario 123: rerun failure classifies test as unresolved ---

func TestRunSuiteRerunFailureClassifiesUnresolved(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")

	// JUnit XML: two tests fail the initial run.
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="2" failures="2" errors="0" time="2.0">
  <testsuite name="__tests__/checkout.test.js" tests="2" failures="2" time="2.0">
    <testcase classname="CheckoutService" name="should apply discount"
              file="__tests__/checkout.test.js" time="0.5">
      <failure message="expected 10 but got 0" type="AssertionError">expected 10 but got 0</failure>
    </testcase>
    <testcase classname="CheckoutService" name="should calculate tax"
              file="__tests__/checkout.test.js" time="0.5">
      <failure message="expected 5 but got 0" type="AssertionError">expected 5 but got 0</failure>
    </testcase>
  </testsuite>
</testsuites>`

	// Main command succeeds and writes XML.
	mainScript := filepath.Join(dir, "fake-jest")
	mainScriptContent := fmt.Sprintf("#!/bin/sh\ncat > %s << 'EOF'\n%s\nEOF\nexit 1\n", xmlPath, failXML)
	if err := os.WriteFile(mainScript, []byte(mainScriptContent), 0755); err != nil {
		t.Fatalf("write main script: %v", err)
	}

	// Rerun command uses a nonexistent binary so it fails with exit 127.
	configPath := writeSuiteConfig(t, dir, "backend", mainScript, xmlPath, "/nonexistent-rerun-binary-xyz")

	issueCreated := false
	server := fakeSuiteCrashAPI(t, &issueCreated)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	output, exitErr := executeRunCmd(t, []string{
		"--config", configPath,
		"--output", resultsPath,
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	exitCode := extractExitCode(t, exitErr)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two failing tests whose rerun command exits 127 (not found)",
		Should:   "exit with code 2 (infrastructure error — no genuine failures)",
		Actual:   exitCode,
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rerun command fails with exit 127",
		Should:   "not create GitHub Issues (unresolved, not flaky)",
		Actual:   issueCreated,
		Expected: false,
	})

	// results.json must be written with both tests as "unresolved".
	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr != nil {
		t.Fatalf("results.json not written: %v", readErr)
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatalf("invalid JSON in results.json: %v", err)
	}

	tests, _ := results["tests"].([]interface{})
	unresolvedCount := 0
	for _, item := range tests {
		if tMap, ok := item.(map[string]interface{}); ok {
			if tMap["status"] == "unresolved" {
				unresolvedCount++
			}
		}
	}

	riteway.Assert(t, riteway.Case[int]{
		Given:    "two tests whose rerun command fails",
		Should:   "write both tests with status 'unresolved' in results.json",
		Actual:   unresolvedCount,
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "rerun command fails",
		Should:   "print a message mentioning 'rerun' failure",
		Actual:   strings.Contains(output, "rerun"),
		Expected: true,
	})
}

// --- Scenario 124: genuine failures and rerun failures → exit 1 (priority rule) ---

func TestRunSuiteGenuineFailurePriorityOverUnresolved(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "junit.xml")

	// JUnit XML: three failing tests.
	// - "test A" (flaky): first run fails, rerun passes
	// - "test B" (unresolved): rerun command crashes
	// - "test C" (genuine): fails all retries
	failXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="3" failures="3" errors="0" time="3.0">
  <testsuite name="__tests__/suite.test.js" tests="3" failures="3" time="3.0">
    <testcase classname="Suite" name="test A"
              file="__tests__/suite.test.js" time="0.5">
      <failure message="flaky" type="AssertionError">flaky</failure>
    </testcase>
    <testcase classname="Suite" name="test B"
              file="__tests__/suite.test.js" time="0.5">
      <failure message="unresolved" type="AssertionError">unresolved</failure>
    </testcase>
    <testcase classname="Suite" name="test C"
              file="__tests__/suite.test.js" time="0.5">
      <failure message="genuine" type="AssertionError">genuine</failure>
    </testcase>
  </testsuite>
</testsuites>`

	passXMLForA := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="jest tests" tests="1" failures="0" errors="0" time="0.5">
  <testsuite name="__tests__/suite.test.js" tests="1" failures="0" time="0.5">
    <testcase classname="Suite" name="test A"
              file="__tests__/suite.test.js" time="0.5">
    </testcase>
  </testsuite>
</testsuites>`

	// Main command: writes fail XML and exits 1.
	mainScript := filepath.Join(dir, "fake-jest-main")
	mainScriptContent := fmt.Sprintf("#!/bin/sh\ncat > %s << 'EOF'\n%s\nEOF\nexit 1\n", xmlPath, failXML)
	if err := os.WriteFile(mainScript, []byte(mainScriptContent), 0755); err != nil {
		t.Fatalf("write main script: %v", err)
	}

	// Rerun script:
	// - For "test A": first rerun passes (writes pass XML, exits 0).
	// - For "test B": uses a nonexistent binary (but we need it to fail). We do this
	//   by having the rerun script behave differently based on the test name arg.
	// - For "test C": always fails (exits 1).
	//
	// Strategy: use a counting rerun script that:
	//   - increments a counter file per test name
	//   - passes for "test A", fails for everything else
	rerunScript := filepath.Join(dir, "fake-jest-rerun")
	rerunScriptContent := fmt.Sprintf(`#!/bin/sh
TEST_NAME="$@"
case "$TEST_NAME" in
  *"test A"*)
    cat > %s << 'EOF'
%s
EOF
    exit 0
    ;;
  *"test C"*)
    # Genuine: always fail (test failure, not crash)
    exit 1
    ;;
  *)
    # test B: crash (binary not found simulation)
    exit 127
    ;;
esac
`, xmlPath, passXMLForA)
	if err := os.WriteFile(rerunScript, []byte(rerunScriptContent), 0755); err != nil {
		t.Fatalf("write rerun script: %v", err)
	}

	// Suite config using the rerun script.
	// rerun_command uses {name} — the runner substitutes the test name before invoking,
	// so "$@" in the script receives the expanded test name (e.g. "test A").
	suiteConfigDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(suiteConfigDir, 0755); err != nil {
		t.Fatalf("mkdir .quarantine: %v", err)
	}
	configPath := filepath.Join(suiteConfigDir, "config.yml")
	configContent := fmt.Sprintf(`version: 1
test_suites:
  - name: backend
    command: ["%s"]
    junitxml: "%s"
    rerun_command: ["%s", "{name}"]
    retries: 1
`, mainScript, xmlPath, rerunScript)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}

	server := fakeSuiteCrashAPI(t, nil)
	defer server.Close()

	resultsPath := filepath.Join(dir, "results.json")
	_, exitErr := executeRunCmd(t, []string{
		"--config", configPath,
		"--output", resultsPath,
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	exitCode := extractExitCode(t, exitErr)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one flaky test, one unresolved test, one genuine failure",
		Should:   "exit with code 1 (genuine failure takes priority over unresolved exit 2)",
		Actual:   exitCode,
		Expected: 1,
	})

	// Verify results.json is written and has the expected statuses.
	resultsData, readErr := os.ReadFile(resultsPath)
	if readErr != nil {
		t.Fatalf("results.json not written: %v", readErr)
	}

	var results map[string]interface{}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatalf("invalid JSON in results.json: %v", err)
	}

	statuses := map[string]string{}
	tests, _ := results["tests"].([]interface{})
	for _, item := range tests {
		if tMap, ok := item.(map[string]interface{}); ok {
			name, _ := tMap["name"].(string)
			status, _ := tMap["status"].(string)
			statuses[name] = status
		}
	}

	riteway.Assert(t, riteway.Case[string]{
		Given:    "test A passes on rerun",
		Should:   "be classified as 'flaky'",
		Actual:   statuses["test A"],
		Expected: "flaky",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "test B rerun exits 127 (infrastructure crash)",
		Should:   "be classified as 'unresolved'",
		Actual:   statuses["test B"],
		Expected: "unresolved",
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "test C fails all retries",
		Should:   "be classified as 'failed'",
		Actual:   statuses["test C"],
		Expected: "failed",
	})
}

// --- Unit tests for resolveExitCode pure function ---

func TestResolveExitCodeAllPass(t *testing.T) {
	res := result.Result{
		Summary: result.Summary{Total: 3, Passed: 3},
	}

	riteway.Assert(t, riteway.Case[int]{
		Given:    "all tests pass (no failures, no unresolved)",
		Should:   "return 0",
		Actual:   resolveExitCode(res),
		Expected: 0,
	})
}

func TestResolveExitCodeGenuineFailure(t *testing.T) {
	res := result.Result{
		Summary: result.Summary{Total: 2, Failed: 1, Unresolved: 1},
	}

	riteway.Assert(t, riteway.Case[int]{
		Given:    "one genuine failure and one unresolved",
		Should:   "return 1 (genuine failure takes priority)",
		Actual:   resolveExitCode(res),
		Expected: 1,
	})
}

func TestResolveExitCodeUnresolvedOnly(t *testing.T) {
	res := result.Result{
		Summary: result.Summary{Total: 2, Unresolved: 2},
	}

	riteway.Assert(t, riteway.Case[int]{
		Given:    "only unresolved tests (no genuine failures)",
		Should:   "return 2 (infrastructure error)",
		Actual:   resolveExitCode(res),
		Expected: 2,
	})
}
