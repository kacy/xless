#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BINARY_PATH=${XLESS_SMOKE_BINARY:-/tmp/xless-smoke}
KEEP_FIXTURES=${XLESS_SMOKE_KEEP:-0}

if ! command -v xcodebuild >/dev/null 2>&1; then
	echo "xcodebuild is required for delegated smoke tests" >&2
	exit 1
fi

if ! xcodebuild -showsdks 2>/dev/null | grep -q "iphonesimulator"; then
	echo "iphonesimulator SDK is not available in the active Xcode install" >&2
	exit 1
fi

if ! build_output=$(go build -o "$BINARY_PATH" "$ROOT_DIR" 2>&1); then
	echo "$build_output" >&2
	exit 1
fi

run_fixture() {
	fixture_name=$1
	fixture_source=$2
	target_name=$3

	tmp_dir=$(mktemp -d "/tmp/xless-smoke-${fixture_name}.XXXXXX")
	cleanup() {
		if [ "$KEEP_FIXTURES" = "1" ]; then
			echo "kept smoke fixture: $tmp_dir"
			return
		fi
		rm -rf "$tmp_dir"
	}
	trap cleanup EXIT INT TERM

	cp -R "$fixture_source"/. "$tmp_dir"/

	echo "== ${fixture_name} =="
	(
		cd "$tmp_dir"
		"$BINARY_PATH" info --target "$target_name" --json >/dev/null
		"$BINARY_PATH" build --platform simulator --target "$target_name" --json >/dev/null
		"$BINARY_PATH" clean --json >/dev/null
	)

	cleanup
	trap - EXIT INT TERM
}

run_fixture "project" "$ROOT_DIR/testdata/smoke/project/ExampleProject" "ExampleProject"
run_fixture "workspace" "$ROOT_DIR/testdata/smoke/workspace/ExampleWorkspace" "WorkspaceApp"

echo "delegated smoke checks passed"
