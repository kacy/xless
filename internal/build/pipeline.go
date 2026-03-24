package build

import (
	"context"
	"fmt"

	"github.com/kacyfortner/ios-build-cli/internal/config"
	"github.com/kacyfortner/ios-build-cli/internal/output"
	"github.com/kacyfortner/ios-build-cli/internal/toolchain"
)

// BuildContext is the mutable state passed through pipeline stages.
type BuildContext struct {
	Ctx            context.Context
	Config         *config.ProjectConfig
	Target         *config.TargetConfig
	Toolchain      toolchain.Toolchain
	Platform       toolchain.Platform
	BuildConfig    string // "debug" or "release"
	ProjectDir     string // working directory
	BuildDir       string // .build/{target_name}/
	ExecutablePath string // set by compiler
	AppBundlePath  string // set by bundler
	IPAPath        string // set by packager (device builds only)
	Out            output.Formatter
}

// Stage is a single step in the build pipeline.
type Stage interface {
	Name() string
	Run(bc *BuildContext) error
}

// Pipeline runs a sequence of stages in order.
type Pipeline struct {
	stages []Stage
}

// NewPipeline creates a pipeline from the given stages.
func NewPipeline(stages ...Stage) *Pipeline {
	return &Pipeline{stages: stages}
}

// Run executes each stage in order. It checks for context cancellation
// before each stage and stops on the first error.
func (p *Pipeline) Run(bc *BuildContext) error {
	for _, s := range p.stages {
		if err := bc.Ctx.Err(); err != nil {
			return &BuildError{
				Stage: s.Name(),
				Err:   err,
				Hint:  "build cancelled",
			}
		}

		bc.Out.Info(s.Name(), "status", "running")

		if err := s.Run(bc); err != nil {
			if be, ok := err.(*BuildError); ok {
				return be
			}
			return &BuildError{
				Stage: s.Name(),
				Err:   err,
			}
		}
	}
	return nil
}

// BuildError is a build failure with an actionable hint.
type BuildError struct {
	Stage string
	Err   error
	Hint  string
}

func (e *BuildError) Error() string {
	msg := fmt.Sprintf("%s: %s", e.Stage, e.Err)
	if e.Hint != "" {
		msg += fmt.Sprintf(" (hint: %s)", e.Hint)
	}
	return msg
}

func (e *BuildError) Unwrap() error {
	return e.Err
}
