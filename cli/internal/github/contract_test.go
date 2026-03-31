//go:build contract

package github

// Contract test helpers.
//
// These helpers create GitHub clients that point at the Prism mock server
// instead of the real GitHub API. They are used by all *_contract_test.go
// files in this package.
//
// The package is `github` (not `github_test`) so that helpers can access
// unexported fields — specifically c.httpClient, which must be replaced with a
// transport that injects `Prefer: code=NNN` headers for error-path testing.

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// prismURL returns the base URL of the running Prism mock server.
// It reads PRISM_URL from the environment and calls t.Fatal if it is unset.
func prismURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("PRISM_URL")
	if u == "" {
		t.Fatal("PRISM_URL is not set — contract tests must be run via 'make contract-test' or scripts/run-contract-tests.sh")
	}
	return u
}

// preferHeader builds a Prefer header value for the given HTTP status code.
// Used to instruct Prism to return a specific response code.
func preferHeader(code int) string {
	return fmt.Sprintf("code=%d", code)
}

// preferExampleHeader builds a Prefer header value for a named example.
// Used to select a specific Prism response example when multiple are defined.
func preferExampleHeader(name string) string {
	return fmt.Sprintf("example=%s", name)
}

// preferRoundTripper is an http.RoundTripper that injects Prefer headers.
// It wraps an existing transport and adds one or more Prefer header values.
type preferRoundTripper struct {
	transport http.RoundTripper
	values    []string // individual Prefer values (each becomes a separate Prefer header)
}

func (rt *preferRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for _, v := range rt.values {
		req.Header.Add("Prefer", v)
	}
	return rt.transport.RoundTrip(req)
}

// newPrismClient creates a GitHub API client configured to speak to Prism.
// Uses a fake token (Prism does not validate auth).
func newPrismClient(t *testing.T) *Client {
	t.Helper()
	url := prismURL(t)

	t.Setenv("GITHUB_TOKEN", "contract-test-token")
	t.Setenv("QUARANTINE_GITHUB_API_BASE_URL", url)

	c, err := NewClient("octocat", "Hello-World")
	if err != nil {
		t.Fatalf("newPrismClient: %v", err)
	}
	c.SetRetryDelay(0)
	return c
}

// newPrismClientWithPrefer creates a GitHub API client that injects one or
// more Prefer header values into every outgoing request. This allows
// error-path testing by instructing Prism to return a specific status code
// and/or example.
//
// Example: newPrismClientWithPrefer(t, preferHeader(404), preferExampleHeader("branch-not-found"))
func newPrismClientWithPrefer(t *testing.T, preferValues ...string) *Client {
	t.Helper()
	c := newPrismClient(t)
	c.httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &preferRoundTripper{
			transport: http.DefaultTransport,
			values:    preferValues,
		},
	}
	return c
}
