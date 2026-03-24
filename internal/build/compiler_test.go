package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kacy/xless/internal/config"
	"github.com/kacy/xless/internal/toolchain"
)

// mockToolchain implements toolchain.Toolchain for tests.
type mockToolchain struct {
	arch string
}

func (m *mockToolchain) SwiftcPath() string                  { return "/usr/bin/swiftc" }
func (m *mockToolchain) SDKPath(_ toolchain.Platform) string { return "/sdk" }
func (m *mockToolchain) SwiftVersion() string                { return "5.10" }
func (m *mockToolchain) XcodeVersion() string                { return "16.0" }
func (m *mockToolchain) Arch() string                        { return m.arch }

func TestResolveSwiftFilesDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Sources", "main.swift"), "// main")
	writeFile(t, filepath.Join(dir, "Sources", "app.swift"), "// app")
	writeFile(t, filepath.Join(dir, "Sources", "readme.md"), "# readme")

	files, err := resolveSwiftFiles(dir, []string{"Sources/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 swift files, got %d: %v", len(files), files)
	}
}

func TestResolveSwiftFilesIndividual(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.swift"), "// main")
	writeFile(t, filepath.Join(dir, "helper.swift"), "// helper")

	files, err := resolveSwiftFiles(dir, []string{"main.swift", "helper.swift"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestResolveSwiftFilesRejectsNonSwiftIndividualFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"), "# docs")

	_, err := resolveSwiftFiles(dir, []string{"README.md"})
	if err == nil {
		t.Fatal("expected error for non-swift source file")
	}
}

func TestResolveSwiftFilesEmptyDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Sources"), 0o755)

	_, err := resolveSwiftFiles(dir, []string{"Sources/"})
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
}

func TestResolveSwiftFilesMissingDir(t *testing.T) {
	dir := t.TempDir()

	_, err := resolveSwiftFiles(dir, []string{"NonExistent/"})
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestResolveSwiftFilesDedup(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Sources", "main.swift"), "// main")

	// list both the directory and the individual file
	files, err := resolveSwiftFiles(dir, []string{"Sources/", "Sources/main.swift"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file after dedup, got %d: %v", len(files), files)
	}
}

func TestResolveSwiftFilesNestedSubdirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Sources", "main.swift"), "// main")
	writeFile(t, filepath.Join(dir, "Sources", "Views", "content.swift"), "// content")
	writeFile(t, filepath.Join(dir, "Sources", "Models", "user.swift"), "// user")

	files, err := resolveSwiftFiles(dir, []string{"Sources/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files from nested dirs, got %d: %v", len(files), files)
	}
}

func TestBuildTriple(t *testing.T) {
	tests := []struct {
		name     string
		arch     string
		minIOS   string
		platform toolchain.Platform
		want     string
	}{
		{
			name:     "arm64 simulator",
			arch:     "arm64",
			minIOS:   "16.0",
			platform: toolchain.PlatformSimulator,
			want:     "arm64-apple-ios16.0-simulator",
		},
		{
			name:     "x86_64 simulator",
			arch:     "x86_64",
			minIOS:   "16.0",
			platform: toolchain.PlatformSimulator,
			want:     "x86_64-apple-ios16.0-simulator",
		},
		{
			name:     "arm64 device",
			arch:     "arm64",
			minIOS:   "17.0",
			platform: toolchain.PlatformDevice,
			want:     "arm64-apple-ios17.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTriple(tt.arch, tt.minIOS, tt.platform)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSwiftcArgsDebug(t *testing.T) {
	bc := &BuildContext{
		Platform:    toolchain.PlatformSimulator,
		BuildConfig: "debug",
		Toolchain:   &mockToolchain{arch: "arm64"},
		Target: &config.TargetConfig{
			Name:   "MyApp",
			MinIOS: "16.0",
		},
		BuildDir: ".build/MyApp",
	}
	bc.ExecutablePath = filepath.Join(bc.BuildDir, bc.Target.Name)

	args := buildSwiftcArgs(bc, []string{"main.swift"})

	assertContains(t, args, "-sdk", "/sdk")
	assertContains(t, args, "-target", "arm64-apple-ios16.0-simulator")
	assertContainsValue(t, args, "-Onone")
	assertContainsValue(t, args, "-g")
	assertContainsValue(t, args, "-DDEBUG")
	assertContainsValue(t, args, "-emit-executable")
	assertNotContainsValue(t, args, "-O")
}

func TestBuildSwiftcArgsRelease(t *testing.T) {
	bc := &BuildContext{
		Platform:    toolchain.PlatformSimulator,
		BuildConfig: "release",
		Toolchain:   &mockToolchain{arch: "arm64"},
		Target: &config.TargetConfig{
			Name:   "MyApp",
			MinIOS: "16.0",
		},
		BuildDir: ".build/MyApp",
	}
	bc.ExecutablePath = filepath.Join(bc.BuildDir, bc.Target.Name)

	args := buildSwiftcArgs(bc, []string{"main.swift"})

	assertContainsValue(t, args, "-O")
	assertNotContainsValue(t, args, "-Onone")
	assertNotContainsValue(t, args, "-g")
	assertNotContainsValue(t, args, "-DDEBUG")
}

func TestBuildSwiftcArgsDeviceTriple(t *testing.T) {
	bc := &BuildContext{
		Platform:    toolchain.PlatformDevice,
		BuildConfig: "debug",
		Toolchain:   &mockToolchain{arch: "arm64"},
		Target: &config.TargetConfig{
			Name:   "MyApp",
			MinIOS: "16.0",
		},
		BuildDir: ".build/MyApp",
	}
	bc.ExecutablePath = filepath.Join(bc.BuildDir, bc.Target.Name)

	args := buildSwiftcArgs(bc, []string{"main.swift"})

	assertContains(t, args, "-sdk", "/sdk")
	assertContains(t, args, "-target", "arm64-apple-ios16.0")
}

func TestBuildSwiftcArgsUserFlags(t *testing.T) {
	bc := &BuildContext{
		Platform:    toolchain.PlatformSimulator,
		BuildConfig: "debug",
		Toolchain:   &mockToolchain{arch: "arm64"},
		Target: &config.TargetConfig{
			Name:       "MyApp",
			MinIOS:     "16.0",
			SwiftFlags: []string{"-DXLESS_BUILD", "-warn-concurrency"},
		},
		BuildDir: ".build/MyApp",
	}
	bc.ExecutablePath = filepath.Join(bc.BuildDir, bc.Target.Name)

	args := buildSwiftcArgs(bc, []string{"main.swift"})

	assertContainsValue(t, args, "-DXLESS_BUILD")
	assertContainsValue(t, args, "-warn-concurrency")
}

// test helpers

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertContains(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return
		}
	}
	t.Errorf("args missing %s %s: %v", flag, value, args)
}

func assertContainsValue(t *testing.T, args []string, value string) {
	t.Helper()
	for _, a := range args {
		if a == value {
			return
		}
	}
	t.Errorf("args missing %s: %v", value, args)
}

func assertNotContainsValue(t *testing.T, args []string, value string) {
	t.Helper()
	for _, a := range args {
		if a == value {
			t.Errorf("args should not contain %s: %v", value, args)
			return
		}
	}
}
