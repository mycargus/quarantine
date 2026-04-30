package main

import (
	"errors"

	"github.com/mycargus/quarantine/cli/internal/config"
	gh "github.com/mycargus/quarantine/cli/internal/github"
)

// validateGitHubFields verifies that `github.owner` and `github.repo` are
// present and non-empty in the loaded config. Returns nil when both are set;
// returns a typed config error message (per Scenario 177 / ADR-037) when
// either is missing or empty.
//
// This is a pure function — no I/O.
func validateGitHubFields(cfg *config.Config) error {
	if cfg.GitHub.Owner == "" || cfg.GitHub.Repo == "" {
		return errors.New(formatRunMissingGitHubFieldsError())
	}
	return nil
}

// formatRunMissingGitHubFieldsError returns the canonical user-facing error
// message printed when `quarantine run` finds the loaded config is missing
// `github.owner` or `github.repo`.
//
// This is a pure function — no I/O.
func formatRunMissingGitHubFieldsError() string {
	return `Error [config]: github.owner and github.repo are required in .quarantine/config.yml.
Run 'quarantine init' or edit the config to add them.`
}

// tokenMissingError returns the canonical missing-token error when the
// supplied token string is empty; returns nil otherwise. Per Scenario 178 /
// ADR-037, `quarantine run` must fail fast with this exact message when
// neither QUARANTINE_GITHUB_TOKEN nor GITHUB_TOKEN is present in the
// environment.
//
// This is a pure function — no I/O.
func tokenMissingError(token string) error {
	if token == "" {
		return errors.New(formatRunMissingTokenError())
	}
	return nil
}

// formatRunMissingTokenError returns the canonical user-facing error message
// printed when `quarantine run` cannot resolve a GitHub token from
// QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN.
//
// This is a pure function — no I/O.
func formatRunMissingTokenError() string {
	return "Error: No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN."
}

// validateGitHubToken returns the canonical missing-token error when neither
// QUARANTINE_GITHUB_TOKEN nor GITHUB_TOKEN is set; returns nil otherwise.
//
// This is a thin I/O shell over gh.ResolveToken() (which reads env vars). The
// pure decision lives in tokenMissingError; this function only adapts env I/O
// to that decision.
func validateGitHubToken() error {
	return tokenMissingError(gh.ResolveToken())
}
