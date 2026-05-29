#!/bin/sh
set -eu
export LC_ALL=C
export LANG=C

ROOT_DIR=$(CDPATH= cd "$(dirname "$0")/../.." && pwd)
COMMON_LIB="$ROOT_DIR/macos/script/lib/acceptance-common.sh"
DEFAULT_APP_DIR="$ROOT_DIR/macos/dist/SuperMover.app"

usage() {
  printf 'usage: %s [path/to/SuperMover.app]\n' "$(basename "$0")"
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi
if [ "$#" -gt 1 ]; then
  usage >&2
  exit 2
fi

APP_INPUT=${1:-$DEFAULT_APP_DIR}
case "$APP_INPUT" in
  /*) APP_DIR=$APP_INPUT ;;
  *)
    APP_PARENT=$(CDPATH= cd "$(dirname "$APP_INPUT")" 2>/dev/null && pwd || printf '.')
    APP_DIR="$APP_PARENT/$(basename "$APP_INPUT")"
    ;;
esac

. "$COMMON_LIB"

CHECKED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
INFO_PLIST="$APP_DIR/Contents/Info.plist"
RESOURCES_DIR="$APP_DIR/Contents/Resources"
PROVENANCE_FILE="$RESOURCES_DIR/supermover-provenance.json"
NOTARIZATION_SIDECAR_FILE="$(dirname "$APP_DIR")/$(basename "$APP_DIR").notary/notarization.json"
EXPECTED_PROVENANCE_SCHEMA="supermover.macos.provenance.v1"
EXPECTED_AUDIT_SCHEMA="supermover.macos.app_audit.v1"
EXPECTED_NOTARIZATION_SCHEMA="supermover.macos.notarization.v1"
EXPECTED_CLI_RELATIVE_PATH="Contents/Resources/bin/supermover"

if ! command -v jq >/dev/null 2>&1; then
  printf '{"schema":"supermover.macos.app_audit.v1","status":"blocked","readiness":"blocked","checked_at":"%s","app_path":"%s","checks":[{"id":"tool.jq","status":"blocked","severity":"blocking","summary":"jq is required","detail":"Install jq to run the structured macOS app audit."}]}\n' "$CHECKED_AT" "$APP_DIR"
  exit 1
fi

TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/supermover-audit-app.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT HUP INT TERM

CHECKS_NDJSON="$TMP_DIR/checks.ndjson"
HASHES_NDJSON="$TMP_DIR/hashes.ndjson"
: > "$CHECKS_NDJSON"
: > "$HASHES_NDJSON"

BLOCKING_COUNT=0
REVIEW_COUNT=0

add_check() {
  check_id=$1
  check_status=$2
  check_severity=$3
  check_summary=$4
  check_detail=$5
  jq -nc \
    --arg id "$check_id" \
    --arg status "$check_status" \
    --arg severity "$check_severity" \
    --arg summary "$check_summary" \
    --arg detail "$check_detail" \
    '{id: $id, status: $status, severity: $severity, summary: $summary, detail: $detail}' \
    >> "$CHECKS_NDJSON"
  case "$check_status" in
    blocked) BLOCKING_COUNT=$((BLOCKING_COUNT + 1)) ;;
    review) REVIEW_COUNT=$((REVIEW_COUNT + 1)) ;;
  esac
}

run_capture() {
  RUN_CAPTURE_OUT=$1
  shift
  set +e
  "$@" > "$RUN_CAPTURE_OUT" 2>&1
  RUN_CAPTURE_STATUS=$?
  set -e
}

plist_get() {
  plist_key=$1
  if [ -f "$INFO_PLIST" ] && command -v plutil >/dev/null 2>&1; then
    safe_key=$(printf '%s' "$plist_key" | tr -c 'A-Za-z0-9_' '_')
    plist_out="$TMP_DIR/plist-$safe_key.txt"
    if plutil -extract "$plist_key" raw -o - "$INFO_PLIST" > "$plist_out" 2>/dev/null; then
      cat "$plist_out"
    fi
  fi
}

hash_one() {
  hash_abs_path=$1
  hash_rel_path=$2
  if [ -f "$hash_abs_path" ]; then
    hash_sha=$(shasum -a 256 "$hash_abs_path" | awk '{print $1}')
    hash_bytes=$(wc -c < "$hash_abs_path" | tr -d '[:space:]')
    jq -nc \
      --arg path "$hash_rel_path" \
      --arg sha256 "$hash_sha" \
      --argjson bytes "$hash_bytes" \
      '{path: $path, sha256: $sha256, bytes: $bytes}' \
      >> "$HASHES_NDJSON"
  fi
}

write_unavailable_codesign_json() {
  unavailable_subject=$1
  unavailable_path=$2
  unavailable_reason=$3
  unavailable_json=$4
  jq -n \
    --arg subject "$unavailable_subject" \
    --arg path "$unavailable_path" \
    --arg reason "$unavailable_reason" \
    '{
      subject: $subject,
      path: $path,
      available: false,
      reason: $reason,
      verify: null,
      details: null,
      ad_hoc: null,
      hardened_runtime: null,
      developer_id_application: null,
      team_identifier: null,
      authorities: [],
      entitlements: null
    }' > "$unavailable_json"
}

audit_codesign_subject() {
  cs_subject=$1
  cs_path=$2
  cs_json=$3
  if [ ! -e "$cs_path" ]; then
    write_unavailable_codesign_json "$cs_subject" "$cs_path" "path does not exist" "$cs_json"
    add_check "codesign.$cs_subject.exists" "blocked" "blocking" "$cs_subject path is missing" "$cs_path"
    return
  fi
  if ! command -v codesign >/dev/null 2>&1; then
    write_unavailable_codesign_json "$cs_subject" "$cs_path" "codesign is unavailable" "$cs_json"
    add_check "codesign.$cs_subject.available" "blocked" "blocking" "codesign is unavailable" "Cannot verify $cs_subject signing details."
    return
  fi

  cs_verify_out="$TMP_DIR/codesign-$cs_subject-verify.txt"
  cs_details_out="$TMP_DIR/codesign-$cs_subject-details.txt"
  cs_entitlements_out="$TMP_DIR/codesign-$cs_subject-entitlements.plist"
  run_capture "$cs_verify_out" codesign --verify --strict --verbose=4 "$cs_path"
  cs_verify_status=$RUN_CAPTURE_STATUS
  run_capture "$cs_details_out" codesign -dv --verbose=4 "$cs_path"
  cs_details_status=$RUN_CAPTURE_STATUS
  run_capture "$cs_entitlements_out" codesign -d --entitlements - --xml "$cs_path"
  cs_entitlements_status=$RUN_CAPTURE_STATUS

  if grep -q '^Signature=adhoc$' "$cs_details_out"; then
    cs_ad_hoc=true
  else
    cs_ad_hoc=false
  fi
  if grep -q '^CodeDirectory .*flags=.*runtime' "$cs_details_out"; then
    cs_hardened_runtime=true
  else
    cs_hardened_runtime=false
  fi
  if grep -q '^Authority=Developer ID Application' "$cs_details_out"; then
    cs_developer_id=true
  else
    cs_developer_id=false
  fi
  cs_team_identifier=$(sed -n 's/^TeamIdentifier=//p' "$cs_details_out" | sed -n '1p')
  sed -n 's/^Authority=//p' "$cs_details_out" > "$TMP_DIR/codesign-$cs_subject-authorities.txt"
  jq -R -s 'split("\n") | map(select(length > 0))' "$TMP_DIR/codesign-$cs_subject-authorities.txt" > "$TMP_DIR/codesign-$cs_subject-authorities.json"
  cs_entitlements_plist="$TMP_DIR/codesign-$cs_subject-entitlements-only.plist"
  awk 'seen || /^<\?xml/ || /^<plist/ { seen = 1; print }' "$cs_entitlements_out" > "$cs_entitlements_plist"
  if [ "$cs_entitlements_status" -eq 0 ] && grep -q '<plist' "$cs_entitlements_plist"; then
    cs_entitlements_json="$TMP_DIR/codesign-$cs_subject-entitlements.json"
    plutil -convert json -o "$cs_entitlements_json" "$cs_entitlements_plist" 2>/dev/null || printf 'null\n' > "$cs_entitlements_json"
    cs_entitlements_available=true
  else
    cs_entitlements_json="$TMP_DIR/codesign-$cs_subject-entitlements.json"
    printf 'null\n' > "$cs_entitlements_json"
    cs_entitlements_available=false
  fi

  jq -n \
    --arg subject "$cs_subject" \
    --arg path "$cs_path" \
    --argjson verify_exit "$cs_verify_status" \
    --argjson details_exit "$cs_details_status" \
    --argjson entitlements_exit "$cs_entitlements_status" \
    --argjson entitlements_available "$cs_entitlements_available" \
    --argjson ad_hoc "$cs_ad_hoc" \
    --argjson hardened_runtime "$cs_hardened_runtime" \
    --argjson developer_id "$cs_developer_id" \
    --arg team_identifier "$cs_team_identifier" \
    --rawfile verify_output "$cs_verify_out" \
    --rawfile details_output "$cs_details_out" \
    --rawfile entitlements_output "$cs_entitlements_out" \
    --slurpfile authorities "$TMP_DIR/codesign-$cs_subject-authorities.json" \
    --slurpfile entitlements "$cs_entitlements_json" \
    '{
      subject: $subject,
      path: $path,
      available: true,
      verify: {
        exit_code: $verify_exit,
        status: (if $verify_exit == 0 then "pass" else "blocked" end),
        output: $verify_output
      },
      details: {
        exit_code: $details_exit,
        status: (if $details_exit == 0 then "pass" else "blocked" end),
        output: $details_output
      },
      entitlements_dump: {
        exit_code: $entitlements_exit,
        status: (if $entitlements_exit == 0 and $entitlements_available then "pass" else "blocked" end),
        output: $entitlements_output
      },
      ad_hoc: $ad_hoc,
      hardened_runtime: $hardened_runtime,
      developer_id_application: $developer_id,
      team_identifier: (if $team_identifier == "" or $team_identifier == "not set" then null else $team_identifier end),
      authorities: $authorities[0],
      entitlements: $entitlements[0]
    }' > "$cs_json"

  if [ "$cs_verify_status" -eq 0 ]; then
    add_check "codesign.$cs_subject.verify" "pass" "required" "$cs_subject code signature verifies" "codesign --verify --strict succeeded."
  else
    add_check "codesign.$cs_subject.verify" "blocked" "blocking" "$cs_subject code signature does not verify" "$(tr '\n' ' ' < "$cs_verify_out")"
  fi

  if [ "$cs_details_status" -ne 0 ]; then
    add_check "codesign.$cs_subject.identity" "blocked" "blocking" "$cs_subject signing identity is unavailable" "$(tr '\n' ' ' < "$cs_details_out")"
  elif [ "$cs_ad_hoc" = "true" ]; then
    add_check "codesign.$cs_subject.identity" "blocked" "blocking" "$cs_subject is ad-hoc signed" "Ad-hoc signing is local review evidence only, not distribution readiness."
  elif [ "$cs_developer_id" = "true" ]; then
    add_check "codesign.$cs_subject.identity" "pass" "required" "$cs_subject has Developer ID Application authority" "Developer ID Application authority is present; notarization and stapling are validated separately."
  else
    add_check "codesign.$cs_subject.identity" "blocked" "blocking" "$cs_subject is not signed for Developer ID distribution" "Expected a Developer ID Application authority."
  fi
  if [ "$cs_details_status" -eq 0 ] && [ "$cs_hardened_runtime" = "true" ]; then
    add_check "codesign.$cs_subject.runtime" "pass" "required" "$cs_subject has hardened runtime enabled" "codesign flags include runtime."
  else
    add_check "codesign.$cs_subject.runtime" "blocked" "blocking" "$cs_subject is missing hardened runtime" "Expected codesign --options runtime evidence."
  fi
  if [ "$cs_entitlements_available" = "true" ]; then
    add_check "codesign.$cs_subject.entitlements" "pass" "required" "$cs_subject entitlements were dumped" "codesign -d --entitlements - succeeded."
    for entitlement_key in \
      "com.apple.security.files.user-selected.read-write" \
      "com.apple.security.network.client" \
      "com.apple.security.network.server"
    do
      if jq -e --arg key "$entitlement_key" '.[$key] == true' "$cs_entitlements_json" >/dev/null; then
        add_check "codesign.$cs_subject.entitlement.$entitlement_key" "pass" "required" "$cs_subject entitlement is present" "$entitlement_key"
      else
        add_check "codesign.$cs_subject.entitlement.$entitlement_key" "blocked" "blocking" "$cs_subject entitlement is missing" "$entitlement_key"
      fi
    done
  else
    add_check "codesign.$cs_subject.entitlements" "blocked" "blocking" "$cs_subject entitlements are unavailable" "$(tr '\n' ' ' < "$cs_entitlements_out")"
  fi
}

write_assessment_json() {
  assessment_tool=$1
  assessment_available=$2
  assessment_status=$3
  assessment_exit=$4
  assessment_command=$5
  assessment_output=$6
  assessment_json=$7
  jq -n \
    --arg tool "$assessment_tool" \
    --argjson available "$assessment_available" \
    --arg status "$assessment_status" \
    --argjson exit_code "$assessment_exit" \
    --arg command "$assessment_command" \
    --rawfile output "$assessment_output" \
    '{
      tool: $tool,
      available: $available,
      status: $status,
      exit_code: $exit_code,
      command: $command,
      output: $output
    }' > "$assessment_json"
}

audit_spctl() {
  spctl_json=$1
  spctl_out="$TMP_DIR/spctl.txt"
  : > "$spctl_out"
  if [ ! -d "$APP_DIR" ]; then
    write_assessment_json "spctl" false "blocked" 127 "spctl --assess --type execute --verbose=4" "$spctl_out" "$spctl_json"
    add_check "spctl.assess" "blocked" "blocking" "spctl assessment cannot run" "App bundle path is missing."
    return
  fi
  if ! command -v spctl >/dev/null 2>&1; then
    write_assessment_json "spctl" false "blocked" 127 "spctl --assess --type execute --verbose=4" "$spctl_out" "$spctl_json"
    add_check "spctl.assess" "blocked" "blocking" "spctl is unavailable" "Cannot assess Gatekeeper execution readiness."
    return
  fi
  run_capture "$spctl_out" spctl --assess --type execute --verbose=4 "$APP_DIR"
  spctl_exit=$RUN_CAPTURE_STATUS
  if [ "$spctl_exit" -eq 0 ]; then
    spctl_status=pass
    add_check "spctl.assess" "pass" "required" "spctl assessment passed" "Gatekeeper accepted the app for execution."
  else
    spctl_status=blocked
    add_check "spctl.assess" "blocked" "blocking" "spctl assessment failed" "$(tr '\n' ' ' < "$spctl_out")"
  fi
  write_assessment_json "spctl" true "$spctl_status" "$spctl_exit" "spctl --assess --type execute --verbose=4" "$spctl_out" "$spctl_json"
}

audit_stapler() {
  stapler_json=$1
  stapler_out="$TMP_DIR/stapler.txt"
  : > "$stapler_out"
  if [ ! -d "$APP_DIR" ]; then
    write_assessment_json "stapler" false "blocked" 127 "xcrun stapler validate" "$stapler_out" "$stapler_json"
    add_check "stapler.validate" "blocked" "blocking" "stapler validation cannot run" "App bundle path is missing."
    return
  fi
  if ! command -v xcrun >/dev/null 2>&1 || ! xcrun -f stapler >/dev/null 2>&1; then
    write_assessment_json "stapler" false "blocked" 127 "xcrun stapler validate" "$stapler_out" "$stapler_json"
    add_check "stapler.validate" "blocked" "blocking" "stapler is unavailable" "Cannot validate a notarization ticket staple."
    return
  fi
  run_capture "$stapler_out" xcrun stapler validate "$APP_DIR"
  stapler_exit=$RUN_CAPTURE_STATUS
  if [ "$stapler_exit" -eq 0 ]; then
    stapler_status=pass
    add_check "stapler.validate" "pass" "required" "stapler validation passed" "A stapled notarization ticket was validated."
  else
    stapler_status=blocked
    add_check "stapler.validate" "blocked" "blocking" "stapler validation failed" "$(tr '\n' ' ' < "$stapler_out")"
  fi
  write_assessment_json "stapler" true "$stapler_status" "$stapler_exit" "xcrun stapler validate" "$stapler_out" "$stapler_json"
}

audit_canonical_notarization_sidecar_if_present() {
  if [ ! -f "$NOTARIZATION_SIDECAR_FILE" ] && [ ! -L "$NOTARIZATION_SIDECAR_FILE" ]; then
    return 0
  fi

  if ! acceptance_notarization_source_is_safe "$NOTARIZATION_SIDECAR_FILE"; then
    add_check "notarization.sidecar.parse" "blocked" "blocking" "canonical notarization sidecar uses an unsafe symlink path" "$NOTARIZATION_SIDECAR_FILE"
    add_check "notarization.sidecar.currentness" "blocked" "blocking" "canonical notarization sidecar does not match the current app and bundled provenance" "$NOTARIZATION_SIDECAR_FILE"
    add_check "notarization.sidecar.release_ready" "blocked" "blocking" "canonical notarization sidecar does not record release-ready notarization evidence" "$NOTARIZATION_SIDECAR_FILE"
    return 0
  fi

  if ! jq -e \
    --arg schema "$EXPECTED_NOTARIZATION_SCHEMA" \
    '.schema == $schema and (.status | type == "string") and (.status | length > 0)' \
    "$NOTARIZATION_SIDECAR_FILE" >/dev/null 2>&1; then
    add_check "notarization.sidecar.parse" "blocked" "blocking" "canonical notarization sidecar is malformed" "$NOTARIZATION_SIDECAR_FILE"
    add_check "notarization.sidecar.currentness" "blocked" "blocking" "canonical notarization sidecar does not match the current app and bundled provenance" "$NOTARIZATION_SIDECAR_FILE"
    add_check "notarization.sidecar.release_ready" "blocked" "blocking" "canonical notarization sidecar does not record release-ready notarization evidence" "$NOTARIZATION_SIDECAR_FILE"
    return 0
  fi

  add_check "notarization.sidecar.parse" "pass" "required" "canonical notarization sidecar parses" "$NOTARIZATION_SIDECAR_FILE"

  sidecar_notary_dir=$(dirname "$NOTARIZATION_SIDECAR_FILE")
  sidecar_notary_log_path=$(jq -r '.notary_log.path // ""' "$NOTARIZATION_SIDECAR_FILE")
  sidecar_submission_id=$(jq -r '.submission.id // ""' "$NOTARIZATION_SIDECAR_FILE")
  sidecar_expected_notary_log_path="$sidecar_notary_dir/notary-log.json"
  sidecar_notary_log_accepted=false
  if [ -n "$sidecar_notary_log_path" ] \
    && [ "$sidecar_notary_log_path" = "$sidecar_expected_notary_log_path" ] \
    && acceptance_notarization_regular_artifact_is_safe "$sidecar_notary_log_path" \
    && acceptance_notary_log_is_accepted "$sidecar_notary_log_path" "$sidecar_submission_id"; then
    sidecar_notary_log_accepted=true
    add_check "notarization.sidecar.notary_log" "pass" "required" "canonical notarization sidecar references accepted current notary log evidence" "$sidecar_notary_log_path"
  else
    add_check "notarization.sidecar.notary_log" "blocked" "blocking" "canonical notarization sidecar does not reference accepted current notary log evidence" "path='$sidecar_notary_log_path' expected='$sidecar_expected_notary_log_path'"
  fi

  sidecar_current=false
  if acceptance_notarization_evidence_matches_current_app "$NOTARIZATION_SIDECAR_FILE" "$APP_DIR"; then
    sidecar_current=true
    add_check "notarization.sidecar.currentness" "pass" "required" "canonical notarization sidecar matches the current app and bundled provenance" "$NOTARIZATION_SIDECAR_FILE"
  else
    add_check "notarization.sidecar.currentness" "blocked" "blocking" "canonical notarization sidecar does not match the current app and bundled provenance" "$NOTARIZATION_SIDECAR_FILE"
  fi

  sidecar_status=$(jq -r '.status // ""' "$NOTARIZATION_SIDECAR_FILE")
  sidecar_submission_status=$(jq -r '.submission.status // ""' "$NOTARIZATION_SIDECAR_FILE")
  sidecar_auth_mode=$(jq -r '.auth_mode // ""' "$NOTARIZATION_SIDECAR_FILE")
  sidecar_audit_status=$(jq -r '.audit.status // ""' "$NOTARIZATION_SIDECAR_FILE")
  sidecar_audit_readiness=$(jq -r '.audit.readiness // ""' "$NOTARIZATION_SIDECAR_FILE")
  sidecar_audit_pass_ready=$(jq -r '.audit.pass_ready // false' "$NOTARIZATION_SIDECAR_FILE")
  if jq -e '(.submission.id | type == "string") and ((.submission.id | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))' "$NOTARIZATION_SIDECAR_FILE" >/dev/null 2>&1; then
    sidecar_submission_uuid=true
  else
    sidecar_submission_uuid=false
  fi
  case "$sidecar_auth_mode" in
    keychain_profile|api_key|apple_id) sidecar_auth_mode_ready=true ;;
    *) sidecar_auth_mode_ready=false ;;
  esac
  if jq -e '.failure == null' "$NOTARIZATION_SIDECAR_FILE" >/dev/null 2>&1; then
    sidecar_failure_absent=true
  else
    sidecar_failure_absent=false
  fi

  if [ "$sidecar_current" = "true" ] \
    && [ "$sidecar_status" = "pass" ] \
    && [ "$sidecar_submission_uuid" = "true" ] \
    && [ "$sidecar_submission_status" = "Accepted" ] \
    && [ "$sidecar_auth_mode_ready" = "true" ] \
    && [ "$sidecar_failure_absent" = "true" ] \
    && [ "$sidecar_notary_log_accepted" = "true" ] \
    && [ "$sidecar_audit_status" = "pass" ] \
    && [ "$sidecar_audit_readiness" = "distribution_ready" ] \
    && [ "$sidecar_audit_pass_ready" = "true" ]; then
    add_check "notarization.sidecar.release_ready" "pass" "required" "canonical notarization sidecar records release-ready notarization evidence" "status='$sidecar_status' auth_mode='$sidecar_auth_mode' submission.id='$sidecar_submission_id' submission='$sidecar_submission_status' failure_absent='$sidecar_failure_absent' notary_log.path='$sidecar_notary_log_path' audit.status='$sidecar_audit_status' audit.readiness='$sidecar_audit_readiness' audit.pass_ready='$sidecar_audit_pass_ready'"
  else
    add_check "notarization.sidecar.release_ready" "blocked" "blocking" "canonical notarization sidecar does not record release-ready notarization evidence" "status='$sidecar_status' auth_mode='$sidecar_auth_mode' submission.id='$sidecar_submission_id' submission_uuid='$sidecar_submission_uuid' submission='$sidecar_submission_status' failure_absent='$sidecar_failure_absent' notary_log.path='$sidecar_notary_log_path' notary_log_accepted='$sidecar_notary_log_accepted' audit.status='$sidecar_audit_status' audit.readiness='$sidecar_audit_readiness' audit.pass_ready='$sidecar_audit_pass_ready'"
  fi
}

if [ -d "$APP_DIR" ]; then
  add_check "app.exists" "pass" "required" "app bundle exists" "$APP_DIR"
else
  add_check "app.exists" "blocked" "blocking" "app bundle is missing" "$APP_DIR"
fi

PLIST_LINT_OUT="$TMP_DIR/plist-lint.txt"
if [ -f "$INFO_PLIST" ]; then
  add_check "plist.exists" "pass" "required" "Info.plist exists" "$INFO_PLIST"
  if command -v plutil >/dev/null 2>&1; then
    run_capture "$PLIST_LINT_OUT" plutil -lint "$INFO_PLIST"
    PLIST_LINT_STATUS=$RUN_CAPTURE_STATUS
    if [ "$PLIST_LINT_STATUS" -eq 0 ]; then
      add_check "plist.lint" "pass" "required" "Info.plist is valid" "$(tr '\n' ' ' < "$PLIST_LINT_OUT")"
    else
      add_check "plist.lint" "blocked" "blocking" "Info.plist is invalid" "$(tr '\n' ' ' < "$PLIST_LINT_OUT")"
    fi
  else
    : > "$PLIST_LINT_OUT"
    PLIST_LINT_STATUS=127
    add_check "plist.lint" "blocked" "blocking" "plutil is unavailable" "Cannot validate Info.plist."
  fi
else
  : > "$PLIST_LINT_OUT"
  PLIST_LINT_STATUS=127
  add_check "plist.exists" "blocked" "blocking" "Info.plist is missing" "$INFO_PLIST"
fi

PLIST_BUNDLE_ID=$(plist_get CFBundleIdentifier)
PLIST_ICON_FILE=$(plist_get CFBundleIconFile)
PLIST_SHORT_VERSION=$(plist_get CFBundleShortVersionString)
PLIST_BUNDLE_VERSION=$(plist_get CFBundleVersion)
PLIST_EXECUTABLE=$(plist_get CFBundleExecutable)
PLIST_PACKAGE_TYPE=$(plist_get CFBundlePackageType)
PLIST_LOCAL_NETWORK_USAGE=$(plist_get NSLocalNetworkUsageDescription)

if [ "$PLIST_BUNDLE_ID" = "" ]; then
  add_check "plist.bundle_id" "blocked" "blocking" "Info.plist is missing CFBundleIdentifier" "CFBundleIdentifier is required for provenance comparison."
else
  add_check "plist.bundle_id" "pass" "required" "Info.plist bundle identifier is present" "$PLIST_BUNDLE_ID"
fi
if [ "$PLIST_ICON_FILE" = "" ]; then
  add_check "plist.icon_file" "blocked" "blocking" "Info.plist is missing CFBundleIconFile" "CFBundleIconFile is required for the packaged app icon."
else
  add_check "plist.icon_file" "pass" "required" "Info.plist app icon reference is present" "$PLIST_ICON_FILE"
fi
if [ "$PLIST_SHORT_VERSION" = "" ]; then
  add_check "plist.short_version" "blocked" "blocking" "Info.plist is missing CFBundleShortVersionString" "CFBundleShortVersionString is required for provenance comparison."
else
  add_check "plist.short_version" "pass" "required" "Info.plist short version is present" "$PLIST_SHORT_VERSION"
fi
if [ "$PLIST_EXECUTABLE" = "" ]; then
  add_check "plist.executable" "blocked" "blocking" "Info.plist is missing CFBundleExecutable" "CFBundleExecutable is required to locate the app executable."
else
  add_check "plist.executable" "pass" "required" "Info.plist executable is present" "$PLIST_EXECUTABLE"
fi
if [ "$PLIST_LOCAL_NETWORK_USAGE" = "" ]; then
  add_check "plist.local_network_usage" "blocked" "blocking" "Info.plist is missing NSLocalNetworkUsageDescription" "Installed-app LAN discovery, pairing, and serve flows require an explicit Local Network usage description."
else
  add_check "plist.local_network_usage" "pass" "required" "Info.plist Local Network usage description is present" "$PLIST_LOCAL_NETWORK_USAGE"
fi

APP_ICON_FILE_NAME="$PLIST_ICON_FILE"
case "$APP_ICON_FILE_NAME" in
  *.icns) ;;
  *) APP_ICON_FILE_NAME="$APP_ICON_FILE_NAME.icns" ;;
esac
APP_ICON_PATH="$RESOURCES_DIR/$APP_ICON_FILE_NAME"
if [ "$PLIST_ICON_FILE" != "" ] && [ -f "$APP_ICON_PATH" ]; then
  add_check "app.icon.exists" "pass" "required" "app icon resource exists" "Contents/Resources/$APP_ICON_FILE_NAME"
else
  add_check "app.icon.exists" "blocked" "blocking" "app icon resource is missing" "Contents/Resources/$APP_ICON_FILE_NAME"
fi

APP_EXECUTABLE_PATH="$APP_DIR/Contents/MacOS/$PLIST_EXECUTABLE"
if [ "$PLIST_EXECUTABLE" != "" ] && [ -x "$APP_EXECUTABLE_PATH" ]; then
  add_check "app.executable" "pass" "required" "app executable exists and is executable" "Contents/MacOS/$PLIST_EXECUTABLE"
else
  add_check "app.executable" "blocked" "blocking" "app executable is missing or not executable" "Contents/MacOS/$PLIST_EXECUTABLE"
fi

jq -n \
  --arg path "$INFO_PLIST" \
  --argjson lint_exit "$PLIST_LINT_STATUS" \
  --rawfile lint_output "$PLIST_LINT_OUT" \
  --arg bundle_id "$PLIST_BUNDLE_ID" \
  --arg icon_file "$PLIST_ICON_FILE" \
  --arg short_version "$PLIST_SHORT_VERSION" \
  --arg bundle_version "$PLIST_BUNDLE_VERSION" \
  --arg executable "$PLIST_EXECUTABLE" \
  --arg package_type "$PLIST_PACKAGE_TYPE" \
  --arg local_network_usage "$PLIST_LOCAL_NETWORK_USAGE" \
  '{
    path: $path,
    lint: {
      exit_code: $lint_exit,
      status: (if $lint_exit == 0 then "pass" else "blocked" end),
      output: $lint_output
    },
    bundle_id: (if $bundle_id == "" then null else $bundle_id end),
    icon_file: (if $icon_file == "" then null else $icon_file end),
    short_version: (if $short_version == "" then null else $short_version end),
    bundle_version: (if $bundle_version == "" then null else $bundle_version end),
    executable: (if $executable == "" then null else $executable end),
    package_type: (if $package_type == "" then null else $package_type end),
    local_network_usage_description: (if $local_network_usage == "" then null else $local_network_usage end)
  }' > "$TMP_DIR/plist.json"

PROVENANCE_LOADED=false
PROVENANCE_ERROR=""
PROV_SCHEMA=""
PROV_APP_BUNDLE_ID=""
PROV_APP_VERSION=""
PROV_GIT_COMMIT=""
PROV_GIT_DIRTY=""
PROV_CLI_VERSION=""
PROV_CLI_RELATIVE_PATH=""
PROV_SIGNING=""
PROVENANCE_NORMALIZED="$TMP_DIR/provenance.normalized.json"
if [ -f "$PROVENANCE_FILE" ]; then
  run_capture "$TMP_DIR/provenance-parse.err" jq -e . "$PROVENANCE_FILE"
  PROVENANCE_PARSE_STATUS=$RUN_CAPTURE_STATUS
  if [ "$PROVENANCE_PARSE_STATUS" -eq 0 ]; then
    jq . "$PROVENANCE_FILE" > "$PROVENANCE_NORMALIZED"
    PROVENANCE_LOADED=true
    add_check "provenance.parse" "pass" "required" "provenance JSON parses" "$PROVENANCE_FILE"
    PROV_SCHEMA=$(jq -r '.schema // ""' "$PROVENANCE_NORMALIZED")
    PROV_APP_BUNDLE_ID=$(jq -r '.app_bundle_id // ""' "$PROVENANCE_NORMALIZED")
    PROV_APP_VERSION=$(jq -r '.app_version // ""' "$PROVENANCE_NORMALIZED")
    PROV_GIT_COMMIT=$(jq -r '.git_commit // ""' "$PROVENANCE_NORMALIZED")
    PROV_GIT_DIRTY=$(jq -r 'if has("git_dirty") then (.git_dirty | tostring) else "" end' "$PROVENANCE_NORMALIZED")
    PROV_CLI_VERSION=$(jq -r '.cli_version // ""' "$PROVENANCE_NORMALIZED")
    PROV_CLI_RELATIVE_PATH=$(jq -r '.cli_relative_path // ""' "$PROVENANCE_NORMALIZED")
    PROV_SIGNING=$(jq -r '.signing // ""' "$PROVENANCE_NORMALIZED")
  else
    PROVENANCE_ERROR=$(tr '\n' ' ' < "$TMP_DIR/provenance-parse.err")
    printf 'null\n' > "$PROVENANCE_NORMALIZED"
    add_check "provenance.parse" "blocked" "blocking" "provenance JSON is malformed" "$PROVENANCE_ERROR"
  fi
else
  printf 'null\n' > "$PROVENANCE_NORMALIZED"
  add_check "provenance.exists" "blocked" "blocking" "provenance manifest is missing" "$PROVENANCE_FILE"
fi

if [ "$PROVENANCE_LOADED" = "true" ]; then
  if [ "$PROV_SCHEMA" = "$EXPECTED_PROVENANCE_SCHEMA" ]; then
    add_check "provenance.schema" "pass" "required" "provenance schema matches" "$PROV_SCHEMA"
  else
    add_check "provenance.schema" "blocked" "blocking" "provenance schema mismatch" "got '$PROV_SCHEMA', want '$EXPECTED_PROVENANCE_SCHEMA'"
  fi
  if [ "$PROV_APP_BUNDLE_ID" = "$PLIST_BUNDLE_ID" ] && [ "$PROV_APP_BUNDLE_ID" != "" ]; then
    add_check "provenance.app_bundle_id" "pass" "required" "provenance bundle id matches Info.plist" "$PROV_APP_BUNDLE_ID"
  else
    add_check "provenance.app_bundle_id" "blocked" "blocking" "provenance bundle id does not match Info.plist" "provenance='$PROV_APP_BUNDLE_ID' plist='$PLIST_BUNDLE_ID'"
  fi
  if [ "$PROV_APP_VERSION" = "$PLIST_SHORT_VERSION" ] && [ "$PROV_APP_VERSION" != "" ]; then
    add_check "provenance.app_version" "pass" "required" "provenance app version matches Info.plist" "$PROV_APP_VERSION"
  else
    add_check "provenance.app_version" "blocked" "blocking" "provenance app version does not match Info.plist" "provenance='$PROV_APP_VERSION' plist='$PLIST_SHORT_VERSION'"
  fi
  if [ "$PROV_CLI_RELATIVE_PATH" = "$EXPECTED_CLI_RELATIVE_PATH" ]; then
    add_check "provenance.cli_relative_path" "pass" "required" "provenance CLI path is expected" "$PROV_CLI_RELATIVE_PATH"
  else
    add_check "provenance.cli_relative_path" "blocked" "blocking" "provenance CLI path is unexpected" "got '$PROV_CLI_RELATIVE_PATH', want '$EXPECTED_CLI_RELATIVE_PATH'"
  fi
  case "$PROV_SIGNING" in
    ""|"unsigned")
      add_check "provenance.signing" "blocked" "blocking" "provenance records an unsigned build" "Manifest signing='$PROV_SIGNING'. This is local review evidence only."
      ;;
    "-")
      add_check "provenance.signing" "blocked" "blocking" "provenance records ad-hoc signing" "Ad-hoc signing is local review evidence only."
      ;;
    *)
      add_check "provenance.signing" "pass" "required" "provenance records a signing identity" "Manifest signing='$PROV_SIGNING'. codesign, spctl, and stapler checks remain authoritative."
      ;;
  esac
fi

PROVENANCE_EXISTS=false
if [ -f "$PROVENANCE_FILE" ]; then
  PROVENANCE_EXISTS=true
fi
jq -n \
  --arg path "$PROVENANCE_FILE" \
  --argjson exists "$PROVENANCE_EXISTS" \
  --argjson loaded "$PROVENANCE_LOADED" \
  --arg error "$PROVENANCE_ERROR" \
  --slurpfile manifest "$PROVENANCE_NORMALIZED" \
  '{
    path: $path,
    exists: $exists,
    loaded: $loaded,
    error: (if $error == "" then null else $error end),
    manifest: $manifest[0]
  }' > "$TMP_DIR/provenance.json"

GIT_HEAD="unknown"
GIT_DIRTY=true
GIT_STATUS_FILE="$TMP_DIR/git-status.txt"
: > "$GIT_STATUS_FILE"
if git -C "$ROOT_DIR" rev-parse --short=12 HEAD >/dev/null 2>&1; then
  GIT_HEAD=$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD)
  git -C "$ROOT_DIR" status --porcelain --untracked-files=normal -- . > "$GIT_STATUS_FILE" 2>/dev/null || true
  if [ -s "$GIT_STATUS_FILE" ]; then
    GIT_DIRTY=true
  else
    GIT_DIRTY=false
  fi
  add_check "git.head" "pass" "required" "current git HEAD was read" "$GIT_HEAD"
else
  add_check "git.head" "blocked" "blocking" "current git HEAD is unavailable" "$ROOT_DIR"
fi
GIT_DIRTY_COUNT=$(sed '/^$/d' "$GIT_STATUS_FILE" | wc -l | tr -d '[:space:]')
sed '/^$/d' "$GIT_STATUS_FILE" | jq -R -s 'split("\n") | map(select(length > 0))' > "$TMP_DIR/git-dirty-lines.json"

if [ "$PROVENANCE_LOADED" = "true" ]; then
  if [ "$PROV_GIT_COMMIT" = "$GIT_HEAD" ]; then
    add_check "git.provenance_head" "pass" "required" "provenance git commit matches current HEAD" "$GIT_HEAD"
  else
    add_check "git.provenance_head" "blocked" "blocking" "provenance git commit does not match current HEAD" "provenance='$PROV_GIT_COMMIT' current='$GIT_HEAD'"
  fi
  if [ "$PROV_GIT_DIRTY" = "$GIT_DIRTY" ]; then
    add_check "git.provenance_dirty" "pass" "required" "provenance dirty bit matches current worktree" "$GIT_DIRTY"
  else
    add_check "git.provenance_dirty" "blocked" "blocking" "provenance dirty bit does not match current worktree" "provenance='$PROV_GIT_DIRTY' current='$GIT_DIRTY'"
  fi
  if [ "$PROV_GIT_DIRTY" = "true" ] || [ "$GIT_DIRTY" = "true" ]; then
    add_check "git.clean" "blocked" "blocking" "worktree is dirty" "Dirty paths recorded by git status: $GIT_DIRTY_COUNT"
  else
    add_check "git.clean" "pass" "required" "worktree is clean" "Provenance and current git state are clean."
  fi
else
  add_check "git.clean" "blocked" "blocking" "worktree cleanliness cannot be tied to provenance" "A readable provenance manifest is required."
fi

jq -n \
  --arg root "$ROOT_DIR" \
  --arg head "$GIT_HEAD" \
  --argjson dirty "$GIT_DIRTY" \
  --argjson dirty_count "$GIT_DIRTY_COUNT" \
  --slurpfile dirty_lines "$TMP_DIR/git-dirty-lines.json" \
  '{
    root: $root,
    current_head: $head,
    current_dirty: $dirty,
    dirty_path_count: $dirty_count,
    dirty_status_lines: $dirty_lines[0]
  }' > "$TMP_DIR/git.json"

CLI_PATH="$APP_DIR/$EXPECTED_CLI_RELATIVE_PATH"
CLI_VERSION=""
CLI_VERSION_STATUS=127
CLI_VERSION_OUT="$TMP_DIR/cli-version.txt"
: > "$CLI_VERSION_OUT"
if [ -x "$CLI_PATH" ]; then
  add_check "cli.exists" "pass" "required" "bundled CLI exists and is executable" "$EXPECTED_CLI_RELATIVE_PATH"
  run_capture "$CLI_VERSION_OUT" "$CLI_PATH" version
  CLI_VERSION_STATUS=$RUN_CAPTURE_STATUS
  if [ "$CLI_VERSION_STATUS" -eq 0 ]; then
    CLI_VERSION=$(sed -n '1p' "$CLI_VERSION_OUT")
    add_check "cli.version.command" "pass" "required" "bundled CLI version command succeeded" "$CLI_VERSION"
  else
    add_check "cli.version.command" "blocked" "blocking" "bundled CLI version command failed" "$(tr '\n' ' ' < "$CLI_VERSION_OUT")"
  fi
else
  add_check "cli.exists" "blocked" "blocking" "bundled CLI is missing or not executable" "$EXPECTED_CLI_RELATIVE_PATH"
fi
if [ "$PROVENANCE_LOADED" = "true" ]; then
  if [ "$CLI_VERSION_STATUS" -eq 0 ] && [ "$CLI_VERSION" = "$PROV_CLI_VERSION" ]; then
    add_check "cli.version.provenance" "pass" "required" "bundled CLI version matches provenance" "$CLI_VERSION"
  else
    add_check "cli.version.provenance" "blocked" "blocking" "bundled CLI version does not match provenance" "cli='$CLI_VERSION' provenance='$PROV_CLI_VERSION'"
  fi
fi

jq -n \
  --arg path "$CLI_PATH" \
  --arg relative_path "$EXPECTED_CLI_RELATIVE_PATH" \
  --arg version "$CLI_VERSION" \
  --argjson version_exit "$CLI_VERSION_STATUS" \
  --rawfile version_output "$CLI_VERSION_OUT" \
  '{
    path: $path,
    relative_path: $relative_path,
    version: (if $version == "" then null else $version end),
    version_command: {
      exit_code: $version_exit,
      status: (if $version_exit == 0 then "pass" else "blocked" end),
      output: $version_output
    }
  }' > "$TMP_DIR/cli.json"

hash_one "$INFO_PLIST" "Contents/Info.plist"
hash_one "$APP_ICON_PATH" "Contents/Resources/$APP_ICON_FILE_NAME"
if [ "$PLIST_EXECUTABLE" != "" ]; then
  hash_one "$APP_EXECUTABLE_PATH" "Contents/MacOS/$PLIST_EXECUTABLE"
fi
hash_one "$CLI_PATH" "$EXPECTED_CLI_RELATIVE_PATH"
hash_one "$PROVENANCE_FILE" "Contents/Resources/supermover-provenance.json"
if [ -s "$HASHES_NDJSON" ]; then
  add_check "hashes.basic" "pass" "required" "basic bundle file hashes were recorded" "Info.plist, app icon, app executable, CLI, and provenance are hashed when present."
else
  add_check "hashes.basic" "blocked" "blocking" "no bundle file hashes were recorded" "Expected at least Info.plist, app icon, app executable, CLI, or provenance."
fi

audit_codesign_subject "app" "$APP_DIR" "$TMP_DIR/codesign-app.json"
audit_codesign_subject "cli" "$CLI_PATH" "$TMP_DIR/codesign-cli.json"
audit_spctl "$TMP_DIR/spctl.json"
audit_stapler "$TMP_DIR/stapler.json"
audit_canonical_notarization_sidecar_if_present

jq -s '.' "$CHECKS_NDJSON" > "$TMP_DIR/checks.json"
jq -s '.' "$HASHES_NDJSON" > "$TMP_DIR/hashes.json"

if [ "$BLOCKING_COUNT" -gt 0 ]; then
  AUDIT_STATUS=blocked
  READINESS=blocked
elif [ "$REVIEW_COUNT" -gt 0 ]; then
  AUDIT_STATUS=review_only
  READINESS=review_only
else
  AUDIT_STATUS=pass
  READINESS=distribution_ready
fi

jq -n \
  --arg schema "$EXPECTED_AUDIT_SCHEMA" \
  --arg status "$AUDIT_STATUS" \
  --arg readiness "$READINESS" \
  --arg checked_at "$CHECKED_AT" \
  --arg app_path "$APP_DIR" \
  --arg repo_root "$ROOT_DIR" \
  --arg expected_cli_relative_path "$EXPECTED_CLI_RELATIVE_PATH" \
  --argjson blocking_count "$BLOCKING_COUNT" \
  --argjson review_count "$REVIEW_COUNT" \
  --slurpfile checks "$TMP_DIR/checks.json" \
  --slurpfile plist "$TMP_DIR/plist.json" \
  --slurpfile provenance "$TMP_DIR/provenance.json" \
  --slurpfile git_state "$TMP_DIR/git.json" \
  --slurpfile cli "$TMP_DIR/cli.json" \
  --slurpfile hashes "$TMP_DIR/hashes.json" \
  --slurpfile codesign_app "$TMP_DIR/codesign-app.json" \
  --slurpfile codesign_cli "$TMP_DIR/codesign-cli.json" \
  --slurpfile spctl "$TMP_DIR/spctl.json" \
  --slurpfile stapler "$TMP_DIR/stapler.json" \
  '{
    schema: $schema,
    status: $status,
    readiness: $readiness,
    checked_at: $checked_at,
    app_path: $app_path,
    repo_root: $repo_root,
    summary: {
      pass_ready: ($status == "pass"),
      blocking_checks: $blocking_count,
      review_checks: $review_count,
      expected_cli_relative_path: $expected_cli_relative_path,
      note: (if $status == "pass" then "Developer ID signing, clean provenance, Gatekeeper assessment, and stapled notarization evidence are present." else "This audit is local evidence only and does not prove notarized distribution readiness." end)
    },
    plist: $plist[0],
    provenance: $provenance[0],
    git: $git_state[0],
    cli: $cli[0],
    hashes: $hashes[0],
    signing: {
      codesign: {
        app: $codesign_app[0],
        cli: $codesign_cli[0]
      },
      spctl: $spctl[0],
      stapler: $stapler[0]
    },
    checks: $checks[0]
  }'

if [ "$BLOCKING_COUNT" -gt 0 ]; then
  exit 1
fi
exit 0
