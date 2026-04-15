package main

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

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
