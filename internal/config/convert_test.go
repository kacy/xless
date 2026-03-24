package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kacy/xless/internal/xcodeproj"
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

func TestConvertUnsupportedCapabilities(t *testing.T) {
	xt := xcodeproj.XcodeTarget{
		Name:          "ComplexApp",
		ProductType:   "com.apple.product-type.application",
		SourceFiles:   []string{"Sources/App.swift", "Legacy/AppDelegate.m"},
		ResourceFiles: []string{"Assets.xcassets", "Main.storyboard", "Model.xcdatamodeld"},
		LinkInputs: []xcodeproj.LinkInput{
			{Path: "MapKit.framework", SourceTree: "SDKROOT"},
			{Path: "libsqlite3.tbd", SourceTree: "SDKROOT"},
			{Path: "Vendor/WeatherKit.framework", SourceTree: "<group>"},
		},
		PackageProducts: []string{"WeatherKit"},
		PackageReferences: []string{
			"remote https://example.com/weatherkit.git",
		},
		ShellScriptPhases: []string{"Generate Assets"},
		CopyFilesPhases:   []string{"Embed Support Files"},
		Configurations: []xcodeproj.BuildConfig{
			{
				Name: "Debug",
				Settings: map[string]string{
					"PRODUCT_BUNDLE_IDENTIFIER":  "com.example.ComplexApp",
					"IPHONEOS_DEPLOYMENT_TARGET": "17.0",
					"OTHER_LDFLAGS":              "$(inherited) -ObjC -framework MapKit",
					"FRAMEWORK_SEARCH_PATHS":     "$(inherited) Vendor/Frameworks",
					"LIBRARY_SEARCH_PATHS":       "Vendor/Libraries",
					"SWIFT_OBJC_BRIDGING_HEADER": "ComplexApp-Bridging-Header.h",
				},
			},
		},
	}

	cfg := ConvertXcodeProject(&xcodeproj.XcodeProject{
		Name:    "ComplexApp",
		Targets: []xcodeproj.XcodeTarget{xt},
	}, "debug")

	target := cfg.Targets[0]
	if len(target.Frameworks) != 1 || target.Frameworks[0] != "MapKit.framework" {
		t.Fatalf("frameworks = %v", target.Frameworks)
	}
	if len(target.Libraries) != 1 || target.Libraries[0] != "sqlite3" {
		t.Fatalf("libraries = %v", target.Libraries)
	}
	if len(target.LinkerFlags) != 1 || target.LinkerFlags[0] != "-ObjC" {
		t.Fatalf("linker flags = %v", target.LinkerFlags)
	}
	if len(target.FrameworkSearchPaths) != 1 || target.FrameworkSearchPaths[0] != "Vendor/Frameworks" {
		t.Fatalf("framework search paths = %v", target.FrameworkSearchPaths)
	}
	if len(target.LibrarySearchPaths) != 1 || target.LibrarySearchPaths[0] != "Vendor/Libraries" {
		t.Fatalf("library search paths = %v", target.LibrarySearchPaths)
	}
	if len(target.PackageRefs) != 1 || target.PackageRefs[0] != "remote https://example.com/weatherkit.git" {
		t.Fatalf("package refs = %v", target.PackageRefs)
	}

	unsupported := strings.Join(target.Unsupported, "\n")
	for _, want := range []string{
		"non-swift source file Legacy/AppDelegate.m",
		"asset catalog resource Assets.xcassets",
		"Interface Builder resource Main.storyboard",
		"Core Data model resource Model.xcdatamodeld",
		"Objective-C bridging header ComplexApp-Bridging-Header.h",
		"non-SDK framework dependency Vendor/WeatherKit.framework",
		"custom linker flags: -ObjC",
		"framework search paths: Vendor/Frameworks",
		"library search paths: Vendor/Libraries",
		"shell script build phases: Generate Assets",
		"copy files build phases: Embed Support Files",
	} {
		if !strings.Contains(unsupported, want) {
			t.Fatalf("unsupported = %q, want %q", unsupported, want)
		}
	}
}

func TestSplitBuildSettingList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"$(inherited) Vendor/Frameworks Vendor/More", []string{"Vendor/Frameworks", "Vendor/More"}},
		{"-ObjC -framework MapKit", []string{"-ObjC", "-framework", "MapKit"}},
	}

	for _, tt := range tests {
		got := splitBuildSettingList(tt.input)
		if len(got) != len(tt.want) {
			t.Fatalf("splitBuildSettingList(%q) = %v, want %v", tt.input, got, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("splitBuildSettingList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestClassifyLinkerFlags(t *testing.T) {
	frameworks, unsupported := classifyLinkerFlags([]string{"-framework", "MapKit", "-ObjC", "-framework", "StoreKit"})

	if len(frameworks) != 2 || frameworks[0] != "MapKit.framework" || frameworks[1] != "StoreKit.framework" {
		t.Fatalf("frameworks = %v", frameworks)
	}
	if len(unsupported) != 1 || unsupported[0] != "-ObjC" {
		t.Fatalf("unsupported = %v", unsupported)
	}
}

func TestClassifyLibraryFlags(t *testing.T) {
	libraries, unsupported := classifyLibraryFlags([]string{"-lsqlite3", "-l", "z", "-ObjC"})

	if len(libraries) != 2 || libraries[0] != "sqlite3" || libraries[1] != "z" {
		t.Fatalf("libraries = %v", libraries)
	}
	if len(unsupported) != 1 || unsupported[0] != "-ObjC" {
		t.Fatalf("unsupported = %v", unsupported)
	}
}
