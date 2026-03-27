package build

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kacy/xless/internal/config"
	"github.com/kacy/xless/internal/toolchain"
)

func TestSignStageSignsPackageBundlesBeforeApp(t *testing.T) {
	originalRun := runSignCommand
	t.Cleanup(func() {
		runSignCommand = originalRun
	})

	var calls [][]string
	runSignCommand = func(_ context.Context, name string, args ...string) (*toolchain.CommandResult, error) {
		if name != "codesign" {
			t.Fatalf("command = %q, want codesign", name)
		}
		calls = append(calls, append([]string(nil), args...))
		return &toolchain.CommandResult{}, nil
	}

	bc := &BuildContext{
		Ctx:           context.Background(),
		Platform:      toolchain.PlatformSimulator,
		AppBundlePath: filepath.Join(t.TempDir(), "Weather.app"),
		PackageResourceBundles: []string{
			"/tmp/build/WeatherUI.bundle",
			"/tmp/build/SharedAssets.bundle",
		},
		Target: &config.TargetConfig{
			Signing: config.SigningConfig{},
		},
		Out: noopFormatter{},
	}

	if err := (SignStage{}).Run(bc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("codesign calls = %d, want 3", len(calls))
	}
	if got := calls[0][len(calls[0])-1]; got != filepath.Join(bc.AppBundlePath, "WeatherUI.bundle") {
		t.Fatalf("first signed path = %q", got)
	}
	if got := calls[1][len(calls[1])-1]; got != filepath.Join(bc.AppBundlePath, "SharedAssets.bundle") {
		t.Fatalf("second signed path = %q", got)
	}
	if got := calls[2][len(calls[2])-1]; got != bc.AppBundlePath {
		t.Fatalf("final signed path = %q", got)
	}
}

func TestSignStageOnlyAppliesEntitlementsToAppBundle(t *testing.T) {
	originalRun := runSignCommand
	t.Cleanup(func() {
		runSignCommand = originalRun
	})

	var calls [][]string
	runSignCommand = func(_ context.Context, _ string, args ...string) (*toolchain.CommandResult, error) {
		calls = append(calls, append([]string(nil), args...))
		return &toolchain.CommandResult{}, nil
	}

	dir := t.TempDir()
	entitlements := filepath.Join(dir, "Weather.entitlements")
	writeFile(t, entitlements, "<plist/>")

	bc := &BuildContext{
		Ctx:           context.Background(),
		ProjectDir:    dir,
		Platform:      toolchain.PlatformSimulator,
		AppBundlePath: filepath.Join(dir, "Weather.app"),
		PackageResourceBundles: []string{
			filepath.Join(dir, ".build", "WeatherUI.bundle"),
		},
		Target: &config.TargetConfig{
			Signing: config.SigningConfig{
				Identity:     "Apple Development: kacy@example.com",
				Entitlements: "Weather.entitlements",
			},
		},
		Out: noopFormatter{},
	}

	if err := (SignStage{}).Run(bc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("codesign calls = %d, want 2", len(calls))
	}
	if containsValue(calls[0], "--entitlements") {
		t.Fatalf("nested bundle args should not include entitlements: %v", calls[0])
	}
	if !containsSequence(calls[1], "--entitlements", entitlements) {
		t.Fatalf("app args should include entitlements: %v", calls[1])
	}
}
