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
	httpClient          *http.Client
	baseURL             string
	token               string
	owner               string
	repo                string
	retryDelay          time.Duration
	rateLimitWarningFn  func(msg string)
}

// SetRateLimitWarningFunc sets a callback that is called when the GitHub API
// rate limit is below 10% of the total limit. The callback receives a formatted
// warning message.
func (c *Client) SetRateLimitWarningFunc(fn func(msg string)) {
	c.rateLimitWarningFn = fn
}

// NewClient creates a new GitHub API client. The token is resolved from
// QUARANTINE_GITHUB_TOKEN, falling back to GITHUB_TOKEN.
//
// The base URL defaults to https://api.github.com but can be overridden
// via QUARANTINE_GITHUB_API_BASE_URL (used in tests).
func NewClient(owner, repo string) (*Client, error) {
	token := resolveToken()
	if token == "" {
		return nil, fmt.Errorf("no GitHub token found. Set QUARANTINE_GITHUB_TOKEN or GITHUB_TOKEN")
	}

	baseURL := defaultBaseURL
	if override := os.Getenv("QUARANTINE_GITHUB_API_BASE_URL"); override != "" {
		baseURL = override
	}

	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    baseURL,
		token:      token,
		owner:      owner,
		repo:       repo,
		retryDelay: 2 * time.Second,
	}, nil
}

// SetRetryDelay overrides the retry delay (used in tests to eliminate sleeps).
func (c *Client) SetRetryDelay(d time.Duration) {
	c.retryDelay = d
}

// resolveToken checks QUARANTINE_GITHUB_TOKEN first, then GITHUB_TOKEN.
func resolveToken() string {
	if token := os.Getenv("QUARANTINE_GITHUB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GITHUB_TOKEN")
}

// ResolveToken checks QUARANTINE_GITHUB_TOKEN first, then GITHUB_TOKEN.
// Exported for use by other packages in the CLI.
func ResolveToken() string {
	return resolveToken()
}

// repoPath returns the API path prefix for the configured repository.
func (c *Client) repoPath() string {
	return fmt.Sprintf("/repos/%s/%s", c.owner, c.repo)
}
