package toolchain

// Platform represents a build platform (simulator or device).
type Platform string

const (
	PlatformSimulator Platform = "simulator"
	PlatformDevice    Platform = "device"
)

// Toolchain provides access to Apple build tools and SDK paths.
type Toolchain interface {
	SwiftcPath() string
	SDKPath(platform Platform) string
	SwiftVersion() string
	XcodeVersion() string
	Arch() string
}

// XcodeToolchain implements Toolchain using paths discovered from xcrun.
type XcodeToolchain struct {
	info *ToolchainInfo
}

// NewToolchain creates a Toolchain from discovered toolchain info.
func NewToolchain(info *ToolchainInfo) Toolchain {
	return &XcodeToolchain{info: info}
}

func (t *XcodeToolchain) SwiftcPath() string  { return t.info.SwiftcPath }
func (t *XcodeToolchain) SwiftVersion() string { return t.info.SwiftVersion }
func (t *XcodeToolchain) XcodeVersion() string { return t.info.XcodeVersion }
func (t *XcodeToolchain) Arch() string         { return t.info.Arch }

func (t *XcodeToolchain) SDKPath(platform Platform) string {
	switch platform {
	case PlatformDevice:
		return t.info.DeviceSDKPath
	case PlatformSimulator:
		return t.info.SimulatorSDKPath
	default:
		return t.info.SimulatorSDKPath
	}
}
