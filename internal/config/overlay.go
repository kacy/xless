package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// overlayYAML maps the xless.yml schema used in xcodeproj overlay mode.
type overlayYAML struct {
	Defaults  DefaultsConfig `yaml:"defaults"`
	Overrides struct {
		Targets map[string]targetOverride `yaml:"targets"`
	} `yaml:"overrides"`
}

// targetOverride holds per-target overrides that can be applied on top of
// xcodeproj-derived config.
type targetOverride struct {
	Signing    *SigningConfig `yaml:"signing"`
	SwiftFlags []string       `yaml:"swift_flags"`
	MinIOS     string         `yaml:"min_ios"`
}

// ApplyOverlay reads an xless.yml overlay file and merges it into cfg.
// Returns warnings for unknown target names (not errors — overlay may be ahead
// of the xcodeproj state). Returns error only for parse/read failures.
func ApplyOverlay(cfg *ProjectConfig, overlayPath string) ([]string, error) {
	data, err := os.ReadFile(overlayPath)
	if err != nil {
		return nil, fmt.Errorf("reading overlay: %w", err)
	}

	var ov overlayYAML
	if err := yaml.Unmarshal(data, &ov); err != nil {
		return nil, fmt.Errorf("parsing overlay %s: %w", overlayPath, err)
	}

	// merge defaults
	if ov.Defaults.Target != "" {
		cfg.Defaults.Target = ov.Defaults.Target
	}
	if ov.Defaults.Config != "" {
		cfg.Defaults.Config = ov.Defaults.Config
	}
	if ov.Defaults.Simulator != "" {
		cfg.Defaults.Simulator = ov.Defaults.Simulator
	}
	if ov.Defaults.Device != "" {
		cfg.Defaults.Device = ov.Defaults.Device
	}

	// merge per-target overrides
	var warnings []string
	for name, override := range ov.Overrides.Targets {
		target := cfg.FindTarget(name)
		if target == nil {
			warnings = append(warnings, fmt.Sprintf(
				"overlay references target %q which does not exist in the xcodeproj", name,
			))
			continue
		}

		// signing — full replace (all-or-nothing)
		if override.Signing != nil {
			target.Signing = *override.Signing
		}

		// swift_flags — appended to xcodeproj flags
		if len(override.SwiftFlags) > 0 {
			target.SwiftFlags = append(target.SwiftFlags, override.SwiftFlags...)
		}

		// min_ios — replaces xcodeproj value
		if override.MinIOS != "" {
			target.MinIOS = override.MinIOS
		}
	}

	return warnings, nil
}
