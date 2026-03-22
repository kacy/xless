package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Mode describes how xless resolves project configuration.
type Mode int

const (
	// ModeXcodeproj uses .xcodeproj as source of truth, with optional xless.yml overlay.
	ModeXcodeproj Mode = iota
	// ModeNative uses xless.yml as the full configuration source.
	ModeNative
)

func (m Mode) String() string {
	switch m {
	case ModeXcodeproj:
		return "xcodeproj"
	case ModeNative:
		return "native"
	default:
		return "unknown"
	}
}

// DetectResult holds the outcome of project detection.
type DetectResult struct {
	Mode         Mode
	XcodeprojDir string // path to .xcodeproj directory, empty if not found
	ConfigFile   string // path to xless.yml, empty if not found
}

// Detect scans dir to determine the project mode.
//
// Detection matrix:
//
//	.xcodeproj found + xless.yml found  → ModeXcodeproj (xcodeproj is truth, yml is overlay)
//	.xcodeproj found + no xless.yml     → ModeXcodeproj (zero config needed)
//	no .xcodeproj    + xless.yml found  → ModeNative
//	no .xcodeproj    + no xless.yml     → error with hint
func Detect(dir string) (*DetectResult, error) {
	xcodeprojDir := findXcodeproj(dir)
	configFile := findConfigFile(dir)

	if xcodeprojDir == "" && configFile == "" {
		return nil, fmt.Errorf(
			"no .xcodeproj or xless.yml found in %s (run `xless init` to create a project, or open an existing xcode project directory)",
			dir,
		)
	}

	result := &DetectResult{
		XcodeprojDir: xcodeprojDir,
		ConfigFile:   configFile,
	}

	if xcodeprojDir != "" {
		result.Mode = ModeXcodeproj
	} else {
		result.Mode = ModeNative
	}

	return result, nil
}

// findXcodeproj returns the first .xcodeproj directory found in dir, or empty string.
func findXcodeproj(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".xcodeproj") {
			return filepath.Join(dir, entry.Name())
		}
	}

	return ""
}

// findConfigFile returns the path to xless.yml if it exists in dir, or empty string.
func findConfigFile(dir string) string {
	path := filepath.Join(dir, "xless.yml")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// also check xless.yaml
	path = filepath.Join(dir, "xless.yaml")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	return ""
}
