package xcodeproj

import (
	"fmt"
	"os"
	"path/filepath"

	"howett.net/plist"
)

// Parse reads a .xcodeproj directory and returns the raw deserialized pbxproj.
func Parse(xcodeprojDir string) (*RawProject, error) {
	pbxprojPath := filepath.Join(xcodeprojDir, "project.pbxproj")

	data, err := os.ReadFile(pbxprojPath)
	if err != nil {
		return nil, fmt.Errorf("reading pbxproj: %w (is this a valid .xcodeproj directory?)", err)
	}

	// the pbxproj is an OpenStep-format plist. howett.net/plist handles
	// all three formats (OpenStep, XML, binary) transparently.
	var raw map[string]any
	if _, err := plist.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing pbxproj: %w", err)
	}

	rootObject := stringFromAny(raw["rootObject"])
	if rootObject == "" {
		return nil, fmt.Errorf("pbxproj missing rootObject")
	}

	objectsRaw, ok := raw["objects"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("pbxproj missing objects table")
	}

	// convert the objects table from map[string]any to map[string]map[string]any
	objects := make(map[string]map[string]any, len(objectsRaw))
	for id, v := range objectsRaw {
		if obj, ok := v.(map[string]any); ok {
			objects[id] = obj
		}
	}

	return &RawProject{
		ArchiveVersion: stringFromAny(raw["archiveVersion"]),
		ObjectVersion:  stringFromAny(raw["objectVersion"]),
		RootObject:     rootObject,
		Objects:        objects,
	}, nil
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
