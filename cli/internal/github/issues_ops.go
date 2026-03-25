// Package github provides GitHub API operations for issue creation and PR comments.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// issueResponse is the JSON response from POST /repos/{owner}/{repo}/issues.
type issueResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

// dedupSearchResponse is used by SearchOpenIssue to include html_url.
type dedupSearchResponse struct {
	TotalCount int                `json:"total_count"`
	Items      []dedupSearchItem  `json:"items"`
}

type dedupSearchItem struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

// PRComment represents a single comment on a pull request / issue.
type PRComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}

// SearchOpenIssue checks whether an open issue already exists for the given
// test hash. Returns (issueNumber, issueURL, found, error).
// On search error, returns found=false so the caller can proceed to create.
func (c *Client) SearchOpenIssue(ctx context.Context, testHash string) (issueNumber int, issueURL string, found bool, err error) {
	q := fmt.Sprintf("repo:%s/%s is:issue is:open label:quarantine label:quarantine:%s", c.owner, c.repo, testHash)
	params := url.Values{
		"q":        {q},
		"per_page": {"1"},
	}
	apiPath := "/search/issues?" + params.Encode()

	req, reqErr := c.newRequestWithContext(ctx, "GET", apiPath, nil)
	if reqErr != nil {
		return 0, "", false, fmt.Errorf("build request: %w", reqErr)
	}

	resp, doErr := c.doWithRetry(req)
	if doErr != nil {
		return 0, "", false, doErr
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, "", false, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status %d from GET /search/issues (dedup)", resp.StatusCode),
		}
	}

	var sr dedupSearchResponse
	if decErr := json.NewDecoder(resp.Body).Decode(&sr); decErr != nil {
		return 0, "", false, fmt.Errorf("decode dedup search response: %w", decErr)
	}

	if sr.TotalCount > 0 && len(sr.Items) > 0 {
		return sr.Items[0].Number, sr.Items[0].HTMLURL, true, nil
	}
	return 0, "", false, nil
}

// CreateIssue creates a GitHub issue with the given title, body, and labels.
// Returns the issue number and URL on success.
func (c *Client) CreateIssue(ctx context.Context, title, body string, labels []string) (issueNumber int, issueURL string, err error) {
	reqBody := map[string]interface{}{
		"title":  title,
		"body":   body,
		"labels": labels,
	}
	bodyBytes, marshalErr := json.Marshal(reqBody)
	if marshalErr != nil {
		return 0, "", fmt.Errorf("marshal issue body: %w", marshalErr)
	}

	apiPath := fmt.Sprintf("%s/issues", c.repoPath())
	req, reqErr := c.newRequestWithContext(ctx, "POST", apiPath, bytes.NewReader(bodyBytes))
	if reqErr != nil {
		return 0, "", fmt.Errorf("build request: %w", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, doErr := c.doWithRetry(req)
	if doErr != nil {
		return 0, "", doErr
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return 0, "", &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("create issue failed with status %d", resp.StatusCode),
		}
	}

	var ir issueResponse
	if decErr := json.NewDecoder(resp.Body).Decode(&ir); decErr != nil {
		return 0, "", fmt.Errorf("decode issue response: %w", decErr)
	}

	return ir.Number, ir.HTMLURL, nil
}

// ListPRComments returns up to 100 comments on the given PR / issue number.
// Capped at 100 per spec (no pagination needed for find-existing).
func (c *Client) ListPRComments(ctx context.Context, prNumber int) ([]PRComment, error) {
	apiPath := fmt.Sprintf("%s/issues/%d/comments?per_page=100", c.repoPath(), prNumber)
	req, reqErr := c.newRequestWithContext(ctx, "GET", apiPath, nil)
	if reqErr != nil {
		return nil, fmt.Errorf("build request: %w", reqErr)
	}

	resp, doErr := c.doWithRetry(req)
	if doErr != nil {
		return nil, doErr
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("list PR comments failed with status %d", resp.StatusCode),
		}
	}

	var comments []PRComment
	if decErr := json.NewDecoder(resp.Body).Decode(&comments); decErr != nil {
		return nil, fmt.Errorf("decode PR comments response: %w", decErr)
	}
	return comments, nil
}

// CreatePRComment posts a new comment on the given PR / issue number.
func (c *Client) CreatePRComment(ctx context.Context, prNumber int, body string) error {
	reqBody := map[string]string{"body": body}
	bodyBytes, marshalErr := json.Marshal(reqBody)
	if marshalErr != nil {
		return fmt.Errorf("marshal PR comment body: %w", marshalErr)
	}

	apiPath := fmt.Sprintf("%s/issues/%d/comments", c.repoPath(), prNumber)
	req, reqErr := c.newRequestWithContext(ctx, "POST", apiPath, bytes.NewReader(bodyBytes))
	if reqErr != nil {
		return fmt.Errorf("build request: %w", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, doErr := c.doWithRetry(req)
	if doErr != nil {
		return doErr
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("create PR comment failed with status %d", resp.StatusCode),
		}
	}
	return nil
}

// UpdatePRComment updates an existing comment by ID via PATCH.
func (c *Client) UpdatePRComment(ctx context.Context, commentID int64, body string) error {
	reqBody := map[string]string{"body": body}
	bodyBytes, marshalErr := json.Marshal(reqBody)
	if marshalErr != nil {
		return fmt.Errorf("marshal PR comment body: %w", marshalErr)
	}

	apiPath := fmt.Sprintf("%s/issues/comments/%d", c.repoPath(), commentID)
	req, reqErr := c.newRequestWithContext(ctx, "PATCH", apiPath, bytes.NewReader(bodyBytes))
	if reqErr != nil {
		return fmt.Errorf("build request: %w", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, doErr := c.doWithRetry(req)
	if doErr != nil {
		return doErr
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("update PR comment failed with status %d", resp.StatusCode),
		}
	}
	return nil
}

