acceptance_require_flag() {
  value=$1
  name=$2
  if [ -z "$value" ]; then
    printf 'missing required %s\n' "$name" >&2
    exit 2
  fi
}

acceptance_cleanup_pid() {
  pid=$1
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
}

acceptance_write_tls_identity() {
  name=$1
  cert_path=$2
  key_path=$3
  openssl req -x509 -newkey ed25519 -nodes \
    -keyout "$key_path" \
    -out "$cert_path" \
    -days 365 \
    -subj "/CN=$name" >/dev/null 2>&1
  chmod 600 "$cert_path" "$key_path"
}

acceptance_reserve_local_port() {
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

acceptance_wait_for_file() {
  wait_file_path=$1
  wait_file_timeout=$2
  wait_file_i=0
  while [ "$wait_file_i" -lt "$wait_file_timeout" ]; do
    if [ -s "$wait_file_path" ]; then
      return 0
    fi
    sleep 1
    wait_file_i=$((wait_file_i + 1))
  done
  return 1
}

acceptance_wait_for_json_field() {
  wait_json_path=$1
  wait_json_field=$2
  wait_json_timeout=$3
  wait_json_i=0
  while [ "$wait_json_i" -lt "$wait_json_timeout" ]; do
    if [ -s "$wait_json_path" ] && jq -e "$wait_json_field | strings | length > 0" "$wait_json_path" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    wait_json_i=$((wait_json_i + 1))
  done
  return 1
}

acceptance_wait_for_serve_ready() {
  wait_ready_path=$1
  wait_ready_timeout=$2
  wait_ready_require_verification=$3
  wait_ready_require_receiver=$4
  if ! acceptance_wait_for_json_field "$wait_ready_path" '.address // ""' "$wait_ready_timeout"; then
    return 1
  fi
  if ! acceptance_wait_for_json_field "$wait_ready_path" '.mode // ""' "$wait_ready_timeout"; then
    return 1
  fi
  if [ "$wait_ready_require_verification" = "1" ]; then
    if ! acceptance_wait_for_json_field "$wait_ready_path" '.verification_code // ""' "$wait_ready_timeout"; then
      return 1
    fi
  fi
  if [ "$wait_ready_require_receiver" = "1" ]; then
    if ! acceptance_wait_for_json_field "$wait_ready_path" '.receiver_address // ""' "$wait_ready_timeout"; then
      return 1
    fi
    wait_ready_i=0
    while [ "$wait_ready_i" -lt "$wait_ready_timeout" ]; do
      if jq -e '.receiver_routes == true and .push_network == true and .trusted == true and .transfer == true' "$wait_ready_path" >/dev/null 2>&1; then
        return 0
      fi
      sleep 1
      wait_ready_i=$((wait_ready_i + 1))
    done
    return 1
  fi
  return 0
}

acceptance_json_get() {
  file=$1
  expr=$2
  jq -r "$expr" "$file"
}

acceptance_require_regular_file() {
  path=$1
  label=$2
  if ! python3 - "$path" "$label" <<'PY'
import os
import sys

path, label = sys.argv[1:3]
if not os.path.exists(path):
    sys.stderr.write(f"missing {label}: {path}\n")
    sys.exit(1)
if os.path.islink(path) or not os.path.isfile(path) or os.stat(path).st_nlink != 1:
    sys.stderr.write(f"invalid {label}: {path}\n")
    sys.exit(1)
PY
  then
    exit 1
  fi
}

acceptance_ensure_bundle_root() {
  bundle_root=$1
  mkdir -p "$bundle_root"
  if [ -e "$bundle_root/meta.json" ] || [ -L "$bundle_root/meta.json" ]; then
    acceptance_require_regular_file "$bundle_root/meta.json" "bundle meta"
    return
  fi
  jq -n '{
    schema: "supermover.acceptance.two_machine.v1",
    status: "in_progress",
    collection: {
      mode: "unknown",
      machine_count: 0
    },
    roles: {},
    evidence: {}
  }' > "$bundle_root/meta.json"
  acceptance_require_regular_file "$bundle_root/meta.json" "bundle meta"
}

acceptance_lock_dir() {
  bundle_root=$1
  printf '%s\n' "$bundle_root/.meta.lock"
}

acceptance_acquire_lock() {
  lock_dir=$1
  timeout_seconds=$2
  i=0
  while ! mkdir "$lock_dir" 2>/dev/null; do
    if [ "$i" -ge "$timeout_seconds" ]; then
      printf 'timed out waiting for acceptance lock: %s\n' "$lock_dir" >&2
      exit 5
    fi
    sleep 1
    i=$((i + 1))
  done
}

acceptance_release_lock() {
  lock_dir=$1
  rmdir "$lock_dir" 2>/dev/null || true
}

acceptance_update_bundle_meta_locked() {
  bundle_root=$1
  shift
  tmp_path="$bundle_root/.meta.tmp"
  lock_dir=$(acceptance_lock_dir "$bundle_root")
  acceptance_acquire_lock "$lock_dir" 30
  set +e
  jq "$@" "$bundle_root/meta.json" > "$tmp_path"
  status=$?
  if [ "$status" -eq 0 ]; then
    mv "$tmp_path" "$bundle_root/meta.json"
    status=$?
  fi
  set -e
  acceptance_release_lock "$lock_dir"
  if [ "$status" -ne 0 ]; then
    rm -f "$tmp_path"
    exit "$status"
  fi
}

acceptance_record_cli_facts() {
  sm_bin=$1
  app_dir=$2
  bundle_root=$3
  role=$4
  version_output_path="$bundle_root/$role.version.txt"
  provenance_output_path="$bundle_root/$role.provenance.json"
  acceptance_require_safe_packaging_output_leaf "$version_output_path" "$role"
  acceptance_require_safe_packaging_output_leaf "$provenance_output_path" "$role"
  version_output_tmp="$bundle_root/.$role.version.txt.tmp.$$"
  provenance_output_tmp="$bundle_root/.$role.provenance.json.tmp.$$"
  rm -f "$version_output_tmp"
  rm -f "$provenance_output_tmp"
  "$sm_bin" version > "$version_output_tmp"
  cp "$app_dir/Contents/Resources/supermover-provenance.json" "$provenance_output_tmp"
  mv -f "$version_output_tmp" "$version_output_path"
  mv -f "$provenance_output_tmp" "$provenance_output_path"
}

acceptance_record_machine_cli_facts() {
  sm_bin=$1
  app_dir=$2
  bundle_root=$3
  machine=$4
  acceptance_record_cli_facts "$sm_bin" "$app_dir" "$bundle_root" "$machine"
}

acceptance_app_audit_matches_current_app() {
  audit_output_path=$1
  app_dir=$2
  provenance_path="$app_dir/Contents/Resources/supermover-provenance.json"
  if [ -L "$audit_output_path" ] || [ ! -f "$audit_output_path" ] || [ ! -f "$provenance_path" ]; then
    return 1
  fi
  if ! jq -e '.schema == "supermover.macos.app_audit.v1" and (.app_path | type == "string") and (.app_path | length > 0)' "$audit_output_path" >/dev/null 2>&1; then
    return 1
  fi
  if ! jq -e '.schema == "supermover.macos.provenance.v1"' "$provenance_path" >/dev/null 2>&1; then
    return 1
  fi
  audit_app_path=$(jq -r '.app_path // ""' "$audit_output_path" 2>/dev/null || printf '')
  if [ -z "$audit_app_path" ] || [ "$audit_app_path" != "$app_dir" ]; then
    return 1
  fi
  current_manifest=$(jq -S -c '.' "$provenance_path" 2>/dev/null || printf '')
  audit_manifest=$(jq -S -c '.provenance.manifest' "$audit_output_path" 2>/dev/null || printf '')
  if [ -z "$current_manifest" ] || [ -z "$audit_manifest" ] || [ "$audit_manifest" = "null" ] || [ "$audit_manifest" != "$current_manifest" ]; then
    return 1
  fi
  return 0
}

acceptance_record_app_audit() {
  audit_script=$1
  app_dir=$2
  bundle_root=$3
  machine=$4
  collected_by=$5
  if [ ! -x "$audit_script" ]; then
    printf 'missing app audit helper: %s\n' "$audit_script" >&2
    exit 5
  fi
  acceptance_ensure_bundle_root "$bundle_root"
  output_rel="$machine.app-audit.json"
  output_path="$bundle_root/$output_rel"
  acceptance_require_safe_packaging_output_leaf "$bundle_root/$machine.version.txt" "$machine"
  acceptance_require_safe_packaging_output_leaf "$bundle_root/$machine.provenance.json" "$machine"
  acceptance_require_safe_packaging_output_leaf "$output_path" "$machine"
  acceptance_record_machine_cli_facts "$app_dir/Contents/Resources/bin/supermover" "$app_dir" "$bundle_root" "$machine"
  if acceptance_app_audit_matches_current_app "$output_path" "$app_dir"; then
    existing_collected_by=$(jq -r --arg machine "$machine" '.evidence.app_audit[$machine].collected_by // ""' "$bundle_root/meta.json" 2>/dev/null || printf '')
    if [ -n "$existing_collected_by" ]; then
      collected_by="$existing_collected_by"
    fi
    audit_status=$(jq -r '.status // "blocked"' "$output_path")
    audit_readiness=$(jq -r '.readiness // "blocked"' "$output_path")
    audit_blocking_checks=$(jq -r '.summary.blocking_checks // 0' "$output_path")
    audit_pass_ready=$(jq -r '.summary.pass_ready // false' "$output_path")
    if [ "$audit_status" = "pass" ] && [ "$audit_readiness" = "distribution_ready" ] && [ "$audit_pass_ready" = "true" ]; then
      audit_exit=0
    else
      audit_exit=1
    fi
    acceptance_update_bundle_meta_locked "$bundle_root" \
      --arg machine "$machine" \
      --arg collected_by "$collected_by" \
      --arg output "$output_rel" \
      --arg status "$audit_status" \
      --arg readiness "$audit_readiness" \
      --arg pass_ready "$audit_pass_ready" \
      --argjson exit_code "$audit_exit" \
      --argjson blocking_checks "$audit_blocking_checks" \
      '.evidence.app_audit = (.evidence.app_audit // {})
      | .evidence.app_audit[$machine] = {
          collected_by: $collected_by,
          output: $output,
          exit_code: $exit_code,
          status: $status,
          readiness: $readiness,
          pass_ready: ($pass_ready == "true"),
          blocking_checks: $blocking_checks
        }'
    return 0
  fi
  output_tmp="$bundle_root/.$machine.app-audit.json.tmp.$$"
  rm -f "$output_tmp"
  set +e
  "$audit_script" "$app_dir" > "$output_tmp"
  audit_exit=$?
  set -e
  if [ "$audit_exit" -ne 0 ] && [ "$audit_exit" -ne 1 ]; then
    rm -f "$output_tmp"
    printf 'app audit failed unexpectedly for %s (exit %s): %s\n' "$machine" "$audit_exit" "$output_path" >&2
    exit 5
  fi
  if ! jq -e '.schema == "supermover.macos.app_audit.v1" and (.status | type == "string") and (.readiness | type == "string")' "$output_tmp" >/dev/null 2>&1; then
    rm -f "$output_tmp"
    printf 'app audit output malformed for %s: %s\n' "$machine" "$output_path" >&2
    exit 5
  fi
  mv -f "$output_tmp" "$output_path"
  audit_status=$(jq -r '.status // "blocked"' "$output_path")
  audit_readiness=$(jq -r '.readiness // "blocked"' "$output_path")
  audit_blocking_checks=$(jq -r '.summary.blocking_checks // 0' "$output_path")
  audit_pass_ready=$(jq -r '.summary.pass_ready // false' "$output_path")
  acceptance_update_bundle_meta_locked "$bundle_root" \
    --arg machine "$machine" \
    --arg collected_by "$collected_by" \
    --arg output "$output_rel" \
    --arg status "$audit_status" \
    --arg readiness "$audit_readiness" \
    --arg pass_ready "$audit_pass_ready" \
    --argjson exit_code "$audit_exit" \
    --argjson blocking_checks "$audit_blocking_checks" \
    '.evidence.app_audit = (.evidence.app_audit // {})
    | .evidence.app_audit[$machine] = {
        collected_by: $collected_by,
        output: $output,
        exit_code: $exit_code,
        status: $status,
        readiness: $readiness,
        pass_ready: ($pass_ready == "true"),
        blocking_checks: $blocking_checks
      }'
}

acceptance_default_app_audit_executable() {
  app_dir=$1
  bundled_helper="$app_dir/Contents/Resources/bin/supermover-app-audit"
  if [ -x "$bundled_helper" ]; then
    printf '%s\n' "$bundled_helper"
    return 0
  fi
  printf '%s\n' "$(CDPATH= cd "$(dirname "$0")/.." && pwd)/script/audit-app.sh"
}

acceptance_bundle_collection_mode() {
  bundle_root=$1
  jq -r '.collection.mode // "unknown"' "$bundle_root/meta.json"
}

acceptance_notarization_regular_artifact_is_safe() {
  artifact_path=$1
  python3 - "$artifact_path" <<'PY'
import os
import sys

path = sys.argv[1]
if not os.path.exists(path):
    sys.exit(1)
if os.path.islink(path) or not os.path.isfile(path) or os.stat(path).st_nlink != 1:
    sys.exit(1)
PY
}

acceptance_require_safe_packaging_output_leaf() {
  artifact_path=$1
  machine=$2
  if [ -e "$artifact_path" ] || [ -L "$artifact_path" ]; then
    if ! acceptance_notarization_regular_artifact_is_safe "$artifact_path"; then
      printf 'unsafe %s packaging evidence: %s\n' "$machine" "$artifact_path" >&2
      exit 5
    fi
  fi
}

acceptance_notarization_directory_is_safe() {
  artifact_path=$1
  python3 - "$artifact_path" <<'PY'
import os
import sys

path = sys.argv[1]
if not os.path.exists(path):
    sys.exit(1)
if os.path.islink(path) or not os.path.isdir(path):
    sys.exit(1)
PY
}

acceptance_notarization_source_is_safe() {
  notary_source_path=$1
  notary_source_dir=$(dirname "$notary_source_path")
  if ! acceptance_notarization_directory_is_safe "$notary_source_dir"; then
    return 1
  fi
  if ! acceptance_notarization_regular_artifact_is_safe "$notary_source_path"; then
    return 1
  fi
  return 0
}

acceptance_notary_log_is_accepted() {
  notary_log_path=$1
  submission_id=${2:-}
  jq -e --arg submission_id "$submission_id" '
    def lower_ascii: ascii_downcase;
    .status == "Accepted"
    and ((.issues == null) or (.issues | type == "array"))
    and (.jobId | type == "string")
    and (($submission_id | gsub("^\\s+|\\s+$"; "") | lower_ascii) == (.jobId | gsub("^\\s+|\\s+$"; "") | lower_ascii))
    and (($submission_id | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))
    and ((.jobId | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))
  ' "$notary_log_path" >/dev/null 2>&1
}

acceptance_notarization_evidence_matches_current_app() {
  notary_source_path=$1
  app_dir=$2
  provenance_path="$app_dir/Contents/Resources/supermover-provenance.json"
  notary_source_dir=$(dirname "$notary_source_path")
  expected_audit_path="$notary_source_dir/post-staple.audit.json"

  if ! acceptance_notarization_source_is_safe "$notary_source_path"; then
    return 1
  fi
  if [ ! -f "$provenance_path" ]; then
    return 1
  fi
  if ! jq -e '.schema == "supermover.macos.provenance.v1"' "$provenance_path" >/dev/null 2>&1; then
    return 1
  fi

  audit_path=$(jq -r '.audit.path // ""' "$notary_source_path" 2>/dev/null || printf '')
  notary_log_path=$(jq -r '.notary_log.path // ""' "$notary_source_path" 2>/dev/null || printf '')
  submission_id=$(jq -r '.submission.id // ""' "$notary_source_path" 2>/dev/null || printf '')
  if [ -z "$audit_path" ] || [ "$audit_path" != "$expected_audit_path" ] || [ ! -f "$audit_path" ]; then
    return 1
  fi
  expected_notary_log_path="$notary_source_dir/notary-log.json"
  if [ -z "$notary_log_path" ] || [ "$notary_log_path" != "$expected_notary_log_path" ] || [ ! -f "$notary_log_path" ]; then
    return 1
  fi
  if ! acceptance_notarization_regular_artifact_is_safe "$audit_path"; then
    return 1
  fi
  if ! acceptance_notarization_regular_artifact_is_safe "$notary_log_path"; then
    return 1
  fi
  if ! acceptance_notary_log_is_accepted "$notary_log_path" "$submission_id"; then
    return 1
  fi
  if ! jq -e '.schema == "supermover.macos.app_audit.v1"' "$audit_path" >/dev/null 2>&1; then
    return 1
  fi

  notary_app_path=$(jq -r '.app_path // ""' "$notary_source_path" 2>/dev/null || printf '')
  audit_app_path=$(jq -r '.app_path // ""' "$audit_path" 2>/dev/null || printf '')
  if [ -z "$notary_app_path" ] || [ "$notary_app_path" != "$app_dir" ]; then
    return 1
  fi
  if [ -z "$audit_app_path" ] || [ "$audit_app_path" != "$app_dir" ]; then
    return 1
  fi

  audit_status=$(jq -r '.status // ""' "$audit_path" 2>/dev/null || printf '')
  audit_readiness=$(jq -r '.readiness // ""' "$audit_path" 2>/dev/null || printf '')
  audit_pass_ready=$(jq -r '.summary.pass_ready // false' "$audit_path" 2>/dev/null || printf 'false')
  notary_audit_status=$(jq -r '.audit.status // ""' "$notary_source_path" 2>/dev/null || printf '')
  notary_audit_readiness=$(jq -r '.audit.readiness // ""' "$notary_source_path" 2>/dev/null || printf '')
  notary_audit_pass_ready=$(jq -r '.audit.pass_ready // false' "$notary_source_path" 2>/dev/null || printf 'false')
  if [ "$audit_status" != "$notary_audit_status" ] || [ "$audit_readiness" != "$notary_audit_readiness" ] || [ "$audit_pass_ready" != "$notary_audit_pass_ready" ]; then
    return 1
  fi

  current_manifest=$(jq -S -c '.' "$provenance_path" 2>/dev/null || printf '')
  audit_manifest=$(jq -S -c '.provenance.manifest' "$audit_path" 2>/dev/null || printf '')
  if [ -z "$current_manifest" ] || [ -z "$audit_manifest" ] || [ "$audit_manifest" = "null" ] || [ "$audit_manifest" != "$current_manifest" ]; then
    return 1
  fi

  return 0
}

acceptance_record_notarization_evidence_if_present() {
  bundle_root=$1
  machine=$2
  app_dir=$3
  collected_by=${4:-phase-preflight}
  app_name=$(basename "$app_dir")
  notary_source_dir="$(dirname "$app_dir")/$app_name.notary"
  notary_source_path="$notary_source_dir/notarization.json"
  notary_log_source_path="$notary_source_dir/notary-log.json"
  notary_output_path="$bundle_root/$machine.notarization.json"
  notary_log_output_path="$bundle_root/$machine.notary-log.json"
  if [ ! -f "$notary_source_path" ] && [ ! -L "$notary_source_path" ]; then
    return 1
  fi
  if ! acceptance_notarization_source_is_safe "$notary_source_path"; then
    acceptance_clear_stale_notarization_evidence "$bundle_root" "$machine"
    printf 'unsafe %s notarization evidence: %s\n' "$machine" "$notary_source_path" >&2
    exit 5
  fi
  if ! jq -e '.schema == "supermover.macos.notarization.v1" and (.status | type == "string") and (.status | length > 0)' "$notary_source_path" >/dev/null 2>&1; then
    acceptance_clear_stale_notarization_evidence "$bundle_root" "$machine"
    printf 'malformed %s notarization evidence: %s\n' "$machine" "$notary_source_path" >&2
    exit 5
  fi
  audit_path=$(jq -r '.audit.path // ""' "$notary_source_path" 2>/dev/null || printf '')
  expected_audit_path="$notary_source_dir/post-staple.audit.json"
  if [ -n "$audit_path" ] && [ "$audit_path" = "$expected_audit_path" ] && ! acceptance_notarization_regular_artifact_is_safe "$audit_path"; then
    acceptance_clear_stale_notarization_evidence "$bundle_root" "$machine"
    printf 'unsafe %s notarization evidence: %s\n' "$machine" "$audit_path" >&2
    exit 5
  fi
  if ! acceptance_notarization_regular_artifact_is_safe "$notary_log_source_path"; then
    acceptance_clear_stale_notarization_evidence "$bundle_root" "$machine"
    printf 'unsafe %s notarization evidence: %s\n' "$machine" "$notary_log_source_path" >&2
    exit 5
  fi
  submission_id=$(jq -r '.submission.id // ""' "$notary_source_path" 2>/dev/null || printf '')
  if ! acceptance_notary_log_is_accepted "$notary_log_source_path" "$submission_id"; then
    acceptance_clear_stale_notarization_evidence "$bundle_root" "$machine"
    printf '%s.notary-log.json is not accepted notarization log evidence: %s\n' "$machine" "$notary_log_source_path" >&2
    exit 5
  fi
  if ! acceptance_notarization_evidence_matches_current_app "$notary_source_path" "$app_dir"; then
    acceptance_clear_stale_notarization_evidence "$bundle_root" "$machine"
    printf 'stale %s notarization evidence: %s\n' "$machine" "$notary_source_path" >&2
    exit 5
  fi
  if [ -e "$notary_output_path" ] || [ -L "$notary_output_path" ]; then
    if ! acceptance_notarization_regular_artifact_is_safe "$notary_output_path"; then
      acceptance_clear_stale_notarization_evidence "$bundle_root" "$machine"
      printf 'unsafe %s notarization evidence: %s\n' "$machine" "$notary_output_path" >&2
      exit 5
    fi
  fi
  if [ -e "$notary_log_output_path" ] || [ -L "$notary_log_output_path" ]; then
    if ! acceptance_notarization_regular_artifact_is_safe "$notary_log_output_path"; then
      acceptance_clear_stale_notarization_evidence "$bundle_root" "$machine"
      printf 'unsafe %s notarization evidence: %s\n' "$machine" "$notary_log_output_path" >&2
      exit 5
    fi
  fi
  notary_output_tmp="$bundle_root/.$machine.notarization.json.tmp.$$"
  notary_log_output_tmp="$bundle_root/.$machine.notary-log.json.tmp.$$"
  rm -f "$notary_output_tmp"
  rm -f "$notary_log_output_tmp"
  cp "$notary_log_source_path" "$notary_log_output_tmp"
  jq --arg notary_log "$machine.notary-log.json" '.notary_log.path = $notary_log' "$notary_source_path" > "$notary_output_tmp"
  mv -f "$notary_log_output_tmp" "$notary_log_output_path"
  mv -f "$notary_output_tmp" "$notary_output_path"
  acceptance_update_bundle_meta_locked "$bundle_root" \
    --arg machine "$machine" \
    --arg output "$machine.notarization.json" \
    --arg notary_log "$machine.notary-log.json" \
    --arg collected_by "$collected_by" \
    --arg status "$(jq -r '.status // ""' "$notary_output_path")" \
    --arg audit_status "$(jq -r '.audit.status // ""' "$notary_output_path")" \
    --arg audit_readiness "$(jq -r '.audit.readiness // ""' "$notary_output_path")" \
    --arg audit_pass_ready "$(jq -r '.audit.pass_ready // false' "$notary_output_path")" \
    '.evidence.notarization = (.evidence.notarization // {})
    | .evidence.notarization[$machine] = {
        collected_by: $collected_by,
        output: $output,
        notary_log: $notary_log,
        status: $status,
        audit_status: $audit_status,
        audit_readiness: $audit_readiness,
        audit_pass_ready: ($audit_pass_ready == "true")
      }'
  return 0
}

acceptance_clear_stale_notarization_evidence() {
  bundle_root=$1
  machine=$2
  notary_output_path="$bundle_root/$machine.notarization.json"
  notary_log_output_path="$bundle_root/$machine.notary-log.json"
  rm -f "$notary_output_path"
  rm -f "$notary_log_output_path"
  acceptance_update_bundle_meta_locked "$bundle_root" \
    --arg machine "$machine" \
    '.evidence.notarization = (.evidence.notarization // {})
    | .evidence.notarization |= with_entries(select(.key != $machine))
    | if (.evidence.notarization | length) == 0 then
        del(.evidence.notarization)
      else
        .
      end'
}

acceptance_require_ready_app_audit_for_collection() {
  bundle_root=$1
  machine=$2
  app_dir=$3
  collected_by=${4:-phase-preflight}
  collection_mode=$(acceptance_bundle_collection_mode "$bundle_root")
  app_name=$(basename "$app_dir")
  notary_source_dir="$(dirname "$app_dir")/$app_name.notary"
  notary_source_path="$notary_source_dir/notarization.json"
  notary_output_path="$bundle_root/$machine.notarization.json"

  if ! acceptance_record_notarization_evidence_if_present "$bundle_root" "$machine" "$app_dir" "$collected_by"; then
    acceptance_clear_stale_notarization_evidence "$bundle_root" "$machine"
  fi

  if [ "$collection_mode" != "two_machine" ]; then
    return 0
  fi
  output_path="$bundle_root/$machine.app-audit.json"
  if ! jq -e '.schema == "supermover.macos.app_audit.v1" and .status == "pass" and .readiness == "distribution_ready" and .summary.pass_ready == true' "$output_path" >/dev/null 2>&1; then
    printf 'installed-app acceptance requires install-ready %s app audit before phase execution in collection.mode=two_machine: %s\n' "$machine" "$output_path" >&2
    exit 5
  fi
  if [ ! -f "$notary_source_path" ]; then
    printf 'installed-app acceptance requires release-ready %s notarization evidence before phase execution in collection.mode=two_machine: %s\n' "$machine" "$notary_source_path" >&2
    exit 5
  fi
  if ! jq -e '
    .schema == "supermover.macos.notarization.v1"
    and .status == "pass"
    and (.submission.id | type == "string") and ((.submission.id | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))
    and .submission.status == "Accepted"
    and (.auth_mode == "keychain_profile" or .auth_mode == "api_key" or .auth_mode == "apple_id")
    and (.failure == null)
    and (.notary_log.path | type == "string") and ((.notary_log.path | gsub("^\\s+|\\s+$"; "")) | length > 0)
    and .audit.status == "pass"
    and .audit.readiness == "distribution_ready"
    and .audit.pass_ready == true
  ' "$notary_output_path" >/dev/null 2>&1; then
    printf 'installed-app acceptance requires release-ready %s notarization evidence before phase execution in collection.mode=two_machine: %s\n' "$machine" "$notary_output_path" >&2
    exit 5
  fi
}

acceptance_require_cli_help_contains() {
  sm_bin=$1
  output_path=$2
  expected=$3
  shift 3
  help_output=$("$sm_bin" "$@" 2>&1)
  printf '%s\n' "$help_output" > "$output_path"
  if ! printf '%s\n' "$help_output" | grep -F -q -- "$expected"; then
    return 1
  fi
  return 0
}

acceptance_record_cli_surface() {
  bundle_root=$1
  role=$2
  command_path=$3
  expected=$4
  output_path=$5
  acceptance_update_bundle_meta_locked "$bundle_root" \
    --arg role "$role" \
    --arg command_path "$command_path" \
    --arg expected "$expected" \
    --arg output_path "$output_path" \
    '.evidence.cli_surface = (.evidence.cli_surface // {})
    | .evidence.cli_surface[$role] = {
        command: $command_path,
        expected: $expected,
        output: $output_path,
        status: "pass"
      }'
}

acceptance_preflight_cli_surface() {
  sm_bin=$1
  app_dir=$2
  bundle_root=$3
  role=$4
  command_path=$5
  expected=$6
  shift 6
  acceptance_ensure_bundle_root "$bundle_root"
  acceptance_record_cli_facts "$sm_bin" "$app_dir" "$bundle_root" "$role"
  output_rel="$role.help.txt"
  output_path="$bundle_root/$output_rel"
  if ! acceptance_require_cli_help_contains "$sm_bin" "$output_path" "$expected" "$@"; then
    printf 'bundled CLI surface missing for %s: expected %s in `%s`; rebuild the app bundle with macos/script/build-app.sh\n' "$role" "$expected" "$command_path" >&2
    exit 5
  fi
  acceptance_record_cli_surface "$bundle_root" "$role" "$command_path" "$expected" "$output_rel"
}

acceptance_write_json() {
  output_path=$1
  shift
  jq -n "$@" > "$output_path"
}

acceptance_bundle_session_id() {
  bundle_root=$1
  sed -n 's/.* session=\([^ ]*\) .*/\1/p' "$bundle_root/source.network-push.txt" | sed -n '1p'
}

acceptance_profile_pairing_receipt_id() {
  profile_path=$1
  python3 - "$profile_path" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    doc = json.load(f)
print(doc["target"]["pairing_receipt_id"])
PY
}

acceptance_update_bundle_meta() {
  bundle_root=$1
  shift
  acceptance_update_bundle_meta_locked "$bundle_root" "$@"
  if [ "${SUPERMOVER_ACCEPTANCE_SKIP_WORKFLOW_REFRESH:-0}" != "1" ] && command -v acceptance_two_machine_refresh_workflow_summary >/dev/null 2>&1; then
    SUPERMOVER_ACCEPTANCE_SKIP_WORKFLOW_REFRESH=1 acceptance_two_machine_refresh_workflow_summary "$bundle_root"
  fi
}

acceptance_resolve_bundle_relative_path() {
  bundle_root=$1
  artifact_rel=$2
  acceptance_require_flag "$artifact_rel" "bundle artifact path"
  python3 - "$bundle_root" "$artifact_rel" <<'PY'
import os
import sys

bundle_root, artifact_rel = sys.argv[1:3]
trimmed = artifact_rel.strip()

def fail(message):
    sys.stderr.write(message + "\n")
    raise SystemExit(5)

if not trimmed:
    fail("missing bundle artifact path")
if trimmed.startswith("/") or trimmed.startswith("~"):
    fail(f"bundle artifact path must be relative: {artifact_rel}")

parts = trimmed.split("/")
if any(part in ("", ".", "..") for part in parts):
    fail(f"bundle artifact path contains unsafe traversal: {artifact_rel}")

current = bundle_root
if os.path.islink(current):
    fail(f"bundle artifact path resolves through symlink: {artifact_rel}")

for part in parts:
    current = os.path.join(current, part)
    if os.path.islink(current):
        fail(f"bundle artifact path resolves through symlink: {artifact_rel}")

sys.stdout.write(current + "\n")
PY
}
