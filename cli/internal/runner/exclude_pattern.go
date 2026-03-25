package runner

import "strings"

// MatchesExcludePattern returns true if testID matches any of the glob patterns.
// Patterns are matched against the full test_id string (format: file_path::classname::name).
// Glob syntax: * matches within a segment (no path separators), ** matches
// across separators (any characters including / and ::).
// This is a pure function — no I/O.
func MatchesExcludePattern(testID string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchGlob(pattern, testID) {
			return true
		}
	}
	return false
}

// matchGlob matches pattern against s using a simple glob implementation that
// supports * (within segment) and ** (across all separators).
// We treat the test_id as a flat string — separators are :: and /, both
// treated equally for ** matching. * matches any character except / and :.
func matchGlob(pattern, s string) bool {
	return globMatch(pattern, s)
}

// globMatch implements ** and * glob matching against a string.
// ** matches any sequence of characters (including separators).
// * matches any sequence of characters except '/' and ':'.
func globMatch(pattern, s string) bool {
	// Base cases.
	if pattern == "" {
		return s == ""
	}
	if pattern == "**" {
		return true
	}

	// Handle ** at the start.
	if strings.HasPrefix(pattern, "**") {
		rest := pattern[2:]
		// ** followed by nothing — match anything.
		if rest == "" {
			return true
		}
		// ** followed by a separator then more pattern.
		// Try matching the rest against every suffix of s.
		for i := 0; i <= len(s); i++ {
			if globMatch(rest, s[i:]) {
				return true
			}
		}
		return false
	}

	// Handle * (not **) — matches any char except '/' and ':'.
	if strings.HasPrefix(pattern, "*") {
		rest := pattern[1:]
		for i := 0; i <= len(s); i++ {
			if i > 0 {
				ch := s[i-1]
				if ch == '/' || ch == ':' {
					break
				}
			}
			if globMatch(rest, s[i:]) {
				return true
			}
		}
		return false
	}

	// Literal character match.
	if len(s) == 0 {
		return false
	}
	if pattern[0] == s[0] {
		return globMatch(pattern[1:], s[1:])
	}
	return false
}
