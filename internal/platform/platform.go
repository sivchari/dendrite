// Package platform provides OS/architecture mapping for resolving asset
// pattern placeholders into concrete download URLs.
package platform

import "strings"

// Platform represents a target OS and architecture combination.
type Platform struct {
	OS   string
	Arch string
}

// Predefined platform constants for common targets.
var (
	DarwinARM64 = Platform{OS: "darwin", Arch: "arm64"}
	DarwinAMD64 = Platform{OS: "darwin", Arch: "amd64"}
	LinuxARM64  = Platform{OS: "linux", Arch: "arm64"}
	LinuxAMD64  = Platform{OS: "linux", Arch: "amd64"}
)

// Mapping holds the set of placeholder replacement values for a given
// OS or architecture. For example, the "darwin" OS maps to the names
// ["darwin", "macOS"], meaning {os} can be replaced with either value.
type Mapping struct {
	OSNames   []string
	ArchNames []string
}

// defaultMappings contains the built-in OS/arch placeholder values.
// The outer key is "<os>/<arch>" (e.g. "darwin/arm64").
var defaultMappings = map[string]Mapping{
	"darwin/arm64": {
		OSNames:   []string{"darwin", "macOS"},
		ArchNames: []string{"arm64", "aarch64"},
	},
	"darwin/amd64": {
		OSNames:   []string{"darwin", "macOS"},
		ArchNames: []string{"amd64", "x86_64"},
	},
	"linux/arm64": {
		OSNames:   []string{"linux", "Linux"},
		ArchNames: []string{"arm64", "aarch64"},
	},
	"linux/amd64": {
		OSNames:   []string{"linux", "Linux"},
		ArchNames: []string{"amd64", "x86_64"},
	},
}

// DefaultMapping returns the default Mapping for the given platform.
// If the platform is not found in the built-in table, it returns a
// Mapping that uses the raw OS and Arch values as single-element lists.
func DefaultMapping(p Platform) Mapping {
	key := p.OS + "/" + p.Arch

	if m, ok := defaultMappings[key]; ok {
		return m
	}

	return Mapping{
		OSNames:   []string{p.OS},
		ArchNames: []string{p.Arch},
	}
}

// Resolve expands an asset pattern into all possible concrete asset names
// by substituting {version}, {os}, and {arch} placeholders with the
// appropriate values for the given platform.
//
// The version string has a leading "v" stripped before substitution.
// For example, given:
//
//	asset    = "gh_{version}_{os}_{arch}.tar.gz"
//	version  = "v2.87.0"
//	platform = DarwinARM64
//
// Resolve returns:
//
//	["gh_2.87.0_darwin_arm64.tar.gz",
//	 "gh_2.87.0_darwin_aarch64.tar.gz",
//	 "gh_2.87.0_macOS_arm64.tar.gz",
//	 "gh_2.87.0_macOS_aarch64.tar.gz"]
func Resolve(asset, version string, p Platform) []string {
	return ResolveWithPrefix(asset, version, "v", p)
}

// ResolveWithPrefix is like Resolve but strips the given prefix from version.
func ResolveWithPrefix(asset, version, prefix string, p Platform) []string {
	m := DefaultMapping(p)

	ver := strings.TrimPrefix(version, prefix)
	base := strings.ReplaceAll(asset, "{version}", ver)

	seen := make(map[string]struct{})
	results := make([]string, 0, len(m.OSNames)*len(m.ArchNames))

	for _, osName := range m.OSNames {
		for _, archName := range m.ArchNames {
			r := strings.ReplaceAll(base, "{os}", osName)
			r = strings.ReplaceAll(r, "{arch}", archName)

			if _, dup := seen[r]; !dup {
				seen[r] = struct{}{}

				results = append(results, r)
			}
		}
	}

	return results
}
