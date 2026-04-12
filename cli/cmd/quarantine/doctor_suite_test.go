package main

import (
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// writeTempSuiteConfig writes a .quarantine/config.yml to a temp dir and returns the path.
func writeTempSuiteConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	suiteDir := dir + "/.quarantine"
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatalf("writeTempSuiteConfig mkdir: %v", err)
	}
	path := suiteDir + "/config.yml"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTempSuiteConfig write: %v", err)
	}
	return path
}

// --- Scenario 114: quarantine doctor validates test_suites array and rejects invalid configs ---

func TestDoctorRejectsInvalidSuiteConfig(t *testing.T) {
	path := writeTempSuiteConfig(t, `
version: 1
test_suites:
  - name: "My Backend!"
    command: "bundle exec rspec"
    rerun_command: ["bundle", "exec", "rspec", "-e", "{name}"]
`)

	stdout, err := executeDoctorCmd(t, []string{"--config", path}, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a .quarantine/config.yml with an invalid suite entry",
		Should:   "return an error (exit 2)",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with an invalid name containing uppercase letters and special chars",
		Should:   "report name validation error",
		Actual:   strings.Contains(stdout, `suite "My Backend!": name must match [a-z0-9][a-z0-9-]* (max 30 chars)`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with command as a string instead of a YAML array",
		Should:   "report command-is-string error",
		Actual:   strings.Contains(stdout, `suite "My Backend!": command must be a YAML array, not a string. Use: command: ["bundle", "exec", "rspec"]`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite missing the junitxml field",
		Should:   "report junitxml is required",
		Actual:   strings.Contains(stdout, `suite "My Backend!": junitxml is required`),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a suite with a valid rerun_command array",
		Should:   "not report any errors about rerun_command",
		Actual:   !strings.Contains(stdout, "rerun_command"),
		Expected: true,
	})
}
