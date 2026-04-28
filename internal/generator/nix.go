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
  dontStrip = true;
{{ if .NeedsUnzip }}
  nativeBuildInputs = [ unzip ];
{{ end }}
  installPhase = ''
    mkdir -p $out/bin
    {{ .ExtractCmd }}
{{ range .Renames }}    mv $out/bin/{{ .From }} $out/bin/{{ .To }}
{{ end }}{{ range .Bins }}    chmod +x $out/bin/{{ . }}
{{ end }}  '';
}
`

// nixTemplate is the parsed template, compiled once at init time.
var nixTemplate = template.Must(template.New("default.nix").Parse(nixTemplateText))

// renameEntry represents a mv command in the installPhase.
type renameEntry struct {
	From string
	To   string
}

// templateData holds the values injected into the nix template.
type templateData struct {
	Pname      string
	Version    string
	URL        string
	SHA256     string
	Bins       []string
	Renames    []renameEntry
	NeedsUnzip bool
	ExtractCmd string
}

// assetFormat represents the type of asset archive.
type assetFormat int

const (
	formatTar assetFormat = iota
	formatZip
	formatRaw
)

// detectFormat determines the asset format. If formatOverride is set, it takes
// precedence over URL extension detection.
func detectFormat(url, formatOverride string) assetFormat {
	if formatOverride != "" {
		switch formatOverride {
		case "zip":
			return formatZip
		case "raw":
			return formatRaw
		default:
			return formatTar
		}
	}

	switch {
	case strings.HasSuffix(url, ".zip"):
		return formatZip
	case strings.HasSuffix(url, ".tar.gz"),
		strings.HasSuffix(url, ".tar.xz"),
		strings.HasSuffix(url, ".tar.bz2"),
		strings.HasSuffix(url, ".tgz"):
		return formatTar
	default:
		return formatRaw
	}
}

// tarCommand returns the appropriate tar command based on the format override or URL extension.
func tarCommand(url, formatOverride string, stripComponents int) string {
	key := formatOverride
	if key == "" {
		key = url
	}

	var base string

	switch {
	case key == "tar":
		base = "tar -xf $src -C $out/bin"
	case key == "tar.bz2" || strings.HasSuffix(key, ".tar.bz2"):
		base = "tar -xjf $src -C $out/bin"
	case key == "tar.xz" || strings.HasSuffix(key, ".tar.xz"):
		base = "tar -xJf $src -C $out/bin"
	default:
		base = "tar -xzf $src -C $out/bin"
	}

	if stripComponents > 0 {
		base += fmt.Sprintf(" --strip-components=%d", stripComponents)
	}

	return base
}

// zipCommand returns the unzip command. When strip_components > 0,
// it extracts to a temp dir then copies only the bins to $out/bin.
func zipCommand(bins []string, stripComponents int) string {
	if stripComponents == 0 {
		return "unzip -o $src -d $out/bin"
	}

	cmd := "tmpdir=$(mktemp -d) && unzip -o $src -d $tmpdir && find $tmpdir -type f \\( "

	for i, bin := range bins {
		if i > 0 {
			cmd += " -o "
		}

		cmd += "-name " + bin
	}

	cmd += " \\) -exec cp {} $out/bin/ \\;"

	return cmd
}

// Generate renders a default.nix file for the given input.
// The version has its prefix stripped in the output based on VersionPrefix.
func Generate(input *GenerateInput) ([]byte, error) {
	version := strings.TrimPrefix(input.Tool.Version, input.Tool.VersionPrefix)

	bins := input.Tool.Bins
	if len(bins) == 0 {
		bins = []string{input.Tool.Repo}
	}

	format := detectFormat(input.Lock.URL, input.Tool.Format)
	needsUnzip := format == formatZip

	var extractCmd string

	switch format {
	case formatTar:
		extractCmd = tarCommand(input.Lock.URL, input.Tool.Format, input.Tool.StripComponents)
	case formatZip:
		extractCmd = zipCommand(bins, input.Tool.StripComponents)
	case formatRaw:
		if len(bins) == 1 {
			extractCmd = "cp $src $out/bin/" + bins[0]
		} else {
			extractCmd = "cp $src $out/bin/" + input.Tool.Repo
		}
	}

	var renames []renameEntry

	for _, bin := range bins {
		if src, ok := input.Tool.BinMap[bin]; ok {
			renames = append(renames, renameEntry{From: src, To: bin})
		}
	}

	data := templateData{
		Pname:      input.Tool.Repo,
		Version:    version,
		URL:        input.Lock.URL,
		SHA256:     input.Lock.SHA256,
		Bins:       bins,
		Renames:    renames,
		NeedsUnzip: needsUnzip,
		ExtractCmd: extractCmd,
	}

	var buf bytes.Buffer
	if err := nixTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute nix template for %s: %w", data.Pname, err)
	}

	return buf.Bytes(), nil
}

// GenerateAll writes a default.nix file for each input into outDir/<owner>/<repo>/default.nix,
// and generates an index default.nix at outDir/default.nix that imports all packages.
func GenerateAll(inputs []GenerateInput, outDir string) error {
	for i := range inputs {
		content, err := Generate(&inputs[i])
		if err != nil {
			return err
		}

		dir := filepath.Join(outDir, inputs[i].Tool.Owner, inputs[i].Tool.Repo)

		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // standard directory permissions for Nix packages
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		path := filepath.Join(dir, "default.nix")

		if err := os.WriteFile(path, content, 0o600); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}

	if err := generateIndex(inputs, outDir); err != nil {
		return fmt.Errorf("failed to generate index: %w", err)
	}

	return nil
}

// generateIndex writes an index default.nix that imports all generated packages.
// It is a no-op when inputs is empty.
func generateIndex(inputs []GenerateInput, outDir string) error {
	if len(inputs) == 0 {
		return nil
	}

	var buf bytes.Buffer

	buf.WriteString("# Auto-generated by dendrite. Do not edit.\n")
	buf.WriteString("{ pkgs }:\n[\n")

	for i := range inputs {
		fmt.Fprintf(&buf, "  (pkgs.callPackage ./%s/%s { })\n", inputs[i].Tool.Owner, inputs[i].Tool.Repo)
	}

	buf.WriteString("]\n")

	path := filepath.Join(outDir, "default.nix")

	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}
