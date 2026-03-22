package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kacyfortner/ios-build-cli/internal/xcodeproj"
)

func setupTestProj(t *testing.T, fixture string) string {
	t.Helper()
	dir := t.TempDir()
	xcodeprojDir := filepath.Join(dir, "Test.xcodeproj")
	os.Mkdir(xcodeprojDir, 0o755)

	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	os.WriteFile(filepath.Join(xcodeprojDir, "project.pbxproj"), data, 0o644)

	return xcodeprojDir
}

func TestConvertSimple(t *testing.T) {
	xcodeprojDir := setupTestProj(t, "../xcodeproj/testdata/simple.pbxproj")

	raw, err := xcodeproj.Parse(xcodeprojDir)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	xp, err := xcodeproj.Resolve(raw)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	cfg := ConvertXcodeProject(xp, "debug")

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
	if tc.Version != "1.2.0" {
		t.Errorf("version = %q", tc.Version)
	}
	if tc.Signing.TeamID != "ABC123DEF" {
		t.Errorf("team_id = %q", tc.Signing.TeamID)
	}
	if tc.ProductType != ProductApp {
		t.Errorf("product_type = %q", tc.ProductType)
	}
}

func TestConvertMulti(t *testing.T) {
	xcodeprojDir := setupTestProj(t, "../xcodeproj/testdata/multi.pbxproj")

	raw, err := xcodeproj.Parse(xcodeprojDir)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	xp, err := xcodeproj.Resolve(raw)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	cfg := ConvertXcodeProject(xp, "debug")

	if len(cfg.Targets) != 3 {
		t.Fatalf("targets = %d, want 3", len(cfg.Targets))
	}

	app := cfg.FindTarget("MultiApp")
	if app == nil {
		t.Fatal("MultiApp target not found")
	}
	if app.ProductType != ProductApp {
		t.Errorf("MultiApp type = %q", app.ProductType)
	}
	if app.BundleID != "com.example.MultiApp" {
		t.Errorf("MultiApp bundle_id = %q", app.BundleID)
	}
	if app.MinIOS != "16.0" {
		t.Errorf("MultiApp min_ios = %q", app.MinIOS)
	}

	// swift flags should strip $(inherited)
	if len(app.SwiftFlags) != 2 {
		t.Errorf("MultiApp swift_flags = %v, want [-DDEBUG, -DFEATURE_FLAG]", app.SwiftFlags)
	}

	widget := cfg.FindTarget("MultiWidget")
	if widget == nil {
		t.Fatal("MultiWidget not found")
	}
	if widget.ProductType != ProductAppExtension {
		t.Errorf("MultiWidget type = %q", widget.ProductType)
	}

	tests := cfg.FindTarget("MultiAppTests")
	if tests == nil {
		t.Fatal("MultiAppTests not found")
	}
	if tests.ProductType != ProductUnitTest {
		t.Errorf("MultiAppTests type = %q", tests.ProductType)
	}
}

func TestConvertReleaseConfig(t *testing.T) {
	xcodeprojDir := setupTestProj(t, "../xcodeproj/testdata/simple.pbxproj")

	raw, err := xcodeproj.Parse(xcodeprojDir)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	xp, err := xcodeproj.Resolve(raw)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	cfg := ConvertXcodeProject(xp, "release")

	tc := cfg.Targets[0]
	if tc.Signing.Identity != "Apple Distribution" {
		t.Errorf("release signing identity = %q, want %q", tc.Signing.Identity, "Apple Distribution")
	}
}

func TestSplitFlags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"$(inherited) -DDEBUG -DFEATURE", []string{"-DDEBUG", "-DFEATURE"}},
		{"-DSINGLE", []string{"-DSINGLE"}},
		{"$(inherited)", nil},
	}

	for _, tt := range tests {
		got := splitFlags(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitFlags(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitFlags(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
