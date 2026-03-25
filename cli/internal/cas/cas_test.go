package cas_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/cas"
	ghclient "github.com/mycargus/quarantine/internal/github"
	"github.com/mycargus/quarantine/internal/quarantine"
)

func newTestGHClient(t *testing.T, serverURL string) *ghclient.Client {
	t.Helper()
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "ghp_test")
	t.Setenv("QUARANTINE_GITHUB_API_BASE_URL", serverURL)
	c, err := ghclient.NewClient("my-org", "my-project")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetRetryDelay(0)
	return c
}

func marshalState(t *testing.T, s *quarantine.State) []byte {
	t.Helper()
	b, err := s.Marshal()
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	return b
}

// --- WriteStateWithCAS ---

func TestWriteStateWithCASSucceedsOnFirstAttempt(t *testing.T) {
	localState := quarantine.NewEmptyState()
	content := marshalState(t, localState)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	c := newTestGHClient(t, server.URL)
	err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "sha-initial", "quarantine/state", 3)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "UpdateContents returns 200 on first attempt",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
}

func TestWriteStateWithCASRetriesOn409AndSucceeds(t *testing.T) {
	remoteState := quarantine.NewEmptyState()
	remoteState.AddTest(quarantine.Entry{
		TestID: "file.js::Foo::bar",
		Name:   "bar",
	})
	remoteContent := marshalState(t, remoteState)
	remoteEncoded := base64.StdEncoding.EncodeToString(remoteContent)

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if r.Method == "GET" {
			// Re-read: return remote state with new SHA.
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": remoteEncoded,
				"sha":     "new-sha-after-conflict",
			})
			return
		}
		// First PUT → 409 conflict; second PUT → 200.
		if n <= 2 {
			w.WriteHeader(http.StatusConflict)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(server.Close)

	localState := quarantine.NewEmptyState()
	content := marshalState(t, localState)

	c := newTestGHClient(t, server.URL)
	err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "stale-sha", "quarantine/state", 3)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "first UpdateContents returns 409, re-read succeeds, second UpdateContents returns 200",
		Should:   "return no error after retry",
		Actual:   err,
		Expected: nil,
	})
}

func TestWriteStateWithCASReturnsErrorAfterExhaustingRetries(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		if r.Method == "GET" {
			// Re-read returns empty state.
			w.Header().Set("Content-Type", "application/json")
			emptyState := quarantine.NewEmptyState()
			b, _ := emptyState.Marshal()
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": base64.StdEncoding.EncodeToString(b),
				"sha":     "new-sha",
			})
			return
		}
		// All PUTs → 409.
		w.WriteHeader(http.StatusConflict)
	}))
	t.Cleanup(server.Close)

	localState := quarantine.NewEmptyState()
	content := marshalState(t, localState)

	c := newTestGHClient(t, server.URL)
	err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "sha", "quarantine/state", 3)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all 3 UpdateContents attempts return 409",
		Should:   "return an error after exhausting retries",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestWriteStateWithCASReturnsErrorOnNon409Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	localState := quarantine.NewEmptyState()
	content := marshalState(t, localState)

	c := newTestGHClient(t, server.URL)
	err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "sha", "quarantine/state", 3)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "UpdateContents returns 403 (non-retryable error)",
		Should:   "return an error immediately",
		Actual:   err != nil,
		Expected: true,
	})
}
