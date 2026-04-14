package config

// Tests for TestSuite.TimeoutDuration() and RerunTimeoutDuration().
// These are pure functions on struct fields — tests construct TestSuite
// directly rather than parsing YAML, keeping the relevant input visible.

import (
	"strings"
	"testing"
	"time"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Unit tests for TestSuite.TimeoutDuration() ---

func TestTimeoutDurationEmptyStringReturnsZero(t *testing.T) {
	suite := TestSuite{}

	d, parseErr := suite.TimeoutDuration()

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a TestSuite with no Timeout field",
		Should:   "return no parse error",
		Actual:   parseErr,
		Expected: nil,
	})
	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a TestSuite with no Timeout field",
		Should:   "return zero duration",
		Actual:   d,
		Expected: 0,
	})
}

func TestTimeoutDurationOneMinute(t *testing.T) {
	suite := TestSuite{Timeout: "1m"}

	d, parseErr := suite.TimeoutDuration()

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a TestSuite with Timeout '1m'",
		Should:   "return no parse error",
		Actual:   parseErr,
		Expected: nil,
	})
	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a TestSuite with Timeout '1m'",
		Should:   "return 1 minute duration",
		Actual:   d,
		Expected: time.Minute,
	})
}

func TestTimeoutDurationThirtySeconds(t *testing.T) {
	suite := TestSuite{Timeout: "30s"}

	d, parseErr := suite.TimeoutDuration()

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a TestSuite with Timeout '30s'",
		Should:   "return no parse error",
		Actual:   parseErr,
		Expected: nil,
	})
	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a TestSuite with Timeout '30s'",
		Should:   "return 30 seconds duration",
		Actual:   d,
		Expected: 30 * time.Second,
	})
}

func TestTimeoutDurationInvalidStringReturnsError(t *testing.T) {
	suite := TestSuite{Timeout: "notaduration"}

	_, parseErr := suite.TimeoutDuration()

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a TestSuite with Timeout 'notaduration'",
		Should:   "return a non-nil parse error",
		Actual:   parseErr != nil,
		Expected: true,
	})

	if parseErr != nil {
		riteway.Assert(t, riteway.Case[bool]{
			Given:    "a TestSuite with Timeout 'notaduration'",
			Should:   "include the invalid token in the error message",
			Actual:   strings.Contains(parseErr.Error(), "notaduration"),
			Expected: true,
		})
	}
}

// --- Unit tests for TestSuite.RerunTimeoutDuration() ---

func TestRerunTimeoutDurationEmptyStringReturnsZero(t *testing.T) {
	suite := TestSuite{}

	d, parseErr := suite.RerunTimeoutDuration()

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a TestSuite with no RerunTimeout field",
		Should:   "return no parse error",
		Actual:   parseErr,
		Expected: nil,
	})
	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a TestSuite with no RerunTimeout field",
		Should:   "return zero duration",
		Actual:   d,
		Expected: 0,
	})
}

func TestRerunTimeoutDurationTwoMinutes(t *testing.T) {
	suite := TestSuite{RerunTimeout: "2m"}

	d, parseErr := suite.RerunTimeoutDuration()

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a TestSuite with RerunTimeout '2m'",
		Should:   "return no parse error",
		Actual:   parseErr,
		Expected: nil,
	})
	riteway.Assert(t, riteway.Case[time.Duration]{
		Given:    "a TestSuite with RerunTimeout '2m'",
		Should:   "return 2 minutes duration",
		Actual:   d,
		Expected: 2 * time.Minute,
	})
}
