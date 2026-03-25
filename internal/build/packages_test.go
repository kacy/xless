package build

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kacy/xless/internal/config"
	"github.com/kacy/xless/internal/swiftpm"
	"github.com/kacy/xless/internal/toolchain"
)

func TestResolvePackageReferenceDirLocal(t *testing.T) {
	dir := t.TempDir()
	packageDir := filepath.Join(dir, "Packages", "WeatherCore")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	bc := &BuildContext{RootDir: dir}
	got, err := resolvePackageReferenceDir("local Packages/WeatherCore", bc)
	if err != nil {
		t.Fatalf("resolvePackageReferenceDir: %v", err)
	}
	if got != packageDir {
		t.Fatalf("dir = %q, want %q", got, packageDir)
	}
}

func TestResolvePackageReferenceDirRemoteCheckout(t *testing.T) {
	dir := t.TempDir()
	checkoutDir := filepath.Join(dir, "SourcePackages", "checkouts", "swift-collections")
	if err := os.MkdirAll(checkoutDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	bc := &BuildContext{RootDir: dir}
	got, err := resolvePackageReferenceDir("remote https://github.com/apple/swift-collections.git", bc)
	if err != nil {
		t.Fatalf("resolvePackageReferenceDir: %v", err)
	}
	if got != checkoutDir {
		t.Fatalf("dir = %q, want %q", got, checkoutDir)
	}
}

func TestResolvePackageReferenceDirRemoteCheckoutUsesResolvedIdentity(t *testing.T) {
	dir := t.TempDir()
	checkoutDir := filepath.Join(dir, "SourcePackages", "checkouts", "swift-collections")
	if err := os.MkdirAll(checkoutDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	bc := &BuildContext{
		RootDir: dir,
		Config: &config.ProjectConfig{
			ResolvedPackages: []swiftpm.ResolvedPackage{
				{
					Identity: "swift-collections",
					Location: "https://github.com/apple/swift-collections.git",
				},
			},
		},
	}
	got, err := resolvePackageReferenceDir("remote https://github.com/apple/swift-collections.git", bc)
	if err != nil {
		t.Fatalf("resolvePackageReferenceDir: %v", err)
	}
	if got != checkoutDir {
		t.Fatalf("dir = %q, want %q", got, checkoutDir)
	}
}

func TestPackageBuildOrder(t *testing.T) {
	weatherCore := packageManifest{
		Name: "WeatherCore",
		Path: "/tmp/WeatherCore",
		Products: []packageProduct{
			{Name: "WeatherCore", Targets: []string{"WeatherCore"}},
		},
		Targets: []packageTargetManifest{
			{Name: "WeatherCore", Type: "regular", Sources: []string{"WeatherCore.swift"}},
		},
	}
	weatherUI := packageManifest{
		Name: "WeatherUI",
		Path: "/tmp/WeatherUI",
		Products: []packageProduct{
			{Name: "WeatherUI", Targets: []string{"WeatherUI"}},
		},
		Targets: []packageTargetManifest{
			{
				Name:    "WeatherUI",
				Type:    "regular",
				Sources: []string{"WeatherUI.swift"},
				Dependencies: []packageTargetDependency{
					{Product: []string{"WeatherCore", "WeatherCore"}},
				},
			},
		},
	}

	infos := []packageManifestInfo{
		{Manifest: &weatherCore},
		{Manifest: &weatherUI},
	}
	productIndex, err := buildProductIndex(infos)
	if err != nil {
		t.Fatalf("buildProductIndex: %v", err)
	}
	order, err := packageBuildOrder(infos, productIndex, []string{"WeatherUI"})
	if err != nil {
		t.Fatalf("packageBuildOrder: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("build order = %d, want 2", len(order))
	}
	if order[0].Target.Name != "WeatherCore" || order[1].Target.Name != "WeatherUI" {
		t.Fatalf("build order = [%s, %s]", order[0].Target.Name, order[1].Target.Name)
	}
}

func TestBuildProductIndexRejectsDuplicateProducts(t *testing.T) {
	infos := []packageManifestInfo{
		{
			Manifest: &packageManifest{
				Path:     "/tmp/A",
				Products: []packageProduct{{Name: "SharedKit"}},
			},
		},
		{
			Manifest: &packageManifest{
				Path:     "/tmp/B",
				Products: []packageProduct{{Name: "SharedKit"}},
			},
		},
	}

	_, err := buildProductIndex(infos)
	if err == nil {
		t.Fatal("expected duplicate product error")
	}
}

func TestPackageBuildOrderRejectsTargetResources(t *testing.T) {
	infos := []packageManifestInfo{
		{
			Manifest: &packageManifest{
				Name: "WeatherUI",
				Path: "/tmp/WeatherUI",
				Products: []packageProduct{
					{Name: "WeatherUI", Targets: []string{"WeatherUI"}},
				},
				Targets: []packageTargetManifest{
					{
						Name:      "WeatherUI",
						Type:      "regular",
						Sources:   []string{"WeatherUI.swift"},
						Resources: []packageResource{{Rule: "process", Path: "Resources"}},
					},
				},
			},
		},
	}

	productIndex, err := buildProductIndex(infos)
	if err != nil {
		t.Fatalf("buildProductIndex: %v", err)
	}
	_, err = packageBuildOrder(infos, productIndex, []string{"WeatherUI"})
	if err == nil || !strings.Contains(err.Error(), "uses resources") {
		t.Fatalf("error = %v, want resource support error", err)
	}
}

func TestPackageBuildOrderRejectsUnsupportedSwiftSetting(t *testing.T) {
	infos := []packageManifestInfo{
		{
			Manifest: &packageManifest{
				Name: "WeatherUI",
				Path: "/tmp/WeatherUI",
				Products: []packageProduct{
					{Name: "WeatherUI", Targets: []string{"WeatherUI"}},
				},
				Targets: []packageTargetManifest{
					{
						Name:          "WeatherUI",
						Type:          "regular",
						Sources:       []string{"WeatherUI.swift"},
						SwiftSettings: []packageSetting{{Name: "somethingElse"}},
					},
				},
			},
		},
	}

	productIndex, err := buildProductIndex(infos)
	if err != nil {
		t.Fatalf("buildProductIndex: %v", err)
	}
	_, err = packageBuildOrder(infos, productIndex, []string{"WeatherUI"})
	if err == nil || !strings.Contains(err.Error(), "unsupported swift setting") {
		t.Fatalf("error = %v, want unsupported swift setting error", err)
	}
}

func TestPackageBuildOrderAcceptsSupportedSettings(t *testing.T) {
	infos := []packageManifestInfo{
		{
			Manifest: &packageManifest{
				Name: "WeatherUI",
				Path: "/tmp/WeatherUI",
				Products: []packageProduct{
					{Name: "WeatherUI", Targets: []string{"WeatherUI"}},
				},
				Targets: []packageTargetManifest{
					{
						Name:    "WeatherUI",
						Type:    "regular",
						Sources: []string{"WeatherUI.swift"},
						SwiftSettings: []packageSetting{
							{Name: "define", Value: packageSettingValue{"WEATHER_UI"}},
							{Name: "enableUpcomingFeature", Value: packageSettingValue{"BareSlashRegexLiterals"}},
						},
						LinkerSettings: []packageSetting{
							{Name: "linkedFramework", Value: packageSettingValue{"StoreKit"}},
							{Name: "linkedLibrary", Value: packageSettingValue{"sqlite3"}},
							{Name: "unsafeFlags", Value: packageSettingValue{"-ObjC"}},
						},
					},
				},
			},
		},
	}

	productIndex, err := buildProductIndex(infos)
	if err != nil {
		t.Fatalf("buildProductIndex: %v", err)
	}
	order, err := packageBuildOrder(infos, productIndex, []string{"WeatherUI"})
	if err != nil {
		t.Fatalf("packageBuildOrder: %v", err)
	}
	if len(order) != 1 || order[0].Target.Name != "WeatherUI" {
		t.Fatalf("build order = %+v", order)
	}

	swiftArgs, err := packageSwiftArgs(order[0].Target, "debug")
	if err != nil {
		t.Fatalf("packageSwiftArgs: %v", err)
	}
	if !containsValue(swiftArgs, "-DWEATHER_UI") || !containsSequence(swiftArgs, "-enable-upcoming-feature", "BareSlashRegexLiterals") {
		t.Fatalf("swift args = %v", swiftArgs)
	}

	frameworks, libraries, linkerFlags, err := packageLinkerInputs(order[0].Target, "debug")
	if err != nil {
		t.Fatalf("packageLinkerInputs: %v", err)
	}
	if len(frameworks) != 1 || frameworks[0] != "StoreKit.framework" {
		t.Fatalf("frameworks = %v", frameworks)
	}
	if len(libraries) != 1 || libraries[0] != "sqlite3" {
		t.Fatalf("libraries = %v", libraries)
	}
	if len(linkerFlags) != 1 || linkerFlags[0] != "-ObjC" {
		t.Fatalf("linker flags = %v", linkerFlags)
	}
}

func TestPackageBuildOrderRejectsUnsupportedLinkerSetting(t *testing.T) {
	infos := []packageManifestInfo{
		{
			Manifest: &packageManifest{
				Name: "WeatherUI",
				Path: "/tmp/WeatherUI",
				Products: []packageProduct{
					{Name: "WeatherUI", Targets: []string{"WeatherUI"}},
				},
				Targets: []packageTargetManifest{
					{
						Name:    "WeatherUI",
						Type:    "regular",
						Sources: []string{"WeatherUI.swift"},
						LinkerSettings: []packageSetting{
							{Name: "linkedFramework", Value: packageSettingValue{"StoreKit"}},
							{Name: "somethingElse", Value: packageSettingValue{"x"}},
						},
					},
				},
			},
		},
	}

	productIndex, err := buildProductIndex(infos)
	if err != nil {
		t.Fatalf("buildProductIndex: %v", err)
	}
	_, err = packageBuildOrder(infos, productIndex, []string{"WeatherUI"})
	if err == nil || !strings.Contains(err.Error(), "unsupported linker setting") {
		t.Fatalf("error = %v, want unsupported linker setting error", err)
	}
}

func TestPackageDependenciesStageRun(t *testing.T) {
	dir := t.TempDir()
	packageDir := filepath.Join(dir, "Packages", "WeatherCore")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	originalDump := dumpPackageManifest
	originalBuild := buildPackageLibrary
	t.Cleanup(func() {
		dumpPackageManifest = originalDump
		buildPackageLibrary = originalBuild
	})

	dumpPackageManifest = func(_ context.Context, packageDir, _, _ string) (*packageManifest, error) {
		return &packageManifest{
			Name: "WeatherCore",
			Path: packageDir,
			Products: []packageProduct{
				{Name: "WeatherCore", Targets: []string{"WeatherCore"}},
			},
			Targets: []packageTargetManifest{
				{Name: "WeatherCore", Type: "regular", Path: "Sources/WeatherCore", Sources: []string{"WeatherCore.swift"}},
			},
		}, nil
	}

	var builtTargets []string
	buildPackageLibrary = func(_ *BuildContext, item packageBuildItem, _ []string, modulePath, libraryPath, _, _, _ string) error {
		builtTargets = append(builtTargets, item.Target.Name)
		if err := os.MkdirAll(filepath.Dir(modulePath), 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(libraryPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(modulePath, []byte("module"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(libraryPath, []byte("archive"), 0o644)
	}

	bc := &BuildContext{
		Ctx:         context.Background(),
		RootDir:     dir,
		BuildDir:    filepath.Join(dir, ".build", "App"),
		BuildConfig: "debug",
		Platform:    toolchain.PlatformSimulator,
		Toolchain:   &mockToolchain{arch: "arm64"},
		Target: &config.TargetConfig{
			Name:        "App",
			MinIOS:      "16.0",
			Packages:    []string{"WeatherCore"},
			PackageRefs: []string{"local Packages/WeatherCore"},
		},
	}

	stage := PackageDependenciesStage{}
	if err := stage.Run(bc); err != nil {
		t.Fatalf("stage.Run: %v", err)
	}

	if len(builtTargets) != 1 || builtTargets[0] != "WeatherCore" {
		t.Fatalf("built targets = %v", builtTargets)
	}
	if len(bc.PackageModuleDirs) != 1 || !strings.Contains(bc.PackageModuleDirs[0], "/packages/modules") {
		t.Fatalf("package module dirs = %v", bc.PackageModuleDirs)
	}
	if len(bc.PackageLibraryDirs) != 1 || !strings.Contains(bc.PackageLibraryDirs[0], "/packages/lib") {
		t.Fatalf("package library dirs = %v", bc.PackageLibraryDirs)
	}
	if len(bc.PackageLibraries) != 1 || bc.PackageLibraries[0] != "WeatherCore" {
		t.Fatalf("package libraries = %v", bc.PackageLibraries)
	}
}

func containsValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsSequence(values []string, first, second string) bool {
	for i := 0; i+1 < len(values); i++ {
		if values[i] == first && values[i+1] == second {
			return true
		}
	}
	return false
}

func TestResolvePackageReferencesTriggersXcodeResolutionForRemotePackages(t *testing.T) {
	dir := t.TempDir()
	checkoutDir := filepath.Join(dir, "App.xcworkspace", "SourcePackages", "checkouts", "swift-collections")
	if err := os.MkdirAll(filepath.Dir(checkoutDir), 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}

	originalResolve := resolvePackageDependencies
	t.Cleanup(func() {
		resolvePackageDependencies = originalResolve
	})

	called := 0
	resolvePackageDependencies = func(bc *BuildContext) error {
		called++
		return os.MkdirAll(checkoutDir, 0o755)
	}

	bc := &BuildContext{
		RootDir:      dir,
		WorkspaceDir: filepath.Join(dir, "App.xcworkspace"),
		Config: &config.ProjectConfig{
			ResolvedPackages: []swiftpm.ResolvedPackage{
				{Identity: "swift-collections", Location: "https://github.com/apple/swift-collections.git"},
			},
		},
		Target: &config.TargetConfig{
			PackageRefs: []string{"remote https://github.com/apple/swift-collections.git"},
		},
	}

	refs, err := resolvePackageReferences(bc)
	if err != nil {
		t.Fatalf("resolvePackageReferences: %v", err)
	}
	if called != 1 {
		t.Fatalf("resolvePackageDependencies called %d times, want 1", called)
	}
	if len(refs) != 1 || refs[0].Dir != checkoutDir {
		t.Fatalf("refs = %+v, want checkout dir %q", refs, checkoutDir)
	}
}

func TestResolvePackageReferencesRefreshesResolvedPackagesAfterResolution(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "App.xcworkspace")
	resolvedDir := filepath.Join(workspaceDir, "xcshareddata", "swiftpm")
	if err := os.MkdirAll(resolvedDir, 0o755); err != nil {
		t.Fatalf("mkdir resolved dir: %v", err)
	}
	checkoutDir := filepath.Join(workspaceDir, "SourcePackages", "checkouts", "swift-collections")

	originalResolve := resolvePackageDependencies
	t.Cleanup(func() {
		resolvePackageDependencies = originalResolve
	})

	resolvePackageDependencies = func(bc *BuildContext) error {
		data := `{"pins":[{"identity":"swift-collections","location":"https://example.com/collections.git","state":{"version":"1.0.0","revision":"abc123"}}],"version":2}`
		if err := os.WriteFile(filepath.Join(resolvedDir, "Package.resolved"), []byte(data), 0o644); err != nil {
			return err
		}
		return os.MkdirAll(checkoutDir, 0o755)
	}

	bc := &BuildContext{
		RootDir:      dir,
		WorkspaceDir: workspaceDir,
		Config:       &config.ProjectConfig{},
		Target: &config.TargetConfig{
			PackageRefs: []string{"remote https://example.com/collections.git"},
		},
	}

	refs, err := resolvePackageReferences(bc)
	if err != nil {
		t.Fatalf("resolvePackageReferences: %v", err)
	}
	if len(refs) != 1 || refs[0].Dir != checkoutDir {
		t.Fatalf("refs = %+v, want checkout dir %q", refs, checkoutDir)
	}
	if len(bc.Config.ResolvedPackages) != 1 || bc.Config.ResolvedPackages[0].Identity != "swift-collections" {
		t.Fatalf("resolved packages = %+v", bc.Config.ResolvedPackages)
	}
}

func TestResolvePackageReferencesFailsWithoutProjectContext(t *testing.T) {
	dir := t.TempDir()
	bc := &BuildContext{
		RootDir: dir,
		Target: &config.TargetConfig{
			PackageRefs: []string{"remote https://github.com/apple/swift-collections.git"},
		},
	}

	_, err := resolvePackageReferences(bc)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "SourcePackages/checkouts") {
		t.Fatalf("error = %v", err)
	}
}
