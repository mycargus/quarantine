//go:build contract

package github

import (
	"context"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

func TestContractGetContents200(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	content, sha, err := c.GetContents(ctx, "quarantine.json", "quarantine/state")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /repos/{owner}/{repo}/contents/{path} returns 200",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET contents 200 response",
		Should: "return non-empty content (base64-decoded)",
		Actual: len(content) > 0,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET contents 200 response",
		Should: "return non-empty SHA",
		Actual: sha != "",
		Expected: true,
	})
}

func TestContractGetContents404BranchNotFound(t *testing.T) {
	c := newPrismClientWithPrefer(t,
		preferHeader(404),
		preferExampleHeader("branch-not-found"),
	)
	ctx := context.Background()

	_, _, err := c.GetContents(ctx, "quarantine.json", "quarantine/state")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /repos/{owner}/{repo}/contents/{path} returns 404 with 'No commit found for the ref' message",
		Should: "return a BranchNotFoundError",
		Actual: func() bool {
			if err == nil {
				return false
			}
			_, ok := err.(*BranchNotFoundError)
			return ok
		}(),
		Expected: true,
	})
}

func TestContractGetContents404FileNotFound(t *testing.T) {
	c := newPrismClientWithPrefer(t,
		preferHeader(404),
		preferExampleHeader("file-not-found"),
	)
	ctx := context.Background()

	content, sha, err := c.GetContents(ctx, "quarantine.json", "quarantine/state")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /repos/{owner}/{repo}/contents/{path} returns 404 with generic 'Not Found' message (file absent on existing branch)",
		Should: "return no error (empty state)",
		Actual: err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:  "file-not-found 404",
		Should: "return empty content",
		Actual: len(content),
		Expected: 0,
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:  "file-not-found 404",
		Should: "return empty SHA",
		Actual: sha,
		Expected: "",
	})
}

func TestContractUpdateContents200(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	err := c.UpdateContents(ctx, "quarantine.json", "quarantine/state", "chore: update quarantine state", []byte(`{"version":1,"tests":{}}`), "abc123")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "PUT /repos/{owner}/{repo}/contents/{path} returns 200 (update existing file)",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
}

func TestContractUpdateContents409Conflict(t *testing.T) {
	c := newPrismClientWithPrefer(t, preferHeader(409))
	ctx := context.Background()

	err := c.UpdateContents(ctx, "quarantine.json", "quarantine/state", "chore: update quarantine state", []byte(`{}`), "stale-sha")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "PUT /repos/{owner}/{repo}/contents/{path} returns 409 (CAS conflict)",
		Should: "return APIError with StatusCode 409",
		Actual: func() bool {
			apiErr, ok := err.(*APIError)
			return ok && apiErr.StatusCode == 409
		}(),
		Expected: true,
	})
}

func TestContractUpdateContents422ValidationError(t *testing.T) {
	c := newPrismClientWithPrefer(t, preferHeader(422))
	ctx := context.Background()

	err := c.UpdateContents(ctx, "quarantine.json", "quarantine/state", "chore: update quarantine state", []byte(`{}`), "")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "PUT /repos/{owner}/{repo}/contents/{path} returns 422 (validation error)",
		Should: "return APIError with StatusCode 422",
		Actual: func() bool {
			apiErr, ok := err.(*APIError)
			return ok && apiErr.StatusCode == 422
		}(),
		Expected: true,
	})
}
