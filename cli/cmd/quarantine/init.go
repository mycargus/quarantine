package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mycargus/quarantine/cli/internal/detect"
	"github.com/mycargus/quarantine/cli/internal/git"
	ghclient "github.com/mycargus/quarantine/cli/internal/github"
)

// fileExists reports whether the file at path exists.
// This is an I/O helper — not pure.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// runInit implements the `quarantine init` command.
// It auto-detects test frameworks from package.json and Gemfile,
// creates .quarantine/config.yml with per-suite entries (if not already present),
// creates .quarantine/.gitignore (if not already present), and sets up the
// quarantine/state branch. Re-running is safe — existing artifacts are skipped.
func runInit(cmd *cobra.Command, args []string) error {
	cmd.SetOut(cmd.OutOrStdout())

	// Step 1: Resolve working directory (fail-fast before any other work).
	cwd, err := os.Getwd()
	if err != nil {
		cmd.Printf("Error: could not determine current directory: %v\n", err)
		return fmt.Errorf("getwd: %w", err)
	}

	// Step 2: Detect frameworks from project files.
	detected := detect.Scan(cwd)
	frameworks := detected.Names()

	if len(frameworks) == 0 {
		cmd.Printf("No test frameworks detected.\n")
	} else {
		cmd.Printf("Detected test frameworks: %s\n", strings.Join(frameworks, ", "))
	}

	// Step 3: Validate GitHub token.
	token := ghclient.ResolveToken()
	if token == "" {
		cmd.Printf(`
Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.

  export QUARANTINE_GITHUB_TOKEN=ghp_your_token_here

Required token scope: repo (read/write contents, create issues, post PR comments)
`)
		return fmt.Errorf("no GitHub token")
	}

	// Step 5: Detect owner/repo from git remote.

	owner, repo, err := git.ParseRemote(cwd)
	if err != nil {
		cmd.Printf("%s\n", classifyGitRemoteError(err))
		return fmt.Errorf("git remote: %w", err)
	}

	// Step 6: Create GitHub client and validate token.
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

	// Step 7: Write .quarantine/config.yml and .quarantine/.gitignore (skip if existing).
	if err := os.MkdirAll(".quarantine", 0755); err != nil {
		cmd.Printf("Error: failed to create .quarantine directory: %v\n", err)
		return fmt.Errorf("mkdir .quarantine: %w", err)
	}

	configSkipped := fileExists(".quarantine/config.yml")
	if configSkipped {
		cmd.Printf(".quarantine/config.yml already exists — skipping.\n")
	} else {
		var configContent string
		if len(frameworks) == 0 {
			configContent = formatNoFrameworkConfig(owner, repo)
		} else {
			configContent = formatInitConfig(owner, repo, frameworks)
		}
		if err := os.WriteFile(".quarantine/config.yml", []byte(configContent), 0644); err != nil {
			cmd.Printf("Error: failed to write .quarantine/config.yml: %v\n", err)
			return fmt.Errorf("write config: %w", err)
		}
		if len(frameworks) > 0 {
			cmd.Printf("Pre-filled %d suite entries in .quarantine/config.yml\n", len(frameworks))
		}
	}

	gitignoreSkipped := fileExists(".quarantine/.gitignore")
	if gitignoreSkipped {
		cmd.Printf(".quarantine/.gitignore already exists — skipping.\n")
	} else {
		gitignoreContent := formatQuarantineGitignore()
		if err := os.WriteFile(".quarantine/.gitignore", []byte(gitignoreContent), 0644); err != nil {
			cmd.Printf("Error: failed to write .quarantine/.gitignore: %v\n", err)
			return fmt.Errorf("write gitignore: %w", err)
		}
	}

	// Step 8: Check and create the quarantine/state branch.
	_, branchExists, err := client.GetRef(ctx, "quarantine/state")
	if err != nil {
		cmd.Printf("Error: failed to check branch status: %v\n", err)
		return fmt.Errorf("check branch: %w", err)
	}

	recoveryMode := isRecoveryMode(configSkipped, gitignoreSkipped, branchExists)

	if branchExists {
		cmd.Printf("quarantine/state branch already exists — skipping.\n")
	} else {
		if recoveryMode {
			cmd.Printf("quarantine/state branch not found — creating.\n")
		}
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
	}

	// Write .quarantine/README.md if it does not already exist on the state branch.
	// Checked independently of branch creation so that re-running init is idempotent
	// even when a previous attempt failed after creating the branch.
	_, readmeSHA, readmeGetErr := client.GetContents(ctx, ".quarantine/README.md", "quarantine/state")
	if readmeGetErr == nil && readmeSHA == "" {
		readmeContent := []byte(`# quarantine/state

This branch stores quarantine state managed by the quarantine CLI.
Do not edit files on this branch manually.
`)
		if err := client.PutContents(ctx, ".quarantine/README.md", "quarantine: initialize state branch", readmeContent, ""); err != nil {
			cmd.Printf("Error: failed to write .quarantine/README.md: %v\n", err)
			return fmt.Errorf("put contents: %w", err)
		}
	}

	cmd.Printf("GitHub token validated.\n")

	if configSkipped && gitignoreSkipped && branchExists {
		cmd.Printf("\n%s", formatAlreadyInitializedSummary())
	} else if recoveryMode {
		cmd.Printf("\n%s", formatRecoverySummary())
	} else {
		cmd.Printf("%s", formatInitSummary(owner, repo, frameworks, branchExists))
	}

	return nil
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

// formatNoFrameworkConfig generates the content of .quarantine/config.yml when
// no test frameworks were detected. It uses a commented example suite entry so
// the developer has a template to work from.
// This is a pure function — no I/O.
func formatNoFrameworkConfig(owner, repo string) string {
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
  # Add your test suites here. Example:
  # - name: unit
  #   command: ["npm", "test"]
  #   junitxml: "junit.xml"
  #   rerun_command: ["npx", "jest", "--testNamePattern", "{name}"]
  #   retries: 3
`, owner, repo)
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

// formatAlreadyInitializedSummary returns the output when all quarantine
// artifacts already exist and no changes were made.
// This is a pure function — no I/O.
func formatAlreadyInitializedSummary() string {
	return "Quarantine is already initialized. Edit .quarantine/config.yml to add test suites.\n"
}

// isRecoveryMode returns true when config and .gitignore were already present
// but the quarantine/state branch did not exist — indicating a recovery scenario.
// This is a pure function — no I/O.
func isRecoveryMode(configSkipped, gitignoreSkipped, branchExists bool) bool {
	return configSkipped && gitignoreSkipped && !branchExists
}

// formatRecoverySummary returns the output when the quarantine/state branch was
// recreated because it was missing while config and .gitignore already existed.
// This is a pure function — no I/O.
func formatRecoverySummary() string {
	return `Quarantine recovered. The state branch has been recreated.
Previous quarantine state was on the deleted branch and is not recoverable.
`
}

// formatInitSummary returns the success output for quarantine init.
// This is a pure function — no I/O.
func formatInitSummary(owner, repo string, frameworks []string, branchExists bool) string {
	branchStatus := "created"
	if branchExists {
		branchStatus = "already exists"
	}
	var nextStep string
	if len(frameworks) == 0 {
		nextStep = "Next step: edit .quarantine/config.yml to add your test suites,\nthen run `quarantine doctor` to validate."
	} else {
		nextStep = "Next step: review .quarantine/config.yml, adjust suite names and commands,\nthen run `quarantine doctor` to validate."
	}
	return fmt.Sprintf(`
Quarantine initialized.
  Config:   .quarantine/config.yml (created)
  Branch:   quarantine/state (%s)

%s
`, branchStatus, nextStep)
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
