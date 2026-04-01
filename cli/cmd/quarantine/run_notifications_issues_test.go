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

	gh "github.com/mycargus/quarantine/cli/internal/github"
	"github.com/mycargus/quarantine/cli/internal/result"
	riteway "github.com/mycargus/riteway-golang"
	"github.com/spf13/cobra"
)

// newNotifTestClient constructs a gh.Client pointing at the given fake server URL.
func newNotifTestClient(t *testing.T, serverURL string) *gh.Client {
	t.Helper()
	t.Setenv("QUARANTINE_GITHUB_TOKEN", "ghp_test")
	t.Setenv("QUARANTINE_GITHUB_API_BASE_URL", serverURL)
	client, err := gh.NewClient("test-owner", "test-repo")
	if err != nil {
		t.Fatalf("newNotifTestClient: %v", err)
	}
	client.SetRetryDelay(0)
	return client
}

// discardCmd returns a cobra.Command that discards all printed output.
func discardCmd() *cobra.Command {
	cmd := &cobra.Command{}
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	return cmd
}

// fakeDedupeAndCreateServer builds a minimal httptest.Server that handles:
//   - GET /search/issues?…is%3Aopen  → returns openIssueResp
//   - POST /repos/…/issues           → increments issueCreatedPtr and returns a stub
func fakeDedupeAndCreateServer(
	t *testing.T,
	openIssueResp map[string]interface{},
	issueCreatedPtr *int32,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(openIssueResp)

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			atomic.AddInt32(issueCreatedPtr, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   200,
				"html_url": "https://github.com/test-owner/test-repo/issues/200",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// noOpenIssues is a convenience value for dedup search returning no results.
var noOpenIssues = map[string]interface{}{
	"total_count": 0,
	"items":       []interface{}{},
}

// --- Mutation 7: t.Status != "flaky" → == "flaky" ---

// TestCreateIssuesSkipsNonFlakyTests verifies that passed/failed tests are not
// processed by createIssuesForNewFlakyTests. The mutation flips the guard so
// non-flaky tests ARE processed; this test catches that by asserting zero issue
// creation calls when the result contains only passed/failed entries.
func TestCreateIssuesSkipsNonFlakyTests(t *testing.T) {
	var issueCreated int32
	server := fakeDedupeAndCreateServer(t, noOpenIssues, &issueCreated)
	defer server.Close()

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::passed-test", Name: "passed-test", Status: "passed"},
			{TestID: "suite::failed-test", Name: "failed-test", Status: "failed"},
		},
	}

	refs := createIssuesForNewFlakyTests(
		context.Background(), discardCmd(), client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result contains only passed and failed tests (no flaky)",
		Should:   "not create any GitHub issues",
		Actual:   int(atomic.LoadInt32(&issueCreated)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result contains only passed and failed tests (no flaky)",
		Should:   "return an empty refs map",
		Actual:   len(refs),
		Expected: 0,
	})
}

// TestCreateIssuesOnlyProcessesFlakyStatus verifies the positive case: a flaky
// test triggers issue creation while a passed test in the same result does not.
func TestCreateIssuesOnlyProcessesFlakyStatus(t *testing.T) {
	var issueCreated int32
	server := fakeDedupeAndCreateServer(t, noOpenIssues, &issueCreated)
	defer server.Close()

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-test", Name: "flaky-test", Status: "flaky"},
			{TestID: "suite::passed-test", Name: "passed-test", Status: "passed"},
		},
	}

	refs := createIssuesForNewFlakyTests(
		context.Background(), discardCmd(), client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result contains one flaky and one passed test",
		Should:   "create exactly one GitHub issue (for the flaky test only)",
		Actual:   int(atomic.LoadInt32(&issueCreated)),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result contains one flaky and one passed test",
		Should:   "return a ref map with exactly one entry",
		Actual:   len(refs),
		Expected: 1,
	})
}

// --- Mutation 8: if found → if !found ---

// TestCreateIssuesSkipsCreationWhenDedupFindsExistingIssue verifies that when
// dedup search returns an existing open issue, CreateIssue is never called.
// The mutation inverts the `found` check, so a found issue triggers creation
// instead of skipping it; this test catches that.
func TestCreateIssuesSkipsCreationWhenDedupFindsExistingIssue(t *testing.T) {
	var issueCreated int32

	existingIssueResp := map[string]interface{}{
		"total_count": 1,
		"items": []interface{}{
			map[string]interface{}{
				"number":   42,
				"html_url": "https://github.com/test-owner/test-repo/issues/42",
			},
		},
	}
	server := fakeDedupeAndCreateServer(t, existingIssueResp, &issueCreated)
	defer server.Close()

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-test", Name: "flaky-test", Status: "flaky"},
		},
	}

	refs := createIssuesForNewFlakyTests(
		context.Background(), discardCmd(), client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "dedup search finds an existing open issue for the flaky test",
		Should:   "not call CreateIssue",
		Actual:   int(atomic.LoadInt32(&issueCreated)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "dedup search finds an existing open issue",
		Should:   "return the existing issue number in the refs map",
		Actual:   refs["suite::flaky-test"].Number,
		Expected: 42,
	})
}

// --- Mutation 9: apiErr.StatusCode == 410 → != 410 ---

// TestCreateIssues410BreaksLoopAfterFirstAttempt verifies that a 410 Gone
// response breaks the issue-creation loop after one attempt, logs the
// "Issues disabled" warning, and returns an empty refs map.
// The mutation changes == 410 to != 410, so a 410 falls through to the
// per-test continue path (both tests are attempted) and a non-410 error
// triggers the break. The attempt-count and warning-text assertions catch both.
func TestCreateIssues410BreaksLoopAfterFirstAttempt(t *testing.T) {
	var issueAttempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(noOpenIssues)

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			atomic.AddInt32(&issueAttempts, 1)
			w.WriteHeader(http.StatusGone) // 410
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Issues are disabled for this repo",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	var warnings strings.Builder
	cmd := &cobra.Command{}
	cmd.SetErr(&warnings)
	cmd.SetOut(&warnings)

	client := newNotifTestClient(t, server.URL)

	// Two flaky tests: after the 410 on the first, the second must not be attempted.
	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-one", Name: "flaky-one", Status: "flaky"},
			{TestID: "suite::flaky-two", Name: "flaky-two", Status: "flaky"},
		},
	}

	refs := createIssuesForNewFlakyTests(
		context.Background(), cmd, client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GitHub Issues disabled (410 Gone) on first creation attempt",
		Should:   "attempt issue creation exactly once then stop",
		Actual:   int(atomic.LoadInt32(&issueAttempts)),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GitHub Issues disabled (410 Gone)",
		Should:   "return an empty refs map",
		Actual:   len(refs),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub Issues disabled (410 Gone)",
		Should:   "log the 'GitHub Issues are disabled' warning",
		Actual:   strings.Contains(warnings.String(), "GitHub Issues are disabled"),
		Expected: true,
	})
}

// TestCreateIssuesNon410ErrorContinuesLoop verifies that a non-410 error logs a
// per-test warning and continues processing subsequent tests, not breaking the
// loop. This is the counterpart to the 410 break test.
func TestCreateIssuesNon410ErrorContinuesLoop(t *testing.T) {
	var issueAttempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(noOpenIssues)

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			atomic.AddInt32(&issueAttempts, 1)
			w.WriteHeader(http.StatusInternalServerError) // 500, not 410
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Internal Server Error",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-one", Name: "flaky-one", Status: "flaky"},
			{TestID: "suite::flaky-two", Name: "flaky-two", Status: "flaky"},
		},
	}

	createIssuesForNewFlakyTests(
		context.Background(), discardCmd(), client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GitHub API returns 500 (non-410) on issue creation",
		Should:   "attempt issue creation for both tests (loop continues after error)",
		Actual:   int(atomic.LoadInt32(&issueAttempts)),
		Expected: 2,
	})
}

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

// --- Mutation 12: t.Status != "flaky" → == "flaky" in createIssuesForNewFlakyTests ---

// TestCreateIssuesSkipsFailedTests verifies that a test with Status=="failed"
// is skipped and does not produce an issue. This kills the mutation that flips
// the guard so failed tests ARE processed.
func TestCreateIssuesSkipsFailedTests(t *testing.T) {
	var issueCreated int32
	server := fakeDedupeAndCreateServer(t, noOpenIssues, &issueCreated)
	defer server.Close()

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::failed-test", Name: "failed-test", Status: "failed"},
		},
	}

	refs := createIssuesForNewFlakyTests(
		context.Background(), discardCmd(), client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result with only a failed test",
		Should:   "create no issues",
		Actual:   int(atomic.LoadInt32(&issueCreated)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "result with only a failed test",
		Should:   "return an empty refs map",
		Actual:   len(refs),
		Expected: 0,
	})
}

// --- Mutation 13: skipReasons[t.TestID] != "" → == "" in createIssuesForNewFlakyTests ---

// TestCreateIssuesSkipsTestsWithSkipReasons kills the mutation on line 325:
// `skipReasons[t.TestID] != ""` → `== ""`. Tests that have a skip reason must
// NOT be processed for issue creation.
func TestCreateIssuesSkipsTestsWithSkipReasons(t *testing.T) {
	var issueCreated int32
	server := fakeDedupeAndCreateServer(t, noOpenIssues, &issueCreated)
	defer server.Close()

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-new-to-pr", Name: "flaky-new-to-pr", Status: "flaky"},
		},
	}
	skipReasons := map[string]string{
		"suite::flaky-new-to-pr": "new-to-pr",
	}

	refs := createIssuesForNewFlakyTests(
		context.Background(), discardCmd(), client,
		res, nil, "main", "abc123", 0, skipReasons,
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "flaky test with a skip reason",
		Should:   "not create a GitHub issue",
		Actual:   int(atomic.LoadInt32(&issueCreated)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "flaky test with a skip reason",
		Should:   "not add the test to the refs map",
		Actual:   len(refs),
		Expected: 0,
	})
}

// TestCreateIssuesProcessesTestsWithoutSkipReasons verifies the counterpart:
// a flaky test WITHOUT a skip reason is processed and an issue is created.
func TestCreateIssuesProcessesTestsWithoutSkipReasons(t *testing.T) {
	var issueCreated int32
	server := fakeDedupeAndCreateServer(t, noOpenIssues, &issueCreated)
	defer server.Close()

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-normal", Name: "flaky-normal", Status: "flaky"},
		},
	}
	// No skip reason for this test.
	skipReasons := map[string]string{}

	refs := createIssuesForNewFlakyTests(
		context.Background(), discardCmd(), client,
		res, nil, "main", "abc123", 0, skipReasons,
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "flaky test without a skip reason",
		Should:   "create a GitHub issue",
		Actual:   int(atomic.LoadInt32(&issueCreated)),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "flaky test without a skip reason",
		Should:   "add the test to the refs map",
		Actual:   len(refs),
		Expected: 1,
	})
}

// --- Mutation 14: found → !found in createIssuesForNewFlakyTests ---
// (Already covered by TestCreateIssuesSkipsCreationWhenDedupFindsExistingIssue
// but the following test specifically names the mutation for clarity.)

// TestCreateIssuesExistingIssueReturnsItsRef verifies that when dedup search
// finds an open issue, the returned refs map contains that issue's details.
// This catches the mutation that inverts `found`, causing a new issue to be
// created even when one already exists, and the existing one's ref to be lost.
func TestCreateIssuesExistingIssueReturnsItsRef(t *testing.T) {
	var issueCreated int32

	existingResp := map[string]interface{}{
		"total_count": 1,
		"items": []interface{}{
			map[string]interface{}{
				"number":   55,
				"html_url": "https://github.com/test-owner/test-repo/issues/55",
			},
		},
	}
	server := fakeDedupeAndCreateServer(t, existingResp, &issueCreated)
	defer server.Close()

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::known-flaky", Name: "known-flaky", Status: "flaky"},
		},
	}

	refs := createIssuesForNewFlakyTests(
		context.Background(), discardCmd(), client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "dedup search finds issue #55 for the flaky test",
		Should:   "not create a new issue",
		Actual:   int(atomic.LoadInt32(&issueCreated)),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "dedup search finds issue #55",
		Should:   "return the existing issue URL in the refs map",
		Actual:   refs["suite::known-flaky"].URL,
		Expected: "https://github.com/test-owner/test-repo/issues/55",
	})
}

// --- Mutation 15: t.FailureMessage != nil → == nil in createIssuesForNewFlakyTests ---

// TestCreateIssuesIncludesFailureMessageInIssueBody kills the mutation on
// line 344: `t.FailureMessage != nil` → `== nil`. When the test has a
// non-nil FailureMessage, the issue body sent to GitHub must contain it.
func TestCreateIssuesIncludesFailureMessageInIssueBody(t *testing.T) {
	var capturedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(noOpenIssues)

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			var req map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if b, ok := req["body"].(string); ok {
				capturedBody = b
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   300,
				"html_url": "https://github.com/test-owner/test-repo/issues/300",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newNotifTestClient(t, server.URL)

	fm := "assertion failed: expected true but got false"
	res := result.Result{
		Tests: []result.TestEntry{
			{
				TestID:         "suite::flaky-with-msg",
				Name:           "flaky-with-msg",
				Status:         "flaky",
				FailureMessage: &fm,
			},
		},
	}

	createIssuesForNewFlakyTests(
		context.Background(), discardCmd(), client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "flaky test with non-nil FailureMessage",
		Should:   "include the failure message text in the issue body",
		Actual:   strings.Contains(capturedBody, "assertion failed: expected true but got false"),
		Expected: true,
	})
}

// TestCreateIssuesNilFailureMessageProducesEmptyBodySection verifies that a
// nil FailureMessage does not crash and produces an issue body without the
// failure text (empty string is passed to renderIssueBody).
func TestCreateIssuesNilFailureMessageProducesEmptyBodySection(t *testing.T) {
	var capturedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(noOpenIssues)

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			var req map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if b, ok := req["body"].(string); ok {
				capturedBody = b
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   301,
				"html_url": "https://github.com/test-owner/test-repo/issues/301",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{
				TestID:         "suite::flaky-no-msg",
				Name:           "flaky-no-msg",
				Status:         "flaky",
				FailureMessage: nil,
			},
		},
	}

	createIssuesForNewFlakyTests(
		context.Background(), discardCmd(), client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "flaky test with nil FailureMessage",
		Should:   "still create an issue (non-empty body sent to GitHub)",
		Actual:   len(capturedBody) > 0,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "flaky test with nil FailureMessage",
		Should:   "not include a Failure Message section in the issue body",
		Actual:   strings.Contains(capturedBody, "### Failure Message"),
		Expected: false,
	})
}

// --- Mutation 16: apiErr.StatusCode == 410 → != 410 ---
// (Already covered by TestCreateIssues410BreaksLoopAfterFirstAttempt and
// TestCreateIssuesNon410ErrorContinuesLoop. Adding a focused warning-text test
// to ensure the 410 path logs the right message and the non-410 path does not.)

// TestCreateIssues410LogsDisabledWarning verifies that a 410 response logs the
// "GitHub Issues are disabled" warning (not a per-test warning).
func TestCreateIssues410LogsDisabledWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(noOpenIssues)

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			w.WriteHeader(http.StatusGone) // 410
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Issues are disabled",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetErr(&buf)
	cmd.SetOut(&buf)

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-one", Name: "flaky-one", Status: "flaky"},
		},
	}

	createIssuesForNewFlakyTests(
		context.Background(), cmd, client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub returns 410 Gone on issue creation",
		Should:   "log 'GitHub Issues are disabled' warning",
		Actual:   strings.Contains(buf.String(), "GitHub Issues are disabled"),
		Expected: true,
	})
}

// TestCreateIssuesNon410LogsPerTestWarning verifies that a non-410 error logs a
// per-test warning (not the "Issues disabled" message), and does NOT break the loop.
func TestCreateIssuesNon410LogsPerTestWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search/issues") &&
			strings.Contains(r.URL.RawQuery, "is%3Aopen"):
			_ = json.NewEncoder(w).Encode(noOpenIssues)

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/issues"):
			w.WriteHeader(http.StatusInternalServerError) // 500
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Internal Server Error",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetErr(&buf)
	cmd.SetOut(&buf)

	client := newNotifTestClient(t, server.URL)

	res := result.Result{
		Tests: []result.TestEntry{
			{TestID: "suite::flaky-one", Name: "flaky-one", Status: "flaky"},
		},
	}

	createIssuesForNewFlakyTests(
		context.Background(), cmd, client,
		res, nil, "main", "abc123", 0, nil,
	)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub returns 500 (non-410) on issue creation",
		Should:   "not log 'GitHub Issues are disabled' warning",
		Actual:   strings.Contains(buf.String(), "GitHub Issues are disabled"),
		Expected: false,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "GitHub returns 500 (non-410) on issue creation",
		Should:   "log per-test 'Could not create issue' warning",
		Actual:   strings.Contains(buf.String(), "Could not create issue"),
		Expected: true,
	})
}
