package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mycargus/quarantine/cli/internal/config"
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
//
// Per ADR-037, init is a two-phase flow:
//
//   - Phase 1 (config does not yet exist): detect frameworks, scan
//     `git remote -v` for advisory github.com hints, write a partial
//     `.quarantine/config.yml` with an empty `github` block plus hint comments,
//     write `.quarantine/.gitignore`, print hand-edit instructions, and exit 2.
//     Phase 1 makes no GitHub API call and does not require a token.
//
//   - Phase 2 (config exists with non-empty `github.owner` and `github.repo`):
//     validate the GitHub token, create the `quarantine/state` branch
//     (idempotent), and exit 0.
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

	// Phase 1: no .quarantine/config.yml yet — write a partial config with an
	// empty github block (and any github.com remote hints), print hand-edit
	// instructions, and exit 2. Phase 1 makes no GitHub API call.
	if !fileExists(".quarantine/config.yml") {
		hints := git.ScanGitHubRemotes(cwd)
		if err := os.MkdirAll(".quarantine", 0755); err != nil {
			cmd.Printf("Error: failed to create .quarantine directory: %v\n", err)
			return fmt.Errorf("mkdir .quarantine: %w", err)
		}
		configContent := formatPartialConfig(frameworks, hints)
		if err := os.WriteFile(".quarantine/config.yml", []byte(configContent), 0644); err != nil {
			cmd.Printf("Error: failed to write .quarantine/config.yml: %v\n", err)
			return fmt.Errorf("write config: %w", err)
		}
		gitignoreContent := formatQuarantineGitignore()
		if err := os.WriteFile(".quarantine/.gitignore", []byte(gitignoreContent), 0644); err != nil {
			cmd.Printf("Error: failed to write .quarantine/.gitignore: %v\n", err)
			return fmt.Errorf("write gitignore: %w", err)
		}
		cmd.Printf("%s", formatPhase1ExitMessage())
		return exitCodeError(2)
	}

	// Phase 2 (config exists): read owner/repo from config (NOT git origin),
	// validate token, create the quarantine/state branch (idempotent), exit 0.

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

	// Step 5: Read owner/repo from .quarantine/config.yml. Per ADR-037, the
	// CLI MUST NOT inspect the git origin URL in phase 2.
	cfg, err := config.Load(".quarantine/config.yml")
	if err != nil {
		cmd.Printf("Error [config]: failed to read .quarantine/config.yml: %v\n", err)
		return exitCodeError(2)
	}
	if cfg.GitHub.Owner == "" || cfg.GitHub.Repo == "" {
		cmd.Printf("%s", formatPhase1ExitMessage())
		return exitCodeError(2)
	}
	owner := cfg.GitHub.Owner
	repo := cfg.GitHub.Repo

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

	cmd.Printf("%s", formatPhase2Summary(owner, repo, branchExists))

	return nil
}

// formatPhase2Summary returns the M20 phase-2 setup-complete summary printed
// when `quarantine init` re-runs against an existing `.quarantine/config.yml`
// with valid `github.owner` and `github.repo`. The branch parenthetical is
// "created" when init created `quarantine/state` and "already exists" when
// the branch was already present (idempotent re-run, NFR-2.2.4).
// This is a pure function — no I/O.
func formatPhase2Summary(owner, repo string, branchExists bool) string {
	branchStatus := "created"
	if branchExists {
		branchStatus = "already exists"
	}
	return fmt.Sprintf(`
Quarantine initialized.
  github.owner:  %s (from config)
  github.repo:   %s (from config)
  Branch:        quarantine/state (%s)
`, owner, repo, branchStatus)
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

// formatPartialConfig generates the content of `.quarantine/config.yml` for
// `quarantine init` phase 1. It emits an empty `github` block with placeholder
// comments, optional github.com hint comments, and the same `test_suites`
// entries (or commented example) used by the legacy formatters.
// This is a pure function — no I/O.
func formatPartialConfig(frameworks []string, hints []git.GitHubRemoteHint) string {
	var b strings.Builder
	b.WriteString("version: 1\n\n")
	b.WriteString("github:\n")
	b.WriteString("  owner: # set to your GitHub organization or user\n")
	b.WriteString("  repo:  # set to your GitHub repository name\n")
	if len(hints) > 0 {
		b.WriteString("  # detected GitHub remotes (review before using):\n")
		for _, h := range hints {
			fmt.Fprintf(&b, "  #   %s -> %s/%s\n", h.Name, h.Owner, h.Repo)
		}
	}
	b.WriteString("\n")
	b.WriteString("issue_tracker: github\n")
	b.WriteString("labels:\n")
	b.WriteString("  - quarantine\n")
	b.WriteString("notifications:\n")
	b.WriteString("  github_pr_comment: true\n")
	b.WriteString("storage:\n")
	b.WriteString("  branch: quarantine/state\n\n")
	b.WriteString("test_suites:\n")
	if len(frameworks) == 0 {
		b.WriteString("  # Add your test suites here. Example:\n")
		b.WriteString("  # - name: unit\n")
		b.WriteString("  #   command: [\"npm\", \"test\"]\n")
		b.WriteString("  #   junitxml: \"junit.xml\"\n")
		b.WriteString("  #   rerun_command: [\"npx\", \"jest\", \"--testNamePattern\", \"{name}\"]\n")
		b.WriteString("  #   retries: 3\n")
		return b.String()
	}
	for _, fw := range frameworks {
		entry := buildSuiteEntry(fw)
		cmdParts := entry["command"].([]string)
		rerun := entry["rerun_command"].([]string)
		fmt.Fprintf(&b, "  - name: %s\n", entry["name"])
		fmt.Fprintf(&b, "    command: [%s]\n", joinQuoted(cmdParts))
		fmt.Fprintf(&b, "    junitxml: %q\n", entry["junitxml"])
		fmt.Fprintf(&b, "    rerun_command: [%s]\n", joinQuoted(rerun))
		fmt.Fprintf(&b, "    retries: %d\n", entry["retries"])
	}
	return b.String()
}

// formatPhase1ExitMessage returns the user-facing message printed when
// `quarantine init` phase 1 has written a partial config and the developer
// must hand-edit `github.owner` and `github.repo` before re-running.
// This is a pure function — no I/O.
func formatPhase1ExitMessage() string {
	return `Error [config]: github.owner and github.repo are required.
.quarantine/config.yml has been created. Edit it to set:

  github:
    owner: <your-github-org-or-user>
    repo:  <your-github-repo-name>

Then re-run 'quarantine init' to complete setup.
`
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

// isRecoveryMode returns true when config and .gitignore were already present
// but the quarantine/state branch did not exist — indicating a recovery scenario.
// This is a pure function — no I/O.
func isRecoveryMode(configSkipped, gitignoreSkipped, branchExists bool) bool {
	return configSkipped && gitignoreSkipped && !branchExists
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
