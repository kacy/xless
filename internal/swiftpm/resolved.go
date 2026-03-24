package swiftpm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolvedFile describes a parsed Package.resolved file.
type ResolvedFile struct {
	Path     string
	Packages []ResolvedPackage
}

// ResolvedPackage is a normalized package pin from Package.resolved.
type ResolvedPackage struct {
	Identity string
	Location string
	Version  string
	Revision string
	Branch   string
}

type resolvedV1 struct {
	Object struct {
		Pins []resolvedPin `json:"pins"`
	} `json:"object"`
}

type resolvedV2 struct {
	Pins []resolvedPin `json:"pins"`
}

type resolvedPin struct {
	Identity      string `json:"identity"`
	Package       string `json:"package"`
	Location      string `json:"location"`
	RepositoryURL string `json:"repositoryURL"`
	State         struct {
		Branch   string `json:"branch"`
		Revision string `json:"revision"`
		Version  string `json:"version"`
	} `json:"state"`
}

// ParseResolved reads and normalizes a Package.resolved file.
func ParseResolved(path string) (*ResolvedFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading Package.resolved: %w", err)
	}

	var v2 resolvedV2
	if err := json.Unmarshal(data, &v2); err == nil && len(v2.Pins) > 0 {
		return &ResolvedFile{Path: path, Packages: normalizePins(v2.Pins)}, nil
	}

	var v1 resolvedV1
	if err := json.Unmarshal(data, &v1); err == nil {
		return &ResolvedFile{Path: path, Packages: normalizePins(v1.Object.Pins)}, nil
	}

	return nil, fmt.Errorf("parsing Package.resolved: unsupported or invalid schema")
}

func normalizePins(pins []resolvedPin) []ResolvedPackage {
	packages := make([]ResolvedPackage, 0, len(pins))
	for _, pin := range pins {
		location := pin.Location
		if location == "" {
			location = pin.RepositoryURL
		}

		identity := pin.Identity
		if identity == "" {
			identity = pin.Package
		}
		if identity == "" {
			identity = inferIdentity(location)
		}

		packages = append(packages, ResolvedPackage{
			Identity: identity,
			Location: location,
			Version:  pin.State.Version,
			Revision: pin.State.Revision,
			Branch:   pin.State.Branch,
		})
	}
	return packages
}

func inferIdentity(location string) string {
	location = strings.TrimSpace(location)
	location = strings.TrimSuffix(location, "/")
	location = strings.TrimSuffix(location, ".git")
	if location == "" {
		return ""
	}
	return filepath.Base(location)
}

// FindResolvedForWorkspace returns the standard workspace Package.resolved path if it exists.
func FindResolvedForWorkspace(workspaceDir string) string {
	return firstExisting(
		filepath.Join(workspaceDir, "xcshareddata", "swiftpm", "Package.resolved"),
	)
}

// FindResolvedForXcodeproj returns the standard project Package.resolved path if it exists.
func FindResolvedForXcodeproj(xcodeprojDir string) string {
	return firstExisting(
		filepath.Join(xcodeprojDir, "project.xcworkspace", "xcshareddata", "swiftpm", "Package.resolved"),
	)
}

func firstExisting(paths ...string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
