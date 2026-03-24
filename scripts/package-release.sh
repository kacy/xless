#!/bin/sh

set -eu

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
OUTPUT_DIR="${OUTPUT_DIR:-dist}"
GOOS_TARGET="${GOOS_TARGET:-darwin}"
GOARCH_TARGET="${GOARCH_TARGET:-arm64}"
LATEST_ARCHIVE_NAME="${LATEST_ARCHIVE_NAME:-xless_darwin_arm64.tar.gz}"

case "$VERSION" in
  *[!A-Za-z0-9._-]*)
    echo "VERSION may contain only letters, digits, dot, underscore, and hyphen (got: $VERSION)." >&2
    exit 1
    ;;
esac

for tool in go tar mktemp cp; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "required tool '$tool' is not available on PATH." >&2
    exit 1
  fi
done

if command -v shasum >/dev/null 2>&1; then
  checksum_file() {
    shasum -a 256 "$1" | awk '{print $1}'
  }
elif command -v sha256sum >/dev/null 2>&1; then
  checksum_file() {
    sha256sum "$1" | awk '{print $1}'
  }
else
  echo "required checksum tool not found (need shasum or sha256sum)." >&2
  exit 1
fi

VERSIONED_ARCHIVE_NAME="xless_${VERSION}_${GOOS_TARGET}_${GOARCH_TARGET}.tar.gz"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/xless-package.XXXXXX")"
STAGE_DIR="$WORK_DIR/stage"
BUILD_DIR="$WORK_DIR/build"

cleanup() {
  rm -rf "$WORK_DIR"
}

trap cleanup EXIT INT TERM

mkdir -p "$STAGE_DIR" "$BUILD_DIR" "$OUTPUT_DIR"

echo "Building xless $VERSION for $GOOS_TARGET/$GOARCH_TARGET"
CGO_ENABLED=0 GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" \
  go build -trimpath -ldflags "-X main.version=$VERSION" -o "$BUILD_DIR/xless" .

cp "$BUILD_DIR/xless" "$STAGE_DIR/xless"

if [ -f "README.md" ]; then
  cp "README.md" "$STAGE_DIR/README.md"
fi

if [ -f "LICENSE" ]; then
  cp "LICENSE" "$STAGE_DIR/LICENSE"
fi

VERSIONED_ARCHIVE_PATH="$OUTPUT_DIR/$VERSIONED_ARCHIVE_NAME"
LATEST_ARCHIVE_PATH="$OUTPUT_DIR/$LATEST_ARCHIVE_NAME"

tar -czf "$VERSIONED_ARCHIVE_PATH" -C "$STAGE_DIR" .
cp "$VERSIONED_ARCHIVE_PATH" "$LATEST_ARCHIVE_PATH"

VERSIONED_SUM="$(checksum_file "$VERSIONED_ARCHIVE_PATH")"
LATEST_SUM="$(checksum_file "$LATEST_ARCHIVE_PATH")"

printf '%s  %s\n' "$VERSIONED_SUM" "$VERSIONED_ARCHIVE_NAME" > "$VERSIONED_ARCHIVE_PATH.sha256"
printf '%s  %s\n' "$LATEST_SUM" "$LATEST_ARCHIVE_NAME" > "$LATEST_ARCHIVE_PATH.sha256"

echo "Created:"
echo "  $VERSIONED_ARCHIVE_PATH"
echo "  $VERSIONED_ARCHIVE_PATH.sha256"
echo "  $LATEST_ARCHIVE_PATH"
echo "  $LATEST_ARCHIVE_PATH.sha256"
