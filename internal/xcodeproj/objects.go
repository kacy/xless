// Package xcodeproj reads .xcodeproj/project.pbxproj files and extracts
// target, source, and build setting information.
package xcodeproj

// RawProject holds the deserialized pbxproj as loosely typed maps.
// The pbxproj format is an OpenStep-encoded plist with a flat object table
// keyed by 24-character hex IDs.
type RawProject struct {
	ArchiveVersion string
	ObjectVersion  string
	RootObject     string
	Objects        map[string]map[string]any
}

// XcodeProject holds fully resolved project data — targets with their
// configurations, source files, and resources extracted from the object graph.
type XcodeProject struct {
	Name    string
	Targets []XcodeTarget
}

// XcodeTarget is a resolved native target.
type XcodeTarget struct {
	Name              string
	ProductType       string // e.g. "com.apple.product-type.application"
	Configurations    []BuildConfig
	SourceFiles       []string // relative paths
	ResourceFiles     []string // relative paths
	FrameworkFiles    []string // relative paths
	LinkInputs        []LinkInput
	Dependencies      []string // target names this target depends on
	PackageProducts   []string
	PackageReferences []string
	ShellScriptPhases []string
	CopyFilesPhases   []string
}

// LinkInput is a file referenced from PBXFrameworksBuildPhase.
type LinkInput struct {
	Path       string
	SourceTree string
}

// BuildConfig holds resolved build settings for a named configuration.
type BuildConfig struct {
	Name     string
	Settings map[string]string
}

// known pbxproj isa values we care about
const (
	isaPBXProject                      = "PBXProject"
	isaPBXNativeTarget                 = "PBXNativeTarget"
	isaXCConfigurationList             = "XCConfigurationList"
	isaXCBuildConfiguration            = "XCBuildConfiguration"
	isaPBXSourcesBuildPhase            = "PBXSourcesBuildPhase"
	isaPBXResourcesBuildPhase          = "PBXResourcesBuildPhase"
	isaPBXFrameworksBuildPhase         = "PBXFrameworksBuildPhase"
	isaPBXShellScriptBuildPhase        = "PBXShellScriptBuildPhase"
	isaPBXCopyFilesBuildPhase          = "PBXCopyFilesBuildPhase"
	isaPBXBuildFile                    = "PBXBuildFile"
	isaPBXFileReference                = "PBXFileReference"
	isaPBXGroup                        = "PBXGroup"
	isaPBXTargetDependency             = "PBXTargetDependency"
	isaXCSwiftPackageProductDependency = "XCSwiftPackageProductDependency"
	isaXCRemoteSwiftPackageReference   = "XCRemoteSwiftPackageReference"
	isaXCLocalSwiftPackageReference    = "XCLocalSwiftPackageReference"
)

// common Xcode product types
const (
	productTypeApplication  = "com.apple.product-type.application"
	productTypeAppExtension = "com.apple.product-type.app-extension"
	productTypeFramework    = "com.apple.product-type.framework"
	productTypeUnitTest     = "com.apple.product-type.bundle.unit-testing"
)

// getString safely extracts a string value from an object map.
func getString(obj map[string]any, key string) string {
	if v, ok := obj[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getStringSlice safely extracts a []string from an object map.
// pbxproj arrays come through as []any.
func getStringSlice(obj map[string]any, key string) []string {
	v, ok := obj[key]
	if !ok {
		return nil
	}

	switch arr := v.(type) {
	case []any:
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return arr
	default:
		return nil
	}
}

// getMap safely extracts a map[string]any from an object map.
func getMap(obj map[string]any, key string) map[string]any {
	if v, ok := obj[key]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}
