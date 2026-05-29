#!/bin/sh
set -eu
export LC_ALL=C
export LANG=C

ROOT_DIR=$(CDPATH= cd "$(dirname "$0")/../.." && pwd)
APP_DIR="${SUPERMOVER_ACCEPTANCE_APP_DIR:-$ROOT_DIR/macos/dist/SuperMover.app}"
SOURCE_APP_DIR="${SUPERMOVER_ACCEPTANCE_SOURCE_APP_DIR:-$APP_DIR}"
TARGET_APP_DIR="${SUPERMOVER_ACCEPTANCE_TARGET_APP_DIR:-$APP_DIR}"
SOURCE_SM="$SOURCE_APP_DIR/Contents/Resources/bin/supermover"
TARGET_SM="$TARGET_APP_DIR/Contents/Resources/bin/supermover"
. "$ROOT_DIR/macos/script/lib/acceptance-common.sh"

export SUPERMOVER_ACCEPTANCE_COLLECTION_MODE="same_machine"
export SUPERMOVER_ACCEPTANCE_MACHINE_COUNT="1"
export SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID="same-machine"
export SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL="same-machine source"
export SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID="same-machine"
export SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL="same-machine target"

usage() {
  cat <<'EOF'
Usage:
  macos/script/acceptance-two-machine-same-machine.sh

Runs the installed-app five-phase two-machine harness locally on one machine:
  1. target-serve (pairing-only)
  2. source-pair
  3. target-import
  4. target-serve (paired receiver)
  5. source-transfer
  6. evaluate

This is packaged-app wiring evidence only. It is not real two-machine LAN,
Local Network/firewall prompt, or notarized distribution evidence.
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ] || [ "${1:-}" = "help" ]; then
  usage
  exit 0
fi
if [ "$#" -ne 0 ]; then
  usage >&2
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  printf 'jq is required for acceptance checks\n' >&2
  exit 1
fi

if [ "${SUPERMOVER_ACCEPTANCE_SKIP_BUILD:-}" != "1" ]; then
  "$ROOT_DIR/macos/script/build-app.sh"
fi

if [ ! -x "$SOURCE_SM" ]; then
  printf 'missing source bundled CLI: %s\n' "$SOURCE_SM" >&2
  exit 1
fi

if [ ! -x "$TARGET_SM" ]; then
  printf 'missing target bundled CLI: %s\n' "$TARGET_SM" >&2
  exit 1
fi

TMP_PARENT="${TMPDIR:-$ROOT_DIR/.tmp}"
mkdir -p "$TMP_PARENT"
WORK_DIR=$(mktemp -d "$TMP_PARENT/supermover-two-machine-same-machine-XXXXXX")
TARGET_BUNDLE_ROOT="$WORK_DIR/target-bundle"
SOURCE_BUNDLE_ROOT="$WORK_DIR/source-bundle"
BUNDLE_ROOT="$WORK_DIR/bundle"
SOURCE_ROOT="$WORK_DIR/source-root"
TARGET_ROOT="$WORK_DIR/target-root"
SOURCE_PROFILE="$WORK_DIR/source.profile.json"
TARGET_PROFILE="$WORK_DIR/target.profile.json"
TLS_DIR="$WORK_DIR/tls"
SESSION_ID="same-machine-five-phase-001"
PROFILE_ID="same-machine-migration"
PROFILE_NAME="Same Machine Migration"
USE_ARCHIVE_HANDOFF=${SUPERMOVER_ACCEPTANCE_USE_ARCHIVE_HANDOFF:-0}

cleanup_pid_file() {
  pid_file=$1
  if [ -f "$pid_file" ]; then
    acceptance_cleanup_pid "$(cat "$pid_file")"
  fi
}

WAIT_FOR_READY_RUNNER_STATUS=0

wait_for_recorded_target_phase_or_runner() {
  runner_pid=$1
  bundle_root=$2
  phase_number=$3
  timeout_seconds=$4
  require_verification=$5
  require_receiver=$6
  stderr_path=$7
  ready_file="$bundle_root/target.ready.phase-$phase_number.json"

  i=0
  while [ "$i" -lt "$timeout_seconds" ]; do
    if acceptance_wait_for_serve_ready "$ready_file" 1 "$require_verification" "$require_receiver" \
      && jq -e --argjson phase "$phase_number" '
        ((.evidence.target_serve_phases // []) | any((.phase // 0) == $phase))
        and ((.evidence.target_ready.mode // "") | length > 0)
      ' "$bundle_root/meta.json" >/dev/null 2>&1; then
      WAIT_FOR_READY_RUNNER_STATUS=0
      return 0
    fi
    if ! kill -0 "$runner_pid" 2>/dev/null; then
      set +e
      wait "$runner_pid"
      WAIT_FOR_READY_RUNNER_STATUS=$?
      set -e
      printf 'target serve exited before readiness was recorded: %s\n' "$stderr_path" >&2
      if [ -f "$stderr_path" ]; then
        cat "$stderr_path" >&2
      fi
      return 1
    fi
    i=$((i + 1))
  done
  WAIT_FOR_READY_RUNNER_STATUS=1
  return 1
}

handoff_bundle() {
  archive_path=$1
  destination_bundle_root=$2
  incoming_bundle_root=$3
  exporting_machine_id=$4
  exporting_machine_label=$5
  importing_machine_id=$6
  importing_machine_label=$7
  archive_dir=$(dirname "$archive_path")
  archive_name=$(basename "$archive_path")
  archive_stem=${archive_name%.tgz}
  if [ "$archive_stem" = "$archive_name" ]; then
    archive_stem=${archive_name%.tar.gz}
  fi
  if [ "$archive_stem" = "$archive_name" ]; then
    archive_stem=${archive_name}
  fi
  manifest_path="$archive_dir/$archive_stem.manifest.json"
  env \
    SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID="$exporting_machine_id" \
    SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL="$exporting_machine_label" \
    sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" pack-bundle --bundle-root "$incoming_bundle_root" --archive "$archive_path"
  env \
    SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_ID="$importing_machine_id" \
    SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_LABEL="$importing_machine_label" \
    sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" unpack-bundle --archive "$archive_path" --manifest "$manifest_path" --bundle-root "$destination_bundle_root.unpacked"
  sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" merge-bundle --bundle-root "$destination_bundle_root" --incoming-bundle-root "$destination_bundle_root.unpacked"
}

on_exit() {
  status=$?
  cleanup_pid_file "$TARGET_BUNDLE_ROOT/target.serve.phase-1.pid"
  cleanup_pid_file "$TARGET_BUNDLE_ROOT/target.serve.phase-2.pid"
  if [ "$status" -ne 0 ]; then
    printf 'same-machine two-machine acceptance failed (exit %s): %s\n' "$status" "$WORK_DIR" >&2
  fi
}
trap on_exit EXIT

mkdir -p "$TARGET_BUNDLE_ROOT" "$SOURCE_BUNDLE_ROOT" "$BUNDLE_ROOT" "$SOURCE_ROOT/.hidden-dir" "$TARGET_ROOT" "$TLS_DIR"
printf 'payload\n' > "$SOURCE_ROOT/data.txt"
printf 'hidden\n' > "$SOURCE_ROOT/.hidden-file"
printf 'nested\n' > "$SOURCE_ROOT/.hidden-dir/nested.txt"
: > "$SOURCE_ROOT/empty.bin"

SOURCE_CERT="$TLS_DIR/source.crt"
SOURCE_KEY="$TLS_DIR/source.key"
TARGET_CERT="$TLS_DIR/target.crt"
TARGET_KEY="$TLS_DIR/target.key"
acceptance_write_tls_identity source "$SOURCE_CERT" "$SOURCE_KEY"
acceptance_write_tls_identity target "$TARGET_CERT" "$TARGET_KEY"
TARGET_RECEIVER_PORT=$(acceptance_reserve_local_port)
DISCOVERY_PORT=$(acceptance_reserve_local_port)

"$SOURCE_SM" profile init --profile "$SOURCE_PROFILE" --source "$SOURCE_ROOT" --target "$TARGET_ROOT" --id "$PROFILE_ID" --name "$PROFILE_NAME" > "$WORK_DIR/source.profile-init.txt"
"$TARGET_SM" profile init --profile "$TARGET_PROFILE" --source "$SOURCE_ROOT" --target "$TARGET_ROOT" --id "$PROFILE_ID" --name "$PROFILE_NAME" > "$WORK_DIR/target.profile-init.txt"

python3 - "$SOURCE_PROFILE" "$SOURCE_CERT" "$SOURCE_KEY" "$TARGET_RECEIVER_PORT" <<'PY'
import json, sys
path, cert_path, key_path, port = sys.argv[1:]
with open(path) as f:
    doc = json.load(f)
doc["network"] = {
    "receiver_url": f"https://127.0.0.1:{port}",
    "local_tls_identity": {
        "certificate_path": cert_path,
        "private_key_path": key_path,
    },
}
with open(path, "w") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
PY

python3 - "$TARGET_PROFILE" "$TARGET_CERT" "$TARGET_KEY" "$TARGET_RECEIVER_PORT" <<'PY'
import json, sys
path, cert_path, key_path, port = sys.argv[1:]
with open(path) as f:
    doc = json.load(f)
doc["network"] = {
    "receiver_url": f"https://127.0.0.1:{port}",
    "local_tls_identity": {
        "certificate_path": cert_path,
        "private_key_path": key_path,
    },
}
with open(path, "w") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
PY

sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" target-serve --profile "$TARGET_PROFILE" --bundle-root "$TARGET_BUNDLE_ROOT" > "$WORK_DIR/target-serve-1.out" 2> "$WORK_DIR/target-serve-1.err" &
RUNNER_ONE=$!
if ! wait_for_recorded_target_phase_or_runner "$RUNNER_ONE" "$TARGET_BUNDLE_ROOT" 1 15 1 0 "$WORK_DIR/target-serve-1.err"; then
  printf 'target pairing serve phase recording failed: %s\n' "$TARGET_BUNDLE_ROOT/target.ready.phase-1.json" >&2
  exit "${WAIT_FOR_READY_RUNNER_STATUS:-1}"
fi

sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" source-browse --bundle-root "$SOURCE_BUNDLE_ROOT" --listen "127.0.0.1:$DISCOVERY_PORT" --timeout 600ms > "$WORK_DIR/source-browse.out" 2> "$WORK_DIR/source-browse.err" &
BROWSE_RUNNER=$!
sleep 0.1
sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" target-advertise --profile "$TARGET_PROFILE" --bundle-root "$TARGET_BUNDLE_ROOT" --dest "127.0.0.1:$DISCOVERY_PORT" --duration 300ms --interval 50ms > "$WORK_DIR/target-advertise.out" 2> "$WORK_DIR/target-advertise.err"
wait "$BROWSE_RUNNER"
if [ "$USE_ARCHIVE_HANDOFF" = "1" ]; then
  handoff_bundle "$WORK_DIR/target-to-final-phase-1.tgz" "$BUNDLE_ROOT" "$TARGET_BUNDLE_ROOT" \
    "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL" \
    "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL" \
    > "$WORK_DIR/merge-target-1.out" 2> "$WORK_DIR/merge-target-1.err"
  handoff_bundle "$WORK_DIR/source-to-final-phase-1.tgz" "$BUNDLE_ROOT" "$SOURCE_BUNDLE_ROOT" \
    "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL" \
    "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL" \
    > "$WORK_DIR/merge-source-1.out" 2> "$WORK_DIR/merge-source-1.err"
else
  sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" merge-bundle --bundle-root "$BUNDLE_ROOT" --incoming-bundle-root "$TARGET_BUNDLE_ROOT" > "$WORK_DIR/merge-target-1.out" 2> "$WORK_DIR/merge-target-1.err"
  sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" merge-bundle --bundle-root "$BUNDLE_ROOT" --incoming-bundle-root "$SOURCE_BUNDLE_ROOT" > "$WORK_DIR/merge-source-1.out" 2> "$WORK_DIR/merge-source-1.err"
fi
jq -e '.status == "advertised" and .trusted == false' "$BUNDLE_ROOT/target.advertise.json" >/dev/null
jq -e '.trusted == false and (.candidate_count | type == "number") and (.candidates | type == "array")' "$BUNDLE_ROOT/source.browse.json" >/dev/null

if [ "$USE_ARCHIVE_HANDOFF" = "1" ]; then
  handoff_bundle "$WORK_DIR/target-to-source-phase-1.tgz" "$SOURCE_BUNDLE_ROOT" "$TARGET_BUNDLE_ROOT" \
    "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL" \
    "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL" \
    > "$WORK_DIR/merge-target-to-source-phase-1.out" 2> "$WORK_DIR/merge-target-to-source-phase-1.err"
else
  sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" merge-bundle --bundle-root "$SOURCE_BUNDLE_ROOT" --incoming-bundle-root "$TARGET_BUNDLE_ROOT" > "$WORK_DIR/merge-target-to-source-phase-1.out" 2> "$WORK_DIR/merge-target-to-source-phase-1.err"
fi

sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" source-pair --profile "$SOURCE_PROFILE" --bundle-root "$SOURCE_BUNDLE_ROOT" > "$WORK_DIR/source-pair.out" 2> "$WORK_DIR/source-pair.err"
if [ ! -f "$SOURCE_BUNDLE_ROOT/source.pair.json" ]; then
  printf 'source pairing did not write source-local receipt evidence: %s\n' "$SOURCE_BUNDLE_ROOT/source.pair.json" >&2
  exit 1
fi
if [ -f "$BUNDLE_ROOT/source.pair.json" ]; then
  printf 'source pairing must not write directly into final aggregate bundle: %s\n' "$BUNDLE_ROOT/source.pair.json" >&2
  exit 1
fi
if [ "$USE_ARCHIVE_HANDOFF" = "1" ]; then
  handoff_bundle "$WORK_DIR/source-to-target-pairing.tgz" "$TARGET_BUNDLE_ROOT" "$SOURCE_BUNDLE_ROOT" \
    "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL" \
    "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL" \
    > "$WORK_DIR/merge-pairing.out" 2> "$WORK_DIR/merge-pairing.err"
else
  sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" merge-bundle --bundle-root "$TARGET_BUNDLE_ROOT" --incoming-bundle-root "$SOURCE_BUNDLE_ROOT" > "$WORK_DIR/merge-pairing.out" 2> "$WORK_DIR/merge-pairing.err"
fi
sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" target-import --profile "$TARGET_PROFILE" --bundle-root "$TARGET_BUNDLE_ROOT" > "$WORK_DIR/target-import.out" 2> "$WORK_DIR/target-import.err"
if [ "$USE_ARCHIVE_HANDOFF" = "1" ]; then
  handoff_bundle "$WORK_DIR/target-to-final-import.tgz" "$BUNDLE_ROOT" "$TARGET_BUNDLE_ROOT" \
    "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL" \
    "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL" \
    > "$WORK_DIR/merge-target-import.out" 2> "$WORK_DIR/merge-target-import.err"
else
  sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" merge-bundle --bundle-root "$BUNDLE_ROOT" --incoming-bundle-root "$TARGET_BUNDLE_ROOT" > "$WORK_DIR/merge-target-import.out" 2> "$WORK_DIR/merge-target-import.err"
fi

cleanup_pid_file "$TARGET_BUNDLE_ROOT/target.serve.phase-1.pid"

sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" target-serve --profile "$TARGET_PROFILE" --bundle-root "$TARGET_BUNDLE_ROOT" > "$WORK_DIR/target-serve-2.out" 2> "$WORK_DIR/target-serve-2.err" &
RUNNER_TWO=$!
if ! wait_for_recorded_target_phase_or_runner "$RUNNER_TWO" "$TARGET_BUNDLE_ROOT" 2 15 1 1 "$WORK_DIR/target-serve-2.err"; then
  printf 'target receiver serve phase recording failed: %s\n' "$TARGET_BUNDLE_ROOT/target.ready.phase-2.json" >&2
  exit "${WAIT_FOR_READY_RUNNER_STATUS:-1}"
fi

if [ "$USE_ARCHIVE_HANDOFF" = "1" ]; then
  handoff_bundle "$WORK_DIR/target-to-final-ready.tgz" "$BUNDLE_ROOT" "$TARGET_BUNDLE_ROOT" \
    "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL" \
    "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL" \
    > "$WORK_DIR/merge-target-ready-2.out" 2> "$WORK_DIR/merge-target-ready-2.err"
  handoff_bundle "$WORK_DIR/target-to-source-ready.tgz" "$SOURCE_BUNDLE_ROOT" "$TARGET_BUNDLE_ROOT" \
    "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL" \
    "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL" \
    > "$WORK_DIR/merge-target-2.out" 2> "$WORK_DIR/merge-target-2.err"
else
  sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" merge-bundle --bundle-root "$BUNDLE_ROOT" --incoming-bundle-root "$TARGET_BUNDLE_ROOT" > "$WORK_DIR/merge-target-ready-2.out" 2> "$WORK_DIR/merge-target-ready-2.err"
  sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" merge-bundle --bundle-root "$SOURCE_BUNDLE_ROOT" --incoming-bundle-root "$TARGET_BUNDLE_ROOT" > "$WORK_DIR/merge-target-2.out" 2> "$WORK_DIR/merge-target-2.err"
fi
sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" source-transfer --profile "$SOURCE_PROFILE" --bundle-root "$SOURCE_BUNDLE_ROOT" --session "$SESSION_ID" > "$WORK_DIR/source-transfer.out" 2> "$WORK_DIR/source-transfer.err"
if [ "$USE_ARCHIVE_HANDOFF" = "1" ]; then
  handoff_bundle "$WORK_DIR/source-to-final-transfer.tgz" "$BUNDLE_ROOT" "$SOURCE_BUNDLE_ROOT" \
    "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL" \
    "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID" "$SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL" \
    > "$WORK_DIR/merge-source-transfer.out" 2> "$WORK_DIR/merge-source-transfer.err"
else
  sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" merge-bundle --bundle-root "$BUNDLE_ROOT" --incoming-bundle-root "$SOURCE_BUNDLE_ROOT" > "$WORK_DIR/merge-source-transfer.out" 2> "$WORK_DIR/merge-source-transfer.err"
fi
sh "$ROOT_DIR/macos/script/acceptance-two-machine.sh" evaluate --bundle-root "$BUNDLE_ROOT" --target-root "$TARGET_ROOT" --source-profile "$SOURCE_PROFILE" > "$WORK_DIR/evaluate.out" 2> "$WORK_DIR/evaluate.err"

cleanup_pid_file "$TARGET_BUNDLE_ROOT/target.serve.phase-2.pid"

jq -e '.status == "evidence_collected"' "$BUNDLE_ROOT/meta.json" >/dev/null
jq -e '.status == "evidence_collected"' "$BUNDLE_ROOT/evaluation.json" >/dev/null
jq -e '.evidence.app_audit.source.output == "source.app-audit.json" and .evidence.app_audit.target.output == "target.app-audit.json"' "$BUNDLE_ROOT/meta.json" >/dev/null
jq -e '.collection.mode == "same_machine" and .collection.machine_count == 1' "$BUNDLE_ROOT/meta.json" >/dev/null
jq -e '.roles.source_pair.machine_id == "same-machine" and .roles.target.machine_id == "same-machine"' "$BUNDLE_ROOT/meta.json" >/dev/null
jq -e '.evidence.discovery.target_advertise.output == "target.advertise.json" and .evidence.discovery.source_browse.output == "source.browse.json"' "$BUNDLE_ROOT/meta.json" >/dev/null
jq -e '.evidence.source_pair.pairing_receipt_id != "" and .evidence.source_transfer.session_id == "'"$SESSION_ID"'"' "$BUNDLE_ROOT/meta.json" >/dev/null
jq -e '.trusted == false and (.candidate_count | type == "number") and (.candidates | type == "array")' "$BUNDLE_ROOT/source.browse.json" >/dev/null
jq -e '.status == "advertised" and .trusted == false' "$BUNDLE_ROOT/target.advertise.json" >/dev/null
jq -e '.mode == "pairing-only"' "$BUNDLE_ROOT/target.ready.phase-1.json" >/dev/null
jq -e '.mode == "pairing" and .receiver_routes == true and .push_network == true' "$BUNDLE_ROOT/target.ready.phase-2.json" >/dev/null

printf 'same-machine two-machine acceptance passed: %s\n' "$WORK_DIR"
