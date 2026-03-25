package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Unit tests for pure functions: testHash, detectPRNumber, renderIssueBody, renderPRComment ---

func TestTestHash(t *testing.T) {
	// SHA-256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	// First 8 hex chars: 2cf24dba
	riteway.Assert(t, riteway.Case[string]{
		Given:    `test_id "hello"`,
		Should:   "return first 8 hex chars of SHA-256",
		Actual:   testHash("hello"),
		Expected: "2cf24dba",
	})

	// Deterministic: same input → same output.
	riteway.Assert(t, riteway.Case[string]{
		Given:    "same test_id called twice",
		Should:   "return the same hash",
		Actual:   testHash("src/payment.test.js::PaymentService::should handle charge timeout"),
		Expected: testHash("src/payment.test.js::PaymentService::should handle charge timeout"),
	})

	// Different inputs → different hashes (collision is theoretically possible
	// but practically impossible with SHA-256 for these inputs).
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "two different test IDs",
		Should:   "return different hashes",
		Actual:   testHash("a::b::c") != testHash("x::y::z"),
		Expected: true,
	})

	// Hash is always exactly 8 hex characters.
	h := testHash("src/payment.test.js::PaymentService::should handle charge timeout")
	riteway.Assert(t, riteway.Case[int]{
		Given:    "any test_id",
		Should:   "return a hash of exactly 8 characters",
		Actual:   len(h),
		Expected: 8,
	})
}

func TestDetectPRNumber(t *testing.T) {
	// When --pr flag is set, use it directly.
	riteway.Assert(t, riteway.Case[int]{
		Given:    "--pr flag set to 42",
		Should:   "return 42 without reading any file",
		Actual: func() int {
			n, _ := detectPRNumber(42, "")
			return n
		}(),
		Expected: 42,
	})

	// When prFlag is 0 and GITHUB_EVENT_PATH points to a pull_request event.
	eventJSON := `{"pull_request":{"number":99}}`
	eventPath := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0644); err != nil {
		t.Fatalf("write event file: %v", err)
	}
	n, err := detectPRNumber(0, eventPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "prFlag=0 and GITHUB_EVENT_PATH contains pull_request.number=99",
		Should:   "return no error",
		Actual:   err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "prFlag=0 and GITHUB_EVENT_PATH contains pull_request.number=99",
		Should:   "return PR number 99",
		Actual:   n,
		Expected: 99,
	})

	// When prFlag is 0 and eventPath is empty.
	n, err = detectPRNumber(0, "")
	riteway.Assert(t, riteway.Case[int]{
		Given:    "prFlag=0 and no event path",
		Should:   "return 0",
		Actual:   n,
		Expected: 0,
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "prFlag=0 and no event path",
		Should:   "return no error",
		Actual:   err == nil,
		Expected: true,
	})
}

func TestRenderPRComment(t *testing.T) {
	data := PRCommentData{
		Total:   5,
		Passed:  4,
		Failed:  0,
		Flaky:   1,
		Version: "0.1.0",
		NewlyFlaky: []FlakyEntry{
			{
				Name:      "PaymentService > should handle charge timeout",
				IssueURL:  "https://github.com/test-owner/test-repo/issues/101",
				IssueNum:  101,
			},
		},
	}
	comment := renderPRComment(data)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment rendered",
		Should:   "start with <!-- quarantine-bot --> on the first line",
		Actual:   strings.HasPrefix(comment, "<!-- quarantine-bot -->"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment rendered with one newly flaky test",
		Should:   "contain the quarantine-bot marker",
		Actual:   strings.Contains(comment, "<!-- quarantine-bot -->"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment rendered with one newly flaky test",
		Should:   "contain the flaky test name",
		Actual:   strings.Contains(comment, "PaymentService > should handle charge timeout"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment rendered with one newly flaky test",
		Should:   "contain the issue URL",
		Actual:   strings.Contains(comment, "https://github.com/test-owner/test-repo/issues/101"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment rendered with one newly flaky test",
		Should:   "contain the version",
		Actual:   strings.Contains(comment, "0.1.0"),
		Expected: true,
	})
}

func TestRenderIssueBody(t *testing.T) {
	body := renderIssueBody(IssueBodyData{
		TestID:    "src/payment.test.js::PaymentService::should handle charge timeout",
		Suite:     "src/payment.test.js",
		Name:      "should handle charge timeout",
		Timestamp: "2026-03-25T12:00:00Z",
		Branch:    "main",
		CommitSHA: "abc1234",
		PRNumber:  99,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "issue body rendered",
		Should:   "contain the test ID",
		Actual:   strings.Contains(body, "src/payment.test.js::PaymentService::should handle charge timeout"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "issue body rendered",
		Should:   "contain the branch name",
		Actual:   strings.Contains(body, "main"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "issue body rendered",
		Should:   "contain the PR number",
		Actual:   strings.Contains(body, "99"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "issue body rendered",
		Should:   "mention that closing the issue will unquarantine the test",
		Actual:   strings.Contains(body, "Close this issue to unquarantine"),
		Expected: true,
	})
}

func TestDetectPRNumberFromEventFile(t *testing.T) {
	// pull_request_target event uses .number at top level.
	eventJSON := `{"number":55,"action":"opened"}`
	eventPath := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0644); err != nil {
		t.Fatalf("write event file: %v", err)
	}

	n, err := detectPRNumber(0, eventPath)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pull_request_target event with .number=55",
		Should:   "return no error",
		Actual:   err == nil,
		Expected: true,
	})
	riteway.Assert(t, riteway.Case[int]{
		Given:    "pull_request_target event with .number=55",
		Should:   "return 55",
		Actual:   n,
		Expected: 55,
	})
}

func TestDetectPRNumberPrefersFlag(t *testing.T) {
	// Even if event file has a different PR number, flag wins.
	eventJSON := `{"pull_request":{"number":77}}`
	eventPath := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0644); err != nil {
		t.Fatalf("write event file: %v", err)
	}

	n, _ := detectPRNumber(42, eventPath)
	riteway.Assert(t, riteway.Case[int]{
		Given:    "--pr flag=42 and event file contains PR number 77",
		Should:   "prefer the flag value (42)",
		Actual:   n,
		Expected: 42,
	})
}

// Test that the PR comment marker is on the FIRST line (spec requirement).
func TestRenderPRCommentMarkerIsFirstLine(t *testing.T) {
	data := PRCommentData{
		Total:   1,
		Passed:  1,
		Version: "0.1.0",
	}
	comment := renderPRComment(data)
	firstLine := strings.SplitN(comment, "\n", 2)[0]

	riteway.Assert(t, riteway.Case[string]{
		Given:    "PR comment with no flaky tests",
		Should:   "have <!-- quarantine-bot --> as the first line",
		Actual:   firstLine,
		Expected: "<!-- quarantine-bot -->",
	})
}

func TestRenderPRCommentNilFlakySection(t *testing.T) {
	// When there are no flaky tests, the flaky section should not appear.
	data := PRCommentData{
		Total:      2,
		Passed:     2,
		NewlyFlaky: nil,
		Version:    "0.1.0",
	}
	comment := renderPRComment(data)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "PR comment with no flaky tests",
		Should:   "not contain 'New Flaky Tests Detected' section",
		Actual:   strings.Contains(comment, "New Flaky Tests Detected"),
		Expected: false,
	})
}

// Verify that the JSON event parsing is correct.
func TestDetectPRNumberFromEventFileInvalidJSON(t *testing.T) {
	eventPath := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(eventPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("write event file: %v", err)
	}

	n, _ := detectPRNumber(0, eventPath)
	riteway.Assert(t, riteway.Case[int]{
		Given:    "event file with invalid JSON",
		Should:   "return 0 (skip gracefully)",
		Actual:   n,
		Expected: 0,
	})
}

