package detect_test

import (
	"os"
	"path/filepath"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/cli/internal/detect"
)

// --- Scan integration test (I/O shell) ---

func TestScanDetectsJestAndRSpec(t *testing.T) {
	dir := t.TempDir()

	pkgJSON := `{
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`
	gemfile := `source 'https://rubygems.org'

gem 'rspec'
gem 'rails'
`

	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Gemfile"), []byte(gemfile), 0644); err != nil {
		t.Fatalf("write Gemfile: %v", err)
	}

	result := detect.Scan(dir)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a directory with package.json (jest) and Gemfile (rspec)",
		Should:   "detect two frameworks",
		Actual:   len(result.Frameworks),
		Expected: 2,
	})

	t.Run("jest entry", func(t *testing.T) {
		if len(result.Frameworks) < 1 {
			t.Skip("no frameworks detected")
		}
		riteway.Assert(t, riteway.Case[detect.DetectedFramework]{
			Given:    "a directory with package.json (jest) and Gemfile (rspec)",
			Should:   "detect jest with source 'package.json devDependencies'",
			Actual:   result.Frameworks[0],
			Expected: detect.DetectedFramework{Name: "jest", Source: "package.json devDependencies"},
		})
	})

	t.Run("rspec entry", func(t *testing.T) {
		if len(result.Frameworks) < 2 {
			t.Skip("fewer than 2 frameworks detected")
		}
		riteway.Assert(t, riteway.Case[detect.DetectedFramework]{
			Given:    "a directory with package.json (jest) and Gemfile (rspec)",
			Should:   "detect rspec with source 'Gemfile'",
			Actual:   result.Frameworks[1],
			Expected: detect.DetectedFramework{Name: "rspec", Source: "Gemfile"},
		})
	})
}

// --- ParsePackageJSON pure function tests ---

func TestParsePackageJSONJestInDevDependencies(t *testing.T) {
	result := detect.ParsePackageJSON(`{"devDependencies":{"jest":"^29.0.0"}}`)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "package.json with jest in devDependencies",
		Should:   "detect one framework",
		Actual:   len(result),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[detect.DetectedFramework]{
		Given:    "package.json with jest in devDependencies",
		Should:   "return jest with correct source",
		Actual:   result[0],
		Expected: detect.DetectedFramework{Name: "jest", Source: "package.json devDependencies"},
	})
}

func TestParsePackageJSONVitestBeforeJest(t *testing.T) {
	content := `{"devDependencies":{"jest":"^29.0.0","vitest":"^1.0.0"}}`
	result := detect.ParsePackageJSON(content)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "package.json with both jest and vitest in devDependencies",
		Should:   "detect two frameworks",
		Actual:   len(result),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[detect.DetectedFramework]{
		Given:    "package.json with both jest and vitest in devDependencies",
		Should:   "return vitest first",
		Actual:   result[0],
		Expected: detect.DetectedFramework{Name: "vitest", Source: "package.json devDependencies"},
	})

	riteway.Assert(t, riteway.Case[detect.DetectedFramework]{
		Given:    "package.json with both jest and vitest in devDependencies",
		Should:   "return jest second",
		Actual:   result[1],
		Expected: detect.DetectedFramework{Name: "jest", Source: "package.json devDependencies"},
	})
}

// --- ParseGemfile pure function tests ---

func TestParseGemfileRSpec(t *testing.T) {
	result := detect.ParseGemfile("gem 'rspec', '~> 3.12'\n")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Gemfile with gem 'rspec'",
		Should:   "detect one framework",
		Actual:   len(result),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[detect.DetectedFramework]{
		Given:    "Gemfile with gem 'rspec'",
		Should:   "return rspec with source 'Gemfile'",
		Actual:   result[0],
		Expected: detect.DetectedFramework{Name: "rspec", Source: "Gemfile"},
	})
}

func TestParseGemfileRSpecCore(t *testing.T) {
	result := detect.ParseGemfile(`gem "rspec-core", "~> 3.12"` + "\n")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Gemfile with gem \"rspec-core\"",
		Should:   "detect one framework",
		Actual:   len(result),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[detect.DetectedFramework]{
		Given:    "Gemfile with gem \"rspec-core\"",
		Should:   "return rspec with source 'Gemfile'",
		Actual:   result[0],
		Expected: detect.DetectedFramework{Name: "rspec", Source: "Gemfile"},
	})
}

func TestParseGemfileCommentedGemIgnored(t *testing.T) {
	result := detect.ParseGemfile("# gem 'rspec'\n")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "Gemfile with only a commented-out rspec gem",
		Should:   "detect no frameworks",
		Actual:   len(result),
		Expected: 0,
	})
}

func TestParsePackageJSONMalformedJSONReturnsEmpty(t *testing.T) {
	result := detect.ParsePackageJSON("{not valid json}")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "malformed package.json content",
		Should:   "return no frameworks",
		Actual:   len(result),
		Expected: 0,
	})
}

func TestParsePackageJSONJestInDependencies(t *testing.T) {
	result := detect.ParsePackageJSON(`{"dependencies":{"jest":"^29.0.0"}}`)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "package.json with jest in dependencies (not devDependencies)",
		Should:   "detect one framework",
		Actual:   len(result),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "package.json with jest in dependencies",
		Should:   "report source as 'package.json dependencies'",
		Actual:   result[0].Source,
		Expected: "package.json dependencies",
	})
}

func TestParseGemfileEmptyInput(t *testing.T) {
	result := detect.ParseGemfile("")

	riteway.Assert(t, riteway.Case[int]{
		Given:    "empty Gemfile content",
		Should:   "return no frameworks",
		Actual:   len(result),
		Expected: 0,
	})
}

// --- Scan edge case: empty directory ---

func TestScanEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	result := detect.Scan(dir)

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a directory with no package.json or Gemfile",
		Should:   "return no frameworks",
		Actual:   len(result.Frameworks),
		Expected: 0,
	})
}
