package config

// CLIFlags holds flag values that override config file settings.
type CLIFlags struct {
	Platform    string // "simulator" or "device"
	Target      string // target name
	BuildConfig string // "debug" or "release"
	Device      string // device name or UDID
	ConfigPath  string // path to xless.yml/xless.yaml
}
