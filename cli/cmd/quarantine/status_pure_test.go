package main

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"

	riteway "github.com/mycargus/riteway-golang"
)

// --- averageDurationMs ---

func TestAverageDurationMsReturnsNilForEmptySlice(t *testing.T) {
	result := averageDurationMs(nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "nil durations slice",
		Should:   "return nil",
		Actual:   result == nil,
		Expected: true,
	})
}

func TestAverageDurationMsReturnsAverageForNonEmpty(t *testing.T) {
	result := averageDurationMs([]int64{4000, 4200, 4400})

	if result == nil {
		t.Fatal("expected non-nil result for [4000, 4200, 4400]")
	}

	riteway.Assert(t, riteway.Case[int64]{
		Given:    "durations [4000, 4200, 4400]",
		Should:   "return average of 4200",
		Actual:   *result,
		Expected: int64(4200),
	})
}

func TestAverageDurationMsSingleElement(t *testing.T) {
	result := averageDurationMs([]int64{4200})

	if result == nil {
		t.Fatal("expected non-nil result for [4200]")
	}

	riteway.Assert(t, riteway.Case[int64]{
		Given:    "single duration of 4200ms",
		Should:   "return 4200",
		Actual:   *result,
		Expected: int64(4200),
	})
}

// --- formatDuration ---

func TestFormatDurationSecondsWithOneDecimal(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "4200 milliseconds",
		Should:   "return '4.2s'",
		Actual:   formatDuration(4200),
		Expected: "4.2s",
	})
}

func TestFormatDurationWholeSeconds(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "3000 milliseconds",
		Should:   "return '3.0s'",
		Actual:   formatDuration(3000),
		Expected: "3.0s",
	})
}

func TestFormatDurationSubSecond(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "500 milliseconds",
		Should:   "return '0.5s'",
		Actual:   formatDuration(500),
		Expected: "0.5s",
	})
}

// --- daysBetween (MISS-003) ---

func TestDaysBetweenSameTime(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "t equals now",
		Should:   "return 0",
		Actual:   daysBetween(now, now),
		Expected: 0,
	})
}

func TestDaysBetweenAlmostOneDay(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	almost := now.Add(-23 * time.Hour).Add(-59 * time.Minute)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "t is 23h59m before now",
		Should:   "return 0 (not yet a full day)",
		Actual:   daysBetween(almost, now),
		Expected: 0,
	})
}

func TestDaysBetweenExactlyOneDay(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "t is exactly 24h before now",
		Should:   "return 1",
		Actual:   daysBetween(now.Add(-24*time.Hour), now),
		Expected: 1,
	})
}

// --- computeStatusText ---

func TestComputeStatusTextWithDuration(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	issueNum42 := 42
	issueNum51 := 51
	issueNum63 := 63

	entries := []statusEntry{
		{
			TestID:        "spec/models/user_spec.rb::User::validates email",
			Name:          "validates email",
			QuarantinedAt: now.AddDate(0, 0, -45).Format(time.RFC3339),
			LastFailureAt: now.AddDate(0, 0, -2).Format(time.RFC3339),
			IssueNumber:   &issueNum42,
		},
		{
			TestID:        "spec/models/order_spec.rb::Order::calculates total",
			Name:          "calculates total",
			QuarantinedAt: now.AddDate(0, 0, -30).Format(time.RFC3339),
			LastFailureAt: now.AddDate(0, 0, -29).Format(time.RFC3339),
			IssueNumber:   &issueNum51,
		},
		{
			TestID:        "spec/services/payment_spec.rb::Payment::retries charge",
			Name:          "retries charge",
			QuarantinedAt: now.AddDate(0, 0, -3).Format(time.RFC3339),
			LastFailureAt: now.AddDate(0, 0, -3).Format(time.RFC3339),
			IssueNumber:   &issueNum63,
		},
	}

	avgMs := int64(4200)
	result := computeStatusText("backend", entries, &avgMs, now)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend suite with 3 entries and avg 4200ms",
		Should:   "contain 'Suite: backend'",
		Actual:   strings.Contains(result, "Suite: backend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend suite with 3 entries",
		Should:   "contain 'Quarantined tests: 3'",
		Actual:   strings.Contains(result, "Quarantined tests: 3"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "avg duration 4200ms",
		Should:   "contain 'Avg quarantined test duration: 4.2s'",
		Actual:   strings.Contains(result, "Avg quarantined test duration: 4.2s"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "3 tests at 4.2s each",
		Should:   "contain estimated CI time ~12.6s",
		Actual:   strings.Contains(result, "~12.6s"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "validates email quarantined 45 days ago",
		Should:   "contain '45 days'",
		Actual:   strings.Contains(result, "45 days"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "validates email last failed 2 days ago",
		Should:   "contain 'last failed 2 days ago'",
		Actual:   strings.Contains(result, "last failed 2 days ago"),
		Expected: true,
	})
}

func TestComputeStatusTextNilDuration(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	issueNum := 42
	entries := []statusEntry{
		{
			TestID:        "spec/models/user_spec.rb::User::validates email",
			Name:          "validates email",
			QuarantinedAt: now.AddDate(0, 0, -10).Format(time.RFC3339),
			LastFailureAt: now.AddDate(0, 0, -1).Format(time.RFC3339),
			IssueNumber:   &issueNum,
		},
	}

	result := computeStatusText("backend", entries, nil, now)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "nil avgDurationMs (no artifact data)",
		Should:   "not mention avg duration",
		Actual:   !strings.Contains(result, "Avg quarantined test duration"),
		Expected: true,
	})
}

// TestComputeStatusTextBadQuarantinedAt verifies the fallback when
// QuarantinedAt cannot be parsed. (MISS-001)
func TestComputeStatusTextBadQuarantinedAt(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	issueNum := 42
	entries := []statusEntry{
		{
			TestID:        "spec/models/user_spec.rb::User::validates email",
			Name:          "validates email",
			QuarantinedAt: "not-a-date",
			LastFailureAt: now.AddDate(0, 0, -1).Format(time.RFC3339),
			IssueNumber:   &issueNum,
		},
	}

	result := computeStatusText("backend", entries, nil, now)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "malformed QuarantinedAt timestamp",
		Should:   "still include the test entry (0 days as fallback)",
		Actual:   strings.Contains(result, "validates email") && strings.Contains(result, "0 days"),
		Expected: true,
	})
}

// TestComputeStatusTextBadLastFailureAt verifies the fallback when
// LastFailureAt cannot be parsed. (MISS-001)
func TestComputeStatusTextBadLastFailureAt(t *testing.T) {
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	issueNum := 42
	entries := []statusEntry{
		{
			TestID:        "spec/models/user_spec.rb::User::validates email",
			Name:          "validates email",
			QuarantinedAt: now.AddDate(0, 0, -5).Format(time.RFC3339),
			LastFailureAt: "not-a-date",
			IssueNumber:   &issueNum,
		},
	}

	result := computeStatusText("backend", entries, nil, now)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "malformed LastFailureAt timestamp",
		Should:   "still include the test entry with 'last failed 0 days ago' as fallback",
		Actual:   strings.Contains(result, "validates email") && strings.Contains(result, "last failed 0 days ago"),
		Expected: true,
	})
}

// --- parseResultsJSON (MISS-002) ---

func TestParseResultsJSONMalformedJSON(t *testing.T) {
	result := parseResultsJSON([]byte("not json"))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "malformed JSON bytes",
		Should:   "return nil",
		Actual:   result == nil,
		Expected: true,
	})
}

func TestParseResultsJSONFiltersToQuarantined(t *testing.T) {
	data := []byte(`{
		"tests": [
			{"status": "quarantined", "duration_ms": 4200},
			{"status": "passed", "duration_ms": 1000},
			{"status": "quarantined", "duration_ms": 3800},
			{"status": "failed", "duration_ms": 500}
		]
	}`)

	result := parseResultsJSON(data)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "4 tests with statuses quarantined, passed, quarantined, failed",
		Should:   "return only the 2 quarantined test durations",
		Actual:   len(result),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[int64]{
		Given:    "first quarantined test with duration_ms 4200",
		Should:   "return 4200 as first duration",
		Actual:   result[0],
		Expected: int64(4200),
	})

	riteway.Assert(t, riteway.Case[int64]{
		Given:    "second quarantined test with duration_ms 3800",
		Should:   "return 3800 as second duration",
		Actual:   result[1],
		Expected: int64(3800),
	})
}

func TestParseResultsJSONEmptyTests(t *testing.T) {
	data := []byte(`{"tests": []}`)

	result := parseResultsJSON(data)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "empty tests array",
		Should:   "return nil (no quarantined durations)",
		Actual:   result == nil,
		Expected: true,
	})
}

// --- extractResultsFromZip (MISS-002) ---

func TestExtractResultsFromZipInvalidZip(t *testing.T) {
	_, err := extractResultsFromZip([]byte("not a zip"))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "invalid ZIP bytes",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestExtractResultsFromZipMissingResultsJSON(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	_, _ = w.Create("other.json")
	_ = w.Close()

	_, err := extractResultsFromZip(buf.Bytes())

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "ZIP containing other.json but no results.json",
		Should:   "return an error",
		Actual:   err != nil,
		Expected: true,
	})
}

func TestExtractResultsFromZipHappyPath(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("results.json")
	_, _ = f.Write([]byte(`{"tests":[]}`))
	_ = w.Close()

	data, err := extractResultsFromZip(buf.Bytes())

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "valid ZIP with results.json",
		Should:   "return nil error",
		Actual:   err == nil,
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "valid ZIP with results.json containing '{\"tests\":[]}'",
		Should:   "return the JSON bytes",
		Actual:   strings.Contains(string(data), "tests"),
		Expected: true,
	})
}

// --- computeAllSuitesSummary ---

func TestComputeAllSuitesSummaryEmptySlice(t *testing.T) {
	result := computeAllSuitesSummary(nil)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "nil suiteCount slice (no suites configured)",
		Should:   "contain SUITE header",
		Actual:   strings.Contains(result, "SUITE"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "nil suiteCount slice (no suites configured)",
		Should:   "contain 'Total'",
		Actual:   strings.Contains(result, "Total"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "nil suiteCount slice (no suites configured)",
		Should:   "show total of 0",
		Actual:   strings.Contains(result, "0"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "nil suiteCount slice (no suites configured)",
		Should:   "contain the hint about suite-name",
		Actual:   strings.Contains(result, "quarantine status <suite-name>"),
		Expected: true,
	})
}

func TestComputeAllSuitesSummarySingleSuite(t *testing.T) {
	result := computeAllSuitesSummary([]suiteCount{{Name: "backend", QuarantinedCount: 3}})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "single suite 'backend' with 3 quarantined tests",
		Should:   "contain 'backend'",
		Actual:   strings.Contains(result, "backend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "single suite 'backend' with 3 quarantined tests",
		Should:   "contain '3'",
		Actual:   strings.Contains(result, "3"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "single suite with 3 quarantined tests",
		Should:   "contain 'Total'",
		Actual:   strings.Contains(result, "Total"),
		Expected: true,
	})
}

func TestComputeAllSuitesSummary(t *testing.T) {
	counts := []suiteCount{
		{Name: "backend", QuarantinedCount: 5},
		{Name: "frontend", QuarantinedCount: 2},
	}

	result := computeAllSuitesSummary(counts)

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend (5) and frontend (2) suite counts",
		Should:   "contain 'backend'",
		Actual:   strings.Contains(result, "backend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend suite with 5 quarantined tests",
		Should:   "contain '5'",
		Actual:   strings.Contains(result, "5"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend (5) and frontend (2) suite counts",
		Should:   "contain 'frontend'",
		Actual:   strings.Contains(result, "frontend"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "frontend suite with 2 quarantined tests",
		Should:   "contain '2'",
		Actual:   strings.Contains(result, "2"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend (5) and frontend (2) suite counts",
		Should:   "contain 'Total'",
		Actual:   strings.Contains(result, "Total"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "backend (5) and frontend (2) summing to 7",
		Should:   "contain '7'",
		Actual:   strings.Contains(result, "7"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "all-suites summary output",
		Should:   "contain hint about suite-name for details",
		Actual:   strings.Contains(result, "quarantine status <suite-name>"),
		Expected: true,
	})
}
