package config

import (
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
		Name:         xt.Name,
		ProductType:  mapProductType(xt.ProductType),
		Sources:      xt.SourceFiles,
		Resources:    xt.ResourceFiles,
		Dependencies: xt.Dependencies,
	}

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
