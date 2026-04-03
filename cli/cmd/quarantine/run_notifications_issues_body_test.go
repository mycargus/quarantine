package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mycargus/quarantine/cli/internal/result"
	riteway "github.com/mycargus/riteway-golang"
	"github.com/spf13/cobra"
)

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
