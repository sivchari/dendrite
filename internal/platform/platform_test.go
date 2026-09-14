package platform

import (
	"testing"
)

func TestResolve(t *testing.T) { //nolint:funlen // table-driven test
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		version string
		prefix  string
		want    string
	}{
		{
			name:    "basic version replacement",
			pattern: "gh_{version}_macOS_arm64.tar.gz",
			version: "v2.87.0",
			prefix:  "v",
			want:    "gh_2.87.0_macOS_arm64.tar.gz",
		},
		{
			name:    "no placeholders at all",
			pattern: "static-binary.tar.gz",
			version: "v1.0.0",
			prefix:  "v",
			want:    "static-binary.tar.gz",
		},
		{
			name:    "version already without v prefix",
			pattern: "tool-{version}.tar.gz",
			version: "1.2.3",
			prefix:  "v",
			want:    "tool-1.2.3.tar.gz",
		},
		{
			name:    "multiple version placeholders",
			pattern: "{version}/tool-{version}-darwin-arm64.tar.gz",
			version: "v3.0.0",
			prefix:  "v",
			want:    "3.0.0/tool-3.0.0-darwin-arm64.tar.gz",
		},
		{
			name:    "empty version string",
			pattern: "tool-{version}-darwin.tar.gz",
			version: "",
			prefix:  "v",
			want:    "tool--darwin.tar.gz",
		},
		{
			name:    "go prefix",
			pattern: "go{version}.darwin-arm64.tar.gz",
			version: "go1.26.0",
			prefix:  "go",
			want:    "go1.26.0.darwin-arm64.tar.gz",
		},
		{
			name:    "URL with version placeholder",
			pattern: "https://example.com/tool/{version}/tool-{version}-linux-amd64.tar.gz",
			version: "v1.5.0",
			prefix:  "v",
			want:    "https://example.com/tool/1.5.0/tool-1.5.0-linux-amd64.tar.gz",
		},
		{
			name:    "slash in tag uses last path segment",
			pattern: "haro_{version}_darwin_arm64.tar.gz",
			version: "haro-cli/v0.3.1",
			prefix:  "v",
			want:    "haro_0.3.1_darwin_arm64.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Resolve(tt.pattern, tt.version, tt.prefix)

			if got != tt.want {
				t.Errorf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizedVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		prefix  string
		want    string
	}{
		{"strips v prefix", "v2.87.0", "v", "2.87.0"},
		{"no prefix match unchanged", "14.1.0", "v", "14.1.0"},
		{"slash in tag", "haro-cli/v0.3.1", "v", "0.3.1"},
		{"slash in tag without prefix match", "haro-cli/0.3.1", "v", "0.3.1"},
		{"multiple slashes uses last segment", "team/haro-cli/v0.3.1", "v", "0.3.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SanitizedVersion(tt.version, tt.prefix)

			if got != tt.want {
				t.Errorf("SanitizedVersion(%q, %q) = %q, want %q", tt.version, tt.prefix, got, tt.want)
			}
		})
	}
}

func assertResolveCandidates(t *testing.T, got []string, first string, contains []string) {
	t.Helper()

	if len(got) == 0 {
		t.Fatal("ResolveCandidates() returned no candidates")
	}

	if got[0] != first {
		t.Errorf("ResolveCandidates()[0] = %q, want %q", got[0], first)
	}

	if len(contains) == 0 && len(got) != 1 {
		t.Fatalf("ResolveCandidates() length = %d, want 1\ngot: %#v", len(got), got)
	}

	for _, want := range contains {
		if !stringSliceContains(got, want) {
			t.Errorf("ResolveCandidates() does not contain %q\ngot: %#v", want, got)
		}
	}
}

func TestResolveCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pattern  string
		version  string
		prefix   string
		platform Platform
		first    string
		contains []string
	}{
		{
			name:     "darwin arm64 aliases",
			pattern:  "gh_{version}_{os}_{arch}.zip",
			version:  "v2.87.0",
			prefix:   "v",
			platform: DarwinARM64,
			first:    "gh_2.87.0_darwin_arm64.zip",
			contains: []string{
				"gh_2.87.0_darwin_arm64.tar.gz",
				"gh_2.87.0_macOS_arm64.zip",
				"gh_2.87.0_Darwin_aarch64.zip",
			},
		},
		{
			name:     "linux amd64 aliases",
			pattern:  "ripgrep-{version}-{arch}-{os}.tar.gz",
			version:  "14.1.0",
			prefix:   "v",
			platform: LinuxAMD64,
			first:    "ripgrep-14.1.0-amd64-linux.tar.gz",
			contains: []string{
				"ripgrep-14.1.0-amd64-linux.zip",
				"ripgrep-14.1.0-x86_64-linux.tar.gz",
				"ripgrep-14.1.0-x86_64-unknown-linux-musl.tar.gz",
			},
		},
		{
			name:     "literal pattern has no archive fallback",
			pattern:  "tool-{version}.zip",
			version:  "v1.0.0",
			prefix:   "v",
			platform: LinuxARM64,
			first:    "tool-1.0.0.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ResolveCandidates(tt.pattern, tt.version, tt.prefix, tt.platform)

			assertResolveCandidates(t, got, tt.first, tt.contains)
		})
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}

func TestNixSystem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		platformKey string
		want        string
	}{
		{"darwin_arm64", "darwin_arm64", "aarch64-darwin"},
		{"darwin_amd64", "darwin_amd64", "x86_64-darwin"},
		{"linux_arm64", "linux_arm64", "aarch64-linux"},
		{"linux_amd64", "linux_amd64", "x86_64-linux"},
		{"unknown key", "freebsd_amd64", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NixSystem(tt.platformKey)

			if got != tt.want {
				t.Errorf("NixSystem(%q) = %q, want %q", tt.platformKey, got, tt.want)
			}
		})
	}
}

func TestPlatformConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform Platform
		wantOS   string
		wantArch string
	}{
		{"DarwinARM64", DarwinARM64, "darwin", "arm64"},
		{"DarwinAMD64", DarwinAMD64, "darwin", "amd64"},
		{"LinuxARM64", LinuxARM64, "linux", "arm64"},
		{"LinuxAMD64", LinuxAMD64, "linux", "amd64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.platform.OS != tt.wantOS {
				t.Errorf("OS = %q, want %q", tt.platform.OS, tt.wantOS)
			}

			if tt.platform.Arch != tt.wantArch {
				t.Errorf("Arch = %q, want %q", tt.platform.Arch, tt.wantArch)
			}
		})
	}
}
