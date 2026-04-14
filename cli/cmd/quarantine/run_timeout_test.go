package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// fakeTimeoutAPI creates a minimal test server for the timeout scenario.
// It handles branch check and returns 404 for state reads.
func fakeTimeoutAPI(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/git/ref/heads/quarantine/state"):
			_, _ = fmt.Fprint(w, `{"ref":"refs/heads/quarantine/state","object":{"sha":"abc123","type":"commit"}}`)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusNotFound)

		case strings.Contains(r.URL.Path, "/search/issues"):
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"total_count":0,"items":[]}`)

		case strings.Contains(r.URL.Path, "/comments"):
			if r.Method == "GET" {
				_, _ = fmt.Fprint(w, `[]`)
			} else {
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprint(w, `{"id":1,"body":"comment"}`)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// --- Scenario 131: timeout kills hanging process, processes partial XML, exits 2 ---

func TestRunCommandTimeoutPartialXML(t *testing.T) {
	dir := t.TempDir()

	// Build partial XML with 80 passing tests.
	// Use a variable so the count in the output assertion stays in sync with the builder.
	const partialTestCount = 80
	xmlPath := filepath.Join(dir, "rspec.xml")
	xmlContent := buildPartialRspecXML(partialTestCount)
	if err := os.WriteFile(xmlPath, []byte(xmlContent), 0644); err != nil {
		t.Fatalf("write rspec.xml: %v", err)
	}

	// Fake binary: hangs indefinitely using exec so no child process is spawned.
	// Using "exec sleep" ensures a single process that receives SIGTERM/SIGKILL directly.
	// The XML is already pre-written at xmlPath so quarantine finds it after kill.
	fakeBin := filepath.Join(dir, "fake-rspec")
	script := "#!/bin/sh\nexec sleep 9999\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	// Write config with a very short timeout (2s) so the test stays fast.
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
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
    timeout: "2s"
`, fakeBin, xmlPath)
	if err := os.WriteFile(filepath.Join(suiteConfigDir, "config.yml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}
	chdirTest(t, dir)

	server := fakeTimeoutAPI(t)
	defer server.Close()

	output, exitErr := executeRunCmd(t, []string{
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	exitCode := extractExitCode(t, exitErr)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a backend suite whose command hangs past the 2s timeout with partial XML pre-written",
		Should:   "exit with code 2 (quarantine infrastructure error)",
		Actual:   exitCode,
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite whose command hangs past the 2s timeout",
		Should:   "print complete timeout error message with duration",
		Actual:   strings.Contains(output, "Error [timeout]: test command timed out after 2s."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "partial XML with 80 tests exists when the timeout fires",
		Should:   "print partial results message with matching count from XML",
		Actual:   strings.Contains(output, fmt.Sprintf("Partial results processed: %d tests", partialTestCount)),
		Expected: true,
	})
}

// --- Scenario 132: timeout kills hanging process, no XML produced, exits 2 with specific message ---

func TestRunCommandTimeoutNoXML(t *testing.T) {
	dir := t.TempDir()

	// Fake binary: hangs indefinitely. Does NOT write any XML.
	fakeBin := filepath.Join(dir, "fake-rspec")
	script := "#!/bin/sh\nexec sleep 9999\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	xmlPath := filepath.Join(dir, "rspec.xml")

	// Write config with a very short timeout (2s) so the test stays fast.
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
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
    timeout: "2s"
`, fakeBin, xmlPath)
	if err := os.WriteFile(filepath.Join(suiteConfigDir, "config.yml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config.yml: %v", err)
	}
	chdirTest(t, dir)

	server := fakeTimeoutAPI(t)
	defer server.Close()

	output, exitErr := executeRunCmd(t, []string{
		"backend",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	exitCode := extractExitCode(t, exitErr)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a backend suite whose command hangs past the 2s timeout and produces no JUnit XML",
		Should:   "exit with code 2 (quarantine infrastructure error)",
		Actual:   exitCode,
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite whose command hangs past the 2s timeout and produces no JUnit XML",
		Should:   "print no-XML timeout error message with duration and XML path",
		Actual:   strings.Contains(output, "Error [timeout]: test command timed out after 2s and produced no JUnit XML at"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a backend suite whose command hangs past the 2s timeout and produces no JUnit XML",
		Should:   "print check-runner suggestion",
		Actual:   strings.Contains(output, "Check that your test runner can start successfully outside of quarantine."),
		Expected: true,
	})
}

// buildPartialRspecXML generates a JUnit XML string with n passing test cases.
func buildPartialRspecXML(n int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`<testsuite name="rspec" tests="%d" skipped="0" failures="0" errors="0" time="10.0">`, n))
	sb.WriteString("\n")
	for i := range n {
		sb.WriteString(fmt.Sprintf(
			`  <testcase classname="spec.models.user_spec" name="test %d" file="./spec/models/user_spec.rb" time="0.1"></testcase>`,
			i+1,
		))
		sb.WriteString("\n")
	}
	sb.WriteString("</testsuite>\n")
	return sb.String()
}
