package cmd

import (
	"runtime"

	"github.com/kacyfortner/ios-build-cli/internal/output"
	"github.com/kacyfortner/ios-build-cli/internal/toolchain"
	"github.com/spf13/cobra"
)

// cliVersion is set by Execute() from the value injected via ldflags.
var cliVersion = "dev"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print cli and toolchain versions",
	Run: func(cmd *cobra.Command, args []string) {
		info, err := toolchain.Discover(cmd.Context())

		versions := output.OrderedMap{
			{Key: "xless", Value: cliVersion},
			{Key: "go", Value: runtime.Version()},
			{Key: "arch", Value: info.Arch},
		}

		if info.SwiftVersion != "" {
			versions = append(versions, output.KV{Key: "swift", Value: info.SwiftVersion})
		}
		if info.XcodeVersion != "" {
			versions = append(versions, output.KV{Key: "xcode", Value: info.XcodeVersion})
		}
		if info.SimulatorSDKPath != "" {
			versions = append(versions, output.KV{Key: "simulator sdk", Value: info.SimulatorSDKPath})
		}
		if info.DeviceSDKPath != "" {
			versions = append(versions, output.KV{Key: "device sdk", Value: info.DeviceSDKPath})
		}

		out.Data("version", versions)

		if err != nil {
			out.Warn("incomplete toolchain detection", "hint", "is xcode installed? run `xcode-select -p` to check")
		}
	},
}
