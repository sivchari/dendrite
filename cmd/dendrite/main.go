// Package main provides the dendrite CLI for managing Nix package generation from GitHub releases.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sivchari/dendrite/internal/config"
	"github.com/sivchari/dendrite/internal/generator"
	"github.com/sivchari/dendrite/internal/lock"
	"github.com/sivchari/dendrite/internal/platform"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dendrite <command> [flags]")
		fmt.Fprintln(os.Stderr, "commands: generate, lock")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "lock":
		if err := runLock(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "commands: generate, lock")
		os.Exit(1)
	}
}

// toolName reconstructs the "owner/repo@version" identifier from a parsed Tool.
func toolName(t *config.Tool) string {
	return t.Owner + "/" + t.Repo + "@" + t.Version
}

// lockFilePath derives the lock file path from the config file path.
// It replaces the config filename with "dendrite.lock.yaml" in the same directory.
func lockFilePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "dendrite.lock.yaml")
}

// parsePlatform parses a "os/arch" string into a platform.Platform.
func parsePlatform(s string) (platform.Platform, error) {
	parts := strings.SplitN(s, "/", 2)

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return platform.Platform{}, fmt.Errorf("invalid platform format %q: expected os/arch (e.g. darwin/arm64)", s)
	}

	return platform.Platform{OS: parts[0], Arch: parts[1]}, nil
}

// buildGitHubReleaseURL constructs the download URL for a GitHub release asset.
func buildGitHubReleaseURL(owner, repo, version, assetName string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, version, assetName)
}

func runGenerate(args []string) error { //nolint:funlen // CLI command with sequential steps
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)

	configFile := fs.String("f", "dendrite.yaml", "path to config file")
	outDir := fs.String("o", "packages/", "output directory")
	runLockFirst := fs.Bool("lock", false, "run lock before generate")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	cfg, err := config.Parse(*configFile)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if *runLockFirst {
		platformStrs := cfg.Platforms()

		var platforms []platform.Platform

		for _, s := range platformStrs {
			p, parseErr := parsePlatform(s)
			if parseErr != nil {
				return fmt.Errorf("failed to parse platform: %w", parseErr)
			}

			platforms = append(platforms, p)
		}

		lockPath := lockFilePath(*configFile)

		if lockErr := executeLock(cfg, lockPath, platforms); lockErr != nil {
			return fmt.Errorf("failed to execute lock: %w", lockErr)
		}
	}

	lockPath := lockFilePath(*configFile)

	lf, err := lock.Read(lockPath)
	if err != nil {
		return fmt.Errorf("failed to read lock file: %w\nrun 'dendrite lock' first", err)
	}

	var inputs []generator.GenerateInput

	for i := range cfg.Tools {
		name := toolName(&cfg.Tools[i])

		locks := make(map[string]generator.LockEntry)

		// Collect lock entries for all platforms in the lock file for this tool.
		for j := range lf.Tools {
			if lf.Tools[j].Name != name {
				continue
			}

			for pKey, asset := range lf.Tools[j].Assets {
				locks[pKey] = generator.LockEntry{
					URL:    asset.URL,
					SHA256: asset.SHA256,
				}
			}

			break
		}

		if len(locks) == 0 {
			return fmt.Errorf("no lock entries found for %s; run 'dendrite lock' first", name)
		}

		inputs = append(inputs, generator.GenerateInput{
			Tool:  cfg.Tools[i],
			Locks: locks,
		})
	}

	if err := generator.GenerateAll(inputs, *outDir); err != nil {
		return fmt.Errorf("failed to generate nix files: %w", err)
	}

	fmt.Fprintf(os.Stderr, "generated %d package(s) in %s\n", len(inputs), *outDir)

	return nil
}

func runLock(args []string) error {
	fs := flag.NewFlagSet("lock", flag.ContinueOnError)

	configFile := fs.String("f", "dendrite.yaml", "path to config file")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	cfg, err := config.Parse(*configFile)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	platformStrs := cfg.Platforms()

	if len(platformStrs) == 0 {
		return fmt.Errorf("no target platforms found in config asset keys")
	}

	var platforms []platform.Platform

	for _, s := range platformStrs {
		p, parseErr := parsePlatform(s)
		if parseErr != nil {
			return fmt.Errorf("failed to parse platform %q: %w", s, parseErr)
		}

		platforms = append(platforms, p)
	}

	lockPath := lockFilePath(*configFile)

	if err := executeLock(cfg, lockPath, platforms); err != nil {
		return fmt.Errorf("failed to execute lock: %w", err)
	}

	return nil
}

// executeLock performs the lock flow: for each tool and platform, resolve asset URLs,
// prefetch SHA256 hashes, and write the lock file.
func executeLock(cfg *config.Config, lockPath string, platforms []platform.Platform) error { //nolint:funlen // sequential CLI logic
	// Read existing lock file if it exists.
	var existing *lock.File

	if _, statErr := os.Stat(lockPath); statErr == nil {
		var readErr error

		existing, readErr = lock.Read(lockPath)
		if readErr != nil {
			return fmt.Errorf("failed to read existing lock file: %w", readErr)
		}
	}

	lf := &lock.File{}

	for i := range cfg.Tools {
		name := toolName(&cfg.Tools[i])
		tl := lock.ToolLock{
			Name:   name,
			Assets: make(map[string]lock.AssetLock),
		}

		for j := range platforms {
			pKey := lock.PlatformKey(platforms[j])
			platformStr := platforms[j].OS + "/" + platforms[j].Arch

			// Check existing lock file for unchanged entries.
			if existing != nil {
				if entry := lock.Lookup(existing, name, pKey); entry != nil {
					fmt.Fprintf(os.Stderr, "reusing existing lock for %s on %s\n", name, platformStr)

					tl.Assets[pKey] = lock.AssetLock{
						URL:    entry.URL,
						SHA256: entry.SHA256,
					}

					continue
				}
			}

			fmt.Fprintf(os.Stderr, "locking %s for %s...\n", name, platformStr)

			candidates := resolveCandidates(&cfg.Tools[i], platforms[j])

			var locked bool

			for _, url := range candidates {
				sha256, prefetchErr := lock.PrefetchURL(url)
				if prefetchErr != nil {
					continue
				}

				tl.Assets[pKey] = lock.AssetLock{
					URL:    url,
					SHA256: sha256,
				}

				locked = true

				break
			}

			if !locked {
				return fmt.Errorf("failed to lock %s for %s: none of the candidate URLs succeeded", name, platformStr)
			}
		}

		lf.Tools = append(lf.Tools, tl)
	}

	if err := lock.Write(lockPath, lf); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "lock file written to %s\n", lockPath)

	return nil
}

// resolveCandidates returns the candidate URL for a tool on a given platform.
func resolveCandidates(tool *config.Tool, p platform.Platform) []string {
	if tool.URL != "" {
		return []string{platform.Resolve(tool.URL, tool.Version, tool.VersionPrefix)}
	}

	platformKey := p.OS + "/" + p.Arch

	assetPattern, ok := tool.Asset[platformKey]
	if !ok {
		return nil
	}

	assetName := platform.Resolve(assetPattern, tool.Version, tool.VersionPrefix)

	return []string{buildGitHubReleaseURL(tool.Owner, tool.Repo, tool.Version, assetName)}
}
