// Package github provides GitHub API operations for quarantine state management.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// searchIssuesResponse is the JSON response from GET /search/issues.
type searchIssuesResponse struct {
	TotalCount int           `json:"total_count"`
	Items      []searchIssue `json:"items"`
}

// searchIssue is a single item in a search issues response.
type searchIssue struct {
	Number int `json:"number"`
}

// maxSearchPages is the maximum number of pages fetched (1000 results total).
const maxSearchPages = 10

// SearchClosedIssues returns the issue numbers of all closed issues in the
// repository that have the given label. It paginates up to 1000 results
// (10 pages × 100 per page). If total_count > 1000, truncated is set to true.
func (c *Client) SearchClosedIssues(ctx context.Context, label string) (closedIssueNumbers []int, truncated bool, totalCount int, err error) {
	var allNumbers []int
	seenTotal := 0

	for page := 1; page <= maxSearchPages; page++ {
		q := fmt.Sprintf("repo:%s/%s is:issue is:closed label:%s", c.owner, c.repo, label)
		params := url.Values{
			"q":        {q},
			"per_page": {"100"},
			"page":     {fmt.Sprintf("%d", page)},
		}
		apiPath := "/search/issues?" + params.Encode()

		req, reqErr := c.newRequestWithContext(ctx, "GET", apiPath, nil)
		if reqErr != nil {
			return nil, false, 0, fmt.Errorf("build request: %w", reqErr)
		}

		resp, doErr := c.doWithRetry(req)
		if doErr != nil {
			return nil, false, 0, doErr
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, false, 0, &APIError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("unexpected status %d from GET /search/issues", resp.StatusCode),
			}
		}

		var sr searchIssuesResponse
		if decErr := json.NewDecoder(resp.Body).Decode(&sr); decErr != nil {
			_ = resp.Body.Close()
			return nil, false, 0, fmt.Errorf("decode search response: %w", decErr)
		}
		_ = resp.Body.Close()

		if page == 1 {
			seenTotal = sr.TotalCount
		}

		for _, item := range sr.Items {
			allNumbers = append(allNumbers, item.Number)
		}

		// Stop paginating when we've retrieved all available items or hit the page cap.
		if len(sr.Items) < 100 {
			break
		}
	}

	isTruncated := seenTotal > 1000
	return allNumbers, isTruncated, seenTotal, nil
}
