#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
MACOS_DIR="$ROOT_DIR/macos"
. "$MACOS_DIR/script/lib/app-build-resources.sh"
BUILD_DIR="$MACOS_DIR/.build/arm64-apple-macosx/release"
APP_DIR="$MACOS_DIR/dist/SuperMover.app"
CONTENTS_DIR="$APP_DIR/Contents"
MACOS_CONTENTS_DIR="$CONTENTS_DIR/MacOS"
RESOURCES_DIR="$CONTENTS_DIR/Resources"
BUNDLE_BIN_DIR="$RESOURCES_DIR/bin"
PROVENANCE_FILE="$RESOURCES_DIR/supermover-provenance.json"
ASSETS_DIR="$MACOS_DIR/script/assets"
ICON_SOURCE_FILE="$ASSETS_DIR/AppIcon/SuperMoverAppIcon.png"
REFERENCE_IMAGES_DIR="$ASSETS_DIR/ReferenceImages"
ICON_NAME="SuperMover"
ICON_FILE="$RESOURCES_DIR/$ICON_NAME.icns"
CODESIGN_IDENTITY="${SUPERMOVER_CODESIGN_IDENTITY:-}"

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

sign_one() {
  sign_target=$1
  shift
  if [ "$CODESIGN_IDENTITY" = "-" ]; then
    codesign --force --options runtime "$@" --sign "$CODESIGN_IDENTITY" "$sign_target"
  else
    codesign --force --options runtime --timestamp "$@" --sign "$CODESIGN_IDENTITY" "$sign_target"
  fi
}

build_icon() {
  iconset_dir=$1
  mkdir -p "$iconset_dir"
  for size in 16 32 128 256 512; do
    sips -z "$size" "$size" "$ICON_SOURCE_FILE" --out "$iconset_dir/icon_${size}x${size}.png" >/dev/null
    retina_size=$((size * 2))
    sips -z "$retina_size" "$retina_size" "$ICON_SOURCE_FILE" --out "$iconset_dir/icon_${size}x${size}@2x.png" >/dev/null
  done
  iconutil -c icns "$iconset_dir" -o "$ICON_FILE"
}

copy_optional_reference_images() {
  if [ -d "$REFERENCE_IMAGES_DIR" ]; then
    mkdir -p "$RESOURCES_DIR/ReferenceImages"
    cp -R "$REFERENCE_IMAGES_DIR/." "$RESOURCES_DIR/ReferenceImages/"
  fi
}

"$MACOS_DIR/script/bootstrap-build-env.sh" --for-build-app

cd "$MACOS_DIR"
swift build -c release
go build -o "$MACOS_DIR/dist/supermover" "$ROOT_DIR/cmd/supermover"

rm -rf "$APP_DIR"
mkdir -p "$MACOS_CONTENTS_DIR" "$RESOURCES_DIR" "$BUNDLE_BIN_DIR"
cp "$MACOS_DIR/script/Info.plist" "$CONTENTS_DIR/Info.plist"
cp "$BUILD_DIR/SuperMoverApp" "$MACOS_CONTENTS_DIR/SuperMoverApp"
cp "$MACOS_DIR/dist/supermover" "$BUNDLE_BIN_DIR/supermover"
cp "$BUILD_DIR/SuperMoverPackagedAppAudit" "$BUNDLE_BIN_DIR/supermover-app-audit"
chmod +x "$BUNDLE_BIN_DIR/supermover"
chmod +x "$BUNDLE_BIN_DIR/supermover-app-audit"
copy_swiftpm_app_resources "$BUILD_DIR/SuperMoverApp_SuperMoverApp.bundle" "$RESOURCES_DIR"
ICON_WORK_DIR=$(mktemp -d "$MACOS_DIR/.build/$ICON_NAME.icon.XXXXXX")
ICONSET_DIR="$ICON_WORK_DIR/$ICON_NAME.iconset"
trap 'rm -rf "$ICON_WORK_DIR"' EXIT INT TERM
build_icon "$ICONSET_DIR"
copy_optional_reference_images
rm -f "$RESOURCES_DIR/.gitkeep"

GIT_COMMIT=$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD 2>/dev/null || printf 'unknown')
GIT_STATUS=$(git -C "$ROOT_DIR" status --porcelain --untracked-files=normal -- . 2>/dev/null || true)
GIT_DIRTY=$([ -z "$GIT_STATUS" ] && printf 'false' || printf 'true')
CLI_VERSION=$("$BUNDLE_BIN_DIR/supermover" version 2>/dev/null || printf 'unknown')
BUILD_PROFILE="${SUPERMOVER_BUILD_PROFILE:-local-release}"
BUILT_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

cat > "$PROVENANCE_FILE" <<EOF
{
  "schema": "supermover.macos.provenance.v1",
  "app_bundle_id": "dev.supermover.macapp",
  "app_version": "0.1.0",
  "build_profile": "$(json_escape "$BUILD_PROFILE")",
  "git_commit": "$(json_escape "$GIT_COMMIT")",
  "git_dirty": $GIT_DIRTY,
  "cli_version": "$(json_escape "$CLI_VERSION")",
  "cli_relative_path": "Contents/Resources/bin/supermover",
  "built_at": "$(json_escape "$BUILT_AT")",
  "signing": "$(json_escape "${CODESIGN_IDENTITY:-unsigned}")"
}
EOF

if [ -n "$CODESIGN_IDENTITY" ]; then
  sign_one "$BUNDLE_BIN_DIR/supermover" --entitlements "$MACOS_DIR/script/SuperMover.entitlements"
  sign_one "$BUNDLE_BIN_DIR/supermover-app-audit" --entitlements "$MACOS_DIR/script/SuperMover.entitlements"
  sign_one "$APP_DIR" --entitlements "$MACOS_DIR/script/SuperMover.entitlements"
  codesign --verify --strict "$BUNDLE_BIN_DIR/supermover"
  codesign --verify --strict "$BUNDLE_BIN_DIR/supermover-app-audit"
  codesign --verify --deep --strict "$APP_DIR"
  printf 'Signed bundled CLI and %s with identity %s\n' "$APP_DIR" "$CODESIGN_IDENTITY"
else
  printf 'Built unsigned local app. Set SUPERMOVER_CODESIGN_IDENTITY to sign.\n'
fi

printf 'Built %s\n' "$APP_DIR"
