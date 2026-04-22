package lock

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sivchari/dendrite/internal/platform"
)

func TestReadWrite(t *testing.T) { //nolint:funlen // table-driven test with detailed verification
	t.Parallel()

	original := &File{
		Tools: []ToolLock{
			{
				Name: "cli/cli@v2.87.0",
				Assets: map[string]AssetLock{
					"darwin_arm64": {
						URL:    "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_macOS_arm64.tar.gz",
						SHA256: "sha256-abc123",
					},
					"linux_amd64": {
						URL:    "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_linux_amd64.tar.gz",
						SHA256: "sha256-def456",
					},
				},
			},
			{
				Name: "BurntSushi/ripgrep@14.1.0",
				Assets: map[string]AssetLock{
					"darwin_arm64": {
						URL:    "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-aarch64-apple-darwin.tar.gz",
						SHA256: "sha256-ghi789",
					},
				},
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "dendrite.lock.yaml")

	if err := Write(path, original); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify header comment is present.
	data, err := os.ReadFile(path) //nolint:gosec // test file path from t.TempDir
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	content := string(data)

	if got := content[:len(lockFileHeader)]; got != lockFileHeader {
		t.Errorf("header mismatch\ngot:  %q\nwant: %q", got, lockFileHeader)
	}

	// Read back and compare.
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(got.Tools) != len(original.Tools) {
		t.Fatalf("tools count mismatch: got %d, want %d", len(got.Tools), len(original.Tools))
	}

	for i, tool := range got.Tools {
		if tool.Name != original.Tools[i].Name {
			t.Errorf("tools[%d].Name = %q, want %q", i, tool.Name, original.Tools[i].Name)
		}

		for key, wantAsset := range original.Tools[i].Assets {
			gotAsset, ok := tool.Assets[key]
			if !ok {
				t.Errorf("tools[%d].Assets[%q] not found", i, key)

				continue
			}

			if gotAsset.URL != wantAsset.URL {
				t.Errorf("tools[%d].Assets[%q].URL = %q, want %q", i, key, gotAsset.URL, wantAsset.URL)
			}

			if gotAsset.SHA256 != wantAsset.SHA256 {
				t.Errorf("tools[%d].Assets[%q].SHA256 = %q, want %q", i, key, gotAsset.SHA256, wantAsset.SHA256)
			}
		}
	}
}

func TestReadNotFound(t *testing.T) {
	t.Parallel()

	_, err := Read(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("Read should return an error for nonexistent file")
	}
}

func TestLookup(t *testing.T) { //nolint:funlen // table-driven test with multiple cases
	t.Parallel()

	lock := &File{
		Tools: []ToolLock{
			{
				Name: "cli/cli@v2.87.0",
				Assets: map[string]AssetLock{
					"darwin_arm64": {
						URL:    "https://example.com/gh_darwin_arm64.tar.gz",
						SHA256: "sha256-aaa",
					},
					"linux_amd64": {
						URL:    "https://example.com/gh_linux_amd64.tar.gz",
						SHA256: "sha256-bbb",
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		toolName    string
		platformKey string
		wantNil     bool
		wantSHA256  string
	}{
		{
			name:        "existing tool and platform",
			toolName:    "cli/cli@v2.87.0",
			platformKey: "darwin_arm64",
			wantNil:     false,
			wantSHA256:  "sha256-aaa",
		},
		{
			name:        "existing tool, missing platform",
			toolName:    "cli/cli@v2.87.0",
			platformKey: "linux_arm64",
			wantNil:     true,
		},
		{
			name:        "missing tool",
			toolName:    "unknown/tool@v1.0.0",
			platformKey: "darwin_arm64",
			wantNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Lookup(lock, tt.toolName, tt.platformKey)

			if tt.wantNil {
				if got != nil {
					t.Errorf("Lookup(%q, %q) = %+v, want nil", tt.toolName, tt.platformKey, got)
				}

				return
			}

			if got == nil {
				t.Fatalf("Lookup(%q, %q) = nil, want non-nil", tt.toolName, tt.platformKey)
			}

			if got.SHA256 != tt.wantSHA256 {
				t.Errorf("SHA256 = %q, want %q", got.SHA256, tt.wantSHA256)
			}
		})
	}
}

func TestPlatformKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		p    platform.Platform
		want string
	}{
		{
			name: "darwin arm64",
			p:    platform.DarwinARM64,
			want: "darwin_arm64",
		},
		{
			name: "darwin amd64",
			p:    platform.DarwinAMD64,
			want: "darwin_amd64",
		},
		{
			name: "linux arm64",
			p:    platform.LinuxARM64,
			want: "linux_arm64",
		},
		{
			name: "linux amd64",
			p:    platform.LinuxAMD64,
			want: "linux_amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := PlatformKey(tt.p)

			if got != tt.want {
				t.Errorf("PlatformKey(%v) = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}

func TestPrefetchURL(t *testing.T) {
	t.Parallel()
	// PrefetchURL requires nix-prefetch-url to be installed.
	// Skip if not available.
	if _, err := exec.LookPath("nix-prefetch-url"); err != nil {
		t.Skip("nix-prefetch-url not found; skipping PrefetchURL test")
	}
	// If nix-prefetch-url is available, test with an empty / known URL
	// is impractical (network-dependent). Skip in unit tests.
	t.Skip("skipping PrefetchURL test: requires network access")
}
