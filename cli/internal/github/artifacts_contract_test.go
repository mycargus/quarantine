//go:build contract

package github

import (
	"context"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

func TestContractListArtifacts200(t *testing.T) {
	c := newPrismClient(t)
	ctx := context.Background()

	artifacts, err := c.ListArtifacts(ctx, "quarantine-results-backend", 10)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos/{owner}/{repo}/actions/artifacts returns 200",
		Should:   "return no error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos/{owner}/{repo}/actions/artifacts returns 200",
		Should:   "return non-nil artifacts slice",
		Actual:   artifacts != nil,
		Expected: true,
	})
}

func TestContractListArtifacts404(t *testing.T) {
	c := newPrismClientWithPrefer(t, preferHeader(404))
	ctx := context.Background()

	_, err := c.ListArtifacts(ctx, "quarantine-results-backend", 10)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GET /repos/{owner}/{repo}/actions/artifacts returns 404",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}
