package runner_test

import (
	"testing"

	"github.com/mycargus/quarantine/internal/runner"
	riteway "github.com/mycargus/riteway-golang"
)

// --- MatchesExcludePattern pure function unit tests ---

func TestMatchesExcludePatternNoPatterns(t *testing.T) {
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no exclude patterns",
		Should:   "return false (test is not excluded)",
		Actual:   runner.MatchesExcludePattern("src/unit.test.js::UnitService::should compute", nil),
		Expected: false,
	})
}

func TestMatchesExcludePatternEmptyPatterns(t *testing.T) {
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "empty exclude patterns slice",
		Should:   "return false (test is not excluded)",
		Actual:   runner.MatchesExcludePattern("src/unit.test.js::UnitService::should compute", []string{}),
		Expected: false,
	})
}

func TestMatchesExcludePatternFilePathGlobMatches(t *testing.T) {
	// test_id format: file_path::classname::name
	testID := "test/integration/api_test.js::ApiTest::should connect"

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "test_id whose file path matches 'test/integration/**'",
		Should:   "return true (test is excluded)",
		Actual:   runner.MatchesExcludePattern(testID, []string{"test/integration/**"}),
		Expected: true,
	})
}

func TestMatchesExcludePatternFilePathGlobNoMatch(t *testing.T) {
	testID := "src/unit.test.js::UnitService::should compute"

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "test_id that does not match 'test/integration/**'",
		Should:   "return false (test is not excluded)",
		Actual:   runner.MatchesExcludePattern(testID, []string{"test/integration/**"}),
		Expected: false,
	})
}

func TestMatchesExcludePatternClassnameGlobMatches(t *testing.T) {
	// Pattern **::SlowServiceTest::* should match any test with classname SlowServiceTest.
	testID := "src/slow.test.js::SlowServiceTest::should timeout"

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "test_id with classname 'SlowServiceTest' matching '**::SlowServiceTest::*'",
		Should:   "return true (test is excluded)",
		Actual:   runner.MatchesExcludePattern(testID, []string{"**::SlowServiceTest::*"}),
		Expected: true,
	})
}

func TestMatchesExcludePatternClassnameGlobNoMatch(t *testing.T) {
	testID := "src/fast.test.js::FastService::should run quickly"

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "test_id with classname 'FastService' not matching '**::SlowServiceTest::*'",
		Should:   "return false (test is not excluded)",
		Actual:   runner.MatchesExcludePattern(testID, []string{"**::SlowServiceTest::*"}),
		Expected: false,
	})
}

func TestMatchesExcludePatternFirstPatternMatches(t *testing.T) {
	testID := "test/integration/api_test.js::ApiTest::should connect"

	// First pattern matches.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "first pattern in list matches test_id",
		Should:   "return true",
		Actual:   runner.MatchesExcludePattern(testID, []string{"test/integration/**", "src/**"}),
		Expected: true,
	})
}

func TestMatchesExcludePatternSecondPatternMatches(t *testing.T) {
	testID := "src/slow.test.js::SlowServiceTest::should timeout"

	// Second pattern matches.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "second pattern in list matches test_id",
		Should:   "return true",
		Actual:   runner.MatchesExcludePattern(testID, []string{"test/integration/**", "**::SlowServiceTest::*"}),
		Expected: true,
	})
}

func TestMatchesExcludePatternNoPatternMatches(t *testing.T) {
	testID := "src/unit.test.js::UnitService::should compute"

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "no pattern in the list matches test_id",
		Should:   "return false",
		Actual:   runner.MatchesExcludePattern(testID, []string{"test/integration/**", "**::SlowServiceTest::*"}),
		Expected: false,
	})
}

func TestMatchesExcludePatternStarMatchesWithinSegment(t *testing.T) {
	// Single * should match within a path segment, not across separators.
	testID := "src/utils.test.js::UtilsService::should format"

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern 'src/*.test.js::*::*' matching test in src/",
		Should:   "return true",
		Actual:   runner.MatchesExcludePattern(testID, []string{"src/*.test.js::*::*"}),
		Expected: true,
	})
}

func TestMatchesExcludePatternDoubleStarMatchesAcrossSeparators(t *testing.T) {
	// ** should match test/integration/api_test.js which has nested path.
	testID := "test/integration/api_test.js::ApiTest::should connect"

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern 'test/**' matching deeply nested file path",
		Should:   "return true",
		Actual:   runner.MatchesExcludePattern(testID, []string{"test/**"}),
		Expected: true,
	})
}

// TestMatchesExcludePatternEmptyPatternMatchesEmptyString verifies that an
// empty pattern matches only the empty string (line 33: s == "").
// Kills mutation: `s == ""` → `s != ""`.
func TestMatchesExcludePatternEmptyPatternMatchesEmptyString(t *testing.T) {
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "empty pattern and empty test ID",
		Should:   "match (empty pattern == empty string)",
		Actual:   runner.MatchesExcludePattern("", []string{""}),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "empty pattern and non-empty test ID",
		Should:   "not match (empty pattern only matches empty string)",
		Actual:   runner.MatchesExcludePattern("foo::bar::baz", []string{""}),
		Expected: false,
	})
}

// TestMatchesExcludePatternStarDoesNotMatchColonSeparator verifies that *
// stops matching at ':' (colon separator used in test IDs).
// Kills mutations on line 62: removing ':' from `ch == '/' || ch == ':'`.
func TestMatchesExcludePatternStarDoesNotMatchColonSeparator(t *testing.T) {
	// Pattern "foo*baz" should NOT match "foo::baz" because * cannot cross '::'.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern 'foo*baz' and test ID 'foo::baz'",
		Should:   "not match (* cannot cross :: separator)",
		Actual:   runner.MatchesExcludePattern("foo::baz", []string{"foo*baz"}),
		Expected: false,
	})
}

// TestMatchesExcludePatternDoubleStarCanMatchColonSeparator verifies that **
// CAN cross :: (unlike *).
func TestMatchesExcludePatternDoubleStarCanMatchColonSeparator(t *testing.T) {
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern 'foo**baz' and test ID 'foo::baz'",
		Should:   "match (** can cross :: separator)",
		Actual:   runner.MatchesExcludePattern("foo::baz", []string{"foo**baz"}),
		Expected: true,
	})
}
