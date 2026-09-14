// Package platform provides version resolution for expanding asset
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

// DefaultPlatforms are used when a tool declares a single asset or URL pattern.
var DefaultPlatforms = []Platform{
	LinuxAMD64,
	LinuxARM64,
	DarwinARM64,
}

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

// Resolve expands an asset pattern by substituting the {version} placeholder
// with SanitizedVersion(version, prefix).
func Resolve(pattern, version, prefix string) string {
	return strings.ReplaceAll(pattern, "{version}", SanitizedVersion(version, prefix))
}

// SanitizedVersion reduces a release version/tag to a token safe for
// embedding in asset filenames, URLs, and Nix store paths. Tags that embed a
// path (e.g. "haro-cli/v0.3.1", used to disambiguate releases across
// multiple tools in one repository) are first reduced to their last path
// segment, since "/" is not valid in those contexts; the given prefix is
// then stripped from the result.
func SanitizedVersion(version, prefix string) string {
	if idx := strings.LastIndex(version, "/"); idx != -1 {
		version = version[idx+1:]
	}

	return strings.TrimPrefix(version, prefix)
}

// ResolveCandidates expands a pattern for a platform using common release asset
// spellings for OS and architecture names.
func ResolveCandidates(pattern, version, prefix string, p Platform) []string {
	resolved := Resolve(pattern, version, prefix)

	if !strings.Contains(resolved, "{os}") && !strings.Contains(resolved, "{arch}") {
		return []string{resolved}
	}

	seen := make(map[string]struct{})
	candidates := make([]string, 0)

	for _, osName := range osAliases(p.OS) {
		for _, archName := range archAliases(p.Arch) {
			candidate := strings.ReplaceAll(resolved, "{os}", osName)
			candidate = strings.ReplaceAll(candidate, "{arch}", archName)

			candidates = appendUniqueCandidates(candidates, seen, archiveCandidates(candidate)...)
		}
	}

	return candidates
}

func appendUniqueCandidates(candidates []string, seen map[string]struct{}, values ...string) []string {
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}

		candidates = append(candidates, value)
	}

	return candidates
}

func archiveCandidates(pattern string) []string {
	extensions := []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".zip"}

	for _, ext := range extensions {
		if strings.HasSuffix(pattern, ext) {
			candidates := []string{pattern}
			base := strings.TrimSuffix(pattern, ext)

			for _, alt := range extensions {
				if alt == ext {
					continue
				}

				candidates = append(candidates, base+alt)
			}

			return candidates
		}
	}

	return []string{pattern}
}

func osAliases(os string) []string {
	switch os {
	case "darwin":
		return []string{"darwin", "macOS", "apple-darwin", "Darwin"}
	case "linux":
		return []string{"linux", "Linux", "unknown-linux-musl", "unknown-linux-gnu"}
	default:
		return []string{os}
	}
}

func archAliases(arch string) []string {
	switch arch {
	case "amd64":
		return []string{"amd64", "x86_64"}
	case "arm64":
		return []string{"arm64", "aarch64"}
	default:
		return []string{arch}
	}
}
