package runner_test

import (
	"testing"

	"github.com/mycargus/quarantine/cli/internal/runner"
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

// TestMatchesExcludePatternDoubleStarAtEndMatchesRemainingSegments verifies
// that a pattern ending with ** matches all remaining characters after the
// literal prefix — including nested path segments.
// Kills mutation on line 44: `return true` → `return false`.
func TestMatchesExcludePatternDoubleStarAtEndMatchesRemainingSegments(t *testing.T) {
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern 'com/example/**' and test ID 'com/example/Foo/Bar'",
		Should:   "match (** at end matches all remaining path segments)",
		Actual:   runner.MatchesExcludePattern("com/example/Foo/Bar", []string{"com/example/**"}),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern 'com/example/**' and test ID with deeply nested path and classname",
		Should:   "match (** at end matches remaining path and test_id segments)",
		Actual:   runner.MatchesExcludePattern(
			"com/example/sub/pkg::SomeClass::someMethod",
			[]string{"com/example/**"},
		),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern 'other/**' and test ID 'com/example/Foo/Bar'",
		Should:   "not match (literal prefix differs)",
		Actual:   runner.MatchesExcludePattern("com/example/Foo/Bar", []string{"other/**"}),
		Expected: false,
	})
}

// TestMatchesExcludePatternDoubleStarConsumesFull verifies that ** in the
// middle of a pattern can consume ALL remaining characters so that the rest
// of the pattern matches the empty suffix. This exercises the i == len(s)
// iteration of the loop on line 48.
// Kills mutation on line 48: `i <= len(s)` → `i < len(s)`.
func TestMatchesExcludePatternDoubleStarConsumesFull(t *testing.T) {
	// Pattern "**::SlowTest" against "SlowTest": the ** handler tries every
	// prefix of "SlowTest". At i=len("SlowTest"), s[i:] is "" and
	// globMatch("::SlowTest", "") returns false. The match is found at i=0
	// where globMatch("::SlowTest", "SlowTest") also fails (no leading ::).
	// Use a case where ** must consume the WHOLE string so that the recursive
	// call sees an empty remainder.
	//
	// Pattern "**/SlowTest" against "/SlowTest": rest = "/SlowTest".
	// i=0: globMatch("/SlowTest", "/SlowTest") → true.
	// This is not the i=len(s) case.
	//
	// To exercise i=len(s), we need a case where only globMatch(rest, "") is
	// true. Since * matches zero characters, pattern "***" uses ** handler
	// (rest="*"). At i=len(s), globMatch("*","") returns true (empty match).
	// Without i<=len(s), that iteration is skipped, but i=0 already tries
	// globMatch("*", s) which for non-empty s also succeeds via the * handler
	// consuming nothing. So we need s to contain separators that block *.
	//
	// Pattern "***" against "a/b": ** handler, rest="*".
	// i=0: globMatch("*","a/b") → * stops at /, only matches "" prefix, then
	//   globMatch("","a/b") → false. No match at i=0.
	// i=1: globMatch("*",""/b") → * stops at /, tries "", then
	//   globMatch("","/b") → false.
	// i=2: globMatch("*","/b") → * hits / immediately, only i=0 tried,
	//   globMatch("","/b") → false.
	// i=3: globMatch("*","b") → * matches "b", globMatch("","") → true. Match!
	// i=4=len("a/b"): globMatch("*","") → * matches "", globMatch("","")→true.
	// So i=3 finds the match before i=len(s). Need s with trailing separator.
	//
	// Pattern "***" against "a/": ** handler, rest="*".
	// i=0: globMatch("*","a/") → * tries "","a" then ch='/' breaks; neither
	//   leaves empty remainder for globMatch("",suffix). False.
	// i=1: globMatch("*","/") → * hits '/' at i=0 (i>0 check), waits—actually
	//   i=0 is tried first: globMatch("","/") → false. Then ch=s[0]='/', break.
	//   Returns false.
	// i=2: globMatch("*","") → true (i=2=len("a/")=2).
	// So i=2=len(s) is the ONLY winning iteration. With i<len(s) the loop
	// stops at i=1, misses i=2, and returns false.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern '***' and test ID 'a/' (trailing separator)",
		Should:   "match (** consumes 'a/', * matches empty remainder)",
		Actual:   runner.MatchesExcludePattern("a/", []string{"***"}),
		Expected: true,
	})

	// Confirm the inverse: a string without trailing separator is still
	// matched (via an earlier iteration), so the boundary is specific to
	// trailing-separator inputs.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern '***' and test ID 'ab' (no separator)",
		Should:   "match (**  can consume '' and * matches 'ab')",
		Actual:   runner.MatchesExcludePattern("ab", []string{"***"}),
		Expected: true,
	})
}

// TestMatchesExcludePatternSingleStarDoesNotCrossSlash verifies that a single
// * will not match across a '/' path separator. This directly targets the '/'
// branch of the `ch == '/' || ch == ':'` guard on line 62.
// Kills mutation on line 62: removing the '/' check, leaving only ch == ':'.
func TestMatchesExcludePatternSingleStarDoesNotCrossSlash(t *testing.T) {
	// foo/*/baz should NOT match foo/bar/qux/baz because * cannot cross /.
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern 'foo/*/baz' and test ID 'foo/bar/qux/baz'",
		Should:   "not match (* cannot cross / separator)",
		Actual:   runner.MatchesExcludePattern("foo/bar/qux/baz", []string{"foo/*/baz"}),
		Expected: false,
	})

	// Confirm the positive case: foo/*/baz DOES match foo/bar/baz (single segment).
	riteway.Assert(t, riteway.Case[bool]{
		Given:    "pattern 'foo/*/baz' and test ID 'foo/bar/baz'",
		Should:   "match (* matches single path segment 'bar')",
		Actual:   runner.MatchesExcludePattern("foo/bar/baz", []string{"foo/*/baz"}),
		Expected: true,
	})
}
