// Package runner handles test command execution and exclusion flag construction.
package runner

import (
	"fmt"
	"strings"

	"github.com/mycargus/quarantine/internal/quarantine"
)

// BuildExclusionArgs builds framework-specific CLI flags to exclude quarantined
// tests from execution. This is a pure function — no I/O.
//
// Jest: uses --testNamePattern with a negative lookahead regex combining all
// quarantined test names. Special regex characters are escaped via EscapeJestPattern.
//
// Vitest: uses -t with a negative lookahead regex (same pattern as Jest).
//
// RSpec: returns nil — no pre-execution exclusion for individual tests in v1;
// post-execution filtering is used instead.
//
// Returns nil if entries is empty or nil.
func BuildExclusionArgs(fw Framework, entries []quarantine.Entry) []string {
	if len(entries) == 0 {
		return nil
	}

	switch fw {
	case Jest:
		return buildJestExclusionArgs(entries)
	case Vitest:
		return buildVitestExclusionArgs(entries)
	case RSpec:
		return nil
	default:
		return nil
	}
}

// buildJestExclusionArgs constructs --testNamePattern with a negative lookahead
// regex that skips all quarantined test names.
func buildJestExclusionArgs(entries []quarantine.Entry) []string {
	pattern := buildNegativeLookaheadPattern(entries)
	return []string{"--testNamePattern", pattern}
}

// buildVitestExclusionArgs constructs -t with a negative lookahead regex.
func buildVitestExclusionArgs(entries []quarantine.Entry) []string {
	pattern := buildNegativeLookaheadPattern(entries)
	return []string{"-t", pattern}
}

// buildNegativeLookaheadPattern builds a regex like ^(?!.*(name1|name2)).*$
// that matches any string NOT containing any of the given test names.
// Test names are escaped for regex safety using EscapeJestPattern.
func buildNegativeLookaheadPattern(entries []quarantine.Entry) string {
	escaped := make([]string, len(entries))
	for i, e := range entries {
		escaped[i] = EscapeJestPattern(e.Name)
	}
	return fmt.Sprintf("^(?!.*(%s)).*$", strings.Join(escaped, "|"))
}
