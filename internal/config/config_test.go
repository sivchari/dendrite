package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBytes(t *testing.T) { //nolint:funlen,gocognit,cyclop // table-driven test with many cases
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
		check     func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid config with all fields",
			input: `
tools:
  - name: cli/cli@v2.87.0
    asset: gh_{version}_{os}_{arch}.tar.gz
    bins:
      - gh
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.Tools) != 1 {
					t.Fatalf("expected 1 tool, got %d", len(cfg.Tools))
				}
				tool := cfg.Tools[0]
				assertEqual(t, "Owner", "cli", tool.Owner)
				assertEqual(t, "Repo", "cli", tool.Repo)
				assertEqual(t, "Version", "v2.87.0", tool.Version)
				assertEqual(t, "Asset", "gh_{version}_{os}_{arch}.tar.gz", tool.Asset)
				assertSliceEqual(t, "Bins", []string{"gh"}, tool.Bins)
			},
		},
		{
			name: "bins defaults to repo name",
			input: `
tools:
  - name: aquaproj/aqua@v2.39.0
    asset: aqua_{os}_{arch}.tar.gz
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.Tools) != 1 {
					t.Fatalf("expected 1 tool, got %d", len(cfg.Tools))
				}
				tool := cfg.Tools[0]
				assertEqual(t, "Owner", "aquaproj", tool.Owner)
				assertEqual(t, "Repo", "aqua", tool.Repo)
				assertEqual(t, "Version", "v2.39.0", tool.Version)
				assertSliceEqual(t, "Bins", []string{"aqua"}, tool.Bins)
			},
		},
		{
			name: "multiple tools",
			input: `
tools:
  - name: cli/cli@v2.87.0
    asset: gh_{version}_{os}_{arch}.tar.gz
    bins:
      - gh
  - name: BurntSushi/ripgrep@14.1.0
    asset: ripgrep-{version}-{arch}-{os}.tar.gz
    bins:
      - rg
  - name: aquaproj/aqua@v2.39.0
    asset: aqua_{os}_{arch}.tar.gz
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.Tools) != 3 {
					t.Fatalf("expected 3 tools, got %d", len(cfg.Tools))
				}
				assertEqual(t, "Tools[0].Owner", "cli", cfg.Tools[0].Owner)
				assertSliceEqual(t, "Tools[0].Bins", []string{"gh"}, cfg.Tools[0].Bins)
				assertEqual(t, "Tools[1].Owner", "BurntSushi", cfg.Tools[1].Owner)
				assertEqual(t, "Tools[1].Repo", "ripgrep", cfg.Tools[1].Repo)
				assertSliceEqual(t, "Tools[1].Bins", []string{"rg"}, cfg.Tools[1].Bins)
				assertEqual(t, "Tools[2].Owner", "aquaproj", cfg.Tools[2].Owner)
				assertSliceEqual(t, "Tools[2].Bins", []string{"aqua"}, cfg.Tools[2].Bins)
			},
		},
		{
			name: "multiple bins for a single tool",
			input: `
tools:
  - name: cli/cli@v2.87.0
    asset: gh_{version}_{os}_{arch}.tar.gz
    bins:
      - gh
      - gh-auth
      - gh-repo
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.Tools) != 1 {
					t.Fatalf("expected 1 tool, got %d", len(cfg.Tools))
				}
				tool := cfg.Tools[0]
				assertEqual(t, "Owner", "cli", tool.Owner)
				assertEqual(t, "Repo", "cli", tool.Repo)
				assertSliceEqual(t, "Bins", []string{"gh", "gh-auth", "gh-repo"}, tool.Bins)
			},
		},
		{
			name: "version without v prefix",
			input: `
tools:
  - name: BurntSushi/ripgrep@14.1.0
    asset: ripgrep-{version}-{arch}-{os}.tar.gz
    bins:
      - rg
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				tool := cfg.Tools[0]
				assertEqual(t, "Version", "14.1.0", tool.Version)
			},
		},
		{
			name: "empty tools list",
			input: `
tools: []
`,
			wantErr:   true,
			errSubstr: "tools list must not be empty",
		},
		{
			name:      "no tools key",
			input:     `{}`,
			wantErr:   true,
			errSubstr: "tools list must not be empty",
		},
		{
			name: "missing name",
			input: `
tools:
  - asset: foo_{os}.tar.gz
`,
			wantErr:   true,
			errSubstr: "name is required",
		},
		{
			name: "invalid name format - no version",
			input: `
tools:
  - name: cli/cli
    asset: gh_{os}.tar.gz
`,
			wantErr:   true,
			errSubstr: "must match owner/repo@version pattern",
		},
		{
			name: "invalid name format - no owner",
			input: `
tools:
  - name: cli@v1.0.0
    asset: gh_{os}.tar.gz
`,
			wantErr:   true,
			errSubstr: "must match owner/repo@version pattern",
		},
		{
			name: "invalid name format - empty string",
			input: `
tools:
  - name: ""
    asset: gh_{os}.tar.gz
`,
			wantErr:   true,
			errSubstr: "name is required",
		},
		{
			name: "missing asset",
			input: `
tools:
  - name: cli/cli@v2.87.0
`,
			wantErr:   true,
			errSubstr: "asset or url is required",
		},
		{
			name: "asset is whitespace only",
			input: `
tools:
  - name: cli/cli@v2.87.0
    asset: "   "
`,
			wantErr:   true,
			errSubstr: "asset or url is required",
		},
		{
			name: "multiple validation errors in one tool",
			input: `
tools:
  - bins:
      - gh
`,
			wantErr:   true,
			errSubstr: "name is required",
		},
		{
			name: "multiple tools with errors",
			input: `
tools:
  - name: bad-name
    asset: foo.tar.gz
  - name: cli/cli@v1.0.0
    asset: ""
`,
			wantErr:   true,
			errSubstr: "must match owner/repo@version pattern",
		},
		{
			name:      "invalid YAML syntax",
			input:     `tools: [[[invalid`,
			wantErr:   true,
			errSubstr: "failed to parse YAML",
		},
		{
			name: "name with nested path",
			input: `
tools:
  - name: hashicorp/terraform@v1.9.0
    asset: terraform_{version}_{os}_{arch}.zip
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				tool := cfg.Tools[0]
				assertEqual(t, "Owner", "hashicorp", tool.Owner)
				assertEqual(t, "Repo", "terraform", tool.Repo)
				assertEqual(t, "Version", "v1.9.0", tool.Version)
				assertSliceEqual(t, "Bins", []string{"terraform"}, tool.Bins)
			},
		},
		{
			name: "url instead of asset",
			input: `
tools:
  - name: golang/go@go1.26.0
    url: https://go.dev/dl/go{version}.darwin-arm64.tar.gz
    version_prefix: "go"
    bins:
      - go
      - gofmt
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				tool := cfg.Tools[0]
				assertEqual(t, "Owner", "golang", tool.Owner)
				assertEqual(t, "Repo", "go", tool.Repo)
				assertEqual(t, "Version", "go1.26.0", tool.Version)
				assertEqual(t, "URL", "https://go.dev/dl/go{version}.darwin-arm64.tar.gz", tool.URL)
				assertEqual(t, "Asset", "", tool.Asset)
				assertEqual(t, "VersionPrefix", "go", tool.VersionPrefix)
				assertSliceEqual(t, "Bins", []string{"go", "gofmt"}, tool.Bins)
			},
		},
		{
			name: "asset and url mutually exclusive",
			input: `
tools:
  - name: cli/cli@v2.87.0
    asset: gh_{version}_{os}_{arch}.tar.gz
    url: https://example.com/gh.tar.gz
`,
			wantErr:   true,
			errSubstr: "asset and url are mutually exclusive",
		},
		{
			name: "version_prefix go for golang style",
			input: `
tools:
  - name: golang/go@go1.26.0
    asset: go{version}.darwin-arm64.tar.gz
    version_prefix: "go"
    bins:
      - go
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				tool := cfg.Tools[0]
				assertEqual(t, "VersionPrefix", "go", tool.VersionPrefix)
				assertEqual(t, "Version", "go1.26.0", tool.Version)
			},
		},
		{
			name: "version_prefix empty string for no stripping",
			input: `
tools:
  - name: BurntSushi/ripgrep@14.1.0
    asset: ripgrep-{version}-{arch}-{os}.tar.gz
    version_prefix: ""
    bins:
      - rg
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				tool := cfg.Tools[0]
				assertEqual(t, "VersionPrefix", "", tool.VersionPrefix)
				assertEqual(t, "Version", "14.1.0", tool.Version)
			},
		},
		{
			name: "os_map and arch_map",
			input: `
tools:
  - name: masaushi/accessory@v0.4.0
    asset: accessory_{os}_{arch}.tar.gz
    os_map:
      darwin: Darwin
    arch_map:
      amd64: x86_64
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				tool := cfg.Tools[0]
				if len(tool.OSMap) != 1 {
					t.Fatalf("expected 1 os_map entry, got %d", len(tool.OSMap))
				}
				assertEqual(t, "OSMap[darwin]", "Darwin", tool.OSMap["darwin"])
				if len(tool.ArchMap) != 1 {
					t.Fatalf("expected 1 arch_map entry, got %d", len(tool.ArchMap))
				}
				assertEqual(t, "ArchMap[amd64]", "x86_64", tool.ArchMap["amd64"])
			},
		},
		{
			name: "no os_map or arch_map defaults to nil",
			input: `
tools:
  - name: cli/cli@v2.87.0
    asset: gh_{version}_{os}_{arch}.tar.gz
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				tool := cfg.Tools[0]
				if tool.OSMap != nil {
					t.Errorf("expected OSMap to be nil, got %v", tool.OSMap)
				}
				if tool.ArchMap != nil {
					t.Errorf("expected ArchMap to be nil, got %v", tool.ArchMap)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := ParseBytes([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if tt.errSubstr != "" && !containsSubstring(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestParse(t *testing.T) { //nolint:funlen // comprehensive file-based test
	t.Parallel()

	t.Run("valid file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "dendrite.yaml")
		content := `
tools:
  - name: cli/cli@v2.87.0
    asset: gh_{version}_{os}_{arch}.tar.gz
    bins:
      - gh
`

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := Parse(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(cfg.Tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(cfg.Tools))
		}

		assertEqual(t, "Owner", "cli", cfg.Tools[0].Owner)
	})

	t.Run("file not found", func(t *testing.T) {
		t.Parallel()

		_, err := Parse("/nonexistent/path/dendrite.yaml")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}

		if !containsSubstring(err.Error(), "failed to read config file") {
			t.Errorf("error %q does not mention file read failure", err.Error())
		}
	})

	t.Run("invalid YAML in file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "bad.yaml")

		if err := os.WriteFile(path, []byte(`tools: [[[`), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := Parse(path)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})

	t.Run("file with validation errors", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "invalid.yaml")
		content := `
tools:
  - name: badformat
    asset: foo.tar.gz
`

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := Parse(path)
		if err == nil {
			t.Fatal("expected validation error")
		}

		if !containsSubstring(err.Error(), "must match owner/repo@version pattern") {
			t.Errorf("error %q does not mention pattern mismatch", err.Error())
		}
	})
}

func TestNamePattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		valid bool
		owner string
		repo  string
		ver   string
	}{
		{"cli/cli@v2.87.0", true, "cli", "cli", "v2.87.0"},
		{"BurntSushi/ripgrep@14.1.0", true, "BurntSushi", "ripgrep", "14.1.0"},
		{"aquaproj/aqua@v2.39.0", true, "aquaproj", "aqua", "v2.39.0"},
		{"hashicorp/terraform@v1.9.0", true, "hashicorp", "terraform", "v1.9.0"},
		{"a/b@c", true, "a", "b", "c"},
		{"noversion/repo", false, "", "", ""},
		{"noowner@v1.0.0", false, "", "", ""},
		{"", false, "", "", ""},
		{"@v1.0.0", false, "", "", ""},
		{"/repo@v1.0.0", false, "", "", ""},
		{"owner/@v1.0.0", false, "", "", ""},
		{"owner/repo@", false, "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			matches := namePattern.FindStringSubmatch(tt.input)

			if tt.valid {
				if matches == nil {
					t.Fatalf("expected %q to match, but it did not", tt.input)
				}

				assertEqual(t, "owner", tt.owner, matches[1])
				assertEqual(t, "repo", tt.repo, matches[2])
				assertEqual(t, "version", tt.ver, matches[3])
			} else if matches != nil {
				t.Fatalf("expected %q not to match, but got %v", tt.input, matches)
			}
		})
	}
}

// assertEqual is a test helper for comparing strings.
func assertEqual(t *testing.T, field, want, got string) {
	t.Helper()

	if want != got {
		t.Errorf("%s: want %q, got %q", field, want, got)
	}
}

// assertSliceEqual is a test helper for comparing string slices.
func assertSliceEqual(t *testing.T, field string, want, got []string) {
	t.Helper()

	if len(want) != len(got) {
		t.Errorf("%s: want %v (len %d), got %v (len %d)", field, want, len(want), got, len(got))

		return
	}

	for i := range want {
		if want[i] != got[i] {
			t.Errorf("%s[%d]: want %q, got %q", field, i, want[i], got[i])
		}
	}
}

// containsSubstring checks whether s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
