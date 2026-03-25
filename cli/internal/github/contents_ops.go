// Package github provides GitHub API operations for quarantine state management.
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// BranchNotFoundError is returned by GetContents when the ref (branch) does not exist.
type BranchNotFoundError struct {
	Branch string
}

func (e *BranchNotFoundError) Error() string {
	return fmt.Sprintf("branch %q not found (not initialized)", e.Branch)
}

// contentsResponse represents the JSON response from GET /repos/{owner}/{repo}/contents/{path}.
type contentsResponse struct {
	Content string `json:"content"`
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

// UpdateContents creates or updates a file via the GitHub Contents API on a specified branch.
// It is like PutContents but accepts a branch parameter instead of hardcoding "quarantine/state".
// Returns *APIError{StatusCode: 409} on CAS conflict, *APIError{StatusCode: 422} on validation error.
func (c *Client) UpdateContents(ctx context.Context, path, branch, message string, content []byte, sha string) error {
	encoded := base64.StdEncoding.EncodeToString(content)

	body := map[string]interface{}{
		"message": message,
		"content": encoded,
		"branch":  branch,
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
		return &APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("update contents failed with status %d", resp.StatusCode)}
	}
	return nil
}

// GetContents retrieves a file from the given ref (branch) via the GitHub Contents API.
// Returns base64-decoded content and the file's SHA.
//
// On 404: if the response message indicates the ref does not exist, returns
// BranchNotFoundError. If the file simply doesn't exist on the branch,
// returns empty content, empty sha, nil error (treat as empty state).
//
// On other errors, returns a non-nil error.
func (c *Client) GetContents(ctx context.Context, path, ref string) (content []byte, sha string, err error) {
	apiPath := fmt.Sprintf("%s/contents/%s?ref=%s", c.repoPath(), strings.TrimPrefix(path, "/"), ref)
	req, reqErr := c.newRequestWithContext(ctx, "GET", apiPath, nil)
	if reqErr != nil {
		return nil, "", fmt.Errorf("build request: %w", reqErr)
	}

	resp, doErr := c.doWithRetry(req)
	if doErr != nil {
		return nil, "", doErr
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		var cr contentsResponse
		if decErr := json.NewDecoder(resp.Body).Decode(&cr); decErr != nil {
			return nil, "", fmt.Errorf("decode contents response: %w", decErr)
		}
		// GitHub wraps base64 in newlines — strip them before decoding.
		cleaned := strings.ReplaceAll(cr.Content, "\n", "")
		decoded, b64Err := base64.StdEncoding.DecodeString(cleaned)
		if b64Err != nil {
			return nil, "", fmt.Errorf("decode base64 contents: %w", b64Err)
		}
		return decoded, cr.SHA, nil

	case http.StatusNotFound:
		// Distinguish branch-not-found from file-not-found by reading the
		// GitHub error message.
		var cr contentsResponse
		_ = json.NewDecoder(resp.Body).Decode(&cr)
		if strings.Contains(cr.Message, "No commit found for the ref") {
			return nil, "", &BranchNotFoundError{Branch: ref}
		}
		// File not found on existing branch → empty state, no error.
		return nil, "", nil

	case http.StatusUnauthorized:
		return nil, "", &APIError{StatusCode: 401, Message: "GitHub token is invalid or expired"}

	default:
		return nil, "", &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status %d from GET %s", resp.StatusCode, apiPath),
		}
	}
}
