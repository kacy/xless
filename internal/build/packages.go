package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kacy/xless/internal/swiftpm"
	"github.com/kacy/xless/internal/toolchain"
	"howett.net/plist"
)

const packageResolutionTimeout = 5 * time.Minute

type PackageDependenciesStage struct{}

func (PackageDependenciesStage) Name() string { return "packages" }

func (PackageDependenciesStage) Run(bc *BuildContext) error {
	if len(bc.Target.Packages) == 0 && len(bc.Target.PackageRefs) == 0 {
		return nil
	}

	moduleDir := filepath.Join(bc.BuildDir, "packages", "modules")
	libraryDir := filepath.Join(bc.BuildDir, "packages", "lib")
	cacheHome := filepath.Join(bc.BuildDir, "packages", "home")
	clangCache := filepath.Join(cacheHome, ".cache", "clang", "ModuleCache")
	for _, dir := range []string{moduleDir, libraryDir, cacheHome, clangCache} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return &BuildError{
				Stage: "packages",
				Err:   fmt.Errorf("cannot create package build directory %s: %w", dir, err),
				Hint:  "check write permissions for .build",
			}
		}
	}

	refs, err := resolvePackageReferences(bc)
	if err != nil {
		return &BuildError{
			Stage: "packages",
			Err:   err,
			Hint:  "run `xcodebuild -resolvePackageDependencies` if remote package checkouts are missing",
		}
	}

	manifests, err := loadPackageManifests(bc.Ctx, refs, cacheHome, clangCache)
	if err != nil {
		return &BuildError{
			Stage: "packages",
			Err:   err,
			Hint:  "ensure package manifests are valid pure-Swift packages",
		}
	}

	productIndex, err := buildProductIndex(manifests)
	if err != nil {
		return &BuildError{
			Stage: "packages",
			Err:   err,
			Hint:  "package product names must be unique across resolved dependencies",
		}
	}
	buildOrder, err := packageBuildOrder(manifests, productIndex, bc.Target.Packages)
	if err != nil {
		return &BuildError{
			Stage: "packages",
			Err:   err,
			Hint:  "xless currently supports pure-Swift library products only",
		}
	}

	for _, item := range buildOrder {
		frameworks, libraries, linkerFlags, err := packageLinkerInputs(item.Target, bc.BuildConfig)
		if err != nil {
			return &BuildError{
				Stage: "packages",
				Err:   err,
				Hint:  "package linker settings must be compatible with xless's direct swiftc linking",
			}
		}
		bc.PackageFrameworks = append(bc.PackageFrameworks, frameworks...)
		bc.PackageLibraries = append(bc.PackageLibraries, libraries...)
		bc.PackageLinkerFlags = append(bc.PackageLinkerFlags, linkerFlags...)

		resourceBundle, err := compilePackageTarget(bc, item, moduleDir, libraryDir, cacheHome, clangCache)
		if err != nil {
			return &BuildError{
				Stage: "packages",
				Err:   err,
				Hint:  "package dependencies must be pure Swift library targets",
			}
		}
		if resourceBundle != "" {
			bc.PackageResourceBundles = append(bc.PackageResourceBundles, resourceBundle)
		}
		bc.PackageLibraries = append(bc.PackageLibraries, item.Target.Name)
	}

	if len(buildOrder) > 0 {
		bc.PackageModuleDirs = append(bc.PackageModuleDirs, moduleDir)
		bc.PackageLibraryDirs = append(bc.PackageLibraryDirs, libraryDir)
	}

	return nil
}

type packageReference struct {
	Ref string
	Dir string
}

type packageManifest struct {
	Name     string                  `json:"name"`
	Path     string                  `json:"-"`
	Products []packageProduct        `json:"products"`
	Targets  []packageTargetManifest `json:"targets"`
}

type packageProduct struct {
	Name    string   `json:"name"`
	Targets []string `json:"targets"`
}

type packageTargetManifest struct {
	Name           string                    `json:"name"`
	Type           string                    `json:"type"`
	Path           string                    `json:"path"`
	Sources        []string                  `json:"sources"`
	Resources      []packageResource         `json:"resources"`
	SwiftSettings  []packageSetting          `json:"swiftSettings"`
	LinkerSettings []packageSetting          `json:"linkerSettings"`
	CSettings      []packageSetting          `json:"cSettings"`
	CXXSettings    []packageSetting          `json:"cxxSettings"`
	Dependencies   []packageTargetDependency `json:"dependencies"`
}

type packageResource struct {
	Rule string `json:"rule"`
	Path string `json:"path"`
}

type packageSetting struct {
	Name      string                   `json:"name"`
	Value     packageSettingValue      `json:"value"`
	Condition *packageSettingCondition `json:"condition"`
}

type packageSettingCondition struct {
	PlatformNames []string `json:"platformNames"`
	Config        string   `json:"config"`
}

type packageSettingValue []string

func (v *packageSettingValue) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*v = nil
		return nil
	}

	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*v = []string{single}
		return nil
	}

	var multi []string
	if err := json.Unmarshal(data, &multi); err == nil {
		*v = multi
		return nil
	}

	return fmt.Errorf("unsupported package setting value: %s", string(data))
}

type packageTargetDependency struct {
	ByName  []string `json:"byName"`
	Target  []string `json:"target"`
	Product []string `json:"product"`
}

type packageManifestInfo struct {
	Manifest *packageManifest
	Ref      packageReference
}

type packageBuildItem struct {
	Manifest *packageManifest
	Target   *packageTargetManifest
}

var dumpPackageManifest = realDumpPackageManifest
var buildPackageLibrary = realBuildPackageLibrary
var resolvePackageDependencies = realResolvePackageDependencies

func resolvePackageReferences(bc *BuildContext) ([]packageReference, error) {
	refs := make([]packageReference, 0, len(bc.Target.PackageRefs))
	var missingRemoteRefs []string

	for _, ref := range bc.Target.PackageRefs {
		dir, err := resolvePackageReferenceDir(ref, bc)
		if err != nil {
			if strings.HasPrefix(ref, "remote ") && canResolveRemotePackages(bc) {
				missingRemoteRefs = append(missingRemoteRefs, ref)
				continue
			}
			return nil, err
		}
		refs = append(refs, packageReference{Ref: ref, Dir: dir})
	}

	if len(missingRemoteRefs) == 0 {
		return refs, nil
	}

	if err := resolvePackageDependencies(bc); err != nil {
		return nil, err
	}
	refreshResolvedPackages(bc)

	for _, ref := range missingRemoteRefs {
		dir, err := resolvePackageReferenceDir(ref, bc)
		if err != nil {
			return nil, err
		}
		refs = append(refs, packageReference{Ref: ref, Dir: dir})
	}
	return refs, nil
}

func canResolveRemotePackages(bc *BuildContext) bool {
	return bc.WorkspaceDir != "" || bc.XcodeprojDir != ""
}

func resolvePackageReferenceDir(ref string, bc *BuildContext) (string, error) {
	switch {
	case strings.HasPrefix(ref, "local "):
		path := strings.TrimSpace(strings.TrimPrefix(ref, "local "))
		if path == "" {
			return "", fmt.Errorf("local package reference %q has no path", ref)
		}
		abs := resolveProjectPath(bc.RootDir, path)
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("local package reference %q: %w", ref, err)
		}
		return abs, nil
	case strings.HasPrefix(ref, "remote "):
		url := strings.TrimSpace(strings.TrimPrefix(ref, "remote "))
		if url == "" {
			return "", fmt.Errorf("remote package reference %q has no URL", ref)
		}
		candidates := remoteCheckoutCandidates(url, bc)
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
		return "", fmt.Errorf("remote package %q is not available in SourcePackages/checkouts", url)
	default:
		return "", fmt.Errorf("unsupported package reference %q", ref)
	}
}

func remoteCheckoutCandidates(url string, bc *BuildContext) []string {
	names := []string{strings.TrimSuffix(filepath.Base(strings.TrimSuffix(url, "/")), ".git")}
	if bc.Config != nil {
		for _, pkg := range bc.Config.ResolvedPackages {
			if pkg.Location == url && pkg.Identity != "" {
				names = append(names, pkg.Identity)
			}
		}
	}
	names = uniqueStrings(names)

	var candidates []string
	for _, base := range names {
		candidates = append(candidates,
			filepath.Join(bc.RootDir, "SourcePackages", "checkouts", base),
			filepath.Join(bc.BuildDir, "SourcePackages", "checkouts", base),
		)
		if bc.WorkspaceDir != "" {
			candidates = append(candidates, filepath.Join(bc.WorkspaceDir, "SourcePackages", "checkouts", base))
		}
		if bc.XcodeprojDir != "" {
			candidates = append(candidates, filepath.Join(filepath.Dir(bc.XcodeprojDir), "SourcePackages", "checkouts", base))
		}
	}

	return uniqueStrings(candidates)
}

func loadPackageManifests(ctx context.Context, refs []packageReference, cacheHome, clangCache string) ([]packageManifestInfo, error) {
	infos := make([]packageManifestInfo, 0, len(refs))
	for _, ref := range refs {
		manifest, err := dumpPackageManifest(ctx, ref.Dir, cacheHome, clangCache)
		if err != nil {
			return nil, fmt.Errorf("describing package %s: %w", ref.Dir, err)
		}
		infos = append(infos, packageManifestInfo{Manifest: manifest, Ref: ref})
	}
	return infos, nil
}

func realDumpPackageManifest(ctx context.Context, packageDir, cacheHome, clangCache string) (*packageManifest, error) {
	env := map[string]string{
		"HOME":                    cacheHome,
		"CLANG_MODULE_CACHE_PATH": clangCache,
	}
	result, err := toolchain.RunCommandEnv(ctx, env, "swift", "package", "dump-package", "--package-path", packageDir)
	if err != nil {
		stderr := ""
		if result != nil {
			stderr = strings.TrimSpace(result.Stderr)
		}
		if stderr != "" {
			return nil, fmt.Errorf("swift package dump-package failed:\n%s", stderr)
		}
		return nil, err
	}

	var manifest packageManifest
	if err := json.Unmarshal([]byte(result.Stdout), &manifest); err != nil {
		return nil, fmt.Errorf("parsing swift package manifest JSON: %w", err)
	}
	manifest.Path = packageDir
	return &manifest, nil
}

func realResolvePackageDependencies(bc *BuildContext) error {
	ctx := bc.Ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, packageResolutionTimeout)
		defer cancel()
	}

	args := []string{"-resolvePackageDependencies"}
	switch {
	case bc.WorkspaceDir != "":
		args = append(args, "-workspace", bc.WorkspaceDir)
	case bc.XcodeprojDir != "":
		args = append(args, "-project", bc.XcodeprojDir)
	default:
		return fmt.Errorf("cannot resolve package dependencies without a workspace or xcodeproj")
	}

	result, err := toolchain.RunCommand(ctx, "xcodebuild", args...)
	if err != nil {
		stderr := ""
		if result != nil {
			stderr = strings.TrimSpace(result.Stderr)
		}
		if stderr != "" {
			return fmt.Errorf("xcodebuild -resolvePackageDependencies failed:\n%s", stderr)
		}
		return err
	}
	return nil
}

func refreshResolvedPackages(bc *BuildContext) {
	if bc.Config == nil {
		return
	}

	resolvedPath := ""
	switch {
	case bc.WorkspaceDir != "":
		resolvedPath = swiftpm.FindResolvedForWorkspace(bc.WorkspaceDir)
	case bc.XcodeprojDir != "":
		resolvedPath = swiftpm.FindResolvedForXcodeproj(bc.XcodeprojDir)
	}
	if resolvedPath == "" {
		return
	}

	resolved, err := swiftpm.ParseResolved(resolvedPath)
	if err != nil {
		return
	}

	bc.Config.PackageResolvedFile = resolved.Path
	bc.Config.ResolvedPackages = resolved.Packages
}

func buildProductIndex(manifests []packageManifestInfo) (map[string]packageManifestInfo, error) {
	index := make(map[string]packageManifestInfo)
	for _, info := range manifests {
		for _, product := range info.Manifest.Products {
			if existing, ok := index[product.Name]; ok {
				return nil, fmt.Errorf(
					"duplicate package product %q from %s and %s",
					product.Name,
					existing.Manifest.Path,
					info.Manifest.Path,
				)
			}
			index[product.Name] = info
		}
	}
	return index, nil
}

func packageBuildOrder(manifests []packageManifestInfo, products map[string]packageManifestInfo, requiredProducts []string) ([]packageBuildItem, error) {
	type targetKey struct {
		packagePath string
		targetName  string
	}

	seen := make(map[targetKey]bool)
	var order []packageBuildItem

	var visitTarget func(manifest *packageManifest, targetName string) error
	visitTarget = func(manifest *packageManifest, targetName string) error {
		target := findPackageTarget(manifest, targetName)
		if target == nil {
			return fmt.Errorf("package %q target %q not found", manifest.Name, targetName)
		}
		if err := validatePackageTargetSupport(target); err != nil {
			return err
		}

		key := targetKey{packagePath: manifest.Path, targetName: target.Name}
		if seen[key] {
			return nil
		}
		seen[key] = true

		for _, dep := range target.Dependencies {
			if depName := firstDependencyName(dep.Target); depName != "" {
				if err := visitTarget(manifest, depName); err != nil {
					return err
				}
				continue
			}
			if depName := firstDependencyName(dep.ByName); depName != "" {
				if sameTarget := findPackageTarget(manifest, depName); sameTarget != nil {
					if err := visitTarget(manifest, depName); err != nil {
						return err
					}
					continue
				}
				productInfo, ok := products[depName]
				if !ok {
					return fmt.Errorf("package dependency %q not found", depName)
				}
				for _, productTarget := range findPackageProduct(productInfo.Manifest, depName).Targets {
					if err := visitTarget(productInfo.Manifest, productTarget); err != nil {
						return err
					}
				}
				continue
			}
			if depName := firstDependencyName(dep.Product); depName != "" {
				productInfo, ok := products[depName]
				if !ok {
					return fmt.Errorf("package product dependency %q not found", depName)
				}
				product := findPackageProduct(productInfo.Manifest, depName)
				if product == nil {
					return fmt.Errorf("package product %q not found", depName)
				}
				for _, productTarget := range product.Targets {
					if err := visitTarget(productInfo.Manifest, productTarget); err != nil {
						return err
					}
				}
			}
		}

		order = append(order, packageBuildItem{Manifest: manifest, Target: target})
		return nil
	}

	for _, productName := range requiredProducts {
		info, ok := products[productName]
		if !ok {
			return nil, fmt.Errorf("package product %q not found in resolved package references", productName)
		}
		product := findPackageProduct(info.Manifest, productName)
		if product == nil {
			return nil, fmt.Errorf("package product %q not found in manifest for %s", productName, info.Manifest.Name)
		}
		for _, targetName := range product.Targets {
			if err := visitTarget(info.Manifest, targetName); err != nil {
				return nil, err
			}
		}
	}

	return order, nil
}

func firstDependencyName(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func findPackageProduct(manifest *packageManifest, name string) *packageProduct {
	for i := range manifest.Products {
		if manifest.Products[i].Name == name {
			return &manifest.Products[i]
		}
	}
	return nil
}

func findPackageTarget(manifest *packageManifest, name string) *packageTargetManifest {
	for i := range manifest.Targets {
		if manifest.Targets[i].Name == name {
			return &manifest.Targets[i]
		}
	}
	return nil
}

func compilePackageTarget(bc *BuildContext, item packageBuildItem, moduleDir, libraryDir, cacheHome, clangCache string) (string, error) {
	sourceDir := packageTargetSourceDir(item.Manifest.Path, item.Target)
	var sources []string
	for _, source := range item.Target.Sources {
		sources = append(sources, filepath.Join(sourceDir, source))
	}

	resourceBundlePath, generatedSource, err := stagePackageResources(bc, item)
	if err != nil {
		return "", err
	}
	if generatedSource != "" {
		sources = append(sources, generatedSource)
	}
	if len(sources) == 0 {
		return "", fmt.Errorf("package target %q has no Swift sources", item.Target.Name)
	}

	modulePath := filepath.Join(moduleDir, item.Target.Name+".swiftmodule")
	libraryPath := filepath.Join(libraryDir, "lib"+item.Target.Name+".a")

	if err := buildPackageLibrary(bc, item, sources, modulePath, libraryPath, moduleDir, cacheHome, clangCache); err != nil {
		return "", err
	}
	return resourceBundlePath, nil
}

func validatePackageTargetSupport(target *packageTargetManifest) error {
	if target.Type != "regular" {
		return fmt.Errorf("package target %q uses unsupported type %q", target.Name, target.Type)
	}
	if err := validatePackageResources(target); err != nil {
		return err
	}
	if err := validatePackageSwiftSettings(target); err != nil {
		return err
	}
	if err := validatePackageLinkerSettings(target); err != nil {
		return err
	}
	if len(target.CSettings) > 0 || len(target.CXXSettings) > 0 {
		return fmt.Errorf("package target %q uses c-family settings, which xless does not support", target.Name)
	}
	return nil
}

func validatePackageResources(target *packageTargetManifest) error {
	for _, resource := range target.Resources {
		switch resource.Rule {
		case "process", "copy":
		default:
			return fmt.Errorf("package target %q uses unsupported resource rule %q", target.Name, resource.Rule)
		}
		switch strings.ToLower(filepath.Ext(resource.Path)) {
		case ".xcassets":
			return fmt.Errorf("package target %q uses asset catalog resource %q, which xless does not compile", target.Name, resource.Path)
		case ".storyboard", ".xib":
			return fmt.Errorf("package target %q uses Interface Builder resource %q, which xless does not compile", target.Name, resource.Path)
		case ".xcdatamodel", ".xcdatamodeld":
			return fmt.Errorf("package target %q uses Core Data resource %q, which xless does not compile", target.Name, resource.Path)
		}
	}
	return nil
}

func packageTargetSourceDir(packageDir string, target *packageTargetManifest) string {
	if target.Path != "" {
		return filepath.Join(packageDir, target.Path)
	}
	return filepath.Join(packageDir, "Sources", target.Name)
}

func realBuildPackageLibrary(bc *BuildContext, item packageBuildItem, sources []string, modulePath, libraryPath, moduleDir, cacheHome, clangCache string) error {
	swiftArgs, err := packageSwiftArgs(item.Target, bc.BuildConfig)
	if err != nil {
		return err
	}

	args := []string{
		"-parse-as-library",
		"-emit-library",
		"-static",
		"-emit-module",
		"-module-name", item.Target.Name,
		"-emit-module-path", modulePath,
		"-sdk", bc.Toolchain.SDKPath(bc.Platform),
		"-target", buildTriple(bc.Toolchain.Arch(), bc.Target.MinIOS, bc.Platform),
		"-I", moduleDir,
	}
	if bc.BuildConfig == "release" {
		args = append(args, "-O")
	} else {
		args = append(args, "-Onone", "-g", "-DDEBUG")
	}
	args = append(args, swiftArgs...)
	args = append(args, sources...)
	args = append(args, "-o", libraryPath)

	env := map[string]string{
		"HOME":                    cacheHome,
		"CLANG_MODULE_CACHE_PATH": clangCache,
	}
	result, err := toolchain.RunCommandEnv(bc.Ctx, env, bc.Toolchain.SwiftcPath(), args...)
	if err != nil {
		stderr := ""
		if result != nil {
			stderr = strings.TrimSpace(result.Stderr)
		}
		if stderr != "" {
			return fmt.Errorf("swift package target %q failed:\n%s", item.Target.Name, stderr)
		}
		return fmt.Errorf("swift package target %q failed: %w", item.Target.Name, err)
	}
	return nil
}

func stagePackageResources(bc *BuildContext, item packageBuildItem) (bundlePath string, generatedSource string, err error) {
	if len(item.Target.Resources) == 0 {
		return "", "", nil
	}

	sourceDir := packageTargetSourceDir(item.Manifest.Path, item.Target)
	bundleName := packageResourceBundleName(item.Manifest, item.Target)
	bundlePath = filepath.Join(bc.BuildDir, "packages", "resources", bundleName)
	if err := os.RemoveAll(bundlePath); err != nil {
		return "", "", fmt.Errorf("clearing package resource bundle %q: %w", bundleName, err)
	}
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		return "", "", fmt.Errorf("creating package resource bundle %q: %w", bundleName, err)
	}

	for _, resource := range item.Target.Resources {
		resourcePath := filepath.Join(sourceDir, resource.Path)
		info, err := os.Stat(resourcePath)
		if err != nil {
			return "", "", fmt.Errorf("package resource %q: %w", resource.Path, err)
		}
		switch resource.Rule {
		case "process":
			if info.IsDir() {
				if err := copyDirContents(resourcePath, bundlePath); err != nil {
					return "", "", fmt.Errorf("processing package resource directory %q: %w", resource.Path, err)
				}
			} else {
				dst := filepath.Join(bundlePath, filepath.Base(resourcePath))
				if err := copyFile(resourcePath, dst); err != nil {
					return "", "", fmt.Errorf("processing package resource file %q: %w", resource.Path, err)
				}
			}
		case "copy":
			dst := filepath.Join(bundlePath, filepath.Clean(resource.Path))
			if info.IsDir() {
				if err := copyDir(resourcePath, dst); err != nil {
					return "", "", fmt.Errorf("copying package resource directory %q: %w", resource.Path, err)
				}
			} else {
				if err := copyFile(resourcePath, dst); err != nil {
					return "", "", fmt.Errorf("copying package resource file %q: %w", resource.Path, err)
				}
			}
		default:
			return "", "", fmt.Errorf("package target %q uses unsupported resource rule %q", item.Target.Name, resource.Rule)
		}
	}

	if err := writePackageResourceInfoPlist(bundlePath, item.Manifest, item.Target); err != nil {
		return "", "", err
	}

	generatedDir := filepath.Join(bc.BuildDir, "packages", "generated", item.Target.Name)
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		return "", "", fmt.Errorf("creating generated source dir for package target %q: %w", item.Target.Name, err)
	}
	generatedSource = filepath.Join(generatedDir, "xless_package_resources.swift")
	if err := os.WriteFile(generatedSource, []byte(packageResourceAccessorSource(bundleName)), 0o644); err != nil {
		return "", "", fmt.Errorf("writing package resource accessor for %q: %w", item.Target.Name, err)
	}
	return bundlePath, generatedSource, nil
}

func writePackageResourceInfoPlist(bundlePath string, manifest *packageManifest, target *packageTargetManifest) error {
	info := map[string]any{
		"CFBundleName":        target.Name,
		"CFBundlePackageType": "BNDL",
		"CFBundleIdentifier":  fmt.Sprintf("dev.xless.package.%s.%s", bundleIdentifierComponent(manifest.Name), bundleIdentifierComponent(target.Name)),
	}

	plistPath := filepath.Join(bundlePath, "Info.plist")
	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("creating package resource Info.plist for %q: %w", target.Name, err)
	}
	defer f.Close()

	enc := plist.NewEncoder(f)
	enc.Indent("\t")
	if err := enc.Encode(info); err != nil {
		return fmt.Errorf("writing package resource Info.plist for %q: %w", target.Name, err)
	}
	return nil
}

func packageResourceBundleName(manifest *packageManifest, target *packageTargetManifest) string {
	return fmt.Sprintf("%s_%s.bundle", bundleNameComponent(manifest.Name), bundleNameComponent(target.Name))
}

func packageResourceAccessorSource(bundleName string) string {
	return fmt.Sprintf(`import Foundation

private final class XLESSBundleFinder {}

extension Foundation.Bundle {
    static var module: Foundation.Bundle = {
        let bundleName = %q
        let candidates: [URL?] = [
            Foundation.Bundle.main.resourceURL,
            Foundation.Bundle(for: XLESSBundleFinder.self).resourceURL,
            Foundation.Bundle.main.bundleURL,
            Foundation.Bundle(for: XLESSBundleFinder.self).bundleURL,
        ]

        for candidate in candidates {
            guard let bundleURL = candidate?.appendingPathComponent(bundleName) else {
                continue
            }
            if let bundle = Foundation.Bundle(url: bundleURL) {
                return bundle
            }
        }

        fatalError("unable to find package resource bundle \(bundleName)")
    }()
}
`, bundleName)
}

func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		entrySrc := filepath.Join(src, entry.Name())
		entryDst := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(entrySrc, entryDst); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(entrySrc, entryDst); err != nil {
			return err
		}
	}
	return nil
}

func bundleNameComponent(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "Package"
	}
	return b.String()
}

func bundleIdentifierComponent(name string) string {
	component := strings.ToLower(bundleNameComponent(name))
	component = strings.Trim(component, "._-")
	if component == "" {
		return "package"
	}
	return component
}

func validatePackageSwiftSettings(target *packageTargetManifest) error {
	for _, setting := range target.SwiftSettings {
		if !packageSettingApplies(setting, "debug") && !packageSettingApplies(setting, "release") {
			continue
		}
		switch setting.Name {
		case "define", "unsafeFlags", "enableUpcomingFeature", "enableExperimentalFeature":
		default:
			return fmt.Errorf("package target %q uses unsupported swift setting %q", target.Name, setting.Name)
		}
	}
	return nil
}

func validatePackageLinkerSettings(target *packageTargetManifest) error {
	for _, setting := range target.LinkerSettings {
		if !packageSettingApplies(setting, "debug") && !packageSettingApplies(setting, "release") {
			continue
		}
		switch setting.Name {
		case "linkedFramework", "linkedLibrary", "unsafeFlags":
		default:
			return fmt.Errorf("package target %q uses unsupported linker setting %q", target.Name, setting.Name)
		}
	}
	return nil
}

func packageSwiftArgs(target *packageTargetManifest, buildConfig string) ([]string, error) {
	var args []string
	for _, setting := range target.SwiftSettings {
		if !packageSettingApplies(setting, buildConfig) {
			continue
		}
		switch setting.Name {
		case "define":
			for _, value := range setting.Value {
				args = append(args, "-D"+value)
			}
		case "unsafeFlags":
			args = append(args, setting.Value...)
		case "enableUpcomingFeature":
			for _, value := range setting.Value {
				args = append(args, "-enable-upcoming-feature", value)
			}
		case "enableExperimentalFeature":
			for _, value := range setting.Value {
				args = append(args, "-enable-experimental-feature", value)
			}
		default:
			return nil, fmt.Errorf("package target %q uses unsupported swift setting %q", target.Name, setting.Name)
		}
	}
	return args, nil
}

func packageLinkerInputs(target *packageTargetManifest, buildConfig string) (frameworks []string, libraries []string, linkerFlags []string, err error) {
	for _, setting := range target.LinkerSettings {
		if !packageSettingApplies(setting, buildConfig) {
			continue
		}
		switch setting.Name {
		case "linkedFramework":
			for _, value := range setting.Value {
				if strings.HasSuffix(value, ".framework") {
					frameworks = append(frameworks, value)
				} else {
					frameworks = append(frameworks, value+".framework")
				}
			}
		case "linkedLibrary":
			libraries = append(libraries, setting.Value...)
		case "unsafeFlags":
			linkerFlags = append(linkerFlags, setting.Value...)
		default:
			return nil, nil, nil, fmt.Errorf("package target %q uses unsupported linker setting %q", target.Name, setting.Name)
		}
	}
	return uniqueStrings(frameworks), uniqueStrings(libraries), linkerFlags, nil
}

func packageSettingApplies(setting packageSetting, buildConfig string) bool {
	if setting.Condition == nil {
		return true
	}
	if setting.Condition.Config != "" && !strings.EqualFold(setting.Condition.Config, buildConfig) {
		return false
	}
	if len(setting.Condition.PlatformNames) == 0 {
		return true
	}
	for _, platform := range setting.Condition.PlatformNames {
		if strings.EqualFold(platform, "ios") {
			return true
		}
	}
	return false
}
