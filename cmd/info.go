package cmd

import (
	"fmt"
	"strings"

	"github.com/kacy/xless/internal/build"
	"github.com/kacy/xless/internal/output"
	"github.com/kacy/xless/internal/project"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(infoCmd)
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "display resolved project configuration",
	Long:  "reads the project configuration (from .xcodeproj and/or xless.yml) and displays the resolved result. useful for debugging and verifying settings.",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cliFlags()
		_, cfg, det, err := loadProject(flags)
		if err != nil {
			out.Error(err.Error())
			return
		}

		out.Info("project detected", "mode", det.Mode.String())

		if det.XcodeprojDir != "" {
			out.Info("xcodeproj", "path", det.XcodeprojDir)
		}
		if det.WorkspaceDir != "" {
			out.Info("xcworkspace", "path", det.WorkspaceDir)
		}
		if det.ConfigFile != "" {
			out.Info("config file", "path", det.ConfigFile)
		}

		// show project info
		out.Data("project", output.OrderedMap{
			{Key: "name", Value: cfg.Project.Name},
			{Key: "mode", Value: det.Mode.String()},
			{Key: "targets", Value: fmt.Sprintf("%d", len(cfg.Targets))},
			{Key: "resolved_packages", Value: fmt.Sprintf("%d", len(cfg.ResolvedPackages))},
		})
		if cfg.PackageResolvedFile != "" {
			out.Data("package_resolution", output.OrderedMap{
				{Key: "file", Value: cfg.PackageResolvedFile},
				{Key: "packages", Value: fmt.Sprintf("%d", len(cfg.ResolvedPackages))},
			})
		}
		for _, pkg := range cfg.ResolvedPackages {
			packageMap := output.OrderedMap{
				{Key: "identity", Value: pkg.Identity},
				{Key: "location", Value: pkg.Location},
			}
			if pkg.Version != "" {
				packageMap = append(packageMap, output.KV{Key: "version", Value: pkg.Version})
			}
			if pkg.Branch != "" {
				packageMap = append(packageMap, output.KV{Key: "branch", Value: pkg.Branch})
			}
			if pkg.Revision != "" {
				packageMap = append(packageMap, output.KV{Key: "revision", Value: pkg.Revision})
			}
			out.Data("package:"+pkg.Identity, packageMap)
		}

		targets, err := selectedTargets(cfg, flags)
		if err != nil {
			printTargetSelectionError(err)
			return
		}

		for _, t := range targets {
			backend := "native"
			if det.Mode != project.ModeNative {
				backend = "xcodebuild"
			}
			targetMap := output.OrderedMap{
				{Key: "name", Value: t.Name},
				{Key: "bundle_id", Value: t.BundleID},
				{Key: "product_type", Value: string(t.ProductType)},
				{Key: "min_ios", Value: t.MinIOS},
				{Key: "version", Value: t.Version},
				{Key: "build_number", Value: t.BuildNum},
				{Key: "build_backend", Value: backend},
			}

			if len(t.Sources) > 0 {
				targetMap = append(targetMap, output.KV{Key: "sources", Value: strings.Join(t.Sources, ", ")})
			}
			if len(t.Resources) > 0 {
				targetMap = append(targetMap, output.KV{Key: "resources", Value: strings.Join(t.Resources, ", ")})
			}
			if len(t.SwiftFlags) > 0 {
				targetMap = append(targetMap, output.KV{Key: "swift_flags", Value: strings.Join(t.SwiftFlags, " ")})
			}
			if t.InfoPlist != "" {
				targetMap = append(targetMap, output.KV{Key: "info_plist", Value: t.InfoPlist})
			}
			if t.Signing.Identity != "" {
				targetMap = append(targetMap, output.KV{Key: "signing_identity", Value: t.Signing.Identity})
			}
			if t.Signing.TeamID != "" {
				targetMap = append(targetMap, output.KV{Key: "team_id", Value: t.Signing.TeamID})
			}
			if t.Signing.Entitlements != "" {
				targetMap = append(targetMap, output.KV{Key: "entitlements", Value: t.Signing.Entitlements})
			}
			if t.Signing.ProvisioningProfile != "" {
				targetMap = append(targetMap, output.KV{Key: "provisioning_profile", Value: t.Signing.ProvisioningProfile})
			}
			if len(t.Dependencies) > 0 {
				targetMap = append(targetMap, output.KV{Key: "dependencies", Value: strings.Join(t.Dependencies, ", ")})
			}
			if len(t.Packages) > 0 {
				targetMap = append(targetMap, output.KV{Key: "packages", Value: strings.Join(t.Packages, ", ")})
			}
			if len(t.PackageRefs) > 0 {
				targetMap = append(targetMap, output.KV{Key: "package_refs", Value: strings.Join(t.PackageRefs, ", ")})
			}
			if len(t.Frameworks) > 0 {
				targetMap = append(targetMap, output.KV{Key: "frameworks", Value: strings.Join(t.Frameworks, ", ")})
			}
			if len(t.Libraries) > 0 {
				targetMap = append(targetMap, output.KV{Key: "libraries", Value: strings.Join(t.Libraries, ", ")})
			}
			if len(t.LinkerFlags) > 0 {
				targetMap = append(targetMap, output.KV{Key: "linker_flags", Value: strings.Join(t.LinkerFlags, " ")})
			}
			if len(t.FrameworkSearchPaths) > 0 {
				targetMap = append(targetMap, output.KV{Key: "framework_search_paths", Value: strings.Join(t.FrameworkSearchPaths, ", ")})
			}
			if len(t.LibrarySearchPaths) > 0 {
				targetMap = append(targetMap, output.KV{Key: "library_search_paths", Value: strings.Join(t.LibrarySearchPaths, ", ")})
			}
			if len(t.ShellScriptPhases) > 0 {
				targetMap = append(targetMap, output.KV{Key: "shell_script_phases", Value: strings.Join(t.ShellScriptPhases, ", ")})
			}
			if len(t.CopyFilesPhases) > 0 {
				targetMap = append(targetMap, output.KV{Key: "copy_files_phases", Value: strings.Join(t.CopyFilesPhases, ", ")})
			}
			if t.SourceRoot != "" {
				targetMap = append(targetMap, output.KV{Key: "source_root", Value: t.SourceRoot})
			}
			if backend == "xcodebuild" {
				selection, err := build.ResolveXcodebuildSelection(cmd.Context(), det.WorkspaceDir, det.XcodeprojDir, t.Name, flags.Scheme)
				if err != nil {
					targetMap = append(targetMap, output.KV{Key: "xcode_selector_error", Value: err.Error()})
					if hint := build.XcodebuildSelectionHint(err); hint != "" {
						targetMap = append(targetMap, output.KV{Key: "xcode_selector_hint", Value: hint})
					}
				} else {
					if selection.Scheme != "" {
						targetMap = append(targetMap, output.KV{Key: "xcode_scheme", Value: selection.Scheme})
					}
					targetMap = append(targetMap, output.KV{Key: "xcode_selector", Value: selection.Selector()})
				}
			}
			if len(t.Unsupported) > 0 {
				key := "unsupported"
				if backend == "xcodebuild" {
					key = "parsed_notes"
				}
				targetMap = append(targetMap, output.KV{Key: key, Value: strings.Join(t.Unsupported, "; ")})
			}

			out.Data(fmt.Sprintf("target:%s", t.Name), targetMap)
		}

		// show defaults
		defaults := output.OrderedMap{}
		if cfg.Defaults.Target != "" {
			defaults = append(defaults, output.KV{Key: "target", Value: cfg.Defaults.Target})
		}
		defaults = append(defaults, output.KV{Key: "config", Value: cfg.Defaults.Config})
		defaults = append(defaults, output.KV{Key: "simulator", Value: cfg.Defaults.Simulator})
		if cfg.Defaults.Device != "" {
			defaults = append(defaults, output.KV{Key: "device", Value: cfg.Defaults.Device})
		}

		out.Data("defaults", defaults)
	},
}
