package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kacy/xless/internal/toolchain"
)

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
		if err := compilePackageTarget(bc, item, moduleDir, libraryDir, cacheHome, clangCache); err != nil {
			return &BuildError{
				Stage: "packages",
				Err:   err,
				Hint:  "package dependencies must be pure Swift library targets",
			}
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
	Name string `json:"name"`
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

func resolvePackageReferences(bc *BuildContext) ([]packageReference, error) {
	refs := make([]packageReference, 0, len(bc.Target.PackageRefs))
	for _, ref := range bc.Target.PackageRefs {
		dir, err := resolvePackageReferenceDir(ref, bc)
		if err != nil {
			return nil, err
		}
		refs = append(refs, packageReference{Ref: ref, Dir: dir})
	}
	return refs, nil
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

func compilePackageTarget(bc *BuildContext, item packageBuildItem, moduleDir, libraryDir, cacheHome, clangCache string) error {
	sourceDir := packageTargetSourceDir(item.Manifest.Path, item.Target)
	var sources []string
	for _, source := range item.Target.Sources {
		sources = append(sources, filepath.Join(sourceDir, source))
	}
	if len(sources) == 0 {
		return fmt.Errorf("package target %q has no Swift sources", item.Target.Name)
	}

	modulePath := filepath.Join(moduleDir, item.Target.Name+".swiftmodule")
	libraryPath := filepath.Join(libraryDir, "lib"+item.Target.Name+".a")

	if err := buildPackageLibrary(bc, item, sources, modulePath, libraryPath, moduleDir, cacheHome, clangCache); err != nil {
		return err
	}
	return nil
}

func validatePackageTargetSupport(target *packageTargetManifest) error {
	if target.Type != "regular" {
		return fmt.Errorf("package target %q uses unsupported type %q", target.Name, target.Type)
	}
	if len(target.Resources) > 0 {
		return fmt.Errorf("package target %q uses resources, which xless does not bundle yet", target.Name)
	}
	if len(target.SwiftSettings) > 0 {
		return fmt.Errorf("package target %q uses package swift settings, which xless does not apply yet", target.Name)
	}
	if len(target.LinkerSettings) > 0 {
		return fmt.Errorf("package target %q uses package linker settings, which xless does not apply yet", target.Name)
	}
	if len(target.CSettings) > 0 || len(target.CXXSettings) > 0 {
		return fmt.Errorf("package target %q uses c-family settings, which xless does not support", target.Name)
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
