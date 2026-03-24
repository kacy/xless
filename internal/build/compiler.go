package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kacy/xless/internal/toolchain"
)

const compileTimeout = 5 * time.Minute

// CompileStage compiles swift source files via swiftc.
type CompileStage struct{}

func (CompileStage) Name() string { return "compile" }

func (CompileStage) Run(bc *BuildContext) error {
	// set a compilation timeout if the context has no deadline
	ctx := bc.Ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, compileTimeout)
		defer cancel()
	}

	// resolve source files
	sources, err := resolveSwiftFiles(bc.ProjectDir, bc.Target.Sources)
	if err != nil {
		return &BuildError{
			Stage: "compile",
			Err:   err,
			Hint:  "check that your source directories exist and contain .swift files",
		}
	}

	// create build directory
	if err := os.MkdirAll(bc.BuildDir, 0o755); err != nil {
		return &BuildError{
			Stage: "compile",
			Err:   fmt.Errorf("cannot create build directory: %w", err),
			Hint:  "could not create build directory",
		}
	}

	// build the executable path
	bc.ExecutablePath = filepath.Join(bc.BuildDir, bc.Target.Name)

	// construct swiftc args and run
	args := buildSwiftcArgs(bc, sources)

	bc.Out.Info("compile", "files", fmt.Sprintf("%d", len(sources)))

	result, err := toolchain.RunCommand(ctx, bc.Toolchain.SwiftcPath(), args...)
	if err != nil {
		stderr := ""
		if result != nil {
			stderr = strings.TrimSpace(result.Stderr)
		}
		detail := fmt.Errorf("swiftc failed: %w", err)
		if stderr != "" {
			detail = fmt.Errorf("swiftc failed:\n%s: %w", stderr, err)
		}
		return &BuildError{
			Stage: "compile",
			Err:   detail,
			Hint:  "check your swift source files for compile errors",
		}
	}

	return nil
}

// resolveSwiftFiles expands source entries into individual .swift file paths.
// Entries ending in "/" or that are directories are walked recursively.
// Other entries are treated as individual .swift files.
func resolveSwiftFiles(projectDir string, sources []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string

	for _, src := range sources {
		abs := resolveProjectPath(projectDir, src)

		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("source path %q: %w", src, err)
		}

		if info.IsDir() {
			err := filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(d.Name(), ".swift") {
					if !seen[path] {
						seen[path] = true
						files = append(files, path)
					}
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walking source directory %q: %w", src, err)
			}
		} else {
			if filepath.Ext(abs) != ".swift" {
				return nil, fmt.Errorf("source file %q must be a .swift file", src)
			}
			if !seen[abs] {
				seen[abs] = true
				files = append(files, abs)
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .swift files found in sources")
	}

	return files, nil
}

// buildTriple returns the target triple for the given arch, minIOS, and platform.
// e.g. "arm64-apple-ios16.0-simulator" or "arm64-apple-ios16.0"
func buildTriple(arch string, minIOS string, platform toolchain.Platform) string {
	triple := fmt.Sprintf("%s-apple-ios%s", arch, minIOS)
	if platform == toolchain.PlatformSimulator {
		triple += "-simulator"
	}
	return triple
}

// buildSwiftcArgs constructs the full argument list for swiftc.
func buildSwiftcArgs(bc *BuildContext, sources []string) []string {
	args := []string{
		"-sdk", bc.Toolchain.SDKPath(bc.Platform),
		"-target", buildTriple(bc.Toolchain.Arch(), bc.Target.MinIOS, bc.Platform),
	}

	// optimization flags
	if bc.BuildConfig == "release" {
		args = append(args, "-O")
	} else {
		args = append(args, "-Onone", "-g", "-DDEBUG")
	}

	// source files
	args = append(args, sources...)

	// user-specified flags
	args = append(args, bc.Target.SwiftFlags...)

	// output
	args = append(args, "-emit-executable", "-o", bc.ExecutablePath)

	return args
}
