package cmd

import (
	"path/filepath"
	"time"

	"github.com/kacy/xless/internal/build"
	"github.com/kacy/xless/internal/config"
	"github.com/kacy/xless/internal/output"
	"github.com/kacy/xless/internal/project"
	"github.com/kacy/xless/internal/toolchain"
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

		bc, cfg, det, ok := buildApp(cmd)
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
			{Key: "backend", Value: buildBackendLabel(shouldDelegateBuild(det))},
			{Key: "bundle", Value: bc.AppBundlePath},
		}
		if bc.XcodeSchemeResolved != "" {
			data = append(data, output.KV{Key: "scheme", Value: bc.XcodeSchemeResolved})
		}
		if bc.XcodeSelectorFlag != "" && bc.XcodeSelectorValue != "" {
			data = append(data, output.KV{Key: "xcode_selector", Value: bc.XcodeSelectorFlag + "=" + bc.XcodeSelectorValue})
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
func buildApp(cmd *cobra.Command) (*build.BuildContext, *config.ProjectConfig, *project.DetectResult, bool) {
	flags := cliFlags()
	dir, cfg, det, err := loadProject(flags)
	if err != nil {
		out.Error(err.Error())
		return nil, nil, nil, false
	}

	target, err := selectedTarget(cfg, flags)
	if err != nil {
		printTargetSelectionError(err)
		return nil, nil, nil, false
	}
	if det.Mode == project.ModeNative {
		if err := config.ValidateTargetSupport(target); err != nil {
			out.Error(err.Error())
			return nil, nil, nil, false
		}
	}

	platform := resolvePlatform(flags)
	buildConfig := resolveBuildConfig(flags, cfg)
	buildDir := filepath.Join(dir, ".build", target.Name)
	bc := &build.BuildContext{
		Ctx:          cmd.Context(),
		Config:       cfg,
		Target:       target,
		Platform:     platform,
		BuildConfig:  buildConfig,
		XcodeScheme:  flags.Scheme,
		RootDir:      dir,
		WorkspaceDir: det.WorkspaceDir,
		XcodeprojDir: det.XcodeprojDir,
		ProjectDir:   dir,
		BuildDir:     buildDir,
		Out:          out,
	}

	if target.SourceRoot != "" {
		bc.ProjectDir = target.SourceRoot
	}

	backend := "native"
	if shouldDelegateBuild(det) {
		backend = "xcodebuild"
	}

	if shouldDelegateBuild(det) {
		selection, err := build.ResolveXcodebuildSelection(cmd.Context(), det.WorkspaceDir, det.XcodeprojDir, target.Name, flags.Scheme)
		if err != nil {
			if hint := build.XcodebuildSelectionHint(err); hint != "" {
				out.Error("xcodebuild: "+err.Error(), "hint", hint)
			} else {
				out.Error("xcodebuild: " + err.Error())
			}
			return nil, nil, nil, false
		}
		bc.XcodeSchemeResolved = selection.Scheme
		bc.XcodeSelectorFlag = selection.Flag
		bc.XcodeSelectorValue = selection.Value
		out.Info("build", "target", target.Name, "platform", string(platform), "config", buildConfig, "backend", backend, "scheme", bc.XcodeSchemeResolved)
		if err := build.NewPipeline(build.XcodebuildBuildStage{}).Run(bc); err != nil {
			out.Error(err.Error())
			return nil, nil, nil, false
		}
		return bc, cfg, det, true
	}

	out.Info("build", "target", target.Name, "platform", string(platform), "config", buildConfig, "backend", backend)

	tc, err := discoverToolchain(cmd)
	if err != nil {
		return nil, nil, nil, false
	}
	bc.Toolchain = tc

	stages := []build.Stage{
		build.PackageDependenciesStage{},
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
		return nil, nil, nil, false
	}

	return bc, cfg, det, true
}

func shouldDelegateBuild(det *project.DetectResult) bool {
	return det != nil && (det.Mode == project.ModeWorkspace || det.Mode == project.ModeXcodeproj)
}

func buildBackendLabel(delegated bool) string {
	if delegated {
		return "xcodebuild"
	}
	return "native"
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
	if flags.BuildConfig != "" {
		return flags.BuildConfig
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
