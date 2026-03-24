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

		bc, cfg, ok := buildApp(cmd)
		if !ok {
			return
		}

		elapsed := time.Since(start)
		_ = cfg // used only by run for deploy phase

		artifact := bc.AppBundlePath
		if bc.IPAPath != "" {
			artifact = bc.IPAPath
		}
		out.Success("build complete", "output", artifact, "time", elapsed.Round(time.Millisecond).String())

		data := output.OrderedMap{
			{Key: "target", Value: bc.Target.Name},
			{Key: "bundle_id", Value: bc.Target.BundleID},
			{Key: "platform", Value: string(bc.Platform)},
			{Key: "config", Value: bc.BuildConfig},
			{Key: "bundle", Value: bc.AppBundlePath},
		}
		if bc.IPAPath != "" {
			data = append(data, output.KV{Key: "ipa", Value: bc.IPAPath})
		}
		data = append(data, output.KV{Key: "time", Value: elapsed.Round(time.Millisecond).String()})
		out.Data("build", data)
	},
}

// buildApp runs the standard build pipeline (compile → bundle → sign).
// returns the build context, loaded config, and true on success.
// on failure, errors are printed to out and false is returned.
func buildApp(cmd *cobra.Command) (*build.BuildContext, *config.ProjectConfig, bool) {
	dir, err := os.Getwd()
	if err != nil {
		out.Error("cannot determine working directory", "error", err.Error())
		return nil, nil, false
	}

	flags := cliFlags()
	cfg, _, err := config.Load(dir, flags)
	if err != nil {
		out.Error(err.Error())
		return nil, nil, false
	}

	target := resolveTarget(cfg, flags)
	if target == nil {
		return nil, nil, false
	}

	platform := resolvePlatform(flags)
	buildConfig := resolveBuildConfig(flags, cfg)

	tc, err := discoverToolchain(cmd)
	if err != nil {
		return nil, nil, false
	}

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

	stages := []build.Stage{
		build.CompileStage{},
		build.BundleStage{},
		build.SignStage{},
	}
	if platform == toolchain.PlatformDevice {
		stages = append(stages, build.PackageStage{})
	}
	pipeline := build.NewPipeline(stages...)

	if err := pipeline.Run(bc); err != nil {
		out.Error(err.Error())
		return nil, nil, false
	}

	return bc, cfg, true
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
