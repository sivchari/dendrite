// Package platform provides version resolution for expanding asset
// pattern placeholders into a concrete download URL.
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

// nixSystemMap maps lock file platform keys ("os_arch") to Nix system identifiers.
var nixSystemMap = map[string]string{
	"darwin_arm64": "aarch64-darwin",
	"darwin_amd64": "x86_64-darwin",
	"linux_arm64":  "aarch64-linux",
	"linux_amd64":  "x86_64-linux",
}

// NixSystem converts a lock file platform key (e.g. "darwin_arm64") to the
// corresponding Nix system identifier (e.g. "aarch64-darwin").
// Returns empty string if the key is not recognized.
func NixSystem(platformKey string) string {
	return nixSystemMap[platformKey]
}

// Resolve expands an asset pattern by substituting the {version} placeholder.
// The version string has the given prefix stripped before substitution.
func Resolve(pattern, version, prefix string) string {
	ver := strings.TrimPrefix(version, prefix)

	return strings.ReplaceAll(pattern, "{version}", ver)
}
