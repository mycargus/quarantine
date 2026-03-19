package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/mycargus/quarantine/internal/git"
	ghclient "github.com/mycargus/quarantine/internal/github"
	"github.com/mycargus/quarantine/internal/quarantine"
)

// runInit implements the `quarantine init` command.
func runInit(cmd *cobra.Command, args []string) error {
	in := bufio.NewReader(cmd.InOrStdin())

	// Step 1: Check for existing quarantine.yml.
	configPath := "quarantine.yml"
	if _, err := os.Stat(configPath); err == nil {
		cmd.Printf("quarantine.yml already exists. Overwrite? [y/N] ")
		answer, _ := in.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			cmd.Printf("Aborted. Existing quarantine.yml preserved.\n")
			return nil
		}
	}

	// Step 2: Prompt for framework.
	framework := ""
	validFrameworks := map[string]bool{"jest": true, "rspec": true, "vitest": true}
	for {
		cmd.Printf("Which test framework? [rspec/jest/vitest] ")
		input, _ := in.ReadString('\n')
		input = strings.TrimSpace(input)
		if validFrameworks[input] {
			framework = input
			break
		}
		cmd.Printf("Invalid framework '%s'. Supported: rspec, jest, vitest.\n", input)
	}

	// Step 3: Prompt for retries.
	retries := 3
	cmd.Printf("How many retries for failing tests? [3] ")
	retriesInput, _ := in.ReadString('\n')
	retriesInput = strings.TrimSpace(retriesInput)
	if retriesInput != "" {
		if n, err := strconv.Atoi(retriesInput); err == nil {
			retries = n
		}
	}

	// Step 4: Prompt for junitxml path.
	defaultJUnit := frameworkDefaultJUnit(framework)
	cmd.Printf("Path/glob for JUnit XML output? [%s] ", defaultJUnit)
	junitInput, _ := in.ReadString('\n')
	junitInput = strings.TrimSpace(junitInput)
	junitxml := defaultJUnit
	if junitInput != "" {
		junitxml = junitInput
	}

	// Step 5: Write quarantine.yml (BEFORE GitHub operations).
	if err := writeConfig(configPath, framework, retries, junitxml, defaultJUnit); err != nil {
		cmd.Printf("Error: failed to write quarantine.yml: %v\n", err)
		return fmt.Errorf("write config: %w", err)
	}

	// Step 6: Validate GitHub token.
	token := githubToken()
	if token == "" {
		cmd.Printf(`
Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.

  export QUARANTINE_GITHUB_TOKEN=ghp_your_token_here

Required token scope: repo (read/write contents, create issues, post PR comments)
`)
		return fmt.Errorf("no GitHub token")
	}

	// Step 7: Detect owner/repo from git remote.
	cwd, err := os.Getwd()
	if err != nil {
		cmd.Printf("Error: could not determine current directory: %v\n", err)
		return fmt.Errorf("getwd: %w", err)
	}

	owner, repo, err := git.ParseRemote(cwd)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not a git repository") {
			cmd.Printf("Error: Not a git repository. Run 'quarantine init' from the root of a git repository.\n")
		} else if strings.Contains(errMsg, "not a GitHub URL") {
			// Extract the URL from the error for the detailed message.
			// The git package returns "remote 'origin' is not a GitHub URL: <url>"
			cmd.Printf("Error: Remote 'origin' is not a GitHub URL: %s\nQuarantine v1 supports GitHub repositories only. Set github.owner and github.repo\nin quarantine.yml manually if using a non-standard remote.\n",
				extractURLFromError(errMsg))
		} else {
			cmd.Printf("Error: %v\n", err)
		}
		return fmt.Errorf("git remote: %w", err)
	}

	// Step 8: Create GitHub client and run API calls in required order.
	client, err := ghclient.NewClient(owner, repo)
	if err != nil {
		cmd.Printf("Error: %v\n", err)
		return fmt.Errorf("github client: %w", err)
	}

	ctx := context.Background()

	// Call 1: GET /repos/{owner}/{repo} — validate token and permissions.
	cmd.Printf("\nValidating GitHub token... ")
	repoInfo, err := client.GetRepo(ctx)
	if err != nil {
		cmd.Printf("FAILED\n")
		printGitHubError(cmd, err, owner, repo)
		return fmt.Errorf("get repo: %w", err)
	}
	cmd.Printf("OK\n")
	cmd.Printf("Testing repository access (%s/%s)... OK\n", owner, repo)

	// Call 2: GET /repos/{owner}/{repo}/git/ref/heads/quarantine/state — check if branch exists.
	_, branchExists, err := client.GetRef(ctx, "quarantine/state")
	if err != nil {
		cmd.Printf("Error: failed to check branch status: %v\n", err)
		return fmt.Errorf("check branch: %w", err)
	}

	if branchExists {
		cmd.Printf("Branch 'quarantine/state' already exists. Skipping branch creation.\n")
	} else {
		// Call 3: GET /repos/{owner}/{repo}/git/ref/heads/{default_branch} — get HEAD SHA.
		defaultBranch := repoInfo.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main"
		}
		baseSHA, _, err := client.GetRef(ctx, defaultBranch)
		if err != nil {
			cmd.Printf("Error: failed to get default branch SHA: %v\n", err)
			return fmt.Errorf("get ref: %w", err)
		}

		// Call 4: POST /repos/{owner}/{repo}/git/refs — create branch.
		cmd.Printf("Creating quarantine/state branch... ")
		if err := client.CreateRef(ctx, "quarantine/state", baseSHA); err != nil {
			cmd.Printf("FAILED\n")
			cmd.Printf("Error: failed to create branch: %v\n", err)
			return fmt.Errorf("create ref: %w", err)
		}
		cmd.Printf("OK\n")

		// Call 5: PUT /repos/{owner}/{repo}/contents/quarantine.json — write empty state.
		emptyState := quarantine.NewEmptyState()
		stateJSON, err := emptyState.Marshal()
		if err != nil {
			cmd.Printf("Error: failed to marshal quarantine.json: %v\n", err)
			return fmt.Errorf("marshal state: %w", err)
		}

		if err := client.PutContents(ctx, "quarantine.json", "quarantine: initialize empty state", stateJSON, ""); err != nil {
			cmd.Printf("Error: failed to write quarantine.json: %v\n", err)
			return fmt.Errorf("put contents: %w", err)
		}
	}

	// Jest-specific recommendation.
	if framework == "jest" {
		cmd.Printf(`
Recommended jest-junit configuration (in jest.config.js or package.json):

  "jest-junit": {
    "classNameTemplate": "{classname}",
    "titleTemplate": "{title}",
    "ancestorSeparator": " > ",
    "addFileAttribute": "true"
  }

This produces well-structured JUnit XML for quarantine's test identification.
`)
	}

	// Print summary.
	branchStatus := "created"
	if branchExists {
		branchStatus = "already exists"
	}
	workflowSnippet := frameworkWorkflowSnippet(framework, junitxml)

	cmd.Printf(`
Quarantine initialized successfully.

  Config:     quarantine.yml (created)
  Framework:  %s
  Retries:    %d
  JUnit XML:  %s
  Branch:     quarantine/state (%s)

Next steps:
  1. Add quarantine to your CI workflow:

     - name: Run tests
       run: %s
       env:
         QUARANTINE_GITHUB_TOKEN: ${{ secrets.QUARANTINE_GITHUB_TOKEN }}

     - name: Upload quarantine results
       if: always()
       uses: actions/upload-artifact@v4
       with:
         name: quarantine-results-${{ github.run_id }}
         path: .quarantine/results.json

  2. Run `+"`quarantine doctor`"+` to verify your configuration.
`, framework, retries, junitxml, branchStatus, workflowSnippet)

	return nil
}

// writeConfig writes quarantine.yml with the given settings.
// Fields that match defaults are omitted except version and framework.
func writeConfig(path, framework string, retries int, junitxml, defaultJUnit string) error {
	// Build minimal config map — always include version and framework.
	// Omit fields that match defaults.
	cfg := map[string]interface{}{
		"version":   1,
		"framework": framework,
	}

	if retries != 3 {
		cfg["retries"] = retries
	}

	if junitxml != defaultJUnit {
		cfg["junitxml"] = junitxml
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// printGitHubError prints a user-friendly error message for GitHub API errors.
func printGitHubError(cmd *cobra.Command, err error, owner, repo string) {
	errStr := err.Error()
	if strings.Contains(errStr, "403") || strings.Contains(errStr, "lacks permission") {
		cmd.Printf("Error: GitHub token lacks permission to access repository '%s/%s'.\nRequired scope: repo. Check your token permissions at https://github.com/settings/tokens\n", owner, repo)
	} else if strings.Contains(errStr, "401") {
		cmd.Printf("Error: GitHub token is invalid or expired. Check QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.\n")
	} else if strings.Contains(errStr, "connection") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "failed") {
		cmd.Printf("Error: Unable to reach GitHub API: %v.\nCheck your network connection and try again.\n", err)
	} else {
		cmd.Printf("Error: GitHub API error: %v\n", err)
	}
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

// frameworkWorkflowSnippet returns the recommended CI workflow run snippet for a framework.
func frameworkWorkflowSnippet(framework, junitxml string) string {
	switch framework {
	case "jest":
		return "quarantine run -- jest --ci --reporters=default --reporters=jest-junit"
	case "rspec":
		return fmt.Sprintf("quarantine run -- rspec --format RspecJunitFormatter --out %s", junitxml)
	case "vitest":
		return "quarantine run -- vitest run --reporter=junit"
	default:
		return "quarantine run -- <your test command>"
	}
}
