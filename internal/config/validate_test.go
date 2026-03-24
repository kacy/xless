package config

import (
	"strings"
	"testing"
)

func TestValidateEmpty(t *testing.T) {
	cfg := &ProjectConfig{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty config")
	}
	if !strings.Contains(err.Error(), "project name is required") {
		t.Errorf("error = %q, want mention of project name", err)
	}
	if !strings.Contains(err.Error(), "at least one target") {
		t.Errorf("error = %q, want mention of targets", err)
	}
}

func TestValidateDuplicateTargetNames(t *testing.T) {
	cfg := &ProjectConfig{
		Project: ProjectInfo{Name: "Test"},
		Targets: []TargetConfig{
			{Name: "App", BundleID: "com.test.app"},
			{Name: "App", BundleID: "com.test.app2"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate target names")
	}
	if !strings.Contains(err.Error(), "duplicate target name") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateMissingBundleID(t *testing.T) {
	cfg := &ProjectConfig{
		Project: ProjectInfo{Name: "Test"},
		Targets: []TargetConfig{
			{Name: "App"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing bundle_id")
	}
	if !strings.Contains(err.Error(), "bundle_id is required") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateBadDefaultTarget(t *testing.T) {
	cfg := &ProjectConfig{
		Project: ProjectInfo{Name: "Test"},
		Targets: []TargetConfig{
			{Name: "App", BundleID: "com.test.app"},
		},
		Defaults: DefaultsConfig{Target: "NonExistent"},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for bad default target")
	}
	if !strings.Contains(err.Error(), "does not match any target") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateValid(t *testing.T) {
	cfg := &ProjectConfig{
		Project: ProjectInfo{Name: "Test"},
		Targets: []TargetConfig{
			{Name: "App", BundleID: "com.test.app"},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFindTarget(t *testing.T) {
	cfg := &ProjectConfig{
		Targets: []TargetConfig{
			{Name: "App"},
			{Name: "Widget"},
		},
	}
	if got := cfg.FindTarget("Widget"); got == nil || got.Name != "Widget" {
		t.Error("FindTarget(Widget) failed")
	}
	if got := cfg.FindTarget("Nope"); got != nil {
		t.Error("FindTarget(Nope) should return nil")
	}
}

func TestDefaultTarget(t *testing.T) {
	t.Run("prefers defaults.target", func(t *testing.T) {
		cfg := &ProjectConfig{
			Targets: []TargetConfig{
				{Name: "Lib", ProductType: ProductFramework},
				{Name: "App", ProductType: ProductApp},
			},
			Defaults: DefaultsConfig{Target: "Lib"},
		}
		if got := cfg.DefaultTarget(); got.Name != "Lib" {
			t.Errorf("got %q, want Lib", got.Name)
		}
	})

	t.Run("falls back to first app", func(t *testing.T) {
		cfg := &ProjectConfig{
			Targets: []TargetConfig{
				{Name: "Lib", ProductType: ProductFramework},
				{Name: "App", ProductType: ProductApp},
			},
		}
		if got := cfg.DefaultTarget(); got.Name != "App" {
			t.Errorf("got %q, want App", got.Name)
		}
	})

	t.Run("falls back to first target", func(t *testing.T) {
		cfg := &ProjectConfig{
			Targets: []TargetConfig{
				{Name: "Lib", ProductType: ProductFramework},
			},
		}
		if got := cfg.DefaultTarget(); got.Name != "Lib" {
			t.Errorf("got %q, want Lib", got.Name)
		}
	})

	t.Run("nil for empty", func(t *testing.T) {
		cfg := &ProjectConfig{}
		if got := cfg.DefaultTarget(); got != nil {
			t.Error("expected nil for empty targets")
		}
	})
}

func TestValidateTargetSupport(t *testing.T) {
	target := &TargetConfig{
		Name:        "App",
		Unsupported: []string{"non-swift source file AppDelegate.m", "shell script build phases: Generate Assets"},
	}

	err := ValidateTargetSupport(target)
	if err == nil {
		t.Fatal("expected unsupported target error")
	}
	if !strings.Contains(err.Error(), "unsupported capabilities") {
		t.Fatalf("error = %q", err)
	}
	if !strings.Contains(err.Error(), "Generate Assets") {
		t.Fatalf("error = %q", err)
	}
}
