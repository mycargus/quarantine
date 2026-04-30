// Package git provides utilities for working with git repositories.
package git

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// ParseGitHubURL parses a GitHub remote URL (HTTPS or SSH) and returns the
// owner and repository name. Returns an error if the URL is not a GitHub URL.
//
// Accepts:
//   - https://github.com/owner/repo.git
//   - https://github.com/owner/repo
//   - git@github.com:owner/repo.git
func ParseGitHubURL(rawURL string) (owner, repo string, err error) {
	if rawURL == "" {
		return "", "", fmt.Errorf("remote URL is empty")
	}

	// SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(rawURL, "git@github.com:") {
		path := strings.TrimPrefix(rawURL, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("remote 'origin' is not a GitHub URL: %s", rawURL)
		}
		return parts[0], parts[1], nil
	}

	// HTTPS format: https://github.com/owner/repo.git
	u, err := url.Parse(rawURL)
	if err != nil || u.Host != "github.com" {
		return "", "", fmt.Errorf("remote 'origin' is not a GitHub URL: %s", rawURL)
	}

	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("remote 'origin' is not a GitHub URL: %s", rawURL)
	}

	return parts[0], parts[1], nil
}

// GitHubRemoteHint describes a single github.com remote found by ScanGitHubRemotes.
// It is advisory only — used to emit YAML hint comments under the github block in
// `.quarantine/config.yml`. The fields are never used to populate the explicit
// `github.owner`/`github.repo` resolution path (per ADR-037).
type GitHubRemoteHint struct {
	Name  string
	Owner string
	Repo  string
}

// ScanGitHubRemotes runs `git remote -v` in dir and returns one entry per
// github.com remote (deduplicated by remote name). Non-github.com remotes are
// silently dropped. Failures (not a git repo, command unavailable, etc.) yield
// an empty slice — the caller treats this as "no hints available", never an error.
func ScanGitHubRemotes(dir string) []GitHubRemoteHint {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseGitRemoteVerbose(string(out))
}

// parseGitRemoteVerbose extracts github.com hints from the textual output of
// `git remote -v`. Each remote typically appears twice (fetch and push) with the
// same URL; we keep only the first occurrence per remote name.
// This is a pure function — no I/O.
func parseGitRemoteVerbose(output string) []GitHubRemoteHint {
	var hints []GitHubRemoteHint
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		url := fields[1]
		if seen[name] {
			continue
		}
		owner, repo, err := ParseGitHubURL(url)
		if err != nil {
			continue
		}
		seen[name] = true
		hints = append(hints, GitHubRemoteHint{Name: name, Owner: owner, Repo: repo})
	}
	return hints
}

// ParseRemote runs `git remote get-url origin` in the given directory and
// parses the result as a GitHub URL. Returns the owner and repository name.
func ParseRemote(dir string) (owner, repo string, err error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// Check if it's a "not a git repository" error.
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "not a git repository") || strings.Contains(stderr, "Not a git repository") {
				return "", "", fmt.Errorf("not a git repository: run 'quarantine init' from the root of a git repository")
			}
		}
		return "", "", fmt.Errorf("could not get git remote: %w", err)
	}

	rawURL := strings.TrimSpace(string(out))
	owner, repo, err = ParseGitHubURL(rawURL)
	if err != nil {
		return "", "", err
	}
	return owner, repo, nil
}
