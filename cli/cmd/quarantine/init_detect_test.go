package main

import (
	"os"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// TestInitDetectsJestAndRSpec is the integration test for Scenario 110.
// Given a repo with package.json (jest in devDependencies) and Gemfile (gem 'rspec'),
// when quarantine init runs, it auto-detects both frameworks and creates
// .quarantine/config.yml with two pre-filled suite entries.
func TestInitDetectsJestAndRSpec(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")

	// Write package.json with jest in devDependencies.
	pkgJSON := `{
  "name": "my-project",
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`
	if err := os.WriteFile(dir+"/package.json", []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	// Write Gemfile with rspec.
	gemfile := `source 'https://rubygems.org'
gem 'rspec', '~> 3.12'
`
	if err := os.WriteFile(dir+"/Gemfile", []byte(gemfile), 0644); err != nil {
		t.Fatalf("write Gemfile: %v", err)
	}

	mockServer := newInitTestServer(t)

	stdout, err := executeInitCmd(t,
		"", // no stdin needed — auto-detection, no prompts
		dir,
		map[string]string{
			"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
			"QUARANTINE_GITHUB_API_BASE_URL": mockServer.server.URL,
		},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "repo with jest in package.json and gem 'rspec' in Gemfile",
		Should:   "exit without error",
		Actual:   err == nil,
		Expected: true,
	})

	// Both framework names must appear in output (unique to two-framework case).
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "repo with jest and rspec",
		Should:   "mention jest in output",
		Actual:   strings.Contains(stdout, "jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "repo with jest and rspec",
		Should:   "mention rspec in output",
		Actual:   strings.Contains(stdout, "rspec"),
		Expected: true,
	})

	// Suite count must reflect both frameworks (unique assertion).
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "repo with jest and rspec",
		Should:   "mention suite count in output",
		Actual:   strings.Contains(stdout, "2"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "repo with jest and rspec",
		Should:   "mention suite entries in output",
		Actual:   strings.Contains(stdout, "suite"),
		Expected: true,
	})

	// Config must contain both suite entries (unique to this two-framework test).
	configContent, readErr := os.ReadFile(dir + "/.quarantine/config.yml")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine init completes successfully",
		Should:   "create .quarantine/config.yml",
		Actual:   readErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    ".quarantine/config.yml created by init",
		Should:   "contain jest suite entry",
		Actual:   strings.Contains(string(configContent), "name: jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    ".quarantine/config.yml created by init",
		Should:   "contain rspec suite entry",
		Actual:   strings.Contains(string(configContent), "name: rspec"),
		Expected: true,
	})

	// .quarantine/.gitignore must be created (first place this is tested end-to-end).
	gitignoreContent, gitignoreErr := os.ReadFile(dir + "/.quarantine/.gitignore")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "quarantine init completes successfully",
		Should:   "create .quarantine/.gitignore",
		Actual:   gitignoreErr == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    ".quarantine/.gitignore created by init",
		Should:   "contain config.yml exception",
		Actual:   strings.Contains(string(gitignoreContent), "!config.yml"),
		Expected: true,
	})
}
