// Package config handles parsing and validation of quarantine.yml.
package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the full quarantine.yml configuration.
type Config struct {
	Version       int            `yaml:"version"`
	Framework     string         `yaml:"framework"`
	Retries       int            `yaml:"retries,omitempty"`
	JUnitXML      string         `yaml:"junitxml,omitempty"`
	GitHub        GitHubConfig   `yaml:"github,omitempty"`
	IssueTracker  string         `yaml:"issue_tracker,omitempty"`
	Labels        []string       `yaml:"labels,omitempty"`
	Notifications Notifications  `yaml:"notifications,omitempty"`
	Storage       StorageConfig  `yaml:"storage,omitempty"`
	Exclude       []string       `yaml:"exclude,omitempty"`
	RerunCommand  string         `yaml:"rerun_command,omitempty"`

	// unknownTopLevel holds any top-level keys not in the known schema.
	// Populated by Parse; consumed by Validate to emit warnings.
	unknownTopLevel []string

	// unknownNotifications holds any keys under `notifications` not in the known schema.
	// Populated by Parse; consumed by Validate to emit errors.
	unknownNotifications []string

	// unknownStorage holds any keys under `storage` not in the known schema.
	// Populated by Parse; consumed by Validate to emit errors.
	unknownStorage []string
}

// GitHubConfig holds repository identification settings.
type GitHubConfig struct {
	Owner string `yaml:"owner,omitempty"`
	Repo  string `yaml:"repo,omitempty"`
}

// Notifications controls how the CLI notifies developers.
type Notifications struct {
	GitHubPRComment *bool `yaml:"github_pr_comment,omitempty"`
}

// StorageConfig controls where quarantine state is stored.
type StorageConfig struct {
	Branch string `yaml:"branch,omitempty"`
}

// validFrameworks lists the v1-supported frameworks.
var validFrameworks = map[string]bool{
	"jest":   true,
	"rspec":  true,
	"vitest": true,
}

// knownTopLevelKeys is the full set of keys defined in the v1 schema.
var knownTopLevelKeys = map[string]bool{
	"version":       true,
	"framework":     true,
	"retries":       true,
	"junitxml":      true,
	"github":        true,
	"issue_tracker": true,
	"labels":        true,
	"notifications": true,
	"storage":       true,
	"exclude":       true,
	"rerun_command": true,
}

// knownNotificationKeys is the full set of keys under `notifications` in the v1 schema.
var knownNotificationKeys = map[string]bool{
	"github_pr_comment": true,
}

// knownStorageKeys is the full set of keys under `storage` in the v1 schema.
var knownStorageKeys = map[string]bool{
	"branch": true,
}

// frameworkDefaults maps frameworks to their default junitxml glob patterns.
var frameworkDefaults = map[string]string{
	"jest":   "junit.xml",
	"rspec":  "rspec.xml",
	"vitest": "junit-report.xml",
}

// Load reads and parses a quarantine.yml file from the given path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open config file: %w", err)
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads and parses quarantine.yml from a reader.
//
// It decodes the document twice: once into a yaml.Node tree (to inspect raw
// keys) and once into the Config struct. Unknown keys at the top level,
// under `notifications`, and under `storage` are recorded on the returned
// Config and surfaced as errors or warnings by Validate.
func Parse(r io.Reader) (*Config, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read quarantine.yml: %w", err)
	}

	// First pass: decode into the Config struct.
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse quarantine.yml: %w", err)
	}

	// Second pass: decode into the yaml.Node tree to detect unknown keys.
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		// Structural errors were already caught above; skip unknown-key detection.
		return &cfg, nil
	}

	// root is a document node; the actual mapping is its first child.
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return &cfg, nil
	}

	topMapping := root.Content[0]
	if topMapping.Kind != yaml.MappingNode {
		return &cfg, nil
	}

	// MappingNode Content is interleaved key/value pairs: [k0, v0, k1, v1, ...].
	for i := 0; i+1 < len(topMapping.Content); i += 2 {
		key := topMapping.Content[i].Value
		valNode := topMapping.Content[i+1]

		if !knownTopLevelKeys[key] {
			cfg.unknownTopLevel = append(cfg.unknownTopLevel, key)
			continue
		}

		// Inspect nested sections for unknown keys.
		switch key {
		case "notifications":
			cfg.unknownNotifications = collectUnknownKeys(valNode, knownNotificationKeys)
		case "storage":
			cfg.unknownStorage = collectUnknownKeys(valNode, knownStorageKeys)
		}
	}

	return &cfg, nil
}

// collectUnknownKeys returns any mapping keys in node that are not in known.
func collectUnknownKeys(node *yaml.Node, known map[string]bool) []string {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	var unknown []string
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	return unknown
}

// ApplyDefaults fills in default values for optional fields.
func (c *Config) ApplyDefaults() {
	if c.Retries == 0 {
		c.Retries = 3
	}
	if c.JUnitXML == "" {
		if def, ok := frameworkDefaults[c.Framework]; ok {
			c.JUnitXML = def
		}
	}
	if c.IssueTracker == "" {
		c.IssueTracker = "github"
	}
	if len(c.Labels) == 0 {
		c.Labels = []string{"quarantine"}
	}
	if c.Notifications.GitHubPRComment == nil {
		t := true
		c.Notifications.GitHubPRComment = &t
	}
	if c.Storage.Branch == "" {
		c.Storage.Branch = "quarantine/state"
	}
}

// Validate checks the configuration against all schema rules. Returns a
// slice of errors and a slice of warnings.
func (c *Config) Validate() (errs []string, warns []string) {
	// version
	if c.Version == 0 {
		errs = append(errs, "Missing required field 'version' in quarantine.yml.")
	} else if c.Version != 1 {
		errs = append(errs, fmt.Sprintf(
			"Unsupported config version: %d. This version of the CLI supports version 1.", c.Version))
	}

	// framework
	if c.Framework == "" {
		errs = append(errs, "Missing required field 'framework' in quarantine.yml.")
	} else if !validFrameworks[c.Framework] {
		errs = append(errs, fmt.Sprintf(
			"Unknown framework '%s'. Supported frameworks: rspec, jest, vitest.", c.Framework))
	}

	// retries
	if c.Retries != 0 && (c.Retries < 1 || c.Retries > 10) {
		errs = append(errs, fmt.Sprintf(
			"Invalid retries value: %d. Must be between 1 and 10.", c.Retries))
	}

	// issue_tracker
	if c.IssueTracker != "" && c.IssueTracker != "github" {
		errs = append(errs, fmt.Sprintf(
			"Unsupported issue_tracker '%s'. This version supports: github. "+
				"Jira support is planned for a future release.", c.IssueTracker))
	}

	// labels
	if len(c.Labels) > 0 {
		if len(c.Labels) != 1 || c.Labels[0] != "quarantine" {
			errs = append(errs, "Custom labels are not supported in this version. Only ['quarantine'] is accepted.")
		}
	}

	// unknown top-level keys → warnings
	for _, key := range c.unknownTopLevel {
		warns = append(warns, fmt.Sprintf(
			"Unknown field '%s' in quarantine.yml will be ignored.", key))
	}

	// unknown notifications keys → errors
	for _, key := range c.unknownNotifications {
		errs = append(errs, fmt.Sprintf(
			"Unknown notification channel '%s'. This version supports: github_pr_comment. "+
				"Slack and email notifications are planned for a future release.", key))
	}

	// unknown storage keys → errors
	for _, key := range c.unknownStorage {
		errs = append(errs, fmt.Sprintf(
			"Unknown storage field '%s'. This version supports: branch.", key))
	}

	return errs, warns
}
