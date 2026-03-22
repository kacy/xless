package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyOverlayDefaults(t *testing.T) {
	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "xless.yml")
	os.WriteFile(overlayPath, []byte(`
defaults:
  target: "Widget"
  config: "release"
  simulator: "iPhone 15 Pro"
  device: "my-iphone"
`), 0o644)

	cfg := &ProjectConfig{
		Targets: []TargetConfig{
			{Name: "App", BundleID: "com.test.app"},
			{Name: "Widget", BundleID: "com.test.widget"},
		},
	}

	warnings, err := ApplyOverlay(cfg, overlayPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	if cfg.Defaults.Target != "Widget" {
		t.Errorf("target = %q", cfg.Defaults.Target)
	}
	if cfg.Defaults.Config != "release" {
		t.Errorf("config = %q", cfg.Defaults.Config)
	}
	if cfg.Defaults.Simulator != "iPhone 15 Pro" {
		t.Errorf("simulator = %q", cfg.Defaults.Simulator)
	}
	if cfg.Defaults.Device != "my-iphone" {
		t.Errorf("device = %q", cfg.Defaults.Device)
	}
}

func TestApplyOverlaySigningReplace(t *testing.T) {
	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "xless.yml")
	os.WriteFile(overlayPath, []byte(`
overrides:
  targets:
    App:
      signing:
        identity: "Apple Development: kacy@example.com"
        team_id: "NEW_TEAM"
`), 0o644)

	cfg := &ProjectConfig{
		Targets: []TargetConfig{
			{
				Name:     "App",
				BundleID: "com.test.app",
				Signing: SigningConfig{
					Identity: "old-identity",
					TeamID:   "OLD_TEAM",
					Entitlements: "old.entitlements",
				},
			},
		},
	}

	warnings, err := ApplyOverlay(cfg, overlayPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// signing should be fully replaced
	if cfg.Targets[0].Signing.Identity != "Apple Development: kacy@example.com" {
		t.Errorf("identity = %q", cfg.Targets[0].Signing.Identity)
	}
	if cfg.Targets[0].Signing.TeamID != "NEW_TEAM" {
		t.Errorf("team_id = %q", cfg.Targets[0].Signing.TeamID)
	}
	// entitlements should be empty (full replace, not merge)
	if cfg.Targets[0].Signing.Entitlements != "" {
		t.Errorf("entitlements = %q, should be empty after full replace", cfg.Targets[0].Signing.Entitlements)
	}
}

func TestApplyOverlaySwiftFlagsAppend(t *testing.T) {
	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "xless.yml")
	os.WriteFile(overlayPath, []byte(`
overrides:
  targets:
    App:
      swift_flags: ["-DXLESS_BUILD", "-DCUSTOM"]
`), 0o644)

	cfg := &ProjectConfig{
		Targets: []TargetConfig{
			{
				Name:       "App",
				BundleID:   "com.test.app",
				SwiftFlags: []string{"-DDEBUG"},
			},
		},
	}

	_, err := ApplyOverlay(cfg, overlayPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// flags should be appended
	want := []string{"-DDEBUG", "-DXLESS_BUILD", "-DCUSTOM"}
	got := cfg.Targets[0].SwiftFlags
	if len(got) != len(want) {
		t.Fatalf("swift_flags = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("swift_flags[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestApplyOverlayMinIOSReplace(t *testing.T) {
	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "xless.yml")
	os.WriteFile(overlayPath, []byte(`
overrides:
  targets:
    App:
      min_ios: "17.0"
`), 0o644)

	cfg := &ProjectConfig{
		Targets: []TargetConfig{
			{
				Name:     "App",
				BundleID: "com.test.app",
				MinIOS:   "16.0",
			},
		},
	}

	_, err := ApplyOverlay(cfg, overlayPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Targets[0].MinIOS != "17.0" {
		t.Errorf("min_ios = %q, want %q", cfg.Targets[0].MinIOS, "17.0")
	}
}

func TestApplyOverlayUnknownTargetWarning(t *testing.T) {
	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "xless.yml")
	os.WriteFile(overlayPath, []byte(`
overrides:
  targets:
    NonExistent:
      min_ios: "17.0"
`), 0o644)

	cfg := &ProjectConfig{
		Targets: []TargetConfig{
			{Name: "App", BundleID: "com.test.app"},
		},
	}

	warnings, err := ApplyOverlay(cfg, overlayPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want 1 warning", warnings)
	}
	if warnings[0] == "" {
		t.Error("expected non-empty warning")
	}
}

func TestApplyOverlayInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "xless.yml")
	os.WriteFile(overlayPath, []byte("{{invalid"), 0o644)

	cfg := &ProjectConfig{}
	_, err := ApplyOverlay(cfg, overlayPath)
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestApplyOverlayPartialDefaults(t *testing.T) {
	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "xless.yml")
	os.WriteFile(overlayPath, []byte(`
defaults:
  simulator: "iPhone 15"
`), 0o644)

	cfg := &ProjectConfig{
		Defaults: DefaultsConfig{
			Target:    "App",
			Config:    "debug",
			Simulator: "iPhone 16 Pro",
			Device:    "my-device",
		},
	}

	_, err := ApplyOverlay(cfg, overlayPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// only simulator should change
	if cfg.Defaults.Simulator != "iPhone 15" {
		t.Errorf("simulator = %q", cfg.Defaults.Simulator)
	}
	if cfg.Defaults.Target != "App" {
		t.Errorf("target changed to %q", cfg.Defaults.Target)
	}
	if cfg.Defaults.Config != "debug" {
		t.Errorf("config changed to %q", cfg.Defaults.Config)
	}
	if cfg.Defaults.Device != "my-device" {
		t.Errorf("device changed to %q", cfg.Defaults.Device)
	}
}
