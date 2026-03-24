package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kacy/xless/internal/project"
	"github.com/kacy/xless/internal/workspace"
	"github.com/kacy/xless/internal/xcodeproj"
	"gopkg.in/yaml.v3"
)

// Load detects the project mode and loads the unified config.
// Resolution order: CLI flags > xless.yml > xcodeproj (live-read) > defaults.
func Load(dir string, flags CLIFlags) (*ProjectConfig, *project.DetectResult, error) {
	det, err := project.Detect(dir, flags.ConfigPath)
	if err != nil {
		return nil, nil, err
	}

	var cfg *ProjectConfig

	switch det.Mode {
	case project.ModeNative:
		cfg, err = loadNative(det.ConfigFile)
	case project.ModeWorkspace:
		cfg, err = loadWorkspace(det, flags)
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
	Signing  SigningConfig  `yaml:"signing"`
	Defaults DefaultsConfig `yaml:"defaults"`
}

func (n *nativeYAML) toProjectConfig() *ProjectConfig {
	target := TargetConfig{
		Name:        n.Project.Name,
		BundleID:    n.Project.BundleID,
		ProductType: ProductApp,
		Sources:     n.Build.Sources,
		MinIOS:      n.Build.MinIOS,
		SwiftFlags:  n.Build.SwiftFlags,
		Signing:     n.Signing,
		Version:     n.Project.Version,
		BuildNum:    n.Project.BuildNumber,
	}
	if n.Build.Type != "" && n.Build.Type != "simple" {
		target.Unsupported = append(target.Unsupported, "native build type "+n.Build.Type)
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

	var ov *overlayYAML
	if det.ConfigFile != "" {
		ov, err = loadOverlay(det.ConfigFile)
		if err != nil {
			return nil, err
		}
	}

	configName := flags.BuildConfig
	if configName == "" && ov != nil && ov.Defaults.Config != "" {
		configName = ov.Defaults.Config
	}
	if configName == "" {
		configName = DefaultConfig
	}

	cfg := ConvertXcodeProject(xp, configName)
	for i := range cfg.Targets {
		cfg.Targets[i].SourceRoot = filepath.Dir(det.XcodeprojDir)
	}

	// apply xless.yml overlay if present
	if ov != nil {
		det.Warnings = append(det.Warnings, applyOverlay(cfg, ov)...)
	}

	return cfg, nil
}

func loadWorkspace(det *project.DetectResult, flags CLIFlags) (*ProjectConfig, error) {
	ws, err := workspace.Parse(det.WorkspaceDir)
	if err != nil {
		return nil, err
	}
	if len(ws.Projects) == 0 {
		return nil, fmt.Errorf("workspace %s contains no .xcodeproj references", det.WorkspaceDir)
	}

	var ov *overlayYAML
	if det.ConfigFile != "" {
		ov, err = loadOverlay(det.ConfigFile)
		if err != nil {
			return nil, err
		}
	}

	configName := flags.BuildConfig
	if configName == "" && ov != nil && ov.Defaults.Config != "" {
		configName = ov.Defaults.Config
	}
	if configName == "" {
		configName = DefaultConfig
	}

	projectName := strings.TrimSuffix(filepath.Base(det.WorkspaceDir), ".xcworkspace")
	cfg := &ProjectConfig{
		Project: ProjectInfo{Name: projectName},
	}

	for _, projDir := range ws.Projects {
		raw, err := xcodeproj.Parse(projDir)
		if err != nil {
			return nil, err
		}
		xp, err := xcodeproj.Resolve(raw)
		if err != nil {
			return nil, err
		}
		memberCfg := ConvertXcodeProject(xp, configName)
		for _, target := range memberCfg.Targets {
			target.SourceRoot = filepath.Dir(projDir)
			cfg.Targets = append(cfg.Targets, target)
		}
	}

	if ov != nil {
		det.Warnings = append(det.Warnings, applyOverlay(cfg, ov)...)
	}

	return cfg, nil
}

// applyFlags overrides config values with CLI flags.
func applyFlags(cfg *ProjectConfig, flags CLIFlags) {
	if flags.Target != "" {
		cfg.Defaults.Target = flags.Target
	}
	if flags.BuildConfig != "" {
		cfg.Defaults.Config = flags.BuildConfig
	}
	if flags.Device != "" {
		cfg.Defaults.Device = flags.Device
	}
}
