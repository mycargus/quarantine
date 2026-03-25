package config_test

import (
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"

	"github.com/mycargus/quarantine/internal/config"
)

// errorContains reports whether err is non-nil and its message contains sub.
func errorContains(err error, sub string) bool {
	return err != nil && strings.Contains(err.Error(), sub)
}

// parseYAML is a test helper that calls Parse on a YAML string.
func parseYAML(t *testing.T, yaml string) *config.Config {
	t.Helper()
	cfg, err := config.Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	return cfg
}

// containsString reports whether s appears in slice.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// --- Parse ---

func TestParseValidMinimalConfig(t *testing.T) {
	yaml := `
version: 1
framework: jest
`
	cfg, err := config.Parse(strings.NewReader(yaml))

	riteway.Assert(t, riteway.Case[error]{
		Given:    "a minimal valid quarantine.yml",
		Should:   "parse without error",
		Actual:   err,
		Expected: nil,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "a minimal valid quarantine.yml",
		Should:   "decode version correctly",
		Actual:   cfg.Version,
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[string]{
		Given:    "a minimal valid quarantine.yml",
		Should:   "decode framework correctly",
		Actual:   cfg.Framework,
		Expected: "jest",
	})
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path/quarantine.yml")

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "a path that does not exist",
		Should:   "return an error wrapping 'could not open config file'",
		Actual:   errorContains(err, "could not open config file"),
		Expected: true,
	})
}

func TestParseInvalidYAML(t *testing.T) {
	yaml := `version: [unclosed`

	_, err := config.Parse(strings.NewReader(yaml))

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "malformed YAML",
		Should:   "return a non-nil error",
		Actual:   err != nil,
		Expected: true,
	})
}
