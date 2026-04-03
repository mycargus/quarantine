package github

import (
	"net/http"
	"testing"
	"time"

	riteway "github.com/mycargus/riteway-golang"
)

// newResponseWithStatus creates a minimal *http.Response with the given status
// and headers for use in pure-function unit tests.
func newResponseWithStatus(status int, headers map[string]string) *http.Response {
	h := make(http.Header)
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{StatusCode: status, Header: h}
}

// --- shouldRetryHTTP ---

func TestShouldRetryHTTP5xx(t *testing.T) {
	for _, status := range []int{500, 502, 503, 504} {
		resp := newResponseWithStatus(status, nil)
		riteway.Assert(t, riteway.Case[bool]{
			Given:    "a 5xx response",
			Should:   "return true (retry warranted)",
			Actual:   shouldRetryHTTP(resp),
			Expected: true,
		})
	}
}

func TestShouldRetryHTTP429(t *testing.T) {
	resp := newResponseWithStatus(http.StatusTooManyRequests, nil)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a 429 response",
		Should:   "return true (retry warranted)",
		Actual:   shouldRetryHTTP(resp),
		Expected: true,
	})
}

func TestShouldRetryHTTP403WithRetryAfter(t *testing.T) {
	resp := newResponseWithStatus(http.StatusForbidden, map[string]string{
		"Retry-After": "5",
	})
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a 403 response with Retry-After header",
		Should:   "return true (retry warranted)",
		Actual:   shouldRetryHTTP(resp),
		Expected: true,
	})
}

func TestShouldRetryHTTP403WithoutRetryAfter(t *testing.T) {
	resp := newResponseWithStatus(http.StatusForbidden, nil)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a 403 response without Retry-After header",
		Should:   "return false (not retryable)",
		Actual:   shouldRetryHTTP(resp),
		Expected: false,
	})
}

func TestShouldRetryHTTP401(t *testing.T) {
	resp := newResponseWithStatus(http.StatusUnauthorized, nil)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a 401 response",
		Should:   "return false (not retryable)",
		Actual:   shouldRetryHTTP(resp),
		Expected: false,
	})
}

func TestShouldRetryHTTP200(t *testing.T) {
	resp := newResponseWithStatus(http.StatusOK, nil)
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a 200 response",
		Should:   "return false (not retryable)",
		Actual:   shouldRetryHTTP(resp),
		Expected: false,
	})
}

// --- retryWaitDuration ---

func TestRetryWaitDurationNoHeader(t *testing.T) {
	resp := newResponseWithStatus(http.StatusServiceUnavailable, nil)
	defaultWait := 5 * time.Second
	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a response with no Retry-After header",
		Should:   "return the defaultWait duration",
		Actual:   retryWaitDuration(resp, defaultWait),
		Expected: defaultWait,
	})
}

func TestRetryWaitDurationShortRetryAfter(t *testing.T) {
	resp := newResponseWithStatus(http.StatusTooManyRequests, map[string]string{
		"Retry-After": "10",
	})
	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a response with Retry-After: 10 (within 30s threshold)",
		Should:   "return 10 seconds",
		Actual:   retryWaitDuration(resp, 2*time.Second),
		Expected: 10 * time.Second,
	})
}

func TestRetryWaitDurationLongRetryAfterReturnsZero(t *testing.T) {
	resp := newResponseWithStatus(http.StatusTooManyRequests, map[string]string{
		"Retry-After": "31",
	})
	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a response with Retry-After: 31 (exceeds 30s threshold)",
		Should:   "return 0 (signal: do not retry)",
		Actual:   retryWaitDuration(resp, 2*time.Second),
		Expected: 0,
	})
}

func TestRetryWaitDurationInvalidHeaderFallsBackToDefault(t *testing.T) {
	resp := newResponseWithStatus(http.StatusTooManyRequests, map[string]string{
		"Retry-After": "not-a-number",
	})
	defaultWait := 3 * time.Second
	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a response with non-numeric Retry-After header",
		Should:   "fall back to defaultWait duration",
		Actual:   retryWaitDuration(resp, defaultWait),
		Expected: defaultWait,
	})
}

func TestRetryWaitDurationNegativeHeaderFallsBackToDefault(t *testing.T) {
	resp := newResponseWithStatus(http.StatusTooManyRequests, map[string]string{
		"Retry-After": "-5",
	})
	defaultWait := 3 * time.Second
	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a response with negative Retry-After header",
		Should:   "fall back to defaultWait duration",
		Actual:   retryWaitDuration(resp, defaultWait),
		Expected: defaultWait,
	})
}
