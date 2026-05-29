#!/bin/sh
set -eu
export LC_ALL=C
export LANG=C

ROOT_DIR=$(CDPATH= cd "$(dirname "$0")/../.." && pwd)
DEFAULT_APP_DIR="$ROOT_DIR/macos/dist/SuperMover.app"
DEFAULT_WORK_DIR=""

usage() {
  cat <<'EOF'
usage: notarize-app.sh [--app path/to/SuperMover.app] [--work-dir path]

Environment-based auth (choose exactly one mode):
  SUPERMOVER_NOTARY_KEYCHAIN_PROFILE[+SUPERMOVER_NOTARY_KEYCHAIN_PATH]
  SUPERMOVER_NOTARY_API_KEY + SUPERMOVER_NOTARY_KEY_ID + SUPERMOVER_NOTARY_ISSUER
  SUPERMOVER_NOTARY_APPLE_ID + SUPERMOVER_NOTARY_PASSWORD + SUPERMOVER_NOTARY_TEAM_ID

Optional overrides:
  SUPERMOVER_AUDIT_APP_SCRIPT    path to audit-app.sh
  SUPERMOVER_NOTARY_TIMEOUT      timeout for notarytool submit --wait (default: 30m)
EOF
}

resolve_path() {
  input_path=$1
  case "$input_path" in
    /*) printf '%s\n' "$input_path" ;;
    *)
      parent=$(CDPATH= cd "$(dirname "$input_path")" 2>/dev/null && pwd || printf '%s' "$(pwd)")
      printf '%s/%s\n' "$parent" "$(basename "$input_path")"
      ;;
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

command_output_or_empty() {
  output_path=$1
  if [ -f "$output_path" ]; then
    cat "$output_path"
  fi
}

is_uuid_shaped() {
  value=$1
  case "$value" in
    [0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f]-[0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f]-[0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f]-[0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f]-[0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f])
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

APP_INPUT=$DEFAULT_APP_DIR
WORK_DIR_INPUT=$DEFAULT_WORK_DIR

while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --app)
      APP_INPUT=${2:?missing value for --app}
      shift 2
      ;;
    --work-dir)
      WORK_DIR_INPUT=${2:?missing value for --work-dir}
      shift 2
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

APP_DIR=$(resolve_path "$APP_INPUT")
if [ "$WORK_DIR_INPUT" = "$DEFAULT_WORK_DIR" ] || [ -z "$WORK_DIR_INPUT" ]; then
  WORK_DIR_INPUT="$(dirname "$APP_DIR")/$(basename "$APP_DIR").notary"
fi
WORK_DIR=$(resolve_path "$WORK_DIR_INPUT")
SIDECAR_DIR="$(dirname "$APP_DIR")/$(basename "$APP_DIR").notary"
AUDIT_SCRIPT=${SUPERMOVER_AUDIT_APP_SCRIPT:-"$ROOT_DIR/macos/script/audit-app.sh"}
NOTARY_TIMEOUT=${SUPERMOVER_NOTARY_TIMEOUT:-30m}
CHECKED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

mkdir -p "$WORK_DIR"
mkdir -p "$SIDECAR_DIR"

ARCHIVE_PATH=""
SUBMISSION_PATH="$WORK_DIR/notary-submit.json"
NOTARY_LOG_PATH="$WORK_DIR/notary-log.json"
POST_AUDIT_PATH="$WORK_DIR/post-staple.audit.json"
SIDECAR_NOTARY_LOG_PATH="$SIDECAR_DIR/notary-log.json"
SIDECAR_POST_AUDIT_PATH="$SIDECAR_DIR/post-staple.audit.json"
SUBMIT_OUT="$WORK_DIR/notary-submit.raw"
STAPLE_OUT="$WORK_DIR/stapler-staple.txt"
VALIDATE_OUT="$WORK_DIR/stapler-validate.txt"
AUDIT_STDERR="$WORK_DIR/post-staple.audit.stderr.txt"
RESULT_PATH="$WORK_DIR/notarization.json"
SIDECAR_RESULT_PATH="$SIDECAR_DIR/notarization.json"

AUTH_MODE=""
AUTH_COUNT=0
INCOMPLETE_AUTH=0

KEYCHAIN_PROFILE=${SUPERMOVER_NOTARY_KEYCHAIN_PROFILE:-}
KEYCHAIN_PATH=${SUPERMOVER_NOTARY_KEYCHAIN_PATH:-}
API_KEY=${SUPERMOVER_NOTARY_API_KEY:-}
API_KEY_ID=${SUPERMOVER_NOTARY_KEY_ID:-}
API_ISSUER=${SUPERMOVER_NOTARY_ISSUER:-}
APPLE_ID=${SUPERMOVER_NOTARY_APPLE_ID:-}
APPLE_PASSWORD=${SUPERMOVER_NOTARY_PASSWORD:-}
APPLE_TEAM_ID=${SUPERMOVER_NOTARY_TEAM_ID:-}

if [ -n "$KEYCHAIN_PROFILE" ]; then
  AUTH_COUNT=$((AUTH_COUNT + 1))
  AUTH_MODE=keychain_profile
fi

if [ -n "$API_KEY" ] || [ -n "$API_KEY_ID" ] || [ -n "$API_ISSUER" ]; then
  if [ -n "$API_KEY" ] && [ -n "$API_KEY_ID" ] && [ -n "$API_ISSUER" ]; then
    AUTH_COUNT=$((AUTH_COUNT + 1))
    AUTH_MODE=api_key
  else
    INCOMPLETE_AUTH=1
  fi
fi

if [ -n "$APPLE_ID" ] || [ -n "$APPLE_PASSWORD" ] || [ -n "$APPLE_TEAM_ID" ]; then
  if [ -n "$APPLE_ID" ] && [ -n "$APPLE_PASSWORD" ] && [ -n "$APPLE_TEAM_ID" ]; then
    AUTH_COUNT=$((AUTH_COUNT + 1))
    AUTH_MODE=apple_id
  else
    INCOMPLETE_AUTH=1
  fi
fi

emit_json() {
  result_status=$1
  failure_id=$2
  failure_summary=$3
  failure_detail=$4

  jq -n \
    --arg schema "supermover.macos.notarization.v1" \
    --arg checked_at "$CHECKED_AT" \
    --arg status "$result_status" \
    --arg app_path "$APP_DIR" \
    --arg work_dir "$WORK_DIR" \
    --arg auth_mode "$AUTH_MODE" \
    --arg archive_path "$ARCHIVE_PATH" \
    --arg submission_path "$SUBMISSION_PATH" \
    --arg log_path "$NOTARY_LOG_PATH" \
    --arg post_audit_path "$POST_AUDIT_PATH" \
    --arg failure_id "$failure_id" \
    --arg failure_summary "$failure_summary" \
    --arg failure_detail "$failure_detail" \
    'def maybe_file(path):
       if path != "" and (path | length) > 0 and (path | startswith("/")) and (path | test("."))
       then path
       else null
       end;
     def maybe_text(value):
       if value == "" then null else value end;
     {
       schema: $schema,
       checked_at: $checked_at,
       status: $status,
       app_path: $app_path,
       work_dir: $work_dir,
       auth_mode: maybe_text($auth_mode),
       archive_path: maybe_text($archive_path),
       submission: (
         if ($submission_path | startswith("/")) and (input_filename|length) >= 0 then
           (try (input | fromjson) catch null)
         else null
         end
       ),
       notary_log: null,
       audit: null,
       failure: (
         if $failure_id == "" then null else
           {id: $failure_id, summary: $failure_summary, detail: $failure_detail}
         end
       )
     }' \
    < /dev/null
}

submission_json_or_null() {
  if [ -f "$SUBMISSION_PATH" ]; then
    jq -c . "$SUBMISSION_PATH" 2>/dev/null || printf 'null'
  else
    printf 'null'
  fi
}

notary_log_json_or_null() {
  log_source_path=""
  if [ -f "$SIDECAR_NOTARY_LOG_PATH" ]; then
    log_source_path="$SIDECAR_NOTARY_LOG_PATH"
  elif [ -f "$NOTARY_LOG_PATH" ]; then
    log_source_path="$NOTARY_LOG_PATH"
  fi
  if [ -n "$log_source_path" ]; then
    jq -nc --arg path "$log_source_path" '{path: $path}'
  else
    printf 'null'
  fi
}

audit_json_or_null() {
  audit_source_path=""
  if [ -f "$SIDECAR_POST_AUDIT_PATH" ]; then
    audit_source_path="$SIDECAR_POST_AUDIT_PATH"
  elif [ -f "$POST_AUDIT_PATH" ]; then
    audit_source_path="$POST_AUDIT_PATH"
  fi

  if [ -n "$audit_source_path" ] && [ -f "$audit_source_path" ]; then
    jq -c '{
      path: input_filename,
      status: .status,
      readiness: .readiness,
      pass_ready: (.summary.pass_ready // false),
      blocking_checks: (.summary.blocking_checks // 0)
    }' "$audit_source_path" 2>/dev/null || printf 'null'
  else
    printf 'null'
  fi
}

emit_result() {
  result_status=$1
  failure_id=$2
  failure_summary=$3
  failure_detail=$4
  submission_json=$(submission_json_or_null)
  log_json=$(notary_log_json_or_null)
  audit_json=$(audit_json_or_null)
  jq -n \
    --arg schema "supermover.macos.notarization.v1" \
    --arg checked_at "$CHECKED_AT" \
    --arg status "$result_status" \
    --arg app_path "$APP_DIR" \
    --arg work_dir "$WORK_DIR" \
    --arg auth_mode "$AUTH_MODE" \
    --arg archive_path "$ARCHIVE_PATH" \
    --arg failure_id "$failure_id" \
    --arg failure_summary "$failure_summary" \
    --arg failure_detail "$failure_detail" \
    --argjson submission "$submission_json" \
    --argjson notary_log "$log_json" \
    --argjson audit "$audit_json" \
    '{
      schema: $schema,
      checked_at: $checked_at,
      status: $status,
      app_path: $app_path,
      work_dir: $work_dir,
      auth_mode: (if $auth_mode == "" then null else $auth_mode end),
      archive_path: (if $archive_path == "" then null else $archive_path end),
      submission: (
        if $submission == null then null else
          $submission + {path: $submission.path // null}
        end
      ),
      notary_log: $notary_log,
      audit: $audit,
      failure: (
        if $failure_id == "" then null else
          {id: $failure_id, summary: $failure_summary, detail: $failure_detail}
        end
      )
    }'
}

fail_closed() {
  failure_id=$1
  failure_summary=$2
  failure_detail=$3
  if [ "$RESULT_PATH" = "$SIDECAR_RESULT_PATH" ]; then
    emit_result blocked "$failure_id" "$failure_summary" "$failure_detail" | tee "$RESULT_PATH"
  else
    emit_result blocked "$failure_id" "$failure_summary" "$failure_detail" | tee "$RESULT_PATH" "$SIDECAR_RESULT_PATH"
  fi
  exit 1
}

fail_closed_without_jq() {
  json='{"schema":"supermover.macos.notarization.v1","status":"blocked","failure":{"id":"missing_tool_jq","summary":"jq is required","detail":"Install jq to run the structured notarization workflow."}}'
  if [ "$RESULT_PATH" = "$SIDECAR_RESULT_PATH" ]; then
    printf '%s\n' "$json" | tee "$RESULT_PATH"
  else
    printf '%s\n' "$json" | tee "$RESULT_PATH" "$SIDECAR_RESULT_PATH"
  fi
  exit 1
}

if ! command -v jq >/dev/null 2>&1; then
  fail_closed_without_jq
fi

if [ ! -d "$APP_DIR" ]; then
  fail_closed missing_app "app bundle is missing" "$APP_DIR"
fi
if [ ! -x "$AUDIT_SCRIPT" ]; then
  fail_closed missing_audit_script "audit-app.sh is unavailable" "$AUDIT_SCRIPT"
fi
if ! command -v ditto >/dev/null 2>&1; then
  fail_closed missing_ditto "ditto is unavailable" "A zip archive is required for notarytool submission."
fi
if ! command -v xcrun >/dev/null 2>&1; then
  fail_closed missing_xcrun "xcrun is unavailable" "xcrun is required for notarytool and stapler."
fi
if [ "$INCOMPLETE_AUTH" -eq 1 ]; then
  fail_closed incomplete_credentials "notary credentials are incomplete" "Provide a full keychain profile, API key triplet, or Apple ID triplet."
fi
if [ "$AUTH_COUNT" -eq 0 ]; then
  fail_closed missing_credentials "notary credentials are missing" "Set SUPERMOVER_NOTARY_KEYCHAIN_PROFILE, a full API key triplet, or an Apple ID triplet."
fi
if [ "$AUTH_COUNT" -gt 1 ]; then
  fail_closed ambiguous_credentials "multiple notary auth modes are configured" "Configure exactly one authentication mode before submitting a release app."
fi

app_name=$(basename "$APP_DIR")
ARCHIVE_PATH="$WORK_DIR/$app_name.zip"
run_capture "$WORK_DIR/ditto.txt" ditto -c -k --keepParent "$APP_DIR" "$ARCHIVE_PATH"
if [ "$RUN_CAPTURE_STATUS" -ne 0 ] || [ ! -f "$ARCHIVE_PATH" ]; then
  fail_closed archive_failed "app archive creation failed" "$(command_output_or_empty "$WORK_DIR/ditto.txt")"
fi

submit_notarytool() {
  case "$AUTH_MODE" in
    keychain_profile)
      if [ -n "$KEYCHAIN_PATH" ]; then
        xcrun notarytool submit -p "$KEYCHAIN_PROFILE" --keychain "$KEYCHAIN_PATH" --wait --timeout "$NOTARY_TIMEOUT" --output-format json "$ARCHIVE_PATH"
      else
        xcrun notarytool submit -p "$KEYCHAIN_PROFILE" --wait --timeout "$NOTARY_TIMEOUT" --output-format json "$ARCHIVE_PATH"
      fi
      ;;
    api_key)
      xcrun notarytool submit -k "$API_KEY" -d "$API_KEY_ID" -i "$API_ISSUER" --wait --timeout "$NOTARY_TIMEOUT" --output-format json "$ARCHIVE_PATH"
      ;;
    apple_id)
      xcrun notarytool submit --apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID" --wait --timeout "$NOTARY_TIMEOUT" --output-format json "$ARCHIVE_PATH"
      ;;
  esac
}

run_capture "$SUBMIT_OUT" submit_notarytool
if [ "$RUN_CAPTURE_STATUS" -ne 0 ]; then
  fail_closed submit_failed "notary submission failed" "$(command_output_or_empty "$SUBMIT_OUT")"
fi
cp "$SUBMIT_OUT" "$SUBMISSION_PATH"

submission_id=$(jq -r '.id // ""' "$SUBMISSION_PATH" 2>/dev/null || printf '')
submission_status=$(jq -r '.status // ""' "$SUBMISSION_PATH" 2>/dev/null || printf '')
submission_message=$(jq -r '.message // ""' "$SUBMISSION_PATH" 2>/dev/null || printf '')
if [ -z "$submission_id" ] || ! is_uuid_shaped "$submission_id"; then
  fail_closed malformed_submit_response "notary submission output is malformed" "$(command_output_or_empty "$SUBMISSION_PATH")"
fi

fetch_notary_log() {
  case "$AUTH_MODE" in
    keychain_profile)
      if [ -n "$KEYCHAIN_PATH" ]; then
        xcrun notarytool log -p "$KEYCHAIN_PROFILE" --keychain "$KEYCHAIN_PATH" "$submission_id" "$NOTARY_LOG_PATH"
      else
        xcrun notarytool log -p "$KEYCHAIN_PROFILE" "$submission_id" "$NOTARY_LOG_PATH"
      fi
      ;;
    api_key)
      xcrun notarytool log -k "$API_KEY" -d "$API_KEY_ID" -i "$API_ISSUER" "$submission_id" "$NOTARY_LOG_PATH"
      ;;
    apple_id)
      xcrun notarytool log --apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID" "$submission_id" "$NOTARY_LOG_PATH"
      ;;
  esac
}

run_capture "$WORK_DIR/notary-log.stdout" fetch_notary_log
if [ "$RUN_CAPTURE_STATUS" -ne 0 ]; then
  fail_closed log_failed "notary log retrieval failed" "$(command_output_or_empty "$WORK_DIR/notary-log.stdout")"
fi

if [ "$submission_status" != "Accepted" ]; then
  fail_closed notary_rejected "notary submission did not reach Accepted" "${submission_message:-$submission_status}"
fi
if ! jq -e --arg submission_id "$submission_id" '
  .status == "Accepted"
  and ((.issues == null) or (.issues | type == "array"))
  and (.jobId | type == "string")
  and (($submission_id | gsub("^\\s+|\\s+$"; "") | ascii_downcase) == (.jobId | gsub("^\\s+|\\s+$"; "") | ascii_downcase))
  and (($submission_id | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))
  and ((.jobId | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))
' "$NOTARY_LOG_PATH" >/dev/null 2>&1; then
  fail_closed notary_log_not_accepted "notary log is not accepted" "$(command_output_or_empty "$NOTARY_LOG_PATH")"
fi

run_capture "$STAPLE_OUT" xcrun stapler staple "$APP_DIR"
if [ "$RUN_CAPTURE_STATUS" -ne 0 ]; then
  fail_closed stapler_staple_failed "stapler staple failed" "$(command_output_or_empty "$STAPLE_OUT")"
fi

run_capture "$VALIDATE_OUT" xcrun stapler validate "$APP_DIR"
if [ "$RUN_CAPTURE_STATUS" -ne 0 ]; then
  fail_closed stapler_validate_failed "stapler validate failed" "$(command_output_or_empty "$VALIDATE_OUT")"
fi

# Replace any previous sibling result before the post-staple audit so stale
# currentness evidence cannot block the newly stapled app from producing its
# fresh notarization sidecar.
rm -f "$SIDECAR_RESULT_PATH"
rm -f "$SIDECAR_POST_AUDIT_PATH"
rm -f "$SIDECAR_NOTARY_LOG_PATH"

set +e
"$AUDIT_SCRIPT" "$APP_DIR" > "$POST_AUDIT_PATH" 2> "$AUDIT_STDERR"
AUDIT_EXIT=$?
set -e

if [ -f "$POST_AUDIT_PATH" ] && [ "$POST_AUDIT_PATH" != "$SIDECAR_POST_AUDIT_PATH" ]; then
  cp "$POST_AUDIT_PATH" "$SIDECAR_POST_AUDIT_PATH"
fi
if [ -f "$NOTARY_LOG_PATH" ] && [ "$NOTARY_LOG_PATH" != "$SIDECAR_NOTARY_LOG_PATH" ]; then
  cp "$NOTARY_LOG_PATH" "$SIDECAR_NOTARY_LOG_PATH"
fi

if ! jq -e '.schema == "supermover.macos.app_audit.v1"' "$POST_AUDIT_PATH" >/dev/null 2>&1; then
  fail_closed malformed_post_staple_audit "post-staple audit output is malformed" "$(command_output_or_empty "$POST_AUDIT_PATH")"
fi

audit_status=$(jq -r '.status // ""' "$POST_AUDIT_PATH")
audit_readiness=$(jq -r '.readiness // ""' "$POST_AUDIT_PATH")
audit_pass_ready=$(jq -r '.summary.pass_ready // false' "$POST_AUDIT_PATH")

if [ "$AUDIT_EXIT" -ne 0 ] || [ "$audit_status" != "pass" ] || [ "$audit_readiness" != "distribution_ready" ] || [ "$audit_pass_ready" != "true" ]; then
  fail_closed post_staple_audit_blocked "post-staple app audit is still blocked" "$(command_output_or_empty "$POST_AUDIT_PATH")"
fi

if [ "$RESULT_PATH" = "$SIDECAR_RESULT_PATH" ]; then
  emit_result pass "" "" "" | tee "$RESULT_PATH"
else
  emit_result pass "" "" "" | tee "$RESULT_PATH" "$SIDECAR_RESULT_PATH"
fi
exit 0
