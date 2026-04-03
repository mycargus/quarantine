package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	gh "github.com/mycargus/quarantine/cli/internal/github"
	riteway "github.com/mycargus/riteway-golang"
)

// fakeTimeoutError implements net.Error with Timeout() == true.
type fakeTimeoutError struct{}

func (fakeTimeoutError) Error() string   { return "timeout" }
func (fakeTimeoutError) Timeout() bool   { return true }
func (fakeTimeoutError) Temporary() bool { return true }

var _ net.Error = fakeTimeoutError{}

// --- degradedMsg pure function ---

func TestDegradedMsgWith401(t *testing.T) {
	err := &gh.APIError{StatusCode: 401, Message: "unauthorized"}
	msg := degradedMsg(err)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 401",
		Should:   "mention '401' in the message",
		Actual:   strings.Contains(msg, "401"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 401",
		Should:   "mention 'QUARANTINE_GITHUB_TOKEN' in the message",
		Actual:   strings.Contains(msg, "QUARANTINE_GITHUB_TOKEN"),
		Expected: true,
	})
}

func TestDegradedMsgWith403WithRequestID(t *testing.T) {
	err := &gh.APIError{StatusCode: 403, Message: "forbidden", RequestID: "req-abc-123"}
	msg := degradedMsg(err)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 403 and RequestID",
		Should:   "mention '403' in the message",
		Actual:   strings.Contains(msg, "403"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 403 and RequestID 'req-abc-123'",
		Should:   "include the request ID in the message",
		Actual:   strings.Contains(msg, "req-abc-123"),
		Expected: true,
	})
}

func TestDegradedMsgWith403WithoutRequestID(t *testing.T) {
	err := &gh.APIError{StatusCode: 403, Message: "forbidden", RequestID: ""}
	msg := degradedMsg(err)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 403 and no RequestID",
		Should:   "mention '403' in the message",
		Actual:   strings.Contains(msg, "403"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 403 and no RequestID",
		Should:   "mention 'repo' scope in the message",
		Actual:   strings.Contains(msg, "repo"),
		Expected: true,
	})
}

func TestDegradedMsgWith429LongRetryAfter(t *testing.T) {
	err := &gh.APIError{StatusCode: 429, Message: "rate limited", RetryAfter: 31}
	msg := degradedMsg(err)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 429 and RetryAfter=31",
		Should:   "include '31s' in the message",
		Actual:   strings.Contains(msg, "31s"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 429 and RetryAfter=31",
		Should:   "mention 'threshold' in the message",
		Actual:   strings.Contains(msg, "threshold"),
		Expected: true,
	})
}

func TestDegradedMsgWith429ShortRetryAfter(t *testing.T) {
	err := &gh.APIError{StatusCode: 429, Message: "rate limited", RetryAfter: 5}
	msg := degradedMsg(err)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 429 and RetryAfter=5 (within threshold)",
		Should:   "mention rate limited in the message",
		Actual:   strings.Contains(msg, "rate limited") || strings.Contains(msg, "rate"),
		Expected: true,
	})
}

func TestDegradedMsgWith5xxServerError(t *testing.T) {
	err := &gh.APIError{StatusCode: 503, Message: "service unavailable"}
	msg := degradedMsg(err)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 503",
		Should:   "mention '503' in the message",
		Actual:   strings.Contains(msg, "503"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 503",
		Should:   "mention 'server error' in the message",
		Actual:   strings.Contains(msg, "server error"),
		Expected: true,
	})
}

func TestDegradedMsgWithGenericAPIError(t *testing.T) {
	// A non-5xx, non-401/403/429 API error falls through to the generic message.
	err := &gh.APIError{StatusCode: 422, Message: "unprocessable"}
	msg := degradedMsg(err)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "APIError with status 422 (not 401/403/429/5xx)",
		Should:   "return the generic 'Unable to reach GitHub API' message",
		Actual:   strings.Contains(msg, "Unable to reach GitHub API"),
		Expected: true,
	})
}

func TestDegradedMsgWithGenericNonAPIError(t *testing.T) {
	err := errors.New("some random error")
	msg := degradedMsg(err)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a generic non-APIError",
		Should:   "return the generic 'Unable to reach GitHub API' message",
		Actual:   strings.Contains(msg, "Unable to reach GitHub API"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a generic non-APIError with message 'some random error'",
		Should:   "include the original error in the message",
		Actual:   strings.Contains(msg, "some random error"),
		Expected: true,
	})
}

func TestDegradedMsgWithTimeoutError(t *testing.T) {
	err := fmt.Errorf("request failed: %w", fakeTimeoutError{})
	msg := degradedMsg(err)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a net.Error with Timeout()=true",
		Should:   "return the timeout-specific message",
		Actual:   strings.Contains(msg, "timed out"),
		Expected: true,
	})
}

func TestIsTimeoutErrorWithTimeoutError(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", fakeTimeoutError{})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a wrapped net.Error with Timeout()=true",
		Should:   "return true",
		Actual:   isTimeoutError(err),
		Expected: true,
	})
}

func TestIsTimeoutErrorWithNonTimeoutError(t *testing.T) {
	err := errors.New("ordinary error")
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "an ordinary error (not net.Error)",
		Should:   "return false",
		Actual:   isTimeoutError(err),
		Expected: false,
	})
}

func TestDegradedMsgWithWrappedAPIError(t *testing.T) {
	apiErr := &gh.APIError{StatusCode: 401, Message: "unauthorized"}
	wrapped := fmt.Errorf("wrapped: %w", apiErr)
	msg := degradedMsg(wrapped)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a wrapped APIError with status 401",
		Should:   "unwrap and recognize the 401 status code",
		Actual:   strings.Contains(msg, "401"),
		Expected: true,
	})
}
