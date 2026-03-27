package config

// CLIFlags holds flag values that override config file settings.
type CLIFlags struct {
	Platform    string // "simulator" or "device"
	Target      string // target name
	Scheme      string // xcode scheme override
	BuildConfig string // build configuration name, e.g. Debug or Release
	Device      string // device name or UDID
	ConfigPath  string // path to xless.yml/xless.yaml
}
