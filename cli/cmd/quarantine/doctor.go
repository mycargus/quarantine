package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mycargus/quarantine/cli/internal/config"
	ghclient "github.com/mycargus/quarantine/cli/internal/github"
	"github.com/spf13/cobra"
)

// runDoctor implements the `quarantine doctor` command per ADR-037.
//
// Doctor MUST:
//  1. Read `github.owner` and `github.repo` from `.quarantine/config.yml`
//     only — it MUST NOT inspect the git origin URL.
//  2. Make a single `GET /repos/{owner}/{repo}` reachability call.
//  3. Exit 0 on 200; surface the GitHub status on 4xx and exit 2.
//  4. NOT introspect `response.permissions` or `response.has_issues` — those
//     are token-scope diagnostics and out of doctor's scope.
func runDoctor(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(".quarantine/config.yml")
	if err != nil {
		cmd.Printf("Error: quarantine.yml not found in the current directory.\nRun 'quarantine init' to create one.\n")
		return fmt.Errorf("quarantine.yml not found")
	}

	cfg.ApplyDefaults()

	errs, warns := cfg.Validate()

	// Validate test_suites when present.
	if len(cfg.TestSuites) > 0 {
		errs = append(errs, config.ValidateSuites(cfg.TestSuites)...)
	}

	if len(errs) > 0 {
		cmd.Printf("quarantine.yml has errors.\n\nErrors:\n")
		for _, e := range errs {
			cmd.Printf("  - %s\n", e)
		}
		if len(warns) > 0 {
			cmd.Printf("\nWarnings:\n")
			for _, w := range warns {
				cmd.Printf("  - %s\n", w)
			}
		}
		return fmt.Errorf("quarantine.yml has errors")
	}

	// Valid config — print resolved configuration.
	cmd.Printf("quarantine.yml is valid.\n\nResolved configuration:\n")
	cmd.Printf("  version:         %d\n", cfg.Version)
	cmd.Printf("  retries:         %d\n", cfg.Retries)
	cmd.Printf("  issue_tracker:   %s\n", cfg.IssueTracker)
	cmd.Printf("  labels:          [%s]\n", strings.Join(cfg.Labels, ", "))

	prComment := false
	if cfg.Notifications.GitHubPRComment != nil {
		prComment = *cfg.Notifications.GitHubPRComment
	}
	cmd.Printf("  notifications:   github_pr_comment: %v\n", prComment)

	branchNote := ""
	if cfg.Storage.Branch == "quarantine/state" {
		branchNote = " (default)"
	}
	cmd.Printf("  storage.branch:  %s%s\n", cfg.Storage.Branch, branchNote)

	// Scan for retryTimes in jest config files relative to the repo root.
	retryHits := detectRetryTimesInRepo(".quarantine/config.yml")
	if len(retryHits) > 0 {
		cmd.Printf("\nWarning: %s contains 'retryTimes'. Framework-level retries hide\nfailures from JUnit XML, preventing quarantine from detecting flaky tests.\nRemove retryTimes before using quarantine.\n", strings.Join(retryHits, ", "))
	}

	// Validate github.owner / github.repo per ADR-037 — required, no auto-detect.
	if err := validateGitHubFields(cfg); err != nil {
		cmd.Printf("\n%s\n", err.Error())
		return exitCodeError(2)
	}

	// Validate token — surface canonical missing-token error and exit 2.
	if err := validateGitHubToken(); err != nil {
		cmd.Printf("\n%s\n", err.Error())
		return exitCodeError(2)
	}

	// Single reachability call: GET /repos/{owner}/{repo}.
	client, err := ghclient.NewClient(cfg.GitHub.Owner, cfg.GitHub.Repo)
	if err != nil {
		cmd.Printf("\nError: %v\n", err)
		return exitCodeError(2)
	}
	if _, err := client.GetRepo(context.Background()); err != nil {
		cmd.Printf("\nError: %v\n", err)
		return exitCodeError(2)
	}

	cmd.Printf("\n%s", formatDoctorReachableSummary(cfg.GitHub.Owner, cfg.GitHub.Repo))

	return nil
}

// formatDoctorReachableSummary returns the M20 success summary printed by
// `quarantine doctor` after a successful reachability check. Per ADR-037, the
// summary explicitly notes the owner/repo were read from config (not detected
// from origin) and that the target is reachable.
//
// This is a pure function — no I/O.
func formatDoctorReachableSummary(owner, repo string) string {
	return fmt.Sprintf(
		"github.owner:  %s (from config)\n"+
			"github.repo:   %s (from config)\n"+
			"token:         authenticated\n"+
			"target:        reachable\n",
		owner, repo,
	)
}

// detectRetryTimesInRepo reads known jest config files and top-level test files
// from the repo root and returns file paths where a non-zero retryTimes value
// was found.
//
// I/O shell — reads files from disk, delegates detection to detectRetryTimes.
func detectRetryTimesInRepo(configPath string) []string {
	// Determine the repo root from the config path.
	// If the config lives inside a ".quarantine" directory, go one level up.
	configDir := filepath.Dir(configPath)
	repoRoot := configDir
	if filepath.Base(configDir) == ".quarantine" {
		repoRoot = filepath.Dir(configDir)
	}

	// Named jest config files to always check.
	namedCandidates := []string{
		"jest.config.js",
		"jest.config.ts",
		"jest.config.mjs",
	}

	files := make(map[string]string)
	for _, name := range namedCandidates {
		data, err := os.ReadFile(filepath.Join(repoRoot, name))
		if err == nil {
			files[name] = string(data)
		}
	}

	// Also scan top-level test files for call-style retryTimes(N) usage.
	for _, pat := range []string{"*.test.js", "*.test.ts", "*.spec.js", "*.spec.ts"} {
		matches, _ := filepath.Glob(filepath.Join(repoRoot, pat))
		for _, m := range matches {
			data, err := os.ReadFile(m)
			if err == nil {
				files[filepath.Base(m)] = string(data)
			}
		}
	}

	return detectRetryTimes(files)
}
