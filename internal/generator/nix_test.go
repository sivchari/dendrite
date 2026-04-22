package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sivchari/dendrite/internal/config"
)

func TestGenerate(t *testing.T) { //nolint:funlen // table-driven test with many cases
	t.Parallel()

	tests := []struct {
		name    string
		input   GenerateInput
		want    string
		wantErr bool
	}{
		{
			name: "basic tool with v prefix",
			input: GenerateInput{
				Tool: config.Tool{
					Owner:         "cli",
					Repo:          "cli",
					Version:       "v2.87.0",
					Asset:         "gh_{version}_{os}_{arch}.tar.gz",
					Bins:          []string{"gh"},
					VersionPrefix: "v",
				},
				Lock: LockEntry{
					URL:    "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_macOS_arm64.tar.gz",
					SHA256: "sha256-XXXX",
				},
			},
			want: `{
  stdenv,
  fetchurl,
}:
stdenv.mkDerivation rec {
  pname = "cli";
  version = "2.87.0";

  src = fetchurl {
    url = "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_macOS_arm64.tar.gz";
    sha256 = "sha256-XXXX";
  };

  dontUnpack = true;

  installPhase = ''
    mkdir -p $out/bin
    tar -xzf $src -C $out/bin
    chmod +x $out/bin/gh
  '';
}
`,
		},
		{
			name: "version without v prefix",
			input: GenerateInput{
				Tool: config.Tool{
					Owner:         "BurntSushi",
					Repo:          "ripgrep",
					Version:       "14.1.0",
					Asset:         "ripgrep-{version}-{arch}-{os}.tar.gz",
					Bins:          []string{"rg"},
					VersionPrefix: "v",
				},
				Lock: LockEntry{
					URL:    "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-arm64-linux.tar.gz",
					SHA256: "sha256-YYYY",
				},
			},
			want: `{
  stdenv,
  fetchurl,
}:
stdenv.mkDerivation rec {
  pname = "ripgrep";
  version = "14.1.0";

  src = fetchurl {
    url = "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-arm64-linux.tar.gz";
    sha256 = "sha256-YYYY";
  };

  dontUnpack = true;

  installPhase = ''
    mkdir -p $out/bin
    tar -xzf $src -C $out/bin
    chmod +x $out/bin/rg
  '';
}
`,
		},
		{
			name: "bins defaults to repo name",
			input: GenerateInput{
				Tool: config.Tool{
					Owner:         "aquaproj",
					Repo:          "aqua",
					Version:       "v2.39.0",
					Asset:         "aqua_{os}_{arch}.tar.gz",
					Bins:          []string{"aqua"},
					VersionPrefix: "v",
				},
				Lock: LockEntry{
					URL:    "https://github.com/aquaproj/aqua/releases/download/v2.39.0/aqua_darwin_arm64.tar.gz",
					SHA256: "sha256-0zVLnO4O5TPqyHdX6migzW2rUH/QQYAsCD4A2PYOzGM=",
				},
			},
			want: `{
  stdenv,
  fetchurl,
}:
stdenv.mkDerivation rec {
  pname = "aqua";
  version = "2.39.0";

  src = fetchurl {
    url = "https://github.com/aquaproj/aqua/releases/download/v2.39.0/aqua_darwin_arm64.tar.gz";
    sha256 = "sha256-0zVLnO4O5TPqyHdX6migzW2rUH/QQYAsCD4A2PYOzGM=";
  };

  dontUnpack = true;

  installPhase = ''
    mkdir -p $out/bin
    tar -xzf $src -C $out/bin
    chmod +x $out/bin/aqua
  '';
}
`,
		},
		{
			name: "bins falls back to repo when empty",
			input: GenerateInput{
				Tool: config.Tool{
					Owner:         "hashicorp",
					Repo:          "terraform",
					Version:       "v1.9.0",
					Asset:         "terraform_{version}_{os}_{arch}.zip",
					Bins:          nil,
					VersionPrefix: "v",
				},
				Lock: LockEntry{
					URL:    "https://github.com/hashicorp/terraform/releases/download/v1.9.0/terraform_1.9.0_darwin_arm64.zip",
					SHA256: "sha256-ZZZZ",
				},
			},
			want: `{
  stdenv,
  fetchurl,
  unzip,
}:
stdenv.mkDerivation rec {
  pname = "terraform";
  version = "1.9.0";

  src = fetchurl {
    url = "https://github.com/hashicorp/terraform/releases/download/v1.9.0/terraform_1.9.0_darwin_arm64.zip";
    sha256 = "sha256-ZZZZ";
  };

  dontUnpack = true;

  nativeBuildInputs = [ unzip ];

  installPhase = ''
    mkdir -p $out/bin
    unzip -o $src -d $out/bin
    chmod +x $out/bin/terraform
  '';
}
`,
		},
		{
			name: "sha256 with base64 encoding",
			input: GenerateInput{
				Tool: config.Tool{
					Owner:         "junegunn",
					Repo:          "fzf",
					Version:       "v0.55.0",
					Asset:         "fzf-{version}-{os}_{arch}.tar.gz",
					Bins:          []string{"fzf"},
					VersionPrefix: "v",
				},
				Lock: LockEntry{
					URL:    "https://github.com/junegunn/fzf/releases/download/v0.55.0/fzf-0.55.0-darwin_arm64.tar.gz",
					SHA256: "sha256-abc123def456==",
				},
			},
			want: `{
  stdenv,
  fetchurl,
}:
stdenv.mkDerivation rec {
  pname = "fzf";
  version = "0.55.0";

  src = fetchurl {
    url = "https://github.com/junegunn/fzf/releases/download/v0.55.0/fzf-0.55.0-darwin_arm64.tar.gz";
    sha256 = "sha256-abc123def456==";
  };

  dontUnpack = true;

  installPhase = ''
    mkdir -p $out/bin
    tar -xzf $src -C $out/bin
    chmod +x $out/bin/fzf
  '';
}
`,
		},
		{
			name: "multiple bins",
			input: GenerateInput{
				Tool: config.Tool{
					Owner:         "cli",
					Repo:          "cli",
					Version:       "v2.87.0",
					Asset:         "gh_{version}_{os}_{arch}.tar.gz",
					Bins:          []string{"gh", "gh-auth", "gh-repo"},
					VersionPrefix: "v",
				},
				Lock: LockEntry{
					URL:    "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_macOS_arm64.tar.gz",
					SHA256: "sha256-XXXX",
				},
			},
			want: `{
  stdenv,
  fetchurl,
}:
stdenv.mkDerivation rec {
  pname = "cli";
  version = "2.87.0";

  src = fetchurl {
    url = "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_macOS_arm64.tar.gz";
    sha256 = "sha256-XXXX";
  };

  dontUnpack = true;

  installPhase = ''
    mkdir -p $out/bin
    tar -xzf $src -C $out/bin
    chmod +x $out/bin/gh
    chmod +x $out/bin/gh-auth
    chmod +x $out/bin/gh-repo
  '';
}
`,
		},
		{
			name: "zip asset uses unzip instead of tar",
			input: GenerateInput{
				Tool: config.Tool{
					Owner:         "hashicorp",
					Repo:          "packer",
					Version:       "v1.11.0",
					Asset:         "packer_{version}_{os}_{arch}.zip",
					Bins:          []string{"packer"},
					VersionPrefix: "v",
				},
				Lock: LockEntry{
					URL:    "https://releases.hashicorp.com/packer/1.11.0/packer_1.11.0_darwin_arm64.zip",
					SHA256: "sha256-PACKER",
				},
			},
			want: `{
  stdenv,
  fetchurl,
  unzip,
}:
stdenv.mkDerivation rec {
  pname = "packer";
  version = "1.11.0";

  src = fetchurl {
    url = "https://releases.hashicorp.com/packer/1.11.0/packer_1.11.0_darwin_arm64.zip";
    sha256 = "sha256-PACKER";
  };

  dontUnpack = true;

  nativeBuildInputs = [ unzip ];

  installPhase = ''
    mkdir -p $out/bin
    unzip -o $src -d $out/bin
    chmod +x $out/bin/packer
  '';
}
`,
		},
		{
			name: "version_prefix go strips go prefix",
			input: GenerateInput{
				Tool: config.Tool{
					Owner:         "golang",
					Repo:          "go",
					Version:       "go1.26.0",
					Asset:         "go{version}.darwin-arm64.tar.gz",
					Bins:          []string{"go", "gofmt"},
					VersionPrefix: "go",
				},
				Lock: LockEntry{
					URL:    "https://go.dev/dl/go1.26.0.darwin-arm64.tar.gz",
					SHA256: "sha256-GOLANG",
				},
			},
			want: `{
  stdenv,
  fetchurl,
}:
stdenv.mkDerivation rec {
  pname = "go";
  version = "1.26.0";

  src = fetchurl {
    url = "https://go.dev/dl/go1.26.0.darwin-arm64.tar.gz";
    sha256 = "sha256-GOLANG";
  };

  dontUnpack = true;

  installPhase = ''
    mkdir -p $out/bin
    tar -xzf $src -C $out/bin
    chmod +x $out/bin/go
    chmod +x $out/bin/gofmt
  '';
}
`,
		},
		{
			name: "version_prefix empty keeps version as-is",
			input: GenerateInput{
				Tool: config.Tool{
					Owner:         "BurntSushi",
					Repo:          "ripgrep",
					Version:       "14.1.0",
					Asset:         "ripgrep-{version}-{arch}-{os}.tar.gz",
					Bins:          []string{"rg"},
					VersionPrefix: "",
				},
				Lock: LockEntry{
					URL:    "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-arm64-linux.tar.gz",
					SHA256: "sha256-NOPREFIX",
				},
			},
			want: `{
  stdenv,
  fetchurl,
}:
stdenv.mkDerivation rec {
  pname = "ripgrep";
  version = "14.1.0";

  src = fetchurl {
    url = "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-arm64-linux.tar.gz";
    sha256 = "sha256-NOPREFIX";
  };

  dontUnpack = true;

  installPhase = ''
    mkdir -p $out/bin
    tar -xzf $src -C $out/bin
    chmod +x $out/bin/rg
  '';
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Generate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("Generate() mismatch\ngot:\n%s\nwant:\n%s", string(got), tt.want)
			}
		})
	}
}

func TestGenerate_versionStripping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		version     string
		wantVersion string
	}{
		{"strips v prefix", "v2.87.0", "2.87.0"},
		{"no v prefix unchanged", "14.1.0", "14.1.0"},
		{"single v prefix", "v1.0.0", "1.0.0"},
		{"no double strip", "vv1.0.0", "v1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := GenerateInput{
				Tool: config.Tool{
					Owner:         "owner",
					Repo:          "repo",
					Version:       tt.version,
					Asset:         "tool.tar.gz",
					Bins:          []string{"tool"},
					VersionPrefix: "v",
				},
				Lock: LockEntry{
					URL:    "https://example.com/tool.tar.gz",
					SHA256: "sha256-test",
				},
			}

			got, err := Generate(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantLine := `  version = "` + tt.wantVersion + `";`

			if !strings.Contains(string(got), wantLine) {
				t.Errorf("output does not contain %q\ngot:\n%s", wantLine, string(got))
			}
		})
	}
}

func TestGenerate_outputStructure(t *testing.T) { //nolint:funlen // output structure verification requires many assertions
	t.Parallel()

	input := GenerateInput{
		Tool: config.Tool{
			Owner:         "cli",
			Repo:          "cli",
			Version:       "v2.87.0",
			Asset:         "gh_{version}_{os}_{arch}.tar.gz",
			Bins:          []string{"gh"},
			VersionPrefix: "v",
		},
		Lock: LockEntry{
			URL:    "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_macOS_arm64.tar.gz",
			SHA256: "sha256-XXXX",
		},
	}

	got, err := Generate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := string(got)

	// Verify the nix file starts with the function arguments block.
	if !strings.HasPrefix(output, "{\n  stdenv,\n  fetchurl,\n}:\n") {
		t.Error("output does not start with expected function arguments block")
	}

	// Verify it contains stdenv.mkDerivation rec.
	if !strings.Contains(output, "stdenv.mkDerivation rec {") {
		t.Error("output does not contain stdenv.mkDerivation rec")
	}

	// Verify dontUnpack is present.
	if !strings.Contains(output, "dontUnpack = true;") {
		t.Error("output does not contain dontUnpack = true")
	}

	// Verify installPhase contains the expected commands.
	if !strings.Contains(output, "mkdir -p $out/bin") {
		t.Error("output does not contain mkdir -p $out/bin")
	}

	if !strings.Contains(output, "tar -xzf $src -C $out/bin") {
		t.Error("output does not contain tar command")
	}

	if !strings.Contains(output, "chmod +x $out/bin/gh") {
		t.Error("output does not contain chmod command for binary")
	}

	// Verify the file ends with a closing brace and newline.
	if !strings.HasSuffix(output, "}\n") {
		t.Error("output does not end with closing brace and newline")
	}
}

func TestGenerateAll(t *testing.T) { //nolint:funlen // table-driven test with many cases
	t.Parallel()

	outDir := t.TempDir()

	inputs := []GenerateInput{
		{
			Tool: config.Tool{
				Owner:         "cli",
				Repo:          "cli",
				Version:       "v2.87.0",
				Asset:         "gh_{version}_{os}_{arch}.tar.gz",
				Bins:          []string{"gh"},
				VersionPrefix: "v",
			},
			Lock: LockEntry{
				URL:    "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_macOS_arm64.tar.gz",
				SHA256: "sha256-XXXX",
			},
		},
		{
			Tool: config.Tool{
				Owner:         "BurntSushi",
				Repo:          "ripgrep",
				Version:       "14.1.0",
				Asset:         "ripgrep-{version}-{arch}-{os}.tar.gz",
				Bins:          []string{"rg"},
				VersionPrefix: "v",
			},
			Lock: LockEntry{
				URL:    "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-arm64-linux.tar.gz",
				SHA256: "sha256-YYYY",
			},
		},
		{
			Tool: config.Tool{
				Owner:         "aquaproj",
				Repo:          "aqua",
				Version:       "v2.39.0",
				Asset:         "aqua_{os}_{arch}.tar.gz",
				Bins:          []string{"aqua"},
				VersionPrefix: "v",
			},
			Lock: LockEntry{
				URL:    "https://github.com/aquaproj/aqua/releases/download/v2.39.0/aqua_darwin_arm64.tar.gz",
				SHA256: "sha256-ZZZZ",
			},
		},
	}

	if err := GenerateAll(inputs, outDir); err != nil {
		t.Fatalf("GenerateAll() unexpected error: %v", err)
	}

	// Verify directory structure and file existence.
	// Directory is now repo name, pname is repo name.
	wantFiles := []struct {
		repo string
		bins []string
	}{
		{"cli", []string{"gh"}},
		{"ripgrep", []string{"rg"}},
		{"aqua", []string{"aqua"}},
	}

	for _, wf := range wantFiles {
		path := filepath.Join(outDir, wf.repo, "default.nix")

		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected file %s to exist: %v", path, err)

			continue
		}

		if info.IsDir() {
			t.Errorf("expected %s to be a file, not a directory", path)

			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("failed to read %s: %v", path, err)

			continue
		}

		// Verify pname is the repo name.
		wantPname := `pname = "` + wf.repo + `";`

		if !strings.Contains(string(content), wantPname) {
			t.Errorf("file %s does not contain %q\ncontent:\n%s", path, wantPname, string(content))
		}

		// Verify chmod references each binary.
		for _, bin := range wf.bins {
			wantChmod := "chmod +x $out/bin/" + bin

			if !strings.Contains(string(content), wantChmod) {
				t.Errorf("file %s does not contain %q", path, wantChmod)
			}
		}
	}
}

func TestGenerateAll_emptyInputs(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	if err := GenerateAll(nil, outDir); err != nil {
		t.Fatalf("GenerateAll() with nil inputs: unexpected error: %v", err)
	}

	if err := GenerateAll([]GenerateInput{}, outDir); err != nil {
		t.Fatalf("GenerateAll() with empty inputs: unexpected error: %v", err)
	}

	// Verify no files were created.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("failed to read outDir: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected empty output directory, got %d entries", len(entries))
	}
}

func TestGenerateAll_invalidOutDir(t *testing.T) { //nolint:funlen // setup-heavy test
	t.Parallel()

	inputs := []GenerateInput{
		{
			Tool: config.Tool{
				Owner:         "cli",
				Repo:          "cli",
				Version:       "v1.0.0",
				Asset:         "tool.tar.gz",
				Bins:          []string{"gh"},
				VersionPrefix: "v",
			},
			Lock: LockEntry{
				URL:    "https://example.com/tool.tar.gz",
				SHA256: "sha256-test",
			},
		},
	}

	// Use a path that cannot be created (nested under a file, not a directory).
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")

	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := GenerateAll(inputs, filepath.Join(blocker, "subdir"))
	if err == nil {
		t.Fatal("expected error for invalid output directory, got nil")
	}
}

func TestGenerateAll_contentMatchesGenerate(t *testing.T) { //nolint:funlen // setup-heavy integration test
	t.Parallel()

	outDir := t.TempDir()

	input := GenerateInput{
		Tool: config.Tool{
			Owner:         "cli",
			Repo:          "cli",
			Version:       "v2.87.0",
			Asset:         "gh_{version}_{os}_{arch}.tar.gz",
			Bins:          []string{"gh"},
			VersionPrefix: "v",
		},
		Lock: LockEntry{
			URL:    "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_macOS_arm64.tar.gz",
			SHA256: "sha256-XXXX",
		},
	}

	// Generate via both paths.
	expected, err := Generate(input)
	if err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}

	if err := GenerateAll([]GenerateInput{input}, outDir); err != nil {
		t.Fatalf("GenerateAll() unexpected error: %v", err)
	}

	path := filepath.Join(outDir, "cli", "default.nix")

	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	if string(actual) != string(expected) {
		t.Errorf("GenerateAll output differs from Generate output\nGenerateAll:\n%s\nGenerate:\n%s",
			string(actual), string(expected))
	}
}

func TestGenerateAll_binsFallbackToRepo(t *testing.T) { //nolint:funlen // setup-heavy integration test
	t.Parallel()

	outDir := t.TempDir()

	input := GenerateInput{
		Tool: config.Tool{
			Owner:         "hashicorp",
			Repo:          "terraform",
			Version:       "v1.9.0",
			Asset:         "terraform_{version}_{os}_{arch}.zip",
			Bins:          nil,
			VersionPrefix: "v",
		},
		Lock: LockEntry{
			URL:    "https://example.com/terraform.zip",
			SHA256: "sha256-test",
		},
	}

	if err := GenerateAll([]GenerateInput{input}, outDir); err != nil {
		t.Fatalf("GenerateAll() unexpected error: %v", err)
	}

	// When bins is empty, the directory and pname should use the repo name.
	path := filepath.Join(outDir, "terraform", "default.nix")

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}

	if !strings.Contains(string(content), `pname = "terraform"`) {
		t.Errorf("pname should be terraform, got:\n%s", string(content))
	}

	if !strings.Contains(string(content), "chmod +x $out/bin/terraform") {
		t.Errorf("chmod should reference terraform, got:\n%s", string(content))
	}
}
