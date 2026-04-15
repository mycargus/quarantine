package detect

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DetectedFramework represents a test framework found in the project.
type DetectedFramework struct {
	Name   string
	Source string
}

// Result holds the frameworks detected by Scan.
type Result struct {
	Frameworks []DetectedFramework
}

// Names returns the framework names from the detection result.
// This is a pure function -- no I/O.
func (r Result) Names() []string {
	names := make([]string, len(r.Frameworks))
	for i, fw := range r.Frameworks {
		names[i] = fw.Name
	}
	return names
}

// Scan reads project files from dir and returns detected test frameworks.
// Detection is advisory only -- Scan never returns an error.
func Scan(dir string) Result {
	pkgJSON, _ := os.ReadFile(filepath.Join(dir, "package.json"))
	gemfile, _ := os.ReadFile(filepath.Join(dir, "Gemfile"))

	var frameworks []DetectedFramework
	frameworks = append(frameworks, ParsePackageJSON(string(pkgJSON))...)
	frameworks = append(frameworks, ParseGemfile(string(gemfile))...)

	return Result{Frameworks: frameworks}
}

// ParsePackageJSON detects jest and vitest in package.json content.
// Pure function -- no I/O.
func ParsePackageJSON(content string) []DetectedFramework {
	var pkg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return nil
	}

	var frameworks []DetectedFramework

	for _, section := range []string{"dependencies", "devDependencies"} {
		raw, ok := pkg[section]
		if !ok {
			continue
		}
		var deps map[string]json.RawMessage
		if err := json.Unmarshal(raw, &deps); err != nil {
			continue
		}
		source := "package.json " + section

		for _, name := range []string{"vitest", "jest"} {
			if _, found := deps[name]; found {
				frameworks = append(frameworks, DetectedFramework{Name: name, Source: source})
			}
		}
	}

	return frameworks
}

var rspecPattern = regexp.MustCompile(`gem\s+['"]rspec(-core)?['"]`)

// ParseGemfile detects rspec in Gemfile content.
// Pure function -- no I/O.
func ParseGemfile(content string) []DetectedFramework {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if rspecPattern.MatchString(line) {
			return []DetectedFramework{{Name: "rspec", Source: "Gemfile"}}
		}
	}

	return nil
}
