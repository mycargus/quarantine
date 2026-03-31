//go:build contract

package github

import (
	"context"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

func TestContractListPRComments200(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	comments, err := c.ListPRComments(ctx, 1347)

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /repos/{owner}/{repo}/issues/{number}/comments returns 200",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "list PR comments 200 response",
		Should: "return a non-nil slice",
		Actual: comments != nil,
		Expected: true,
	})
	// Prism returns the example with one comment.
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "list PR comments 200 response",
		Should: "each comment has a non-zero ID",
		Actual: func() bool {
			for _, c := range comments {
				if c.ID == 0 {
					return false
				}
			}
			return true
		}(),
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "list PR comments 200 response",
		Should: "each comment has a non-empty body",
		Actual: func() bool {
			for _, c := range comments {
				if c.Body == "" {
					return false
				}
			}
			return true
		}(),
		Expected: true,
	})
}

func TestContractCreatePRComment201(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	err := c.CreatePRComment(ctx, 1347, "<!-- quarantine-bot -->\n## Quarantine Summary")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "POST /repos/{owner}/{repo}/issues/{number}/comments returns 201",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
}

func TestContractUpdatePRComment200(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	err := c.UpdatePRComment(ctx, 10, "<!-- quarantine-bot -->\n## Quarantine Summary (updated)")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "PATCH /repos/{owner}/{repo}/issues/comments/{id} returns 200",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
}
