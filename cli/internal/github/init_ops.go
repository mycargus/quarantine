// Package github provides a client for interacting with the GitHub API
// for quarantine state management, issue creation, and PR comments.
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RepoInfo contains repository metadata returned by GET /repos/{owner}/{repo}.
type RepoInfo struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// RefInfo contains ref information returned by GET /repos/{owner}/{repo}/git/ref/{ref}.
type RefInfo struct {
	Ref    string    `json:"ref"`
	Object RefObject `json:"object"`
}

// RefObject is the nested object inside a RefInfo response.
type RefObject struct {
	SHA  string `json:"sha"`
	Type string `json:"type"`
}

// GetRepo calls GET /repos/{owner}/{repo} and returns repository metadata.
// Returns an error if the token lacks permission (403), repo is not found (404),
// or any other API error occurs.
func (c *Client) GetRepo(ctx context.Context) (*RepoInfo, error) {
	req, err := c.newRequestWithContext(ctx, "GET", c.repoPath(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		var info RepoInfo
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			return nil, fmt.Errorf("decode repo response: %w", err)
		}
		return &info, nil
	case http.StatusUnauthorized:
		return nil, &APIError{StatusCode: 401, Message: "GitHub token is invalid or expired"}
	case http.StatusForbidden:
		return nil, &APIError{StatusCode: 403, Message: fmt.Sprintf("GitHub token lacks permission to access repository '%s/%s'", c.owner, c.repo)}
	case http.StatusNotFound:
		return nil, &APIError{StatusCode: 404, Message: fmt.Sprintf("repository '%s/%s' not found", c.owner, c.repo)}
	default:
		return nil, &APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("unexpected status %d from GET %s", resp.StatusCode, c.repoPath())}
	}
}

// GetRef calls GET /repos/{owner}/{repo}/git/ref/heads/{ref}.
// Returns the SHA and exists=true on 200, exists=false on 404 (branch doesn't exist).
func (c *Client) GetRef(ctx context.Context, ref string) (sha string, exists bool, err error) {
	path := fmt.Sprintf("%s/git/ref/heads/%s", c.repoPath(), ref)
	req, reqErr := c.newRequestWithContext(ctx, "GET", path, nil)
	if reqErr != nil {
		return "", false, fmt.Errorf("build request: %w", reqErr)
	}

	resp, doErr := c.doWithRetry(req)
	if doErr != nil {
		return "", false, doErr
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		var info RefInfo
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			return "", false, fmt.Errorf("decode ref response: %w", err)
		}
		return info.Object.SHA, true, nil
	case http.StatusNotFound:
		return "", false, nil
	default:
		return "", false, &APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("unexpected status %d from GET %s", resp.StatusCode, path)}
	}
}

// CreateRef calls POST /repos/{owner}/{repo}/git/refs to create a new branch ref.
func (c *Client) CreateRef(ctx context.Context, ref, sha string) error {
	body := map[string]string{
		"ref": "refs/heads/" + ref,
		"sha": sha,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	path := fmt.Sprintf("%s/git/refs", c.repoPath())
	req, err := c.newRequestWithContext(ctx, "POST", path, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return &APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("create ref failed with status %d", resp.StatusCode)}
	}
	return nil
}

// PutContents calls PUT /repos/{owner}/{repo}/contents/{path} to create or update a file.
// sha should be empty for new file creation.
func (c *Client) PutContents(ctx context.Context, path, message string, content []byte, sha string) error {
	encoded := base64.StdEncoding.EncodeToString(content)

	body := map[string]interface{}{
		"message": message,
		"content": encoded,
		"branch":  "quarantine/state",
	}
	if sha != "" {
		body["sha"] = sha
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	apiPath := fmt.Sprintf("%s/contents/%s", c.repoPath(), strings.TrimPrefix(path, "/"))
	req, err := c.newRequestWithContext(ctx, "PUT", apiPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return &APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("put contents failed with status %d", resp.StatusCode)}
	}
	return nil
}

// APIError represents a GitHub API error with an HTTP status code.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("GitHub API error %d: %s", e.StatusCode, e.Message)
}

// newRequestWithContext creates an authenticated HTTP request with a context.
func (c *Client) newRequestWithContext(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	return req, nil
}

// doWithRetry executes an HTTP request with retry-once-on-network-error behavior.
// After a successful response, it checks rate limit headers and fires the warning
// callback if remaining < 10% of limit.
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Retry once after a delay for network errors.
		time.Sleep(c.retryDelay)
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("GitHub API request failed: %w", err)
		}
	}
	c.checkRateLimit(resp)
	return resp, nil
}

// checkRateLimit inspects X-RateLimit-* headers and fires the warning callback
// if the remaining quota is below 10% of the total limit.
func (c *Client) checkRateLimit(resp *http.Response) {
	if c.rateLimitWarningFn == nil {
		return
	}
	limitStr := resp.Header.Get("X-RateLimit-Limit")
	remainingStr := resp.Header.Get("X-RateLimit-Remaining")
	resetStr := resp.Header.Get("X-RateLimit-Reset")
	if limitStr == "" || remainingStr == "" {
		return
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit == 0 {
		return
	}
	remaining, err := strconv.Atoi(remainingStr)
	if err != nil {
		return
	}
	if remaining*10 < limit {
		var resetTime string
		if ts, parseErr := strconv.ParseInt(resetStr, 10, 64); parseErr == nil {
			resetTime = time.Unix(ts, 0).UTC().Format("15:04:05")
		} else {
			resetTime = resetStr
		}
		msg := fmt.Sprintf(
			"[quarantine] WARNING: GitHub API rate limit low (%d remaining, resets at %s UTC). "+
				"Consider using a PAT for higher limits (5,000 req/hr vs 1,000 req/hr for GITHUB_TOKEN).",
			remaining, resetTime,
		)
		c.rateLimitWarningFn(msg)
	}
}
