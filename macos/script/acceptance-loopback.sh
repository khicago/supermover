#!/bin/sh
set -eu
export LC_ALL=C
export LANG=C

ROOT_DIR=$(CDPATH= cd "$(dirname "$0")/../.." && pwd)
APP_DIR="$ROOT_DIR/macos/dist/SuperMover.app"
SM="$APP_DIR/Contents/Resources/bin/supermover"
. "$ROOT_DIR/macos/script/lib/acceptance-common.sh"
. "$ROOT_DIR/macos/script/lib/acceptance-network.sh"

if ! command -v jq >/dev/null 2>&1; then
  printf 'jq is required for acceptance checks\n' >&2
  exit 1
fi

TMP_PARENT="${TMPDIR:-$ROOT_DIR/.tmp}"
mkdir -p "$TMP_PARENT"
WORK_DIR=$(mktemp -d "$TMP_PARENT/supermover-loopback-XXXXXX")
SOURCE_DIR="$WORK_DIR/source"
TARGET_DIR="$WORK_DIR/target"
PROFILE="$WORK_DIR/profile.json"
NETWORK_SOURCE_DIR="$WORK_DIR/network-source"
NETWORK_TARGET_DIR="$WORK_DIR/network-target"
NETWORK_SOURCE_PROFILE="$WORK_DIR/network-source.profile.json"
NETWORK_TARGET_PROFILE="$WORK_DIR/network-target.profile.json"
NETWORK_TLS_DIR="$WORK_DIR/tls"
SESSION_ID="loopback-001"
SYNC_SESSION_ID="loopback-sync-001"
NETWORK_SESSION_ID="loopback-network-001"
EXPECTED_FILES=5

start_target_serve() {
  profile_path=$1
  log_prefix=$2
  readiness_json=$3
  require_receiver=${4:-0}
  "$SM" serve --profile "$profile_path" --listen 127.0.0.1:0 --ready-file "$readiness_json" > "$WORK_DIR/$log_prefix.stdout" 2> "$WORK_DIR/$log_prefix.stderr" &
  serve_pid=$!
  echo "$serve_pid" > "$WORK_DIR/$log_prefix.pid"
  if ! acceptance_wait_for_serve_ready "$readiness_json" 10 1 "$require_receiver"; then
    printf 'serve readiness timeout: %s\n' "$readiness_json" >&2
    acceptance_cleanup_pid "$serve_pid"
    exit 1
  fi
  tmp_json="$readiness_json.tmp"
  jq -n \
    --arg pid "$serve_pid" \
    --slurpfile ready "$readiness_json" \
    '$ready[0] + {pid: ($pid|tonumber)}' > "$tmp_json"
  mv "$tmp_json" "$readiness_json"
}

on_exit() {
  status=$?
  if [ "$status" -ne 0 ]; then
    printf 'loopback acceptance failed (exit %s): %s\n' "$status" "$WORK_DIR" >&2
    jq -n \
      --arg work_dir "$WORK_DIR" \
      --arg profile "$PROFILE" \
      --arg target "$TARGET_DIR" \
      --arg exit_code "$status" \
      '{
        schema: "supermover.acceptance.loopback.v1",
        status: "fail",
        work_dir: $work_dir,
        profile: $profile,
        target: $target,
        exit_code: ($exit_code | tonumber)
      }' > "$WORK_DIR/summary.json" 2>/dev/null || true
  fi
}
trap on_exit EXIT

if [ "${SUPERMOVER_ACCEPTANCE_SKIP_BUILD:-}" != "1" ]; then
  "$ROOT_DIR/macos/script/build-app.sh"
fi

if [ ! -x "$SM" ]; then
  printf 'missing bundled CLI: %s\n' "$SM" >&2
  exit 1
fi
acceptance_preflight_cli_surface "$SM" "$APP_DIR" "$WORK_DIR" "loopback-serve" "serve --help" "-ready-file string" serve --help

mkdir -p "$SOURCE_DIR/.hidden" "$SOURCE_DIR/dir" "$TARGET_DIR" "$NETWORK_SOURCE_DIR" "$NETWORK_TARGET_DIR" "$NETWORK_TLS_DIR"
printf 'hello\n' > "$SOURCE_DIR/file.txt"
: > "$SOURCE_DIR/zero.bin"
printf 'dot file\n' > "$SOURCE_DIR/.hidden-file"
printf 'secret\n' > "$SOURCE_DIR/.hidden/secret.txt"
dd if=/dev/zero of="$SOURCE_DIR/dir/blob.bin" bs=1024 count=64 >/dev/null 2>&1
printf 'network secret\n' > "$NETWORK_SOURCE_DIR/.hidden-network.txt"
printf 'network payload\n' > "$NETWORK_SOURCE_DIR/data.txt"
: > "$NETWORK_SOURCE_DIR/empty.txt"

"$SM" version > "$WORK_DIR/version.txt"
CLI_VERSION=$(cat "$WORK_DIR/version.txt")
EXPECTED_GIT_COMMIT=${SUPERMOVER_ACCEPTANCE_EXPECT_GIT_COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD)}
jq -e '
  if (
    .schema == "supermover.macos.provenance.v1"
    and (.app_bundle_id | type == "string" and length > 0)
    and (.app_version | type == "string" and length > 0)
    and (.build_profile | type == "string" and length > 0)
    and .git_commit == $expected_git_commit
    and (.git_dirty | type == "boolean")
    and .cli_version == $expected_version
    and .cli_relative_path == $expected_path
    and (.built_at | type == "string" and length > 0)
    and (.signing | type == "string" and length > 0)
  ) then . else error("incomplete packaged provenance") end
' \
  --arg expected_version "$CLI_VERSION" \
  --arg expected_path "Contents/Resources/bin/supermover" \
  --arg expected_git_commit "$EXPECTED_GIT_COMMIT" \
  "$APP_DIR/Contents/Resources/supermover-provenance.json" > "$WORK_DIR/provenance.json"
set +e
"$ROOT_DIR/macos/script/audit-app.sh" "$APP_DIR" > "$WORK_DIR/app-audit.json"
APP_AUDIT_EXIT=$?
set -e
APP_AUDIT_STATUS=$(jq -r '.status // "blocked"' "$WORK_DIR/app-audit.json")
APP_AUDIT_BLOCKING_CHECKS=$(jq -r '.summary.blocking_checks // 0' "$WORK_DIR/app-audit.json")
PROVENANCE_READINESS=$(jq -r 'if .status == "pass" then "distribution_ready" else "local_review_only" end' "$WORK_DIR/app-audit.json")

"$SM" profile init --profile "$PROFILE" --source "$SOURCE_DIR" --target "$TARGET_DIR" --id loopback-local --name "Loopback Local" > "$WORK_DIR/profile-init.txt"
"$SM" profile lint --profile "$PROFILE" > "$WORK_DIR/profile-lint.txt"
"$SM" push --profile "$PROFILE" --dry-run > "$WORK_DIR/push-dry-run.txt"
"$SM" push --profile "$PROFILE" --session "$SESSION_ID" > "$WORK_DIR/push.txt"
"$SM" verify --profile "$PROFILE" --session "$SESSION_ID" --format json > "$WORK_DIR/verify.json"
"$SM" status --profile "$PROFILE" --format json > "$WORK_DIR/status.json"
"$SM" report --profile "$PROFILE" --session "$SESSION_ID" --format json > "$WORK_DIR/report.json"
"$SM" health --profile "$PROFILE" --format json > "$WORK_DIR/health.json"

jq -e --argjson expected "$EXPECTED_FILES" '.summary.files_verified == $expected and .summary.error_findings == 0 and .summary.warning_findings == 0 and .summary.target_drifts == 0 and .summary.artifact_problems == 0' "$WORK_DIR/verify.json" >/dev/null
jq -e '.overall.status == "clean" and .overall.target_status == "local_target_verified"' "$WORK_DIR/status.json" >/dev/null
jq -e '.overall.status == "local_target_verified" and .summary.artifact_problems == 0' "$WORK_DIR/report.json" >/dev/null
jq -e '.healthy == true and .summary.artifact_problems == 0 and .summary.target_drifts == 0' "$WORK_DIR/health.json" >/dev/null

test -f "$TARGET_DIR/file.txt"
test -f "$TARGET_DIR/zero.bin"
test -f "$TARGET_DIR/.hidden-file"
test -f "$TARGET_DIR/.hidden/secret.txt"
test -f "$TARGET_DIR/dir/blob.bin"

printf 'new\n' > "$SOURCE_DIR/new.txt"
"$SM" sync queue enqueue --profile "$PROFILE" --format json > "$WORK_DIR/sync-queue-enqueue.json"
"$SM" sync queue list --profile "$PROFILE" --format json > "$WORK_DIR/sync-queue-list.json"
"$SM" sync queue ready --profile "$PROFILE" --format json > "$WORK_DIR/sync-queue-ready.json"
jq -e '.summary.ready > 0 and .summary.total > 0' "$WORK_DIR/sync-queue-ready.json" >/dev/null
"$SM" sync run --profile "$PROFILE" --session "$SYNC_SESSION_ID" --format json > "$WORK_DIR/sync-run.json"
jq -e '.run.status == "published" and (.run.published | length) > 0' "$WORK_DIR/sync-run.json" >/dev/null
test -f "$TARGET_DIR/new.txt"

find "$TARGET_DIR" -type f -print | sort | while IFS= read -r file; do
  shasum -a 256 "$file"
done > "$WORK_DIR/target-before-network-negative.sha256"

set +e
"$SM" push --network --profile "$PROFILE" --dry-run > "$WORK_DIR/network-unpaired.out" 2> "$WORK_DIR/network-unpaired.err"
NETWORK_EXIT=$?
set -e
if [ "$NETWORK_EXIT" -eq 0 ]; then
  printf 'expected unpaired push --network dry-run to fail closed\n' >&2
  exit 1
fi
if ! grep -q 'profile is not paired' "$WORK_DIR/network-unpaired.err"; then
  printf 'unexpected unpaired network refusal:\n' >&2
  cat "$WORK_DIR/network-unpaired.err" >&2
  exit 1
fi
find "$TARGET_DIR" -type f -print | sort | while IFS= read -r file; do
  shasum -a 256 "$file"
done > "$WORK_DIR/target-after-network-negative.sha256"
diff -u "$WORK_DIR/target-before-network-negative.sha256" "$WORK_DIR/target-after-network-negative.sha256" > "$WORK_DIR/network-negative-target-diff.txt"

run_network_loopback_acceptance

jq -n \
  --arg work_dir "$WORK_DIR" \
  --arg profile "$PROFILE" \
  --arg target "$TARGET_DIR" \
  --arg session_id "$SESSION_ID" \
  --arg sync_session_id "$SYNC_SESSION_ID" \
  --arg network_source_profile "$NETWORK_SOURCE_PROFILE" \
  --arg network_target_profile "$NETWORK_TARGET_PROFILE" \
  --arg network_target "$NETWORK_TARGET_DIR" \
  --arg network_session_id "$NETWORK_SESSION_ID" \
  --arg pairing_receipt_id "$PAIRING_RECEIPT_ID" \
  --arg provenance_readiness "$PROVENANCE_READINESS" \
  --arg app_audit_status "$APP_AUDIT_STATUS" \
  --argjson app_audit_exit_code "$APP_AUDIT_EXIT" \
  --argjson app_audit_blocking_checks "$APP_AUDIT_BLOCKING_CHECKS" \
  '{
    schema: "supermover.acceptance.loopback.v1",
    status: "pass",
    work_dir: $work_dir,
    profile: $profile,
    target: $target,
    session_id: $session_id,
    sync_session_id: $sync_session_id,
    network_source_profile: $network_source_profile,
    network_target_profile: $network_target_profile,
    network_target: $network_target,
    network_session_id: $network_session_id,
    pairing_receipt_id: $pairing_receipt_id,
    provenance_readiness: $provenance_readiness,
    app_audit_status: $app_audit_status,
    app_audit_exit_code: $app_audit_exit_code,
    app_audit_blocking_checks: $app_audit_blocking_checks,
    negative_checks: ["unpaired push --network dry-run fails closed"],
    network_loopback: {
      status: "pass",
      transfer: "tls13_mtls",
      paired_target_profile: true,
      network_transfer_artifact: "present"
    }
  }' > "$WORK_DIR/summary.json"

printf 'loopback acceptance passed: %s\n' "$WORK_DIR"
