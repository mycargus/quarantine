package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Mutation 10: prFlag > 0 → >= 0 ---

// TestDetectPRNumberZeroFlagReadsEventFile verifies that when prFlag is exactly
// 0, the function does NOT return early but falls through to parse the event
// file. The mutation changes > 0 to >= 0, which would return 0 immediately for
// prFlag=0, ignoring the event file entirely.
func TestDetectPRNumberZeroFlagReadsEventFile(t *testing.T) {
	eventJSON := `{"pull_request":{"number":123}}`
	eventPath := filepath.Join(t.TempDir(), "pr-event.json")
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0644); err != nil {
		t.Fatalf("write event file: %v", err)
	}

	n, err := detectPRNumber(0, eventPath)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "prFlag is 0 and event file contains pull_request.number=123",
		Should:   "return no error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "prFlag is 0 and event file contains pull_request.number=123",
		Should:   "return 123 from the event file (not 0 via early return)",
		Actual:   n,
		Expected: 123,
	})
}

// TestDetectPRNumberPositiveFlagSkipsEventFile verifies the counterpart: when
// prFlag > 0, the event file is not consulted and the flag value is returned.
func TestDetectPRNumberPositiveFlagSkipsEventFile(t *testing.T) {
	// Event file exists but contains a different PR number — it must be ignored.
	eventJSON := `{"pull_request":{"number":999}}`
	eventPath := filepath.Join(t.TempDir(), "pr-event.json")
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0644); err != nil {
		t.Fatalf("write event file: %v", err)
	}

	n, err := detectPRNumber(7, eventPath)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "prFlag is 7 and event file contains pull_request.number=999",
		Should:   "return no error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "prFlag is 7 (positive) — flag takes precedence",
		Should:   "return 7 (not 999 from event file)",
		Actual:   n,
		Expected: 7,
	})
}

// TestDetectPRNumberNegativeFlagFallsThroughToEventFile verifies that a negative
// prFlag value does NOT trigger the early return (it is not > 0), so the event
// file is consulted. This is a boundary case for the > 0 guard.
func TestDetectPRNumberNegativeFlagFallsThroughToEventFile(t *testing.T) {
	eventJSON := `{"number":55}`
	eventPath := filepath.Join(t.TempDir(), "pr-event.json")
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0644); err != nil {
		t.Fatalf("write event file: %v", err)
	}

	n, err := detectPRNumber(-1, eventPath)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "prFlag is -1 (not > 0) and event file contains .number=55",
		Should:   "return no error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "prFlag is -1 (negative, not a valid flag value)",
		Should:   "return 55 from the event file",
		Actual:   n,
		Expected: 55,
	})
}

// --- Mutation 10: client == nil || prNumber == 0 → && ---

// TestPostOrUpdatePRCommentNilClientIsNoOp kills the mutation on line 272:
// `client == nil || prNumber == 0` → `&&`.
// With OR, a nil client ALONE (even when prNumber is non-zero) is enough to
// short-circuit the function. The && mutation would require BOTH to be nil/zero.
func TestPostOrUpdatePRCommentNilClientIsNoOp(t *testing.T) {
	// If the function attempts any HTTP call it would panic (nil client).
	// The test passes by not panicking and not making any network calls.
	cmd := discardCmd()
	// prNumber=5 (non-zero), client=nil → must be a no-op.
	postOrUpdatePRComment(context.Background(), cmd, nil, 5, "body")
	// Reaching here means it returned early; test passes.
}

// TestPostOrUpdatePRCommentZeroPRNumberIsNoOp kills the mutation on line 272:
// `client == nil || prNumber == 0` → `&&`.
// With OR, prNumber==0 ALONE is enough to short-circuit even when client is
// non-nil. We verify no HTTP requests are made to a real server.
func TestPostOrUpdatePRCommentZeroPRNumberIsNoOp(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newNotifTestClient(t, server.URL)
	cmd := discardCmd()

	// prNumber=0 with a valid client → must be a no-op (no HTTP requests).
	postOrUpdatePRComment(context.Background(), cmd, client, 0, "body")

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "prNumber is 0 with a non-nil client",
		Should:   "make no HTTP requests (early return)",
		Actual:   atomic.LoadInt32(&requestCount),
		Expected: 0,
	})
}

// --- Mutation 11: strings.HasPrefix(c.Body, PRCommentMarker) → negated ---

// TestPostOrUpdatePRCommentUpdatesExistingMarkedComment kills the mutation on
// line 284: `strings.HasPrefix(c.Body, PRCommentMarker)` → negated.
// When an existing comment starts with PRCommentMarker, the function must call
// UpdatePRComment (PATCH) instead of CreatePRComment (POST).
func TestPostOrUpdatePRCommentUpdatesExistingMarkedComment(t *testing.T) {
	var createCalls, updateCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/issues/") &&
			strings.Contains(r.URL.Path, "/comments"):
			// Return one existing comment whose body starts with PRCommentMarker.
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":   int64(9001),
					"body": PRCommentMarker + "\n## Quarantine Summary\n",
				},
			})

		case r.Method == "PATCH" && strings.Contains(r.URL.Path, "/comments/"):
			atomic.AddInt32(&updateCalls, 1)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": int64(9001)})

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues/"):
			atomic.AddInt32(&createCalls, 1)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": int64(9002)})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newNotifTestClient(t, server.URL)
	cmd := discardCmd()

	postOrUpdatePRComment(context.Background(), cmd, client, 7, PRCommentMarker+"\n## Updated Summary")

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "an existing PR comment that starts with PRCommentMarker",
		Should:   "call UpdatePRComment (PATCH), not CreatePRComment (POST)",
		Actual:   atomic.LoadInt32(&updateCalls),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "an existing PR comment that starts with PRCommentMarker",
		Should:   "not call CreatePRComment (POST)",
		Actual:   atomic.LoadInt32(&createCalls),
		Expected: 0,
	})
}

// TestPostOrUpdatePRCommentCreatesWhenNoMarkedComment verifies the counterpart:
// when no existing comment has the PRCommentMarker, a new comment is created.
func TestPostOrUpdatePRCommentCreatesWhenNoMarkedComment(t *testing.T) {
	var createCalls, updateCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/issues/") &&
			strings.Contains(r.URL.Path, "/comments"):
			// Return one comment that does NOT have the marker.
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":   int64(100),
					"body": "Some unrelated comment",
				},
			})

		case r.Method == "PATCH" && strings.Contains(r.URL.Path, "/comments/"):
			atomic.AddInt32(&updateCalls, 1)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": int64(100)})

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues/"):
			atomic.AddInt32(&createCalls, 1)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": int64(101)})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newNotifTestClient(t, server.URL)
	cmd := discardCmd()

	postOrUpdatePRComment(context.Background(), cmd, client, 7, PRCommentMarker+"\n## Summary")

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "no existing comment with PRCommentMarker",
		Should:   "call CreatePRComment (POST), not UpdatePRComment (PATCH)",
		Actual:   atomic.LoadInt32(&createCalls),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "no existing comment with PRCommentMarker",
		Should:   "not call UpdatePRComment (PATCH)",
		Actual:   atomic.LoadInt32(&updateCalls),
		Expected: 0,
	})
}
