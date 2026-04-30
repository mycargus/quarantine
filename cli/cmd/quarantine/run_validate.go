package main

import (
	"errors"
	"fmt"

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

// formatStateBranchCreatedMessage returns the canonical stderr message printed
// by `quarantine run` after it has successfully created the state branch on a
// first invocation (per ADR-038).
//
// This is a pure function — no I/O.
func formatStateBranchCreatedMessage(branch string) string {
	return fmt.Sprintf("[quarantine] State branch '%s' created.", branch)
}

// formatStateBranchCreationFailedWarning returns the body of the
// `[quarantine] WARNING:` printed by `quarantine run` when self-bootstrap
// branch creation fails for non-benign reasons (403, 5xx, network) and the
// run continues in degraded mode (per ADR-038).
//
// The caller is responsible for prefixing the result with `[quarantine] WARNING: `
// when emitting it to stderr (matching the existing degraded-mode warning style).
//
// This is a pure function — no I/O.
func formatStateBranchCreationFailedWarning(branch, reason string) string {
	return fmt.Sprintf("Cannot create state branch '%s': %s. Continuing in degraded mode.", branch, reason)
}
