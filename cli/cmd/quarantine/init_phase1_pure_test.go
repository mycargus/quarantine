package main

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/git"
)

// --- formatPartialConfig unit tests ---

func TestFormatPartialConfigNoFrameworksNoHints(t *testing.T) {
	cfg := formatPartialConfig(nil, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks and no github.com hints",
		Should:   "include version: 1",
		Actual:   strings.Contains(cfg, "version: 1"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks and no github.com hints",
		Should:   "include the empty github block placeholder for owner",
		Actual:   strings.Contains(cfg, "owner: # set to your GitHub organization or user"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks and no github.com hints",
		Should:   "include the empty github block placeholder for repo",
		Actual:   strings.Contains(cfg, "repo:  # set to your GitHub repository name"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no github.com hints",
		Should:   "omit the 'detected GitHub remotes' hint block",
		Actual:   !strings.Contains(cfg, "detected GitHub remotes"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no frameworks detected",
		Should:   "include the commented test_suites example",
		Actual:   strings.Contains(cfg, "# Add your test suites here"),
		Expected: true,
	})
}

func TestFormatPartialConfigJestFrameworkNoHints(t *testing.T) {
	cfg := formatPartialConfig([]string{"jest"}, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest framework detected",
		Should:   "include the jest suite stub",
		Actual:   strings.Contains(cfg, "name: jest"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "jest framework detected",
		Should:   "omit the commented test_suites example",
		Actual:   !strings.Contains(cfg, "# Add your test suites here"),
		Expected: true,
	})
}

func TestFormatPartialConfigWithHints(t *testing.T) {
	hints := []git.GitHubRemoteHint{
		{Name: "origin", Owner: "mhargiss", Repo: "quarantine"},
		{Name: "upstream", Owner: "mycargus", Repo: "quarantine"},
	}
	cfg := formatPartialConfig(nil, hints)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two github.com remote hints",
		Should:   "include the 'detected GitHub remotes' hint block header",
		Actual:   strings.Contains(cfg, "# detected GitHub remotes (review before using):"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an origin -> mhargiss/quarantine hint",
		Should:   "render the origin hint comment line",
		Actual:   strings.Contains(cfg, "#   origin -> mhargiss/quarantine"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an upstream -> mycargus/quarantine hint",
		Should:   "render the upstream hint comment line",
		Actual:   strings.Contains(cfg, "#   upstream -> mycargus/quarantine"),
		Expected: true,
	})
}

// --- formatPhase1ExitMessage unit tests ---

func TestFormatPhase1ExitMessageHasErrorPrefix(t *testing.T) {
	msg := formatPhase1ExitMessage(true)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 1 exit message",
		Should:   "begin with the 'Error [config]' prefix",
		Actual:   strings.HasPrefix(msg, "Error [config]: github.owner and github.repo are required."),
		Expected: true,
	})
}

func TestFormatPhase1ExitMessageInstructsHandEdit(t *testing.T) {
	msg := formatPhase1ExitMessage(true)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 1 exit message",
		Should:   "tell the user to edit .quarantine/config.yml",
		Actual:   strings.Contains(msg, ".quarantine/config.yml has been created. Edit it to set:"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 1 exit message",
		Should:   "instruct the user to re-run 'quarantine init' to complete setup",
		Actual:   strings.Contains(msg, "Then re-run 'quarantine init' to complete setup."),
		Expected: true,
	})
}

func TestFormatPhase1ExitMessageWithTokenSetOmitsNote(t *testing.T) {
	msg := formatPhase1ExitMessage(true)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 1 exit message when a GitHub token is set",
		Should:   "omit the token note (token is already configured)",
		Actual:   strings.Contains(msg, "Note: You will also need a GitHub token."),
		Expected: false,
	})
}

func TestFormatPhase1ExitMessageWithoutTokenIncludesNote(t *testing.T) {
	msg := formatPhase1ExitMessage(false)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 1 exit message when no GitHub token is set",
		Should:   "include the token note alerting the user it will be required",
		Actual:   strings.Contains(msg, "Note: You will also need a GitHub token."),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 1 exit message when no GitHub token is set",
		Should:   "name both env var fallbacks for setting the token",
		Actual:   strings.Contains(msg, "QUARANTINE_GITHUB_TOKEN") && strings.Contains(msg, "GITHUB_TOKEN"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 1 exit message when no GitHub token is set",
		Should:   "tell the user the required token scope",
		Actual:   strings.Contains(msg, "required scope: repo"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "phase 1 exit message when no GitHub token is set",
		Should:   "still include the canonical hand-edit instructions",
		Actual:   strings.Contains(msg, "Then re-run 'quarantine init' to complete setup."),
		Expected: true,
	})
}
