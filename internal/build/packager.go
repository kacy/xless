package build

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PackageStage creates an IPA archive from the .app bundle.
// the IPA is a zip with Payload/AppName.app/ structure.
type PackageStage struct{}

func (PackageStage) Name() string { return "package" }

func (PackageStage) Run(bc *BuildContext) error {
	ipaPath := filepath.Join(bc.BuildDir, bc.Target.Name+".ipa")

	if err := createIPA(bc.AppBundlePath, ipaPath); err != nil {
		return &BuildError{
			Stage: "package",
			Err:   fmt.Errorf("cannot create IPA: %w", err),
			Hint:  "ensure the .app bundle exists and is readable",
		}
	}

	bc.IPAPath = ipaPath
	bc.Out.Info("package", "ipa", ipaPath)
	return nil
}

// incompressibleExts are file extensions that don't benefit from deflate compression.
var incompressibleExts = map[string]bool{
	".car": true, ".png": true, ".jpg": true, ".jpeg": true,
	".mobileprovision": true, ".dylib": true,
}

// createIPA zips the app bundle into an IPA with Payload/ prefix.
func createIPA(appDir, ipaPath string) error {
	if err := os.MkdirAll(filepath.Dir(ipaPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(ipaPath)
	if err != nil {
		return err
	}

	w := zip.NewWriter(f)

	walkErr := filepath.WalkDir(appDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(filepath.Dir(appDir), path)
		if err != nil {
			return err
		}
		zipPath := filepath.Join("Payload", rel)

		if d.IsDir() {
			_, err := w.Create(zipPath + "/")
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipPath

		// use Store for already-compressed or binary files
		ext := strings.ToLower(filepath.Ext(path))
		if incompressibleExts[ext] || ext == "" {
			header.Method = zip.Store
		} else {
			header.Method = zip.Deflate
		}

		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}

		_, copyErr := io.Copy(writer, src)
		closeErr := src.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})

	// close zip writer first — this writes the central directory
	if closeErr := w.Close(); closeErr != nil {
		_ = f.Close()
		return closeErr
	}
	if closeErr := f.Close(); closeErr != nil {
		return closeErr
	}

	return walkErr
}
