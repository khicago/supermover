acceptance_two_machine_usage() {
  cat <<'EOF'
Usage:
  macos/script/acceptance-two-machine.sh target-serve --profile <target-profile> --bundle-root <shared-dir>
  macos/script/acceptance-two-machine.sh target-advertise --profile <target-profile> --bundle-root <shared-dir> [--listen <host:port>] [--dest <host:port>] [--duration <duration>] [--interval <duration>]
  macos/script/acceptance-two-machine.sh source-browse --bundle-root <shared-dir> [--listen <host:port>] [--timeout <duration>] [--strict]
  macos/script/acceptance-two-machine.sh source-pair --profile <source-profile> --bundle-root <shared-dir> [--target-address <host:port> --verification-code <code>]
  macos/script/acceptance-two-machine.sh target-import --profile <target-profile> --bundle-root <shared-dir>
  macos/script/acceptance-two-machine.sh source-transfer --profile <source-profile> --bundle-root <shared-dir> --session <id>
  macos/script/acceptance-two-machine.sh record-packaging-evidence --bundle-root <shared-dir> --machine <source|target> --app <path/to/SuperMover.app>
  macos/script/acceptance-two-machine.sh merge-bundle --bundle-root <local-dir> --incoming-bundle-root <other-machine-dir>
  macos/script/acceptance-two-machine.sh pack-bundle --bundle-root <local-dir> --archive <bundle.tgz>
  macos/script/acceptance-two-machine.sh unpack-bundle --archive <bundle.tgz> [--manifest <bundle.manifest.json>] --bundle-root <local-dir>
  macos/script/acceptance-two-machine.sh workflow-status --bundle-root <local-dir> [--require-operator-evidence]
  macos/script/acceptance-two-machine.sh record-operator-evidence --bundle-root <shared-dir> --kind <kind> --status <pass|blocked> --detail <text> [--artifact <path>]
  macos/script/acceptance-two-machine.sh evaluate --bundle-root <shared-dir> --target-root <target-dir> --source-profile <source-profile> [--require-operator-evidence]

This harness does not claim completion by itself. It records installed-app LAN
evidence across two real machines using explicit phase roles:
  target-serve: start installed-app serve and write readiness/provenance evidence
  target-advertise: emit bounded low-information LAN advertisement evidence
  source-browse: capture untrusted LAN candidate evidence
  source-pair: pair against the current target serve and export a durable source-side receipt
  target-import: adopt the exported receipt into target control-plane/profile
  source-transfer: run dry-run and non-dry-run mTLS push against the paired receiver and write transfer evidence
  record-packaging-evidence: collect install-ready app audit plus structured notarization sidecar evidence for one machine into the current bundle
  merge-bundle: merge another machine's locally collected bundle evidence into the current bundle root, refusing conflicts
  pack-bundle: archive one machine's local bundle for cross-machine handoff and write a checksum manifest
  unpack-bundle: verify archive+manifest integrity, then restore into a local bundle root before merge/evaluate
  workflow-status: summarize the current bundle state and the next machine-local actions implied by the current evidence
  record-operator-evidence: write explicit manual evidence records for Local Network/firewall prompts, physical code confirmation, or other non-automated real-device checks. For strict two-machine kinds, pass records are bound to canonical source.machine.json / target.machine.json.
  evaluate: inspect the preserved bundle plus target .supermover artifacts and fail if required evidence is missing

Required bundle contents are created incrementally by the role commands. For real
two-machine runs, each machine may collect into its own local bundle root and
then explicitly hand off and merge the other machine's bundle before downstream
phases that depend on remote artifacts.
EOF
}

acceptance_two_machine_require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    printf 'jq is required for acceptance checks\n' >&2
    exit 1
  fi
}

acceptance_two_machine_require_control_plane_id() {
  value=$1
  label=$2
  if [ -z "$value" ] || [ "$value" = "." ] || [ "$value" = ".." ]; then
    printf 'invalid %s path segment: %s\n' "$label" "$value" >&2
    exit 1
  fi
  case "$value" in
    /*|~*|*/*|*\\*|*[[:space:]]*)
      printf 'invalid %s path segment: %s\n' "$label" "$value" >&2
      exit 1
      ;;
  esac
}

acceptance_two_machine_require_json_control_plane_id() {
  json_path=$1
  expr=$2
  label=$3
  if ! jq -e "
    ($expr) as \$value
    | (\$value | type == \"string\")
    and (\$value | length > 0)
    and (\$value != \".\")
    and (\$value != \"..\")
    and (\$value | startswith(\"~\") | not)
    and (\$value | contains(\"/\") | not)
    and (\$value | contains(\"\\\\\") | not)
    and (\$value | explode | all(. >= 32 and . != 127))
    and (\$value | test(\"[[:space:]]\") | not)
  " "$json_path" >/dev/null; then
    printf 'invalid %s path segment: %s\n' "$label" "$(jq -r "($expr // \"\") | @json" "$json_path" 2>/dev/null || printf '')" >&2
    exit 1
  fi
}

acceptance_two_machine_json_string_or_default() {
  json_path=$1
  expr=$2
  label=$3
  default_value=$4
  if ! jq -er --arg default_value "$default_value" "
    ($expr) as \$value
    | if \$value == null then \$default_value
      elif (\$value | type) == \"string\" then \$value
      else error(\"expected string\")
      end
  " "$json_path"; then
    printf 'invalid %s: expected string\n' "$label" >&2
    exit 1
  fi
}

acceptance_two_machine_require_target_control_artifact() {
  target_root=$1
  relative_path=$2
  label=$3
  python3 - "$target_root" "$relative_path" "$label" <<'PY'
import os
import sys

target_root, relative_path, label = sys.argv[1:4]
if os.path.islink(target_root) or not os.path.isdir(target_root):
    sys.stderr.write(f"invalid target root for {label}: {target_root}\n")
    sys.exit(1)
if not relative_path.startswith(".supermover/"):
    sys.stderr.write(f"invalid {label}: {relative_path}\n")
    sys.exit(1)
parts = relative_path.split("/")
if any(part in ("", ".", "..") for part in parts):
    sys.stderr.write(f"invalid {label}: {relative_path}\n")
    sys.exit(1)
current = target_root
for part in parts:
    current = os.path.join(current, part)
    if os.path.islink(current):
        sys.stderr.write(f"invalid {label}: {relative_path}\n")
        sys.exit(1)
if not os.path.exists(current):
    sys.stderr.write(f"missing {label}: {relative_path}\n")
    sys.exit(1)
if not os.path.isfile(current):
    sys.stderr.write(f"invalid {label}: {relative_path}\n")
    sys.exit(1)
if os.stat(current).st_nlink != 1:
    sys.stderr.write(f"invalid {label}: {relative_path}\n")
    sys.exit(1)
PY
}

acceptance_two_machine_require_regular_evidence_file() {
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

acceptance_two_machine_regular_evidence_file() {
  path=$1
  python3 - "$path" <<'PY'
import os
import sys

path = sys.argv[1]
if not os.path.exists(path):
    sys.exit(1)
if os.path.islink(path) or not os.path.isfile(path) or os.stat(path).st_nlink != 1:
    sys.exit(1)
PY
}

acceptance_two_machine_require_bundle_regular_artifact() {
  bundle_root=$1
  relative_path=$2
  label=$3
  path=$(acceptance_resolve_bundle_relative_path "$bundle_root" "$relative_path")
  acceptance_two_machine_require_regular_evidence_file "$path" "$label"
  printf '%s\n' "$path"
}

acceptance_two_machine_validate_pairing_receipt() {
  receipt_path=$1
  expected_id=$2
  label=$3
  if ! python3 - "$receipt_path" "$expected_id" <<'PY'
import datetime
import json
import sys

path, expected_id = sys.argv[1:3]

def clean_string(value):
    if not isinstance(value, str):
        return ""
    value = value.strip()
    return value

def parse_rfc3339(value):
    text = clean_string(value)
    if not text:
        return None
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        return datetime.datetime.fromisoformat(text)
    except ValueError:
        return None

try:
    with open(path, "r", encoding="utf-8") as file:
        receipt = json.load(file)
except Exception:
    sys.exit(1)

if not isinstance(receipt, dict):
    sys.exit(1)
version = receipt.get("version")
if not isinstance(version, int) or isinstance(version, bool) or version <= 0:
    sys.exit(1)
required = [
    "id",
    "profile_id",
    "target_id",
    "source_device_id",
    "target_device_id",
    "device_public_key",
    "method",
    "verified_at",
    "protocol_version",
]
fields = {key: clean_string(receipt.get(key)) for key in required}
if any(not value for value in fields.values()):
    sys.exit(1)
if fields["id"] != expected_id:
    sys.exit(1)
if fields["device_public_key"] != fields["target_device_id"]:
    sys.exit(1)
if parse_rfc3339(fields["verified_at"]) is None:
    sys.exit(1)
verification_hash = clean_string(receipt.get("verification_hash"))
verification_phrase = clean_string(receipt.get("verification_phrase"))
if not verification_hash and not verification_phrase:
    sys.exit(1)
PY
  then
    printf 'invalid %s: %s\n' "$label" "$receipt_path" >&2
    exit 1
  fi
}

acceptance_two_machine_validate_network_transfer() {
  transfer_path=$1
  expected_session_id=$2
  label=$3
  if ! python3 - "$transfer_path" "$expected_session_id" <<'PY'
import datetime
import json
import sys

path, expected_session_id = sys.argv[1:3]

def clean_string(value):
    if not isinstance(value, str):
        return ""
    return value.strip()

def parse_rfc3339(value):
    text = clean_string(value)
    if not text:
        return None
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        return datetime.datetime.fromisoformat(text)
    except ValueError:
        return None

try:
    with open(path, "r", encoding="utf-8") as file:
        transfer = json.load(file)
except Exception:
    sys.exit(1)

if not isinstance(transfer, dict):
    sys.exit(1)
version = transfer.get("version")
if not isinstance(version, int) or isinstance(version, bool) or version <= 0:
    sys.exit(1)
fields = {
    key: clean_string(transfer.get(key))
    for key in [
        "session_id",
        "profile_id",
        "target_id",
        "source_device_id",
        "target_device_id",
        "protocol_version",
        "started_at",
        "updated_at",
    ]
}
if any(not value for value in fields.values()):
    sys.exit(1)
if fields["session_id"] != expected_session_id:
    sys.exit(1)
if fields["source_device_id"] == fields["target_device_id"]:
    sys.exit(1)
if clean_string(transfer.get("status")) != "published":
    sys.exit(1)
if clean_string(transfer.get("stage")) != "commit":
    sys.exit(1)
if clean_string(transfer.get("encrypted_transfer")) != "tls13_mtls":
    sys.exit(1)
started_at = parse_rfc3339(fields["started_at"])
updated_at = parse_rfc3339(fields["updated_at"])
if started_at is None or updated_at is None or updated_at < started_at:
    sys.exit(1)
PY
  then
    printf 'invalid %s: %s\n' "$label" "$transfer_path" >&2
    exit 1
  fi
}

acceptance_two_machine_record_role() {
  bundle_root=$1
  role=$2
  profile_path=$3
  machine_id=$4
  machine_label=$5
  acceptance_update_bundle_meta "$bundle_root" \
    --arg role "$role" \
    --arg profile "$profile_path" \
    --arg machine_id "$machine_id" \
    --arg machine_label "$machine_label" \
    '.roles[$role] = {
      profile: $profile,
      status: "recorded",
      machine_id: $machine_id,
      machine_label: $machine_label
    }'
}

acceptance_two_machine_record_collection() {
  bundle_root=$1
  mode=${2:-${SUPERMOVER_ACCEPTANCE_COLLECTION_MODE:-two_machine}}
  machine_count=${3:-${SUPERMOVER_ACCEPTANCE_MACHINE_COUNT:-2}}
  acceptance_update_bundle_meta "$bundle_root" \
    --arg mode "$mode" \
    --argjson machine_count "$machine_count" \
    '.collection = {
      mode: $mode,
      machine_count: $machine_count
    }'
}

acceptance_two_machine_sha256() {
  input=$1
  if command -v shasum >/dev/null 2>&1; then
    printf '%s' "$input" | shasum -a 256 | awk '{print $1}'
    return 0
  fi
  if command -v openssl >/dev/null 2>&1; then
    printf '%s' "$input" | openssl dgst -sha256 | awk '{print $NF}'
    return 0
  fi
  return 1
}

acceptance_two_machine_file_sha256() {
  input_path=$1
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$input_path" | awk '{print $1}'
    return 0
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$input_path" | awk '{print $NF}'
    return 0
  fi
  return 1
}

acceptance_two_machine_system_uuid() {
  ioreg -rd1 -c IOPlatformExpertDevice 2>/dev/null | awk -F'"' '/IOPlatformUUID/{print $4; exit}'
}

acceptance_two_machine_resolve_machine_id() {
  override=$1
  if [ -n "$override" ]; then
    printf '%s\n' "$override"
    return 0
  fi
  raw_uuid=$(acceptance_two_machine_system_uuid || true)
  if [ -n "$raw_uuid" ]; then
    hashed=$(acceptance_two_machine_sha256 "$raw_uuid" || true)
    if [ -n "$hashed" ]; then
      printf 'macos-platformuuid-%s\n' "$hashed"
      return 0
    fi
    printf 'macos-platformuuid-raw-%s\n' "$raw_uuid"
    return 0
  fi
  hostname_value=$(hostname 2>/dev/null || true)
  if [ -n "$hostname_value" ]; then
    hashed=$(acceptance_two_machine_sha256 "$hostname_value" || true)
    if [ -n "$hashed" ]; then
      printf 'macos-hostname-%s\n' "$hashed"
      return 0
    fi
    printf 'macos-hostname-raw-%s\n' "$hostname_value"
    return 0
  fi
  printf 'macos-machine-unknown\n'
}

acceptance_two_machine_resolve_machine_label() {
  override=$1
  if [ -n "$override" ]; then
    printf '%s\n' "$override"
    return 0
  fi
  if command -v scutil >/dev/null 2>&1; then
    label=$(scutil --get LocalHostName 2>/dev/null || true)
    if [ -n "$label" ]; then
      printf '%s\n' "$label"
      return 0
    fi
  fi
  hostname_value=$(hostname 2>/dev/null || true)
  if [ -n "$hostname_value" ]; then
    printf '%s\n' "$hostname_value"
    return 0
  fi
  printf '\n'
}

acceptance_two_machine_record_machine_facts() {
  bundle_root=$1
  machine=$2
  machine_id=$(acceptance_two_machine_resolve_machine_id "$3")
  machine_label=$(acceptance_two_machine_resolve_machine_label "$4")
  output_path="$bundle_root/$machine.machine.json"
  acceptance_write_json "$output_path" \
    --arg machine_id "$machine_id" \
    --arg machine_label "$machine_label" \
    '{
      schema: "supermover.acceptance.machine_facts.v1",
      machine_id: $machine_id,
      machine_label: (if ($machine_label | length) > 0 then $machine_label else null end)
    }'
  acceptance_update_bundle_meta "$bundle_root" \
    --arg machine "$machine" \
    --arg output "$machine.machine.json" \
    --arg machine_id "$machine_id" \
    --arg machine_label "$machine_label" \
    '.evidence.machine_facts = (.evidence.machine_facts // {})
    | .evidence.machine_facts[$machine] = {
        output: $output,
        machine_id: $machine_id,
        machine_label: (if ($machine_label | length) > 0 then $machine_label else null end)
      }'
}

acceptance_two_machine_operator_evidence_machine() {
  kind=$1
  case "$kind" in
    local_network|firewall) printf 'target\n' ;;
    pairing_confirmation) printf 'source\n' ;;
    *) printf '\n' ;;
  esac
}

acceptance_two_machine_read_machine_fact_field() {
  bundle_root=$1
  machine=$2
  field=$3
  case "$machine" in
    source|target) ;;
    *) printf '\n'; return 0 ;;
  esac
  artifact_path="$bundle_root/$machine.machine.json"
  if ! acceptance_two_machine_regular_evidence_file "$artifact_path"; then
    printf '\n'
    return 0
  fi
  if ! jq -e '.schema == "supermover.acceptance.machine_facts.v1"' "$artifact_path" >/dev/null 2>&1; then
    printf '\n'
    return 0
  fi
  jq -r --arg field "$field" '(.[$field] // "") | if type == "string" then . else "" end' "$artifact_path"
}

acceptance_two_machine_operator_evidence_expected_machine_id() {
  bundle_root=$1
  kind=$2
  machine=$(acceptance_two_machine_operator_evidence_machine "$kind")
  acceptance_two_machine_read_machine_fact_field "$bundle_root" "$machine" "machine_id"
}

acceptance_two_machine_operator_evidence_expected_machine_label() {
  bundle_root=$1
  kind=$2
  machine=$(acceptance_two_machine_operator_evidence_machine "$kind")
  acceptance_two_machine_read_machine_fact_field "$bundle_root" "$machine" "machine_label"
}

acceptance_two_machine_bundle_export_identity_artifact_name() {
  printf '.supermover-bundle-export.json\n'
}

acceptance_two_machine_bundle_export_identity_summary() {
  bundle_root=$1
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  exporting_machine_id=$(acceptance_two_machine_resolve_machine_id "${SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID:-}")
  exporting_machine_label=$(acceptance_two_machine_resolve_machine_label "${SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL:-}")
  python3 - "$bundle_root" "$exporting_machine_id" "$exporting_machine_label" <<'PY'
import json
import os
import sys

bundle_root, exporting_machine_id, exporting_machine_label = sys.argv[1:4]

CANONICAL_MACHINE_FACT_ARTIFACTS = {
    "source": "source.machine.json",
    "target": "target.machine.json",
}
MACHINE_FACTS_SCHEMA = "supermover.acceptance.machine_facts.v1"

def clean_text(value):
    if not isinstance(value, str):
        return None
    value = value.strip()
    return value or None

def safe_artifact_path(relative_path):
    trimmed = clean_text(relative_path)
    if trimmed is None or trimmed.startswith("/") or trimmed.startswith("~"):
        return None
    parts = trimmed.split("/")
    if any(part in ("", ".", "..") for part in parts):
        return None
    current = bundle_root
    if os.path.islink(current):
        return None
    for part in parts:
        current = os.path.join(current, part)
        if os.path.islink(current):
            return None
    return current

def load_machine_fact_artifact(machine):
    relative_path = CANONICAL_MACHINE_FACT_ARTIFACTS[machine]
    artifact_path = safe_artifact_path(relative_path)
    if artifact_path is None:
        return {
            "machine": machine,
            "path": relative_path,
            "valid": False,
        }
    if not os.path.exists(artifact_path):
        return None
    try:
        with open(artifact_path) as f:
            artifact = json.load(f)
    except (OSError, json.JSONDecodeError):
        return {
            "machine": machine,
            "path": relative_path,
            "valid": False,
        }
    if not isinstance(artifact, dict):
        return {
            "machine": machine,
            "path": relative_path,
            "valid": False,
        }
    machine_id = clean_text(artifact.get("machine_id"))
    schema = clean_text(artifact.get("schema"))
    if machine_id is None or schema != MACHINE_FACTS_SCHEMA:
        return {
            "machine": machine,
            "path": relative_path,
            "valid": False,
        }
    return {
        "machine": machine,
        "path": relative_path,
        "valid": True,
        "machine_id": machine_id,
        "machine_label": clean_text(artifact.get("machine_label")),
    }

exporting_machine_id = clean_text(exporting_machine_id)
exporting_machine_label = clean_text(exporting_machine_label)
artifacts = [
    artifact
    for artifact in (
        load_machine_fact_artifact("source"),
        load_machine_fact_artifact("target"),
    )
    if artifact is not None
]
valid_artifacts = [artifact for artifact in artifacts if artifact.get("valid") is True]
matched_artifacts = [
    artifact
    for artifact in valid_artifacts
    if artifact.get("machine_id") == exporting_machine_id
]
selected_artifact = None
failure_message = None

if not valid_artifacts:
    failure_message = (
        "missing valid canonical source.machine.json / target.machine.json export identity evidence"
    )
elif len(matched_artifacts) == 1:
    selected_artifact = matched_artifacts[0]
elif len(matched_artifacts) > 1:
    if exporting_machine_label is not None:
        label_matches = [
            artifact
            for artifact in matched_artifacts
            if artifact.get("machine_label") == exporting_machine_label
        ]
        if len(label_matches) == 1:
            selected_artifact = label_matches[0]
    if selected_artifact is None:
        failure_message = (
            f"canonical source.machine.json / target.machine.json both match exporting machine_id={exporting_machine_id}; "
            "set SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL to the exact machine_label for the bundle being packed"
        )
else:
    failure_message = (
        f"canonical source.machine.json / target.machine.json do not match exporting machine_id={exporting_machine_id}; "
        "set SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID to the machine that is packing this bundle"
    )

json.dump({
    "ok": selected_artifact is not None,
    "machine": selected_artifact.get("machine") if selected_artifact is not None else None,
    "machine_id": selected_artifact.get("machine_id") if selected_artifact is not None else None,
    "machine_label": selected_artifact.get("machine_label") if selected_artifact is not None else None,
    "failure_message": failure_message,
}, sys.stdout)
sys.stdout.write("\n")
PY
}

acceptance_two_machine_merge_bundle() {
  bundle_root=$1
  incoming_bundle_root=$2
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  if [ ! -d "$incoming_bundle_root" ]; then
    printf 'missing incoming bundle root: %s\n' "$incoming_bundle_root" >&2
    exit 5
  fi
  if ! acceptance_two_machine_validate_unpacked_bundle_tree "$incoming_bundle_root"; then
    exit 5
  fi
  if [ ! -f "$incoming_bundle_root/meta.json" ]; then
    printf 'missing incoming bundle meta: %s\n' "$incoming_bundle_root/meta.json" >&2
    exit 5
  fi

  destination_collection=$(jq -c '.collection // {}' "$bundle_root/meta.json")
  incoming_collection=$(jq -c '.collection // {}' "$incoming_bundle_root/meta.json")
  destination_mode=$(jq -r '.collection.mode // "unknown"' "$bundle_root/meta.json")
  destination_machine_count=$(jq -r '.collection.machine_count // 0' "$bundle_root/meta.json")
  destination_target_ready_phase=$(jq -r '((.evidence.target_serve_phases // []) | map(.phase // 0) | max) // 0' "$bundle_root/meta.json")
  incoming_target_ready_phase=$(jq -r '((.evidence.target_serve_phases // []) | map(.phase // 0) | max) // 0' "$incoming_bundle_root/meta.json")
  if [ "$destination_mode" != "unknown" ] || [ "$destination_machine_count" != "0" ]; then
    if [ "$destination_collection" != "$incoming_collection" ]; then
      printf 'acceptance bundle merge conflict: collection mismatch destination=%s incoming=%s\n' "$destination_collection" "$incoming_collection" >&2
      exit 5
    fi
  fi

  merge_stage_dir=$(mktemp -d "$bundle_root/.merge-staging.XXXXXX")
  artifact_stage_dir="$merge_stage_dir/artifacts"
  staged_artifact_list="$merge_stage_dir/artifacts.list"
  staged_target_ready_path="$merge_stage_dir/target.ready.json"
  tmp_meta="$merge_stage_dir/meta.json"
  published_artifact_list="$merge_stage_dir/published-artifacts.list"
  created_artifact_dirs_list="$merge_stage_dir/created-artifact-dirs.list"
  target_ready_backup_path="$merge_stage_dir/target.ready.backup.json"
  target_ready_published_marker="$merge_stage_dir/target-ready.published"
  target_ready_created_marker="$merge_stage_dir/target-ready.created"
  mkdir -p "$artifact_stage_dir"
  : > "$published_artifact_list"
  : > "$created_artifact_dirs_list"

  python3 - "$bundle_root" "$incoming_bundle_root" "$tmp_meta" "$destination_target_ready_phase" "$incoming_target_ready_phase" "$artifact_stage_dir" "$staged_target_ready_path" <<'PY' || {
import filecmp
import json
import os
import shutil
import stat
import sys

(
    destination_root,
    incoming_root,
    output_path,
    destination_target_ready_phase_raw,
    incoming_target_ready_phase_raw,
    artifact_stage_root,
    target_ready_stage_path,
) = sys.argv[1:8]
destination_path = os.path.join(destination_root, "meta.json")
incoming_path = os.path.join(incoming_root, "meta.json")
destination_target_ready_phase = int(destination_target_ready_phase_raw)
incoming_target_ready_phase = int(incoming_target_ready_phase_raw)

with open(destination_path) as f:
    destination = json.load(f)
with open(incoming_path) as f:
    incoming = json.load(f)

def strip_derived_state(doc):
    if not isinstance(doc, dict):
        return doc
    evidence = doc.get("evidence")
    if not isinstance(evidence, dict) or "workflow_summary" not in evidence:
        return doc
    cleaned = dict(doc)
    cleaned_evidence = dict(evidence)
    cleaned_evidence.pop("workflow_summary", None)
    cleaned["evidence"] = cleaned_evidence
    return cleaned

destination = strip_derived_state(destination)
incoming = strip_derived_state(incoming)

def merge(a, b, path):
    if path == ["evidence", "target_ready"] and isinstance(a, dict) and isinstance(b, dict):
        if incoming_target_ready_phase > destination_target_ready_phase:
            return b
        if destination_target_ready_phase > incoming_target_ready_phase:
            return a
        if a == b:
            return a
        raise ValueError(".".join(path))
    if isinstance(a, dict) and isinstance(b, dict):
        merged = dict(a)
        for key, value in b.items():
            if key not in merged:
                merged[key] = value
                continue
            merged[key] = merge(merged[key], value, path + [str(key)])
        return merged
    if path == ["evidence", "target_serve_phases"] and isinstance(a, list) and isinstance(b, list):
        merged_by_phase = {}
        merged = []
        scalar_seen = set()
        for item in a + b:
            if not isinstance(item, dict):
                key = ("scalar", json.dumps(item, sort_keys=True))
                if key in scalar_seen:
                    continue
                scalar_seen.add(key)
                merged.append(item)
                continue
            phase = item.get("phase")
            if not isinstance(phase, int):
                key = ("dict", json.dumps(item, sort_keys=True))
                if key in scalar_seen:
                    continue
                scalar_seen.add(key)
                merged.append(item)
                continue
            encoded = json.dumps(item, sort_keys=True)
            if phase in merged_by_phase:
                if merged_by_phase[phase] != encoded:
                    raise ValueError(".".join(path + [str(phase)]))
                continue
            merged_by_phase[phase] = encoded
            merged.append(item)
        if all(isinstance(item, dict) and isinstance(item.get("phase"), int) for item in merged):
            merged.sort(key=lambda item: item["phase"])
        return merged
    if path == ["evidence", "bundle_handoffs"] and isinstance(a, list) and isinstance(b, list):
        merged = []
        seen = set()
        for item in a + b:
            if isinstance(item, dict):
                key = (
                    item.get("archive"),
                    item.get("manifest"),
                    item.get("sha256"),
                    item.get("meta"),
                    item.get("exporting_machine_id"),
                    item.get("importing_machine_id"),
                )
            else:
                key = ("scalar", json.dumps(item, sort_keys=True))
            if key in seen:
                continue
            seen.add(key)
            merged.append(item)
        return merged
    if path == ["collection", "mode"] and a == "unknown":
        return b
    if path == ["collection", "machine_count"] and a == 0:
        return b
    if a == b:
        return a
    raise ValueError(".".join(path) or "<root>")

try:
    merged = merge(destination, incoming, [])
except ValueError as error:
    sys.stderr.write(f"acceptance bundle merge conflict: meta.json contains overlapping non-identical values at {error}\n")
    sys.exit(5)

evidence = merged.get("evidence")
if isinstance(evidence, dict):
    evidence.pop("workflow_summary", None)

def unsafe(path, reason):
    rel = os.path.relpath(path, incoming_root)
    sys.stderr.write(f"unsafe acceptance bundle archive entry: {rel} ({reason})\n")
    sys.exit(5)

def artifact_conflict(rel):
    sys.stderr.write(f"acceptance bundle merge conflict: artifact already exists with different contents: {rel}\n")
    sys.exit(5)

def target_ready_conflict(message):
    sys.stderr.write(f"{message}\n")
    sys.exit(5)

def skip_incoming_artifact(rel):
    first = rel.split("/", 1)[0]
    return (
        first.startswith(".merge-staging")
        or rel == "meta.json"
        or rel == "workflow.summary.json"
        or rel == "target.ready.json"
        or rel == "target.serve.next-phase"
        or rel.endswith(".pid")
    )

def require_safe_incoming_file(path):
    try:
        entry = os.lstat(path)
    except OSError:
        unsafe(path, "missing file entry")
    mode = entry.st_mode
    if stat.S_ISLNK(mode):
        unsafe(path, "symlink")
    if not stat.S_ISREG(mode):
        unsafe(path, "non-regular file")
    if entry.st_nlink != 1:
        unsafe(path, "hardlink")

def destination_is_regular_single_link(path):
    try:
        entry = os.lstat(path)
    except OSError:
        return False
    return stat.S_ISREG(entry.st_mode) and entry.st_nlink == 1

def destination_is_directory(path):
    try:
        entry = os.lstat(path)
    except OSError:
        return False
    return stat.S_ISDIR(entry.st_mode)

for current, dirs, files in os.walk(incoming_root, followlinks=False):
    dirs.sort()
    files.sort()

    for name in list(dirs):
        path = os.path.join(current, name)
        rel = os.path.relpath(path, incoming_root)
        if skip_incoming_artifact(rel):
            dirs.remove(name)
            continue
        destination_artifact_path = os.path.join(destination_root, rel)
        if os.path.lexists(destination_artifact_path) and not destination_is_directory(destination_artifact_path):
            artifact_conflict(rel)
        os.makedirs(os.path.join(artifact_stage_root, rel), exist_ok=True)

    for name in files:
        path = os.path.join(current, name)
        rel = os.path.relpath(path, incoming_root)
        if skip_incoming_artifact(rel):
            continue
        require_safe_incoming_file(path)
        destination_artifact_path = os.path.join(destination_root, rel)
        if os.path.lexists(destination_artifact_path):
            if not destination_is_regular_single_link(destination_artifact_path):
                artifact_conflict(rel)
            if not filecmp.cmp(path, destination_artifact_path, shallow=False):
                artifact_conflict(rel)
            continue
        staged_artifact_path = os.path.join(artifact_stage_root, rel)
        os.makedirs(os.path.dirname(staged_artifact_path), exist_ok=True)
        shutil.copy2(path, staged_artifact_path, follow_symlinks=False)

incoming_target_ready_path = os.path.join(incoming_root, "target.ready.json")
destination_target_ready_path = os.path.join(destination_root, "target.ready.json")
if os.path.lexists(incoming_target_ready_path):
    require_safe_incoming_file(incoming_target_ready_path)
    if not os.path.lexists(destination_target_ready_path):
        shutil.copy2(incoming_target_ready_path, target_ready_stage_path, follow_symlinks=False)
    elif not destination_is_regular_single_link(destination_target_ready_path):
        target_ready_conflict(f"acceptance bundle merge conflict: target.ready.json destination is not a file: {destination_target_ready_path}")
    elif incoming_target_ready_phase > destination_target_ready_phase:
        shutil.copy2(incoming_target_ready_path, target_ready_stage_path, follow_symlinks=False)
    elif incoming_target_ready_phase == destination_target_ready_phase and not filecmp.cmp(
        incoming_target_ready_path,
        destination_target_ready_path,
        shallow=False,
    ):
        target_ready_conflict(f"acceptance bundle merge conflict: target.ready.json differs for phase {incoming_target_ready_phase}")

with open(output_path, "w") as f:
    json.dump(merged, f, indent=2)
    f.write("\n")
PY
    status=$?
    rm -rf "$merge_stage_dir"
    exit "$status"
  }

  acceptance_two_machine_merge_bundle_rollback_publish() {
    if [ -f "$target_ready_published_marker" ]; then
      if [ -f "$target_ready_backup_path" ]; then
        mv "$target_ready_backup_path" "$bundle_root/target.ready.json" 2>/dev/null || true
      elif [ -f "$target_ready_created_marker" ]; then
        rm -f "$bundle_root/target.ready.json"
      fi
    fi
    while IFS= read -r published_path; do
      if [ -n "$published_path" ]; then
        rm -f "$published_path"
      fi
    done < "$published_artifact_list"
    sort -r "$created_artifact_dirs_list" | while IFS= read -r created_dir; do
      if [ -n "$created_dir" ]; then
        rmdir "$created_dir" 2>/dev/null || true
      fi
    done
  }

  acceptance_two_machine_merge_bundle_publish_failure() {
    message=$1
    acceptance_two_machine_merge_bundle_rollback_publish
    printf '%s\n' "$message" >&2
    rm -rf "$merge_stage_dir"
    exit 5
  }

  acceptance_two_machine_merge_bundle_mkdir_p() {
    target_dir=$1
    if [ -z "$target_dir" ] || [ "$target_dir" = "." ] || [ -d "$target_dir" ]; then
      return 0
    fi
    missing_dirs="$merge_stage_dir/missing-dirs.tmp"
    : > "$missing_dirs"
    current_dir=$target_dir
    while [ "$current_dir" != "$bundle_root" ] && [ "$current_dir" != "/" ] && [ ! -e "$current_dir" ] && [ ! -L "$current_dir" ]; do
      printf '%s\n' "$current_dir" >> "$missing_dirs"
      current_dir=$(dirname "$current_dir")
    done
    if ! cat "$missing_dirs" >> "$created_artifact_dirs_list"; then
      rm -f "$missing_dirs"
      return 1
    fi
    if ! mkdir -p "$target_dir"; then
      rm -f "$missing_dirs"
      return 1
    fi
    rm -f "$missing_dirs"
    return 0
  }

  find "$artifact_stage_dir" -mindepth 1 -print | sort > "$staged_artifact_list"
  while IFS= read -r staged_path; do
    staged_rel=${staged_path#"$artifact_stage_dir"/}
    destination_path="$bundle_root/$staged_rel"
    if [ -d "$staged_path" ]; then
      if [ -e "$destination_path" ] || [ -L "$destination_path" ]; then
        if [ -L "$destination_path" ] || [ ! -d "$destination_path" ]; then
          acceptance_two_machine_merge_bundle_publish_failure "acceptance bundle merge conflict: artifact already exists with different contents: $staged_rel"
        fi
      else
        if ! acceptance_two_machine_merge_bundle_mkdir_p "$destination_path"; then
          acceptance_two_machine_merge_bundle_publish_failure "failed to publish merged acceptance artifact directory: $staged_rel"
        fi
      fi
      continue
    fi
    destination_parent=$(dirname "$destination_path")
    if ! acceptance_two_machine_merge_bundle_mkdir_p "$destination_parent"; then
      acceptance_two_machine_merge_bundle_publish_failure "failed to publish merged acceptance artifact directory: $staged_rel"
    fi
    if [ -e "$destination_path" ] || [ -L "$destination_path" ]; then
      acceptance_two_machine_merge_bundle_publish_failure "acceptance bundle merge conflict: artifact already exists with different contents: $staged_rel"
    fi
    printf '%s\n' "$destination_path" >> "$published_artifact_list"
    if ! cp "$staged_path" "$destination_path"; then
      rm -f "$destination_path"
      acceptance_two_machine_merge_bundle_publish_failure "failed to publish merged acceptance artifact: $staged_rel"
    fi
  done < "$staged_artifact_list"

  if [ -f "$staged_target_ready_path" ]; then
    if [ -f "$bundle_root/target.ready.json" ]; then
      if ! cp "$bundle_root/target.ready.json" "$target_ready_backup_path"; then
        acceptance_two_machine_merge_bundle_publish_failure "failed to stage target.ready.json rollback copy"
      fi
    else
      : > "$target_ready_created_marker"
    fi
    : > "$target_ready_published_marker"
    if ! mv "$staged_target_ready_path" "$bundle_root/target.ready.json"; then
      acceptance_two_machine_merge_bundle_publish_failure "failed to publish merged target.ready.json"
    fi
  fi
  if ! mv "$tmp_meta" "$bundle_root/meta.json"; then
    acceptance_two_machine_merge_bundle_publish_failure "failed to publish merged acceptance bundle meta"
  fi
  rm -rf "$merge_stage_dir"
  acceptance_two_machine_refresh_workflow_summary "$bundle_root"
}

acceptance_two_machine_pack_bundle() {
  bundle_root=$1
  archive_path=$2
  acceptance_two_machine_require_jq
  if [ ! -d "$bundle_root" ]; then
    printf 'missing bundle root to pack: %s\n' "$bundle_root" >&2
    exit 5
  fi
  if [ ! -f "$bundle_root/meta.json" ]; then
    printf 'missing bundle meta to pack: %s\n' "$bundle_root/meta.json" >&2
    exit 5
  fi
  acceptance_two_machine_refresh_workflow_summary "$bundle_root"
  export_identity=$(acceptance_two_machine_bundle_export_identity_summary "$bundle_root")
  if ! printf '%s\n' "$export_identity" | jq -e '.ok == true' >/dev/null 2>&1; then
    printf '%s\n' "$(printf '%s\n' "$export_identity" | jq -r '.failure_message // "invalid canonical machine-facts export identity"')" >&2
    exit 5
  fi
  archive_parent=$(dirname "$archive_path")
  mkdir -p "$archive_parent"
  rm -f "$archive_path"
  export_artifact_name=$(acceptance_two_machine_bundle_export_identity_artifact_name)
  pack_stage_dir=$(mktemp -d "${TMPDIR:-/tmp}/supermover-bundle-pack-XXXXXX")
  acceptance_write_json "$pack_stage_dir/$export_artifact_name" \
    --arg machine_id "$(printf '%s\n' "$export_identity" | jq -r '.machine_id // ""')" \
    --arg machine_label "$(printf '%s\n' "$export_identity" | jq -r '.machine_label // ""')" \
    '{
      schema: "supermover.acceptance.bundle_export_identity.v1",
      machine_id: $machine_id,
      machine_label: (if ($machine_label | length) > 0 then $machine_label else null end)
    }'
  tar -C "$bundle_root" -czf "$archive_path" . -C "$pack_stage_dir" "$export_artifact_name" || {
    rm -rf "$pack_stage_dir"
    rm -f "$archive_path"
    printf 'failed to pack acceptance bundle archive: %s\n' "$archive_path" >&2
    exit 5
  }
  rm -rf "$pack_stage_dir"
  archive_name=$(basename "$archive_path")
  archive_stem=${archive_name%.tgz}
  if [ "$archive_stem" = "$archive_name" ]; then
    archive_stem=${archive_name%.tar.gz}
  fi
  if [ "$archive_stem" = "$archive_name" ]; then
    archive_stem=${archive_name}
  fi
  manifest_path="$archive_parent/$archive_stem.manifest.json"
  archive_sha256=$(acceptance_two_machine_file_sha256 "$archive_path")
  workflow_summary=$(acceptance_two_machine_workflow_summary_json "$bundle_root" 0)
  exporting_machine_id=$(printf '%s\n' "$export_identity" | jq -r '.machine_id // ""')
  exporting_machine_label=$(printf '%s\n' "$export_identity" | jq -r '.machine_label // ""')
  acceptance_write_json "$manifest_path" \
    --arg schema "supermover.acceptance.bundle_archive.v1" \
    --arg archive "$archive_name" \
    --arg sha256 "$archive_sha256" \
    --arg meta "meta.json" \
    --arg bundle_status "$(jq -r '.status // ""' "$bundle_root/meta.json")" \
    --arg collection_mode "$(jq -r '.collection.mode // "unknown"' "$bundle_root/meta.json")" \
    --arg exporting_machine_id "$exporting_machine_id" \
    --arg exporting_machine_label "$exporting_machine_label" \
    --argjson machine_count "$(jq '.collection.machine_count // 0' "$bundle_root/meta.json")" \
    --argjson workflow_summary "$workflow_summary" \
    '{
      schema: $schema,
      archive: $archive,
      sha256: $sha256,
      meta: $meta,
      bundle_status: $bundle_status,
      collection_mode: $collection_mode,
      machine_count: $machine_count,
      exporting_machine_id: (if ($exporting_machine_id | length) > 0 then $exporting_machine_id else null end),
      exporting_machine_label: (if ($exporting_machine_label | length) > 0 then $exporting_machine_label else null end),
      workflow_summary: $workflow_summary
    }'
}

acceptance_two_machine_validate_bundle_archive_members() {
  archive_path=$1
  python3 - "$archive_path" <<'PY'
import posixpath
import sys
import tarfile

archive_path = sys.argv[1]

def fail(message):
    sys.stderr.write(message + "\n")
    sys.exit(1)

def unsafe(name, reason):
    fail(f"unsafe acceptance bundle archive entry: {name} ({reason})")

try:
    archive = tarfile.open(archive_path, "r:*")
except (OSError, tarfile.TarError):
    fail(f"failed to unpack acceptance bundle archive: {archive_path}")

with archive:
    for member in archive.getmembers():
        name = member.name
        normalized = posixpath.normpath(name)
        if normalized == ".":
            if not member.isdir():
                unsafe(name, "root entry is not a directory")
            continue
        if (
            name == ""
            or name.startswith("/")
            or normalized.startswith("../")
            or normalized == ".."
            or normalized.startswith("~")
        ):
            unsafe(name, "path is not bundle-local")
        if member.issym():
            unsafe(name, "symlink")
        if member.islnk():
            unsafe(name, "hardlink")
        if member.isdir() or member.isfile():
            continue
        unsafe(name, "non-regular file")
PY
}

acceptance_two_machine_validate_unpacked_bundle_tree() {
  unpack_stage_dir=$1
  python3 - "$unpack_stage_dir" <<'PY'
import os
import stat
import sys

root = sys.argv[1]

def rel(path):
    relative = os.path.relpath(path, root)
    if relative == ".":
        return "."
    return relative

def fail(path, reason):
    sys.stderr.write(f"unsafe acceptance bundle archive entry: {rel(path)} ({reason})\n")
    sys.exit(1)

if os.path.islink(root) or not os.path.isdir(root):
    fail(root, "staging root is not a directory")

for current, dirs, files in os.walk(root, followlinks=False):
    for name in dirs:
        path = os.path.join(current, name)
        try:
            mode = os.lstat(path).st_mode
        except OSError:
            fail(path, "missing directory entry")
        if stat.S_ISLNK(mode):
            fail(path, "symlink")
        if not stat.S_ISDIR(mode):
            fail(path, "non-directory entry")
    for name in files:
        path = os.path.join(current, name)
        try:
            entry = os.lstat(path)
        except OSError:
            fail(path, "missing file entry")
        mode = entry.st_mode
        if stat.S_ISLNK(mode):
            fail(path, "symlink")
        if not stat.S_ISREG(mode):
            fail(path, "non-regular file")
        if entry.st_nlink != 1:
            fail(path, "hardlink")
PY
}

acceptance_two_machine_unpack_bundle() {
  archive_path=$1
  manifest_path=$2
  bundle_root=$3
  acceptance_two_machine_require_jq
  if [ ! -f "$archive_path" ]; then
    printf 'missing acceptance bundle archive: %s\n' "$archive_path" >&2
    exit 5
  fi
  if [ -z "$manifest_path" ]; then
    archive_parent=$(dirname "$archive_path")
    archive_name=$(basename "$archive_path")
    archive_stem=${archive_name%.tgz}
    if [ "$archive_stem" = "$archive_name" ]; then
      archive_stem=${archive_name%.tar.gz}
    fi
    if [ "$archive_stem" = "$archive_name" ]; then
      archive_stem=${archive_name}
    fi
    manifest_path="$archive_parent/$archive_stem.manifest.json"
  fi
  if [ ! -f "$manifest_path" ]; then
    printf 'missing acceptance bundle manifest: %s\n' "$manifest_path" >&2
    exit 5
  fi
  if ! jq -e '.schema == "supermover.acceptance.bundle_archive.v1"
    and (.archive | type == "string")
    and (.archive | length > 0)
    and (.sha256 | type == "string")
    and (.sha256 | length == 64)
    and .meta == "meta.json"
    and (has("exporting_machine_id"))
    and (.exporting_machine_id | type == "string")
    and (.exporting_machine_id | length > 0)
    and (has("exporting_machine_label"))
    and ((.exporting_machine_label == null) or (.exporting_machine_label | type == "string"))' "$manifest_path" >/dev/null 2>&1; then
    printf 'malformed acceptance bundle manifest: %s\n' "$manifest_path" >&2
    exit 5
  fi
  expected_archive_name=$(jq -r '.archive' "$manifest_path")
  if [ "$expected_archive_name" != "$(basename "$archive_path")" ]; then
    printf 'acceptance bundle manifest archive mismatch: expected %s got %s\n' "$expected_archive_name" "$(basename "$archive_path")" >&2
    exit 5
  fi
  expected_sha256=$(jq -r '.sha256' "$manifest_path")
  actual_sha256=$(acceptance_two_machine_file_sha256 "$archive_path")
  if [ "$expected_sha256" != "$actual_sha256" ]; then
    printf 'acceptance bundle archive digest mismatch: expected %s got %s\n' "$expected_sha256" "$actual_sha256" >&2
    exit 5
  fi
  importing_machine_id=$(acceptance_two_machine_resolve_machine_id "${SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_ID:-${SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID:-${SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID:-}}}")
  importing_machine_label=$(acceptance_two_machine_resolve_machine_label "${SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_LABEL:-${SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL:-${SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL:-}}}")
  bundle_parent=$(dirname "$bundle_root")
  mkdir -p "$bundle_parent"
  if ! acceptance_two_machine_validate_bundle_archive_members "$archive_path"; then
    exit 5
  fi
  unpack_stage_dir=$(mktemp -d "$bundle_parent/.supermover-unpack.XXXXXX")
  if ! tar -C "$unpack_stage_dir" -xzf "$archive_path" >/dev/null 2>&1; then
    rm -rf "$unpack_stage_dir"
    printf 'failed to unpack acceptance bundle archive: %s\n' "$archive_path" >&2
    exit 5
  fi
  if ! acceptance_two_machine_validate_unpacked_bundle_tree "$unpack_stage_dir"; then
    rm -rf "$unpack_stage_dir"
    exit 5
  fi
  if [ ! -f "$unpack_stage_dir/meta.json" ]; then
    rm -rf "$unpack_stage_dir"
    printf 'unpacked acceptance bundle archive is missing meta.json: %s\n' "$archive_path" >&2
    exit 5
  fi
  export_artifact_name=$(acceptance_two_machine_bundle_export_identity_artifact_name)
  export_identity_path="$unpack_stage_dir/$export_artifact_name"
  if [ ! -f "$export_identity_path" ]; then
    rm -rf "$unpack_stage_dir"
    printf 'unpacked acceptance bundle archive is missing export identity artifact: %s\n' "$archive_path" >&2
    exit 5
  fi
  if ! jq -e '.schema == "supermover.acceptance.bundle_export_identity.v1" and (.machine_id | type == "string") and (.machine_id | length > 0) and ((.machine_label == null) or (.machine_label | type == "string"))' "$export_identity_path" >/dev/null 2>&1; then
    rm -rf "$unpack_stage_dir"
    printf 'unpacked acceptance bundle archive has malformed export identity artifact: %s\n' "$archive_path" >&2
    exit 5
  fi
  exporting_machine_id=$(jq -r '.machine_id' "$export_identity_path")
  exporting_machine_label=$(jq -r '.machine_label // ""' "$export_identity_path")
  manifest_exporting_machine_id=$(jq -r '.exporting_machine_id // ""' "$manifest_path")
  manifest_exporting_machine_label=$(jq -r '.exporting_machine_label // ""' "$manifest_path")
  if [ "$manifest_exporting_machine_id" != "$exporting_machine_id" ]; then
    rm -rf "$unpack_stage_dir"
    printf 'acceptance bundle manifest exporting_machine_id mismatch: expected %s got %s\n' "$exporting_machine_id" "$manifest_exporting_machine_id" >&2
    exit 5
  fi
  if [ "$manifest_exporting_machine_label" != "$exporting_machine_label" ]; then
    rm -rf "$unpack_stage_dir"
    printf 'acceptance bundle manifest exporting_machine_label mismatch: expected %s got %s\n' "$exporting_machine_label" "$manifest_exporting_machine_label" >&2
    exit 5
  fi
  rm -f "$export_identity_path"
  if ! (acceptance_update_bundle_meta_locked "$unpack_stage_dir" \
    --arg archive "$(basename "$archive_path")" \
    --arg manifest "$(basename "$manifest_path")" \
    --arg sha256 "$actual_sha256" \
    --arg meta "meta.json" \
    --arg exporting_machine_id "$exporting_machine_id" \
    --arg exporting_machine_label "$exporting_machine_label" \
    --arg importing_machine_id "$importing_machine_id" \
    --arg importing_machine_label "$importing_machine_label" \
    '.evidence.bundle_handoffs = (.evidence.bundle_handoffs // [])
    + [{
      archive: $archive,
      manifest: $manifest,
      sha256: $sha256,
      meta: $meta,
      verified: true,
      exporting_machine_id: (if ($exporting_machine_id | length) > 0 then $exporting_machine_id else null end),
      exporting_machine_label: (if ($exporting_machine_label | length) > 0 then $exporting_machine_label else null end),
      importing_machine_id: (if ($importing_machine_id | length) > 0 then $importing_machine_id else null end),
      importing_machine_label: (if ($importing_machine_label | length) > 0 then $importing_machine_label else null end)
    }]'); then
    rm -rf "$unpack_stage_dir"
    printf 'failed to record verified acceptance bundle handoff: %s\n' "$archive_path" >&2
    exit 5
  fi
  rm -rf "$bundle_root"
  if ! mv "$unpack_stage_dir" "$bundle_root"; then
    rm -rf "$unpack_stage_dir"
    printf 'failed to publish unpacked acceptance bundle: %s\n' "$bundle_root" >&2
    exit 5
  fi
}

acceptance_two_machine_record_packaging_evidence() {
  bundle_root=$1
  machine=$2
  app_dir=$3
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  case "$machine" in
    source|target) ;;
    *)
      printf 'invalid machine for record-packaging-evidence: %s\n' "$machine" >&2
      exit 5
      ;;
  esac
  if [ ! -d "$app_dir" ]; then
    printf 'missing app bundle for %s packaging evidence: %s\n' "$machine" "$app_dir" >&2
    exit 5
  fi
  audit_script=${SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT:-$(acceptance_default_app_audit_executable "$app_dir")}
  if [ ! -x "$audit_script" ]; then
    printf 'missing audit helper: %s\n' "$audit_script" >&2
    exit 5
  fi
  acceptance_record_app_audit "$audit_script" "$app_dir" "$bundle_root" "$machine" "record-packaging-evidence"
  acceptance_require_ready_app_audit_for_collection "$bundle_root" "$machine" "$app_dir" "record-packaging-evidence"
  acceptance_two_machine_refresh_workflow_summary "$bundle_root"
}

acceptance_two_machine_validate_source_transfer_artifact() {
  kind=$1
  path=$2
  session_id=$3
  python3 - "$kind" "$path" "$session_id" <<'PY'
import json
import math
import sys

MAX_SWIFT_INT = 9223372036854775807
MAX_SWIFT_INT_PLUS_ONE_FLOAT = 9223372036854775808.0


class JSONNumber:
    def __init__(self, raw):
        self.raw = raw


def parse_number(raw):
    return JSONNumber(raw)


def non_empty_string(value):
    return isinstance(value, str) and len(value) > 0


def clean_text(value):
    return value.strip() if isinstance(value, str) else ""


def swift_nonnegative_int(value):
    if not isinstance(value, JSONNumber):
        return False
    raw = value.raw
    try:
        if "." not in raw and "e" not in raw and "E" not in raw:
            parsed = int(raw, 10)
            return 0 <= parsed <= MAX_SWIFT_INT
        parsed = float(raw)
    except (OverflowError, ValueError):
        return False
    return (
        math.isfinite(parsed)
        and parsed.is_integer()
        and 0 <= parsed < MAX_SWIFT_INT_PLUS_ONE_FLOAT
    )


kind, path, session_id = sys.argv[1], sys.argv[2], sys.argv[3]
try:
    with open(path) as f:
        doc = json.load(
            f,
            parse_int=parse_number,
            parse_float=parse_number,
            parse_constant=lambda raw: (_ for _ in ()).throw(ValueError(raw)),
        )
except Exception:
    sys.exit(1)

if kind == "status":
    overall = doc.get("overall") if isinstance(doc, dict) else None
    latest = doc.get("latest_session") if isinstance(doc, dict) else None
    counts = doc.get("counts") if isinstance(doc, dict) else None
    ok = (
        isinstance(doc, dict)
        and non_empty_string(doc.get("profile_id"))
        and non_empty_string(doc.get("target_id"))
        and non_empty_string(clean_text(doc.get("target_root")))
        and isinstance(overall, dict)
        and non_empty_string(overall.get("status"))
        and non_empty_string(overall.get("target_status"))
        and isinstance(latest, dict)
        and latest.get("id") == session_id
        and non_empty_string(latest.get("completeness_status"))
        and swift_nonnegative_int(latest.get("files_expected"))
        and swift_nonnegative_int(latest.get("files_verified"))
        and swift_nonnegative_int(latest.get("verification_errors"))
        and isinstance(counts, dict)
        and swift_nonnegative_int(counts.get("artifact_problems"))
        and swift_nonnegative_int(counts.get("network_transfers"))
    )
elif kind == "health":
    summary = doc.get("summary") if isinstance(doc, dict) else None
    transfers = doc.get("network_transfers") if isinstance(doc, dict) else None
    ok = (
        isinstance(doc, dict)
        and non_empty_string(clean_text(doc.get("target_root")))
        and isinstance(doc.get("healthy"), bool)
        and isinstance(summary, dict)
        and swift_nonnegative_int(summary.get("incomplete_sessions"))
        and swift_nonnegative_int(summary.get("invalid_records"))
        and swift_nonnegative_int(summary.get("artifact_problems"))
        and swift_nonnegative_int(summary.get("target_drifts"))
        and swift_nonnegative_int(summary.get("network_transfers"))
        and isinstance(transfers, list)
        and any(
            isinstance(transfer, dict)
            and transfer.get("session_id") == session_id
            and non_empty_string(transfer.get("status"))
            for transfer in transfers
        )
    )
else:
    ok = False
sys.exit(0 if ok else 1)
PY
}

acceptance_two_machine_json_target_root_matches() {
  path=$1
  target_root=$2
  python3 - "$path" "$target_root" <<'PY'
import json
import os
import sys

path, target_root = sys.argv[1], sys.argv[2]

def normalized_path(value):
    if not isinstance(value, str):
        return ""
    text = value.strip()
    if not text:
        return ""
    return os.path.normpath(text)

try:
    with open(path) as f:
        doc = json.load(f)
except Exception:
    sys.exit(1)

actual = normalized_path(doc.get("target_root") if isinstance(doc, dict) else None)
expected = normalized_path(target_root)
sys.exit(0 if actual and expected and actual == expected else 1)
PY
}

acceptance_two_machine_compute_workflow_status() {
  bundle_root=$1
  require_operator_evidence=${2:-0}
  installed_app_proof=$(acceptance_two_machine_installed_app_proof_summary "$bundle_root")
  release_evidence=$(acceptance_two_machine_release_evidence_summary "$bundle_root")
  python3 - "$bundle_root/meta.json" "$bundle_root" "$require_operator_evidence" "$installed_app_proof" "$release_evidence" <<'PY'
import datetime
import json
import math
import os
import sys

MAX_SWIFT_INT = 9223372036854775807

meta_path, bundle_root, require_operator_evidence = sys.argv[1], sys.argv[2], sys.argv[3] == "1"
installed_app_proof = json.loads(sys.argv[4])
release_evidence = json.loads(sys.argv[5])

with open(meta_path) as f:
    doc = json.load(f)

evidence = doc.get("evidence") or {}
roles = doc.get("roles") or {}
collection = doc.get("collection") or {}
operator = evidence.get("operator") or {}

def quote(text):
    return "'" + str(text).replace("'", "'\"'\"'") + "'"

def clean_text(value):
    return value.strip() if isinstance(value, str) else ""

def raw_text(value):
    return value if isinstance(value, str) else ""

def normalized_path(value):
    text = clean_text(value)
    if not text:
        return ""
    return os.path.normpath(text)

def parse_rfc3339(value):
    text = clean_text(value)
    if not text:
        return None
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        return datetime.datetime.fromisoformat(text)
    except ValueError:
        return None

def safe_control_plane_id(value):
    if not isinstance(value, str):
        return ""
    if not value or value in (".", "..") or value.startswith("~"):
        return ""
    if "/" in value or "\\" in value:
        return ""
    if any(ord(ch) < 32 or ord(ch) == 127 for ch in value):
        return ""
    if any(ch.isspace() for ch in value):
        return ""
    return value

def is_json_int(value):
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return False
    return math.isfinite(value) and value == math.floor(value) and -MAX_SWIFT_INT - 1 <= value <= MAX_SWIFT_INT

def string_list(value):
    return isinstance(value, list) and all(isinstance(item, str) for item in value)

def valid_discovery_advertisement(value):
    if not isinstance(value, dict):
        return False
    return (
        isinstance(value.get("service_type"), str)
        and isinstance(value.get("protocol_version"), str)
        and isinstance(value.get("ephemeral_nonce"), str)
        and string_list(value.get("capability_flags"))
    )

def valid_discovery_hint(value):
    if not isinstance(value, dict):
        return False
    return (
        isinstance(value.get("address"), str)
        and valid_discovery_advertisement(value.get("advertisement"))
        and isinstance(value.get("seen_at"), str)
        and isinstance(value.get("expires_at"), str)
        and isinstance(value.get("trusted"), bool)
    )

def valid_discovery_candidate(value):
    if not isinstance(value, dict):
        return False
    ambiguity_reasons = value.get("ambiguity_reasons")
    return (
        valid_discovery_hint(value.get("hint"))
        and isinstance(value.get("class"), str)
        and is_json_int(value.get("duplicate_count"))
        and (ambiguity_reasons is None or string_list(ambiguity_reasons))
    )

def valid_source_browse_artifact(value):
    if not isinstance(value, dict):
        return False
    candidates = value.get("candidates")
    return (
        isinstance(value.get("source"), str)
        and isinstance(value.get("listen"), str)
        and is_json_int(value.get("candidate_count"))
        and is_json_int(value.get("invalid_packets"))
        and value.get("trusted") is False
        and isinstance(candidates, list)
        and all(valid_discovery_candidate(candidate) for candidate in candidates)
    )

def valid_target_advertise_artifact(value):
    if not isinstance(value, dict):
        return False
    return (
        clean_text(value.get("status")) == "advertised"
        and isinstance(value.get("listen"), str)
        and isinstance(value.get("destination"), str)
        and isinstance(value.get("service_type"), str)
        and isinstance(value.get("protocol_version"), str)
        and isinstance(value.get("ephemeral_nonce"), str)
        and string_list(value.get("capability_flags"))
        and value.get("trusted") is False
        and isinstance(value.get("duration"), str)
        and isinstance(value.get("interval"), str)
    )

def valid_target_ready_artifact(value, meta_ready):
    if not isinstance(value, dict) or not isinstance(meta_ready, dict):
        return False
    address = raw_text(value.get("address"))
    mode = raw_text(value.get("mode"))
    verification = raw_text(value.get("verification_code"))
    meta_address = raw_text(meta_ready.get("address"))
    meta_mode = raw_text(meta_ready.get("mode"))
    meta_verification = raw_text(meta_ready.get("verification_code"))
    if len(address) == 0 or len(mode) == 0:
        return False
    if address != meta_address or mode != meta_mode:
        return False
    if mode in ("pairing", "pairing-only"):
        if len(verification) == 0 or verification != meta_verification:
            return False
    elif len(verification) > 0 and verification != meta_verification:
        return False
    if not isinstance(value.get("trusted"), bool) or not isinstance(value.get("transfer"), bool):
        return False
    optional_strings = ["receiver_address", "expires_at"]
    if any(key in value and value.get(key) is not None and not isinstance(value.get(key), str) for key in optional_strings):
        return False
    optional_bools = ["receiver_routes", "push_network"]
    if any(key in value and value.get(key) is not None and not isinstance(value.get(key), bool) for key in optional_bools):
        return False
    return True

def safe_artifact_path(relative_path):
    trimmed = clean_text(relative_path)
    if not trimmed or trimmed.startswith("/") or trimmed.startswith("~"):
        return None
    parts = trimmed.split("/")
    if any(part in ("", ".", "..") for part in parts):
        return None
    current = bundle_root
    if os.path.islink(current):
        return None
    for part in parts:
        current = os.path.join(current, part)
        if os.path.islink(current):
            return None
    return current

def artifact_exists(relative_path):
    path = safe_artifact_path(relative_path)
    return isinstance(path, str) and os.path.isfile(path) and os.stat(path).st_nlink == 1

def read_json_artifact(relative_path):
    path = safe_artifact_path(relative_path)
    if not isinstance(path, str) or not os.path.isfile(path) or os.stat(path).st_nlink != 1:
        return None
    try:
        with open(path) as f:
            return json.load(f)
    except Exception:
        return None

def valid_pairing_receipt_artifact(relative_path, expected_id):
    receipt = read_json_artifact(relative_path)
    if not isinstance(receipt, dict):
        return False
    version = receipt.get("version")
    if isinstance(version, bool) or not isinstance(version, (int, float)) or not math.isfinite(version) or version != math.floor(version) or version <= 0:
        return False
    required = [
        "id",
        "profile_id",
        "target_id",
        "source_device_id",
        "target_device_id",
        "device_public_key",
        "method",
        "verified_at",
        "protocol_version",
    ]
    values = {key: clean_text(receipt.get(key)) for key in required}
    if any(len(value) == 0 for value in values.values()):
        return False
    if values["id"] != expected_id:
        return False
    if values["device_public_key"] != values["target_device_id"]:
        return False
    if parse_rfc3339(values["verified_at"]) is None:
        return False
    if len(clean_text(receipt.get("verification_hash"))) == 0 and len(clean_text(receipt.get("verification_phrase"))) == 0:
        return False
    return True

def source_pair_matches_target_ready(source_pair, target_ready):
    if not target_ready_done:
        return True
    if not isinstance(source_pair, dict) or not isinstance(target_ready, dict):
        return False
    return (
        len(clean_text(source_pair.get("target_address"))) > 0
        and clean_text(source_pair.get("target_address")) == clean_text(target_ready.get("address"))
    )

def target_ready_receiver_transfer_ready(target_ready):
    return (
        target_ready_done
        and isinstance(target_ready, dict)
        and len(clean_text(target_ready.get("receiver_address"))) > 0
        and target_ready.get("receiver_routes") is True
        and target_ready.get("push_network") is True
        and target_ready.get("transfer") is True
    )

def source_transfer_matches_target_ready(source_transfer, target_ready):
    if not target_ready_done:
        return True
    if not isinstance(source_transfer, dict) or not target_ready_receiver_transfer_ready(target_ready):
        return False
    expected_receiver = clean_text(target_ready.get("receiver_address"))
    return (
        len(clean_text(source_transfer.get("target_address"))) > 0
        and clean_text(source_transfer.get("target_address")) == clean_text(target_ready.get("address"))
        and len(clean_text(source_transfer.get("target_mode"))) > 0
        and clean_text(source_transfer.get("target_mode")) == clean_text(target_ready.get("mode"))
        and clean_text(source_transfer.get("receiver_address")) == expected_receiver
    )

def operator_expected_machine_id(kind):
    machine = {
        "local_network": "target",
        "firewall": "target",
        "pairing_confirmation": "source",
    }.get(kind)
    if machine is None:
        return None
    artifact = read_json_artifact(f"{machine}.machine.json")
    if not isinstance(artifact, dict):
        return None
    if artifact.get("schema") != "supermover.acceptance.machine_facts.v1":
        return None
    machine_id = artifact.get("machine_id")
    if not isinstance(machine_id, str) or len(machine_id) == 0:
        return None
    return machine_id

def has_valid_operator_evidence(kind):
    record = operator.get(kind)
    if not isinstance(record, dict):
        return False
    expected_machine_id = operator_expected_machine_id(kind)
    return (
        record.get("status") == "pass"
        and len(clean_text(record.get("detail"))) > 0
        and expected_machine_id is not None
        and record.get("machine_id") == expected_machine_id
    )

def role_profile(*names):
    for name in names:
        record = roles.get(name) or {}
        profile = record.get("profile")
        if isinstance(profile, str) and len(profile) > 0 and profile != "-":
            return profile
    return None

source_profile = role_profile("source_transfer", "source_pair")
target_profile = role_profile("target", "target_import", "target_advertise")
target_root = ((evidence.get("evaluation") or {}).get("target_root") if isinstance(evidence.get("evaluation"), dict) else None) or "<target-root>"

def commands_for(step_id):
    bundle_arg = quote(bundle_root)
    script = "sh macos/script/acceptance-two-machine.sh"
    if step_id == "target_serve_phase_1":
        profile = quote(target_profile or "<target-profile>")
        return [f"{script} target-serve --profile {profile} --bundle-root {bundle_arg}"]
    if step_id == "source_browse":
        return [f"{script} source-browse --bundle-root {bundle_arg}"]
    if step_id == "target_advertise":
        profile = quote(target_profile or "<target-profile>")
        return [f"{script} target-advertise --profile {profile} --bundle-root {bundle_arg}"]
    if step_id == "source_pair":
        profile = quote(source_profile or "<source-profile>")
        return [f"{script} source-pair --profile {profile} --bundle-root {bundle_arg}"]
    if step_id == "target_import":
        profile = quote(target_profile or "<target-profile>")
        return [f"{script} target-import --profile {profile} --bundle-root {bundle_arg}"]
    if step_id == "source_transfer":
        profile = quote(source_profile or "<source-profile>")
        return [f"{script} source-transfer --profile {profile} --bundle-root {bundle_arg} --session '<session-id>'"]
    if step_id == "operator_local_network":
        return [f"{script} record-operator-evidence --bundle-root {bundle_arg} --kind local_network --status pass --detail 'accepted prompt on target'"]
    if step_id == "operator_firewall":
        return [f"{script} record-operator-evidence --bundle-root {bundle_arg} --kind firewall --status pass --detail 'allowed firewall access on target'"]
    if step_id == "operator_pairing_confirmation":
        return [f"{script} record-operator-evidence --bundle-root {bundle_arg} --kind pairing_confirmation --status pass --detail 'physical pairing code confirmed on both devices'"]
    if step_id == "source_packaging_evidence":
        return [f"{script} record-packaging-evidence --bundle-root {bundle_arg} --machine source --app '<source-app>'"]
    if step_id == "target_packaging_evidence":
        return [f"{script} record-packaging-evidence --bundle-root {bundle_arg} --machine target --app '<target-app>'"]
    if step_id == "bundle_handoff":
        return [
            f"{script} pack-bundle --bundle-root {bundle_arg} --archive '<bundle.tgz>'",
            f"{script} unpack-bundle --archive '<bundle.tgz>' --manifest '<bundle.manifest.json>' --bundle-root '<incoming-bundle>'",
            f"{script} merge-bundle --bundle-root {bundle_arg} --incoming-bundle-root '<incoming-bundle>'",
        ]
    if step_id == "review_bundle_handoff":
        return []
    if step_id == "evaluate":
        profile = quote(source_profile or "<source-profile>")
        target_root_arg = quote(target_root)
        operator_flag = " --require-operator-evidence" if require_operator_evidence else ""
        return [f"{script} evaluate --bundle-root {bundle_arg} --target-root {target_root_arg} --source-profile {profile}{operator_flag}"]
    return []

def has_output(record):
    return isinstance(record, dict) and isinstance(record.get("output"), str) and len(record["output"]) > 0

def has_dict(record):
    return isinstance(record, dict)

def preferred_relative_path(value, fallback):
    cleaned = clean_text(value)
    return cleaned or fallback

target_ready_record = evidence.get("target_ready")
target_ready_artifact = read_json_artifact("target.ready.json") if isinstance(target_ready_record, dict) else None
target_ready_done = valid_target_ready_artifact(target_ready_artifact, target_ready_record)

discovery_record = evidence.get("discovery") or {}
source_browse_record = discovery_record.get("source_browse") or {}
source_browse_record_present = isinstance(discovery_record, dict) and "source_browse" in discovery_record
source_browse_output = clean_text(source_browse_record.get("output") if isinstance(source_browse_record, dict) else None)
source_browse_artifact = read_json_artifact(source_browse_output) if source_browse_output else None
source_browse_done = (
    len(source_browse_output) > 0
    and valid_source_browse_artifact(source_browse_artifact)
)
source_browse_evaluate_ready = (not source_browse_record_present) or source_browse_done

target_advertise_record = discovery_record.get("target_advertise") or {}
target_advertise_record_present = isinstance(discovery_record, dict) and "target_advertise" in discovery_record
target_advertise_output = clean_text(
    target_advertise_record.get("output") if isinstance(target_advertise_record, dict) else None
)
target_advertise_artifact = read_json_artifact(target_advertise_output) if target_advertise_output else None
target_advertise_done = (
    len(target_advertise_output) > 0
    and valid_target_advertise_artifact(target_advertise_artifact)
)
target_advertise_evaluate_ready = (not target_advertise_record_present) or target_advertise_done

source_pair_record = evidence.get("source_pair") or {}
source_pair_output = preferred_relative_path(
    source_pair_record.get("output") if isinstance(source_pair_record, dict) else None,
    "source.pair.json",
)
source_pair_transcript = preferred_relative_path(
    source_pair_record.get("pair") if isinstance(source_pair_record, dict) else None,
    "source.pair.txt",
)
source_pair_artifact = read_json_artifact(source_pair_output)
source_pair_receipt_id = safe_control_plane_id((source_pair_artifact or {}).get("pairing_receipt_id"))
source_pair_receipt_path = clean_text((source_pair_artifact or {}).get("receipt_path"))
source_pair_receipt_ready = (
    len(source_pair_receipt_path) > 0
    and valid_pairing_receipt_artifact(source_pair_receipt_path, source_pair_receipt_id)
)
source_pair_done = (
    isinstance(source_pair_artifact, dict)
    and len(source_pair_receipt_id) > 0
    and source_pair_matches_target_ready(source_pair_artifact, target_ready_artifact)
    and source_pair_receipt_ready
    and artifact_exists(source_pair_transcript)
)

target_import_record = evidence.get("target_import") or {}
target_import_receipt_id = raw_text(target_import_record.get("pairing_receipt_id") if isinstance(target_import_record, dict) else None)
target_import_adopted = clean_text(target_import_record.get("adopted") if isinstance(target_import_record, dict) else None)
target_import_adopted_ready = len(target_import_adopted) > 0 and artifact_exists(target_import_adopted)
target_import_done = (
    has_dict(target_import_record)
    and len(target_import_receipt_id) > 0
    and source_pair_done
    and target_import_receipt_id == source_pair_receipt_id
    and target_import_adopted_ready
)
target_import_evaluate_ready = (
    has_dict(target_import_record)
    and len(target_import_receipt_id) > 0
    and source_pair_done
    and target_import_receipt_id == source_pair_receipt_id
    and target_import_adopted_ready
)

source_transfer_record = evidence.get("source_transfer") or {}
source_transfer_output = preferred_relative_path(
    source_transfer_record.get("output") if isinstance(source_transfer_record, dict) else None,
    "source.transfer.json",
)
source_transfer_verify = preferred_relative_path(
    source_transfer_record.get("verify") if isinstance(source_transfer_record, dict) else None,
    "source.verify.json",
)
source_transfer_report = preferred_relative_path(
    source_transfer_record.get("report") if isinstance(source_transfer_record, dict) else None,
    "source.report.json",
)
source_transfer_status = preferred_relative_path(
    source_transfer_record.get("status") if isinstance(source_transfer_record, dict) else None,
    "source.status.json",
)
source_transfer_health = preferred_relative_path(
    source_transfer_record.get("health") if isinstance(source_transfer_record, dict) else None,
    "source.health.json",
)
source_transfer_push = preferred_relative_path(
    source_transfer_record.get("push") if isinstance(source_transfer_record, dict) else None,
    "source.network-push.txt",
)
source_transfer_artifact = read_json_artifact(source_transfer_output)
source_transfer_session_id = safe_control_plane_id((source_transfer_artifact or {}).get("session_id"))
source_verify_artifact = read_json_artifact(source_transfer_verify)
source_report_artifact = read_json_artifact(source_transfer_report)
source_status_artifact = read_json_artifact(source_transfer_status)
source_health_artifact = read_json_artifact(source_transfer_health)
source_network_push_ready = artifact_exists(source_transfer_push)

def artifact_target_root(artifact):
    return normalized_path(artifact.get("target_root") if isinstance(artifact, dict) else None)

source_transfer_target_roots = [
    artifact_target_root(artifact)
    for artifact in [
        source_verify_artifact,
        source_report_artifact,
        source_status_artifact,
        source_health_artifact,
    ]
]
source_transfer_target_roots_consistent = (
    all(root for root in source_transfer_target_roots)
    and all(root == source_transfer_target_roots[0] for root in source_transfer_target_roots)
)
source_transfer_target_root = source_transfer_target_roots[0] if source_transfer_target_roots_consistent else ""
source_consistency_record = evidence.get("source_consistency") or {}
source_consistency_output = preferred_relative_path(
    source_consistency_record.get("output") if isinstance(source_consistency_record, dict) else None,
    "source.consistency.json",
)
source_consistency_baseline = preferred_relative_path(
    source_consistency_record.get("baseline") if isinstance(source_consistency_record, dict) else None,
    "source.baseline.json",
)
source_consistency_artifact = read_json_artifact(source_consistency_output)
source_consistency_artifact_baseline = clean_text(
    (source_consistency_artifact or {}).get("baseline")
)
if len(source_consistency_artifact_baseline) > 0:
    source_consistency_baseline = source_consistency_artifact_baseline
source_consistency_baseline_ready = artifact_exists(source_consistency_baseline)
verify_summary = source_verify_artifact.get("summary") if isinstance(source_verify_artifact, dict) else None
report_pairing = source_report_artifact.get("pairing") if isinstance(source_report_artifact, dict) else None
report_pairing_receipt_id = raw_text(report_pairing.get("receipt_id") if isinstance(report_pairing, dict) else None)
source_verify_ready = (
    isinstance(verify_summary, dict)
    and is_json_int(verify_summary.get("files_verified"))
    and is_json_int(verify_summary.get("error_findings"))
    and is_json_int(verify_summary.get("artifact_problems"))
    and verify_summary.get("files_verified") >= 1
    and verify_summary.get("error_findings") == 0
    and verify_summary.get("artifact_problems") == 0
)
source_report_ready = (
    isinstance(report_pairing, dict)
    and len(report_pairing_receipt_id) > 0
    and clean_text(report_pairing.get("status")) == "paired_receipt_valid"
    and len(source_pair_receipt_id) > 0
    and report_pairing_receipt_id == source_pair_receipt_id
)
status_overall = source_status_artifact.get("overall") if isinstance(source_status_artifact, dict) else None
status_latest_session = source_status_artifact.get("latest_session") if isinstance(source_status_artifact, dict) else None
status_counts = source_status_artifact.get("counts") if isinstance(source_status_artifact, dict) else None
source_status_ready = (
    isinstance(source_status_artifact, dict)
    and len(clean_text(source_status_artifact.get("profile_id"))) > 0
    and len(clean_text(source_status_artifact.get("target_id"))) > 0
    and len(clean_text(source_status_artifact.get("target_root"))) > 0
    and isinstance(status_overall, dict)
    and len(clean_text(status_overall.get("status"))) > 0
    and len(clean_text(status_overall.get("target_status"))) > 0
    and isinstance(status_latest_session, dict)
    and raw_text(status_latest_session.get("id")) == source_transfer_session_id
    and len(clean_text(status_latest_session.get("completeness_status"))) > 0
    and is_json_int(status_latest_session.get("files_expected"))
    and status_latest_session.get("files_expected") >= 0
    and is_json_int(status_latest_session.get("files_verified"))
    and status_latest_session.get("files_verified") >= 0
    and is_json_int(status_latest_session.get("verification_errors"))
    and status_latest_session.get("verification_errors") >= 0
    and isinstance(status_counts, dict)
    and is_json_int(status_counts.get("artifact_problems"))
    and status_counts.get("artifact_problems") >= 0
    and is_json_int(status_counts.get("network_transfers"))
    and status_counts.get("network_transfers") >= 0
)
health_summary = source_health_artifact.get("summary") if isinstance(source_health_artifact, dict) else None
health_transfers = source_health_artifact.get("network_transfers") if isinstance(source_health_artifact, dict) else None
source_health_ready = (
    isinstance(source_health_artifact, dict)
    and len(clean_text(source_health_artifact.get("target_root"))) > 0
    and isinstance(health_summary, dict)
    and is_json_int(health_summary.get("incomplete_sessions"))
    and health_summary.get("incomplete_sessions") >= 0
    and is_json_int(health_summary.get("invalid_records"))
    and health_summary.get("invalid_records") >= 0
    and is_json_int(health_summary.get("artifact_problems"))
    and health_summary.get("artifact_problems") >= 0
    and is_json_int(health_summary.get("target_drifts"))
    and health_summary.get("target_drifts") >= 0
    and is_json_int(health_summary.get("network_transfers"))
    and health_summary.get("network_transfers") >= 0
    and isinstance(health_transfers, list)
    and any(
        isinstance(transfer, dict)
        and raw_text(transfer.get("session_id")) == source_transfer_session_id
        and len(clean_text(transfer.get("status"))) > 0
        for transfer in health_transfers
    )
)
source_consistency_ready = (
    isinstance(source_consistency_artifact, dict)
    and clean_text(source_consistency_artifact.get("schema")) == "supermover.acceptance.current_source_consistency.v1"
    and clean_text(source_consistency_artifact.get("status")) == "pass"
    and clean_text(source_consistency_artifact.get("mode")) == "current_source_verified"
    and raw_text(source_consistency_artifact.get("session_id")) == source_transfer_session_id
)
evaluation_record = evidence.get("evaluation") or {}
evaluation_output = clean_text(
    evaluation_record.get("output") if isinstance(evaluation_record, dict) else None
)
evaluation_artifact = read_json_artifact(evaluation_output) if evaluation_output else None
recorded_evaluation_target_root = normalized_path(
    evaluation_artifact.get("target_root") if isinstance(evaluation_artifact, dict) else None
)
expected_evaluation_target_root_path = normalized_path(
    evaluation_record.get("target_root") if isinstance(evaluation_record, dict) else None
)
source_transfer_target_root_matches_evaluation = True
if source_transfer_target_root:
    if recorded_evaluation_target_root:
        source_transfer_target_root_matches_evaluation = source_transfer_target_root == recorded_evaluation_target_root
    elif expected_evaluation_target_root_path:
        source_transfer_target_root_matches_evaluation = source_transfer_target_root == expected_evaluation_target_root_path
source_transfer_done = (
    isinstance(source_transfer_artifact, dict)
    and len(source_transfer_session_id) > 0
    and len(source_pair_receipt_id) > 0
    and source_transfer_matches_target_ready(source_transfer_artifact, target_ready_artifact)
    and source_network_push_ready
    and source_transfer_target_roots_consistent
    and source_transfer_target_root_matches_evaluation
    and source_verify_ready
    and source_report_ready
    and source_status_ready
    and source_health_ready
    and source_consistency_ready
    and source_consistency_baseline_ready
)

steps = [
    {
        "id": "target_serve_phase_1",
        "machine": "target",
        "description": "start target pairing serve",
        "done": target_ready_done,
    },
    {
        "id": "source_browse",
        "machine": "source",
        "description": "collect source browse evidence",
        "done": source_browse_done,
    },
    {
        "id": "target_advertise",
        "machine": "target",
        "description": "collect target advertise evidence",
        "done": target_advertise_done,
    },
    {
        "id": "source_pair",
        "machine": "source",
        "description": "export source pairing receipt",
        "done": source_pair_done,
    },
    {
        "id": "target_import",
        "machine": "target",
        "description": "import pairing receipt on target",
        "done": target_import_done,
    },
    {
        "id": "source_transfer",
        "machine": "source",
        "description": "run source mTLS transfer and consistency proof",
        "done": source_transfer_done,
    },
]

if require_operator_evidence:
    steps.extend([
        {
            "id": "operator_local_network",
            "machine": "target",
            "description": "record Local Network prompt evidence",
            "done": has_valid_operator_evidence("local_network"),
        },
        {
            "id": "operator_firewall",
            "machine": "target",
            "description": "record firewall evidence",
            "done": has_valid_operator_evidence("firewall"),
        },
        {
            "id": "operator_pairing_confirmation",
            "machine": "source",
            "description": "record physical pairing confirmation evidence",
            "done": has_valid_operator_evidence("pairing_confirmation"),
        },
    ])

phase_actions = []
for step in steps:
    if not step["done"]:
        phase_actions.append({
            "machine": step["machine"],
            "step": step["id"],
            "action": step["description"],
            "commands": commands_for(step["id"]),
        })

verified_bundle_handoffs = int(installed_app_proof.get("verified_bundle_handoffs", 0))
verified_cross_machine_bundle_handoffs = int(installed_app_proof.get("verified_cross_machine_bundle_handoffs", 0))
matches_recorded_machine_pair = installed_app_proof.get("matches_recorded_machine_pair") is True
has_installed_app_machine_pair_proof = installed_app_proof.get("has_installed_app_machine_pair_proof") is True
installed_app_proof_ok = installed_app_proof.get("ok") is True
installed_app_proof_failures = installed_app_proof.get("failures") or []
role_machine_ids = installed_app_proof.get("role_machine_ids") or {}
machine_fact_ids = installed_app_proof.get("machine_fact_ids") or {}
machine_fact_artifact_ids = installed_app_proof.get("machine_fact_artifact_ids") or {}
machine_facts_consistent = installed_app_proof.get("machine_facts_consistent") is True
source_app_audit_ready = release_evidence.get("source_app_audit_ready") is True
target_app_audit_ready = release_evidence.get("target_app_audit_ready") is True
source_notarization_ready = release_evidence.get("source_notarization_ready") is True
target_notarization_ready = release_evidence.get("target_notarization_ready") is True
installed_app_release_evidence_failures = release_evidence.get("failures") or []
installed_app_release_evidence_ok = release_evidence.get("ok") is True

def has_clean_id(mapping, key):
    if not isinstance(mapping, dict):
        return False
    value = mapping.get(key)
    return isinstance(value, str) and len(value.strip()) > 0

machine_fact_ids_present = has_clean_id(machine_fact_ids, "source") and has_clean_id(machine_fact_ids, "target")
installed_app_blocked_reason = installed_app_proof.get("blocked_reason")
missing_installed_app_requirements = installed_app_proof.get("missing_requirements") or []
requires_machine_identity_correction = installed_app_proof.get("requires_machine_identity_correction") is True
requires_bundle_handoff_proof = installed_app_proof.get("requires_bundle_handoff_proof") is True
final_evaluation_collection_detail = installed_app_proof.get("final_evaluation_collection_detail")
final_evaluation_machine_facts_detail = installed_app_proof.get("final_evaluation_machine_facts_detail")
final_evaluation_bundle_handoff_detail = installed_app_proof.get("final_evaluation_bundle_handoff_detail")
primary_failure = installed_app_proof.get("primary_failure")
failure_message = installed_app_proof.get("failure_message")
recorded_evaluation_require_operator_evidence = None
if isinstance(evaluation_artifact, dict) and isinstance(evaluation_artifact.get("require_operator_evidence"), bool):
    recorded_evaluation_require_operator_evidence = evaluation_artifact.get("require_operator_evidence")
elif isinstance(evaluation_record, dict) and isinstance(evaluation_record.get("require_operator_evidence"), bool):
    recorded_evaluation_require_operator_evidence = evaluation_record.get("require_operator_evidence")
evaluation_pairing_receipt_id = raw_text(
    evaluation_artifact.get("pairing_receipt_id") if isinstance(evaluation_artifact, dict) else None
)
evaluation_session_id = raw_text(
    evaluation_artifact.get("session_id") if isinstance(evaluation_artifact, dict) else None
)
evaluation_target_root = clean_text(
    evaluation_artifact.get("target_root") if isinstance(evaluation_artifact, dict) else None
)
evaluation_status = clean_text(
    evaluation_artifact.get("status") if isinstance(evaluation_artifact, dict) else None
)
expected_evaluation_target_root = clean_text(
    evaluation_record.get("target_root") if isinstance(evaluation_record, dict) else None
)
expected_evaluation_target_root_matches = (
    len(expected_evaluation_target_root_path) == 0
    or recorded_evaluation_target_root == expected_evaluation_target_root_path
)
current_evaluation_recorded = (
    clean_text(doc.get("status")) == "evidence_collected"
    and isinstance(evaluation_artifact, dict)
    and clean_text(evaluation_artifact.get("schema")) == "supermover.acceptance.two_machine.v1"
    and evaluation_status == "evidence_collected"
    and (not require_operator_evidence or recorded_evaluation_require_operator_evidence is True)
    and (not source_pair_done or evaluation_pairing_receipt_id == source_pair_receipt_id)
    and (len(source_transfer_session_id) == 0 or evaluation_session_id == source_transfer_session_id)
    and expected_evaluation_target_root_matches
)

next_actions = []
if require_operator_evidence:
    if not installed_app_release_evidence_ok:
        if not source_app_audit_ready or not source_notarization_ready:
            next_actions.append({
                "machine": "source",
                "step": "source_packaging_evidence",
                "action": "record source release packaging evidence from the installed app before final evaluate",
                "commands": commands_for("source_packaging_evidence"),
            })
        if not target_app_audit_ready or not target_notarization_ready:
            next_actions.append({
                "machine": "target",
                "step": "target_packaging_evidence",
                "action": "record target release packaging evidence from the installed app before final evaluate",
                "commands": commands_for("target_packaging_evidence"),
            })
    elif not installed_app_proof_ok:
        if installed_app_blocked_reason == "invalid_collection":
            next_actions.append({
                "machine": "either",
                "step": "review_collection",
                "action": "correct collection.mode=two_machine and collection.machine_count>=2 before installed-app evaluation",
                "commands": [],
            })
        elif installed_app_blocked_reason == "contradictory_verified_bundle_handoffs":
            next_actions.append({
                "machine": "either",
                "step": "review_bundle_handoff",
                "action": "remove contradictory installed-app bundle_handoff evidence before final evaluate",
                "commands": commands_for("review_bundle_handoff"),
            })
        elif requires_machine_identity_correction:
            next_actions.extend([
                {
                    "machine": "target",
                    "step": "target_serve_phase_1",
                    "action": "rewrite target role and target.machine.json evidence from the installed app before handoff proof",
                    "commands": commands_for("target_serve_phase_1"),
                },
                {
                    "machine": "source",
                    "step": "source_pair",
                    "action": "rewrite source role and source.machine.json evidence from the installed app before handoff proof",
                    "commands": commands_for("source_pair"),
                },
            ])
        elif requires_bundle_handoff_proof:
            next_actions.append({
                "machine": "either",
                "step": "bundle_handoff",
                "action": "pack/unpack/merge bundle evidence across distinct machines before installed-app evaluation",
                "commands": commands_for("bundle_handoff"),
            })
        else:
            next_actions.append({
                "machine": "either",
                "step": "review_collection",
                "action": "review installed-app collection proof before final evaluate",
                "commands": [],
            })
    elif not current_evaluation_recorded:
        if not target_ready_done:
            next_actions.append({
                "machine": "target",
                "step": "target_serve_phase_1",
                "action": "start target pairing serve",
                "commands": commands_for("target_serve_phase_1"),
            })
        if not source_browse_evaluate_ready:
            next_actions.append({
                "machine": "source",
                "step": "source_browse",
                "action": "collect source browse evidence",
                "commands": commands_for("source_browse"),
            })
        if not target_advertise_evaluate_ready:
            next_actions.append({
                "machine": "target",
                "step": "target_advertise",
                "action": "collect target advertise evidence",
                "commands": commands_for("target_advertise"),
            })
        if not source_pair_done:
            next_actions.append({
                "machine": "source",
                "step": "source_pair",
                "action": "export source pairing receipt",
                "commands": commands_for("source_pair"),
            })
        if not target_import_evaluate_ready:
            next_actions.append({
                "machine": "target",
                "step": "target_import",
                "action": "import pairing receipt on target",
                "commands": commands_for("target_import"),
            })
        if not source_transfer_done:
            next_actions.append({
                "machine": "source",
                "step": "source_transfer",
                "action": "run source mTLS transfer and consistency proof",
                "commands": commands_for("source_transfer"),
            })
        if not has_valid_operator_evidence("local_network"):
            next_actions.append({
                "machine": "target",
                "step": "operator_local_network",
                "action": "record Local Network prompt evidence",
                "commands": commands_for("operator_local_network"),
            })
        if not has_valid_operator_evidence("firewall"):
            next_actions.append({
                "machine": "target",
                "step": "operator_firewall",
                "action": "record firewall evidence",
                "commands": commands_for("operator_firewall"),
            })
        if not has_valid_operator_evidence("pairing_confirmation"):
            next_actions.append({
                "machine": "source",
                "step": "operator_pairing_confirmation",
                "action": "record physical pairing confirmation evidence",
                "commands": commands_for("operator_pairing_confirmation"),
            })
        if not next_actions:
            next_actions.append({
                "machine": "either",
                "step": "evaluate",
                "action": "run acceptance evaluate against the merged bundle",
                "commands": commands_for("evaluate"),
            })
    else:
        next_actions = phase_actions
else:
    next_actions = phase_actions
    if not next_actions and not current_evaluation_recorded:
        next_actions.append({
            "machine": "either",
            "step": "evaluate",
            "action": "run acceptance evaluate against the merged bundle",
            "commands": commands_for("evaluate"),
        })

result = {
    "schema": "supermover.acceptance.workflow_status.v1",
    "bundle_status": doc.get("status", "in_progress"),
    "collection_mode": collection.get("mode", "unknown"),
    "machine_count": collection.get("machine_count", 0),
    "verified_bundle_handoffs": verified_bundle_handoffs,
    "verified_cross_machine_bundle_handoffs": verified_cross_machine_bundle_handoffs,
    "matches_recorded_machine_pair": matches_recorded_machine_pair,
    "has_installed_app_machine_pair_proof": has_installed_app_machine_pair_proof,
    "installed_app_proof_ok": installed_app_proof_ok,
    "installed_app_proof_failures": installed_app_proof_failures,
    "source_app_audit_ready": source_app_audit_ready,
    "target_app_audit_ready": target_app_audit_ready,
    "source_notarization_ready": source_notarization_ready,
    "target_notarization_ready": target_notarization_ready,
    "installed_app_release_evidence_ok": installed_app_release_evidence_ok,
    "installed_app_release_evidence_failures": installed_app_release_evidence_failures,
    "ok": current_evaluation_recorded and len(next_actions) == 0,
    "failures": installed_app_proof_failures,
    "blocked_reason": installed_app_blocked_reason,
    "missing_requirements": missing_installed_app_requirements,
    "primary_failure": primary_failure,
    "failure_message": failure_message,
    "final_evaluation_collection_detail": final_evaluation_collection_detail,
    "final_evaluation_machine_facts_detail": final_evaluation_machine_facts_detail,
    "final_evaluation_bundle_handoff_detail": final_evaluation_bundle_handoff_detail,
    "installed_app_blocked_reason": installed_app_blocked_reason,
    "missing_installed_app_requirements": missing_installed_app_requirements,
    "requires_machine_identity_correction": requires_machine_identity_correction,
    "requires_bundle_handoff_proof": requires_bundle_handoff_proof,
    "role_machine_ids": role_machine_ids,
    "machine_fact_ids": machine_fact_ids,
    "machine_fact_artifact_ids": machine_fact_artifact_ids,
    "machine_facts_consistent": machine_facts_consistent,
    "steps": steps,
    "next_actions": next_actions,
}

json.dump(result, sys.stdout, indent=2)
sys.stdout.write("\n")
PY
}

acceptance_two_machine_workflow_status() {
  bundle_root=$1
  require_operator_evidence=${2:-0}
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  acceptance_two_machine_refresh_workflow_summary "$bundle_root"
  acceptance_two_machine_compute_workflow_status "$bundle_root" "$require_operator_evidence"
}

acceptance_two_machine_workflow_summary_json() {
  bundle_root=$1
  require_operator_evidence=${2:-0}
  acceptance_two_machine_compute_workflow_status "$bundle_root" "$require_operator_evidence" | jq -c .
}

acceptance_two_machine_refresh_workflow_summary() {
  bundle_root=$1
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  default_summary=$(acceptance_two_machine_workflow_summary_json "$bundle_root" 0)
  operator_summary=$(acceptance_two_machine_workflow_summary_json "$bundle_root" 1)
  acceptance_write_json "$bundle_root/workflow.summary.json" \
    --arg schema "supermover.acceptance.workflow_summary.v1" \
    --argjson default_summary "$default_summary" \
    --argjson require_operator_evidence_summary "$operator_summary" \
    '{
      schema: $schema,
      default: $default_summary,
      require_operator_evidence: $require_operator_evidence_summary
    }'
  acceptance_update_bundle_meta_locked "$bundle_root" \
    --argjson default_summary "$default_summary" \
    --argjson require_operator_evidence_summary "$operator_summary" \
    '.evidence.workflow_summary = {
      output: "workflow.summary.json",
      default: $default_summary,
      require_operator_evidence: $require_operator_evidence_summary
    }'
}

acceptance_two_machine_installed_app_machine_pair_summary() {
  bundle_root=$1
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  python3 - "$bundle_root/meta.json" "$bundle_root" <<'PY'
import json
import os
import sys

meta_path, bundle_root = sys.argv[1:3]

with open(meta_path) as f:
    doc = json.load(f)

roles = doc.get("roles") or {}
evidence = doc.get("evidence") or {}

def clean_id(value):
    if not isinstance(value, str):
        return None
    value = value.strip()
    return value or None

def safe_artifact_path(relative_path):
    trimmed = clean_id(relative_path)
    if trimmed is None or trimmed.startswith("/") or trimmed.startswith("~"):
        return None
    parts = trimmed.split("/")
    if any(part in ("", ".", "..") for part in parts):
        return None
    current = bundle_root
    if os.path.islink(current):
        return None
    for part in parts:
        current = os.path.join(current, part)
        if os.path.islink(current):
            return None
    if not os.path.isfile(current) or os.stat(current).st_nlink != 1:
        return None
    return current

def distinct_pair(mapping):
    source = clean_id(mapping.get("source"))
    target = clean_id(mapping.get("target"))
    if source is None or target is None or source == target:
        return None
    return {"source": source, "target": target}

role_machine_ids = {
    "source": clean_id((roles.get("source_pair") or {}).get("machine_id")),
    "target": clean_id((roles.get("target") or {}).get("machine_id")),
}

machine_facts = evidence.get("machine_facts") or {}
source_machine_facts = machine_facts.get("source") or {}
target_machine_facts = machine_facts.get("target") or {}
machine_fact_ids = {
    "source": clean_id(source_machine_facts.get("machine_id")),
    "target": clean_id(target_machine_facts.get("machine_id")),
}
machine_fact_outputs = {
    "source": clean_id(source_machine_facts.get("output")),
    "target": clean_id(target_machine_facts.get("output")),
}
canonical_machine_fact_artifacts = {
    "source": "source.machine.json",
    "target": "target.machine.json",
}

def load_machine_fact_artifact(machine):
    relative_path = canonical_machine_fact_artifacts.get(machine)
    if relative_path is None:
        return None
    artifact_path = safe_artifact_path(relative_path)
    if artifact_path is None:
        return None
    try:
        with open(artifact_path) as f:
            artifact = json.load(f)
    except (OSError, json.JSONDecodeError):
        return None
    if not isinstance(artifact, dict):
        return None
    machine_id = clean_id(artifact.get("machine_id"))
    if machine_id is None:
        return None
    return {
        "output": relative_path,
        "schema": artifact.get("schema"),
        "machine_id": machine_id,
        "machine_label": artifact.get("machine_label"),
    }

machine_fact_artifacts = {
    "source": load_machine_fact_artifact("source"),
    "target": load_machine_fact_artifact("target"),
}
machine_fact_artifact_ids = {
    machine: (artifact or {}).get("machine_id")
    for machine, artifact in machine_fact_artifacts.items()
}

machine_facts_consistent = True
for machine in ("source", "target"):
    meta_record = machine_facts.get(machine) or {}
    artifact = machine_fact_artifacts.get(machine)
    output = machine_fact_outputs.get(machine)
    if output is not None and (
        artifact is None or artifact.get("schema") != "supermover.acceptance.machine_facts.v1"
    ):
        machine_facts_consistent = False
        break
    if artifact is not None:
        meta_machine_id = clean_id(meta_record.get("machine_id"))
        if meta_machine_id is not None and meta_machine_id != artifact["machine_id"]:
            machine_facts_consistent = False
            break

role_machine_pair = distinct_pair(role_machine_ids)
machine_fact_pair = distinct_pair(machine_fact_ids)
machine_fact_artifact_pair = distinct_pair(machine_fact_artifact_ids)

role_matches_meta_machine_facts = (
    role_machine_pair is not None
    and machine_fact_pair is not None
    and role_machine_pair == machine_fact_pair
)
role_matches_machine_fact_artifacts = (
    role_machine_pair is not None
    and machine_fact_artifact_pair is not None
    and role_machine_pair == machine_fact_artifact_pair
)
meta_matches_machine_fact_artifacts = (
    machine_fact_pair is not None
    and machine_fact_artifact_pair is not None
    and machine_fact_pair == machine_fact_artifact_pair
)

installed_app_machine_pair = (
    machine_fact_artifact_pair
    if machine_facts_consistent and role_matches_meta_machine_facts and role_matches_machine_fact_artifacts and meta_matches_machine_fact_artifacts
    else None
)

machine_fact_artifacts_present = machine_fact_artifacts.get("source") is not None and machine_fact_artifacts.get("target") is not None
machine_fact_artifacts_schema_ok = (
    machine_fact_artifacts_present
    and (machine_fact_artifacts.get("source") or {}).get("schema") == "supermover.acceptance.machine_facts.v1"
    and (machine_fact_artifacts.get("target") or {}).get("schema") == "supermover.acceptance.machine_facts.v1"
)
machine_fact_artifact_ids_present = (
    machine_fact_artifact_ids.get("source") is not None
    and machine_fact_artifact_ids.get("target") is not None
)
machine_fact_artifact_ids_distinct = (
    machine_fact_artifact_ids_present
    and machine_fact_artifact_ids.get("source") != machine_fact_artifact_ids.get("target")
)

json.dump({
    "role_machine_ids": role_machine_ids,
    "machine_fact_ids": machine_fact_ids,
    "machine_fact_artifact_ids": machine_fact_artifact_ids,
    "machine_fact_artifacts_present": machine_fact_artifacts_present,
    "machine_fact_artifacts_schema_ok": machine_fact_artifacts_schema_ok,
    "machine_fact_artifact_ids_present": machine_fact_artifact_ids_present,
    "machine_fact_artifact_ids_distinct": machine_fact_artifact_ids_distinct,
    "machine_facts_consistent": machine_facts_consistent,
    "role_machine_pair": role_machine_pair,
    "machine_fact_pair": machine_fact_pair,
    "machine_fact_artifact_pair": machine_fact_artifact_pair,
    "role_matches_meta_machine_facts": role_matches_meta_machine_facts,
    "role_matches_machine_fact_artifacts": role_matches_machine_fact_artifacts,
    "meta_matches_machine_fact_artifacts": meta_matches_machine_fact_artifacts,
    "installed_app_machine_pair": installed_app_machine_pair,
    "has_installed_app_machine_pair_proof": installed_app_machine_pair is not None,
}, sys.stdout)
sys.stdout.write("\n")
PY
}

acceptance_two_machine_bundle_handoff_summary() {
  bundle_root=$1
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  machine_pair_summary=$(acceptance_two_machine_installed_app_machine_pair_summary "$bundle_root")
  python3 - "$bundle_root/meta.json" "$machine_pair_summary" <<'PY'
import json
import sys

meta_path = sys.argv[1]
machine_pair_summary = json.loads(sys.argv[2])

with open(meta_path) as f:
    doc = json.load(f)

evidence = doc.get("evidence") or {}

verified_handoffs = [
    item for item in (evidence.get("bundle_handoffs") or [])
    if isinstance(item, dict) and item.get("verified") is True
]
role_machine_ids = machine_pair_summary.get("role_machine_ids") or {}
machine_fact_ids = machine_pair_summary.get("machine_fact_ids") or {}
machine_fact_artifact_ids = machine_pair_summary.get("machine_fact_artifact_ids") or {}
machine_facts_consistent = machine_pair_summary.get("machine_facts_consistent") is True
installed_app_machine_pair = machine_pair_summary.get("installed_app_machine_pair") or {}
has_installed_app_machine_pair_proof = machine_pair_summary.get("has_installed_app_machine_pair_proof") is True
expected_machine_ids = {
    machine_id
    for machine_id in installed_app_machine_pair.values()
    if isinstance(machine_id, str) and len(machine_id) > 0
}

matched_handoffs = [
    item for item in verified_handoffs
    if isinstance(item.get("exporting_machine_id"), str)
    and len(item.get("exporting_machine_id")) > 0
    and isinstance(item.get("importing_machine_id"), str)
    and len(item.get("importing_machine_id")) > 0
    and item.get("exporting_machine_id") != item.get("importing_machine_id")
    and has_installed_app_machine_pair_proof
    and len(expected_machine_ids) == 2
    and {item.get("exporting_machine_id"), item.get("importing_machine_id")} == expected_machine_ids
]
raw_contradictory_handoffs = [
    item for item in verified_handoffs
    if isinstance(item.get("exporting_machine_id"), str)
    and len(item.get("exporting_machine_id")) > 0
    and isinstance(item.get("importing_machine_id"), str)
    and len(item.get("importing_machine_id")) > 0
    and item.get("exporting_machine_id") != item.get("importing_machine_id")
    and has_installed_app_machine_pair_proof
    and len(expected_machine_ids) == 2
    and {item.get("exporting_machine_id"), item.get("importing_machine_id")} != expected_machine_ids
]
contradictory_handoffs = raw_contradictory_handoffs if len(matched_handoffs) > 0 else []

json.dump({
    "verified_bundle_handoffs": len(verified_handoffs),
    "verified_cross_machine_bundle_handoffs": len(matched_handoffs),
    "contradictory_verified_bundle_handoffs": len(contradictory_handoffs),
    "role_machine_ids": role_machine_ids,
    "machine_fact_ids": machine_fact_ids,
    "machine_fact_artifact_ids": machine_fact_artifact_ids,
    "machine_fact_artifacts_present": machine_pair_summary.get("machine_fact_artifacts_present") is True,
    "machine_fact_artifacts_schema_ok": machine_pair_summary.get("machine_fact_artifacts_schema_ok") is True,
    "machine_fact_artifact_ids_present": machine_pair_summary.get("machine_fact_artifact_ids_present") is True,
    "machine_fact_artifact_ids_distinct": machine_pair_summary.get("machine_fact_artifact_ids_distinct") is True,
    "machine_facts_consistent": machine_facts_consistent,
    "installed_app_machine_pair": installed_app_machine_pair,
    "has_installed_app_machine_pair_proof": has_installed_app_machine_pair_proof,
    "has_matched_recorded_machine_pair_handoff": len(matched_handoffs) > 0,
    "matches_recorded_machine_pair": len(matched_handoffs) > 0 and len(contradictory_handoffs) == 0,
}, sys.stdout)
sys.stdout.write("\n")
PY
}

acceptance_two_machine_installed_app_proof_summary() {
  bundle_root=$1
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  handoff_summary=$(acceptance_two_machine_bundle_handoff_summary "$bundle_root")
  python3 - "$bundle_root/meta.json" "$bundle_root" "$handoff_summary" <<'PY'
import json
import sys

meta_path, bundle_root, handoff_summary_json = sys.argv[1:4]

with open(meta_path) as f:
    doc = json.load(f)

handoff_summary = json.loads(handoff_summary_json)

collection = doc.get("collection") or {}
roles = doc.get("roles") or {}

def clean_id(value):
    if not isinstance(value, str):
        return None
    value = value.strip()
    return value or None

role_source_machine_id = clean_id((roles.get("source_pair") or {}).get("machine_id"))
role_target_machine_id = clean_id((roles.get("target") or {}).get("machine_id"))
machine_count = collection.get("machine_count") if isinstance(collection.get("machine_count"), int) else 0
machine_fact_artifact_ids = handoff_summary.get("machine_fact_artifact_ids") or {}
source_machine_id = clean_id(machine_fact_artifact_ids.get("source"))
target_machine_id = clean_id(machine_fact_artifact_ids.get("target"))

collection_ok = (
    collection.get("mode") == "two_machine"
    and machine_count >= 2
)
role_machine_ids_present = role_source_machine_id is not None and role_target_machine_id is not None
role_machine_ids_distinct = role_machine_ids_present and role_source_machine_id != role_target_machine_id
machine_fact_artifacts_present = handoff_summary.get("machine_fact_artifacts_present") is True
machine_fact_artifacts_schema_ok = handoff_summary.get("machine_fact_artifacts_schema_ok") is True
machine_fact_artifact_ids_present = handoff_summary.get("machine_fact_artifact_ids_present") is True
machine_fact_artifact_ids_distinct = handoff_summary.get("machine_fact_artifact_ids_distinct") is True
machine_fact_ids = handoff_summary.get("machine_fact_ids") or {}

def has_clean_id(mapping, key):
    if not isinstance(mapping, dict):
        return False
    value = mapping.get(key)
    return isinstance(value, str) and len(value.strip()) > 0

machine_fact_ids_present = has_clean_id(machine_fact_ids, "source") and has_clean_id(machine_fact_ids, "target")

failures = []
if not collection_ok:
    failures.append("invalid_collection")
if not role_machine_ids_present:
    failures.append("missing_role_machine_ids")
elif not role_machine_ids_distinct:
    failures.append("same_role_machine_ids")
if not machine_fact_artifacts_present:
    failures.append("missing_machine_fact_artifacts")
elif not machine_fact_artifacts_schema_ok:
    failures.append("invalid_machine_fact_artifact_schema")
elif not machine_fact_artifact_ids_present:
    failures.append("missing_machine_fact_artifact_ids")
elif not machine_fact_artifact_ids_distinct:
    failures.append("same_machine_fact_artifact_ids")
if handoff_summary.get("verified_bundle_handoffs", 0) < 1:
    failures.append("missing_verified_bundle_handoffs")
if handoff_summary.get("contradictory_verified_bundle_handoffs", 0) > 0:
    failures.append("contradictory_verified_bundle_handoffs")
if handoff_summary.get("has_matched_recorded_machine_pair_handoff") is not True:
    failures.append("handoff_does_not_match_recorded_machine_pair")

blocked_reason = None
if not collection_ok:
    blocked_reason = "invalid_collection"
elif role_machine_ids_present and not role_machine_ids_distinct:
    blocked_reason = "same_role_machine_ids"
elif machine_fact_artifacts_present and not machine_fact_artifacts_schema_ok:
    blocked_reason = "invalid_machine_fact_artifact_schema"
elif machine_fact_artifact_ids_present and not machine_fact_artifact_ids_distinct:
    blocked_reason = "same_machine_fact_artifact_ids"
elif handoff_summary.get("machine_facts_consistent") is not True:
    blocked_reason = "machine_facts_inconsistent"
elif (
    role_machine_ids_present
    and machine_fact_artifacts_present
    and machine_fact_artifact_ids_present
    and machine_fact_ids_present
    and handoff_summary.get("has_installed_app_machine_pair_proof") is not True
):
    blocked_reason = "conflicting_role_and_machine_facts"
elif "contradictory_verified_bundle_handoffs" in failures:
    blocked_reason = "contradictory_verified_bundle_handoffs"

missing_requirements = []
if not role_machine_ids_present:
    missing_requirements.append("role_machine_ids")
if not machine_fact_artifacts_present:
    missing_requirements.append("machine_fact_artifacts")
elif not machine_fact_artifact_ids_present:
    missing_requirements.append("machine_fact_artifact_ids")
elif not machine_fact_ids_present:
    missing_requirements.append("machine_fact_metadata")
if handoff_summary.get("verified_bundle_handoffs", 0) < 1:
    missing_requirements.append("verified_bundle_handoffs")

requires_machine_identity_correction = (
    blocked_reason is not None
    and blocked_reason != "contradictory_verified_bundle_handoffs"
) or any(
    requirement != "verified_bundle_handoffs"
    for requirement in missing_requirements
)
requires_bundle_handoff_proof = (
    blocked_reason is None
    and len(failures) > 0
    and not requires_machine_identity_correction
    and "handoff_does_not_match_recorded_machine_pair" in failures
)

final_evaluation_collection_detail = None
if blocked_reason == "invalid_collection":
    collection_mode = collection.get("mode")
    if collection_mode != "two_machine":
        label = "missing collection.mode" if not isinstance(collection_mode, str) or len(collection_mode) == 0 else f"collection.mode={collection_mode}"
        final_evaluation_collection_detail = label
    else:
        final_evaluation_collection_detail = f"collection.machine_count={machine_count}"
elif blocked_reason == "same_role_machine_ids":
    shared_machine_id = role_source_machine_id or role_target_machine_id or "unknown"
    final_evaluation_collection_detail = f"source_pair and target share machine_id={shared_machine_id}"
else:
    if role_source_machine_id is None:
        final_evaluation_collection_detail = "missing roles.source_pair.machine_id"
    elif role_target_machine_id is None:
        final_evaluation_collection_detail = "missing roles.target.machine_id"

final_evaluation_machine_facts_detail = None
if blocked_reason == "same_machine_fact_artifact_ids":
    shared_machine_id = source_machine_id or target_machine_id or "unknown"
    final_evaluation_machine_facts_detail = f"source and target machine facts share machine_id={shared_machine_id}"
elif blocked_reason in ("machine_facts_inconsistent", "conflicting_role_and_machine_facts"):
    final_evaluation_machine_facts_detail = "roles.source_pair/target machine_id do not match source.machine.json and target.machine.json"

final_evaluation_bundle_handoff_detail = None
if "missing_verified_bundle_handoffs" in failures:
    final_evaluation_bundle_handoff_detail = "missing verified bundle_handoffs"
elif "contradictory_verified_bundle_handoffs" in failures:
    final_evaluation_bundle_handoff_detail = "bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair"
elif "handoff_does_not_match_recorded_machine_pair" in failures:
    final_evaluation_bundle_handoff_detail = "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"

failure_message = (
    final_evaluation_collection_detail
    or final_evaluation_machine_facts_detail
    or final_evaluation_bundle_handoff_detail
)

json.dump({
    "collection_mode": collection.get("mode"),
    "machine_count": machine_count,
    "role_machine_ids": {
        "source": role_source_machine_id,
        "target": role_target_machine_id,
    },
    "machine_fact_ids": machine_fact_ids,
    "machine_fact_artifact_ids": {
        "source": source_machine_id,
        "target": target_machine_id,
    },
    "collection_ok": collection_ok,
    "role_machine_ids_present": role_machine_ids_present,
    "role_machine_ids_distinct": role_machine_ids_distinct,
    "machine_fact_artifacts_present": machine_fact_artifacts_present,
    "machine_fact_artifacts_schema_ok": machine_fact_artifacts_schema_ok,
    "machine_fact_artifact_ids_present": machine_fact_artifact_ids_present,
    "machine_fact_artifact_ids_distinct": machine_fact_artifact_ids_distinct,
    "verified_bundle_handoffs": handoff_summary.get("verified_bundle_handoffs", 0),
    "verified_cross_machine_bundle_handoffs": handoff_summary.get("verified_cross_machine_bundle_handoffs", 0),
    "matches_recorded_machine_pair": handoff_summary.get("matches_recorded_machine_pair") is True,
    "contradictory_verified_bundle_handoffs": handoff_summary.get("contradictory_verified_bundle_handoffs", 0),
    "machine_facts_consistent": handoff_summary.get("machine_facts_consistent") is True,
    "installed_app_machine_pair": handoff_summary.get("installed_app_machine_pair"),
    "has_installed_app_machine_pair_proof": handoff_summary.get("has_installed_app_machine_pair_proof") is True,
    "blocked_reason": blocked_reason,
    "missing_requirements": missing_requirements,
    "requires_machine_identity_correction": requires_machine_identity_correction,
    "requires_bundle_handoff_proof": requires_bundle_handoff_proof,
    "final_evaluation_collection_detail": final_evaluation_collection_detail,
    "final_evaluation_machine_facts_detail": final_evaluation_machine_facts_detail,
    "final_evaluation_bundle_handoff_detail": final_evaluation_bundle_handoff_detail,
    "primary_failure": (failures[0] if failures else None),
    "failure_message": failure_message,
    "ok": len(failures) == 0,
    "failures": failures,
}, sys.stdout)
sys.stdout.write("\n")
PY
}

acceptance_two_machine_release_evidence_summary() {
  bundle_root=$1
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  python3 - "$bundle_root" <<'PY'
import json
import os
import re
import sys

bundle_root = sys.argv[1]
notary_submission_uuid_re = re.compile(r"^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$")

def clean_text(value):
    if not isinstance(value, str):
        return None
    value = value.strip()
    return value or None

def release_ready_notary_submission_id(value):
    value = clean_text(value)
    return value is not None and notary_submission_uuid_re.fullmatch(value) is not None

def normalized_notary_submission_id(value):
    value = clean_text(value)
    if value is None or notary_submission_uuid_re.fullmatch(value) is None:
        return None
    return value.lower()

def safe_artifact_path(relative_path):
    trimmed = clean_text(relative_path)
    if trimmed is None or trimmed.startswith("/") or trimmed.startswith("~"):
        return None
    parts = trimmed.split("/")
    if any(part in ("", ".", "..") for part in parts):
        return None
    current = bundle_root
    if os.path.islink(current):
        return None
    for part in parts:
        current = os.path.join(current, part)
        if os.path.islink(current):
            return None
    if not os.path.isfile(current) or os.stat(current).st_nlink != 1:
        return None
    return current

def read_bundle_json(relative_path):
    path = safe_artifact_path(relative_path)
    if path is None:
        return None
    try:
        with open(path) as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError, OSError):
        return None

def accepted_notary_log(relative_path, submission_id):
    expected_job_id = normalized_notary_submission_id(submission_id)
    if expected_job_id is None:
        return False
    document = read_bundle_json(relative_path)
    if not isinstance(document, dict):
        return False
    issues = document.get("issues")
    actual_job_id = normalized_notary_submission_id(document.get("jobId"))
    return (
        document.get("status") == "Accepted"
        and (issues is None or isinstance(issues, list))
        and actual_job_id == expected_job_id
    )

def machine_app_audit(machine):
    document = read_bundle_json(f"{machine}.app-audit.json")
    provenance = read_bundle_json(f"{machine}.provenance.json")
    summary = document.get("summary") if isinstance(document, dict) else None
    provenance_record = document.get("provenance") if isinstance(document, dict) else None
    app_path = clean_text((document or {}).get("app_path"))
    provenance_ok = isinstance(provenance, dict) and provenance.get("schema") == "supermover.macos.provenance.v1"
    provenance_manifest = provenance_record.get("manifest") if isinstance(provenance_record, dict) else None
    provenance_matches = provenance_ok and provenance_manifest == provenance
    ready = (
        isinstance(document, dict)
        and document.get("schema") == "supermover.macos.app_audit.v1"
        and document.get("status") == "pass"
        and clean_text(document.get("readiness")) == "distribution_ready"
        and isinstance(summary, dict)
        and summary.get("pass_ready") is True
        and app_path is not None
        and provenance_matches
    )
    if ready:
        failure_message = None
    elif not (
        isinstance(document, dict)
        and document.get("schema") == "supermover.macos.app_audit.v1"
        and document.get("status") == "pass"
        and clean_text(document.get("readiness")) == "distribution_ready"
        and isinstance(summary, dict)
        and summary.get("pass_ready") is True
    ):
        failure_message = f"{machine}.app-audit.json is not install-ready"
    elif app_path is None:
        failure_message = f"{machine}.app-audit.json does not record app_path"
    else:
        failure_message = f"{machine}.app-audit.json does not match {machine}.provenance.json"
    return {
        "ready": ready,
        "app_path": app_path,
        "status": clean_text((document or {}).get("status")),
        "readiness": clean_text((document or {}).get("readiness")),
        "pass_ready": isinstance(summary, dict) and summary.get("pass_ready") is True,
        "failure_message": failure_message,
    }

def machine_notarization(machine, app_audit):
    document = read_bundle_json(f"{machine}.notarization.json")
    audit = document.get("audit") if isinstance(document, dict) else None
    audit_path = clean_text(audit.get("path")) if isinstance(audit, dict) else None
    expected_audit_path = f"{app_audit.get('app_path')}.notary/post-staple.audit.json" if clean_text(app_audit.get("app_path")) is not None else None
    auth_mode = clean_text(document.get("auth_mode")) if isinstance(document, dict) else None
    notary_log_path = clean_text(document.get("notary_log", {}).get("path")) if isinstance(document, dict) and isinstance(document.get("notary_log"), dict) else None
    expected_notary_log_path = f"{machine}.notary-log.json"
    notary_log_artifact_path = safe_artifact_path(notary_log_path) if notary_log_path is not None else None
    status_ready = (
        isinstance(document, dict)
        and document.get("schema") == "supermover.macos.notarization.v1"
        and document.get("status") == "pass"
        and isinstance(document.get("submission"), dict)
        and release_ready_notary_submission_id(document["submission"].get("id"))
        and document["submission"].get("status") == "Accepted"
        and auth_mode in ("keychain_profile", "api_key", "apple_id")
        and document.get("failure") is None
        and isinstance(document.get("notary_log"), dict)
        and notary_log_path is not None
        and notary_log_path == expected_notary_log_path
        and notary_log_artifact_path is not None
        and accepted_notary_log(notary_log_path, document["submission"].get("id"))
        and isinstance(audit, dict)
        and audit.get("status") == "pass"
        and audit.get("readiness") == "distribution_ready"
        and audit.get("pass_ready") is True
    )
    current_matches = (
        status_ready
        and app_audit.get("ready") is True
        and clean_text(document.get("app_path")) == app_audit.get("app_path")
        and audit_path is not None
        and audit_path == expected_audit_path
        and clean_text(audit.get("status")) == app_audit.get("status")
        and clean_text(audit.get("readiness")) == app_audit.get("readiness")
        and audit.get("pass_ready") == app_audit.get("pass_ready")
    )
    ready = status_ready and current_matches
    if ready:
        failure_message = None
    elif not status_ready:
        failure_message = f"{machine}.notarization.json is not release-ready"
    elif audit_path is None:
        failure_message = f"{machine}.notarization.json does not record audit.path"
    else:
        failure_message = f"{machine}.notarization.json does not match {machine}.app-audit.json and {machine}.provenance.json"
    return {
        "ready": ready,
        "failure_message": failure_message,
    }

source_app_audit = machine_app_audit("source")
target_app_audit = machine_app_audit("target")
source_notarization = machine_notarization("source", source_app_audit)
target_notarization = machine_notarization("target", target_app_audit)

failures = []
if not source_app_audit["ready"]:
    failures.append("source.app-audit.json is not install-ready")
if not target_app_audit["ready"]:
    failures.append("target.app-audit.json is not install-ready")
if not source_notarization["ready"]:
    failures.append("source.notarization.json is not release-ready")
if not target_notarization["ready"]:
    failures.append("target.notarization.json is not release-ready")

failure_message = None
for candidate in (
    source_app_audit.get("failure_message"),
    target_app_audit.get("failure_message"),
    source_notarization.get("failure_message"),
    target_notarization.get("failure_message"),
):
    if isinstance(candidate, str) and candidate:
        failure_message = candidate
        break

json.dump({
    "source_app_audit_ready": source_app_audit["ready"],
    "target_app_audit_ready": target_app_audit["ready"],
    "source_notarization_ready": source_notarization["ready"],
    "target_notarization_ready": target_notarization["ready"],
    "ok": len(failures) == 0,
    "failures": failures,
    "failure_message": failure_message,
}, sys.stdout)
sys.stdout.write("\n")
PY
}

acceptance_two_machine_target_advertise() {
  sm_bin=$1
  app_dir=$2
  profile_path=$3
  bundle_root=$4
  listen=$5
  dest=$6
  duration=$7
  interval=$8
  audit_script=${SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT:-$(acceptance_default_app_audit_executable "$app_dir")}
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  target_machine_id=$(acceptance_two_machine_resolve_machine_id "${SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID:-}")
  target_machine_label=$(acceptance_two_machine_resolve_machine_label "${SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL:-}")
  acceptance_two_machine_record_collection "$bundle_root"
  acceptance_two_machine_record_machine_facts "$bundle_root" "target" "$target_machine_id" "$target_machine_label"
  acceptance_record_app_audit "$audit_script" "$app_dir" "$bundle_root" "target" "target-advertise"
  acceptance_require_ready_app_audit_for_collection "$bundle_root" "target" "$app_dir"
  acceptance_preflight_cli_surface "$sm_bin" "$app_dir" "$bundle_root" "target-advertise" "discover advertise --help" "-profile string" discover advertise --help
  acceptance_two_machine_record_role "$bundle_root" target_advertise "$profile_path" "$target_machine_id" "$target_machine_label"
  set -- discover advertise --profile "$profile_path" --format json
  if [ -n "$listen" ]; then
    set -- "$@" --listen "$listen"
  fi
  if [ -n "$dest" ]; then
    set -- "$@" --dest "$dest"
  fi
  if [ -n "$duration" ]; then
    set -- "$@" --duration "$duration"
  fi
  if [ -n "$interval" ]; then
    set -- "$@" --interval "$interval"
  fi
  "$sm_bin" "$@" > "$bundle_root/target.advertise.json"
  jq -e '.status == "advertised" and .trusted == false and (.listen | type == "string") and (.destination | type == "string") and (.capability_flags | type == "array")' \
    "$bundle_root/target.advertise.json" >/dev/null
  acceptance_update_bundle_meta "$bundle_root" \
    '.evidence.discovery = (.evidence.discovery // {})
    | .evidence.discovery.target_advertise = {
        output: "target.advertise.json",
        trusted: false
      }'
}

acceptance_two_machine_source_browse() {
  sm_bin=$1
  app_dir=$2
  bundle_root=$3
  listen=$4
  timeout=$5
  strict=$6
  audit_script=${SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT:-$(acceptance_default_app_audit_executable "$app_dir")}
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  source_machine_id=$(acceptance_two_machine_resolve_machine_id "${SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID:-}")
  source_machine_label=$(acceptance_two_machine_resolve_machine_label "${SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL:-}")
  acceptance_two_machine_record_collection "$bundle_root"
  acceptance_two_machine_record_machine_facts "$bundle_root" "source" "$source_machine_id" "$source_machine_label"
  acceptance_record_app_audit "$audit_script" "$app_dir" "$bundle_root" "source" "source-browse"
  acceptance_require_ready_app_audit_for_collection "$bundle_root" "source" "$app_dir"
  acceptance_preflight_cli_surface "$sm_bin" "$app_dir" "$bundle_root" "source-browse" "discover browse --help" "-timeout string" discover browse --help
  acceptance_two_machine_record_role "$bundle_root" source_browse "-" "$source_machine_id" "$source_machine_label"
  set -- discover browse --format json
  if [ -n "$listen" ]; then
    set -- "$@" --listen "$listen"
  fi
  if [ -n "$timeout" ]; then
    set -- "$@" --timeout "$timeout"
  fi
  if [ "$strict" = "1" ]; then
    set -- "$@" --strict
  fi
  "$sm_bin" "$@" > "$bundle_root/source.browse.json"
  jq -e '.trusted == false and (.candidate_count | type == "number") and (.candidates | type == "array")' \
    "$bundle_root/source.browse.json" >/dev/null
  acceptance_update_bundle_meta "$bundle_root" \
    '.evidence.discovery = (.evidence.discovery // {})
    | .evidence.discovery.source_browse = {
        output: "source.browse.json",
        trusted: false
      }'
}

acceptance_two_machine_record_target_ready() {
  bundle_root=$1
  profile_path=$2
  require_receiver=$3
  stderr_path=$4
  stdout_path=$5
  address=$(jq -r '.address // ""' "$bundle_root/target.ready.json")
  verification_code=$(jq -r '.verification_code // ""' "$bundle_root/target.ready.json")
  mode=$(jq -r '.mode // ""' "$bundle_root/target.ready.json")
  acceptance_require_flag "$address" "target serve address"
  if [ "$mode" = "pairing-only" ] || [ "$mode" = "pairing" ]; then
    acceptance_require_flag "$verification_code" "target verification code"
  fi
  if [ "$require_receiver" = "1" ]; then
    receiver_address=$(jq -r '.receiver_address // ""' "$bundle_root/target.ready.json")
    acceptance_require_flag "$receiver_address" "target receiver address"
  fi
  acceptance_write_json "$bundle_root/target.ready.merged.json" \
    --arg profile "$profile_path" \
    --arg address "$address" \
    --arg verification_code "$verification_code" \
    --arg mode "$mode" \
    --arg stderr_path "$stderr_path" \
    --arg stdout_path "$stdout_path" \
    --slurpfile ready "$bundle_root/target.ready.json" \
    '$ready[0] + {
      profile: $profile,
      address: $address,
      verification_code: $verification_code,
      mode: $mode,
      stderr_path: $stderr_path,
      stdout_path: $stdout_path
    }'
  mv "$bundle_root/target.ready.merged.json" "$bundle_root/target.ready.json"
  acceptance_update_bundle_meta "$bundle_root" \
    --arg address "$address" \
    --arg verification_code "$verification_code" \
    --arg mode "$mode" \
    '.evidence.target_ready = {
      address: $address,
      verification_code: $verification_code,
      mode: $mode
    }'
}

acceptance_two_machine_read_target_ready_field() {
  bundle_root=$1
  field=$2
  jq -r "$field" "$bundle_root/target.ready.json"
}

acceptance_two_machine_wait_for_target_ready() {
  bundle_root=$1
  profile_path=$2
  require_receiver=$3
  stderr_path=$4
  stdout_path=$5
  serve_pid=$6
  wait_timeout=15
  wait_i=0
  while [ "$wait_i" -lt "$wait_timeout" ]; do
    if acceptance_wait_for_serve_ready "$bundle_root/target.ready.json" 1 1 "$require_receiver"; then
      acceptance_two_machine_record_target_ready "$bundle_root" "$profile_path" "$require_receiver" "$stderr_path" "$stdout_path"
      return 0
    fi
    if ! kill -0 "$serve_pid" 2>/dev/null; then
      set +e
      wait "$serve_pid"
      serve_exit=$?
      set -e
      printf 'target serve exited before readiness was recorded: %s\n' "$stderr_path" >&2
      if [ -f "$stderr_path" ]; then
        cat "$stderr_path" >&2
      fi
      exit "$serve_exit"
    fi
    wait_i=$((wait_i + 1))
  done
  printf 'target serve readiness timeout: %s\n' "$bundle_root/target.ready.json" >&2
  exit 1
}

acceptance_next_target_phase() {
  bundle_root=$1
  next_file="$bundle_root/target.serve.next-phase"
  next=1
  if [ -f "$next_file" ]; then
    next=$(cat "$next_file")
  fi
  printf '%s\n' "$next"
  printf '%s\n' $((next + 1)) > "$next_file"
}

acceptance_two_machine_target_serve() {
  sm_bin=$1
  app_dir=$2
  profile_path=$3
  bundle_root=$4
  audit_script=${SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT:-$(acceptance_default_app_audit_executable "$app_dir")}
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  target_machine_id=$(acceptance_two_machine_resolve_machine_id "${SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID:-}")
  target_machine_label=$(acceptance_two_machine_resolve_machine_label "${SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL:-}")
  acceptance_two_machine_record_collection "$bundle_root"
  acceptance_two_machine_record_machine_facts "$bundle_root" "target" "$target_machine_id" "$target_machine_label"
  acceptance_record_app_audit "$audit_script" "$app_dir" "$bundle_root" "target" "target-serve"
  acceptance_require_ready_app_audit_for_collection "$bundle_root" "target" "$app_dir"
  acceptance_preflight_cli_surface "$sm_bin" "$app_dir" "$bundle_root" "target-serve" "serve --help" "-ready-file string" serve --help
  acceptance_two_machine_record_role "$bundle_root" target "$profile_path" "$target_machine_id" "$target_machine_label"
  "$sm_bin" profile lint --profile "$profile_path" > "$bundle_root/target.profile-lint.txt"
  phase_number=$(acceptance_next_target_phase "$bundle_root")
  phase_prefix="$bundle_root/target.serve.phase-$phase_number"
  require_receiver=0
  rm -f "$bundle_root/target.ready.json"
  : > "$phase_prefix.stdout"
  : > "$phase_prefix.stderr"
  "$sm_bin" serve --profile "$profile_path" --listen 0.0.0.0:0 --ready-file "$bundle_root/target.ready.json" \
    > "$phase_prefix.stdout" \
    2> "$phase_prefix.stderr" &
  serve_pid=$!
  printf '%s\n' "$serve_pid" > "$phase_prefix.pid"
  if [ "$phase_number" -gt 1 ]; then
    require_receiver=1
  fi
  acceptance_two_machine_wait_for_target_ready "$bundle_root" "$profile_path" "$require_receiver" "$phase_prefix.stderr" "$phase_prefix.stdout" "$serve_pid"
  cp "$bundle_root/target.ready.json" "$bundle_root/target.ready.phase-$phase_number.json"
  acceptance_update_bundle_meta "$bundle_root" \
    --arg phase "$phase_number" \
    --arg ready "target.ready.phase-$phase_number.json" \
    '.evidence.target_serve_phases = (.evidence.target_serve_phases // [])
    + [{phase: ($phase|tonumber), ready: $ready}]'
  printf 'target serve ready: %s\n' "$bundle_root/target.ready.json"
  wait "$serve_pid"
}

acceptance_two_machine_source_pair() {
  sm_bin=$1
  app_dir=$2
  profile_path=$3
  bundle_root=$4
  target_address=$5
  verification_code=$6
  audit_script=${SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT:-$(acceptance_default_app_audit_executable "$app_dir")}
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  source_machine_id=$(acceptance_two_machine_resolve_machine_id "${SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID:-}")
  source_machine_label=$(acceptance_two_machine_resolve_machine_label "${SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL:-}")
  acceptance_two_machine_record_collection "$bundle_root"
  acceptance_two_machine_record_machine_facts "$bundle_root" "source" "$source_machine_id" "$source_machine_label"
  acceptance_record_app_audit "$audit_script" "$app_dir" "$bundle_root" "source" "source-pair"
  acceptance_require_ready_app_audit_for_collection "$bundle_root" "source" "$app_dir"
  acceptance_preflight_cli_surface "$sm_bin" "$app_dir" "$bundle_root" "source-pair" "pair --help" "-receipt-out string" pair --help
  acceptance_two_machine_record_role "$bundle_root" source_pair "$profile_path" "$source_machine_id" "$source_machine_label"
  if [ -z "$target_address" ] && [ -f "$bundle_root/target.ready.json" ]; then
    target_address=$(acceptance_two_machine_read_target_ready_field "$bundle_root" '.address')
  fi
  if [ -z "$verification_code" ] && [ -f "$bundle_root/target.ready.json" ]; then
    verification_code=$(acceptance_two_machine_read_target_ready_field "$bundle_root" '.verification_code')
  fi
  acceptance_require_flag "$target_address" "--target-address or target.ready.json address"
  acceptance_require_flag "$verification_code" "--verification-code or target.ready.json verification_code"
  receipt_dir="$bundle_root/exported-receipts"
  mkdir -p "$receipt_dir"
  "$sm_bin" profile lint --profile "$profile_path" > "$bundle_root/source.profile-lint.txt"
  "$sm_bin" pair --profile "$profile_path" --target "$target_address" --verification-code "$verification_code" --receipt-out "$receipt_dir" > "$bundle_root/source.pair.txt"
  receipt_id=$(acceptance_profile_pairing_receipt_id "$profile_path")
  acceptance_require_flag "$receipt_id" "source pairing receipt id"
  acceptance_two_machine_require_control_plane_id "$receipt_id" "source pairing receipt id"
  receipt_rel="exported-receipts/$receipt_id.json"
  receipt_path="$bundle_root/$receipt_rel"
  acceptance_two_machine_require_regular_evidence_file "$receipt_path" "exported receipt after pair"
  acceptance_two_machine_validate_pairing_receipt "$receipt_path" "$receipt_id" "exported receipt evidence after pair"
  acceptance_write_json "$bundle_root/source.pair.json" \
    --arg profile "$profile_path" \
    --arg target_address "$target_address" \
    --arg verification_code "$verification_code" \
    --arg receipt_id "$receipt_id" \
    --arg receipt_path "$receipt_rel" \
    '{
      profile: $profile,
      target_address: $target_address,
      verification_code: $verification_code,
      pairing_receipt_id: $receipt_id,
      receipt_path: $receipt_path
    }'
  acceptance_update_bundle_meta "$bundle_root" \
    --arg receipt_id "$receipt_id" \
    --arg receipt_path "$receipt_rel" \
    --arg target_address "$target_address" \
    '.evidence.source_pair = {
      pairing_receipt_id: $receipt_id,
      receipt_path: $receipt_path,
      target_address: $target_address,
      output: "source.pair.json",
      pair: "source.pair.txt"
    }'
}

acceptance_two_machine_target_import() {
  sm_bin=$1
  app_dir=$2
  profile_path=$3
  bundle_root=$4
  audit_script=${SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT:-$(acceptance_default_app_audit_executable "$app_dir")}
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  target_machine_id=$(acceptance_two_machine_resolve_machine_id "${SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID:-}")
  target_machine_label=$(acceptance_two_machine_resolve_machine_label "${SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL:-}")
  acceptance_two_machine_record_collection "$bundle_root"
  acceptance_two_machine_record_machine_facts "$bundle_root" "target" "$target_machine_id" "$target_machine_label"
  acceptance_record_app_audit "$audit_script" "$app_dir" "$bundle_root" "target" "target-import"
  acceptance_require_ready_app_audit_for_collection "$bundle_root" "target" "$app_dir"
  acceptance_preflight_cli_surface "$sm_bin" "$app_dir" "$bundle_root" "target-import" "profile adopt-pairing --help" "-receipt-file string" profile adopt-pairing --help
  acceptance_two_machine_record_role "$bundle_root" target_import "$profile_path" "$target_machine_id" "$target_machine_label"
  acceptance_two_machine_require_json_control_plane_id "$bundle_root/source.pair.json" '.pairing_receipt_id' "pairing_receipt_id"
  receipt_id=$(acceptance_json_get "$bundle_root/source.pair.json" '.pairing_receipt_id')
  acceptance_require_flag "$receipt_id" "source pairing receipt id"
  receipt_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/source.pair.json" '.receipt_path' "source pairing receipt artifact" "")
  receipt_path=$(acceptance_resolve_bundle_relative_path "$bundle_root" "$receipt_rel")
  acceptance_two_machine_require_regular_evidence_file "$receipt_path" "exported receipt"
  acceptance_two_machine_validate_pairing_receipt "$receipt_path" "$receipt_id" "exported receipt evidence"
  "$sm_bin" profile adopt-pairing --profile "$profile_path" --receipt-file "$receipt_path" > "$bundle_root/target.adopt-pairing.txt"
  acceptance_update_bundle_meta "$bundle_root" \
    --arg receipt_id "$receipt_id" \
    '.evidence.target_import = {
      pairing_receipt_id: $receipt_id,
      adopted: "target.adopt-pairing.txt"
    }'
}

acceptance_two_machine_record_operator_evidence() {
  bundle_root=$1
  kind=$2
  status=$3
  detail=$4
  artifact=$5
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  acceptance_require_flag "$kind" "--kind"
  acceptance_require_flag "$status" "--status"
  acceptance_require_flag "$detail" "--detail"
  if ! printf '%s' "$detail" | grep -q '[^[:space:]]'; then
    printf 'operator evidence detail is required\n' >&2
    exit 2
  fi
  case "$status" in
    pass|blocked) ;;
    *)
      printf 'operator evidence status must be pass or blocked, got %s\n' "$status" >&2
      exit 2
      ;;
  esac
  machine_id=$(acceptance_two_machine_operator_evidence_expected_machine_id "$bundle_root" "$kind")
  machine_label=$(acceptance_two_machine_operator_evidence_expected_machine_label "$bundle_root" "$kind")
  if [ "$status" = "pass" ] && [ -z "$machine_id" ]; then
    expected_machine=$(acceptance_two_machine_operator_evidence_machine "$kind")
    if [ -n "$expected_machine" ]; then
      printf 'operator evidence %s requires canonical %s.machine.json before pass recording\n' "$kind" "$expected_machine" >&2
      exit 2
    fi
  fi
  acceptance_update_bundle_meta "$bundle_root" \
    --arg kind "$kind" \
    --arg status "$status" \
    --arg detail "$detail" \
    --arg artifact "$artifact" \
    --arg machine_id "$machine_id" \
    --arg machine_label "$machine_label" \
    '.evidence.operator = (.evidence.operator // {})
    | .evidence.operator[$kind] = {
        status: $status,
        detail: $detail
      }
    | if $artifact == "" then . else .evidence.operator[$kind].artifact = $artifact | . end
    | if $machine_id == "" then . else .evidence.operator[$kind].machine_id = $machine_id | . end
    | if $machine_label == "" then . else .evidence.operator[$kind].machine_label = $machine_label | . end'
}

acceptance_two_machine_source_transfer() {
  sm_bin=$1
  app_dir=$2
  profile_path=$3
  bundle_root=$4
  session_id=$5
  audit_script=${SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT:-$(acceptance_default_app_audit_executable "$app_dir")}
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  source_machine_id=$(acceptance_two_machine_resolve_machine_id "${SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_ID:-}")
  source_machine_label=$(acceptance_two_machine_resolve_machine_label "${SUPERMOVER_ACCEPTANCE_SOURCE_MACHINE_LABEL:-}")
  acceptance_two_machine_record_collection "$bundle_root"
  acceptance_two_machine_record_machine_facts "$bundle_root" "source" "$source_machine_id" "$source_machine_label"
  acceptance_record_app_audit "$audit_script" "$app_dir" "$bundle_root" "source" "source-transfer"
  acceptance_require_ready_app_audit_for_collection "$bundle_root" "source" "$app_dir"
  acceptance_preflight_cli_surface "$sm_bin" "$app_dir" "$bundle_root" "source-transfer-set-network" "profile set-network --help" "-receiver-url string" profile set-network --help
  acceptance_preflight_cli_surface "$sm_bin" "$app_dir" "$bundle_root" "source-transfer-push-network" "push --network --help" "supermover push --network --profile <path>" push --network --help
  acceptance_preflight_cli_surface "$sm_bin" "$app_dir" "$bundle_root" "source-transfer-verify-source-consistency" "verify source-consistency --help" "-baseline string" verify source-consistency --help
  acceptance_two_machine_record_role "$bundle_root" source_transfer "$profile_path" "$source_machine_id" "$source_machine_label"
  acceptance_require_flag "$session_id" "--session"
  if [ ! -f "$bundle_root/source.pair.json" ]; then
    printf 'missing source pairing evidence: %s\n' "$bundle_root/source.pair.json" >&2
    exit 1
  fi
  if [ ! -f "$bundle_root/target.ready.json" ]; then
    printf 'missing target ready evidence: %s\n' "$bundle_root/target.ready.json" >&2
    exit 1
  fi
  target_mode=$(acceptance_two_machine_read_target_ready_field "$bundle_root" '.mode')
  if [ "$target_mode" != "pairing" ]; then
    printf 'expected paired target serve mode=pairing before source transfer, got %s\n' "$target_mode" >&2
    exit 1
  fi
  receiver_address=$(acceptance_two_machine_read_target_ready_field "$bundle_root" '.receiver_address')
  acceptance_require_flag "$receiver_address" "paired target receiver address"
  "$sm_bin" profile set-network --profile "$profile_path" --receiver-url "https://$receiver_address" > "$bundle_root/source.set-network.txt"
  "$sm_bin" push --network --profile "$profile_path" --dry-run --source-baseline "$bundle_root/source.baseline.json" > "$bundle_root/source.network-dry-run.txt"
  "$sm_bin" push --network --profile "$profile_path" --session "$session_id" --source-baseline "$bundle_root/source.baseline.json" > "$bundle_root/source.network-push.txt"
  "$sm_bin" verify --profile "$profile_path" --session "$session_id" --format json > "$bundle_root/source.verify.json"
  "$sm_bin" status --profile "$profile_path" --format json > "$bundle_root/source.status.json"
  "$sm_bin" report --profile "$profile_path" --session "$session_id" --format json > "$bundle_root/source.report.json"
  "$sm_bin" health --profile "$profile_path" --format json > "$bundle_root/source.health.json"
  "$sm_bin" verify source-consistency --profile "$profile_path" --baseline "$bundle_root/source.baseline.json" --format json > "$bundle_root/source.consistency.json"
  consistency_status=$(acceptance_json_get "$bundle_root/source.consistency.json" '.status')
  consistency_mode=$(acceptance_json_get "$bundle_root/source.consistency.json" '.mode')
  acceptance_require_flag "$consistency_status" "source consistency status"
  acceptance_require_flag "$consistency_mode" "source consistency mode"
  acceptance_write_json "$bundle_root/source.transfer.json" \
    --arg profile "$profile_path" \
    --arg session_id "$session_id" \
    --arg target_address "$(acceptance_two_machine_read_target_ready_field "$bundle_root" '.address')" \
    --arg receiver_address "$receiver_address" \
    --arg target_mode "$target_mode" \
    '{
      profile: $profile,
      session_id: $session_id,
      target_address: $target_address,
      receiver_address: $receiver_address,
      target_mode: $target_mode
    }'
  acceptance_update_bundle_meta "$bundle_root" \
    --arg session_id "$session_id" \
    --arg receiver_address "$receiver_address" \
    --arg consistency_status "$consistency_status" \
    --arg consistency_mode "$consistency_mode" \
    '.evidence.source_transfer = {
      session_id: $session_id,
      receiver_address: $receiver_address,
      output: "source.transfer.json",
      verify: "source.verify.json",
      status: "source.status.json",
      report: "source.report.json",
      health: "source.health.json",
      push: "source.network-push.txt"
    }
    | .evidence.source_consistency = {
        output: "source.consistency.json",
        baseline: "source.baseline.json",
        status: $consistency_status,
        mode: $consistency_mode
      }'
}

acceptance_two_machine_evaluate() {
  bundle_root=$1
  target_root=$2
  source_profile=$3
  require_operator_evidence=${4:-0}
  acceptance_two_machine_require_jq
  acceptance_ensure_bundle_root "$bundle_root"
  source_pair_json_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.source_pair.output' "source_pair output" "source.pair.json")
  source_transfer_json_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.source_transfer.output' "source_transfer output" "source.transfer.json")
  source_pair_text_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.source_pair.pair' "source_pair transcript" "source.pair.txt")
  source_push_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.source_transfer.push' "source_transfer push" "source.network-push.txt")
  source_verify_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.source_transfer.verify' "source_transfer verify" "source.verify.json")
  source_status_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.source_transfer.status' "source_transfer status" "source.status.json")
  source_report_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.source_transfer.report' "source_transfer report" "source.report.json")
  source_health_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.source_transfer.health' "source_transfer health" "source.health.json")
  source_consistency_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.source_consistency.output' "source_consistency output" "source.consistency.json")
  source_browse_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.discovery.source_browse.output' "source_browse output" "")
  target_advertise_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.discovery.target_advertise.output' "target_advertise output" "")
  source_provenance=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "source.provenance.json" "source provenance")
  target_provenance=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "target.provenance.json" "target provenance")
  source_app_audit=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "source.app-audit.json" "source app audit")
  target_app_audit=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "target.app-audit.json" "target app audit")
  target_ready=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "target.ready.json" "target ready artifact")
  source_pair_json=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_pair_json_rel" "source pair artifact")
  source_transfer_json=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_transfer_json_rel" "source transfer artifact")
  source_pair_text=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_pair_text_rel" "source pair transcript")
  source_push=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_push_rel" "source network push transcript")
  source_verify=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_verify_rel" "source verify artifact")
  source_status=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_status_rel" "source status artifact")
  source_report=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_report_rel" "source report artifact")
  source_health=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_health_rel" "source health artifact")
  source_consistency=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_consistency_rel" "source consistency artifact")
  if [ "$require_operator_evidence" = "1" ]; then
    source_notarization=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "source.notarization.json" "source notarization")
    target_notarization=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "target.notarization.json" "target notarization")
    source_notarization_audit_path=$(acceptance_two_machine_json_string_or_default "$source_notarization" '.audit.path' "source notarization audit path" "")
    target_notarization_audit_path=$(acceptance_two_machine_json_string_or_default "$target_notarization" '.audit.path' "target notarization audit path" "")
    source_app_path=$(acceptance_two_machine_json_string_or_default "$source_app_audit" '.app_path' "source app audit app_path" "")
    target_app_path=$(acceptance_two_machine_json_string_or_default "$target_app_audit" '.app_path' "target app audit app_path" "")
    if [ "$source_notarization_audit_path" != "$source_app_path.notary/post-staple.audit.json" ]; then
      printf 'source.notarization.json does not reference the current source post-staple audit\n' >&2
      exit 1
    fi
    if [ "$target_notarization_audit_path" != "$target_app_path.notary/post-staple.audit.json" ]; then
      printf 'target.notarization.json does not reference the current target post-staple audit\n' >&2
      exit 1
    fi
    source_notary_auth_mode=$(acceptance_two_machine_json_string_or_default "$source_notarization" '.auth_mode' "source notarization auth_mode" "")
    target_notary_auth_mode=$(acceptance_two_machine_json_string_or_default "$target_notarization" '.auth_mode' "target notarization auth_mode" "")
    case "$source_notary_auth_mode" in
      keychain_profile|api_key|apple_id) ;;
      *)
        printf 'source.notarization.json does not record a supported auth_mode\n' >&2
        exit 1
        ;;
    esac
    case "$target_notary_auth_mode" in
      keychain_profile|api_key|apple_id) ;;
      *)
        printf 'target.notarization.json does not record a supported auth_mode\n' >&2
        exit 1
        ;;
    esac
    if ! jq -e '(.submission.id | type == "string") and ((.submission.id | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))' "$source_notarization" >/dev/null 2>&1; then
      printf 'source.notarization.json does not record a notary submission UUID\n' >&2
      exit 1
    fi
    if ! jq -e '(.submission.id | type == "string") and ((.submission.id | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))' "$target_notarization" >/dev/null 2>&1; then
      printf 'target.notarization.json does not record a notary submission UUID\n' >&2
      exit 1
    fi
    if ! jq -e '.failure == null' "$source_notarization" >/dev/null 2>&1; then
      printf 'source.notarization.json records a notarization failure\n' >&2
      exit 1
    fi
    if ! jq -e '.failure == null' "$target_notarization" >/dev/null 2>&1; then
      printf 'target.notarization.json records a notarization failure\n' >&2
      exit 1
    fi
    source_notary_log_rel=$(acceptance_two_machine_json_string_or_default "$source_notarization" '.notary_log.path' "source notary log" "")
    target_notary_log_rel=$(acceptance_two_machine_json_string_or_default "$target_notarization" '.notary_log.path' "target notary log" "")
    if [ "$source_notary_log_rel" != "source.notary-log.json" ]; then
      printf 'source.notarization.json does not reference source.notary-log.json\n' >&2
      exit 1
    fi
    if [ "$target_notary_log_rel" != "target.notary-log.json" ]; then
      printf 'target.notarization.json does not reference target.notary-log.json\n' >&2
      exit 1
    fi
    source_notary_log=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_notary_log_rel" "source notary log")
    target_notary_log=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$target_notary_log_rel" "target notary log")
    source_notary_submission_id=$(acceptance_two_machine_json_string_or_default "$source_notarization" '.submission.id' "source notary submission id" "")
    target_notary_submission_id=$(acceptance_two_machine_json_string_or_default "$target_notarization" '.submission.id' "target notary submission id" "")
    if ! jq -e --arg submission_id "$source_notary_submission_id" '
      .status == "Accepted"
      and ((.issues == null) or (.issues | type == "array"))
      and (.jobId | type == "string")
      and (($submission_id | gsub("^\\s+|\\s+$"; "") | ascii_downcase) == (.jobId | gsub("^\\s+|\\s+$"; "") | ascii_downcase))
      and (($submission_id | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))
      and ((.jobId | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))
    ' "$source_notary_log" >/dev/null 2>&1; then
      printf 'source.notary-log.json is not accepted notarization log evidence\n' >&2
      exit 1
    fi
    if ! jq -e --arg submission_id "$target_notary_submission_id" '
      .status == "Accepted"
      and ((.issues == null) or (.issues | type == "array"))
      and (.jobId | type == "string")
      and (($submission_id | gsub("^\\s+|\\s+$"; "") | ascii_downcase) == (.jobId | gsub("^\\s+|\\s+$"; "") | ascii_downcase))
      and (($submission_id | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))
      and ((.jobId | gsub("^\\s+|\\s+$"; "")) | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))
    ' "$target_notary_log" >/dev/null 2>&1; then
      printf 'target.notary-log.json is not accepted notarization log evidence\n' >&2
      exit 1
    fi
    source_machine_facts=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "source.machine.json" "source machine facts")
    target_machine_facts=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "target.machine.json" "target machine facts")
  fi
  if [ -n "$source_browse_rel" ]; then
    source_browse=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$source_browse_rel" "source browse artifact")
  fi
  if [ -n "$target_advertise_rel" ]; then
    target_advertise=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$target_advertise_rel" "target advertise artifact")
  fi
  baseline_rel=$(acceptance_two_machine_json_string_or_default "$source_consistency" '.baseline' "source_consistency baseline" "")
  if [ -z "$baseline_rel" ]; then
    baseline_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.source_consistency.baseline' "source_consistency baseline" "source.baseline.json")
  fi
  baseline_path=$(acceptance_two_machine_require_bundle_regular_artifact "$bundle_root" "$baseline_rel" "source consistency baseline")
  jq -e '
    .schema == "supermover.macos.provenance.v1"
    and (.git_commit | type == "string") and (.git_commit | length > 0)
    and (.cli_version | type == "string") and (.cli_version | length > 0)
    and .cli_relative_path == "Contents/Resources/bin/supermover"
    and (.build_profile | type == "string") and (.build_profile | length > 0)
    and (.signing | type == "string") and (.signing | length > 0)
  ' "$source_provenance" >/dev/null
  jq -e '
    .schema == "supermover.macos.provenance.v1"
    and (.git_commit | type == "string") and (.git_commit | length > 0)
    and (.cli_version | type == "string") and (.cli_version | length > 0)
    and .cli_relative_path == "Contents/Resources/bin/supermover"
    and (.build_profile | type == "string") and (.build_profile | length > 0)
    and (.signing | type == "string") and (.signing | length > 0)
  ' "$target_provenance" >/dev/null
  jq -e '
    .schema == "supermover.macos.app_audit.v1"
    and (.status | type == "string") and (.status | length > 0)
    and (.readiness | type == "string") and (.readiness | length > 0)
  ' "$source_app_audit" >/dev/null
  jq -e '
    .schema == "supermover.macos.app_audit.v1"
    and (.status | type == "string") and (.status | length > 0)
    and (.readiness | type == "string") and (.readiness | length > 0)
  ' "$target_app_audit" >/dev/null
  if ! jq -e --slurpfile meta "$bundle_root/meta.json" '
    def clean: if type == "string" then . else "" end;
    def ready: $meta[0].evidence.target_ready;
    (ready | type == "object")
    and (.address | type == "string") and (.address | length > 0)
    and (.mode | type == "string") and (.mode | length > 0)
    and (.trusted | type == "boolean")
    and (.transfer | type == "boolean")
    and ((.receiver_address == null) or (.receiver_address | type == "string"))
    and ((.receiver_routes == null) or (.receiver_routes | type == "boolean"))
    and ((.push_network == null) or (.push_network | type == "boolean"))
    and ((.expires_at == null) or (.expires_at | type == "string"))
    and .address == ready.address
    and .mode == ready.mode
    and (
      if (.mode == "pairing" or .mode == "pairing-only") then
        (.verification_code | type == "string")
        and (.verification_code | length > 0)
        and .verification_code == ready.verification_code
      else
        ((.verification_code == null) or (.verification_code | type == "string"))
        and (((.verification_code // "") | length == 0) or .verification_code == ready.verification_code)
      end
    )
  ' "$target_ready" >/dev/null; then
    printf 'invalid target ready artifact: target.ready.json\n' >&2
    exit 1
  fi
  acceptance_two_machine_require_json_control_plane_id "$source_pair_json" '.pairing_receipt_id' "pairing_receipt_id"
  pairing_receipt_id=$(acceptance_json_get "$source_pair_json" '.pairing_receipt_id')
  acceptance_require_flag "$pairing_receipt_id" "--source-profile pairing_receipt_id"
  source_pair_receipt_rel=$(acceptance_two_machine_json_string_or_default "$source_pair_json" '.receipt_path' "source pairing receipt artifact" "")
  acceptance_require_flag "$source_pair_receipt_rel" "source pairing receipt artifact"
  source_pair_receipt=$(acceptance_resolve_bundle_relative_path "$bundle_root" "$source_pair_receipt_rel")
  acceptance_two_machine_require_regular_evidence_file "$source_pair_receipt" "exported receipt"
  acceptance_two_machine_validate_pairing_receipt "$source_pair_receipt" "$pairing_receipt_id" "exported receipt evidence"
  target_import_receipt_id=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.target_import.pairing_receipt_id' "target_import pairing_receipt_id" "")
  acceptance_require_flag "$target_import_receipt_id" "target_import pairing_receipt_id"
  target_import_adopted_rel=$(acceptance_two_machine_json_string_or_default "$bundle_root/meta.json" '.evidence.target_import.adopted' "target_import adopted artifact" "")
  acceptance_require_flag "$target_import_adopted_rel" "target_import adopted artifact"
  target_import_adopted=$(acceptance_resolve_bundle_relative_path "$bundle_root" "$target_import_adopted_rel")
  if ! acceptance_two_machine_regular_evidence_file "$target_import_adopted"; then
    printf 'missing target_import adopted artifact: %s\n' "$target_import_adopted_rel" >&2
    exit 1
  fi
  if ! jq -e --arg pairing_receipt_id "$pairing_receipt_id" '
    .evidence.target_import.pairing_receipt_id == $pairing_receipt_id
  ' "$bundle_root/meta.json" >/dev/null; then
    printf 'target_import pairing_receipt_id mismatch: expected %s got %s\n' "$pairing_receipt_id" "$target_import_receipt_id" >&2
    exit 1
  fi
  if ! jq -e --slurpfile ready "$target_ready" '
    (.target_address | type == "string")
    and (.target_address | length > 0)
    and .target_address == $ready[0].address
  ' "$source_pair_json" >/dev/null; then
    printf 'source pair target address does not match target.ready.json: %s\n' "$source_pair_json_rel" >&2
    exit 1
  fi
  acceptance_two_machine_require_json_control_plane_id "$source_transfer_json" '.session_id' "session_id"
  session_id=$(acceptance_json_get "$source_transfer_json" '.session_id')
  acceptance_require_flag "$session_id" "source session id"
  if ! jq -e '
    (.receiver_address | type == "string")
    and (.receiver_address | length > 0)
    and .receiver_routes == true
    and .push_network == true
    and .transfer == true
  ' "$target_ready" >/dev/null; then
    printf 'target ready artifact does not prove receiver transfer readiness: target.ready.json\n' >&2
    exit 1
  fi
  if ! jq -e --slurpfile ready "$target_ready" '
    (.target_address | type == "string")
    and (.target_address | length > 0)
    and .target_address == $ready[0].address
    and (.target_mode | type == "string")
    and (.target_mode | length > 0)
    and .target_mode == $ready[0].mode
    and (.receiver_address | type == "string")
    and .receiver_address == $ready[0].receiver_address
  ' "$source_transfer_json" >/dev/null; then
    ready_receiver=$(jq -r '.receiver_address // ""' "$target_ready")
    transfer_receiver=$(jq -r '.receiver_address // ""' "$source_transfer_json")
    if [ -n "$ready_receiver" ] && [ "$transfer_receiver" != "$ready_receiver" ]; then
      printf 'source transfer receiver does not match target.ready.json: %s\n' "$source_transfer_json_rel" >&2
    else
      printf 'source transfer target evidence does not match target.ready.json: %s\n' "$source_transfer_json_rel" >&2
    fi
    exit 1
  fi
  acceptance_two_machine_require_target_control_artifact "$target_root" ".supermover/pairings/$pairing_receipt_id.json" "target pairing receipt"
  acceptance_two_machine_require_target_control_artifact "$target_root" ".supermover/sessions/$session_id/network-transfer.json" "target network-transfer evidence"
  acceptance_two_machine_validate_pairing_receipt \
    "$target_root/.supermover/pairings/$pairing_receipt_id.json" \
    "$pairing_receipt_id" \
    "target pairing receipt evidence"
  acceptance_two_machine_validate_network_transfer \
    "$target_root/.supermover/sessions/$session_id/network-transfer.json" \
    "$session_id" \
    "target network-transfer evidence"
  if ! jq -e '
    (.summary | type == "object")
    and (.summary.files_verified | type == "number")
    and (.summary.files_verified == (.summary.files_verified | floor))
    and (.summary.files_verified >= 1)
    and (.summary.error_findings | type == "number")
    and (.summary.error_findings == (.summary.error_findings | floor))
    and (.summary.error_findings == 0)
    and (.summary.artifact_problems | type == "number")
    and (.summary.artifact_problems == (.summary.artifact_problems | floor))
    and (.summary.artifact_problems == 0)
  ' "$source_verify" >/dev/null; then
    printf 'invalid source verify summary evidence: %s\n' "$source_verify_rel" >&2
    exit 1
  fi
  if ! acceptance_two_machine_json_target_root_matches "$source_verify" "$target_root"; then
    printf 'source verify target_root does not match selected target root: %s\n' "$source_verify_rel" >&2
    exit 1
  fi
  if ! jq -e --arg pairing_receipt_id "$pairing_receipt_id" '
    .pairing.receipt_id == $pairing_receipt_id
    and .pairing.status == "paired_receipt_valid"
  ' \
    "$source_report" >/dev/null; then
    printf 'source report pairing receipt_id mismatch: expected %s\n' "$pairing_receipt_id" >&2
    exit 1
  fi
  if ! acceptance_two_machine_json_target_root_matches "$source_report" "$target_root"; then
    printf 'source report target_root does not match selected target root: %s\n' "$source_report_rel" >&2
    exit 1
  fi
  if ! acceptance_two_machine_validate_source_transfer_artifact "status" "$source_status" "$session_id"; then
    printf 'invalid source status artifact: %s\n' "$source_status_rel" >&2
    exit 1
  fi
  if ! acceptance_two_machine_json_target_root_matches "$source_status" "$target_root"; then
    printf 'source status target_root does not match selected target root: %s\n' "$source_status_rel" >&2
    exit 1
  fi
  if ! acceptance_two_machine_validate_source_transfer_artifact "health" "$source_health" "$session_id"; then
    printf 'invalid source health artifact: %s\n' "$source_health_rel" >&2
    exit 1
  fi
  if ! acceptance_two_machine_json_target_root_matches "$source_health" "$target_root"; then
    printf 'source health target_root does not match selected target root: %s\n' "$source_health_rel" >&2
    exit 1
  fi
  jq -e --arg session_id "$session_id" '
    .schema == "supermover.acceptance.current_source_consistency.v1"
    and .status == "pass"
    and .mode == "current_source_verified"
    and .session_id == $session_id
  ' "$source_consistency" >/dev/null
  if [ -n "${source_browse:-}" ]; then
    if ! jq -e '
      def json_string: type == "string";
      def json_int: type == "number" and . == floor;
      def string_list: type == "array" and all(.[]; type == "string");
      def valid_advertisement:
        (.service_type | json_string)
        and (.protocol_version | json_string)
        and (.ephemeral_nonce | json_string)
        and (.capability_flags | string_list);
      def valid_hint:
        (.address | json_string)
        and (.advertisement | valid_advertisement)
        and (.seen_at | json_string)
        and (.expires_at | json_string)
        and (.trusted | type == "boolean");
      def valid_candidate:
        (.hint | valid_hint)
        and (.class | json_string)
        and (.duplicate_count | json_int)
        and ((.ambiguity_reasons == null) or (.ambiguity_reasons | string_list));
      (.source | json_string)
      and (.listen | json_string)
      and (.candidate_count | json_int)
      and (.invalid_packets | json_int)
      and .trusted == false
      and (.candidates | type == "array")
      and all(.candidates[]; valid_candidate)
    ' "$source_browse" >/dev/null; then
      printf 'invalid source browse artifact: %s\n' "$source_browse_rel" >&2
      exit 1
    fi
  fi
  if [ -n "${target_advertise:-}" ]; then
    if ! jq -e '
      def json_string: type == "string";
      def string_list: type == "array" and all(.[]; type == "string");
      .status == "advertised"
      and (.listen | json_string)
      and (.destination | json_string)
      and (.service_type | json_string)
      and (.protocol_version | json_string)
      and (.ephemeral_nonce | json_string)
      and (.capability_flags | string_list)
      and .trusted == false
      and (.duration | json_string)
      and (.interval | json_string)
    ' "$target_advertise" >/dev/null; then
      printf 'invalid target advertise artifact: %s\n' "$target_advertise_rel" >&2
      exit 1
    fi
  fi
  if [ "$require_operator_evidence" = "1" ]; then
    release_evidence=$(acceptance_two_machine_release_evidence_summary "$bundle_root")
    if ! printf '%s\n' "$release_evidence" | jq -e '.ok == true' >/dev/null; then
      failure_message=$(printf '%s\n' "$release_evidence" | jq -r '.failure_message // "invalid installed-app release evidence"')
      printf '%s\n' "$failure_message" >&2
      exit 1
    fi
    installed_app_proof=$(acceptance_two_machine_installed_app_proof_summary "$bundle_root")
    if ! printf '%s\n' "$installed_app_proof" | jq -e '.ok == true' >/dev/null; then
      failure_message=$(
        printf '%s\n' "$installed_app_proof" | jq -r '
          .final_evaluation_collection_detail
          // .final_evaluation_machine_facts_detail
          // .final_evaluation_bundle_handoff_detail
          // .failure_message
          // "invalid two-machine collection evidence"
        '
      )
      printf '%s\n' "$failure_message" >&2
      exit 1
    fi
    source_operator_machine_id=$(jq -r 'if .schema == "supermover.acceptance.machine_facts.v1" and (.machine_id | type == "string") then .machine_id else "" end' "$source_machine_facts")
    target_operator_machine_id=$(jq -r 'if .schema == "supermover.acceptance.machine_facts.v1" and (.machine_id | type == "string") then .machine_id else "" end' "$target_machine_facts")
    if ! jq -e --arg source_machine_id "$source_operator_machine_id" --arg target_machine_id "$target_operator_machine_id" '
      def valid_operator_evidence($kind):
        (.evidence.operator[$kind] | type == "object")
        and (.evidence.operator[$kind].status == "pass")
        and (.evidence.operator[$kind].detail | type == "string")
        and ((.evidence.operator[$kind].detail | gsub("^\\s+|\\s+$"; "")) | length > 0)
        and (.evidence.operator[$kind].machine_id | type == "string");
      ($source_machine_id | length) > 0
      and ($target_machine_id | length) > 0
      and valid_operator_evidence("local_network")
      and .evidence.operator.local_network.machine_id == $target_machine_id
      and valid_operator_evidence("firewall")
      and .evidence.operator.firewall.machine_id == $target_machine_id
      and valid_operator_evidence("pairing_confirmation")
      and .evidence.operator.pairing_confirmation.machine_id == $source_machine_id
    ' "$bundle_root/meta.json" >/dev/null; then
      printf 'operator evidence must be pass with non-empty detail and machine_id bound to source/target machine facts\n' >&2
      exit 1
    fi
  fi
  acceptance_write_json "$bundle_root/evaluation.json" \
    --arg pairing_receipt_id "$pairing_receipt_id" \
    --arg session_id "$session_id" \
    --arg target_root "$target_root" \
    --arg require_operator_evidence "$require_operator_evidence" \
    '{
      schema: "supermover.acceptance.two_machine.v1",
      status: "evidence_collected",
      pairing_receipt_id: $pairing_receipt_id,
      session_id: $session_id,
      target_root: $target_root,
      require_operator_evidence: ($require_operator_evidence == "1")
    }'
  acceptance_update_bundle_meta "$bundle_root" \
    --arg pairing_receipt_id "$pairing_receipt_id" \
    --arg session_id "$session_id" \
    --arg target_root "$target_root" \
    --arg require_operator_evidence "$require_operator_evidence" \
    '.status = "evidence_collected"
    | .evidence.evaluation = {
        pairing_receipt_id: $pairing_receipt_id,
        session_id: $session_id,
        target_root: $target_root,
        output: "evaluation.json",
        require_operator_evidence: ($require_operator_evidence == "1")
      }'
}
