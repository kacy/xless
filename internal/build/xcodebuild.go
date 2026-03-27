package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kacy/xless/internal/toolchain"
)

var runXcodebuild = func(ctx context.Context, args ...string) (*toolchain.CommandResult, error) {
	return toolchain.RunCommand(ctx, "xcodebuild", args...)
}

type xcodebuildSettingsEntry struct {
	Target        string            `json:"target"`
	BuildSettings map[string]string `json:"buildSettings"`
}

type xcodebuildList struct {
	Project   *xcodebuildListGroup `json:"project"`
	Workspace *xcodebuildListGroup `json:"workspace"`
}

type xcodebuildListGroup struct {
	Name    string   `json:"name"`
	Targets []string `json:"targets"`
	Schemes []string `json:"schemes"`
}

type xcodebuildSelector struct {
	flag  string
	value string
}

type XcodebuildSelectionResolver struct {
	ctx          context.Context
	workspaceDir string
	xcodeprojDir string
	explicit     string
	listing      *xcodebuildListGroup
	listingErr   error
	listingDone  bool
}

type ResolvedXcodebuildSelection struct {
	Flag   string
	Value  string
	Scheme string
}

func (s ResolvedXcodebuildSelection) Selector() string {
	if s.Flag == "" || s.Value == "" {
		return ""
	}
	return s.Flag + "=" + s.Value
}

type xcodebuildSelectionError struct {
	message string
	hint    string
}

func (e *xcodebuildSelectionError) Error() string {
	return e.message
}

func XcodebuildSelectionHint(err error) string {
	var selectionErr *xcodebuildSelectionError
	if errors.As(err, &selectionErr) {
		return selectionErr.hint
	}
	return ""
}

type XcodebuildBuildStage struct{}

func (XcodebuildBuildStage) Name() string { return "xcodebuild" }

func (XcodebuildBuildStage) Run(bc *BuildContext) error {
	selector, err := resolveXcodebuildSelector(bc)
	if err != nil {
		hint := "run `xcodebuild -list` to see the buildable shared schemes for this project"
		var selectionErr *xcodebuildSelectionError
		if errors.As(err, &selectionErr) && selectionErr.hint != "" {
			hint = selectionErr.hint
		}
		return &BuildError{
			Stage: "xcodebuild",
			Err:   err,
			Hint:  hint,
		}
	}
	bc.XcodeSelectorFlag = selector.flag
	bc.XcodeSelectorValue = selector.value
	if selector.flag == "-scheme" {
		bc.XcodeSchemeResolved = selector.value
	}

	settings, err := xcodebuildTargetSettings(bc, selector)
	if err != nil {
		hint := XcodebuildSelectionHint(err)
		if hint == "" {
			hint = "check that the selected Xcode scheme is shared and buildable for the requested platform"
		}
		return &BuildError{
			Stage: "xcodebuild",
			Err:   err,
			Hint:  hint,
		}
	}

	if bc.Platform == toolchain.PlatformDevice {
		return runXcodebuildArchiveExport(bc, selector, settings)
	}

	return runXcodebuildBuild(bc, selector, settings)
}

// ResolveXcodebuildSelection returns the delegated Xcode build entry xless would use.
func ResolveXcodebuildSelection(ctx context.Context, workspaceDir, xcodeprojDir, targetName, scheme string) (*ResolvedXcodebuildSelection, error) {
	return NewXcodebuildSelectionResolver(ctx, workspaceDir, xcodeprojDir, scheme).Resolve(targetName)
}

func NewXcodebuildSelectionResolver(ctx context.Context, workspaceDir, xcodeprojDir, scheme string) *XcodebuildSelectionResolver {
	return &XcodebuildSelectionResolver{
		ctx:          ctx,
		workspaceDir: workspaceDir,
		xcodeprojDir: xcodeprojDir,
		explicit:     scheme,
	}
}

func (r *XcodebuildSelectionResolver) Resolve(targetName string) (*ResolvedXcodebuildSelection, error) {
	selector, err := r.resolve(targetName)
	if err != nil {
		return nil, err
	}

	resolved := &ResolvedXcodebuildSelection{
		Flag:  selector.flag,
		Value: selector.value,
	}
	if selector.flag == "-scheme" {
		resolved.Scheme = selector.value
	}
	return resolved, nil
}

func (r *XcodebuildSelectionResolver) resolve(targetName string) (xcodebuildSelector, error) {
	if r.explicit != "" {
		return xcodebuildSelector{flag: "-scheme", value: r.explicit}, nil
	}
	if targetName == "" {
		return xcodebuildSelector{}, &xcodebuildSelectionError{
			message: "xcodebuild selection requires a target name",
			hint:    "select a target with `--target` or configure defaults.target in xless.yml",
		}
	}

	listing, err := r.loadListing()
	if err != nil {
		return xcodebuildSelector{}, err
	}

	schemes := uniqueStrings(listing.Schemes)
	if containsString(schemes, targetName) {
		return xcodebuildSelector{flag: "-scheme", value: targetName}, nil
	}
	if len(schemes) == 1 {
		return xcodebuildSelector{flag: "-scheme", value: schemes[0]}, nil
	}
	container := "project"
	if r.workspaceDir != "" {
		container = "workspace"
	}
	if len(schemes) == 0 {
		return xcodebuildSelector{}, &xcodebuildSelectionError{
			message: fmt.Sprintf("%s has no shared Xcode schemes for target %q", container, targetName),
			hint:    "share a scheme in Xcode with Product > Scheme > Manage Schemes, then rerun xless or pass --scheme",
		}
	}
	return xcodebuildSelector{}, &xcodebuildSelectionError{
		message: fmt.Sprintf("no shared Xcode scheme matched target %q (available shared schemes: %s)", targetName, strings.Join(schemes, ", ")),
		hint:    "pass --scheme <name> or share a scheme whose name matches the selected target",
	}
}

func (r *XcodebuildSelectionResolver) loadListing() (*xcodebuildListGroup, error) {
	if r.listingDone {
		return r.listing, r.listingErr
	}
	r.listingDone = true
	r.listing, r.listingErr = xcodebuildListJSON(&BuildContext{
		Ctx:          r.ctx,
		WorkspaceDir: r.workspaceDir,
		XcodeprojDir: r.xcodeprojDir,
	})
	return r.listing, r.listingErr
}

func runXcodebuildBuild(bc *BuildContext, selector xcodebuildSelector, settings map[string]string) error {
	args := xcodebuildArgs(bc, selector)
	args = append(args, "build")
	result, err := runXcodebuild(bc.Ctx, args...)
	if err != nil {
		return wrapXcodebuildFailure("build", result, err)
	}

	productDir := settings["TARGET_BUILD_DIR"]
	fullProductName := settings["FULL_PRODUCT_NAME"]
	if productDir == "" || fullProductName == "" {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   fmt.Errorf("xcodebuild did not report TARGET_BUILD_DIR/FULL_PRODUCT_NAME for target %q", bc.Target.Name),
			Hint:  "the selected scheme may not produce an app bundle",
		}
	}

	builtProduct := filepath.Join(productDir, fullProductName)
	info, err := os.Stat(builtProduct)
	if err != nil {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   fmt.Errorf("built product %q: %w", builtProduct, err),
			Hint:  "xcodebuild may have built to a different product type than xless expected",
		}
	}
	if !info.IsDir() || filepath.Ext(builtProduct) != ".app" {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   fmt.Errorf("built product %q is not an app bundle", builtProduct),
			Hint:  "xless run/build currently expects the selected target to produce an .app bundle",
		}
	}

	if err := normalizeBuiltApp(bc, builtProduct); err != nil {
		return err
	}

	bc.Out.Info("xcodebuild", "selector", selector.flag+"="+selector.value, "path", bc.AppBundlePath)
	return nil
}

func xcodebuildTargetSettings(bc *BuildContext, selector xcodebuildSelector) (map[string]string, error) {
	args := xcodebuildArgs(bc, selector)
	args = append(args, "-showBuildSettings", "-json")

	result, err := runXcodebuild(bc.Ctx, args...)
	if err != nil {
		stderr := ""
		if result != nil {
			stderr = strings.TrimSpace(result.Stderr)
		}
		if stderr != "" {
			return nil, &xcodebuildSelectionError{
				message: fmt.Sprintf("xcodebuild -showBuildSettings failed:\n%s", stderr),
				hint:    xcodebuildCommandHint("showBuildSettings", stderr),
			}
		}
		return nil, err
	}

	var entries []xcodebuildSettingsEntry
	if err := json.Unmarshal([]byte(result.Stdout), &entries); err != nil {
		return nil, fmt.Errorf("parsing xcodebuild build settings JSON: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("xcodebuild returned no build settings")
	}

	if settings := selectBuildSettings(entries, bc.Target.Name); settings != nil {
		return settings, nil
	}
	return nil, fmt.Errorf("build settings for target %q not found in xcodebuild output", bc.Target.Name)
}

func runXcodebuildArchiveExport(bc *BuildContext, selector xcodebuildSelector, settings map[string]string) error {
	archivePath := filepath.Join(bc.BuildDir, bc.Target.Name+".xcarchive")
	exportDir := filepath.Join(bc.BuildDir, "Export")
	exportOptionsPath := filepath.Join(bc.BuildDir, "ExportOptions.plist")

	for _, path := range []string{archivePath, exportDir} {
		if err := os.RemoveAll(path); err != nil {
			return &BuildError{
				Stage: "xcodebuild",
				Err:   fmt.Errorf("cannot clear previous xcodebuild output %q: %w", path, err),
			}
		}
	}

	archiveArgs := xcodebuildArgs(bc, selector)
	archiveArgs = append(archiveArgs, "archive", "-archivePath", archivePath)
	result, err := runXcodebuild(bc.Ctx, archiveArgs...)
	if err != nil {
		return wrapXcodebuildFailure("archive", result, err)
	}

	fullProductName := settings["FULL_PRODUCT_NAME"]
	if fullProductName == "" {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   fmt.Errorf("xcodebuild did not report FULL_PRODUCT_NAME for target %q", bc.Target.Name),
			Hint:  "the selected scheme may not produce an app bundle",
		}
	}

	archivedApp := filepath.Join(archivePath, "Products", "Applications", fullProductName)
	info, err := os.Stat(archivedApp)
	if err != nil {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   fmt.Errorf("archived product %q: %w", archivedApp, err),
			Hint:  "xcodebuild archive may have produced a different product type than xless expected",
		}
	}
	if !info.IsDir() || filepath.Ext(archivedApp) != ".app" {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   fmt.Errorf("archived product %q is not an app bundle", archivedApp),
			Hint:  "xless run/build currently expects the selected target to produce an .app bundle",
		}
	}

	if err := normalizeBuiltApp(bc, archivedApp); err != nil {
		return err
	}

	if err := writeExportOptionsPlist(exportOptionsPath, bc.Target.Signing.TeamID); err != nil {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   fmt.Errorf("cannot write export options: %w", err),
			Hint:  "xless could not prepare xcodebuild export options for a device IPA",
		}
	}

	exportArgs := []string{
		"-exportArchive",
		"-archivePath", archivePath,
		"-exportPath", exportDir,
		"-exportOptionsPlist", exportOptionsPath,
	}
	result, err = runXcodebuild(bc.Ctx, exportArgs...)
	if err != nil {
		return wrapXcodebuildFailure("export", result, err)
	}

	ipaPath, err := findExportedIPA(exportDir)
	if err != nil {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   err,
			Hint:  "xcodebuild archive succeeded, but IPA export did not produce a .ipa",
		}
	}

	bc.IPAPath = ipaPath
	bc.Out.Info("xcodebuild",
		"selector", selector.flag+"="+selector.value,
		"archive", archivePath,
		"ipa", ipaPath,
	)
	return nil
}

func normalizeBuiltApp(bc *BuildContext, builtProduct string) error {
	if err := os.MkdirAll(bc.BuildDir, 0o755); err != nil {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   fmt.Errorf("cannot create build directory: %w", err),
			Hint:  "could not create build directory",
		}
	}

	normalizedApp := filepath.Join(bc.BuildDir, filepath.Base(builtProduct))
	if err := os.RemoveAll(normalizedApp); err != nil {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   fmt.Errorf("cannot clear previous app bundle %q: %w", normalizedApp, err),
		}
	}
	if err := copyDir(builtProduct, normalizedApp); err != nil {
		return &BuildError{
			Stage: "xcodebuild",
			Err:   fmt.Errorf("cannot copy built app bundle: %w", err),
			Hint:  "xcodebuild produced an app, but xless could not normalize it into .build",
		}
	}

	bc.AppBundlePath = normalizedApp
	return nil
}

func wrapXcodebuildFailure(step string, result *toolchain.CommandResult, err error) error {
	stderr := ""
	if result != nil {
		stderr = strings.TrimSpace(result.Stderr)
	}
	detail := fmt.Errorf("xcodebuild %s failed: %w", step, err)
	if stderr != "" {
		detail = fmt.Errorf("xcodebuild %s failed:\n%s: %w", step, stderr, err)
	}
	return &BuildError{
		Stage: "xcodebuild",
		Err:   detail,
		Hint:  xcodebuildCommandHint(step, stderr),
	}
}

func writeExportOptionsPlist(path, teamID string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>method</key>
	<string>development</string>
	<key>stripSwiftSymbols</key>
	<true/>
	<key>compileBitcode</key>
	<false/>`
	if teamID != "" {
		content += `
	<key>teamID</key>
	<string>` + teamID + `</string>`
	}
	content += `
</dict>
</plist>
`
	return os.WriteFile(path, []byte(content), 0o644)
}

func findExportedIPA(exportDir string) (string, error) {
	entries, err := os.ReadDir(exportDir)
	if err != nil {
		return "", fmt.Errorf("reading export directory %q: %w", exportDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".ipa") {
			return filepath.Join(exportDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no IPA found in export directory %q", exportDir)
}

func resolveXcodebuildSelector(bc *BuildContext) (xcodebuildSelector, error) {
	targetName := ""
	if bc.Target != nil {
		targetName = bc.Target.Name
	}
	return NewXcodebuildSelectionResolver(bc.Ctx, bc.WorkspaceDir, bc.XcodeprojDir, bc.XcodeScheme).resolve(targetName)
}

func xcodebuildListJSON(bc *BuildContext) (*xcodebuildListGroup, error) {
	args := []string{"-list", "-json"}
	switch {
	case bc.WorkspaceDir != "":
		args = append(args, "-workspace", bc.WorkspaceDir)
	case bc.XcodeprojDir != "":
		args = append(args, "-project", bc.XcodeprojDir)
	default:
		return nil, fmt.Errorf("xcodebuild list requires a workspace or xcodeproj")
	}

	result, err := runXcodebuild(bc.Ctx, args...)
	if err != nil {
		stderr := ""
		if result != nil {
			stderr = strings.TrimSpace(result.Stderr)
		}
		if stderr != "" {
			return nil, &xcodebuildSelectionError{
				message: fmt.Sprintf("xcodebuild -list failed:\n%s", stderr),
				hint:    xcodebuildCommandHint("list", stderr),
			}
		}
		return nil, err
	}

	var listing xcodebuildList
	if err := json.Unmarshal([]byte(result.Stdout), &listing); err != nil {
		return nil, fmt.Errorf("parsing xcodebuild -list JSON: %w", err)
	}
	if listing.Workspace != nil {
		return listing.Workspace, nil
	}
	if listing.Project != nil {
		return listing.Project, nil
	}
	return nil, fmt.Errorf("xcodebuild -list did not return project or workspace data")
}

func selectBuildSettings(entries []xcodebuildSettingsEntry, targetName string) map[string]string {
	for _, entry := range entries {
		if entry.Target == targetName && len(entry.BuildSettings) > 0 {
			return entry.BuildSettings
		}
	}
	for _, entry := range entries {
		if len(entry.BuildSettings) == 0 {
			continue
		}
		if entry.BuildSettings["TARGET_NAME"] == targetName {
			return entry.BuildSettings
		}
	}
	for _, entry := range entries {
		if len(entry.BuildSettings) == 0 {
			continue
		}
		if entry.BuildSettings["PRODUCT_NAME"] == targetName || entry.BuildSettings["FULL_PRODUCT_NAME"] == targetName+".app" {
			return entry.BuildSettings
		}
	}
	return nil
}

func xcodebuildArgs(bc *BuildContext, selector xcodebuildSelector) []string {
	var args []string
	buildRoot := filepath.Join(bc.BuildDir, "XcodeBuild")
	switch {
	case bc.WorkspaceDir != "":
		args = append(args, "-workspace", bc.WorkspaceDir)
	case bc.XcodeprojDir != "":
		args = append(args, "-project", bc.XcodeprojDir)
	}
	args = append(args, selector.flag, selector.value)

	args = append(args,
		"-configuration", xcodebuildConfigurationName(bc.BuildConfig),
		"-sdk", xcodebuildSDK(bc.Platform),
	)
	if selector.flag == "-target" {
		args = append(args,
			"SYMROOT="+filepath.Join(buildRoot, "Build", "Products"),
			"OBJROOT="+filepath.Join(buildRoot, "Build", "Intermediates.noindex"),
		)
	} else {
		args = append(args, "-derivedDataPath", buildRoot)
	}
	args = append(args, "-clonedSourcePackagesDirPath", filepath.Join(bc.BuildDir, "SourcePackages"))

	if destination := xcodebuildDestination(bc.Platform); destination != "" {
		args = append(args, "-destination", destination)
	}
	if bc.Platform == toolchain.PlatformSimulator {
		args = append(args, "CODE_SIGNING_ALLOWED=NO")
	}
	return args
}

func xcodebuildSDK(platform toolchain.Platform) string {
	if platform == toolchain.PlatformDevice {
		return "iphoneos"
	}
	return "iphonesimulator"
}

func xcodebuildConfigurationName(name string) string {
	switch strings.ToLower(name) {
	case "debug":
		return "Debug"
	case "release":
		return "Release"
	default:
		return name
	}
}

func xcodebuildCommandHint(step, stderr string) string {
	if strings.Contains(stderr, "Unable to find a destination matching") {
		if strings.Contains(stderr, "is not installed") {
			return "install the required iOS simulator or device platform in Xcode > Settings > Components, or choose a different platform"
		}
		return "check that a matching simulator/device destination is available for the selected Xcode scheme"
	}
	if strings.Contains(stderr, "Supported platforms for the buildables in the current scheme is empty") {
		return "the selected Xcode scheme is not buildable for this platform; choose a different scheme or platform"
	}
	if strings.Contains(stderr, "is not a workspace file") {
		return "verify that the selected .xcworkspace is a valid top-level Xcode workspace"
	}
	if step == "list" {
		return "run `xcodebuild -list` directly to verify the workspace/project and its shared schemes"
	}
	if step == "showBuildSettings" {
		return "check that the selected scheme is shared and buildable for the requested platform"
	}
	if step == "export" {
		return "check signing, provisioning, and archive export settings for the selected Xcode scheme"
	}
	return "run `xcodebuild -list` to verify the scheme exists and is shared"
}

func xcodebuildDestination(platform toolchain.Platform) string {
	if platform == toolchain.PlatformDevice {
		return "generic/platform=iOS"
	}
	return "generic/platform=iOS Simulator"
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
