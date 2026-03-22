package cmd

import (
	"os"

	"github.com/fatih/color"
	"github.com/kacyfortner/ios-build-cli/internal/config"
	"github.com/kacyfortner/ios-build-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// out is the active output formatter, set in PersistentPreRun.
	out output.Formatter

	rootCmd = &cobra.Command{
		Use:   "xless",
		Short: "build ios apps without the xcode ide",
		Long:  "xless compiles swift, creates .app bundles, and deploys to simulators or devices — all from the terminal.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if viper.GetBool("no-color") {
				color.NoColor = true
			}

			if viper.GetBool("json") {
				out = output.NewJSONFormatter()
			} else {
				out = output.NewHumanFormatter()
			}
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
)

// Execute runs the root command. version is injected from main.
func Execute(version string) {
	cliVersion = version
	if err := rootCmd.Execute(); err != nil {
		if out != nil {
			out.Error(err.Error())
		} else {
			color.New(color.FgRed, color.Bold).Fprintf(os.Stderr, "  error  %s\n", err)
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("json", false, "output as newline-delimited json")
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().String("config", "", "path to config file (default: ./xless.yml)")
	rootCmd.PersistentFlags().Bool("no-color", false, "disable colored output")
	rootCmd.PersistentFlags().String("platform", "", "build platform: simulator or device")
	rootCmd.PersistentFlags().String("target", "", "build target name")
	rootCmd.PersistentFlags().String("build-config", "", "build configuration: debug or release")

	_ = viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("no-color", rootCmd.PersistentFlags().Lookup("no-color"))
	_ = viper.BindPFlag("platform", rootCmd.PersistentFlags().Lookup("platform"))
	_ = viper.BindPFlag("target", rootCmd.PersistentFlags().Lookup("target"))
	_ = viper.BindPFlag("build-config", rootCmd.PersistentFlags().Lookup("build-config"))

	viper.SetEnvPrefix("XLESS")
	viper.AutomaticEnv()
}

// cliFlags builds a CLIFlags from the current viper values.
func cliFlags() config.CLIFlags {
	return config.CLIFlags{
		Platform: viper.GetString("platform"),
		Target:   viper.GetString("target"),
		Config:   viper.GetString("build-config"),
	}
}
