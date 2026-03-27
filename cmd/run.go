package cmd

import (
	"fmt"
	"time"

	"github.com/kacy/xless/internal/build"
	"github.com/kacy/xless/internal/device"
	"github.com/kacy/xless/internal/output"
	"github.com/kacy/xless/internal/toolchain"
	"github.com/spf13/cobra"
)

func init() {
	runCmd.Flags().Bool("logs", false, "tail app logs after launch")
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "build, install, and launch on a simulator or device",
	Long:  "compiles the app, installs it on a simulator or device, and launches it. equivalent to build + install + launch.",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()

		flags := cliFlags()

		// build phase
		bc, cfg, _, ok := buildApp(cmd)
		if !ok {
			return
		}

		out.Success("build complete", "bundle", bc.AppBundlePath)

		// deploy phase — use bc.Platform which was already resolved during build
		var dev device.Device
		var err error

		if bc.Platform == toolchain.PlatformDevice {
			out.Info("resolving device")
			dev, err = device.ResolvePhysicalDevice(cmd.Context(), flags.Device, cfg.Defaults.Device)
		} else {
			out.Info("resolving simulator")
			dev, err = device.ResolveSimulator(cmd.Context(), flags.Device, cfg.Defaults.Simulator)
		}
		if err != nil {
			out.Error(err.Error())
			return
		}

		out.Info("preparing", "device", dev.Name(), "udid", dev.UDID())

		if err := dev.Prepare(cmd.Context()); err != nil {
			out.Error(err.Error())
			return
		}

		artifactPath, err := deployArtifactPath(bc)
		if err != nil {
			out.Error(err.Error())
			return
		}

		out.Info("installing", "artifact", artifactPath, "device", dev.Name())

		if err := dev.Install(cmd.Context(), artifactPath); err != nil {
			out.Error(err.Error())
			return
		}

		out.Info("launching", "bundle_id", bc.Target.BundleID, "device", dev.Name())

		pid, err := dev.Launch(cmd.Context(), bc.Target.BundleID)
		if err != nil {
			out.Error(err.Error())
			return
		}

		elapsed := time.Since(start)
		out.Success("app launched",
			"device", dev.Name(),
			"pid", pid,
			"time", elapsed.Round(time.Millisecond).String(),
		)

		runData := output.OrderedMap{
			{Key: "target", Value: bc.Target.Name},
			{Key: "bundle_id", Value: bc.Target.BundleID},
			{Key: "platform", Value: string(bc.Platform)},
			{Key: "config", Value: bc.BuildConfig},
			{Key: "device", Value: dev.Name()},
			{Key: "udid", Value: dev.UDID()},
			{Key: "pid", Value: pid},
			{Key: "time", Value: elapsed.Round(time.Millisecond).String()},
		}
		if bc.XcodeSchemeResolved != "" {
			runData = append(runData, output.KV{Key: "scheme", Value: bc.XcodeSchemeResolved})
		}
		out.Data("run", runData)
		if bc.XcodeSelectorFlag != "" && bc.XcodeSelectorValue != "" {
			out.Data("xcodebuild", output.OrderedMap{
				{Key: "scheme", Value: bc.XcodeSchemeResolved},
				{Key: "selector", Value: bc.XcodeSelectorFlag + "=" + bc.XcodeSelectorValue},
			})
		}

		// stream logs if requested
		logs, _ := cmd.Flags().GetBool("logs")
		if logs {
			if bc.Platform == toolchain.PlatformDevice {
				out.Warn("log streaming is not yet supported for physical devices")
				return
			}
			streamLogs(cmd, dev.UDID(), bc.Target.BundleID, bc.Target.Name, "")
		}
	},
}

func deployArtifactPath(bc *build.BuildContext) (string, error) {
	if bc.Platform == toolchain.PlatformDevice {
		if bc.IPAPath == "" {
			return "", fmt.Errorf("device deploy artifact missing: expected IPA after delegated archive/export or native package stage")
		}
		return bc.IPAPath, nil
	}

	if bc.AppBundlePath == "" {
		return "", fmt.Errorf("simulator deploy artifact missing: expected app bundle after bundle stage")
	}

	return bc.AppBundlePath, nil
}
