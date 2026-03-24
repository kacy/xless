package build

import (
	"context"
	"errors"
	"testing"

	"github.com/kacy/xless/internal/output"
)

// noopFormatter satisfies output.Formatter for tests.
type noopFormatter struct{}

func (noopFormatter) Info(string, ...any)    {}
func (noopFormatter) Success(string, ...any) {}
func (noopFormatter) Error(string, ...any)   {}
func (noopFormatter) Warn(string, ...any)    {}
func (noopFormatter) Data(string, any)       {}

var _ output.Formatter = noopFormatter{}

// recordingStage records whether it ran.
type recordingStage struct {
	name string
	ran  bool
	err  error
}

func (s *recordingStage) Name() string { return s.name }
func (s *recordingStage) Run(_ *BuildContext) error {
	s.ran = true
	return s.err
}

func newBuildContext() *BuildContext {
	return &BuildContext{
		Ctx: context.Background(),
		Out: noopFormatter{},
	}
}

func TestPipelineRunsStagesInOrder(t *testing.T) {
	stages := []string{}
	appendStage := func(name string) Stage {
		return &funcStage{
			name: name,
			fn: func(_ *BuildContext) error {
				stages = append(stages, name)
				return nil
			},
		}
	}

	p := NewPipeline(appendStage("first"), appendStage("second"), appendStage("third"))
	err := p.Run(newBuildContext())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(stages))
	}
	if stages[0] != "first" || stages[1] != "second" || stages[2] != "third" {
		t.Errorf("stages ran in wrong order: %v", stages)
	}
}

func TestPipelineStopsOnError(t *testing.T) {
	s1 := &recordingStage{name: "first", err: errors.New("fail")}
	s2 := &recordingStage{name: "second"}

	p := NewPipeline(s1, s2)
	err := p.Run(newBuildContext())

	if err == nil {
		t.Fatal("expected error")
	}
	if !s1.ran {
		t.Error("first stage should have run")
	}
	if s2.ran {
		t.Error("second stage should not have run after error")
	}

	var be *BuildError
	if !errors.As(err, &be) {
		t.Fatalf("expected BuildError, got %T", err)
	}
	if be.Stage != "first" {
		t.Errorf("expected stage 'first', got %q", be.Stage)
	}
}

func TestPipelineCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	s1 := &recordingStage{name: "first"}

	bc := &BuildContext{
		Ctx: ctx,
		Out: noopFormatter{},
	}

	p := NewPipeline(s1)
	err := p.Run(bc)

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if s1.ran {
		t.Error("stage should not run when context is cancelled")
	}

	var be *BuildError
	if !errors.As(err, &be) {
		t.Fatalf("expected BuildError, got %T", err)
	}
	if be.Hint != "build cancelled" {
		t.Errorf("expected hint 'build cancelled', got %q", be.Hint)
	}
}

func TestBuildErrorFormat(t *testing.T) {
	tests := []struct {
		name string
		err  BuildError
		want string
	}{
		{
			name: "with hint",
			err:  BuildError{Stage: "compile", Err: errors.New("swiftc failed"), Hint: "check your source files"},
			want: "compile: swiftc failed (hint: check your source files)",
		},
		{
			name: "without hint",
			err:  BuildError{Stage: "bundle", Err: errors.New("copy failed")},
			want: "bundle: copy failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildErrorUnwrap(t *testing.T) {
	inner := errors.New("inner error")
	be := &BuildError{Stage: "test", Err: inner}

	if !errors.Is(be, inner) {
		t.Error("errors.Is should find the inner error")
	}
}

func TestPipelinePreservesBuildError(t *testing.T) {
	hint := "custom hint from stage"
	s := &funcStage{
		name: "custom",
		fn: func(_ *BuildContext) error {
			return &BuildError{Stage: "custom", Err: errors.New("custom fail"), Hint: hint}
		},
	}

	p := NewPipeline(s)
	err := p.Run(newBuildContext())

	var be *BuildError
	if !errors.As(err, &be) {
		t.Fatal("expected BuildError")
	}
	if be.Hint != hint {
		t.Errorf("hint should be preserved, got %q", be.Hint)
	}
}

// funcStage is a test helper that implements Stage with a function.
type funcStage struct {
	name string
	fn   func(*BuildContext) error
}

func (s *funcStage) Name() string               { return s.name }
func (s *funcStage) Run(bc *BuildContext) error { return s.fn(bc) }
