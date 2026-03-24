package cmd

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kacyfortner/ios-build-cli/internal/output"
	"github.com/kacyfortner/ios-build-cli/internal/project"
	"github.com/spf13/cobra"
)

func init() {
	initCmd.Flags().String("template", "simple", "project template: simple or spm")
	initCmd.Flags().String("bundle-id", "", "bundle identifier (default: com.example.<name>)")
	initCmd.Flags().String("min-ios", "16.0", "minimum ios deployment target")
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "scaffold a new xless project",
	Long:  "creates a new ios project with swift source files, xless.yml config, and optionally initializes a git repository.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tmpl, _ := cmd.Flags().GetString("template")
		bundleID, _ := cmd.Flags().GetString("bundle-id")
		minIOS, _ := cmd.Flags().GetString("min-ios")

		dir, err := os.Getwd()
		if err != nil {
			out.Error("cannot determine working directory", "error", err.Error())
			return
		}

		// determine project name and directory
		var name string
		if len(args) > 0 {
			name = args[0]
			dir = filepath.Join(dir, name)

			// create directory if it doesn't exist
			if err := os.MkdirAll(dir, 0o755); err != nil {
				out.Error("cannot create project directory", "error", err.Error())
				return
			}
		} else {
			name = filepath.Base(dir)
		}

		if err := project.ValidateName(name); err != nil {
			out.Error(err.Error())
			return
		}

		var templateType project.TemplateType
		switch tmpl {
		case "spm":
			templateType = project.TemplateSPM
		default:
			templateType = project.TemplateSimple
		}

		cfg := project.ScaffoldConfig{
			Name:     name,
			BundleID: bundleID,
			MinIOS:   minIOS,
			Template: templateType,
		}

		result, err := project.Scaffold(dir, cfg)
		if err != nil {
			out.Error("scaffolding failed", "error", err.Error())
			return
		}

		// initialize git if possible and not already initialized
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			if _, err := exec.LookPath("git"); err == nil {
				gitCmd := exec.Command("git", "init", dir)
				gitCmd.Stdout = nil
				gitCmd.Stderr = nil
				_ = gitCmd.Run() // silently skip if git init fails
			}
		}

		out.Success("project created", "name", name, "template", tmpl)
		for _, f := range result.Files {
			out.Info("created", "file", f)
		}

		// show next steps
		if len(args) > 0 {
			out.Data("next", output.OrderedMap{
				{Key: "1", Value: "cd " + name},
				{Key: "2", Value: "xless build"},
				{Key: "3", Value: "xless run"},
			})
		} else {
			out.Data("next", output.OrderedMap{
				{Key: "1", Value: "xless build"},
				{Key: "2", Value: "xless run"},
			})
		}
	},
}
