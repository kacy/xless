package toolchain

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

// ToolchainInfo holds discovered paths and versions for the Apple toolchain.
type ToolchainInfo struct {
	SwiftcPath       string
	SimulatorSDKPath string
	DeviceSDKPath    string
	SwiftVersion     string
	XcodeVersion     string
	Arch             string
}

// Discover probes the system for Xcode toolchain paths and versions.
// all probes run concurrently. returns partial results on failure —
// callers should check individual fields. always returns non-nil info.
func Discover(ctx context.Context) (*ToolchainInfo, error) {
	info := &ToolchainInfo{
		Arch: goarchToApple(runtime.GOARCH),
	}

	var mu sync.Mutex
	var errs []string

	var wg sync.WaitGroup

	probe := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	probe(func() {
		if result, err := RunCommand(ctx, "xcrun", "--find", "swiftc"); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Sprintf("swiftc not found: %v (is xcode installed? run `xcode-select -p` to check)", err))
			mu.Unlock()
		} else {
			info.SwiftcPath = strings.TrimSpace(result.Stdout)
		}
	})

	probe(func() {
		if result, err := RunCommand(ctx, "xcrun", "--show-sdk-path", "--sdk", "iphonesimulator"); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Sprintf("simulator sdk not found: %v (run `xcode-select --install`)", err))
			mu.Unlock()
		} else {
			info.SimulatorSDKPath = strings.TrimSpace(result.Stdout)
		}
	})

	probe(func() {
		if result, err := RunCommand(ctx, "xcrun", "--show-sdk-path", "--sdk", "iphoneos"); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Sprintf("device sdk not found: %v", err))
			mu.Unlock()
		} else {
			info.DeviceSDKPath = strings.TrimSpace(result.Stdout)
		}
	})

	probe(func() {
		if result, err := RunCommand(ctx, "swiftc", "--version"); err == nil {
			info.SwiftVersion = parseSwiftVersion(result.Stdout)
		}
	})

	probe(func() {
		if result, err := RunCommand(ctx, "xcodebuild", "-version"); err == nil {
			info.XcodeVersion = parseXcodeVersion(result.Stdout)
		}
	})

	wg.Wait()

	if len(errs) > 0 {
		return info, fmt.Errorf("toolchain discovery: %s", strings.Join(errs, "; "))
	}

	return info, nil
}

// parseSwiftVersion extracts the version from swiftc --version output.
// example input: "swift-driver version: 1.87.3 Apple Swift version 5.9.2 ..."
func parseSwiftVersion(output string) string {
	const marker = "Swift version "
	if idx := strings.Index(output, marker); idx >= 0 {
		rest := output[idx+len(marker):]
		if end := strings.IndexAny(rest, " \n("); end > 0 {
			return rest[:end]
		}
		return strings.TrimSpace(rest)
	}
	if line, _, ok := strings.Cut(output, "\n"); ok {
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(output)
}

// parseXcodeVersion extracts "Xcode X.Y" from xcodebuild -version output.
// example input: "Xcode 15.1\nBuild version 15C65"
func parseXcodeVersion(output string) string {
	line, _, _ := strings.Cut(output, "\n")
	return strings.TrimSpace(line)
}

func goarchToApple(goarch string) string {
	switch goarch {
	case "arm64":
		return "arm64"
	case "amd64":
		return "x86_64"
	default:
		return goarch
	}
}
