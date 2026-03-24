package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWorkspaceProjects(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "WeatherApp.xcworkspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	contents := `<?xml version="1.0" encoding="UTF-8"?>
<Workspace version="1.0">
    <FileRef location="group:ExampleApp.xcodeproj"></FileRef>
    <Group location="container:Nested">
        <FileRef location="group:Other/Other.xcodeproj"></FileRef>
    </Group>
</Workspace>`
	if err := os.WriteFile(filepath.Join(workspaceDir, "contents.xcworkspacedata"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ws, err := Parse(workspaceDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ws.Projects) != 2 {
		t.Fatalf("projects = %d, want 2", len(ws.Projects))
	}
	if ws.Projects[0] != filepath.Join(dir, "ExampleApp.xcodeproj") {
		t.Fatalf("first project = %q", ws.Projects[0])
	}
	if ws.Projects[1] != filepath.Join(dir, "Other", "Other.xcodeproj") {
		t.Fatalf("second project = %q", ws.Projects[1])
	}
}
