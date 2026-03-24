package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kacy/xless/internal/output"
	"github.com/kacy/xless/internal/project"
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

		nameArg := ""
		if len(args) > 0 {
			nameArg = args[0]
		}

		name, dir, err := resolveInitProject(dir, nameArg)
		if err != nil {
			out.Error(err.Error())
			return
		}

		if nameArg != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				out.Error("cannot create project directory", "error", err.Error())
				return
			}
		}

		templateType, err := parseTemplateType(tmpl)
		if err != nil {
			out.Error(err.Error(), "hint", "use --template simple or --template spm")
			return
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
			if err := initGitRepo(dir); err != nil {
				out.Warn("git init failed", "error", err.Error())
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

func parseTemplateType(name string) (project.TemplateType, error) {
	switch name {
	case "", string(project.TemplateSimple):
		return project.TemplateSimple, nil
	case string(project.TemplateSPM):
		return project.TemplateSPM, nil
	default:
		return "", fmt.Errorf("unknown template %q", name)
	}
}

func resolveInitProject(baseDir, nameArg string) (string, string, error) {
	if nameArg == "" {
		name := filepath.Base(baseDir)
		if err := project.ValidateName(name); err != nil {
			return "", "", err
		}
		return name, baseDir, nil
	}

	if err := project.ValidateName(nameArg); err != nil {
		return "", "", err
	}

	return nameArg, filepath.Join(baseDir, nameArg), nil
}

func initGitRepo(dir string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}

	gitCmd := exec.Command("git", "init", dir)
	gitCmd.Stdout = nil
	gitCmd.Stderr = nil
	return gitCmd.Run()
}
