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
