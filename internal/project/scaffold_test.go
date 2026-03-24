package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"MyApp", false},
		{"my-app", false},
		{"my_app", false},
		{"app123", false},
		{"A", false},
		{"", true},
		{"123app", true},                 // starts with digit
		{"-app", true},                   // starts with hyphen
		{"_app", true},                   // starts with underscore
		{"my app", true},                 // contains space
		{"my.app", true},                 // contains dot
		{strings.Repeat("a", 65), true},  // too long
		{strings.Repeat("a", 64), false}, // max length
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr = %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestDeriveBundleID(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"MyApp", "com.example.myapp"},
		{"my-app", "com.example.my-app"},
		{"my_app", "com.example.my-app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveBundleID(tt.name)
			if got != tt.want {
				t.Errorf("DeriveBundleID(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestScaffoldSimple(t *testing.T) {
	dir := t.TempDir()

	result, err := Scaffold(dir, ScaffoldConfig{
		Name:     "TestApp",
		MinIOS:   "17.0",
		Template: TemplateSimple,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Dir != dir {
		t.Errorf("expected dir %s, got %s", dir, result.Dir)
	}

	// check expected files exist
	expectedFiles := []string{
		"xless.yml",
		".gitignore",
		filepath.Join("Sources", "TestApp", "TestAppApp.swift"),
	}

	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist: %v", f, err)
		}
	}

	// check xless.yml contains the project name
	data, err := os.ReadFile(filepath.Join(dir, "xless.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `name: "TestApp"`) {
		t.Error("xless.yml should contain project name")
	}
	if !strings.Contains(string(data), `bundle_id: "com.example.testapp"`) {
		t.Error("xless.yml should contain derived bundle ID")
	}

	// check swift file contains app struct
	swiftData, err := os.ReadFile(filepath.Join(dir, "Sources", "TestApp", "TestAppApp.swift"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(swiftData), "struct TestAppApp: App") {
		t.Error("swift file should contain app struct")
	}
}

func TestScaffoldSPM(t *testing.T) {
	dir := t.TempDir()

	result, err := Scaffold(dir, ScaffoldConfig{
		Name:     "SPMApp",
		MinIOS:   "17.0",
		Template: TemplateSPM,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SPM template should also create Package.swift
	hasPackage := false
	for _, f := range result.Files {
		if f == "Package.swift" {
			hasPackage = true
		}
	}
	if !hasPackage {
		t.Error("SPM template should create Package.swift")
	}

	// verify Package.swift content
	data, err := os.ReadFile(filepath.Join(dir, "Package.swift"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `name: "SPMApp"`) {
		t.Error("Package.swift should contain project name")
	}
	if !strings.Contains(string(data), ".iOS(.v17)") {
		t.Error("Package.swift should contain iOS platform version")
	}
}

func TestScaffoldCustomBundleID(t *testing.T) {
	dir := t.TempDir()

	_, err := Scaffold(dir, ScaffoldConfig{
		Name:     "MyApp",
		BundleID: "com.custom.myapp",
		Template: TemplateSimple,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "xless.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `bundle_id: "com.custom.myapp"`) {
		t.Error("xless.yml should contain custom bundle ID")
	}
}

func TestScaffoldExistingProject(t *testing.T) {
	dir := t.TempDir()

	// create an existing xless.yml
	if err := os.WriteFile(filepath.Join(dir, "xless.yml"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Scaffold(dir, ScaffoldConfig{
		Name:     "MyApp",
		Template: TemplateSimple,
	})
	if err == nil {
		t.Fatal("expected error for existing project")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention existing project: %v", err)
	}
}

func TestScaffoldInvalidName(t *testing.T) {
	dir := t.TempDir()

	_, err := Scaffold(dir, ScaffoldConfig{
		Name:     "123invalid",
		Template: TemplateSimple,
	})
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
}

func TestMajorVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"16.0", "16"},
		{"17.2", "17"},
		{"18", "18"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := majorVersion(tt.input)
			if got != tt.want {
				t.Errorf("majorVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
