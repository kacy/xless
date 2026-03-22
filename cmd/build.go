package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kacyfortner/ios-build-cli/internal/build"
	"github.com/kacyfortner/ios-build-cli/internal/config"
	"github.com/kacyfortner/ios-build-cli/internal/output"
	"github.com/kacyfortner/ios-build-cli/internal/toolchain"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(buildCmd)
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "compile and bundle an ios app",
	Long:  "compiles swift source files, creates a .app bundle with Info.plist, and signs for the target platform.",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()

		dir, err := os.Getwd()
		if err != nil {
			out.Error("cannot determine working directory", "error", err.Error())
			return
		}

		flags := cliFlags()
		cfg, _, err := config.Load(dir, flags)
		if err != nil {
			out.Error(err.Error())
			return
		}

		// resolve target
		target := resolveTarget(cfg, flags)
		if target == nil {
			return
		}

		// resolve platform
		platform := resolvePlatform(flags)

		// resolve build config
		buildConfig := resolveBuildConfig(flags, cfg)

		// discover toolchain
		tc, err := discoverToolchain(cmd)
		if err != nil {
			return
		}

		// set up build context
		buildDir := filepath.Join(dir, ".build", target.Name)
		bc := &build.BuildContext{
			Ctx:         cmd.Context(),
			Config:      cfg,
			Target:      target,
			Toolchain:   tc,
			Platform:    platform,
			BuildConfig: buildConfig,
			ProjectDir:  dir,
			BuildDir:    buildDir,
			Out:         out,
		}

		out.Info("build", "target", target.Name, "platform", string(platform), "config", buildConfig)

		// run pipeline
		pipeline := build.NewPipeline(
			build.CompileStage{},
			build.BundleStage{},
			build.SignStage{},
		)

		if err := pipeline.Run(bc); err != nil {
			out.Error(err.Error())
			return
		}

		elapsed := time.Since(start)
		out.Success("build complete", "bundle", bc.AppBundlePath, "time", elapsed.Round(time.Millisecond).String())

		out.Data("build", output.OrderedMap{
			{Key: "target", Value: target.Name},
			{Key: "bundle_id", Value: target.BundleID},
			{Key: "platform", Value: string(platform)},
			{Key: "config", Value: buildConfig},
			{Key: "bundle", Value: bc.AppBundlePath},
			{Key: "time", Value: elapsed.Round(time.Millisecond).String()},
		})
	},
}

// resolveTarget picks the target from flags or config defaults.
func resolveTarget(cfg *config.ProjectConfig, flags config.CLIFlags) *config.TargetConfig {
	if flags.Target != "" {
		t := cfg.FindTarget(flags.Target)
		if t == nil {
			out.Error(fmt.Sprintf("target %q not found", flags.Target),
				"hint", "run `xless info` to see available targets")
			return nil
		}
		return t
	}
	t := cfg.DefaultTarget()
	if t == nil {
		out.Error("no targets found in project configuration")
		return nil
	}
	return t
}

// resolvePlatform returns the platform from flags or defaults to simulator.
func resolvePlatform(flags config.CLIFlags) toolchain.Platform {
	if flags.Platform == string(toolchain.PlatformDevice) {
		return toolchain.PlatformDevice
	}
	return toolchain.PlatformSimulator
}

// resolveBuildConfig returns the build config from flags or config defaults.
func resolveBuildConfig(flags config.CLIFlags, cfg *config.ProjectConfig) string {
	if flags.Config != "" {
		return flags.Config
	}
	if cfg.Defaults.Config != "" {
		return cfg.Defaults.Config
	}
	return "debug"
}

// discoverToolchain detects the Apple toolchain, logging errors via the formatter.
func discoverToolchain(cmd *cobra.Command) (toolchain.Toolchain, error) {
	info, err := toolchain.Discover(cmd.Context())
	if err != nil {
		out.Error("toolchain discovery failed", "error", err.Error(),
			"hint", "is xcode installed? run `xcode-select --install`")
		return nil, err
	}
	return toolchain.NewToolchain(info), nil
}
