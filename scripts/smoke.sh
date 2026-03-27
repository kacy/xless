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
	simulator_udid=$5

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

		if [ -n "$simulator_udid" ]; then
			run_output=$("$BINARY_PATH" run --platform simulator --target "$target_name" --device "$simulator_udid" --json)
			printf '%s\n' "$run_output" | grep -q "\"message\":\"run\""
			printf '%s\n' "$run_output" | grep -q "\"artifact\":"
			printf '%s\n' "$run_output" | grep -q "\"backend\":\"xcodebuild\""
			printf '%s\n' "$run_output" | grep -q "\"scheme\":\"$expected_scheme\""

			run_logs_output="$tmp_dir/run-logs.ndjson"
			("$BINARY_PATH" run --platform simulator --target "$target_name" --device "$simulator_udid" --logs --json >"$run_logs_output" 2>&1) &
			run_logs_pid=$!
			sleep 12
			kill -INT "$run_logs_pid" 2>/dev/null || true
			wait "$run_logs_pid" 2>/dev/null || true
			grep -q "\"message\":\"run\"" "$run_logs_output"
			grep -q "\"message\":\"streaming logs\"" "$run_logs_output"

			logs_output="$tmp_dir/logs.ndjson"
			("$BINARY_PATH" logs --target "$target_name" --device "$simulator_udid" --json >"$logs_output" 2>&1) &
			logs_pid=$!
			sleep 3
			kill -INT "$logs_pid" 2>/dev/null || true
			wait "$logs_pid" 2>/dev/null || true
			grep -q "\"message\":\"streaming logs\"" "$logs_output"

			explicit_logs_output="$tmp_dir/logs-explicit.ndjson"
			("$BINARY_PATH" logs --bundle-id "com.example.ExampleProject" --device "$simulator_udid" --json >"$explicit_logs_output" 2>&1) &
			explicit_logs_pid=$!
			sleep 3
			kill -INT "$explicit_logs_pid" 2>/dev/null || true
			wait "$explicit_logs_pid" 2>/dev/null || true
			grep -q "\"message\":\"streaming logs\"" "$explicit_logs_output"
		else
			echo "skipping simulator run/logs smoke for $target_name: no available simulator device"
		fi

		clean_output=$("$BINARY_PATH" clean --json)
		printf '%s\n' "$clean_output" | grep -q "\"message\":\"cleaned\""
	)

	cleanup
	trap - EXIT INT TERM
}

select_simulator_udid() {
	"$BINARY_PATH" devices --simulators --json | awk -F'"' '
		/"message":"simulator"/ {
			for (i = 1; i <= NF; i++) {
				if ($i == "udid") {
					print $(i+2)
					exit
				}
			}
		}
	'
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

simulator_udid=$(select_simulator_udid)
run_fixture "project" "$ROOT_DIR/testdata/smoke/project/ExampleProject" "ExampleProject" "ExampleProject" "$simulator_udid"
run_workspace_fixture "$ROOT_DIR/testdata/smoke/workspace/ExampleWorkspace" "WorkspaceApp"

echo "delegated smoke checks passed"
