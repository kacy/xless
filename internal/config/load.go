package config

import (
	"fmt"
	"os"

	"github.com/kacy/xless/internal/project"
	"github.com/kacy/xless/internal/xcodeproj"
	"gopkg.in/yaml.v3"
)

// Load detects the project mode and loads the unified config.
// Resolution order: CLI flags > xless.yml > xcodeproj (live-read) > defaults.
func Load(dir string, flags CLIFlags) (*ProjectConfig, *project.DetectResult, error) {
	det, err := project.Detect(dir)
	if err != nil {
		return nil, nil, err
	}

	var cfg *ProjectConfig

	switch det.Mode {
	case project.ModeNative:
		cfg, err = loadNative(det.ConfigFile)
	case project.ModeXcodeproj:
		cfg, err = loadXcodeproj(det, flags)
	default:
		return nil, nil, fmt.Errorf("unknown project mode: %v", det.Mode)
	}

	if err != nil {
		return nil, det, err
	}

	applyDefaults(cfg)
	applyFlags(cfg, flags)

	if err := Validate(cfg); err != nil {
		return nil, det, err
	}

	return cfg, det, nil
}

// loadNative reads a full xless.yml config file.
func loadNative(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var raw nativeYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return raw.toProjectConfig(), nil
}

// nativeYAML maps the xless.yml schema for xless-native projects.
type nativeYAML struct {
	Project struct {
		Name        string `yaml:"name"`
		BundleID    string `yaml:"bundle_id"`
		Version     string `yaml:"version"`
		BuildNumber string `yaml:"build_number"`
	} `yaml:"project"`
	Build struct {
		Type       string   `yaml:"type"`
		Sources    []string `yaml:"sources"`
		MinIOS     string   `yaml:"min_ios"`
		SwiftFlags []string `yaml:"swift_flags"`
	} `yaml:"build"`
	Signing SigningConfig `yaml:"signing"`
	Defaults DefaultsConfig `yaml:"defaults"`
}

func (n *nativeYAML) toProjectConfig() *ProjectConfig {
	target := TargetConfig{
		Name:       n.Project.Name,
		BundleID:   n.Project.BundleID,
		ProductType: ProductApp,
		Sources:    n.Build.Sources,
		MinIOS:     n.Build.MinIOS,
		SwiftFlags: n.Build.SwiftFlags,
		Signing:    n.Signing,
		Version:    n.Project.Version,
		BuildNum:   n.Project.BuildNumber,
	}

	return &ProjectConfig{
		Project:  ProjectInfo{Name: n.Project.Name},
		Targets:  []TargetConfig{target},
		Defaults: n.Defaults,
	}
}

// loadXcodeproj reads the .xcodeproj and optionally applies an xless.yml overlay.
func loadXcodeproj(det *project.DetectResult, flags CLIFlags) (*ProjectConfig, error) {
	raw, err := xcodeproj.Parse(det.XcodeprojDir)
	if err != nil {
		return nil, err
	}

	xp, err := xcodeproj.Resolve(raw)
	if err != nil {
		return nil, err
	}

	configName := flags.Config
	if configName == "" {
		configName = DefaultConfig
	}

	cfg := ConvertXcodeProject(xp, configName)

	// apply xless.yml overlay if present
	if det.ConfigFile != "" {
		warnings, err := ApplyOverlay(cfg, det.ConfigFile)
		if err != nil {
			return nil, fmt.Errorf("applying overlay %s: %w", det.ConfigFile, err)
		}
		_ = warnings // caller can check via separate method if needed
	}

	return cfg, nil
}

// applyFlags overrides config values with CLI flags.
func applyFlags(cfg *ProjectConfig, flags CLIFlags) {
	if flags.Target != "" {
		cfg.Defaults.Target = flags.Target
	}
	if flags.Config != "" {
		cfg.Defaults.Config = flags.Config
	}
	if flags.Device != "" {
		cfg.Defaults.Device = flags.Device
	}
}
