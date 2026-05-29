#!/bin/sh
set -eu
export LC_ALL=C
export LANG=C

ROOT_DIR=$(CDPATH= cd "$(dirname "$0")/../.." && pwd)
APP_DIR="${SUPERMOVER_ACCEPTANCE_APP_DIR:-$ROOT_DIR/macos/dist/SuperMover.app}"
. "$ROOT_DIR/macos/script/lib/acceptance-common.sh"
. "$ROOT_DIR/macos/script/lib/acceptance-two-machine.sh"

acceptance_source_app_dir() {
  printf '%s\n' "${SUPERMOVER_ACCEPTANCE_SOURCE_APP_DIR:-$APP_DIR}"
}

acceptance_target_app_dir() {
  printf '%s\n' "${SUPERMOVER_ACCEPTANCE_TARGET_APP_DIR:-$APP_DIR}"
}

acceptance_sm_bin_for_app_dir() {
  app_dir=$1
  printf '%s\n' "$app_dir/Contents/Resources/bin/supermover"
}

role=${1:-}
shift || true

case "$role" in
  target-serve)
    phase_app_dir=$(acceptance_target_app_dir)
    phase_sm=$(acceptance_sm_bin_for_app_dir "$phase_app_dir")
    profile_path=""
    bundle_root=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --profile) profile_path=$2; shift 2 ;;
        --bundle-root) bundle_root=$2; shift 2 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$profile_path" "--profile"
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_two_machine_target_serve "$phase_sm" "$phase_app_dir" "$profile_path" "$bundle_root"
    ;;
  target-advertise)
    phase_app_dir=$(acceptance_target_app_dir)
    phase_sm=$(acceptance_sm_bin_for_app_dir "$phase_app_dir")
    profile_path=""
    bundle_root=""
    listen=""
    dest=""
    duration=""
    interval=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --profile) profile_path=$2; shift 2 ;;
        --bundle-root) bundle_root=$2; shift 2 ;;
        --listen) listen=$2; shift 2 ;;
        --dest) dest=$2; shift 2 ;;
        --duration) duration=$2; shift 2 ;;
        --interval) interval=$2; shift 2 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$profile_path" "--profile"
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_two_machine_target_advertise "$phase_sm" "$phase_app_dir" "$profile_path" "$bundle_root" "$listen" "$dest" "$duration" "$interval"
    ;;
  source-browse)
    phase_app_dir=$(acceptance_source_app_dir)
    phase_sm=$(acceptance_sm_bin_for_app_dir "$phase_app_dir")
    bundle_root=""
    listen=""
    timeout=""
    strict=0
    while [ $# -gt 0 ]; do
      case "$1" in
        --bundle-root) bundle_root=$2; shift 2 ;;
        --listen) listen=$2; shift 2 ;;
        --timeout) timeout=$2; shift 2 ;;
        --strict) strict=1; shift 1 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_two_machine_source_browse "$phase_sm" "$phase_app_dir" "$bundle_root" "$listen" "$timeout" "$strict"
    ;;
  source-pair)
    phase_app_dir=$(acceptance_source_app_dir)
    phase_sm=$(acceptance_sm_bin_for_app_dir "$phase_app_dir")
    profile_path=""
    bundle_root=""
    target_address=""
    verification_code=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --profile) profile_path=$2; shift 2 ;;
        --bundle-root) bundle_root=$2; shift 2 ;;
        --target-address) target_address=$2; shift 2 ;;
        --verification-code) verification_code=$2; shift 2 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$profile_path" "--profile"
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_two_machine_source_pair "$phase_sm" "$phase_app_dir" "$profile_path" "$bundle_root" "$target_address" "$verification_code"
    ;;
  target-import)
    phase_app_dir=$(acceptance_target_app_dir)
    phase_sm=$(acceptance_sm_bin_for_app_dir "$phase_app_dir")
    profile_path=""
    bundle_root=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --profile) profile_path=$2; shift 2 ;;
        --bundle-root) bundle_root=$2; shift 2 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$profile_path" "--profile"
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_two_machine_target_import "$phase_sm" "$phase_app_dir" "$profile_path" "$bundle_root"
    ;;
  source-transfer)
    phase_app_dir=$(acceptance_source_app_dir)
    phase_sm=$(acceptance_sm_bin_for_app_dir "$phase_app_dir")
    profile_path=""
    bundle_root=""
    session_id=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --profile) profile_path=$2; shift 2 ;;
        --bundle-root) bundle_root=$2; shift 2 ;;
        --session) session_id=$2; shift 2 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$profile_path" "--profile"
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_require_flag "$session_id" "--session"
    acceptance_two_machine_source_transfer "$phase_sm" "$phase_app_dir" "$profile_path" "$bundle_root" "$session_id"
    ;;
  record-packaging-evidence)
    bundle_root=""
    machine=""
    app_dir=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --bundle-root) bundle_root=$2; shift 2 ;;
        --machine) machine=$2; shift 2 ;;
        --app) app_dir=$2; shift 2 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_require_flag "$machine" "--machine"
    acceptance_require_flag "$app_dir" "--app"
    acceptance_two_machine_record_packaging_evidence "$bundle_root" "$machine" "$app_dir"
    ;;
  merge-bundle)
    bundle_root=""
    incoming_bundle_root=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --bundle-root) bundle_root=$2; shift 2 ;;
        --incoming-bundle-root) incoming_bundle_root=$2; shift 2 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_require_flag "$incoming_bundle_root" "--incoming-bundle-root"
    acceptance_two_machine_merge_bundle "$bundle_root" "$incoming_bundle_root"
    ;;
  pack-bundle)
    bundle_root=""
    archive_path=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --bundle-root) bundle_root=$2; shift 2 ;;
        --archive) archive_path=$2; shift 2 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_require_flag "$archive_path" "--archive"
    acceptance_two_machine_pack_bundle "$bundle_root" "$archive_path"
    ;;
  unpack-bundle)
    archive_path=""
    manifest_path=""
    bundle_root=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --archive) archive_path=$2; shift 2 ;;
        --manifest) manifest_path=$2; shift 2 ;;
        --bundle-root) bundle_root=$2; shift 2 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$archive_path" "--archive"
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_two_machine_unpack_bundle "$archive_path" "$manifest_path" "$bundle_root"
    ;;
  workflow-status)
    bundle_root=""
    require_operator_evidence=0
    while [ $# -gt 0 ]; do
      case "$1" in
        --bundle-root) bundle_root=$2; shift 2 ;;
        --require-operator-evidence) require_operator_evidence=1; shift 1 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_two_machine_workflow_status "$bundle_root" "$require_operator_evidence"
    ;;
  record-operator-evidence)
    bundle_root=""
    kind=""
    status=""
    detail=""
    artifact=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --bundle-root) bundle_root=$2; shift 2 ;;
        --kind) kind=$2; shift 2 ;;
        --status) status=$2; shift 2 ;;
        --detail) detail=$2; shift 2 ;;
        --artifact) artifact=$2; shift 2 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_two_machine_record_operator_evidence "$bundle_root" "$kind" "$status" "$detail" "$artifact"
    ;;
  evaluate)
    bundle_root=""
    target_root=""
    source_profile=""
    require_operator_evidence=0
    while [ $# -gt 0 ]; do
      case "$1" in
        --bundle-root) bundle_root=$2; shift 2 ;;
        --target-root) target_root=$2; shift 2 ;;
        --source-profile) source_profile=$2; shift 2 ;;
        --require-operator-evidence) require_operator_evidence=1; shift 1 ;;
        *) printf 'unknown arg: %s\n' "$1" >&2; exit 2 ;;
      esac
    done
    acceptance_require_flag "$bundle_root" "--bundle-root"
    acceptance_require_flag "$target_root" "--target-root"
    acceptance_require_flag "$source_profile" "--source-profile"
    acceptance_two_machine_evaluate "$bundle_root" "$target_root" "$source_profile" "$require_operator_evidence"
    ;;
  ""|-h|--help|help)
    acceptance_two_machine_usage
    ;;
  *)
    printf 'unknown role: %s\n' "$role" >&2
    acceptance_two_machine_usage >&2
    exit 2
    ;;
esac
