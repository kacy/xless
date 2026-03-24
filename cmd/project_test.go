package cmd

import (
	"testing"

	"github.com/kacy/xless/internal/config"
)

func TestSelectedTargetsUsesRequestedTarget(t *testing.T) {
	cfg := &config.ProjectConfig{
		Targets: []config.TargetConfig{
			{Name: "App", BundleID: "com.test.app", ProductType: config.ProductApp},
			{Name: "Widget", BundleID: "com.test.widget", ProductType: config.ProductAppExtension},
		},
	}

	targets, err := selectedTargets(cfg, config.CLIFlags{Target: "Widget"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(targets))
	}
	if targets[0].Name != "Widget" {
		t.Fatalf("selected target = %q, want %q", targets[0].Name, "Widget")
	}
}

func TestSelectedTargetReturnsHelpfulError(t *testing.T) {
	cfg := &config.ProjectConfig{
		Targets: []config.TargetConfig{
			{Name: "App", BundleID: "com.test.app", ProductType: config.ProductApp},
		},
	}

	_, err := selectedTarget(cfg, config.CLIFlags{Target: "Missing"})
	if err == nil {
		t.Fatal("expected error")
	}

	selectionErr, ok := err.(*targetSelectionError)
	if !ok {
		t.Fatalf("expected targetSelectionError, got %T", err)
	}
	if selectionErr.name != "Missing" {
		t.Fatalf("missing target name = %q, want %q", selectionErr.name, "Missing")
	}
}

func TestParseTemplateType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default simple", input: "", want: "simple"},
		{name: "simple", input: "simple", want: "simple"},
		{name: "spm", input: "spm", want: "spm"},
		{name: "unknown", input: "weird", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTemplateType(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("template = %q, want %q", got, tt.want)
			}
		})
	}
}
