package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mycargus/quarantine/cli/internal/git"
	ghclient "github.com/mycargus/quarantine/cli/internal/github"
)

// runInit implements the `quarantine init` command.
// It auto-detects test frameworks from package.json and Gemfile,
// creates .quarantine/config.yml with per-suite entries, creates
// .quarantine/.gitignore, and sets up the quarantine/state branch.
func runInit(cmd *cobra.Command, args []string) error {
	cmd.SetOut(cmd.OutOrStdout())

	// Step 1: Detect frameworks from project files.
	pkgJSON, _ := os.ReadFile("package.json")
	gemfile, _ := os.ReadFile("Gemfile")
	frameworks := detectFrameworks(string(pkgJSON), string(gemfile))

	if len(frameworks) == 0 {
		cmd.Printf("No supported test frameworks detected.\n")
		cmd.Printf("Supported frameworks: jest, vitest, rspec.\n")
		cmd.Printf("Add jest or vitest to package.json devDependencies, or add gem 'rspec' to your Gemfile.\n")
		return fmt.Errorf("no frameworks detected")
	}

	cmd.Printf("Detected test frameworks: %s\n", strings.Join(frameworks, ", "))

	// Step 2: Validate GitHub token.
	token := ghclient.ResolveToken()
	if token == "" {
		cmd.Printf(`
Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.

  export QUARANTINE_GITHUB_TOKEN=ghp_your_token_here

Required token scope: repo (read/write contents, create issues, post PR comments)
`)
		return fmt.Errorf("no GitHub token")
	}

	// Step 3: Detect owner/repo from git remote.
	cwd, err := os.Getwd()
	if err != nil {
		cmd.Printf("Error: could not determine current directory: %v\n", err)
		return fmt.Errorf("getwd: %w", err)
	}

	owner, repo, err := git.ParseRemote(cwd)
	if err != nil {
		cmd.Printf("%s\n", classifyGitRemoteError(err))
		return fmt.Errorf("git remote: %w", err)
	}

	// Step 4: Create GitHub client and validate token.
	client, err := ghclient.NewClient(owner, repo)
	if err != nil {
		cmd.Printf("Error: %v\n", err)
		return fmt.Errorf("github client: %w", err)
	}

	ctx := context.Background()

	// Call 1: GET /repos/{owner}/{repo} — validate token and permissions.
	repoInfo, err := client.GetRepo(ctx)
	if err != nil {
		cmd.Printf("%s\n", classifyGitHubError(err, owner, repo))
		return fmt.Errorf("get repo: %w", err)
	}

	// Step 5: Write .quarantine/config.yml and .quarantine/.gitignore.
	if err := os.MkdirAll(".quarantine", 0755); err != nil {
		cmd.Printf("Error: failed to create .quarantine directory: %v\n", err)
		return fmt.Errorf("mkdir .quarantine: %w", err)
	}

	configContent := formatInitConfig(owner, repo, frameworks)
	if err := os.WriteFile(".quarantine/config.yml", []byte(configContent), 0644); err != nil {
		cmd.Printf("Error: failed to write .quarantine/config.yml: %v\n", err)
		return fmt.Errorf("write config: %w", err)
	}

	gitignoreContent := formatQuarantineGitignore()
	if err := os.WriteFile(".quarantine/.gitignore", []byte(gitignoreContent), 0644); err != nil {
		cmd.Printf("Error: failed to write .quarantine/.gitignore: %v\n", err)
		return fmt.Errorf("write gitignore: %w", err)
	}

	cmd.Printf("Pre-filled %d suite entries in .quarantine/config.yml\n", len(frameworks))

	// Step 6: Check and create the quarantine/state branch.
	_, branchExists, err := client.GetRef(ctx, "quarantine/state")
	if err != nil {
		cmd.Printf("Error: failed to check branch status: %v\n", err)
		return fmt.Errorf("check branch: %w", err)
	}

	if branchExists {
		cmd.Printf("Branch 'quarantine/state' already exists. Skipping branch creation.\n")
	} else {
		defaultBranch := repoInfo.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main"
		}
		baseSHA, _, err := client.GetRef(ctx, defaultBranch)
		if err != nil {
			cmd.Printf("Error: failed to get default branch SHA: %v\n", err)
			return fmt.Errorf("get ref: %w", err)
		}

		if err := client.CreateRef(ctx, "quarantine/state", baseSHA); err != nil {
			cmd.Printf("Error: failed to create branch: %v\n", err)
			return fmt.Errorf("create ref: %w", err)
		}

		readmeContent := []byte(`# quarantine/state

This branch stores quarantine state managed by the quarantine CLI.
Do not edit files on this branch manually.
`)
		if err := client.PutContents(ctx, "README.md", "quarantine: initialize state branch", readmeContent, ""); err != nil {
			cmd.Printf("Error: failed to write README.md: %v\n", err)
			return fmt.Errorf("put contents: %w", err)
		}
	}

	cmd.Printf("%s", formatInitSummary(owner, repo, frameworks, branchExists))

	return nil
}

// detectFrameworks inspects package.json and Gemfile content and returns
// the list of detected framework names (jest, vitest, rspec).
// This is a pure function — no I/O.
func detectFrameworks(pkgJSON, gemfile string) []string {
	var frameworks []string

	if pkgJSON != "" {
		// Parse package.json to detect jest and vitest keys.
		var pkg map[string]json.RawMessage
		if err := json.Unmarshal([]byte(pkgJSON), &pkg); err == nil {
			if hasPackageKey(pkg, "jest") {
				frameworks = append(frameworks, "jest")
			}
			if hasPackageKey(pkg, "vitest") {
				frameworks = append(frameworks, "vitest")
			}
		}
	}

	if gemfile != "" {
		if gemfileContainsRSpec(gemfile) {
			frameworks = append(frameworks, "rspec")
		}
	}

	return frameworks
}

// hasPackageKey returns true if the parsed package.json map contains the given
// key in "dependencies" or "devDependencies".
// This is a pure function — no I/O.
func hasPackageKey(pkg map[string]json.RawMessage, key string) bool {
	for _, section := range []string{"dependencies", "devDependencies"} {
		raw, ok := pkg[section]
		if !ok {
			continue
		}
		var deps map[string]json.RawMessage
		if err := json.Unmarshal(raw, &deps); err != nil {
			continue
		}
		if _, found := deps[key]; found {
			return true
		}
	}
	return false
}

// gemfileContainsRSpec returns true if the Gemfile content declares rspec.
// Detects both gem 'rspec' and gem "rspec" patterns.
// This is a pure function — no I/O.
func gemfileContainsRSpec(gemfile string) bool {
	return strings.Contains(gemfile, "gem 'rspec'") ||
		strings.Contains(gemfile, `gem "rspec"`)
}

// buildSuiteEntry returns the default SuiteEntry for a given framework name.
// This is a pure function — no I/O.
func buildSuiteEntry(framework string) map[string]interface{} {
	switch framework {
	case "jest":
		return map[string]interface{}{
			"name":           "jest",
			"command":        []string{"npx", "jest", "--ci"},
			"junitxml":       "junit.xml",
			"rerun_command":  []string{"npx", "jest", "--testNamePattern", "{name}"},
			"retries":        3,
		}
	case "rspec":
		return map[string]interface{}{
			"name":           "rspec",
			"command":        []string{"bundle", "exec", "rspec"},
			"junitxml":       "rspec.xml",
			"rerun_command":  []string{"bundle", "exec", "rspec", "-e", "{name}"},
			"retries":        3,
		}
	case "vitest":
		return map[string]interface{}{
			"name":          "vitest",
			"command":       []string{"npx", "vitest", "run"},
			"junitxml":      "junit-report.xml",
			"rerun_command": []string{"npx", "vitest", "run", "--reporter=junit", "{file}", "-t", "{name}"},
			"retries":       3,
		}
	}
	return map[string]interface{}{
		"name":          framework,
		"command":       []string{},
		"junitxml":      "junit.xml",
		"rerun_command": []string{},
		"retries":       3,
	}
}

// formatInitConfig generates the content of .quarantine/config.yml.
// This is a pure function — no I/O.
func formatInitConfig(owner, repo string, frameworks []string) string {
	var suites strings.Builder
	for _, fw := range frameworks {
		entry := buildSuiteEntry(fw)
		cmd := entry["command"].([]string)
		rerun := entry["rerun_command"].([]string)
		fmt.Fprintf(&suites, "  - name: %s\n", entry["name"])
		fmt.Fprintf(&suites, "    command: [%s]\n", joinQuoted(cmd))
		fmt.Fprintf(&suites, "    junitxml: %q\n", entry["junitxml"])
		fmt.Fprintf(&suites, "    rerun_command: [%s]\n", joinQuoted(rerun))
		fmt.Fprintf(&suites, "    retries: %d\n", entry["retries"])
	}

	return fmt.Sprintf(`version: 1

github:
  owner: %s
  repo: %s

issue_tracker: github
labels:
  - quarantine
notifications:
  github_pr_comment: true
storage:
  branch: quarantine/state

test_suites:
%s`, owner, repo, suites.String())
}

// joinQuoted returns a comma-separated, double-quoted list of strings for
// embedding inside a YAML flow sequence: ["npx", "jest", "--ci"].
// This is a pure function — no I/O.
func joinQuoted(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}

// formatQuarantineGitignore returns the content for .quarantine/.gitignore.
// This is a pure function — no I/O.
func formatQuarantineGitignore() string {
	return `# Ignore all runtime files. Only config.yml is source-controlled.
*
!.gitignore
!config.yml
`
}

// formatInitSummary returns the success output for quarantine init.
// This is a pure function — no I/O.
func formatInitSummary(owner, repo string, frameworks []string, branchExists bool) string {
	branchStatus := "created"
	if branchExists {
		branchStatus = "already exists"
	}
	return fmt.Sprintf(`
Quarantine initialized.
  Config:   .quarantine/config.yml (created)
  Branch:   quarantine/state (%s)

Next step: review .quarantine/config.yml, adjust suite names and commands,
then run `+"`quarantine doctor`"+` to validate.
`, branchStatus)
}

// classifyGitHubError returns a user-facing error message for a GitHub API failure.
// This is a pure function — no I/O.
func classifyGitHubError(err error, owner, repo string) string {
	errStr := err.Error()
	if strings.Contains(errStr, "403") || strings.Contains(errStr, "lacks permission") {
		return fmt.Sprintf("Error: GitHub token lacks permission to access repository '%s/%s'.\nRequired scope: repo. Check your token permissions at https://github.com/settings/tokens", owner, repo)
	} else if strings.Contains(errStr, "401") {
		return "Error: GitHub token is invalid or expired. Check QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN."
	} else if strings.Contains(errStr, "connection") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "failed") {
		return fmt.Sprintf("Error: Unable to reach GitHub API: %v.\nCheck your network connection and try again.", err)
	}
	return fmt.Sprintf("Error: GitHub API error: %v", err)
}

// classifyGitRemoteError returns a user-facing error message for a git remote parsing failure.
// This is a pure function — no I/O.
func classifyGitRemoteError(err error) string {
	errMsg := err.Error()
	if strings.Contains(errMsg, "not a git repository") {
		return "Error: Not a git repository. Run 'quarantine init' from the root of a git repository."
	} else if strings.Contains(errMsg, "not a GitHub URL") {
		return fmt.Sprintf("Error: Remote 'origin' is not a GitHub URL: %s\nQuarantine v1 supports GitHub repositories only. Set github.owner and github.repo\nin .quarantine/config.yml manually if using a non-standard remote.", extractURLFromError(errMsg))
	}
	return fmt.Sprintf("Error: %v", err)
}

// extractURLFromError extracts the URL from a "not a GitHub URL: <url>" error message.
func extractURLFromError(errMsg string) string {
	const prefix = "not a GitHub URL: "
	idx := strings.LastIndex(errMsg, prefix)
	if idx == -1 {
		return errMsg
	}
	return errMsg[idx+len(prefix):]
}
