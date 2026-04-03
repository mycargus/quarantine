// Package github provides a client for interacting with the GitHub API
// for quarantine state management, issue creation, and PR comments.
package github

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	defaultBaseURL = "https://api.github.com"
	userAgent      = "quarantine-cli/0.1.0"
	defaultTimeout = 10 * time.Second
)

// Client is a GitHub API client for quarantine operations.
type Client struct {
	httpClient         *http.Client
	baseURL            string
	token              string
	owner              string
	repo               string
	retryDelay         time.Duration
	rateLimitWarningFn func(msg string)
	verboseLogFn       func(msg string)
}

// SetRateLimitWarningFunc sets a callback that is called when the GitHub API
// rate limit is below 10% of the total limit. The callback receives a formatted
// warning message.
func (c *Client) SetRateLimitWarningFunc(fn func(msg string)) {
	c.rateLimitWarningFn = fn
}

// SetVerboseLogFunc sets a callback that is called for each API response to
// log rate limit header details in verbose mode.
func (c *Client) SetVerboseLogFunc(fn func(msg string)) {
	c.verboseLogFn = fn
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
		retryDelay: resolveRetryDelay(),
	}, nil
}

// resolveRetryDelay returns the retry delay. Defaults to 2 seconds; overridable
// via QUARANTINE_RETRY_DELAY_SECONDS (e.g. "0" or "0.1") for tests that simulate
// unreachable servers without waiting.
func resolveRetryDelay() time.Duration {
	seconds := 2.0
	if s := os.Getenv("QUARANTINE_RETRY_DELAY_SECONDS"); s != "" {
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			seconds = n
		}
	}
	return time.Duration(seconds * float64(time.Second))
}

// SetRetryDelay overrides the retry delay (used in tests to eliminate sleeps).
func (c *Client) SetRetryDelay(d time.Duration) {
	c.retryDelay = d
}

// RetryDelay returns the current retry delay (used in tests to assert the default).
func (c *Client) RetryDelay() time.Duration {
	return c.retryDelay
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
