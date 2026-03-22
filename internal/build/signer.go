package build

import (
	"fmt"

	"github.com/kacyfortner/ios-build-cli/internal/toolchain"
)

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

	args := []string{"--force", "--sign", identity}

	// attach entitlements if configured
	if bc.Target.Signing.Entitlements != "" {
		args = append(args, "--entitlements", bc.Target.Signing.Entitlements)
	}

	args = append(args, bc.AppBundlePath)

	result, err := toolchain.RunCommand(bc.Ctx, "codesign", args...)
	if err != nil {
		stderr := ""
		if result != nil {
			stderr = result.Stderr
		}
		detail := fmt.Errorf("codesign failed: %w", err)
		if stderr != "" {
			detail = fmt.Errorf("codesign failed: %s", stderr)
		}
		return &BuildError{
			Stage: "sign",
			Err:   detail,
			Hint:  "is xcode command line tools installed? run `xcode-select --install`",
		}
	}

	bc.Out.Info("sign", "identity", identity)
	return nil
}
