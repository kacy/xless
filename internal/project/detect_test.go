package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(dir string)
		wantMode      Mode
		wantXcodeproj bool
		wantConfig    bool
		wantErr       bool
	}{
		{
			name: "workspace only",
			setup: func(dir string) {
				os.Mkdir(filepath.Join(dir, "MyApp.xcworkspace"), 0o755)
			},
			wantMode:      ModeWorkspace,
			wantXcodeproj: false,
			wantConfig:    false,
		},
		{
			name: "workspace takes precedence over xcodeproj",
			setup: func(dir string) {
				os.Mkdir(filepath.Join(dir, "MyApp.xcworkspace"), 0o755)
				os.Mkdir(filepath.Join(dir, "MyApp.xcodeproj"), 0o755)
			},
			wantMode:      ModeWorkspace,
			wantXcodeproj: true,
			wantConfig:    false,
		},
		{
			name: "xcodeproj and xless.yml",
			setup: func(dir string) {
				os.Mkdir(filepath.Join(dir, "MyApp.xcodeproj"), 0o755)
				os.WriteFile(filepath.Join(dir, "xless.yml"), []byte("defaults:\n"), 0o644)
			},
			wantMode:      ModeXcodeproj,
			wantXcodeproj: true,
			wantConfig:    true,
		},
		{
			name: "xcodeproj only",
			setup: func(dir string) {
				os.Mkdir(filepath.Join(dir, "MyApp.xcodeproj"), 0o755)
			},
			wantMode:      ModeXcodeproj,
			wantXcodeproj: true,
			wantConfig:    false,
		},
		{
			name: "xless.yml only",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "xless.yml"), []byte("project:\n  name: MyApp\n"), 0o644)
			},
			wantMode:      ModeNative,
			wantXcodeproj: false,
			wantConfig:    true,
		},
		{
			name: "xless.yaml variant",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "xless.yaml"), []byte("project:\n  name: MyApp\n"), 0o644)
			},
			wantMode:      ModeNative,
			wantXcodeproj: false,
			wantConfig:    true,
		},
		{
			name:    "nothing found",
			setup:   func(dir string) {},
			wantErr: true,
		},
		{
			name: "non-project directory ignored",
			setup: func(dir string) {
				os.Mkdir(filepath.Join(dir, "whatever"), 0o755)
			},
			wantErr: true,
		},
		{
			name: "multiple xcodeproj directories error",
			setup: func(dir string) {
				os.Mkdir(filepath.Join(dir, "AppOne.xcodeproj"), 0o755)
				os.Mkdir(filepath.Join(dir, "AppTwo.xcodeproj"), 0o755)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)

			result, err := Detect(dir, "")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", result.Mode, tt.wantMode)
			}

			if (result.XcodeprojDir != "") != tt.wantXcodeproj {
				t.Errorf("xcodeproj dir present = %v, want %v", result.XcodeprojDir != "", tt.wantXcodeproj)
			}
			if tt.wantMode == ModeWorkspace && result.WorkspaceDir == "" {
				t.Errorf("workspace dir should be present")
			}

			if (result.ConfigFile != "") != tt.wantConfig {
				t.Errorf("config file present = %v, want %v", result.ConfigFile != "", tt.wantConfig)
			}
		})
	}
}

func TestDetectExplicitConfigPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "custom.yml")
	if err := os.WriteFile(configPath, []byte("project:\n  name: MyApp\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := Detect(dir, "custom.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Mode != ModeNative {
		t.Fatalf("mode = %v, want %v", result.Mode, ModeNative)
	}
	if result.ConfigFile != configPath {
		t.Fatalf("config file = %q, want %q", result.ConfigFile, configPath)
	}
}

func TestDetectMissingExplicitConfigPath(t *testing.T) {
	dir := t.TempDir()

	_, err := Detect(dir, "missing.yml")
	if err == nil {
		t.Fatal("expected error for missing explicit config path")
	}
}

func TestModeString(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModeWorkspace, "xcworkspace"},
		{ModeXcodeproj, "xcodeproj"},
		{ModeNative, "native"},
		{Mode(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
