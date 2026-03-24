package xcodeproj

import "testing"

func TestResolveFilePathUsesNestedGroupPath(t *testing.T) {
	raw := &RawProject{
		RootObject: "project",
		Objects: map[string]map[string]any{
			"project": {
				"isa":       isaPBXProject,
				"mainGroup": "main",
			},
			"main": {
				"isa":      isaPBXGroup,
				"children": []any{"features"},
			},
			"features": {
				"isa":      isaPBXGroup,
				"path":     "Features",
				"children": []any{"login"},
			},
			"login": {
				"isa":      isaPBXGroup,
				"path":     "Login",
				"children": []any{"file"},
			},
			"file": {
				"isa":        isaPBXFileReference,
				"path":       "View.swift",
				"sourceTree": "<group>",
			},
		},
	}

	got := resolveFilePath(raw.Objects["file"], "file", buildGroupPaths(raw))
	if got != "Features/Login/View.swift" {
		t.Fatalf("path = %q, want %q", got, "Features/Login/View.swift")
	}
}
