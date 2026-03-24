package build

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kacy/xless/internal/config"
	"github.com/kacy/xless/internal/toolchain"
)

func TestPackageStageCreatesIPA(t *testing.T) {
	tmp := t.TempDir()

	// create a fake .app bundle
	appDir := filepath.Join(tmp, "build", "TestApp.app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "TestApp"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "Info.plist"), []byte("<plist/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// create a subdirectory with a file
	subdir := filepath.Join(appDir, "Assets.car")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "data"), []byte("assets"), 0o644); err != nil {
		t.Fatal(err)
	}

	bc := &BuildContext{
		Ctx:           context.Background(),
		Target:        &config.TargetConfig{Name: "TestApp"},
		Platform:      toolchain.PlatformDevice,
		BuildDir:      filepath.Join(tmp, "build"),
		AppBundlePath: appDir,
		Out:           noopFormatter{},
	}

	stage := PackageStage{}
	if err := stage.Run(bc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify IPA was created
	if bc.IPAPath == "" {
		t.Fatal("IPAPath not set")
	}

	// open and inspect the IPA
	r, err := zip.OpenReader(bc.IPAPath)
	if err != nil {
		t.Fatalf("cannot open IPA: %v", err)
	}
	defer r.Close()

	files := map[string]bool{}
	for _, f := range r.File {
		files[f.Name] = true
	}

	// check expected structure
	expected := []string{
		"Payload/TestApp.app/",
		"Payload/TestApp.app/TestApp",
		"Payload/TestApp.app/Info.plist",
		"Payload/TestApp.app/Assets.car/",
		"Payload/TestApp.app/Assets.car/data",
	}

	for _, path := range expected {
		if !files[path] {
			t.Errorf("missing expected path in IPA: %s", path)
			t.Logf("IPA contents: %v", mapKeys(files))
		}
	}
}

func TestPackageStageMissingAppBundle(t *testing.T) {
	tmp := t.TempDir()

	bc := &BuildContext{
		Ctx:           context.Background(),
		Target:        &config.TargetConfig{Name: "TestApp"},
		Platform:      toolchain.PlatformDevice,
		BuildDir:      filepath.Join(tmp, "build"),
		AppBundlePath: filepath.Join(tmp, "build", "TestApp.app"),
		Out:           noopFormatter{},
	}

	stage := PackageStage{}
	err := stage.Run(bc)
	if err == nil {
		t.Fatal("expected error for missing app bundle")
	}
}

func TestCreateIPAPreservesPermissions(t *testing.T) {
	tmp := t.TempDir()

	appDir := filepath.Join(tmp, "TestApp.app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// create executable with 0755
	if err := os.WriteFile(filepath.Join(appDir, "TestApp"), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	ipaPath := filepath.Join(tmp, "TestApp.ipa")
	if err := createIPA(appDir, ipaPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r, err := zip.OpenReader(ipaPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "TestApp") {
			mode := f.Mode()
			if mode&0o111 == 0 {
				t.Errorf("executable should have execute permission, got %v", mode)
			}
		}
	}
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
