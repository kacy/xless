package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cleanCmd)
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "remove build artifacts",
	Long:  "removes the .build/ directory and all build artifacts.",
	Run: func(cmd *cobra.Command, args []string) {
		dir, err := os.Getwd()
		if err != nil {
			out.Error("cannot determine working directory", "error", err.Error())
			return
		}

		buildDir := filepath.Join(dir, ".build")

		info, err := os.Stat(buildDir)
		if err != nil {
			if os.IsNotExist(err) {
				out.Info("nothing to clean")
				return
			}
			out.Error("cannot inspect build directory", "path", buildDir, "error", err.Error())
			return
		}
		if !info.IsDir() {
			out.Error("build path is not a directory", "path", buildDir)
			return
		}

		if err := os.RemoveAll(buildDir); err != nil {
			out.Error("failed to remove build directory",
				"path", buildDir,
				"error", err.Error(),
				"hint", "try removing manually: rm -rf "+buildDir,
			)
			return
		}

		out.Success("cleaned", "path", buildDir)
	},
}
