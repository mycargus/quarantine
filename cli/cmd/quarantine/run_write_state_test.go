package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mycargus/quarantine/internal/config"
	gh "github.com/mycargus/quarantine/internal/github"
	qstate "github.com/mycargus/quarantine/internal/quarantine"
	riteway "github.com/mycargus/riteway-golang"
)

// newCaptureCmd creates a minimal cobra.Command that captures stderr output.
func newCaptureCmd(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetErr(buf)
	return cmd, buf
}

// newWriteStateClient creates a GitHub client pointed at a test server.
func newWriteStateClient(t *testing.T, server *httptest.Server) *gh.Client {
	t.Helper()
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "ghp_test")
	t.Setenv("QUARANTINE_GITHUB_API_BASE_URL", server.URL)
	c, err := gh.NewClient("owner", "repo")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetRetryDelay(0)
	return c
}

// dirtyState returns a state with one test and its marshaled bytes, plus a
// different originalContent so bytes.Equal returns false and the write proceeds.
func dirtyState(t *testing.T) (*qstate.State, []byte) {
	t.Helper()
	state := qstate.NewEmptyState()
	state.AddTest(qstate.Entry{TestID: "some::test"})
	return state, []byte(`{}`)
}

// --- writeUpdatedQuarantineState: 403 branch protection ---

func TestWriteUpdatedQuarantineState403EmitsWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	cmd, buf := newCaptureCmd(t)
	client := newWriteStateClient(t, server)
	state, originalContent := dirtyState(t)
	cfg := &config.Config{Storage: config.StorageConfig{Branch: "quarantine/state"}}

	writeUpdatedQuarantineState(context.Background(), cmd, cfg, state, originalContent, "", client, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT /contents returns 403 (branch protection)",
		Should:   "emit a warning mentioning the branch is protected",
		Actual:   strings.Contains(buf.String(), "protected"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT /contents returns 403 (branch protection)",
		Should:   "name the specific branch in the warning",
		Actual:   strings.Contains(buf.String(), "quarantine/state"),
		Expected: true,
	})
}

// --- writeUpdatedQuarantineState: 422 oversized state ---

func TestWriteUpdatedQuarantineState422EmitsWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	t.Cleanup(server.Close)

	cmd, buf := newCaptureCmd(t)
	client := newWriteStateClient(t, server)
	state, originalContent := dirtyState(t)
	cfg := &config.Config{Storage: config.StorageConfig{Branch: "quarantine/state"}}

	writeUpdatedQuarantineState(context.Background(), cmd, cfg, state, originalContent, "", client, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PUT /contents returns 422 (oversized state)",
		Should:   "emit a warning mentioning the 1 MB limit",
		Actual:   strings.Contains(buf.String(), "1 MB"),
		Expected: true,
	})
}
