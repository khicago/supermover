run_network_loopback_acceptance() {
  NETWORK_RECEIPT_DIR="$WORK_DIR/network-exported-receipts"
  SOURCE_CERT="$NETWORK_TLS_DIR/source.crt"
  SOURCE_KEY="$NETWORK_TLS_DIR/source.key"
  TARGET_CERT="$NETWORK_TLS_DIR/target.crt"
  TARGET_KEY="$NETWORK_TLS_DIR/target.key"
  acceptance_write_tls_identity source "$SOURCE_CERT" "$SOURCE_KEY"
  acceptance_write_tls_identity target "$TARGET_CERT" "$TARGET_KEY"

  "$SM" profile init --profile "$NETWORK_SOURCE_PROFILE" --source "$NETWORK_SOURCE_DIR" --target "$NETWORK_TARGET_DIR" --id loopback-network --name "Loopback Network Source" > "$WORK_DIR/network-source-profile-init.txt"
  "$SM" profile init --profile "$NETWORK_TARGET_PROFILE" --source "$NETWORK_SOURCE_DIR" --target "$NETWORK_TARGET_DIR" --id loopback-network --name "Loopback Network Target" > "$WORK_DIR/network-target-profile-init.txt"

  python3 - "$NETWORK_SOURCE_PROFILE" "$SOURCE_CERT" "$SOURCE_KEY" <<'PY'
import json, sys
path, cert_path, key_path = sys.argv[1:]
with open(path) as f:
    doc = json.load(f)
doc["network"] = {
    "receiver_url": "https://127.0.0.1:9443",
    "local_tls_identity": {
        "certificate_path": cert_path,
        "private_key_path": key_path,
    },
}
with open(path, "w") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
PY
  python3 - "$NETWORK_TARGET_PROFILE" "$TARGET_CERT" "$TARGET_KEY" <<'PY'
import json, sys
path, cert_path, key_path = sys.argv[1:]
with open(path) as f:
    doc = json.load(f)
doc["network"] = {
    "receiver_url": "https://127.0.0.1:9443",
    "local_tls_identity": {
        "certificate_path": cert_path,
        "private_key_path": key_path,
    },
}
with open(path, "w") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
PY

  "$SM" profile lint --profile "$NETWORK_SOURCE_PROFILE" > "$WORK_DIR/network-source-profile-lint.txt"
  "$SM" profile lint --profile "$NETWORK_TARGET_PROFILE" > "$WORK_DIR/network-target-profile-lint.txt"

  TARGET_RECEIVER_PORT=$(acceptance_reserve_local_port)
  python3 - "$NETWORK_SOURCE_PROFILE" "$TARGET_RECEIVER_PORT" <<'PY'
import json, sys
path, port = sys.argv[1:]
with open(path) as f:
    doc = json.load(f)
doc["network"]["receiver_url"] = f"https://127.0.0.1:{port}"
with open(path, "w") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
PY
  python3 - "$NETWORK_TARGET_PROFILE" "$TARGET_RECEIVER_PORT" <<'PY'
import json, sys
path, port = sys.argv[1:]
with open(path) as f:
    doc = json.load(f)
doc["network"]["receiver_url"] = f"https://127.0.0.1:{port}"
with open(path, "w") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
PY

  start_target_serve "$NETWORK_TARGET_PROFILE" "network-pairing-serve" "$WORK_DIR/network-pairing-ready.json" 0
  PAIRING_ADDRESS=$(acceptance_json_get "$WORK_DIR/network-pairing-ready.json" '.address')
  PAIRING_CODE=$(acceptance_json_get "$WORK_DIR/network-pairing-ready.json" '.verification_code')
  PAIRING_MODE=$(acceptance_json_get "$WORK_DIR/network-pairing-ready.json" '.mode')
  if [ "$PAIRING_MODE" != "pairing-only" ]; then
    printf 'expected initial target serve mode=pairing-only, got %s\n' "$PAIRING_MODE" >&2
    exit 1
  fi

  mkdir -p "$NETWORK_RECEIPT_DIR"
  "$SM" pair --profile "$NETWORK_SOURCE_PROFILE" --target "$PAIRING_ADDRESS" --verification-code "$PAIRING_CODE" --receipt-out "$NETWORK_RECEIPT_DIR" > "$WORK_DIR/network-pair.txt"
  PAIRING_RECEIPT_ID=$(python3 - "$NETWORK_SOURCE_PROFILE" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    doc = json.load(f)
print(doc["target"]["pairing_receipt_id"])
PY
)
  NETWORK_RECEIPT_PATH="$NETWORK_RECEIPT_DIR/$PAIRING_RECEIPT_ID.json"
  test -f "$NETWORK_RECEIPT_PATH"
  "$SM" profile adopt-pairing --profile "$NETWORK_TARGET_PROFILE" --receipt-file "$NETWORK_RECEIPT_PATH" > "$WORK_DIR/network-target-adopt-pairing.txt"

  acceptance_cleanup_pid "$(acceptance_json_get "$WORK_DIR/network-pairing-ready.json" '.pid')"
  start_target_serve "$NETWORK_TARGET_PROFILE" "network-receiver-serve" "$WORK_DIR/network-receiver-ready.json" 1
  RECEIVER_MODE=$(acceptance_json_get "$WORK_DIR/network-receiver-ready.json" '.mode')
  if [ "$RECEIVER_MODE" != "pairing" ]; then
    printf 'expected paired target serve mode=pairing, got %s\n' "$RECEIVER_MODE" >&2
    exit 1
  fi
  if ! jq -e '.receiver_routes == true and .push_network == true and .trusted == true and .transfer == true' "$WORK_DIR/network-receiver-ready.json" >/dev/null; then
    printf 'expected receiver-enabled serve ready evidence to report receiver routes and transfer readiness\n' >&2
    cat "$WORK_DIR/network-receiver-ready.json" >&2
    exit 1
  fi

  "$SM" push --network --profile "$NETWORK_SOURCE_PROFILE" --dry-run > "$WORK_DIR/network-push-dry-run.txt"
  "$SM" push --network --profile "$NETWORK_SOURCE_PROFILE" --session "$NETWORK_SESSION_ID" > "$WORK_DIR/network-push.txt"
  acceptance_cleanup_pid "$(acceptance_json_get "$WORK_DIR/network-receiver-ready.json" '.pid')"

  "$SM" verify --profile "$NETWORK_SOURCE_PROFILE" --session "$NETWORK_SESSION_ID" --format json > "$WORK_DIR/network-verify.json"
  "$SM" status --profile "$NETWORK_SOURCE_PROFILE" --format json > "$WORK_DIR/network-status.json"
  "$SM" report --profile "$NETWORK_SOURCE_PROFILE" --session "$NETWORK_SESSION_ID" --format json > "$WORK_DIR/network-report.json"
  "$SM" health --profile "$NETWORK_SOURCE_PROFILE" --format json > "$WORK_DIR/network-health.json"

  jq -e '.summary.files_verified == 3 and .summary.error_findings == 0 and .summary.warning_findings == 0 and .summary.target_drifts == 0 and .summary.artifact_problems == 0' "$WORK_DIR/network-verify.json" >/dev/null
  jq -e '.overall.status == "clean" and .network.status == "no_evidence" and .counts.network_transfers == 0' "$WORK_DIR/network-status.json" >/dev/null
  jq -e '.overall.status == "local_target_verified" and .summary.network_transfers == 0 and .pairing.receipt_id != ""' "$WORK_DIR/network-report.json" >/dev/null
  jq -e '.healthy == true and .summary.artifact_problems == 0 and .summary.target_drifts == 0 and .summary.network_transfers == 0' "$WORK_DIR/network-health.json" >/dev/null
  test -f "$NETWORK_TARGET_DIR/data.txt"
  test -f "$NETWORK_TARGET_DIR/empty.txt"
  test -f "$NETWORK_TARGET_DIR/.hidden-network.txt"

  PAIRING_RECEIPT_PATH="$NETWORK_TARGET_DIR/.supermover/pairings/$PAIRING_RECEIPT_ID.json"
  NETWORK_TRANSFER_PATH="$NETWORK_TARGET_DIR/.supermover/sessions/$NETWORK_SESSION_ID/network-transfer.json"
  test -f "$PAIRING_RECEIPT_PATH"
  test -f "$NETWORK_TRANSFER_PATH"
  jq -e '.status == "published" and .stage == "commit" and .privacy_overhead != null and .source_device_id != "" and .target_device_id != ""' "$NETWORK_TRANSFER_PATH" >/dev/null
  if ! grep -q 'transfer=published' "$WORK_DIR/network-push.txt" || ! grep -q 'encrypted_transfer=tls13_mtls' "$WORK_DIR/network-push.txt"; then
    printf 'network push output missing published mTLS evidence\n' >&2
    cat "$WORK_DIR/network-push.txt" >&2
    exit 1
  fi
}
