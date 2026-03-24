#!/bin/sh

set -eu

OWNER_REPO="${XLESS_REPO:-kacy/xless}"
VERSION="${XLESS_VERSION:-latest}"
DOWNLOAD_URL_OVERRIDE="${XLESS_DOWNLOAD_URL:-}"
TMP_DIR="${TMPDIR:-/tmp}"

OS="$(uname -s)"
ARCH="$(uname -m)"

for tool in curl tar install mktemp; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "required tool '$tool' is not available on PATH." >&2
    exit 1
  fi
done

if [ "$OS" != "Darwin" ]; then
  echo "xless install is supported on macOS only (detected: $OS)." >&2
  exit 1
fi

if [ "$ARCH" != "arm64" ]; then
  echo "xless install currently supports Apple Silicon only (detected: $ARCH)." >&2
  exit 1
fi

if [ -n "$DOWNLOAD_URL_OVERRIDE" ]; then
  DOWNLOAD_URL="$DOWNLOAD_URL_OVERRIDE"
else
  case "$VERSION" in
    latest)
      DOWNLOAD_URL="https://github.com/$OWNER_REPO/releases/latest/download/xless_darwin_arm64.tar.gz"
      ;;
    *)
      DOWNLOAD_URL="https://github.com/$OWNER_REPO/releases/download/$VERSION/xless_darwin_arm64.tar.gz"
      ;;
  esac
fi

WORK_DIR="$(mktemp -d "$TMP_DIR/xless-install.XXXXXX")"
ARCHIVE_PATH="$WORK_DIR/xless_darwin_arm64.tar.gz"
BINARY_PATH="$WORK_DIR/xless"

cleanup() {
  rm -rf "$WORK_DIR"
}

trap cleanup EXIT INT TERM

echo "Downloading xless from $DOWNLOAD_URL"
curl -fsSL "$DOWNLOAD_URL" -o "$ARCHIVE_PATH"

tar -xzf "$ARCHIVE_PATH" -C "$WORK_DIR"

if [ ! -f "$BINARY_PATH" ]; then
  echo "release archive did not contain an xless binary at the archive root." >&2
  exit 1
fi

chmod +x "$BINARY_PATH"

if [ -n "${XLESS_INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$XLESS_INSTALL_DIR"
elif [ -d "/opt/homebrew/bin" ]; then
  INSTALL_DIR="/opt/homebrew/bin"
else
  INSTALL_DIR="/usr/local/bin"
fi

if [ ! -d "$INSTALL_DIR" ]; then
  if mkdir -p "$INSTALL_DIR" 2>/dev/null; then
    :
  elif command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$INSTALL_DIR"
  else
    echo "failed to create $INSTALL_DIR and sudo is unavailable." >&2
    exit 1
  fi
fi

TARGET_PATH="$INSTALL_DIR/xless"

if install -m 0755 "$BINARY_PATH" "$TARGET_PATH" 2>/dev/null; then
  :
elif command -v sudo >/dev/null 2>&1; then
  sudo install -m 0755 "$BINARY_PATH" "$TARGET_PATH"
else
  echo "failed to write to $TARGET_PATH and sudo is unavailable." >&2
  exit 1
fi

echo "Installed xless to $TARGET_PATH"
echo "Run 'xless version' to verify the installation."
