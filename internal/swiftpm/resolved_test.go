package swiftpm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseResolvedV2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Package.resolved")
	data := `{
  "pins": [
    {
      "identity": "swift-collections",
      "kind": "remoteSourceControl",
      "location": "https://github.com/apple/swift-collections.git",
      "state": {
        "revision": "abc123",
        "version": "1.1.0"
      }
    }
  ],
  "version": 2
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	resolved, err := ParseResolved(path)
	if err != nil {
		t.Fatalf("ParseResolved: %v", err)
	}
	if len(resolved.Packages) != 1 {
		t.Fatalf("packages = %d, want 1", len(resolved.Packages))
	}

	pkg := resolved.Packages[0]
	if pkg.Identity != "swift-collections" {
		t.Fatalf("identity = %q", pkg.Identity)
	}
	if pkg.Location != "https://github.com/apple/swift-collections.git" {
		t.Fatalf("location = %q", pkg.Location)
	}
	if pkg.Version != "1.1.0" {
		t.Fatalf("version = %q", pkg.Version)
	}
	if pkg.Revision != "abc123" {
		t.Fatalf("revision = %q", pkg.Revision)
	}
}

func TestParseResolvedV1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Package.resolved")
	data := `{
  "object": {
    "pins": [
      {
        "package": "Alamofire",
        "repositoryURL": "https://github.com/Alamofire/Alamofire.git",
        "state": {
          "branch": "main",
          "revision": "def456"
        }
      }
    ]
  },
  "version": 1
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	resolved, err := ParseResolved(path)
	if err != nil {
		t.Fatalf("ParseResolved: %v", err)
	}
	if len(resolved.Packages) != 1 {
		t.Fatalf("packages = %d, want 1", len(resolved.Packages))
	}

	pkg := resolved.Packages[0]
	if pkg.Identity != "Alamofire" {
		t.Fatalf("identity = %q", pkg.Identity)
	}
	if pkg.Location != "https://github.com/Alamofire/Alamofire.git" {
		t.Fatalf("location = %q", pkg.Location)
	}
	if pkg.Branch != "main" {
		t.Fatalf("branch = %q", pkg.Branch)
	}
	if pkg.Revision != "def456" {
		t.Fatalf("revision = %q", pkg.Revision)
	}
}

func TestFindResolvedPaths(t *testing.T) {
	dir := t.TempDir()

	workspaceResolved := filepath.Join(dir, "App.xcworkspace", "xcshareddata", "swiftpm", "Package.resolved")
	if err := os.MkdirAll(filepath.Dir(workspaceResolved), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(workspaceResolved, []byte(`{"pins":[]}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	xcodeprojResolved := filepath.Join(dir, "App.xcodeproj", "project.xcworkspace", "xcshareddata", "swiftpm", "Package.resolved")
	if err := os.MkdirAll(filepath.Dir(xcodeprojResolved), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(xcodeprojResolved, []byte(`{"pins":[]}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := FindResolvedForWorkspace(filepath.Join(dir, "App.xcworkspace")); got != workspaceResolved {
		t.Fatalf("workspace resolved path = %q, want %q", got, workspaceResolved)
	}
	if got := FindResolvedForXcodeproj(filepath.Join(dir, "App.xcodeproj")); got != xcodeprojResolved {
		t.Fatalf("xcodeproj resolved path = %q, want %q", got, xcodeprojResolved)
	}
}
