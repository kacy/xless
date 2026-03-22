package device

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kacyfortner/ios-build-cli/internal/toolchain"
)

const bootTimeout = 60 * time.Second

// Simulator implements the Device interface using simctl.
type Simulator struct {
	info SimulatorInfo
}

// NewSimulator creates a Device from a SimulatorInfo.
func NewSimulator(info SimulatorInfo) *Simulator {
	return &Simulator{info: info}
}

func (s *Simulator) Name() string    { return s.info.Name }
func (s *Simulator) UDID() string    { return s.info.UDID }
func (s *Simulator) State() string   { return s.info.State }
func (s *Simulator) Runtime() string { return s.info.Runtime }

// Prepare boots the simulator if it is not already booted.
func (s *Simulator) Prepare(ctx context.Context) error {
	if s.info.State == StateBooted {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, bootTimeout)
	defer cancel()

	_, err := toolchain.RunCommand(ctx, "xcrun", "simctl", "boot", s.info.UDID)
	if err != nil {
		// simctl returns an error if already booted — treat as success
		if strings.Contains(err.Error(), "Unable to boot device in current state: Booted") {
			s.info.State = StateBooted
			return nil
		}
		return &DeviceError{
			Op:   "boot",
			Err:  fmt.Errorf("failed to boot %s: %w", s.info.Name, err),
			Hint: "try `xcrun simctl shutdown all` then retry",
		}
	}

	s.info.State = StateBooted
	return nil
}

// Install copies an .app bundle into the simulator.
func (s *Simulator) Install(ctx context.Context, appPath string) error {
	_, err := toolchain.RunCommand(ctx, "xcrun", "simctl", "install", s.info.UDID, appPath)
	if err != nil {
		return &DeviceError{
			Op:   "install",
			Err:  fmt.Errorf("failed to install on %s: %w", s.info.Name, err),
			Hint: "ensure the simulator is booted and the .app bundle is valid",
		}
	}
	return nil
}

// Launch starts the app on the simulator and returns its PID.
func (s *Simulator) Launch(ctx context.Context, bundleID string) (string, error) {
	result, err := toolchain.RunCommand(ctx, "xcrun", "simctl", "launch", s.info.UDID, bundleID)
	if err != nil {
		return "", &DeviceError{
			Op:   "launch",
			Err:  fmt.Errorf("failed to launch %s on %s: %w", bundleID, s.info.Name, err),
			Hint: fmt.Sprintf("is %q installed? try `xless run` to build and install first", bundleID),
		}
	}

	pid := parseLaunchPID(result.Stdout)
	return pid, nil
}
