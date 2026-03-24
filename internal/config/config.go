// Package config provides the unified project configuration model.
// Both xcodeproj and xless-native projects resolve to the same types.
package config

import "github.com/kacy/xless/internal/swiftpm"

// ProjectConfig is the unified configuration for an xless project.
// The build pipeline receives this and never knows which source produced it.
type ProjectConfig struct {
	Project             ProjectInfo               `yaml:"project"`
	Targets             []TargetConfig            `yaml:"targets"`
	Defaults            DefaultsConfig            `yaml:"defaults"`
	ResolvedPackages    []swiftpm.ResolvedPackage `yaml:"-"`
	PackageResolvedFile string                    `yaml:"-"`
}

// ProjectInfo holds top-level project metadata.
type ProjectInfo struct {
	Name string `yaml:"name"`
}

// TargetConfig describes a single build target (app, extension, test, framework).
type TargetConfig struct {
	Name                 string        `yaml:"name"`
	BundleID             string        `yaml:"bundle_id"`
	ProductType          ProductType   `yaml:"product_type"`
	Sources              []string      `yaml:"sources"`
	Resources            []string      `yaml:"resources"`
	MinIOS               string        `yaml:"min_ios"`
	SwiftFlags           []string      `yaml:"swift_flags"`
	Signing              SigningConfig `yaml:"signing"`
	InfoPlist            string        `yaml:"info_plist"`
	Version              string        `yaml:"version"`
	BuildNum             string        `yaml:"build_number"`
	Dependencies         []string      `yaml:"dependencies"`
	SourceRoot           string        `yaml:"-"`
	Packages             []string      `yaml:"-"`
	PackageRefs          []string      `yaml:"-"`
	Frameworks           []string      `yaml:"-"`
	Libraries            []string      `yaml:"-"`
	LinkerFlags          []string      `yaml:"-"`
	FrameworkSearchPaths []string      `yaml:"-"`
	LibrarySearchPaths   []string      `yaml:"-"`
	ShellScriptPhases    []string      `yaml:"-"`
	CopyFilesPhases      []string      `yaml:"-"`
	Unsupported          []string      `yaml:"-"`
}

// SigningConfig holds code signing settings.
type SigningConfig struct {
	Identity            string `yaml:"identity"`
	ProvisioningProfile string `yaml:"provisioning_profile"`
	Entitlements        string `yaml:"entitlements"`
	TeamID              string `yaml:"team_id"`
}

// DefaultsConfig holds user preferences for target/config/device selection.
type DefaultsConfig struct {
	Target    string `yaml:"target"`
	Config    string `yaml:"config"`
	Simulator string `yaml:"simulator"`
	Device    string `yaml:"device"`
}

// ProductType identifies what a target produces.
type ProductType string

const (
	ProductApp          ProductType = "app"
	ProductAppExtension ProductType = "app-extension"
	ProductUnitTest     ProductType = "unit-test"
	ProductFramework    ProductType = "framework"
)

// FindTarget returns the target with the given name, or nil if not found.
func (c *ProjectConfig) FindTarget(name string) *TargetConfig {
	for i := range c.Targets {
		if c.Targets[i].Name == name {
			return &c.Targets[i]
		}
	}
	return nil
}

// DefaultTarget returns the target selected by defaults, or the first app target,
// or the first target overall. Returns nil only if there are no targets.
func (c *ProjectConfig) DefaultTarget() *TargetConfig {
	if c.Defaults.Target != "" {
		if t := c.FindTarget(c.Defaults.Target); t != nil {
			return t
		}
	}

	// prefer the first app target
	for i := range c.Targets {
		if c.Targets[i].ProductType == ProductApp {
			return &c.Targets[i]
		}
	}

	if len(c.Targets) > 0 {
		return &c.Targets[0]
	}
	return nil
}
