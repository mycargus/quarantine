package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mycargus/quarantine/internal/config"
	"github.com/mycargus/quarantine/internal/git"
	"github.com/spf13/cobra"
)

// runDoctor implements the doctor command logic.
func runDoctor(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(configPath)
	if err != nil {
		cmd.Printf("Error: quarantine.yml not found in the current directory.\nRun 'quarantine init' to create one.\n")
		return fmt.Errorf("quarantine.yml not found")
	}

	cfg.ApplyDefaults()

	errs, warns := cfg.Validate()

	// Check for GitHub token — append warning if missing.
	if githubToken() == "" {
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
	cmd.Printf("  framework:       %s\n", cfg.Framework)
	cmd.Printf("  retries:         %d\n", cfg.Retries)

	defaultJunit := frameworkDefaultJUnit(cfg.Framework)
	junitxmlNote := ""
	if cfg.JUnitXML == defaultJunit {
		junitxmlNote = " (default)"
	}
	cmd.Printf("  junitxml:        %s%s\n", cfg.JUnitXML, junitxmlNote)

	// github.owner / github.repo — auto-detect if not set from config.
	owner, repo := cfg.GitHub.Owner, cfg.GitHub.Repo
	ownerNote, repoNote := "", ""
	if owner == "" || repo == "" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			detectedOwner, detectedRepo, remoteErr := git.ParseRemote(cwd)
			if remoteErr == nil {
				if owner == "" {
					owner = detectedOwner
					ownerNote = " (auto-detected)"
				}
				if repo == "" {
					repo = detectedRepo
					repoNote = " (auto-detected)"
				}
			}
		}
	}
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

	if len(warns) > 0 {
		cmd.Printf("\nWarnings:\n")
		for _, w := range warns {
			cmd.Printf("  - %s\n", w)
		}
	}

	return nil
}

// frameworkDefaultJUnit returns the default junitxml glob for a framework.
func frameworkDefaultJUnit(framework string) string {
	defaults := map[string]string{
		"jest":   "junit.xml",
		"rspec":  "rspec.xml",
		"vitest": "junit-report.xml",
	}
	return defaults[framework]
}

// githubToken resolves the GitHub token from environment variables.
func githubToken() string {
	if t := os.Getenv("QUARANTINE_GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GITHUB_TOKEN")
}

