package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kacyfortner/ios-build-cli/internal/config"
	"github.com/kacyfortner/ios-build-cli/internal/toolchain"
	"howett.net/plist"
)

func TestWriteInfoPlistGenerated(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "MyApp.app")
	os.MkdirAll(appDir, 0o755)

	bc := &BuildContext{
		Ctx:        context.Background(),
		Platform:   toolchain.PlatformSimulator,
		ProjectDir: dir,
		Target: &config.TargetConfig{
			Name:     "MyApp",
			BundleID: "com.example.MyApp",
			Version:  "1.0.0",
			BuildNum: "1",
			MinIOS:   "16.0",
		},
		Out: noopFormatter{},
	}

	if err := writeInfoPlist(bc, appDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// read back and verify
	data, err := os.ReadFile(filepath.Join(appDir, "Info.plist"))
	if err != nil {
		t.Fatalf("cannot read Info.plist: %v", err)
	}

	var info map[string]any
	if _, err := plist.Unmarshal(data, &info); err != nil {
		t.Fatalf("cannot parse Info.plist: %v", err)
	}

	checks := map[string]string{
		"CFBundleName":               "MyApp",
		"CFBundleIdentifier":         "com.example.MyApp",
		"CFBundleVersion":            "1",
		"CFBundleShortVersionString": "1.0.0",
		"CFBundleExecutable":         "MyApp",
		"CFBundlePackageType":        "APPL",
		"MinimumOSVersion":           "16.0",
	}
	for key, want := range checks {
		got, ok := info[key].(string)
		if !ok {
			t.Errorf("Info.plist missing key %q", key)
			continue
		}
		if got != want {
			t.Errorf("Info.plist[%q] = %q, want %q", key, got, want)
		}
	}

	// check simulator platform
	platforms, ok := info["CFBundleSupportedPlatforms"].([]any)
	if !ok || len(platforms) == 0 {
		t.Fatal("Info.plist missing CFBundleSupportedPlatforms")
	}
	if platforms[0] != "iPhoneSimulator" {
		t.Errorf("expected iPhoneSimulator, got %v", platforms[0])
	}

	// UIDeviceFamily should be [1, 2]
	family, ok := info["UIDeviceFamily"].([]any)
	if !ok || len(family) != 2 {
		t.Fatalf("expected UIDeviceFamily [1,2], got %v", info["UIDeviceFamily"])
	}
}

func TestWriteInfoPlistDevice(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "MyApp.app")
	os.MkdirAll(appDir, 0o755)

	bc := &BuildContext{
		Ctx:        context.Background(),
		Platform:   toolchain.PlatformDevice,
		ProjectDir: dir,
		Target: &config.TargetConfig{
			Name:     "MyApp",
			BundleID: "com.example.MyApp",
			Version:  "1.0.0",
			BuildNum: "1",
			MinIOS:   "16.0",
		},
		Out: noopFormatter{},
	}

	if err := writeInfoPlist(bc, appDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(appDir, "Info.plist"))
	if err != nil {
		t.Fatalf("cannot read Info.plist: %v", err)
	}

	var info map[string]any
	if _, err := plist.Unmarshal(data, &info); err != nil {
		t.Fatalf("cannot parse Info.plist: %v", err)
	}

	platforms, ok := info["CFBundleSupportedPlatforms"].([]any)
	if !ok || len(platforms) == 0 {
		t.Fatal("Info.plist missing CFBundleSupportedPlatforms")
	}
	if platforms[0] != "iPhoneOS" {
		t.Errorf("expected iPhoneOS, got %v", platforms[0])
	}
}

func TestWriteInfoPlistCopiesExisting(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "MyApp.app")
	os.MkdirAll(appDir, 0o755)

	// create an existing plist file
	plistContent := []byte("<?xml version=\"1.0\"?>\n<plist><dict><key>Custom</key><string>yes</string></dict></plist>")
	writeFile(t, filepath.Join(dir, "Info.plist"), string(plistContent))

	bc := &BuildContext{
		Ctx:        context.Background(),
		Platform:   toolchain.PlatformSimulator,
		ProjectDir: dir,
		Target: &config.TargetConfig{
			Name:      "MyApp",
			InfoPlist: "Info.plist",
		},
		Out: noopFormatter{},
	}

	if err := writeInfoPlist(bc, appDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify the file was copied, not generated
	data, err := os.ReadFile(filepath.Join(appDir, "Info.plist"))
	if err != nil {
		t.Fatalf("cannot read copied Info.plist: %v", err)
	}
	if string(data) != string(plistContent) {
		t.Error("Info.plist should be copied verbatim when info_plist is set")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()

	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	// destination has nested parent dirs that don't exist yet
	dst := filepath.Join(dir, "a", "b", "dst.txt")
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("cannot read dest: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", string(data), "hello world")
	}
}

func TestCopyResources(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "MyApp.app")
	os.MkdirAll(appDir, 0o755)

	// create a resource file
	writeFile(t, filepath.Join(dir, "icon.png"), "fake-png-data")

	// create a resource directory
	writeFile(t, filepath.Join(dir, "Assets", "logo.png"), "fake-logo")
	writeFile(t, filepath.Join(dir, "Assets", "bg.png"), "fake-bg")

	bc := &BuildContext{
		Ctx:        context.Background(),
		ProjectDir: dir,
		Target: &config.TargetConfig{
			Resources: []string{"icon.png", "Assets"},
		},
		Out: noopFormatter{},
	}

	if err := copyResources(bc, appDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// check file resource
	data, err := os.ReadFile(filepath.Join(appDir, "icon.png"))
	if err != nil {
		t.Fatalf("icon.png not copied: %v", err)
	}
	if string(data) != "fake-png-data" {
		t.Error("icon.png content mismatch")
	}

	// check directory resource
	data, err = os.ReadFile(filepath.Join(appDir, "Assets", "logo.png"))
	if err != nil {
		t.Fatalf("Assets/logo.png not copied: %v", err)
	}
	if string(data) != "fake-logo" {
		t.Error("logo.png content mismatch")
	}

	data, err = os.ReadFile(filepath.Join(appDir, "Assets", "bg.png"))
	if err != nil {
		t.Fatalf("Assets/bg.png not copied: %v", err)
	}
	if string(data) != "fake-bg" {
		t.Error("bg.png content mismatch")
	}
}

func TestCopyResourcesMissing(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "MyApp.app")
	os.MkdirAll(appDir, 0o755)

	bc := &BuildContext{
		Ctx:        context.Background(),
		ProjectDir: dir,
		Target: &config.TargetConfig{
			Resources: []string{"nonexistent.png"},
		},
		Out: noopFormatter{},
	}

	err := copyResources(bc, appDir)
	if err == nil {
		t.Fatal("expected error for missing resource")
	}
}
