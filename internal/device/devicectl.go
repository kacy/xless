package device

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kacyfortner/ios-build-cli/internal/toolchain"
)

// PhysicalDeviceInfo describes a connected physical iOS device.
type PhysicalDeviceInfo struct {
	Name          string
	UDID          string
	DeviceType    string // "iPhone", "iPad"
	OSVersion     string // "iOS" (platform reported by devicectl)
	TransportType string // "wired", "localNetwork"
	Connected     bool
}

// ListPhysicalDevices calls devicectl and returns connected iOS devices.
func ListPhysicalDevices(ctx context.Context) ([]PhysicalDeviceInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	// devicectl writes JSON to a file (apple's recommended approach)
	tmpDir, err := os.MkdirTemp("", "xless-devicectl-*")
	if err != nil {
		return nil, &DeviceError{
			Op:  "list",
			Err: fmt.Errorf("cannot create temp dir: %w", err),
		}
	}
	defer os.RemoveAll(tmpDir)

	outFile := filepath.Join(tmpDir, "devices.json")

	_, err = toolchain.RunCommand(ctx, "xcrun", "devicectl", "list", "devices",
		"--quiet", "--json-output", outFile)
	if err != nil {
		return nil, &DeviceError{
			Op:   "list",
			Err:  fmt.Errorf("devicectl list failed: %w", err),
			Hint: "is xcode 15+ installed? devicectl requires xcode 15 or later",
		}
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		return nil, &DeviceError{
			Op:  "list",
			Err: fmt.Errorf("cannot read devicectl output: %w", err),
		}
	}

	devices, err := parseDevicectlOutput(data)
	if err != nil {
		return nil, &DeviceError{
			Op:  "list",
			Err: fmt.Errorf("cannot parse devicectl output: %w", err),
		}
	}

	return devices, nil
}

// devicectl JSON schema (subset we care about)
type devicectlOutput struct {
	Result struct {
		Devices []devicectlDevice `json:"devices"`
	} `json:"result"`
}

type devicectlDevice struct {
	Capabilities []devicectlCapability `json:"capabilities"`
	DeviceProperties struct {
		Name string `json:"name"`
	} `json:"deviceProperties"`
	HardwareProperties struct {
		UDID        string `json:"udid"`
		DeviceType  string `json:"deviceType"`
		Platform    string `json:"platform"`
	} `json:"hardwareProperties"`
	ConnectionProperties struct {
		TransportType string `json:"transportType"`
		TunnelState   string `json:"tunnelState"`
	} `json:"connectionProperties"`
	VisibilityClass string `json:"visibilityClass"`
}

type devicectlCapability struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// parseDevicectlOutput parses the JSON from devicectl and returns iOS devices.
func parseDevicectlOutput(data []byte) ([]PhysicalDeviceInfo, error) {
	var out devicectlOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("invalid devicectl json: %w", err)
	}

	var devices []PhysicalDeviceInfo
	for _, d := range out.Result.Devices {
		// filter to iOS devices only
		if !isIOSDevice(d) {
			continue
		}

		connected := d.ConnectionProperties.TunnelState == "connected" ||
			d.VisibilityClass == "default"

		devices = append(devices, PhysicalDeviceInfo{
			Name:          d.DeviceProperties.Name,
			UDID:          d.HardwareProperties.UDID,
			DeviceType:    d.HardwareProperties.DeviceType,
			OSVersion:     d.HardwareProperties.Platform,
			TransportType: d.ConnectionProperties.TransportType,
			Connected:     connected,
		})
	}

	return devices, nil
}

// isIOSDevice returns true if the device is an iOS device (iPhone or iPad).
func isIOSDevice(d devicectlDevice) bool {
	platform := d.HardwareProperties.Platform
	return platform == "iOS"
}

