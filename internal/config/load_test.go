package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNative(t *testing.T) {
	dir := t.TempDir()

	yml := `
project:
  name: "TestApp"
  bundle_id: "com.test.TestApp"
  version: "2.0.0"
  build_number: "42"

build:
  sources: ["Sources/", "Lib/"]
  min_ios: "17.0"
  swift_flags: ["-DDEBUG"]

signing:
  identity: "Apple Development"
  team_id: "ABC123"

defaults:
  simulator: "iPhone 15"
  config: "release"
`
	os.WriteFile(filepath.Join(dir, "xless.yml"), []byte(yml), 0o644)

	cfg, det, err := Load(dir, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Mode.String() != "native" {
		t.Errorf("mode = %v, want native", det.Mode)
	}

	if cfg.Project.Name != "TestApp" {
		t.Errorf("project name = %q, want %q", cfg.Project.Name, "TestApp")
	}

	if len(cfg.Targets) != 1 {
		t.Fatalf("targets count = %d, want 1", len(cfg.Targets))
	}

	target := cfg.Targets[0]
	if target.BundleID != "com.test.TestApp" {
		t.Errorf("bundle_id = %q", target.BundleID)
	}
	if target.Version != "2.0.0" {
		t.Errorf("version = %q", target.Version)
	}
	if target.BuildNum != "42" {
		t.Errorf("build_number = %q", target.BuildNum)
	}
	if target.MinIOS != "17.0" {
		t.Errorf("min_ios = %q", target.MinIOS)
	}
	if len(target.Sources) != 2 {
		t.Errorf("sources = %v", target.Sources)
	}
	if len(target.SwiftFlags) != 1 || target.SwiftFlags[0] != "-DDEBUG" {
		t.Errorf("swift_flags = %v", target.SwiftFlags)
	}
	if target.Signing.Identity != "Apple Development" {
		t.Errorf("signing.identity = %q", target.Signing.Identity)
	}
	if target.Signing.TeamID != "ABC123" {
		t.Errorf("signing.team_id = %q", target.Signing.TeamID)
	}

	if cfg.Defaults.Simulator != "iPhone 15" {
		t.Errorf("defaults.simulator = %q", cfg.Defaults.Simulator)
	}
	if cfg.Defaults.Config != "release" {
		t.Errorf("defaults.config = %q", cfg.Defaults.Config)
	}
}

func TestLoadNativeDefaults(t *testing.T) {
	dir := t.TempDir()

	// minimal valid config — defaults should fill in gaps
	yml := `
project:
  name: "MinApp"
  bundle_id: "com.test.MinApp"
`
	os.WriteFile(filepath.Join(dir, "xless.yml"), []byte(yml), 0o644)

	cfg, _, err := Load(dir, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	target := cfg.Targets[0]
	if target.MinIOS != DefaultMinIOS {
		t.Errorf("min_ios = %q, want %q", target.MinIOS, DefaultMinIOS)
	}
	if target.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", target.Version, "1.0.0")
	}
	if target.BuildNum != "1" {
		t.Errorf("build_number = %q", target.BuildNum)
	}
	if len(target.Sources) != 1 || target.Sources[0] != "Sources/" {
		t.Errorf("sources = %v", target.Sources)
	}
	if cfg.Defaults.Config != DefaultConfig {
		t.Errorf("defaults.config = %q, want %q", cfg.Defaults.Config, DefaultConfig)
	}
	if cfg.Defaults.Simulator != DefaultSimulator {
		t.Errorf("defaults.simulator = %q, want %q", cfg.Defaults.Simulator, DefaultSimulator)
	}
}

func TestLoadCLIFlagsOverride(t *testing.T) {
	dir := t.TempDir()

	yml := `
project:
  name: "FlagApp"
  bundle_id: "com.test.FlagApp"

defaults:
  config: "debug"
  simulator: "iPhone 15"
`
	os.WriteFile(filepath.Join(dir, "xless.yml"), []byte(yml), 0o644)

	flags := CLIFlags{
		Target: "FlagApp",
		Config: "release",
		Device: "my-iphone",
	}
	cfg, _, err := Load(dir, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Defaults.Target != "FlagApp" {
		t.Errorf("defaults.target = %q, want %q", cfg.Defaults.Target, "FlagApp")
	}
	if cfg.Defaults.Config != "release" {
		t.Errorf("defaults.config = %q, want %q", cfg.Defaults.Config, "release")
	}
	if cfg.Defaults.Device != "my-iphone" {
		t.Errorf("defaults.device = %q", cfg.Defaults.Device)
	}
	// simulator should not be overridden since flag was empty
	if cfg.Defaults.Simulator != "iPhone 15" {
		t.Errorf("defaults.simulator = %q, want %q", cfg.Defaults.Simulator, "iPhone 15")
	}
}

func TestLoadNoProject(t *testing.T) {
	dir := t.TempDir()
	_, _, err := Load(dir, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "xless.yml"), []byte("{{invalid"), 0o644)
	_, _, err := Load(dir, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestLoadMissingProjectName(t *testing.T) {
	dir := t.TempDir()
	yml := `
project:
  bundle_id: "com.test.NoName"
`
	os.WriteFile(filepath.Join(dir, "xless.yml"), []byte(yml), 0o644)
	_, _, err := Load(dir, CLIFlags{})
	if err == nil {
		t.Fatal("expected validation error for missing project name")
	}
}

func TestLoadXcodeproj(t *testing.T) {
	dir := t.TempDir()

	// create a .xcodeproj with the simple fixture
	xcodeprojDir := filepath.Join(dir, "SimpleApp.xcodeproj")
	os.Mkdir(xcodeprojDir, 0o755)
	data, err := os.ReadFile("../xcodeproj/testdata/simple.pbxproj")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	os.WriteFile(filepath.Join(xcodeprojDir, "project.pbxproj"), data, 0o644)

	cfg, det, err := Load(dir, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Mode.String() != "xcodeproj" {
		t.Errorf("mode = %v, want xcodeproj", det.Mode)
	}

	if len(cfg.Targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(cfg.Targets))
	}

	tc := cfg.Targets[0]
	if tc.Name != "SimpleApp" {
		t.Errorf("name = %q", tc.Name)
	}
	if tc.BundleID != "com.example.SimpleApp" {
		t.Errorf("bundle_id = %q", tc.BundleID)
	}
	if tc.MinIOS != "17.0" {
		t.Errorf("min_ios = %q", tc.MinIOS)
	}
}

func TestLoadXcodeprojWithOverlay(t *testing.T) {
	dir := t.TempDir()

	// xcodeproj
	xcodeprojDir := filepath.Join(dir, "SimpleApp.xcodeproj")
	os.Mkdir(xcodeprojDir, 0o755)
	data, err := os.ReadFile("../xcodeproj/testdata/simple.pbxproj")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	os.WriteFile(filepath.Join(xcodeprojDir, "project.pbxproj"), data, 0o644)

	// overlay
	overlay := `
defaults:
  simulator: "iPhone 15"
  config: "release"

overrides:
  targets:
    SimpleApp:
      signing:
        identity: "Custom Identity"
        team_id: "CUSTOM_TEAM"
      swift_flags: ["-DOVERLAY"]
      min_ios: "18.0"
`
	os.WriteFile(filepath.Join(dir, "xless.yml"), []byte(overlay), 0o644)

	cfg, det, err := Load(dir, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Mode.String() != "xcodeproj" {
		t.Errorf("mode = %v, want xcodeproj", det.Mode)
	}

	tc := cfg.Targets[0]
	if tc.Signing.Identity != "Custom Identity" {
		t.Errorf("signing.identity = %q", tc.Signing.Identity)
	}
	if tc.Signing.TeamID != "CUSTOM_TEAM" {
		t.Errorf("signing.team_id = %q", tc.Signing.TeamID)
	}
	if tc.MinIOS != "18.0" {
		t.Errorf("min_ios = %q, want 18.0", tc.MinIOS)
	}

	// swift flags should include the overlay flag
	found := false
	for _, f := range tc.SwiftFlags {
		if f == "-DOVERLAY" {
			found = true
		}
	}
	if !found {
		t.Errorf("swift_flags = %v, missing -DOVERLAY", tc.SwiftFlags)
	}

	if cfg.Defaults.Simulator != "iPhone 15" {
		t.Errorf("defaults.simulator = %q", cfg.Defaults.Simulator)
	}
}
