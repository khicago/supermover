#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
MACOS_DIR="$ROOT_DIR/macos"
ASSETS_DIR="$MACOS_DIR/script/assets"
ICON_SOURCE_FILE="$ASSETS_DIR/AppIcon/SuperMoverAppIcon.png"

INIT_DIRS=false
CHECK_AUDIT=false
CHECK_NOTARY=false
INSTALL_XCODE=false

usage() {
  cat <<'EOF'
usage: macos/script/bootstrap-build-env.sh [options]

Prepare and verify the local macOS app build environment.

Options:
  --for-build-app        initialize build output directories and verify build tools
  --with-audit           also verify tools used by script/audit-app.sh
  --with-notary          also verify tools used by script/notarize-app.sh
  --install-xcode-tools  request the macOS Command Line Tools installer if missing
  -h, --help             show this help

The script is intentionally conservative. It creates project-local build
directories and checks required tools, but it does not silently install Go,
Developer ID certificates, keychain items, or notarization credentials.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --for-build-app)
      INIT_DIRS=true
      ;;
    --with-audit)
      CHECK_AUDIT=true
      ;;
    --with-notary)
      CHECK_NOTARY=true
      CHECK_AUDIT=true
      ;;
    --install-xcode-tools)
      INSTALL_XCODE=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'unknown option: %s\n\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

missing=0

ok() {
  printf 'ok: %s\n' "$1"
}

fail() {
  printf 'missing: %s\n' "$1" >&2
  missing=1
}

require_command() {
  name=$1
  hint=$2
  if command -v "$name" >/dev/null 2>&1; then
    ok "$name ($(command -v "$name"))"
  else
    fail "$name - $hint"
  fi
}

if [ "$(uname -s)" != "Darwin" ]; then
  fail "macOS host - packaged app builds require macOS"
else
  ok "macOS host"
fi

if xcode-select -p >/dev/null 2>&1; then
  ok "Xcode Command Line Tools ($(xcode-select -p))"
else
  fail "Xcode Command Line Tools - run xcode-select --install"
  if [ "$INSTALL_XCODE" = "true" ]; then
    xcode-select --install >/dev/null 2>&1 || true
    printf 'requested Xcode Command Line Tools installer; rerun after it completes\n' >&2
  fi
fi

require_command swift "install Xcode or Xcode Command Line Tools"
require_command go "install Go matching the version declared in go.mod"
require_command git "install Xcode Command Line Tools or Git"
require_command sips "install Xcode Command Line Tools"
require_command iconutil "install Xcode Command Line Tools"
require_command codesign "install Xcode Command Line Tools"

if [ "$CHECK_AUDIT" = "true" ]; then
  require_command jq "install jq before running script/audit-app.sh"
  require_command spctl "install Xcode Command Line Tools"
  require_command xcrun "install Xcode or Xcode Command Line Tools"
fi

if [ "$CHECK_NOTARY" = "true" ]; then
  require_command ditto "install Xcode Command Line Tools"
  require_command stapler "install Xcode or Xcode Command Line Tools with notarization support"
fi

if [ -f "$ICON_SOURCE_FILE" ]; then
  ok "app icon source"
else
  fail "app icon source at macos/script/assets/AppIcon/SuperMoverAppIcon.png"
fi

if [ -f "$ROOT_DIR/go.mod" ]; then
  required_go=$(awk '$1 == "go" { print $2; exit }' "$ROOT_DIR/go.mod")
  if [ -n "$required_go" ]; then
    ok "go.mod declares Go $required_go"
  fi
fi

if [ "$INIT_DIRS" = "true" ]; then
  mkdir -p "$MACOS_DIR/.build" "$MACOS_DIR/dist"
  ok "initialized macos/.build and macos/dist"
fi

if [ "$missing" -ne 0 ]; then
  cat >&2 <<'EOF'

SuperMover macOS build environment is incomplete.

The bootstrap script only performs project-local initialization and explicit
tool checks. Install missing system tools or credentials manually, then rerun.
EOF
  exit 5
fi

ok "macOS build environment ready"
