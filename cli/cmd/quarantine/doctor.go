package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mycargus/quarantine/cli/internal/config"
	"github.com/mycargus/quarantine/cli/internal/git"
	ghclient "github.com/mycargus/quarantine/cli/internal/github"
	"github.com/spf13/cobra"
)

// runDoctor implements the doctor command logic.
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

	// Check for GitHub token — append warning if missing.
	if ghclient.ResolveToken() == "" {
		warns = append(warns, "No GitHub token found in environment. 'quarantine run' will fail unless QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN is set.")
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

	// github.owner / github.repo — auto-detect if not set from config.
	var detectedOwner, detectedRepo string
	if cfg.GitHub.Owner == "" || cfg.GitHub.Repo == "" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			detectedOwner, detectedRepo, _ = git.ParseRemote(cwd)
		}
	}
	owner, repo, ownerNote, repoNote := resolveDisplayOwnerRepo(cfg.GitHub.Owner, cfg.GitHub.Repo, detectedOwner, detectedRepo)
	cmd.Printf("  github.owner:    %s%s\n", owner, ownerNote)
	cmd.Printf("  github.repo:     %s%s\n", repo, repoNote)

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

	if len(warns) > 0 {
		cmd.Printf("\nWarnings:\n")
		for _, w := range warns {
			cmd.Printf("  - %s\n", w)
		}
	}

	return nil
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

// resolveDisplayOwnerRepo determines owner/repo display values with annotation notes.
// This is a pure function — no I/O.
func resolveDisplayOwnerRepo(cfgOwner, cfgRepo, detectedOwner, detectedRepo string) (owner, repo, ownerNote, repoNote string) {
	owner, repo = cfgOwner, cfgRepo
	if owner == "" {
		owner = detectedOwner
		if owner != "" {
			ownerNote = " (auto-detected)"
		}
	}
	if repo == "" {
		repo = detectedRepo
		if repo != "" {
			repoNote = " (auto-detected)"
		}
	}
	return
}


