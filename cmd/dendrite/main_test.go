package main

import (
	"context"
	"testing"

	"github.com/sivchari/dendrite/internal/config"
	"github.com/sivchari/dendrite/internal/platform"
)

func TestResolveCandidates(t *testing.T) {
	t.Parallel()

	tool := &config.Tool{
		Owner:         "cli",
		Repo:          "cli",
		Version:       "v2.87.0",
		VersionPrefix: "v",
		Asset: map[string]string{
			"darwin/arm64": "gh_{version}_{os}_{arch}.zip",
		},
	}

	got := resolveCandidates(tool, platform.DarwinARM64)

	if len(got) == 0 {
		t.Fatal("resolveCandidates() returned no candidates")
	}

	wantFirst := "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_darwin_arm64.zip"
	if got[0] != wantFirst {
		t.Errorf("resolveCandidates()[0] = %q, want %q", got[0], wantFirst)
	}

	wantFallback := "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_darwin_arm64.tar.gz"
	if !containsString(got, wantFallback) {
		t.Errorf("resolveCandidates() does not contain %q\ngot: %#v", wantFallback, got)
	}

	wantAlias := "https://github.com/cli/cli/releases/download/v2.87.0/gh_2.87.0_macOS_arm64.zip"
	if !containsString(got, wantAlias) {
		t.Errorf("resolveCandidates() does not contain %q\ngot: %#v", wantAlias, got)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}

func TestPrivateToken(t *testing.T) {
	t.Run("no private tools requires no token", func(t *testing.T) {
		cfg := &config.Config{
			Tools: []config.Tool{
				{Owner: "cli", Repo: "cli", Version: "v2.87.0"},
			},
		}

		got, err := privateToken(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "" {
			t.Errorf("privateToken() = %q, want empty string", got)
		}
	})

	t.Run("private tool without token errors", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "")

		cfg := &config.Config{
			Tools: []config.Tool{
				{Owner: "LayerXcom", Repo: "layerone", Version: "v0.3.1", Private: true},
			},
		}

		if _, err := privateToken(cfg); err == nil {
			t.Fatal("expected error for missing token, got nil")
		}
	})

	t.Run("private tool with token succeeds", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "test-token")

		cfg := &config.Config{
			Tools: []config.Tool{
				{Owner: "LayerXcom", Repo: "layerone", Version: "v0.3.1", Private: true},
			},
		}

		got, err := privateToken(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "test-token" {
			t.Errorf("privateToken() = %q, want %q", got, "test-token")
		}
	})
}

func TestLockPrivateAsset_skipsMissingPlatform(t *testing.T) {
	t.Parallel()

	tool := &config.Tool{
		Owner:   "LayerXcom",
		Repo:    "layerone",
		Version: "haro-cli/v0.3.1",
		Private: true,
		Asset: map[string]string{
			"darwin/arm64": "haro_{version}_darwin_arm64.tar.gz",
		},
	}

	_, ok, err := lockPrivateAsset(context.Background(), tool, platform.LinuxAMD64, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ok {
		t.Error("expected ok=false when tool declares no asset pattern for the platform")
	}
}
