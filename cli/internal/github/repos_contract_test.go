//go:build contract

package github

import (
	"context"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

func TestContractGetRepo200(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	info, err := c.GetRepo(ctx)

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /repos/{owner}/{repo} returns 200",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "get repo 200 response",
		Should: "return non-zero ID",
		Actual: info.ID > 0,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "get repo 200 response",
		Should: "return non-empty full_name",
		Actual: info.FullName != "",
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "get repo 200 response",
		Should: "return non-empty default_branch",
		Actual: info.DefaultBranch != "",
		Expected: true,
	})
	// private is a boolean — any value is valid; just verify the field exists by
	// checking the struct was populated (non-zero ID above is sufficient).
}

func TestContractGetRepo403Forbidden(t *testing.T) {
	c := newPrismClientWithPrefer(t, preferHeader(403))
	ctx := context.Background()

	_, err := c.GetRepo(ctx)

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /repos/{owner}/{repo} returns 403 (token lacks permission)",
		Should: "return APIError with StatusCode 403",
		Actual: func() bool {
			apiErr, ok := err.(*APIError)
			return ok && apiErr.StatusCode == 403
		}(),
		Expected: true,
	})
}

func TestContractGetRepo404NotFound(t *testing.T) {
	c := newPrismClientWithPrefer(t, preferHeader(404))
	ctx := context.Background()

	_, err := c.GetRepo(ctx)

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /repos/{owner}/{repo} returns 404 (repo does not exist)",
		Should: "return APIError with StatusCode 404",
		Actual: func() bool {
			apiErr, ok := err.(*APIError)
			return ok && apiErr.StatusCode == 404
		}(),
		Expected: true,
	})
}
