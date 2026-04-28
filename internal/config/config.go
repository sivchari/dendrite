// Package config handles parsing and validation of the dendrite YAML configuration file.
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

// namePattern matches the required "owner/repo@version" format.
var namePattern = regexp.MustCompile(`^([^/]+)/([^@]+)@(.+)$`)

// platformKeyPattern matches the required "os/arch" format for asset keys.
var platformKeyPattern = regexp.MustCompile(`^[a-z]+/[a-z0-9_]+$`)

// Config represents the top-level dendrite configuration.
type Config struct {
	Tools []Tool `yaml:"tools"`
}

// Platforms returns a deduplicated, sorted list of platform keys (e.g. "darwin/arm64")
// derived from the union of all tools' asset map keys.
func (c *Config) Platforms() []string {
	seen := make(map[string]struct{})

	for i := range c.Tools {
		for key := range c.Tools[i].Asset {
			seen[key] = struct{}{}
		}

		for key := range c.Tools[i].URL {
			seen[key] = struct{}{}
		}
	}

	platforms := make([]string, 0, len(seen))
	for key := range seen {
		platforms = append(platforms, key)
	}

	sort.Strings(platforms)

	return platforms
}

// Tool represents a single tool entry parsed from the configuration.
type Tool struct {
	// Owner is the repository owner (e.g. "cli" from "cli/cli@v2.87.0").
	Owner string `yaml:"-"`
	// Repo is the repository name (e.g. "cli" from "cli/cli@v2.87.0").
	Repo string `yaml:"-"`
	// Version is the version string including any prefix (e.g. "v2.87.0").
	Version string `yaml:"-"`
	// Asset maps platform keys (e.g. "darwin/arm64", "linux/amd64") to asset filename
	// patterns with the {version} placeholder. Mutually exclusive with URL.
	Asset map[string]string `yaml:"asset"`
	// URL maps platform keys (e.g. "darwin/arm64") to direct download URL patterns
	// with placeholders. Use this for non-GitHub sources. Mutually exclusive with Asset.
	URL map[string]string `yaml:"url"`
	// Bins is the list of binary names. Defaults to [repo name] if not specified.
	Bins []string `yaml:"bins"`
	// VersionPrefix is the prefix to strip from version when expanding {version}.
	// Defaults to "v". Set to "" to keep version as-is, or "go" for golang/go, etc.
	VersionPrefix string `yaml:"version_prefix"`
	// BinMap maps desired bin name to the actual filename inside the archive.
	// e.g., {"ls-lint": "ls-lint-darwin-arm64"}.
	BinMap map[string]string `yaml:"bin_map"`
	// Format overrides the archive format detection from URL extension.
	// Valid values: "tar.gz", "tar.bz2", "tar.xz", "tar", "zip", "raw".
	// If empty, format is auto-detected from URL extension.
	Format string `yaml:"format"`
	// StripComponents removes leading directory components when extracting tar/zip archives.
	StripComponents int `yaml:"strip_components"`

	// name is the raw "owner/repo@version" string from the YAML.
	name string
	// versionPrefixSet tracks whether the user explicitly set version_prefix.
	versionPrefixSet bool
}

// rawTool is used for YAML unmarshalling before validation.
type rawTool struct {
	Name            string            `yaml:"name"`
	Asset           map[string]string `yaml:"asset"`
	URL             map[string]string `yaml:"url"`
	Bins            []string          `yaml:"bins"`
	VersionPrefix   *string           `yaml:"version_prefix"`
	BinMap          map[string]string `yaml:"bin_map"`
	Format          string            `yaml:"format"`
	StripComponents int               `yaml:"strip_components"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for Tool.
func (t *Tool) UnmarshalYAML(value *yaml.Node) error {
	var raw rawTool
	if err := value.Decode(&raw); err != nil {
		return fmt.Errorf("failed to decode tool entry: %w", err)
	}

	t.name = raw.Name
	t.Asset = raw.Asset
	t.URL = raw.URL
	t.Bins = raw.Bins
	t.BinMap = raw.BinMap
	t.Format = raw.Format
	t.StripComponents = raw.StripComponents

	if raw.VersionPrefix != nil {
		t.VersionPrefix = *raw.VersionPrefix
		t.versionPrefixSet = true
	}

	return nil
}

// Parse reads the YAML file at the given path and returns a validated Config.
func Parse(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is from user-provided flag input
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	return ParseBytes(data)
}

// ParseBytes parses YAML data and returns a validated Config.
func ParseBytes(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validate checks all tools in the config and resolves defaults.
func validate(cfg *Config) error {
	if len(cfg.Tools) == 0 {
		return errors.New("config: tools list must not be empty")
	}

	var errs []error

	for i := range cfg.Tools {
		if err := validateTool(&cfg.Tools[i], i); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// validateTool validates a single tool entry and parses the name field.
func validateTool(t *Tool, index int) error {
	var errs []error

	errs = appendNameErrors(errs, t, index)

	hasAsset := len(t.Asset) > 0
	hasURL := len(t.URL) > 0

	if !hasAsset && !hasURL {
		errs = append(errs, fmt.Errorf("tools[%d]: asset or url is required", index))
	}

	if hasAsset && hasURL {
		errs = append(errs, fmt.Errorf("tools[%d]: asset and url are mutually exclusive", index))
	}

	// Validate that asset keys match the "os/arch" pattern.
	for key := range t.Asset {
		if !platformKeyPattern.MatchString(key) {
			errs = append(errs, fmt.Errorf("tools[%d]: invalid asset key %q: must match os/arch pattern (e.g. darwin/arm64)", index, key))
		}
	}

	// Validate that url keys match the "os/arch" pattern.
	for key := range t.URL {
		if !platformKeyPattern.MatchString(key) {
			errs = append(errs, fmt.Errorf("tools[%d]: invalid url key %q: must match os/arch pattern (e.g. darwin/arm64)", index, key))
		}
	}

	// Default version_prefix to "v" if not explicitly set.
	if !t.versionPrefixSet {
		t.VersionPrefix = "v"
	}

	// Default bins to [repo name] if not specified.
	if len(t.Bins) == 0 && t.Repo != "" {
		t.Bins = []string{t.Repo}
	}

	return errors.Join(errs...)
}

// appendNameErrors validates the tool name and parses owner/repo/version.
func appendNameErrors(errs []error, t *Tool, index int) []error {
	if t.name == "" {
		return append(errs, fmt.Errorf("tools[%d]: name is required", index))
	}

	matches := namePattern.FindStringSubmatch(t.name)
	if matches == nil {
		return append(errs, fmt.Errorf("tools[%d]: name %q must match owner/repo@version pattern", index, t.name))
	}

	t.Owner = matches[1]
	t.Repo = matches[2]
	t.Version = matches[3]

	return errs
}
