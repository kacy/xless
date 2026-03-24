package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/kacy/xless/internal/config"
	"github.com/kacy/xless/internal/project"
)

type targetSelectionError struct {
	name string
}

func (e *targetSelectionError) Error() string {
	return fmt.Sprintf("target %q not found", e.name)
}

func loadProject(flags config.CLIFlags) (string, *config.ProjectConfig, *project.DetectResult, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", nil, nil, fmt.Errorf("cannot determine working directory: %w", err)
	}

	cfg, det, err := config.Load(dir, flags)
	if err != nil {
		return "", nil, nil, err
	}

	for _, warning := range det.Warnings {
		out.Warn(warning)
	}

	return dir, cfg, det, nil
}

func selectedTarget(cfg *config.ProjectConfig, flags config.CLIFlags) (*config.TargetConfig, error) {
	if flags.Target != "" {
		target := cfg.FindTarget(flags.Target)
		if target == nil {
			return nil, &targetSelectionError{name: flags.Target}
		}
		return target, nil
	}

	target := cfg.DefaultTarget()
	if target == nil {
		return nil, fmt.Errorf("no targets found in project configuration")
	}

	return target, nil
}

func selectedTargets(cfg *config.ProjectConfig, flags config.CLIFlags) ([]config.TargetConfig, error) {
	if flags.Target == "" {
		return cfg.Targets, nil
	}

	target, err := selectedTarget(cfg, flags)
	if err != nil {
		return nil, err
	}

	return []config.TargetConfig{*target}, nil
}

func printTargetSelectionError(err error) {
	var selectionErr *targetSelectionError
	if errors.As(err, &selectionErr) {
		out.Error(err.Error(), "hint", "run `xless info` to see available targets")
		return
	}

	out.Error(err.Error())
}
