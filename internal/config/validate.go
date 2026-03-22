package config

import (
	"fmt"
	"strings"
)

// Validate checks the config for missing required fields and returns
// a combined error if any problems are found.
func Validate(cfg *ProjectConfig) error {
	var problems []string

	if cfg.Project.Name == "" {
		problems = append(problems, "project name is required")
	}

	if len(cfg.Targets) == 0 {
		problems = append(problems, "at least one target is required")
	}

	seen := make(map[string]bool)
	for _, t := range cfg.Targets {
		if t.Name == "" {
			problems = append(problems, "target name cannot be empty")
			continue
		}
		if seen[t.Name] {
			problems = append(problems, fmt.Sprintf("duplicate target name: %q", t.Name))
		}
		seen[t.Name] = true

		if t.BundleID == "" {
			problems = append(problems, fmt.Sprintf("target %q: bundle_id is required", t.Name))
		}
	}

	if cfg.Defaults.Target != "" && cfg.FindTarget(cfg.Defaults.Target) == nil {
		problems = append(problems, fmt.Sprintf(
			"defaults.target %q does not match any target (available: %s)",
			cfg.Defaults.Target,
			targetNames(cfg),
		))
	}

	if len(problems) > 0 {
		return fmt.Errorf("config validation:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

func targetNames(cfg *ProjectConfig) string {
	names := make([]string, len(cfg.Targets))
	for i, t := range cfg.Targets {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}
