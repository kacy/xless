package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kacyfortner/ios-build-cli/internal/toolchain"
	"howett.net/plist"
)

// BundleStage creates the .app bundle with executable, Info.plist, and resources.
type BundleStage struct{}

func (BundleStage) Name() string { return "bundle" }

func (BundleStage) Run(bc *BuildContext) error {
	appDir := filepath.Join(bc.BuildDir, bc.Target.Name+".app")
	bc.AppBundlePath = appDir

	// create .app directory
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return &BuildError{
			Stage: "bundle",
			Err:   fmt.Errorf("cannot create app bundle directory: %w", err),
			Hint:  "could not create .app bundle directory",
		}
	}

	// copy executable into bundle
	execDst := filepath.Join(appDir, bc.Target.Name)
	if err := copyFile(bc.ExecutablePath, execDst); err != nil {
		return &BuildError{
			Stage: "bundle",
			Err:   fmt.Errorf("cannot copy executable: %w", err),
			Hint:  "the compile stage may not have produced an executable",
		}
	}

	// make executable
	if err := os.Chmod(execDst, 0o755); err != nil {
		return &BuildError{
			Stage: "bundle",
			Err:   fmt.Errorf("cannot set executable permission: %w", err),
		}
	}

	// write Info.plist
	if err := writeInfoPlist(bc, appDir); err != nil {
		return err
	}

	// copy resources
	if err := copyResources(bc, appDir); err != nil {
		return err
	}

	bc.Out.Info("bundle", "path", appDir)
	return nil
}

// writeInfoPlist either copies an existing Info.plist or generates one from config.
func writeInfoPlist(bc *BuildContext, appDir string) error {
	plistPath := filepath.Join(appDir, "Info.plist")

	// if target specifies an existing plist, copy it
	if bc.Target.InfoPlist != "" {
		src := bc.Target.InfoPlist
		if !filepath.IsAbs(src) {
			src = filepath.Join(bc.ProjectDir, src)
		}
		if err := copyFile(src, plistPath); err != nil {
			return &BuildError{
				Stage: "bundle",
				Err:   fmt.Errorf("cannot copy Info.plist: %w", err),
				Hint:  "check info_plist path in your config if specified",
			}
		}
		return nil
	}

	// generate Info.plist from config values
	supportedPlatforms := []string{"iPhoneSimulator"}
	if bc.Platform == toolchain.PlatformDevice {
		supportedPlatforms = []string{"iPhoneOS"}
	}

	info := map[string]any{
		"CFBundleName":                bc.Target.Name,
		"CFBundleIdentifier":          bc.Target.BundleID,
		"CFBundleVersion":             bc.Target.BuildNum,
		"CFBundleShortVersionString":  bc.Target.Version,
		"CFBundleExecutable":          bc.Target.Name,
		"CFBundlePackageType":         "APPL",
		"MinimumOSVersion":            bc.Target.MinIOS,
		"UILaunchScreen":              map[string]any{},
		"UIDeviceFamily":              []int{1, 2},
		"CFBundleSupportedPlatforms":  supportedPlatforms,
	}

	f, err := os.Create(plistPath)
	if err != nil {
		return &BuildError{
			Stage: "bundle",
			Err:   fmt.Errorf("cannot create Info.plist: %w", err),
		}
	}
	defer f.Close()

	enc := plist.NewEncoder(f)
	enc.Indent("\t")
	if err := enc.Encode(info); err != nil {
		return &BuildError{
			Stage: "bundle",
			Err:   fmt.Errorf("cannot write Info.plist: %w", err),
		}
	}

	return nil
}

// copyResources copies resource files and directories into the app bundle.
func copyResources(bc *BuildContext, appDir string) error {
	for _, res := range bc.Target.Resources {
		src := res
		if !filepath.IsAbs(src) {
			src = filepath.Join(bc.ProjectDir, src)
		}

		info, err := os.Stat(src)
		if err != nil {
			return &BuildError{
				Stage: "bundle",
				Err:   fmt.Errorf("resource %q: %w", res, err),
				Hint:  "check that resource paths in your config exist",
			}
		}

		if info.IsDir() {
			if err := copyDir(src, filepath.Join(appDir, filepath.Base(src))); err != nil {
				return &BuildError{
					Stage: "bundle",
					Err:   fmt.Errorf("cannot copy resource directory %q: %w", res, err),
				}
			}
		} else {
			dst := filepath.Join(appDir, filepath.Base(src))
			if err := copyFile(src, dst); err != nil {
				return &BuildError{
					Stage: "bundle",
					Err:   fmt.Errorf("cannot copy resource %q: %w", res, err),
				}
			}
		}
	}
	return nil
}

// copyFile copies a single file, creating parent directories as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		return copyFile(path, target)
	})
}
