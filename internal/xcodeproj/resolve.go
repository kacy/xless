package xcodeproj

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Resolve walks the pbxproj object graph and returns a fully resolved XcodeProject.
// It follows the chain: rootObject → targets → configurationLists → buildConfigurations,
// and extracts source/resource files from build phases.
func Resolve(raw *RawProject) (*XcodeProject, error) {
	rootObj := raw.Objects[raw.RootObject]
	if rootObj == nil {
		return nil, fmt.Errorf("root object %s not found in objects table", raw.RootObject)
	}

	if getString(rootObj, "isa") != isaPBXProject {
		return nil, fmt.Errorf("root object is %q, expected PBXProject", getString(rootObj, "isa"))
	}

	// extract project name from build settings or mainGroup
	projectName := resolveProjectName(raw, rootObj)

	// resolve targets
	targetIDs := getStringSlice(rootObj, "targets")
	var targets []XcodeTarget

	for _, tid := range targetIDs {
		tObj := raw.Objects[tid]
		if tObj == nil {
			continue
		}
		if getString(tObj, "isa") != isaPBXNativeTarget {
			continue // skip aggregate targets, legacy targets, etc.
		}

		target, err := resolveTarget(raw, tObj)
		if err != nil {
			return nil, fmt.Errorf("resolving target %q: %w", getString(tObj, "name"), err)
		}
		targets = append(targets, *target)
	}

	return &XcodeProject{
		Name:    projectName,
		Targets: targets,
	}, nil
}

func resolveProjectName(raw *RawProject, rootObj map[string]any) string {
	// try the project-level build settings first
	configListID := getString(rootObj, "buildConfigurationList")
	if configListID != "" {
		configs := resolveConfigList(raw, configListID)
		for _, c := range configs {
			if name, ok := c.Settings["PRODUCT_NAME"]; ok && name != "" {
				return name
			}
		}
	}

	// fall back to mainGroup name, or just use the first target's name later
	mainGroupID := getString(rootObj, "mainGroup")
	if mainGroupID != "" {
		if gObj := raw.Objects[mainGroupID]; gObj != nil {
			if name := getString(gObj, "name"); name != "" {
				return name
			}
		}
	}

	return ""
}

func resolveTarget(raw *RawProject, tObj map[string]any) (*XcodeTarget, error) {
	name := getString(tObj, "name")
	productType := getString(tObj, "productType")

	// resolve build configurations
	configListID := getString(tObj, "buildConfigurationList")
	configs := resolveConfigList(raw, configListID)

	// resolve build phases → source files and resources
	buildPhaseIDs := getStringSlice(tObj, "buildPhases")
	var sourceFiles, resourceFiles []string

	for _, phaseID := range buildPhaseIDs {
		phaseObj := raw.Objects[phaseID]
		if phaseObj == nil {
			continue
		}

		isa := getString(phaseObj, "isa")
		fileIDs := getStringSlice(phaseObj, "files")

		switch isa {
		case isaPBXSourcesBuildPhase:
			sourceFiles = append(sourceFiles, resolveFileRefs(raw, fileIDs)...)
		case isaPBXResourcesBuildPhase:
			resourceFiles = append(resourceFiles, resolveFileRefs(raw, fileIDs)...)
		}
	}

	// resolve target dependencies
	depIDs := getStringSlice(tObj, "dependencies")
	var deps []string
	for _, depID := range depIDs {
		depObj := raw.Objects[depID]
		if depObj == nil {
			continue
		}
		// PBXTargetDependency has a "target" field pointing to the dependent target
		targetRef := getString(depObj, "target")
		if targetRef != "" {
			if refObj := raw.Objects[targetRef]; refObj != nil {
				if depName := getString(refObj, "name"); depName != "" {
					deps = append(deps, depName)
				}
			}
		}
	}

	return &XcodeTarget{
		Name:           name,
		ProductType:    productType,
		Configurations: configs,
		SourceFiles:    sourceFiles,
		ResourceFiles:  resourceFiles,
		Dependencies:   deps,
	}, nil
}

func resolveConfigList(raw *RawProject, configListID string) []BuildConfig {
	if configListID == "" {
		return nil
	}

	listObj := raw.Objects[configListID]
	if listObj == nil {
		return nil
	}

	configIDs := getStringSlice(listObj, "buildConfigurations")
	var configs []BuildConfig

	for _, cid := range configIDs {
		cObj := raw.Objects[cid]
		if cObj == nil {
			continue
		}

		name := getString(cObj, "name")
		settings := extractBuildSettings(cObj)

		configs = append(configs, BuildConfig{
			Name:     name,
			Settings: settings,
		})
	}

	return configs
}

// extractBuildSettings pulls the buildSettings dictionary from a config object
// and flattens values to strings.
func extractBuildSettings(cObj map[string]any) map[string]string {
	raw := getMap(cObj, "buildSettings")
	if raw == nil {
		return nil
	}

	settings := make(map[string]string, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case string:
			settings[k] = val
		case []any:
			// some settings are arrays (e.g. OTHER_SWIFT_FLAGS)
			parts := make([]string, 0, len(val))
			for _, item := range val {
				if s, ok := item.(string); ok {
					parts = append(parts, s)
				}
			}
			settings[k] = strings.Join(parts, " ")
		case bool:
			if val {
				settings[k] = "YES"
			} else {
				settings[k] = "NO"
			}
		default:
			settings[k] = fmt.Sprintf("%v", v)
		}
	}

	return settings
}

// resolveFileRefs follows PBXBuildFile → fileRef → PBXFileReference to get paths.
func resolveFileRefs(raw *RawProject, buildFileIDs []string) []string {
	var paths []string
	groupPaths := buildGroupPaths(raw)

	for _, bfID := range buildFileIDs {
		bfObj := raw.Objects[bfID]
		if bfObj == nil {
			continue
		}

		fileRefID := getString(bfObj, "fileRef")
		if fileRefID == "" {
			continue
		}

		frObj := raw.Objects[fileRefID]
		if frObj == nil {
			continue
		}

		path := resolveFilePath(frObj, fileRefID, groupPaths)
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}

// resolveFilePath builds the path for a PBXFileReference by walking up the
// group hierarchy based on sourceTree.
func resolveFilePath(frObj map[string]any, fileRefID string, groupPaths map[string]string) string {
	path := getString(frObj, "path")
	if path == "" {
		path = getString(frObj, "name")
		if path == "" {
			return ""
		}
	}

	sourceTree := getString(frObj, "sourceTree")

	switch sourceTree {
	case "<group>":
		if fullPath := groupPaths[fileRefID]; fullPath != "" {
			return fullPath
		}
		return path
	case "SOURCE_ROOT":
		return path
	case "<absolute>":
		return path
	case "SDKROOT", "BUILT_PRODUCTS_DIR":
		return path
	default:
		return path
	}
}

// FindConfig returns the build configuration with the given name from a target,
// or nil if not found.
func (t *XcodeTarget) FindConfig(name string) *BuildConfig {
	// case-insensitive match since Xcode uses "Debug"/"Release" but users type "debug"/"release"
	lower := strings.ToLower(name)
	for i := range t.Configurations {
		if strings.ToLower(t.Configurations[i].Name) == lower {
			return &t.Configurations[i]
		}
	}
	return nil
}

// Setting returns a build setting value, searching the named config first,
// then falling back to other configs. Returns empty string if not found.
func (t *XcodeTarget) Setting(configName, key string) string {
	if cfg := t.FindConfig(configName); cfg != nil {
		if v, ok := cfg.Settings[key]; ok {
			return v
		}
	}

	// fall back: search all configs
	for _, cfg := range t.Configurations {
		if v, ok := cfg.Settings[key]; ok {
			return v
		}
	}
	return ""
}

// GroupPaths builds a map from file reference ID to its group-relative path.
// This helps resolve <group> sourceTree references. Not used in v1 but
// kept as infrastructure for future group-tree walking.
func buildGroupPaths(raw *RawProject) map[string]string {
	paths := make(map[string]string)

	var walk func(groupID, prefix string)
	walk = func(groupID, prefix string) {
		gObj := raw.Objects[groupID]
		if gObj == nil {
			return
		}

		children := getStringSlice(gObj, "children")
		for _, childID := range children {
			child := raw.Objects[childID]
			if child == nil {
				continue
			}

			childPath := getString(child, "path")
			if childPath == "" {
				childPath = getString(child, "name")
			}
			var fullPath string
			if prefix != "" && childPath != "" {
				fullPath = filepath.Join(prefix, childPath)
			} else {
				fullPath = childPath
			}

			isa := getString(child, "isa")
			if isa == isaPBXGroup {
				walk(childID, fullPath)
			} else if isa == isaPBXFileReference {
				paths[childID] = fullPath
			}
		}
	}

	// find the root project's mainGroup
	for _, obj := range raw.Objects {
		if getString(obj, "isa") == isaPBXProject {
			mainGroupID := getString(obj, "mainGroup")
			if mainGroupID != "" {
				groupPath := getString(raw.Objects[mainGroupID], "path")
				walk(mainGroupID, groupPath)
			}
			break
		}
	}

	return paths
}
