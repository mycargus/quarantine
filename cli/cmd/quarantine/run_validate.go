package main

import (
	"errors"

	"github.com/mycargus/quarantine/cli/internal/config"
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
