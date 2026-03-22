package device

import (
	"context"
	"fmt"
)

// Simulator states reported by simctl.
const (
	StateBooted   = "Booted"
	StateShutdown = "Shutdown"
)

// Device represents a target device for app deployment.
type Device interface {
	Name() string
	UDID() string
	State() string
	Runtime() string
	Prepare(ctx context.Context) error
	Install(ctx context.Context, appPath string) error
	Launch(ctx context.Context, bundleID string) (pid string, err error)
}

// DeviceError is a device operation failure with an actionable hint.
type DeviceError struct {
	Op   string // "boot", "install", "launch", "list", "resolve"
	Err  error
	Hint string
}

func (e *DeviceError) Error() string {
	msg := fmt.Sprintf("%s: %s", e.Op, e.Err)
	if e.Hint != "" {
		msg += fmt.Sprintf(" (hint: %s)", e.Hint)
	}
	return msg
}

func (e *DeviceError) Unwrap() error {
	return e.Err
}
