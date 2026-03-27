package workspace

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Workspace describes a parsed .xcworkspace and the member project paths it references.
type Workspace struct {
	Path     string
	Projects []string
}

type workspaceXML struct {
	FileRefs []fileRefXML `xml:"FileRef"`
	Groups   []groupXML   `xml:"Group"`
}

type groupXML struct {
	FileRefs []fileRefXML `xml:"FileRef"`
	Groups   []groupXML   `xml:"Group"`
}

type fileRefXML struct {
	Location string `xml:"location,attr"`
}

// Parse reads an .xcworkspace directory and extracts referenced .xcodeproj paths.
func Parse(workspaceDir string) (*Workspace, error) {
	data, err := os.ReadFile(filepath.Join(workspaceDir, "contents.xcworkspacedata"))
	if err != nil {
		return nil, fmt.Errorf("reading workspace metadata: %w", err)
	}

	var raw workspaceXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing workspace metadata: %w", err)
	}

	seen := make(map[string]bool)
	var projects []string
	addLocation := func(location string) {
		path := resolveLocation(workspaceDir, location)
		if path == "" || !strings.HasSuffix(path, ".xcodeproj") {
			return
		}
		if !seen[path] {
			seen[path] = true
			projects = append(projects, path)
		}
	}

	collectFileRefs(raw.FileRefs, raw.Groups, addLocation)

	return &Workspace{
		Path:     workspaceDir,
		Projects: projects,
	}, nil
}

func collectFileRefs(fileRefs []fileRefXML, groups []groupXML, add func(string)) {
	for _, ref := range fileRefs {
		add(ref.Location)
	}
	for _, group := range groups {
		collectFileRefs(group.FileRefs, group.Groups, add)
	}
}

func resolveLocation(workspaceDir, location string) string {
	baseDir := filepath.Dir(workspaceDir)
	switch {
	case strings.HasPrefix(location, "group:"):
		return resolveWorkspacePath(baseDir, strings.TrimPrefix(location, "group:"))
	case strings.HasPrefix(location, "self:"):
		return resolveWorkspacePath(baseDir, strings.TrimPrefix(location, "self:"))
	case strings.HasPrefix(location, "container:"):
		return resolveWorkspacePath(baseDir, strings.TrimPrefix(location, "container:"))
	case strings.HasPrefix(location, "absolute:"):
		return filepath.Clean(strings.TrimPrefix(location, "absolute:"))
	default:
		return resolveWorkspacePath(baseDir, location)
	}
}

func resolveWorkspacePath(baseDir, raw string) string {
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	return filepath.Clean(filepath.Join(baseDir, raw))
}
