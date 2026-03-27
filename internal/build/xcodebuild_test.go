package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kacy/xless/internal/config"
	"github.com/kacy/xless/internal/toolchain"
)

func TestXcodebuildArgsWorkspace(t *testing.T) {
	bc := &BuildContext{
		WorkspaceDir: "/tmp/App.xcworkspace",
		BuildDir:     "/tmp/.build/App",
		BuildConfig:  "release",
		Platform:     toolchain.PlatformSimulator,
		Target:       &config.TargetConfig{Name: "Weather"},
	}

	args := xcodebuildArgs(bc, xcodebuildSelector{flag: "-scheme", value: "WeatherApp"})
	assertContains(t, args, "-workspace", "/tmp/App.xcworkspace")
	assertContains(t, args, "-scheme", "WeatherApp")
	assertContains(t, args, "-configuration", "Release")
	assertContains(t, args, "-sdk", "iphonesimulator")
	assertContains(t, args, "-destination", "generic/platform=iOS Simulator")
	assertContainsValue(t, args, "CODE_SIGNING_ALLOWED=NO")
}

func TestXcodebuildArgsProjectDevice(t *testing.T) {
	bc := &BuildContext{
		XcodeprojDir: "/tmp/App.xcodeproj",
		BuildDir:     "/tmp/.build/App",
		BuildConfig:  "debug",
		Platform:     toolchain.PlatformDevice,
		Target:       &config.TargetConfig{Name: "Weather"},
	}

	args := xcodebuildArgs(bc, xcodebuildSelector{flag: "-scheme", value: "Weather"})
	assertContains(t, args, "-project", "/tmp/App.xcodeproj")
	assertContains(t, args, "-scheme", "Weather")
	assertContains(t, args, "-configuration", "Debug")
	assertContains(t, args, "-sdk", "iphoneos")
	assertContains(t, args, "-destination", "generic/platform=iOS")
	assertContains(t, args, "-derivedDataPath", "/tmp/.build/App/XcodeBuild")
	assertNotContainsValue(t, args, "SYMROOT=/tmp/.build/App/XcodeBuild/Build/Products")
	assertNotContainsValue(t, args, "OBJROOT=/tmp/.build/App/XcodeBuild/Build/Intermediates.noindex")
	assertNotContainsValue(t, args, "CODE_SIGNING_ALLOWED=NO")
}

func TestSelectBuildSettingsPrefersTargetName(t *testing.T) {
	entries := []xcodebuildSettingsEntry{
		{Target: "Support", BuildSettings: map[string]string{"TARGET_NAME": "Support", "FULL_PRODUCT_NAME": "Support.app"}},
		{Target: "Weather", BuildSettings: map[string]string{"TARGET_NAME": "Weather", "FULL_PRODUCT_NAME": "Weather.app"}},
	}

	settings := selectBuildSettings(entries, "Weather")
	if settings == nil || settings["TARGET_NAME"] != "Weather" {
		t.Fatalf("settings = %#v", settings)
	}
}

func TestResolveXcodebuildSelectorWorkspaceUsesExactScheme(t *testing.T) {
	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	runXcodebuild = func(_ context.Context, args ...string) (*toolchain.CommandResult, error) {
		if !containsSequence(args, "-workspace", "/tmp/App.xcworkspace") {
			t.Fatalf("args = %v", args)
		}
		return &toolchain.CommandResult{Stdout: `{"workspace":{"name":"App","schemes":["Weather","Widget"]}}`}, nil
	}

	selector, err := resolveXcodebuildSelector(&BuildContext{
		Ctx:          context.Background(),
		WorkspaceDir: "/tmp/App.xcworkspace",
		Target:       &config.TargetConfig{Name: "Weather"},
	})
	if err != nil {
		t.Fatalf("resolveXcodebuildSelector: %v", err)
	}
	if selector.flag != "-scheme" || selector.value != "Weather" {
		t.Fatalf("selector = %+v", selector)
	}
}

func TestResolveXcodebuildSelectorHonorsExplicitScheme(t *testing.T) {
	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	runXcodebuild = func(_ context.Context, _ ...string) (*toolchain.CommandResult, error) {
		return &toolchain.CommandResult{Stdout: `{"workspace":{"name":"App","schemes":["App iOS","Widget"]}}`}, nil
	}

	selector, err := resolveXcodebuildSelector(&BuildContext{
		Ctx:          context.Background(),
		WorkspaceDir: "/tmp/App.xcworkspace",
		XcodeScheme:  "App iOS",
		Target:       &config.TargetConfig{Name: "Weather"},
	})
	if err != nil {
		t.Fatalf("resolveXcodebuildSelector: %v", err)
	}
	if selector.flag != "-scheme" || selector.value != "App iOS" {
		t.Fatalf("selector = %+v", selector)
	}
}

func TestResolveXcodebuildSelectionReturnsFormattedSelector(t *testing.T) {
	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	runXcodebuild = func(_ context.Context, _ ...string) (*toolchain.CommandResult, error) {
		return &toolchain.CommandResult{Stdout: `{"project":{"name":"App","targets":["Weather"],"schemes":["Weather"]}}`}, nil
	}

	selection, err := ResolveXcodebuildSelection(context.Background(), "", "/tmp/App.xcodeproj", "Weather", "")
	if err != nil {
		t.Fatalf("ResolveXcodebuildSelection: %v", err)
	}
	if selection.Scheme != "Weather" {
		t.Fatalf("scheme = %q", selection.Scheme)
	}
	if selection.Selector() != "-scheme=Weather" {
		t.Fatalf("selector = %q", selection.Selector())
	}
}

func TestResolveXcodebuildSelectorWorkspaceFallsBackToSingleScheme(t *testing.T) {
	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	runXcodebuild = func(_ context.Context, _ ...string) (*toolchain.CommandResult, error) {
		return &toolchain.CommandResult{Stdout: `{"workspace":{"name":"App","schemes":["App iOS"]}}`}, nil
	}

	selector, err := resolveXcodebuildSelector(&BuildContext{
		Ctx:          context.Background(),
		WorkspaceDir: "/tmp/App.xcworkspace",
		Target:       &config.TargetConfig{Name: "Weather"},
	})
	if err != nil {
		t.Fatalf("resolveXcodebuildSelector: %v", err)
	}
	if selector.flag != "-scheme" || selector.value != "App iOS" {
		t.Fatalf("selector = %+v", selector)
	}
}

func TestResolveXcodebuildSelectorProjectUsesExactScheme(t *testing.T) {
	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	runXcodebuild = func(_ context.Context, _ ...string) (*toolchain.CommandResult, error) {
		return &toolchain.CommandResult{Stdout: `{"project":{"name":"App","targets":["Weather"],"schemes":["Weather"]}}`}, nil
	}

	selector, err := resolveXcodebuildSelector(&BuildContext{
		Ctx:          context.Background(),
		XcodeprojDir: "/tmp/App.xcodeproj",
		Target:       &config.TargetConfig{Name: "Weather"},
	})
	if err != nil {
		t.Fatalf("resolveXcodebuildSelector: %v", err)
	}
	if selector.flag != "-scheme" || selector.value != "Weather" {
		t.Fatalf("selector = %+v", selector)
	}
}

func TestResolveXcodebuildSelectorProjectRequiresSharedScheme(t *testing.T) {
	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	runXcodebuild = func(_ context.Context, _ ...string) (*toolchain.CommandResult, error) {
		return &toolchain.CommandResult{Stdout: `{"project":{"name":"App","targets":["Weather"],"schemes":[]}}`}, nil
	}

	_, err := resolveXcodebuildSelector(&BuildContext{
		Ctx:          context.Background(),
		XcodeprojDir: "/tmp/App.xcodeproj",
		Target:       &config.TargetConfig{Name: "Weather"},
	})
	if err == nil || !strings.Contains(err.Error(), "no shared Xcode schemes") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveXcodebuildSelectorListFailureReturnsHelpfulError(t *testing.T) {
	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	runXcodebuild = func(_ context.Context, _ ...string) (*toolchain.CommandResult, error) {
		return &toolchain.CommandResult{Stderr: "xcodebuild: error: '/tmp/App.xcodeproj' is not a project file."}, fmt.Errorf("exit status 66")
	}

	_, err := resolveXcodebuildSelector(&BuildContext{
		Ctx:          context.Background(),
		XcodeprojDir: "/tmp/App.xcodeproj",
		Target:       &config.TargetConfig{Name: "Weather"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var selectionErr *xcodebuildSelectionError
	if !errors.As(err, &selectionErr) {
		t.Fatalf("expected xcodebuildSelectionError, got %T", err)
	}
	if selectionErr.hint == "" {
		t.Fatal("expected hint")
	}
	if got := XcodebuildSelectionHint(err); got == "" {
		t.Fatal("expected exported selection hint")
	}
}

func TestXcodebuildSelectionHintReturnsEmptyForOtherErrors(t *testing.T) {
	if got := XcodebuildSelectionHint(fmt.Errorf("plain error")); got != "" {
		t.Fatalf("hint = %q, want empty", got)
	}
}

func TestXcodebuildBuildStageUsesShowBuildSettingsHint(t *testing.T) {
	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	callIndex := 0
	runXcodebuild = func(_ context.Context, _ ...string) (*toolchain.CommandResult, error) {
		callIndex++
		if callIndex == 1 {
			return &toolchain.CommandResult{Stdout: `{"project":{"name":"App","targets":["Weather"],"schemes":["Weather"]}}`}, nil
		}
		return &toolchain.CommandResult{
			Stderr: "xcodebuild: error: Unable to find a destination matching the provided destination specifier",
		}, fmt.Errorf("exit status 70")
	}

	bc := &BuildContext{
		Ctx:          context.Background(),
		XcodeprojDir: "/tmp/App.xcodeproj",
		BuildDir:     "/tmp/.build/App",
		BuildConfig:  "debug",
		Platform:     toolchain.PlatformSimulator,
		Target:       &config.TargetConfig{Name: "Weather"},
		Out:          noopFormatter{},
	}

	err := (XcodebuildBuildStage{}).Run(bc)
	if err == nil {
		t.Fatal("expected error")
	}
	var buildErr *BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("expected BuildError, got %T", err)
	}
	if !strings.Contains(buildErr.Hint, "destination") {
		t.Fatalf("hint = %q", buildErr.Hint)
	}
}

func TestXcodebuildBuildStageCopiesAppBundle(t *testing.T) {
	dir := t.TempDir()
	derivedProduct := filepath.Join(dir, "DerivedData", "Build", "Products", "Debug-iphonesimulator", "Weather.app")
	writeFile(t, filepath.Join(derivedProduct, "Weather"), "binary")
	writeFile(t, filepath.Join(derivedProduct, "Info.plist"), "<plist/>")

	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	callIndex := 0
	runXcodebuild = func(_ context.Context, args ...string) (*toolchain.CommandResult, error) {
		callIndex++
		if callIndex == 1 {
			return &toolchain.CommandResult{Stdout: `{"project":{"name":"Weather","targets":["Weather"],"schemes":["Weather"]}}`}, nil
		}
		if callIndex == 2 {
			stdout := `[{"target":"Weather","buildSettings":{"TARGET_BUILD_DIR":"` + filepath.Dir(derivedProduct) + `","FULL_PRODUCT_NAME":"Weather.app","TARGET_NAME":"Weather"}}]`
			return &toolchain.CommandResult{Stdout: stdout}, nil
		}
		if !containsValue(args, "build") {
			t.Fatalf("expected build invocation, got %v", args)
		}
		return &toolchain.CommandResult{}, nil
	}

	bc := &BuildContext{
		Ctx:          context.Background(),
		XcodeprojDir: filepath.Join(dir, "Weather.xcodeproj"),
		BuildDir:     filepath.Join(dir, ".build", "Weather"),
		BuildConfig:  "debug",
		Platform:     toolchain.PlatformSimulator,
		Target:       &config.TargetConfig{Name: "Weather"},
		Out:          noopFormatter{},
	}

	if err := (XcodebuildBuildStage{}).Run(bc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if bc.AppBundlePath == "" {
		t.Fatal("expected app bundle path")
	}
	data, err := os.ReadFile(filepath.Join(bc.AppBundlePath, "Weather"))
	if err != nil {
		t.Fatalf("read copied app: %v", err)
	}
	if string(data) != "binary" {
		t.Fatalf("copied binary = %q", string(data))
	}
}

func TestXcodebuildBuildStageDeviceArchivesAndExportsIPA(t *testing.T) {
	dir := t.TempDir()
	exportedIPA := filepath.Join(dir, ".build", "Weather", "Export", "Weather.ipa")

	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	callIndex := 0
	runXcodebuild = func(_ context.Context, args ...string) (*toolchain.CommandResult, error) {
		callIndex++
		switch callIndex {
		case 1:
			return &toolchain.CommandResult{Stdout: `{"project":{"name":"Weather","targets":["Weather"],"schemes":["Weather"]}}`}, nil
		case 2:
			stdout := `[{"target":"Weather","buildSettings":{"FULL_PRODUCT_NAME":"Weather.app","TARGET_NAME":"Weather"}}]`
			return &toolchain.CommandResult{Stdout: stdout}, nil
		case 3:
			assertContainsValue(t, args, "archive")
			assertContains(t, args, "-archivePath", filepath.Join(dir, ".build", "Weather", "Weather.xcarchive"))
			archivedApp := filepath.Join(dir, ".build", "Weather", "Weather.xcarchive", "Products", "Applications", "Weather.app")
			writeFile(t, filepath.Join(archivedApp, "Weather"), "binary")
			writeFile(t, filepath.Join(archivedApp, "Info.plist"), "<plist/>")
			return &toolchain.CommandResult{}, nil
		case 4:
			assertContainsValue(t, args, "-exportArchive")
			assertContains(t, args, "-exportPath", filepath.Join(dir, ".build", "Weather", "Export"))
			writeFile(t, exportedIPA, "ipa")
			return &toolchain.CommandResult{}, nil
		default:
			t.Fatalf("unexpected xcodebuild call %d: %v", callIndex, args)
			return nil, nil
		}
	}

	bc := &BuildContext{
		Ctx:          context.Background(),
		XcodeprojDir: filepath.Join(dir, "Weather.xcodeproj"),
		BuildDir:     filepath.Join(dir, ".build", "Weather"),
		BuildConfig:  "debug",
		Platform:     toolchain.PlatformDevice,
		Target: &config.TargetConfig{
			Name: "Weather",
			Signing: config.SigningConfig{
				TeamID: "TEAM123",
			},
		},
		Out: noopFormatter{},
	}

	if err := (XcodebuildBuildStage{}).Run(bc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if bc.AppBundlePath == "" {
		t.Fatal("expected app bundle path")
	}
	if bc.IPAPath != exportedIPA {
		t.Fatalf("IPAPath = %q, want %q", bc.IPAPath, exportedIPA)
	}
	exportOptionsData, err := os.ReadFile(filepath.Join(dir, ".build", "Weather", "ExportOptions.plist"))
	if err != nil {
		t.Fatalf("read export options: %v", err)
	}
	if !strings.Contains(string(exportOptionsData), "<string>development</string>") {
		t.Fatalf("export options missing development method: %s", string(exportOptionsData))
	}
	if !strings.Contains(string(exportOptionsData), "<string>TEAM123</string>") {
		t.Fatalf("export options missing team id: %s", string(exportOptionsData))
	}
}

func TestXcodebuildTargetSettingsErrorsOnInvalidJSON(t *testing.T) {
	originalRun := runXcodebuild
	t.Cleanup(func() {
		runXcodebuild = originalRun
	})

	runXcodebuild = func(_ context.Context, _ ...string) (*toolchain.CommandResult, error) {
		return &toolchain.CommandResult{Stdout: "not json"}, nil
	}

	bc := &BuildContext{
		Ctx:          context.Background(),
		XcodeprojDir: "/tmp/App.xcodeproj",
		BuildDir:     "/tmp/.build/App",
		BuildConfig:  "debug",
		Platform:     toolchain.PlatformSimulator,
		Target:       &config.TargetConfig{Name: "Weather"},
	}

	_, err := xcodebuildTargetSettings(bc, xcodebuildSelector{flag: "-target", value: "Weather"})
	if err == nil || !strings.Contains(err.Error(), "parsing xcodebuild build settings JSON") {
		t.Fatalf("err = %v", err)
	}
}
