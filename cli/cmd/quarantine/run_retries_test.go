package main

import (
	"os"
	"path/filepath"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- resolveOwnerRepo: git detection fallback ---
// Tests verify that git remote detection fires when owner or repo is absent.
// Kills mutations on the owner == "" || repo == "" condition.

// TestRunGitDetectionFillsMissingRepo: config has owner but no repo.
func TestRunGitDetectionFillsMissingRepo(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir, "git@github.com:mycargus/quarantine.git")

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	// Config has owner but no repo — detection must supply "quarantine".
	suiteDir := filepath.Join(dir, ".quarantine")
	_ = os.MkdirAll(suiteDir, 0755)
	configPath := filepath.Join(suiteDir, "config.yml")
	configContent := `version: 1
github:
  owner: mycargus
test_suites:
  - name: unit
    command: ["` + scriptPath + `"]
    junitxml: "` + xmlPath + `"
    rerun_command: ["false"]
    retries: 3
`
	_ = os.WriteFile(configPath, []byte(configContent), 0644)
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config has owner but no repo, running inside a git repo",
		Should:   "auto-detect the repo from git remote and succeed",
		Actual:   err == nil,
		Expected: true,
	})
}

// TestRunGitDetectionFillsMissingOwner: config has repo but no owner.
func TestRunGitDetectionFillsMissingOwner(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir, "git@github.com:mycargus/quarantine.git")

	junitXML := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="1" failures="0">
  <testsuite name="suite" tests="1" failures="0">
    <testcase classname="S" name="passes" time="0.1"/>
  </testsuite>
</testsuites>`

	xmlPath := filepath.Join(dir, "junit.xml")
	scriptPath := writeTestScript(t, dir, xmlPath, junitXML, 0)

	// Config has repo but no owner — detection must supply "mycargus".
	suiteDir := filepath.Join(dir, ".quarantine")
	_ = os.MkdirAll(suiteDir, 0755)
	configPath := filepath.Join(suiteDir, "config.yml")
	configContent := `version: 1
github:
  repo: quarantine
test_suites:
  - name: unit
    command: ["` + scriptPath + `"]
    junitxml: "` + xmlPath + `"
    rerun_command: ["false"]
    retries: 3
`
	_ = os.WriteFile(configPath, []byte(configContent), 0644)
	chdirTest(t, dir)

	server := fakeGitHubAPI(t, true)
	defer server.Close()

	_, err := executeRunCmd(t, []string{
		"unit",
	}, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "config has repo but no owner, running inside a git repo",
		Should:   "auto-detect the owner from git remote and succeed",
		Actual:   err == nil,
		Expected: true,
	})
}
