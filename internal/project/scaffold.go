package project

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

//go:embed templates/simple/*.tmpl templates/spm/*.tmpl
var templates embed.FS

// TemplateType identifies the project template to scaffold.
type TemplateType string

const (
	TemplateSimple TemplateType = "simple"
	TemplateSPM    TemplateType = "spm"
)

// ScaffoldConfig holds the parameters for project scaffolding.
type ScaffoldConfig struct {
	Name     string
	BundleID string
	MinIOS   string
	Template TemplateType
}

// ScaffoldResult describes the files created by Scaffold.
type ScaffoldResult struct {
	Dir   string
	Files []string
}

// templateData is the data passed to templates.
type templateData struct {
	Name        string
	BundleID    string
	MinIOS      string
	MinIOSMajor string
}

var namePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

// ValidateName checks that a project name is valid.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid project name %q: must start with a letter, contain only letters/digits/hyphens/underscores, max 64 characters", name)
	}
	return nil
}

// DeriveBundleID creates a bundle ID from a project name.
func DeriveBundleID(name string) string {
	clean := strings.ToLower(name)
	clean = strings.ReplaceAll(clean, "_", "-")
	return "com.example." + clean
}

// Scaffold creates a new project from a template.
func Scaffold(dir string, cfg ScaffoldConfig) (*ScaffoldResult, error) {
	if err := ValidateName(cfg.Name); err != nil {
		return nil, err
	}

	// check for existing project
	for _, f := range []string{"xless.yml", "xless.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return nil, fmt.Errorf("project already exists: %s found in %s", f, dir)
		}
	}

	if cfg.BundleID == "" {
		cfg.BundleID = DeriveBundleID(cfg.Name)
	}
	if cfg.MinIOS == "" {
		cfg.MinIOS = "16.0"
	}
	if cfg.Template == "" {
		cfg.Template = TemplateSimple
	}

	data := templateData{
		Name:        cfg.Name,
		BundleID:    cfg.BundleID,
		MinIOS:      cfg.MinIOS,
		MinIOSMajor: majorVersion(cfg.MinIOS),
	}

	// render templates
	tmplDir := "templates/" + string(cfg.Template)
	entries, err := templates.ReadDir(tmplDir)
	if err != nil {
		return nil, fmt.Errorf("template %q not found: %w", cfg.Template, err)
	}

	srcDir := filepath.Join(dir, "Sources", cfg.Name)
	outputs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		outPath := outputPath(dir, srcDir, entry.Name(), cfg.Name)
		if _, err := os.Stat(outPath); err == nil {
			return nil, fmt.Errorf("refusing to overwrite existing file: %s", outPath)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("checking %s: %w", outPath, err)
		}
		outputs = append(outputs, outPath)
	}

	for _, outPath := range outputs {
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return nil, fmt.Errorf("creating directory for %s: %w", outPath, err)
		}
	}

	result := &ScaffoldResult{Dir: dir}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		tmplContent, err := templates.ReadFile(filepath.Join(tmplDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading template %s: %w", entry.Name(), err)
		}

		tmpl, err := template.New(entry.Name()).Parse(string(tmplContent))
		if err != nil {
			return nil, fmt.Errorf("parsing template %s: %w", entry.Name(), err)
		}

		// determine output path
		outPath := outputPath(dir, srcDir, entry.Name(), cfg.Name)

		f, err := os.Create(outPath)
		if err != nil {
			return nil, fmt.Errorf("creating %s: %w", outPath, err)
		}

		err = tmpl.Execute(f, data)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("rendering %s: %w", entry.Name(), err)
		}

		// track relative path
		rel, _ := filepath.Rel(dir, outPath)
		result.Files = append(result.Files, rel)
	}

	return result, nil
}

// outputPath maps a template filename to its output location.
func outputPath(projectDir, srcDir, tmplName, appName string) string {
	// strip .tmpl suffix
	name := strings.TrimSuffix(tmplName, ".tmpl")

	switch name {
	case "App.swift":
		return filepath.Join(srcDir, appName+"App.swift")
	case "xless.yml":
		return filepath.Join(projectDir, "xless.yml")
	case "gitignore":
		return filepath.Join(projectDir, ".gitignore")
	case "Package.swift":
		return filepath.Join(projectDir, "Package.swift")
	default:
		return filepath.Join(projectDir, name)
	}
}

// majorVersion extracts the major version from a semver-ish string.
// "16.0" → "16", "17.2" → "17"
func majorVersion(v string) string {
	if idx := strings.Index(v, "."); idx > 0 {
		return v[:idx]
	}
	return v
}
