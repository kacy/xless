package build

import (
	"fmt"
	"path/filepath"

	"github.com/kacy/xless/internal/toolchain"
)

var runSignCommand = toolchain.RunCommand

// SignStage runs codesign on the .app bundle.
type SignStage struct{}

func (SignStage) Name() string { return "sign" }

func (SignStage) Run(bc *BuildContext) error {
	identity := bc.Target.Signing.Identity

	// for simulator builds, use ad-hoc signing when no identity is set
	if identity == "" {
		if bc.Platform == toolchain.PlatformDevice {
			return &BuildError{
				Stage: "sign",
				Err:   fmt.Errorf("no signing identity configured for device build"),
				Hint:  "set signing.identity in xless.yml or run `security find-identity -v -p codesigning`",
			}
		}
		identity = "-"
	}

	// for device builds, embed the provisioning profile before signing
	if bc.Platform == toolchain.PlatformDevice {
		if err := embedProvisioningProfile(bc); err != nil {
			return err
		}
	}

	for _, bundlePath := range nestedPackageBundlePaths(bc) {
		if err := codesignPath(bc, identity, "", bundlePath); err != nil {
			return err
		}
	}

	entitlements := ""
	if bc.Target.Signing.Entitlements != "" {
		entitlements = resolveProjectPath(bc.ProjectDir, bc.Target.Signing.Entitlements)
	}
	if err := codesignPath(bc, identity, entitlements, bc.AppBundlePath); err != nil {
		return err
	}

	bc.Out.Info("sign", "identity", identity)
	return nil
}

func nestedPackageBundlePaths(bc *BuildContext) []string {
	var paths []string
	for _, bundlePath := range uniqueStrings(bc.PackageResourceBundles) {
		paths = append(paths, filepath.Join(bc.AppBundlePath, filepath.Base(bundlePath)))
	}
	return paths
}

func codesignPath(bc *BuildContext, identity, entitlements, path string) error {
	args := []string{"--force", "--sign", identity}
	if entitlements != "" {
		args = append(args, "--entitlements", entitlements)
	}
	args = append(args, path)

	result, err := runSignCommand(bc.Ctx, "codesign", args...)
	if err != nil {
		stderr := ""
		if result != nil {
			stderr = result.Stderr
		}
		detail := fmt.Errorf("codesign failed: %w", err)
		if stderr != "" {
			detail = fmt.Errorf("codesign failed: %s: %w", stderr, err)
		}
		return &BuildError{
			Stage: "sign",
			Err:   detail,
			Hint:  "is xcode command line tools installed? run `xcode-select --install`",
		}
	}
	return nil
}

// embedProvisioningProfile copies the provisioning profile into the app bundle
// as embedded.mobileprovision. required for device builds.
func embedProvisioningProfile(bc *BuildContext) error {
	profile := bc.Target.Signing.ProvisioningProfile
	if profile == "" {
		return &BuildError{
			Stage: "sign",
			Err:   fmt.Errorf("no provisioning profile configured for device build"),
			Hint:  "set signing.provisioning_profile in xless.yml pointing to your .mobileprovision file",
		}
	}

	profile = resolveProjectPath(bc.ProjectDir, profile)

	dst := filepath.Join(bc.AppBundlePath, "embedded.mobileprovision")
	if err := copyFile(profile, dst); err != nil {
		return &BuildError{
			Stage: "sign",
			Err:   fmt.Errorf("cannot copy provisioning profile: %w", err),
			Hint:  "download your provisioning profile from developer.apple.com or xcode",
		}
	}

	return nil
}

// resolveProjectPath resolves a possibly-relative path against the project directory.
func resolveProjectPath(projectDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectDir, path)
}
