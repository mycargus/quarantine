package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

