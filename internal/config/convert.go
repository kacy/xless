package config

import (
	"path/filepath"
	"strings"

	"github.com/kacy/xless/internal/xcodeproj"
)

// xcodeproj product type constants (mirrored here to avoid exporting from xcodeproj)
const (
	xpProductTypeApplication  = "com.apple.product-type.application"
	xpProductTypeAppExtension = "com.apple.product-type.app-extension"
	xpProductTypeFramework    = "com.apple.product-type.framework"
	xpProductTypeUnitTest     = "com.apple.product-type.bundle.unit-testing"
)

// ConvertXcodeProject converts an XcodeProject into the unified config model.
// configName selects which build configuration to extract settings from
// (e.g. "debug" or "release"). If empty, "debug" is used.
func ConvertXcodeProject(xp *xcodeproj.XcodeProject, configName string) *ProjectConfig {
	if configName == "" {
		configName = "debug"
	}

	cfg := &ProjectConfig{
		Project: ProjectInfo{
			Name: xp.Name,
		},
	}

	for _, xt := range xp.Targets {
		tc := convertTarget(xt, configName)
		cfg.Targets = append(cfg.Targets, tc)
	}

	// if project name is empty, use the first target's name
	if cfg.Project.Name == "" && len(cfg.Targets) > 0 {
		cfg.Project.Name = cfg.Targets[0].Name
	}

	return cfg
}

func convertTarget(xt xcodeproj.XcodeTarget, configName string) TargetConfig {
	tc := TargetConfig{
		Name:              xt.Name,
		ProductType:       mapProductType(xt.ProductType),
		Sources:           xt.SourceFiles,
		Resources:         xt.ResourceFiles,
		Dependencies:      xt.Dependencies,
		Packages:          xt.PackageProducts,
		PackageRefs:       xt.PackageReferences,
		ShellScriptPhases: xt.ShellScriptPhases,
		CopyFilesPhases:   xt.CopyFilesPhases,
	}
	tc.Frameworks, tc.Libraries, tc.Unsupported = classifyTargetLinkInputs(xt.LinkInputs, tc.Unsupported)

	// extract build settings from the selected configuration
	tc.BundleID = xt.Setting(configName, "PRODUCT_BUNDLE_IDENTIFIER")
	tc.MinIOS = xt.Setting(configName, "IPHONEOS_DEPLOYMENT_TARGET")
	tc.Version = xt.Setting(configName, "MARKETING_VERSION")
	tc.BuildNum = xt.Setting(configName, "CURRENT_PROJECT_VERSION")
	tc.InfoPlist = xt.Setting(configName, "INFOPLIST_FILE")

	tc.Signing = SigningConfig{
		Identity:     xt.Setting(configName, "CODE_SIGN_IDENTITY"),
		Entitlements: xt.Setting(configName, "CODE_SIGN_ENTITLEMENTS"),
		TeamID:       xt.Setting(configName, "DEVELOPMENT_TEAM"),
	}

	// extract swift flags from OTHER_SWIFT_FLAGS
	if flags := xt.Setting(configName, "OTHER_SWIFT_FLAGS"); flags != "" {
		tc.SwiftFlags = splitFlags(flags)
	}
	linkerFlags := splitBuildSettingList(xt.Setting(configName, "OTHER_LDFLAGS"))
	supportedFrameworks, unsupportedLinkerFlags := classifyLinkerFlags(linkerFlags)
	tc.Frameworks = appendFrameworks(tc.Frameworks, supportedFrameworks...)
	supportedLibraries, unsupportedLinkerFlags := classifyLibraryFlags(unsupportedLinkerFlags)
	tc.Libraries = appendLibraries(tc.Libraries, supportedLibraries...)
	tc.LinkerFlags = unsupportedLinkerFlags
	tc.FrameworkSearchPaths = splitBuildSettingList(xt.Setting(configName, "FRAMEWORK_SEARCH_PATHS"))
	tc.LibrarySearchPaths = splitBuildSettingList(xt.Setting(configName, "LIBRARY_SEARCH_PATHS"))

	for _, src := range tc.Sources {
		if ext := strings.ToLower(filepath.Ext(src)); ext != ".swift" {
			tc.Unsupported = append(tc.Unsupported, "non-swift source file "+src)
		}
	}
	for _, resource := range tc.Resources {
		switch strings.ToLower(filepath.Ext(resource)) {
		case ".xcassets":
			tc.Unsupported = append(tc.Unsupported, "asset catalog resource "+resource)
		case ".storyboard", ".xib":
			tc.Unsupported = append(tc.Unsupported, "Interface Builder resource "+resource)
		case ".xcdatamodel", ".xcdatamodeld":
			tc.Unsupported = append(tc.Unsupported, "Core Data model resource "+resource)
		}
	}
	if header := xt.Setting(configName, "SWIFT_OBJC_BRIDGING_HEADER"); header != "" {
		tc.Unsupported = append(tc.Unsupported, "Objective-C bridging header "+header)
	}
	if len(tc.LinkerFlags) > 0 {
		tc.Unsupported = append(tc.Unsupported, "custom linker flags: "+strings.Join(tc.LinkerFlags, " "))
	}
	if len(tc.FrameworkSearchPaths) > 0 {
		tc.Unsupported = append(tc.Unsupported, "framework search paths: "+strings.Join(tc.FrameworkSearchPaths, ", "))
	}
	if len(tc.LibrarySearchPaths) > 0 {
		tc.Unsupported = append(tc.Unsupported, "library search paths: "+strings.Join(tc.LibrarySearchPaths, ", "))
	}
	if len(tc.ShellScriptPhases) > 0 {
		tc.Unsupported = append(tc.Unsupported, "shell script build phases: "+strings.Join(tc.ShellScriptPhases, ", "))
	}
	if len(tc.CopyFilesPhases) > 0 {
		tc.Unsupported = append(tc.Unsupported, "copy files build phases: "+strings.Join(tc.CopyFilesPhases, ", "))
	}

	return tc
}

func mapProductType(xcodeType string) ProductType {
	switch xcodeType {
	case xpProductTypeApplication:
		return ProductApp
	case xpProductTypeAppExtension:
		return ProductAppExtension
	case xpProductTypeUnitTest:
		return ProductUnitTest
	case xpProductTypeFramework:
		return ProductFramework
	default:
		return ProductApp
	}
}

// splitFlags splits a space-separated flags string, stripping $(inherited).
func splitFlags(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var flags []string
	for _, f := range strings.Fields(s) {
		if f == "$(inherited)" {
			continue
		}
		flags = append(flags, f)
	}
	return flags
}

func splitBuildSettingList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var values []string
	for _, value := range strings.Fields(s) {
		if value == "$(inherited)" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func classifyLinkerFlags(flags []string) (frameworks []string, unsupported []string) {
	for i := 0; i < len(flags); i++ {
		if flags[i] == "-framework" && i+1 < len(flags) {
			frameworks = append(frameworks, flags[i+1]+".framework")
			i++
			continue
		}
		unsupported = append(unsupported, flags[i])
	}
	return frameworks, unsupported
}

func classifyLibraryFlags(flags []string) (libraries []string, unsupported []string) {
	for i := 0; i < len(flags); i++ {
		if flags[i] == "-l" && i+1 < len(flags) {
			libraries = append(libraries, flags[i+1])
			i++
			continue
		}
		if strings.HasPrefix(flags[i], "-l") && len(flags[i]) > 2 {
			libraries = append(libraries, flags[i][2:])
			continue
		}
		unsupported = append(unsupported, flags[i])
	}
	return libraries, unsupported
}

func classifyTargetLinkInputs(inputs []xcodeproj.LinkInput, unsupported []string) (frameworks []string, libraries []string, updatedUnsupported []string) {
	updatedUnsupported = unsupported
	for _, input := range inputs {
		base := filepath.Base(input.Path)
		switch {
		case input.SourceTree == "SDKROOT" && strings.HasSuffix(base, ".framework"):
			frameworks = append(frameworks, base)
		case input.SourceTree == "SDKROOT" && isSDKLibraryFile(base):
			libraries = append(libraries, libraryName(base))
		case strings.HasSuffix(base, ".framework"):
			updatedUnsupported = append(updatedUnsupported, "non-SDK framework dependency "+input.Path)
		case isSDKLibraryFile(base):
			updatedUnsupported = append(updatedUnsupported, "non-SDK library dependency "+input.Path)
		case base != "":
			updatedUnsupported = append(updatedUnsupported, "unsupported link input "+input.Path)
		}
	}
	return appendFrameworks(nil, frameworks...), appendLibraries(nil, libraries...), updatedUnsupported
}

func isSDKLibraryFile(name string) bool {
	return strings.HasPrefix(name, "lib") &&
		(strings.HasSuffix(name, ".tbd") || strings.HasSuffix(name, ".dylib") || strings.HasSuffix(name, ".a"))
}

func libraryName(name string) string {
	name = strings.TrimPrefix(name, "lib")
	switch {
	case strings.HasSuffix(name, ".tbd"):
		return strings.TrimSuffix(name, ".tbd")
	case strings.HasSuffix(name, ".dylib"):
		return strings.TrimSuffix(name, ".dylib")
	case strings.HasSuffix(name, ".a"):
		return strings.TrimSuffix(name, ".a")
	default:
		return name
	}
}

func appendFrameworks(existing []string, frameworks ...string) []string {
	for _, framework := range frameworks {
		found := false
		for _, current := range existing {
			if current == framework {
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, framework)
		}
	}
	return existing
}

func appendLibraries(existing []string, libraries ...string) []string {
	for _, library := range libraries {
		found := false
		for _, current := range existing {
			if current == library {
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, library)
		}
	}
	return existing
}
