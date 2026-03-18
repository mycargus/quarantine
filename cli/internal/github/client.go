// Package github provides a client for interacting with the GitHub API
// for quarantine state management, issue creation, and PR comments.
package github

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	defaultBaseURL = "https://api.github.com"
	userAgent      = "quarantine-cli/0.1.0"
	defaultTimeout = 10 * time.Second
)

// Client is a GitHub API client for quarantine operations.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	owner      string
	repo       string
}

// NewClient creates a new GitHub API client. The token is resolved from
// QUARANTINE_GITHUB_TOKEN, falling back to GITHUB_TOKEN.
func NewClient(owner, repo string) (*Client, error) {
	token := resolveToken()
	if token == "" {
		return nil, fmt.Errorf("no GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN")
	}

	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    defaultBaseURL,
		token:      token,
		owner:      owner,
		repo:       repo,
	}, nil
}

// resolveToken checks QUARANTINE_GITHUB_TOKEN first, then GITHUB_TOKEN.
func resolveToken() string {
	if token := os.Getenv("QUARANTINE_GITHUB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GITHUB_TOKEN")
}

// newRequest creates an authenticated HTTP request with the required headers.
func (c *Client) newRequest(method, path string) (*http.Request, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	return req, nil
}

// repoPath returns the API path prefix for the configured repository.
func (c *Client) repoPath() string {
	return fmt.Sprintf("/repos/%s/%s", c.owner, c.repo)
}
