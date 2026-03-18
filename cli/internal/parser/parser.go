// Package parser handles JUnit XML parsing for Jest, RSpec, and Vitest output.
package parser

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
)

// TestSuites represents the top-level <testsuites> element in JUnit XML.
type TestSuites struct {
	XMLName    xml.Name    `xml:"testsuites"`
	Name       string      `xml:"name,attr,omitempty"`
	Tests      int         `xml:"tests,attr,omitempty"`
	Failures   int         `xml:"failures,attr,omitempty"`
	Errors     int         `xml:"errors,attr,omitempty"`
	Time       float64     `xml:"time,attr,omitempty"`
	TestSuites []TestSuite `xml:"testsuite"`
}

// TestSuite represents a <testsuite> element.
type TestSuite struct {
	XMLName   xml.Name   `xml:"testsuite"`
	Name      string     `xml:"name,attr"`
	Tests     int        `xml:"tests,attr,omitempty"`
	Failures  int        `xml:"failures,attr,omitempty"`
	Errors    int        `xml:"errors,attr,omitempty"`
	Skipped   int        `xml:"skipped,attr,omitempty"`
	Timestamp string     `xml:"timestamp,attr,omitempty"`
	Time      float64    `xml:"time,attr,omitempty"`
	TestCases []TestCase `xml:"testcase"`
}

// TestCase represents a <testcase> element.
type TestCase struct {
	XMLName   xml.Name `xml:"testcase"`
	Classname string   `xml:"classname,attr"`
	Name      string   `xml:"name,attr"`
	File      string   `xml:"file,attr,omitempty"`
	Time      float64  `xml:"time,attr,omitempty"`
	Failure   *Failure `xml:"failure,omitempty"`
	Error     *Error   `xml:"error,omitempty"`
	Skipped   *Skipped `xml:"skipped,omitempty"`
}

// Failure represents a <failure> child element of <testcase>.
type Failure struct {
	Message string `xml:"message,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Body    string `xml:",chardata"`
}

// Error represents an <error> child element of <testcase>.
type Error struct {
	Message string `xml:"message,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Body    string `xml:",chardata"`
}

// Skipped represents a <skipped> child element of <testcase>.
type Skipped struct {
	Message string `xml:"message,attr,omitempty"`
}

// TestResult represents a parsed test case with a constructed test_id.
type TestResult struct {
	TestID         string  `json:"test_id"`
	FilePath       string  `json:"file_path"`
	Classname      string  `json:"classname"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	DurationMs     int     `json:"duration_ms"`
	FailureMessage *string `json:"failure_message"`
}

// Parse reads JUnit XML from a reader and returns parsed test results.
func Parse(r io.Reader) ([]TestResult, error) {
	var suites TestSuites
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&suites); err != nil {
		return nil, fmt.Errorf("failed to parse JUnit XML: %w", err)
	}

	var results []TestResult
	for _, suite := range suites.TestSuites {
		for _, tc := range suite.TestCases {
			result := toTestResult(tc, suite)
			results = append(results, result)
		}
	}

	return results, nil
}

// toTestResult converts a parsed TestCase into a TestResult with a
// constructed test_id.
func toTestResult(tc TestCase, suite TestSuite) TestResult {
	filePath := extractFilePath(tc, suite)
	testID := constructTestID(filePath, tc.Classname, tc.Name)
	status := determineStatus(tc)
	durationMs := int(math.Round(tc.Time * 1000))

	result := TestResult{
		TestID:     testID,
		FilePath:   filePath,
		Classname:  tc.Classname,
		Name:       tc.Name,
		Status:     status,
		DurationMs: durationMs,
	}

	if tc.Failure != nil {
		msg := tc.Failure.Message
		result.FailureMessage = &msg
	} else if tc.Error != nil {
		msg := tc.Error.Message
		result.FailureMessage = &msg
	}

	return result
}

// extractFilePath determines the file path from the test case or suite,
// handling framework-specific variations.
func extractFilePath(tc TestCase, suite TestSuite) string {
	// Jest with addFileAttribute=true, or RSpec: file attribute on testcase.
	if tc.File != "" {
		return tc.File
	}
	// Vitest: suite name is the file path. Also works as a fallback for
	// Jest without addFileAttribute.
	return suite.Name
}

// constructTestID builds the composite test_id as file_path::classname::name.
func constructTestID(filePath, classname, name string) string {
	return filePath + "::" + classname + "::" + name
}

// determineStatus returns the test status based on child elements.
func determineStatus(tc TestCase) string {
	if tc.Failure != nil {
		return "failed"
	}
	if tc.Error != nil {
		return "error"
	}
	if tc.Skipped != nil {
		return "skipped"
	}
	return "passed"
}
