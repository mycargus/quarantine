package main

import (
	"regexp"
)

// retryTimesConfigPattern matches retryTimes: <non-zero digit> (config-file style).
var retryTimesConfigPattern = regexp.MustCompile(`retryTimes\s*:\s*[1-9]`)

// retryTimesCallPattern matches retryTimes(<non-zero digit>) (call-style in test files).
var retryTimesCallPattern = regexp.MustCompile(`retryTimes\(\s*[1-9]`)

// detectRetryTimes scans a map of filename → file content and returns the file
// paths where a non-zero retryTimes value was found.
//
// Pure function — no I/O.
func detectRetryTimes(files map[string]string) []string {
	var hits []string
	for path, content := range files {
		if retryTimesConfigPattern.MatchString(content) || retryTimesCallPattern.MatchString(content) {
			hits = append(hits, path)
		}
	}
	return hits
}
