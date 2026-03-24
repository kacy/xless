package device

import (
	"context"
	"fmt"

	"github.com/kacy/xless/internal/toolchain"
)

// PhysicalDevice implements the Device interface for connected iOS devices via devicectl.
type PhysicalDevice struct {
	info PhysicalDeviceInfo
}

// NewPhysicalDevice creates a Device from a PhysicalDeviceInfo.
func NewPhysicalDevice(info PhysicalDeviceInfo) *PhysicalDevice {
	return &PhysicalDevice{info: info}
}

func (d *PhysicalDevice) Name() string { return d.info.Name }
func (d *PhysicalDevice) UDID() string { return d.info.UDID }
func (d *PhysicalDevice) State() string {
	if d.info.Connected {
		return "Connected"
	}
	return "Disconnected"
}
func (d *PhysicalDevice) Runtime() string { return d.info.OSVersion }

// Prepare is a no-op for physical devices — they're already booted.
func (d *PhysicalDevice) Prepare(_ context.Context) error {
	return nil
}

// Install copies an .app bundle to the physical device via devicectl.
func (d *PhysicalDevice) Install(ctx context.Context, appPath string) error {
	_, err := toolchain.RunCommand(ctx, "xcrun", "devicectl",
		"device", "install", "app", "--device", d.info.UDID, appPath)
	if err != nil {
		return &DeviceError{
			Op:   "install",
			Err:  fmt.Errorf("failed to install on %s: %w", d.info.Name, err),
			Hint: "ensure the device is connected and unlocked, and the app is properly signed",
		}
	}
	return nil
}

// Launch starts the app on the physical device via devicectl.
func (d *PhysicalDevice) Launch(ctx context.Context, bundleID string) (string, error) {
	_, err := toolchain.RunCommand(ctx, "xcrun", "devicectl",
		"device", "process", "launch", "--device", d.info.UDID, bundleID)
	if err != nil {
		return "", &DeviceError{
			Op:   "launch",
			Err:  fmt.Errorf("failed to launch %s on %s: %w", bundleID, d.info.Name, err),
			Hint: fmt.Sprintf("is %q installed? try `xless run --platform device` to build and install first", bundleID),
		}
	}

	// devicectl doesn't reliably report PID in the same way as simctl
	return "", nil
}
