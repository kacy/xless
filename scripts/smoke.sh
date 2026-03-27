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
	expected_scheme=$4

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
		info_output=$("$BINARY_PATH" info --target "$target_name" --json)
		printf '%s\n' "$info_output" | grep -q "\"message\":\"selection\""
		printf '%s\n' "$info_output" | grep -q "\"backend\":\"xcodebuild\""
		printf '%s\n' "$info_output" | grep -q "\"xcode_scheme\":\"$expected_scheme\""

		build_output=$("$BINARY_PATH" build --platform simulator --target "$target_name" --json)
		printf '%s\n' "$build_output" | grep -q "\"message\":\"build\""
		printf '%s\n' "$build_output" | grep -q "\"backend\":\"xcodebuild\""
		printf '%s\n' "$build_output" | grep -q "\"scheme\":\"$expected_scheme\""
		test -d ".build/$target_name/$target_name.app"

		clean_output=$("$BINARY_PATH" clean --json)
		printf '%s\n' "$clean_output" | grep -q "\"message\":\"cleaned\""
	)

	cleanup
	trap - EXIT INT TERM
}

run_workspace_fixture() {
	fixture_source=$1
	target_name=$2

	tmp_dir=$(mktemp -d "/tmp/xless-smoke-workspace.XXXXXX")
	cleanup() {
		if [ "$KEEP_FIXTURES" = "1" ]; then
			echo "kept smoke fixture: $tmp_dir"
			return
		fi
		rm -rf "$tmp_dir"
	}
	trap cleanup EXIT INT TERM

	cp -R "$fixture_source"/. "$tmp_dir"/

	echo "== workspace =="
	(
		cd "$tmp_dir"
		info_output=$("$BINARY_PATH" info --target "$target_name" --json)
		printf '%s\n' "$info_output" | grep -q "\"mode\":\"xcworkspace\""
		printf '%s\n' "$info_output" | grep -q "\"message\":\"selection\""
		printf '%s\n' "$info_output" | grep -q "\"backend\":\"xcodebuild\""
	)

	cleanup
	trap - EXIT INT TERM
}

run_fixture "project" "$ROOT_DIR/testdata/smoke/project/ExampleProject" "ExampleProject" "ExampleProject"
run_workspace_fixture "$ROOT_DIR/testdata/smoke/workspace/ExampleWorkspace" "WorkspaceApp"

echo "delegated smoke checks passed"
