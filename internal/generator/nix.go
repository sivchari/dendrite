// Package generator produces Nix default.nix files from parsed config and lock data.
package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/sivchari/dendrite/internal/config"
)

// LockEntry holds the resolved download URL and its content hash for a single tool.
type LockEntry struct {
	URL    string
	SHA256 string
}

// GenerateInput pairs a tool definition with its lock entry for nix generation.
type GenerateInput struct {
	Tool config.Tool
	Lock LockEntry
}

// nixTemplateText is the text/template source for a default.nix file.
const nixTemplateText = `{{ if .NeedsUnzip }}{
  stdenv,
  fetchurl,
  unzip,
}:{{ else }}{
  stdenv,
  fetchurl,
}:{{ end }}
stdenv.mkDerivation rec {
  pname = "{{ .Pname }}";
  version = "{{ .Version }}";

  src = fetchurl {
    url = "{{ .URL }}";
    sha256 = "{{ .SHA256 }}";
  };

  dontUnpack = true;
{{ if .NeedsUnzip }}
  nativeBuildInputs = [ unzip ];
{{ end }}
  installPhase = ''
    mkdir -p $out/bin
    {{ .ExtractCmd }}
{{ range .Bins }}    chmod +x $out/bin/{{ . }}
{{ end }}  '';
}
`

// nixTemplate is the parsed template, compiled once at init time.
var nixTemplate = template.Must(template.New("default.nix").Parse(nixTemplateText))

// templateData holds the values injected into the nix template.
type templateData struct {
	Pname      string
	Version    string
	URL        string
	SHA256     string
	Bins       []string
	NeedsUnzip bool
	ExtractCmd string
}

// Generate renders a default.nix file for the given input.
// The version has its prefix stripped in the output based on VersionPrefix.
func Generate(input GenerateInput) ([]byte, error) {
	version := strings.TrimPrefix(input.Tool.Version, input.Tool.VersionPrefix)

	bins := input.Tool.Bins
	if len(bins) == 0 {
		bins = []string{input.Tool.Repo}
	}

	needsUnzip := strings.HasSuffix(input.Lock.URL, ".zip")
	extractCmd := "tar -xzf $src -C $out/bin"

	if needsUnzip {
		extractCmd = "unzip -o $src -d $out/bin"
	}

	data := templateData{
		Pname:      input.Tool.Repo,
		Version:    version,
		URL:        input.Lock.URL,
		SHA256:     input.Lock.SHA256,
		Bins:       bins,
		NeedsUnzip: needsUnzip,
		ExtractCmd: extractCmd,
	}

	var buf bytes.Buffer
	if err := nixTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute nix template for %s: %w", data.Pname, err)
	}

	return buf.Bytes(), nil
}

// GenerateAll writes a default.nix file for each input into outDir/<repo>/default.nix.
func GenerateAll(inputs []GenerateInput, outDir string) error {
	for _, input := range inputs {
		content, err := Generate(input)
		if err != nil {
			return err
		}

		dir := filepath.Join(outDir, input.Tool.Repo)

		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // standard directory permissions for Nix packages
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		path := filepath.Join(dir, "default.nix")

		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}

	return nil
}
