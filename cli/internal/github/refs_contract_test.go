//go:build contract

package github

import (
	"context"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

func TestContractGetRef200(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	// Use a slash-free ref name. OpenAPI path parameters don't support slashes —
	// Prism cannot match "quarantine/state" as a single {ref} path segment.
	// The contract is about response shape, not the specific branch name.
	sha, exists, err := c.GetRef(ctx, "main")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /repos/{owner}/{repo}/git/ref/heads/{ref} returns 200",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "get ref 200 response",
		Should: "return exists=true",
		Actual: exists,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "get ref 200 response",
		Should: "return non-empty SHA (object.sha)",
		Actual: sha != "",
		Expected: true,
	})
}

func TestContractGetRef404BranchDoesNotExist(t *testing.T) {
	c := newPrismClientWithPrefer(t, preferHeader(404))
	ctx := context.Background()

	sha, exists, err := c.GetRef(ctx, "main")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "GET /repos/{owner}/{repo}/git/ref/heads/{ref} returns 404 (branch does not exist)",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:  "get ref 404 response",
		Should: "return exists=false",
		Actual: !exists,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:  "get ref 404 response",
		Should: "return empty SHA",
		Actual: sha,
		Expected: "",
	})
}

func TestContractCreateRef201(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	// CreateRef sends POST /repos/{owner}/{repo}/git/refs — no path parameter issue.
	err := c.CreateRef(ctx, "feature-branch", "aa218f56b14c9653891f9e74264a383fa43fefbd")

	riteway.Assert(t, riteway.Case[bool]{
		Given:  "POST /repos/{owner}/{repo}/git/refs returns 201",
		Should: "return no error",
		Actual: err == nil,
		Expected: true,
	})
}
