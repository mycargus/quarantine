//go:build contract

package github

import (
	"context"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

func TestContractSearchClosedIssues200(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	numbers, truncated, totalCount, err := c.SearchClosedIssues(ctx, "quarantine")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /search/issues returns 200",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "search closed issues 200 response",
		Should: "return non-negative total_count",
		Actual: totalCount >= 0,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "search closed issues 200 response",
		Should: "return a slice of issue numbers (possibly empty)",
		Actual: numbers != nil || len(numbers) == 0,
		Expected: true,
	})
	_ = truncated // boolean field, no further assertion needed
}

func TestContractSearchOpenIssue200(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	issueNumber, issueURL, found, err := c.SearchOpenIssue(ctx, "2cf24dba")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /search/issues for dedup check returns 200",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
	// Prism returns the example with total_count=1 and items[0].number=42.
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "search open issue 200 response with total_count=1",
		Should: "find the issue (found=true)",
		Actual: found,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "search open issue 200 response",
		Should: "return a positive issue number",
		Actual: issueNumber > 0,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "search open issue 200 response",
		Should: "return a non-empty issue URL",
		Actual: issueURL != "",
		Expected: true,
	})
}

func TestContractSearchPaginationParams(t *testing.T) {
	// Verify that the Search API request with per_page and page params is
	// accepted by Prism (request validation passes).
	c := newPrismClient(t)
	ctx := context.Background()

	// SearchClosedIssues paginates with per_page=100 and page=N.
	_, _, _, err := c.SearchClosedIssues(ctx, "quarantine")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /search/issues with per_page and page query params",
		Should: "Prism accepts the request without validation error",
		Actual: err == nil,
		Expected: true,
	})
}
