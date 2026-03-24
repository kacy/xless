package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/kacy/xless/internal/config"
	"github.com/kacy/xless/internal/output"
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
		dir, err := os.Getwd()
		if err != nil {
			out.Error("cannot determine working directory", "error", err.Error())
			return
		}

		flags := cliFlags()
		cfg, det, err := config.Load(dir, flags)
		if err != nil {
			out.Error(err.Error())
			return
		}

		out.Info("project detected", "mode", det.Mode.String())

		if det.XcodeprojDir != "" {
			out.Info("xcodeproj", "path", det.XcodeprojDir)
		}
		if det.ConfigFile != "" {
			out.Info("config file", "path", det.ConfigFile)
		}

		// show project info
		out.Data("project", output.OrderedMap{
			{Key: "name", Value: cfg.Project.Name},
			{Key: "mode", Value: det.Mode.String()},
			{Key: "targets", Value: fmt.Sprintf("%d", len(cfg.Targets))},
		})

		// show each target
		for _, t := range cfg.Targets {
			targetMap := output.OrderedMap{
				{Key: "name", Value: t.Name},
				{Key: "bundle_id", Value: t.BundleID},
				{Key: "product_type", Value: string(t.ProductType)},
				{Key: "min_ios", Value: t.MinIOS},
				{Key: "version", Value: t.Version},
				{Key: "build_number", Value: t.BuildNum},
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
