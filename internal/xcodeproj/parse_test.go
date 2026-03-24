package xcodeproj

import (
	"os"
	"path/filepath"
	"testing"
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

func TestParseSimple(t *testing.T) {
	xcodeprojDir := setupTestProj(t, "testdata/simple.pbxproj")

	raw, err := Parse(xcodeprojDir)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if raw.RootObject == "" {
		t.Fatal("rootObject is empty")
	}
	if raw.ObjectVersion != "56" {
		t.Errorf("objectVersion = %q, want %q", raw.ObjectVersion, "56")
	}
	if len(raw.Objects) == 0 {
		t.Fatal("objects table is empty")
	}
}

func TestParseMissingFile(t *testing.T) {
	_, err := Parse("/nonexistent/Test.xcodeproj")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestResolveSimple(t *testing.T) {
	xcodeprojDir := setupTestProj(t, "testdata/simple.pbxproj")

	raw, err := Parse(xcodeprojDir)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	xp, err := Resolve(raw)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(xp.Targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(xp.Targets))
	}

	target := xp.Targets[0]
	if target.Name != "SimpleApp" {
		t.Errorf("name = %q, want %q", target.Name, "SimpleApp")
	}
	if target.ProductType != productTypeApplication {
		t.Errorf("productType = %q", target.ProductType)
	}

	// source files
	if len(target.SourceFiles) != 2 {
		t.Errorf("source files = %v, want 2 files", target.SourceFiles)
	}

	// resource files
	if len(target.ResourceFiles) != 1 {
		t.Errorf("resource files = %v, want 1 file", target.ResourceFiles)
	}

	// build configurations
	if len(target.Configurations) != 2 {
		t.Fatalf("configurations = %d, want 2", len(target.Configurations))
	}

	debug := target.FindConfig("Debug")
	if debug == nil {
		t.Fatal("Debug config not found")
	}
	if debug.Settings["PRODUCT_BUNDLE_IDENTIFIER"] != "com.example.SimpleApp" {
		t.Errorf("bundle id = %q", debug.Settings["PRODUCT_BUNDLE_IDENTIFIER"])
	}
	if debug.Settings["IPHONEOS_DEPLOYMENT_TARGET"] != "17.0" {
		t.Errorf("deployment target = %q", debug.Settings["IPHONEOS_DEPLOYMENT_TARGET"])
	}
	if debug.Settings["MARKETING_VERSION"] != "1.2.0" {
		t.Errorf("version = %q", debug.Settings["MARKETING_VERSION"])
	}
	if debug.Settings["DEVELOPMENT_TEAM"] != "ABC123DEF" {
		t.Errorf("team = %q", debug.Settings["DEVELOPMENT_TEAM"])
	}
}

func TestResolveMulti(t *testing.T) {
	xcodeprojDir := setupTestProj(t, "testdata/multi.pbxproj")

	raw, err := Parse(xcodeprojDir)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	xp, err := Resolve(raw)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(xp.Targets) != 3 {
		t.Fatalf("targets = %d, want 3", len(xp.Targets))
	}

	// verify target names and types
	targets := map[string]string{}
	for _, t := range xp.Targets {
		targets[t.Name] = t.ProductType
	}

	if targets["MultiApp"] != productTypeApplication {
		t.Errorf("MultiApp type = %q", targets["MultiApp"])
	}
	if targets["MultiWidget"] != productTypeAppExtension {
		t.Errorf("MultiWidget type = %q", targets["MultiWidget"])
	}
	if targets["MultiAppTests"] != productTypeUnitTest {
		t.Errorf("MultiAppTests type = %q", targets["MultiAppTests"])
	}

	// verify dependencies
	for _, xt := range xp.Targets {
		switch xt.Name {
		case "MultiWidget":
			if len(xt.Dependencies) != 1 || xt.Dependencies[0] != "MultiApp" {
				t.Errorf("MultiWidget deps = %v, want [MultiApp]", xt.Dependencies)
			}
		case "MultiAppTests":
			if len(xt.Dependencies) != 1 || xt.Dependencies[0] != "MultiApp" {
				t.Errorf("MultiAppTests deps = %v, want [MultiApp]", xt.Dependencies)
			}
		case "MultiApp":
			if len(xt.Dependencies) != 0 {
				t.Errorf("MultiApp deps = %v, want []", xt.Dependencies)
			}
		}
	}

	// verify source file counts
	for _, xt := range xp.Targets {
		switch xt.Name {
		case "MultiApp":
			if len(xt.SourceFiles) != 2 {
				t.Errorf("MultiApp sources = %d, want 2", len(xt.SourceFiles))
			}
		case "MultiWidget":
			if len(xt.SourceFiles) != 1 {
				t.Errorf("MultiWidget sources = %d, want 1", len(xt.SourceFiles))
			}
		case "MultiAppTests":
			if len(xt.SourceFiles) != 1 {
				t.Errorf("MultiAppTests sources = %d, want 1", len(xt.SourceFiles))
			}
		}
	}
}

func TestFindConfigCaseInsensitive(t *testing.T) {
	xt := XcodeTarget{
		Configurations: []BuildConfig{
			{Name: "Debug", Settings: map[string]string{"KEY": "val"}},
			{Name: "Release", Settings: map[string]string{"KEY": "val2"}},
		},
	}

	if c := xt.FindConfig("debug"); c == nil || c.Name != "Debug" {
		t.Error("case-insensitive lookup failed for 'debug'")
	}
	if c := xt.FindConfig("RELEASE"); c == nil || c.Name != "Release" {
		t.Error("case-insensitive lookup failed for 'RELEASE'")
	}
	if c := xt.FindConfig("nonexistent"); c != nil {
		t.Error("expected nil for nonexistent config")
	}
}

func TestSettingFallback(t *testing.T) {
	xt := XcodeTarget{
		Configurations: []BuildConfig{
			{Name: "Debug", Settings: map[string]string{"A": "1"}},
			{Name: "Release", Settings: map[string]string{"A": "2", "B": "3"}},
		},
	}

	// direct match
	if v := xt.Setting("debug", "A"); v != "1" {
		t.Errorf("Setting(debug, A) = %q, want %q", v, "1")
	}

	// fall back across configs
	if v := xt.Setting("debug", "B"); v != "3" {
		t.Errorf("Setting(debug, B) = %q, want %q (fallback)", v, "3")
	}

	// not found anywhere
	if v := xt.Setting("debug", "Z"); v != "" {
		t.Errorf("Setting(debug, Z) = %q, want empty", v)
	}
}

func TestResolvePackageProducts(t *testing.T) {
	raw := &RawProject{
		RootObject: "project",
		Objects: map[string]map[string]any{
			"project": {
				"isa":     isaPBXProject,
				"targets": []any{"target"},
			},
			"target": {
				"isa":                        isaPBXNativeTarget,
				"name":                       "App",
				"productType":                productTypeApplication,
				"buildConfigurationList":     "configs",
				"packageProductDependencies": []any{"pkg"},
			},
			"configs": {
				"isa":                 isaXCConfigurationList,
				"buildConfigurations": []any{"debug"},
			},
			"debug": {
				"isa":           isaXCBuildConfiguration,
				"name":          "Debug",
				"buildSettings": map[string]any{},
			},
			"pkg": {
				"isa":         isaXCSwiftPackageProductDependency,
				"productName": "WeatherKit",
			},
		},
	}

	xp, err := Resolve(raw)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(xp.Targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(xp.Targets))
	}
	if len(xp.Targets[0].PackageProducts) != 1 || xp.Targets[0].PackageProducts[0] != "WeatherKit" {
		t.Fatalf("package products = %v", xp.Targets[0].PackageProducts)
	}
}
