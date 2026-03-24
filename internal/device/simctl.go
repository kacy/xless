package device

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kacy/xless/internal/toolchain"
)

const listTimeout = 15 * time.Second

// SimulatorInfo is a flattened representation of a simulator from simctl.
// only available simulators are returned by ListSimulators.
type SimulatorInfo struct {
	Name    string
	UDID    string
	State   string // StateBooted, StateShutdown, etc.
	Runtime string // human-readable, e.g. "iOS 18.2"
}

// simctlDeviceList is the top-level JSON structure from `simctl list --json devices`.
type simctlDeviceList struct {
	Devices map[string][]simctlDevice `json:"devices"`
}

// simctlDevice is a single device entry from simctl JSON output.
type simctlDevice struct {
	Name        string `json:"name"`
	UDID        string `json:"udid"`
	State       string `json:"state"`
	IsAvailable bool   `json:"isAvailable"`
}

// ListSimulators calls simctl and returns available iOS simulators.
func ListSimulators(ctx context.Context) ([]SimulatorInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	result, err := toolchain.RunCommand(ctx, "xcrun", "simctl", "list", "--json", "devices", "available")
	if err != nil {
		return nil, &DeviceError{
			Op:   "list",
			Err:  fmt.Errorf("simctl list failed: %w", err),
			Hint: "is xcode installed? run `xcode-select --install`",
		}
	}

	sims, err := parseSimulatorList(result.Stdout)
	if err != nil {
		return nil, &DeviceError{
			Op:  "list",
			Err: fmt.Errorf("failed to parse simctl output: %w", err),
		}
	}

	return sims, nil
}

// parseSimulatorList parses the JSON from `simctl list --json devices available`
// and returns only iOS simulators.
func parseSimulatorList(jsonData string) ([]SimulatorInfo, error) {
	var list simctlDeviceList
	if err := json.Unmarshal([]byte(jsonData), &list); err != nil {
		return nil, fmt.Errorf("invalid simctl json: %w", err)
	}

	var sims []SimulatorInfo
	for runtimeID, devices := range list.Devices {
		if !isIOSRuntime(runtimeID) {
			continue
		}
		runtimeName := parseRuntimeName(runtimeID)
		for _, d := range devices {
			if !d.IsAvailable {
				continue
			}
			sims = append(sims, SimulatorInfo{
				Name:    d.Name,
				UDID:    d.UDID,
				State:   d.State,
				Runtime: runtimeName,
			})
		}
	}

	return sims, nil
}

// isIOSRuntime returns true if the runtime identifier is an iOS runtime.
func isIOSRuntime(runtimeID string) bool {
	// format: com.apple.CoreSimulator.SimRuntime.iOS-18-2
	return strings.Contains(runtimeID, ".iOS-")
}

// parseRuntimeName converts a runtime identifier to a human-readable name.
// "com.apple.CoreSimulator.SimRuntime.iOS-18-2" → "iOS 18.2"
func parseRuntimeName(runtimeID string) string {
	// extract everything after the last dot: "iOS-18-2"
	idx := strings.LastIndex(runtimeID, ".")
	if idx < 0 || idx >= len(runtimeID)-1 {
		return runtimeID
	}
	suffix := runtimeID[idx+1:]

	// split on first dash to separate platform from version: "iOS" + "18-2"
	parts := strings.SplitN(suffix, "-", 2)
	if len(parts) != 2 {
		return suffix
	}

	platform := parts[0]
	version := strings.ReplaceAll(parts[1], "-", ".")
	return platform + " " + version
}

// parseLaunchPID extracts the PID from simctl launch output.
// input: "com.example.MyApp: 12345\n" → output: "12345"
func parseLaunchPID(output string) string {
	output = strings.TrimSpace(output)
	idx := strings.LastIndex(output, ": ")
	if idx < 0 || idx >= len(output)-2 {
		return ""
	}
	return strings.TrimSpace(output[idx+2:])
}
