package cas_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	reQuarantined, err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "sha-initial", "quarantine/state", 3, nil)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "UpdateContents returns 200 on first attempt",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "UpdateContents returns 200 on first attempt with no CAS conflict",
		Should:   "return zero re-quarantined tests",
		Actual:   len(reQuarantined),
		Expected: 0,
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
	reQuarantined, err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "stale-sha", "quarantine/state", 3, nil)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "first UpdateContents returns 409, re-read succeeds, second UpdateContents returns 200",
		Should:   "return no error after retry",
		Actual:   err,
		Expected: nil,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "local has no tests and remote has one test contributed by Build A",
		Should:   "return zero re-quarantined tests (new remote test is not a re-quarantine)",
		Actual:   len(reQuarantined),
		Expected: 0,
	})
}

// TestWriteStateWithCASMergedContentContainsBothBuildsTests verifies scenario 28:
// after a 409, the content written on retry is the union of both builds' test sets.
func TestWriteStateWithCASMergedContentContainsBothBuildsTests(t *testing.T) {
	// Build A wrote ApiService test (remote state).
	remoteState := quarantine.NewEmptyState()
	remoteState.AddTest(quarantine.Entry{
		TestID: "api_test.js::ApiService::should handle timeout",
		Name:   "should handle timeout",
	})
	remoteContent := marshalState(t, remoteState)
	remoteEncoded := base64.StdEncoding.EncodeToString(remoteContent)

	// Track the content written on the successful PUT.
	var writtenContent []byte
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": remoteEncoded,
				"sha":     "def456",
			})
			return
		}
		if n == 1 {
			// First PUT: Build B's stale SHA → 409
			w.WriteHeader(http.StatusConflict)
			return
		}
		// Second PUT: merged content → 200. Capture request body.
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			decoded, _ := base64.StdEncoding.DecodeString(body.Content)
			writtenContent = decoded
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	// Build B's local state: CacheService test.
	localState := quarantine.NewEmptyState()
	localState.AddTest(quarantine.Entry{
		TestID: "cache_test.js::CacheService::should handle eviction",
		Name:   "should handle eviction",
	})
	content := marshalState(t, localState)

	c := newTestGHClient(t, server.URL)
	_, err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "abc123", "quarantine/state", 3, nil)
	if err != nil {
		t.Fatalf("WriteStateWithCAS: %v", err)
	}

	// Parse the content that was actually written on retry.
	merged, parseErr := quarantine.ParseState(bytes.NewReader(writtenContent))
	if parseErr != nil {
		t.Fatalf("parse written content: %v", parseErr)
	}

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Build A wrote ApiService test; Build B has CacheService test; 409 forces merge",
		Should:   "write merged state with both tests",
		Actual:   len(merged.Tests),
		Expected: 2,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Build B's CacheService test",
		Should:   "be present in the merged write",
		Actual:   merged.HasTest("cache_test.js::CacheService::should handle eviction"),
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "Build A's ApiService test (from remote state)",
		Should:   "be present in the merged write (no data loss)",
		Actual:   merged.HasTest("api_test.js::ApiService::should handle timeout"),
		Expected: true,
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
	_, err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "sha", "quarantine/state", 3, nil)

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
	_, err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "sha", "quarantine/state", 3, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "UpdateContents returns 403 (non-retryable error)",
		Should:   "return an error immediately",
		Actual:   err != nil,
		Expected: true,
	})
}

// --- Scenario 29: Quarantine/unquarantine race — quarantine wins ---

// TestWriteStateWithCASQuarantineWinsOnUnquarantineRace verifies scenario 29:
// Build B tries to remove a test (unquarantine), but Build A wrote it back.
// After 409 and merge, the test stays quarantined and re-quarantined IDs are returned.
func TestWriteStateWithCASQuarantineWinsOnUnquarantineRace(t *testing.T) {
	// Remote state (Build A wrote this): contains eviction test.
	remoteState := quarantine.NewEmptyState()
	remoteState.AddTest(quarantine.Entry{
		TestID: "cache_test.js::CacheService::should handle eviction",
		Name:   "should handle eviction",
	})
	remoteContent := marshalState(t, remoteState)
	remoteEncoded := base64.StdEncoding.EncodeToString(remoteContent)

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"content": remoteEncoded,
				"sha":     "def456",
			})
			return
		}
		if n == 1 {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	// Build B's local state: eviction test was removed (unquarantined), ApiService added.
	localState := quarantine.NewEmptyState()
	localState.AddTest(quarantine.Entry{
		TestID: "api_test.js::ApiService::should handle timeout",
		Name:   "should handle timeout",
	})
	// Notably: eviction test is NOT in localState (Build B removed it).
	content := marshalState(t, localState)

	// Build B explicitly unquarantined the eviction test (issue closed).
	removedByB := []string{"cache_test.js::CacheService::should handle eviction"}

	c := newTestGHClient(t, server.URL)
	reQuarantined, err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "abc123", "quarantine/state", 3, removedByB)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "Build B unquarantined eviction test; Build A wrote it back; 409 forces merge",
		Should:   "return no error (quarantine wins, write succeeds on retry)",
		Actual:   err,
		Expected: nil,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "eviction test was in remote (Build A) but not local (Build B unquarantined it)",
		Should:   "return one re-quarantined test ID",
		Actual:   len(reQuarantined),
		Expected: 1,
	})
	riteway.Assert(t, riteway.Case[string]{
		Given:    "eviction test re-quarantined after CAS conflict",
		Should:   "identify the eviction test as re-quarantined",
		Actual:   reQuarantined[0],
		Expected: "cache_test.js::CacheService::should handle eviction",
	})
}

// TestDetectReQuarantinedIsZeroWhenNoConflict verifies the pure detection:
// when no CAS conflict occurs, no re-quarantined tests are reported.
func TestDetectReQuarantinedIsZeroWhenNoConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	localState := quarantine.NewEmptyState()
	localState.AddTest(quarantine.Entry{
		TestID: "api_test.js::ApiService::should handle timeout",
		Name:   "should handle timeout",
	})
	content := marshalState(t, localState)

	c := newTestGHClient(t, server.URL)
	reQuarantined, err := cas.WriteStateWithCAS(context.Background(), c, localState, content, "abc123", "quarantine/state", 3, nil)

	riteway.Assert(t, riteway.Case[error]{
		Given:    "UpdateContents returns 200 on first attempt (no conflict)",
		Should:   "return no error",
		Actual:   err,
		Expected: nil,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "no CAS conflict occurred",
		Should:   "return zero re-quarantined tests",
		Actual:   len(reQuarantined),
		Expected: 0,
	})
}

// --- Additional mutation-coverage tests ---

// mockContentsClient is a minimal implementation of githubContentsClient for
// fine-grained control in unit tests.
type mockContentsClient struct {
	updateFn func(ctx context.Context, path, branch, message string, content []byte, sha string) error
	getFn    func(ctx context.Context, path, ref string) ([]byte, string, error)
}

func (m *mockContentsClient) UpdateContents(ctx context.Context, path, branch, message string, content []byte, sha string) error {
	return m.updateFn(ctx, path, branch, message, content, sha)
}

func (m *mockContentsClient) GetContents(ctx context.Context, path, ref string) ([]byte, string, error) {
	return m.getFn(ctx, path, ref)
}

// TestWriteStateWithCASNon409ErrorIsNonRetryable verifies that when UpdateContents
// returns a non-APIError (e.g. a plain Go error), the call is NOT retried.
// Kills mutation on line 62: `||` → `&&`.
// With `&&`, !errors.As() = true → accesses nil apiErr.StatusCode → panic.
func TestWriteStateWithCASNon409ErrorIsNonRetryable(t *testing.T) {
	var callCount int32
	nonAPIErr := fmt.Errorf("connection reset by peer")

	client := &mockContentsClient{
		updateFn: func(_ context.Context, _, _, _ string, _ []byte, _ string) error {
			atomic.AddInt32(&callCount, 1)
			return nonAPIErr
		},
		getFn: func(_ context.Context, _, _ string) ([]byte, string, error) {
			return nil, "", nil
		},
	}

	localState := quarantine.NewEmptyState()
	content, _ := localState.Marshal()

	_, err := cas.WriteStateWithCAS(context.Background(), client, localState, content, "sha", "branch", 3, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "UpdateContents returns a non-APIError (connection reset by peer)",
		Should:   "return an error immediately",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "UpdateContents returns a non-APIError",
		Should:   "call UpdateContents exactly once (no retry for non-409 errors)",
		Actual:   atomic.LoadInt32(&callCount),
		Expected: 1,
	})
}

// TestWriteStateWithCASExhaustsExactlyMaxRetriesPUTs verifies the loop runs
// exactly maxRetries PUT attempts before giving up.
// Kills mutations on line 55: `<= maxRetries` → `< maxRetries` (one fewer iter),
// and `attempt := 1` → `0` or `2` (different iteration counts).
func TestWriteStateWithCASExhaustsExactlyMaxRetriesPUTs(t *testing.T) {
	var putCount int32

	emptyStateBytes, _ := quarantine.NewEmptyState().Marshal()
	emptyEncoded := base64.StdEncoding.EncodeToString(emptyStateBytes)

	client := &mockContentsClient{
		updateFn: func(_ context.Context, _, _, _ string, _ []byte, _ string) error {
			atomic.AddInt32(&putCount, 1)
			// Always return 409 so all maxRetries PUTs are made.
			return &ghclient.APIError{StatusCode: 409, Message: "conflict"}
		},
		getFn: func(_ context.Context, _, _ string) ([]byte, string, error) {
			// Return empty remote state so merge always succeeds.
			decoded, _ := base64.StdEncoding.DecodeString(emptyEncoded)
			return decoded, "new-sha", nil
		},
	}

	localState := quarantine.NewEmptyState()
	content, _ := localState.Marshal()

	const maxRetries = 3
	_, err := cas.WriteStateWithCAS(context.Background(), client, localState, content, "sha", "branch", maxRetries, nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all UpdateContents calls return 409",
		Should:   "return an error after exhausting retries",
		Actual:   err != nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    fmt.Sprintf("maxRetries=%d with all 409s", maxRetries),
		Should:   "attempt UpdateContents exactly maxRetries (3) times",
		Actual:   atomic.LoadInt32(&putCount),
		Expected: maxRetries,
	})
}
