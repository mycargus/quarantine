//go:build contract

package github

import (
	"context"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

func TestContractCreateIssue201(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	issueNumber, issueURL, err := c.CreateIssue(ctx,
		"[Quarantine] should handle eviction",
		"## Flaky Test Detected\n\n**Test ID:** `src/cache.test.js::CacheService::should handle eviction`",
		[]string{"quarantine", "quarantine:2cf24dba"},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "POST /repos/{owner}/{repo}/issues returns 201",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "create issue 201 response",
		Should: "return a positive issue number",
		Actual: issueNumber > 0,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "create issue 201 response",
		Should: "return a non-empty html_url",
		Actual: issueURL != "",
		Expected: true,
	})
}

func TestContractCreateIssue410IssuesDisabled(t *testing.T) {
	c := newPrismClientWithPrefer(t, preferHeader(410))
	ctx := context.Background()

	_, _, err := c.CreateIssue(ctx,
		"[Quarantine] should handle eviction",
		"body",
		[]string{"quarantine", "quarantine:2cf24dba"},
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "POST /repos/{owner}/{repo}/issues returns 410 (GitHub Issues disabled on repo)",
		Should: "return APIError with StatusCode 410",
		Actual: func() bool {
			apiErr, ok := err.(*APIError)
			return ok && apiErr.StatusCode == 410
		}(),
		Expected: true,
	})
}
