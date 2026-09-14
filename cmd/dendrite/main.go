// Package main provides the dendrite CLI for managing Nix package generation from GitHub releases.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
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
func executeLock(cfg *config.Config, lockPath string, platforms []platform.Platform) error {
	existing, err := readExistingLock(lockPath)
	if err != nil {
		return err
	}

	token, err := privateToken(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()
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

			assetLock, ok, lockErr := lockToolPlatform(ctx, existing, &cfg.Tools[i], name, pKey, platformStr, platforms[j], token)
			if lockErr != nil {
				return fmt.Errorf("failed to lock %s for %s: %w", name, platformStr, lockErr)
			}

			if !ok {
				continue
			}

			tl.Assets[pKey] = assetLock
		}

		lf.Tools = append(lf.Tools, tl)
	}

	if err := lock.Write(lockPath, lf); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "lock file written to %s\n", lockPath)

	return nil
}

// readExistingLock reads the lock file at lockPath if it exists, returning an
// empty (non-nil) *lock.File when no lock file has been written yet, so
// callers can look up entries unconditionally.
func readExistingLock(lockPath string) (*lock.File, error) {
	existing, err := lock.Read(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &lock.File{}, nil
		}

		return nil, fmt.Errorf("failed to read existing lock file: %w", err)
	}

	return existing, nil
}

// lockToolPlatform resolves the AssetLock for a single (tool, platform) pair:
// it reuses an existing lock entry when present, and otherwise locks via the
// private GitHub API flow or the public nix-prefetch-url flow. ok is false
// when the tool declares no asset for this platform, meaning it should be
// skipped without error.
func lockToolPlatform(
	ctx context.Context,
	existing *lock.File,
	tool *config.Tool,
	name, pKey, platformStr string,
	p platform.Platform,
	token string,
) (lock.AssetLock, bool, error) {
	if entry := lock.Lookup(existing, name, pKey); entry != nil {
		fmt.Fprintf(os.Stderr, "reusing existing lock for %s on %s\n", name, platformStr)

		return *entry, true, nil
	}

	if tool.Private {
		assetLock, ok, err := lockPrivateAsset(ctx, tool, p, token)
		if !ok || err != nil {
			return lock.AssetLock{}, false, err
		}

		fmt.Fprintf(os.Stderr, "locking %s for %s...\n", name, platformStr)

		return assetLock, true, nil
	}

	return lockPublicAsset(tool, name, platformStr, p)
}

// lockPublicAsset resolves and prefetches the SHA256 hash for a public tool's
// release asset on a single platform, trying each candidate URL in order
// until one succeeds. ok is false when the tool declares no asset pattern
// for this platform.
func lockPublicAsset(tool *config.Tool, name, platformStr string, p platform.Platform) (lock.AssetLock, bool, error) {
	candidates := resolveCandidates(tool, p)
	if len(candidates) == 0 {
		return lock.AssetLock{}, false, nil
	}

	fmt.Fprintf(os.Stderr, "locking %s for %s...\n", name, platformStr)

	for _, url := range candidates {
		sha256, err := lock.PrefetchURL(url)
		if err != nil {
			continue
		}

		return lock.AssetLock{URL: url, SHA256: sha256}, true, nil
	}

	return lock.AssetLock{}, false, errors.New("none of the candidate URLs succeeded")
}

// resolveCandidates returns the candidate URL for a tool on a given platform.
func resolveCandidates(tool *config.Tool, p platform.Platform) []string {
	platformKey := p.OS + "/" + p.Arch

	if urlPattern, ok := tool.URL[platformKey]; ok {
		return platform.ResolveCandidates(urlPattern, tool.Version, tool.VersionPrefix, p)
	}

	assetPattern, ok := tool.Asset[platformKey]
	if !ok {
		return nil
	}

	assetNames := platform.ResolveCandidates(assetPattern, tool.Version, tool.VersionPrefix, p)
	candidates := make([]string, 0, len(assetNames))

	for _, assetName := range assetNames {
		candidates = append(candidates, buildGitHubReleaseURL(tool.Owner, tool.Repo, tool.Version, assetName))
	}

	return candidates
}

// privateToken resolves a GitHub API token if the config declares at least
// one private tool. It returns an empty string (and no error) when no tool
// is private, since no token is needed in that case.
func privateToken(cfg *config.Config) (string, error) {
	for i := range cfg.Tools {
		if cfg.Tools[i].Private {
			token, err := lock.Token()
			if err != nil {
				return "", fmt.Errorf("failed to resolve private token: %w", err)
			}

			return token, nil
		}
	}

	return "", nil
}

// lockPrivateAsset resolves and downloads a private tool's release asset for
// a single platform. The second return value is false when the tool declares
// no asset pattern for that platform, meaning it should be skipped.
func lockPrivateAsset(ctx context.Context, tool *config.Tool, p platform.Platform, token string) (lock.AssetLock, bool, error) {
	platformKey := p.OS + "/" + p.Arch

	pattern, ok := tool.Asset[platformKey]
	if !ok {
		return lock.AssetLock{}, false, nil
	}

	assetName := platform.Resolve(pattern, tool.Version, tool.VersionPrefix)

	assetID, sha256, err := lock.FetchPrivateAsset(ctx, http.DefaultClient, tool.Owner, tool.Repo, tool.Version, assetName, token)
	if err != nil {
		return lock.AssetLock{}, false, fmt.Errorf("failed to fetch private asset %s: %w", assetName, err)
	}

	return lock.AssetLock{
		URL:     lock.PrivateAssetURL(tool.Owner, tool.Repo, assetID),
		SHA256:  sha256,
		AssetID: assetID,
	}, true, nil
}
