package device

import (
	"context"
	"fmt"
	"strings"
)

// ResolveSimulator finds a simulator by specifier, falling back to defaults and auto-detection.
// Resolution order:
//  1. specifier looks like UDID → match by UDID
//  2. specifier is non-empty → match by name (case-insensitive)
//  3. specifier empty, try defaultName → match by name
//  4. first booted simulator
//  5. first available iPhone simulator
//  6. first available simulator
func ResolveSimulator(ctx context.Context, specifier, defaultName string) (Device, error) {
	sims, err := ListSimulators(ctx)
	if err != nil {
		return nil, err
	}

	return resolveFromList(sims, specifier, defaultName)
}

// resolveFromList is the testable core of ResolveSimulator.
func resolveFromList(sims []SimulatorInfo, specifier, defaultName string) (Device, error) {
	if len(sims) == 0 {
		return nil, &DeviceError{
			Op:   "resolve",
			Err:  fmt.Errorf("no simulators available"),
			Hint: "open xcode and install a simulator runtime, or run `xcodebuild -downloadPlatform iOS`",
		}
	}

	// 1. specifier looks like UDID
	if looksLikeUDID(specifier) {
		for _, s := range sims {
			if strings.EqualFold(s.UDID, specifier) {
				return NewSimulator(s), nil
			}
		}
		return nil, &DeviceError{
			Op:   "resolve",
			Err:  fmt.Errorf("no simulator with UDID %q", specifier),
			Hint: "run `xless devices` to see available simulators",
		}
	}

	// 2. specifier is a name
	if specifier != "" {
		if d := findByName(sims, specifier); d != nil {
			return d, nil
		}
		return nil, &DeviceError{
			Op:   "resolve",
			Err:  fmt.Errorf("no simulator named %q", specifier),
			Hint: "run `xless devices` to see available simulators",
		}
	}

	// 3. try defaultName from config
	if defaultName != "" {
		if d := findByName(sims, defaultName); d != nil {
			return d, nil
		}
		// fall through to auto-detection
	}

	// 4. first booted simulator
	for _, s := range sims {
		if s.State == StateBooted {
			return NewSimulator(s), nil
		}
	}

	// 5. first available iPhone
	for _, s := range sims {
		if strings.HasPrefix(s.Name, "iPhone") {
			return NewSimulator(s), nil
		}
	}

	// 6. first available simulator
	return NewSimulator(sims[0]), nil
}

// findByName returns the first simulator matching name (case-insensitive).
func findByName(sims []SimulatorInfo, name string) Device {
	for _, s := range sims {
		if strings.EqualFold(s.Name, name) {
			return NewSimulator(s)
		}
	}
	return nil
}

// looksLikeUDID returns true if s resembles a UUID (36 chars, 4 hyphens).
func looksLikeUDID(s string) bool {
	return len(s) == 36 && strings.Count(s, "-") == 4
}
