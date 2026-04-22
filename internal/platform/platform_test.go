package platform

import (
	"testing"
)

func TestResolve(t *testing.T) { //nolint:funlen // table-driven test with many cases
	t.Parallel()

	tests := []struct {
		name     string
		asset    string
		version  string
		platform Platform
		want     []string
	}{
		{
			name:     "darwin arm64 basic pattern",
			asset:    "gh_{version}_{os}_{arch}.tar.gz",
			version:  "v2.87.0",
			platform: DarwinARM64,
			want: []string{
				"gh_2.87.0_darwin_arm64.tar.gz",
				"gh_2.87.0_darwin_aarch64.tar.gz",
				"gh_2.87.0_macOS_arm64.tar.gz",
				"gh_2.87.0_macOS_aarch64.tar.gz",
			},
		},
		{
			name:     "linux amd64 basic pattern",
			asset:    "gh_{version}_{os}_{arch}.tar.gz",
			version:  "v2.87.0",
			platform: LinuxAMD64,
			want: []string{
				"gh_2.87.0_linux_amd64.tar.gz",
				"gh_2.87.0_linux_x86_64.tar.gz",
				"gh_2.87.0_Linux_amd64.tar.gz",
				"gh_2.87.0_Linux_x86_64.tar.gz",
			},
		},
		{
			name:     "darwin amd64 pattern",
			asset:    "tool-{version}-{os}-{arch}.zip",
			version:  "v1.0.0",
			platform: DarwinAMD64,
			want: []string{
				"tool-1.0.0-darwin-amd64.zip",
				"tool-1.0.0-darwin-x86_64.zip",
				"tool-1.0.0-macOS-amd64.zip",
				"tool-1.0.0-macOS-x86_64.zip",
			},
		},
		{
			name:     "linux arm64 pattern",
			asset:    "ripgrep-{version}-{arch}-{os}.tar.gz",
			version:  "14.1.0",
			platform: LinuxARM64,
			want: []string{
				"ripgrep-14.1.0-arm64-linux.tar.gz",
				"ripgrep-14.1.0-aarch64-linux.tar.gz",
				"ripgrep-14.1.0-arm64-Linux.tar.gz",
				"ripgrep-14.1.0-aarch64-Linux.tar.gz",
			},
		},
		{
			name:     "version without v prefix",
			asset:    "aqua_{os}_{arch}.tar.gz",
			version:  "v2.39.0",
			platform: DarwinARM64,
			want: []string{
				"aqua_darwin_arm64.tar.gz",
				"aqua_darwin_aarch64.tar.gz",
				"aqua_macOS_arm64.tar.gz",
				"aqua_macOS_aarch64.tar.gz",
			},
		},
		{
			name:     "no version placeholder",
			asset:    "tool_{os}_{arch}.tar.gz",
			version:  "v1.0.0",
			platform: DarwinARM64,
			want: []string{
				"tool_darwin_arm64.tar.gz",
				"tool_darwin_aarch64.tar.gz",
				"tool_macOS_arm64.tar.gz",
				"tool_macOS_aarch64.tar.gz",
			},
		},
		{
			name:     "no placeholders at all",
			asset:    "static-binary.tar.gz",
			version:  "v1.0.0",
			platform: LinuxAMD64,
			want: []string{
				"static-binary.tar.gz",
			},
		},
		{
			name:     "version already without v prefix",
			asset:    "tool-{version}.tar.gz",
			version:  "1.2.3",
			platform: LinuxAMD64,
			want: []string{
				"tool-1.2.3.tar.gz",
			},
		},
		{
			name:     "multiple version placeholders",
			asset:    "{version}/tool-{version}-{os}-{arch}.tar.gz",
			version:  "v3.0.0",
			platform: DarwinARM64,
			want: []string{
				"3.0.0/tool-3.0.0-darwin-arm64.tar.gz",
				"3.0.0/tool-3.0.0-darwin-aarch64.tar.gz",
				"3.0.0/tool-3.0.0-macOS-arm64.tar.gz",
				"3.0.0/tool-3.0.0-macOS-aarch64.tar.gz",
			},
		},
		{
			name:     "empty version string",
			asset:    "tool-{version}-{os}.tar.gz",
			version:  "",
			platform: DarwinARM64,
			want: []string{
				"tool--darwin.tar.gz",
				"tool--macOS.tar.gz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Resolve(tt.asset, tt.version, tt.platform)

			if len(got) != len(tt.want) {
				t.Fatalf("Resolve() returned %d results, want %d\n  got:  %v\n  want: %v",
					len(got), len(tt.want), got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Resolve()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDefaultMapping_knownPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		platform      Platform
		wantOSNames   []string
		wantArchNames []string
	}{
		{
			name:          "darwin arm64",
			platform:      DarwinARM64,
			wantOSNames:   []string{"darwin", "macOS"},
			wantArchNames: []string{"arm64", "aarch64"},
		},
		{
			name:          "darwin amd64",
			platform:      DarwinAMD64,
			wantOSNames:   []string{"darwin", "macOS"},
			wantArchNames: []string{"amd64", "x86_64"},
		},
		{
			name:          "linux arm64",
			platform:      LinuxARM64,
			wantOSNames:   []string{"linux", "Linux"},
			wantArchNames: []string{"arm64", "aarch64"},
		},
		{
			name:          "linux amd64",
			platform:      LinuxAMD64,
			wantOSNames:   []string{"linux", "Linux"},
			wantArchNames: []string{"amd64", "x86_64"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := DefaultMapping(tt.platform)
			assertStringSlice(t, "OSNames", m.OSNames, tt.wantOSNames)
			assertStringSlice(t, "ArchNames", m.ArchNames, tt.wantArchNames)
		})
	}
}

func TestDefaultMapping_unknownPlatform(t *testing.T) {
	t.Parallel()

	p := Platform{OS: "freebsd", Arch: "riscv64"}
	m := DefaultMapping(p)

	assertStringSlice(t, "OSNames", m.OSNames, []string{"freebsd"})
	assertStringSlice(t, "ArchNames", m.ArchNames, []string{"riscv64"})
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

func assertStringSlice(t *testing.T, label string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s: length = %d, want %d\n  got:  %v\n  want: %v",
			label, len(got), len(want), got, want)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %q, want %q", label, i, got[i], want[i])
		}
	}
}
