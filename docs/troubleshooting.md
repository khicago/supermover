# Troubleshooting Matrix

Use this matrix to choose the next safe operator action. Prefer collecting
control-plane evidence before rerunning, recovering, pruning, or deleting
anything.

## Scope Boundary

Current troubleshooting applies to the implemented local/mounted migration
slice: `profile`, `scan`, `push`, `verify`, `deleted list`, `health`,
`drift list`, `drift record`, `drift acknowledge`, `drift resolve`, `report`,
`reconcile plan`, `reconcile apply`, `status`, `recover`, non-mutating
`prune --dry-run` candidate/refusal review, reviewed
`prune --apply --approval <id>` over durable approval artifacts, pairing
`serve`/`pair` surfaces, profile-backed receiver routes in paired `serve`
mode, and low-information explicit address hints from `discover`, plus
non-dry-run profile-backed `push --network` transfer, and narrow prune approval
authoring through `prune approve`.
Automated f-22cnwwseh T-004 evidence covers same-session `push --network` resume from an
existing partial receiver session using receiver status offsets and auditable
prior payload-overhead evidence, plus idempotent published-session rerun/commit
retry without reuploading chunks.
Automated f-22rnwcmaz/T-001 evidence covers only the bounded same-session
source-interruption case where profile-backed non-dry-run `push --network`
reaches receiver begin, the receiver accepts payload bytes, in-flight
`network-transfer.json` evidence may be persisted, and a same-profile,
same-session rerun recovers from receiver status with auditable level 2
overhead evidence.
Automated f-22wnwd5pe/T-001 evidence adds a narrower internal `networkrun`
fixture for a source stop immediately after durable in-flight chunk progress
evidence is written; the same session rerun resumes from receiver status,
merges prior privacy-overhead evidence, and publishes matching target content.
This is deterministic acceptance evidence, not a process-kill or daemon
restart claim.
The local migration commands operate from the profile SSOT and target
`.supermover` evidence. The network trust commands currently provide help,
usage validation, explicit address hints, low-information discovery in `serve`,
verified pairing bootstrap, and verification-code pairing receipt/profile
pinning in `pair`. In paired profiles with complete receiver material, `serve`
also exposes upload routes over pinned mutual TLS; non-dry-run `push --network`
connects to those routes through profile-backed pinned TLS 1.3 mTLS.

LAN browsing, discovery trust decisions, OS-managed or detached daemon
behavior, ongoing incremental sync, broad arbitrary interruption recovery,
broad network resume acceptance, network `recover`, broad drift
reconcile/repair, repair receipts, background scans, drift-to-prune
integration, and broader prune release workflow automation are planned unless a
future release gate records command-level evidence for them. The wired daemon
surface is limited to foreground `install`, `run --foreground`, `status`,
`logs`, `restart`, and `stop` lifecycle evidence.
The `drift record` command persists current live detector findings as durable
`.supermover/drift/<id>.json` review records. It does not acknowledge, resolve,
repair, reconcile, prune, suppress future detector findings, or run background
scans.
The `drift acknowledge` command is wired only for existing persisted
`.supermover/drift/<id>.json` records created by refused push or by
`drift record`; it does not acknowledge live detector IDs, repair files,
resolve records, or authorize prune. The `drift resolve` command is wired only
for existing persisted `.supermover/drift/<id>.json` records and closes them
only after a fresh profile-scoped live detector no longer reports the same path
and expected baseline; it does not repair files, rewrite manifests, suppress
future detector findings, broadly reconcile drift, or authorize prune. The
`reconcile plan/apply` surface is separate and narrow: `plan` is non-mutating,
and `apply` requires selected persisted drift IDs, explicit `--apply`, and
`--reason`. It derives source and target from the profile SSOT only, has no
`--target` or `--state-dir` override, and currently repairs only missing
regular files from matching published/source evidence or resolves
already-restored/absent persisted records. It is not broad automatic
reconcile, live-only repair, manifest rewrite, daemon sync, or ongoing sync.
The `prune --dry-run` command surface is wired for non-mutating soft-delete
review, and `prune review` plus `report`
surface prune candidates, refusals, existing receipts, and receipt issues as
read-only evidence. `prune approve` writes durable approval artifacts plus
profile snapshots from fresh dry-run evidence and does not delete target files
or write prune receipts; `prune --apply --approval <id>` remains the only
physical prune path. Protocol-client and `networkrun` paths can emit
traffic privacy level 2 padding, batching, and bounded jitter evidence,
including through non-dry-run `push --network`.
Treat operational references to broader prune release workflow,
drift-to-prune integration, automatic drift repair, or broad drift
reconciliation as future review guidance, not current operator procedures.
Treat `status` as compact local profile/target evidence only, not daemon, LAN,
encrypted-transfer, or sync status. Use `daemon status` only for foreground
daemon lifecycle evidence from `.supermover/daemon`; it does not prove OS
service-manager installation, detached process supervision, crash restart, LAN
browsing, file watching, or ongoing sync.
Treat `serve` as low-information discovery plus verification-code-gated pairing
bootstrap, with authenticated receiver routes only when a paired profile has
complete profile-selected network material; treat `discover` output as
untrusted address hints only.

| Symptom | Likely cause | Evidence to collect | Safe action |
| --- | --- | --- | --- |
| `profile init: ... already exists` | A profile already exists at the chosen path. | Existing profile path and desired source/target. | Edit the existing profile deliberately, then run `profile lint`. Do not overwrite it to change policy silently. |
| `profile init: --profile, --source, and --target are required` | Missing required initialization flag. | Exact command line. | Rerun with all three flags. |
| `profile set-target: --profile and --target are required` | Missing target update flag. | Exact command line. | Rerun with both flags, then lint the profile. |
| `profile lint` fails with validation errors | Profile violates schema or safety policy. | Full profile file and stderr. | Fix the profile. Do not bypass with runtime flags; the profile is the SSOT. |
| `delete_policy.require_review must be true when mode is prune` | Physical prune was enabled without review. | Profile delete policy section. | Set `require_review: true`, review retention and prune policy, then lint again. |
| `delete_policy.allow_physical_prune must be true when mode is prune` | The profile selected prune mode without the explicit physical-prune opt-in. | Profile delete policy section and the operator review record that justified prune mode. | Either set `allow_physical_prune: true` after review or switch back to `record`; do not bypass this with command flags. |
| `delete_policy.allow_physical_prune requires delete_policy.mode prune` | The physical-prune opt-in was set while the profile is not in prune mode. | Profile delete policy section. | Remove `allow_physical_prune` unless the profile intentionally selects reviewed prune mode. |
| `status` or `prune review` shows stale/expired approvals or receipt attention | Existing approval or receipt evidence no longer matches current prune review truth. | `prune review --profile <path> --format json`, `status --profile <path> --format json`, approval JSON, receipt JSON, and current dry-run evidence. | Re-review the current prune state. Author a new approval when the old one is stale or expired. Inspect linked started/failed receipts before retrying apply. Do not treat compact status counts as prune authorization. |
| `plaintext privacy mode without explicit plaintext restore approval` or equivalent validation failure | Target plaintext restore was not explicitly accepted. | Profile privacy policy section. | Set `allow_plaintext_restore` only if the target is trusted; otherwise do not use v1 plaintext restore. |
| Traffic level 2 validation fails | Required padding, batching, jitter, or low-information discovery fields are missing. | Profile privacy policy section. | Restore level 2 fields or choose an explicitly different privacy posture in the profile. |
| `push --network --dry-run` transfers nothing | Dry-run is preflight-only by design. It validates profile, pairing, profile network material, local TLS identity files and pins, scan, and manifest shape without contacting the receiver or writing target artifacts. | Exact command line, stdout/stderr, exit code, profile network section, local TLS identity refs, pairing receipt/profile pins. | Use it as readiness evidence only. Run non-dry-run `push --network --session <id>` for an actual network transfer. |
| New `push --network` session is refused at begin with `refusing to overwrite` | The receiver found a divergent existing target file, symlink, or incompatible directory before accepting payload chunks. Network push is conservative and is not the changed-file sync executor. | Source manifest/scan, target path state, intended session ID, and receiver logs; verify no receiver session artifact was created for the refused new session. | Stop and decide whether the target is meant to be empty/byte-identical or whether this is an update workflow. Do not remove target data or start a new-session retry without review. |
| Non-dry-run `push --network` fails before any `network-transfer.json` exists | The command failed before receiver begin stored a session, such as profile/pairing/network-material refusal, local TLS identity file validation, scan failure, transport setup failure, or begin-auth refusal. Zero-byte regular files are not expected to fail in this pre-begin category on the profile-backed path; they publish through explicit final empty completion when the run is otherwise clean. | Exact command line, stderr, exit code, profile network and privacy sections, source scan evidence, pairing receipt/profile pins, and target `.supermover` listing. | Fix the profile/source/TLS identity issue and rerun. Do not manufacture a network artifact; pre-begin failures intentionally leave none. |
| A clean `push --network` publishes a zero-byte regular file | The profile-backed TLS/mTLS path sent explicit final empty completion evidence through the protocol client. | Command stdout/stderr/exit code, receipt, manifest entry with size `0`, target file presence, and `network-transfer.json`. | Treat this as expected clean published transfer evidence when `health`, `status`, and `report` are otherwise clean. Do not generalize it to LAN browsing, daemon sync, broad resume acceptance, or arbitrary process-kill recovery. |
| Same-session `push --network` rerun reports `resume=receiver_status` | The source consulted receiver status. This line alone does not prove bytes were resumed, because clean first publishes also consult receiver status. | Command stdout/stderr/exit code, `resume_authority`, `resume_outcome`, `resumed_bytes`, session ID, receiver-side `network-session.json`, manifest, receipt, target file digest, prior `network-transfer.json` payload-overhead evidence, and final `network-transfer.json`. | Treat it as supported f-22cnwwseh T-004 resume evidence only when `resume_authority=receiver_status`, `resume_outcome=resumed`, `resumed_bytes` is nonzero, and preserved artifacts show a pre-existing partial receiver session, matching prior payload-overhead evidence, and a rerun that uploaded only remaining bytes before publishing. Preserve `network-transfer.json` as the published transfer and applied-overhead proof. Do not generalize it to arbitrary process-kill recovery or broad interruption acceptance. |
| Rerunning a published `push --network` session uploads no chunks | The receiver reports the session complete; the source revalidates source evidence and retries commit idempotently instead of reuploading payload chunks. | Command stdout/stderr/exit code, `resume_outcome=published_retry`, published receipt/session state, target file digest, and `network-transfer.json`. | This is expected for a matching same-session retry and it can only preserve prior payload padding/batching overhead evidence. If the prior published artifact is missing, corrupt, mismatched, or lacks payload overhead counters, the rerun records `needs_repair` with `payload_overhead_missing`; preserve evidence and block the network release claim until `health`, `status`, and `report` review is clean. |
| `serve` reports missing `network.receiver_url` or `network.local_tls_identity` | The target profile has partial receiver material. `serve` will not silently fall back to pairing-only once any receiver material is present. | Profile `network` section, pairing receipt/profile pins, stderr, and exit code. | Complete the reviewed profile material or remove the partial `network` section before running pairing-only serve. Do not pass certificate/key/address overrides. |
| `daemon run` reports `--foreground is required` | Detached background daemon process management and OS service managers are not wired in this slice. | Exact command line, `daemon --help`, and target `.supermover/daemon` if present. | Run `daemon run --foreground --profile <path>` under your own supervisor, or use `daemon install` only to persist the foreground command plan. |
| `daemon status` shows `stop_requested=true` | A durable `.supermover/daemon/stop-intent.json` exists. A running foreground daemon polls this file and exits through the serve shutdown path. | `daemon status --profile <path> --format json`, `stop-intent.json`, and daemon stdout/stderr. | Preserve the stop intent as lifecycle evidence. If no foreground daemon is running, it remains a pending intent until the next `daemon run --foreground` clears it at startup. |
| `daemon restart` refuses because no foreground daemon is running | Restart is a scoped foreground intent, not PID signaling or OS service control. | `daemon status --profile <path> --format json`, `restart-intent.json` if present, and `.supermover/daemon/events/*.json`. | Start `daemon run --foreground --profile <path>` under your supervisor, then request restart again if needed. Do not infer crash recovery or detached relaunch. |
| `daemon logs` contains lifecycle events but not serve stderr or verification codes | Daemon lifecycle logs are redacted control-plane events, not raw process logs. | `daemon logs --profile <path> --format json`, selected event files, and separate supervisor stdout/stderr if needed. | Use lifecycle events for audit state transitions. Preserve external process logs separately when debugging listener output. |
| `push --network` reports missing `network.receiver_url` or `network.local_tls_identity` | The paired profile lacks the profile-selected endpoint or local certificate/key references required before source-side network transfer or dry-run preflight can proceed. | Profile `network` section, pairing receipt/profile pins, stderr, and exit code. | Add reviewed profile network material; do not pass endpoint or privacy values as command-line overrides. |
| No `network-transfer.json` after `push --network --dry-run` | Dry-run never writes network transfer evidence because it performs no network attempt. | Target `.supermover` listing, command line, stdout/stderr, and `health --profile` / `report --profile` output for available local artifacts. | This is expected. Require `network-transfer.json` only for non-dry-run attempts that reached receiver begin and produced a stored session. |
| `network-transfer.json` has corrupt, missing, or mismatched privacy overhead evidence | Traffic privacy evidence is not auditable, or the artifact does not match the attempted privacy level/session. | The artifact, session receipt, profile snapshot, `health --profile` output, `report --profile --session <session>` output, and artifact validation findings. | Block release of the network transfer path. Preserve artifacts and review through health/report/artifact validation before any release claim. |
| `health`, `status`, or `report` shows a non-published network transfer artifact | A network attempt reached receiver-side artifact writing but ended as `auth_refused`, `interrupted`, `needs_repair`, `publish_failed`, `failed`, or an invalid artifact state. | The generated command output, exit code, `network-transfer.json`, receiver session files, manifest, receipt if present, warning artifacts, and profile snapshot. | Treat the command as a successful review surface only. Do not call the transfer published; use the preserved artifacts to decide rerun, repair, or release-blocking follow-up. |
| Privacy level 2 still exposes total bytes, duration, peer IPs, LAN presence, or Supermover use | Level 2 reduces bounded traffic metadata; it does not provide full traffic anonymity or hide all network observables. | Profile privacy policy, network evidence artifacts if any, pairing/discovery logs, and network environment notes. | Set operator expectations clearly. Do not claim level 2 hides these residual signals, and do not change privacy posture outside the profile SSOT. |
| `scan: scan root "...": not a directory` | Source root is missing, mistyped, or not mounted. | Profile roots and `test -d` result. | Restore or mount the source path, or edit the profile root and lint. |
| Dry run reports unexpected entry count | Include/exclude rules or source root are wrong, or source changed. | Profile, dry-run output, `scan` output. | Stop. Inspect the source and profile before publishing. |
| Dry run reports warnings | Preflight found audit-relevant conditions expected to become warning records if publish can continue. Dry-run output is mainly a warning count. | Dry-run output, optional `scan --format json`, and later warning JSON if published. | Decide whether the count is acceptable for publish. Review full warning JSON after publish. Rerun after profile/source changes if needed. |
| Push fails with `source scan error` | Scanner produced a `scan_error`, so source inventory or soft-delete evidence is unreliable. | stderr, profile roots, source permissions/mount state, optional `scan --format json`. | Fix source readability and rerun dry-run. Do not publish from a scan that has `scan_error`. |
| Published run reports warnings | Local push completed but wrote reviewable warning artifacts. | `.supermover/warnings/*.json`, session receipt, manifest. | Review every warning. Accept, rerun with changed profile/source, or block release. |
| `verify` exits 1 with warnings but no errors | The manifest selected for review has warning findings, such as missing or unsupported digest evidence. | `verify --format json`, manifest entry, target file state. | Treat as a release blocker until explained. Repair evidence or rerun with a manifest that can be fully verified. |
| `verify` reports mode or mtime mismatch | Target regular file metadata no longer matches the manifest. | `verify --format json`, target `stat`, manifest mode/modtime. | Preserve evidence. Determine whether the target was modified after publish or the manifest is stale, then rerun or recover according to session state. |
| `verify` reports missing directory or symlink mismatch | Published non-file entries no longer match the manifest. | `verify --format json`, manifest entry, target `ls -l`/`readlink` output. | Treat as a release blocker until the target tree is repaired or the session is rerun. |
| `drift list` exits non-zero | The selected published manifest does not match the target filesystem, the selected artifacts are damaged, no published manifest exists, or the target/control-plane boundary is unsafe. | `drift list --profile <path> --format json`, optional `--session <id>`, selected manifest/receipt, target file state, profile target path, and stderr for runtime selection or unsafe-boundary errors. | Preserve evidence. Treat output as review-required; this command does not persist detector output, acknowledge, resolve, or repair drift. Prefer JSON for generated reports; text and runtime diagnostics are escaped for operator review. Use `drift record` to persist current findings before acknowledgement review. |
| `dashboard` reports `review_required` | The target-side read-only page found verification findings, live extra/mismatched target paths, review artifacts, or no latest published manifest. | Dashboard raw JSON, `verify --format json`, `drift list --format json`, selected receipt/manifest, and target `.supermover` evidence. | Treat the page as a review surface only. It does not repair, persist drift, compare source changes after publish, or expose a Merkle root. Run it locally or over SSH forwarding using the emitted access-token URL; it intentionally refuses LAN binding. |
| `report` exits non-zero with `target_drifts`, `live_target_drift`, or live drift text | Persisted unresolved target-drift records require review, or the report's live detector found target filesystem drift or artifact problems at report time. An explicit `--session <id>` with scoped persisted drift or artifact evidence can still produce a structured report without a selected manifest; an explicit missing-session request with no scoped report evidence is a stderr-only selection error. | `report --profile <path> --format json`, optional `--session <id>`, `verify --profile <path> --format json`, `drift list --profile <path> --format json`, selected manifest/receipt, target file state, profile target path, and stderr for runtime selection errors. | Preserve report output when one is produced, or preserve stderr and artifacts for selection errors. `drift acknowledge --profile <path> --id <persisted-drift-id> --reason <text>` can add review metadata only to persisted drift records from refused push or `drift record`; `drift resolve --profile <path> --id <persisted-drift-id> --reason <text>` can close a persisted record after target restoration makes the same path and expected baseline clean under a fresh detector. `reconcile plan/apply` can repair only selected persisted missing regular-file drift from published/source evidence, or resolve already-restored/absent persisted records. Do not treat live report drift as a persisted `.supermover/drift/*.json` record until `drift record` writes it. No current command prunes, suppresses, live-repairs, or broadly reconciles drift. |
| `drift acknowledge` exits non-zero | The command was given a live detector ID, a missing or unsafe ID, no reason, a persisted drift record outside the selected profile/target/root scope, a record without published receipt/manifest evidence, an unsafe artifact boundary, or a record that is already acknowledged or resolved. | Exact command line, stderr, `verify --profile <path> --format json`, `report --profile <path> --format json`, or `drift record --profile <path> --format json` showing the persisted drift ID, selected manifest/receipt evidence, profile target path, and the drift artifact path under `.supermover/drift/`. | Do not retry with IDs from `drift list` or `report.live_target_drift` unless `drift record` has persisted them first. Preserve evidence, choose a persisted drift ID, provide a reviewed reason, fix missing scope/evidence issues, or leave already reviewed/resolved records unchanged. Acknowledgement does not repair or resolve drift. |
| `drift resolve` exits non-zero | The command was given a live detector ID, a missing or unsafe ID, no reason, a persisted drift record outside the selected profile/target/root scope, a record without published receipt/manifest evidence, an unsafe artifact boundary, a record that is already resolved, or a fresh live detector still reports drift for the same path and expected baseline. | Exact command line, stderr, `verify --profile <path> --format json`, `report --profile <path> --format json`, `drift list --profile <path> --format json`, or `drift record --profile <path> --format json` showing the persisted drift ID, selected manifest/receipt evidence, profile target path, current target file state, and the drift artifact path under `.supermover/drift/`. | Do not retry with IDs from `drift list` or `report.live_target_drift` unless `drift record` has persisted them first. Preserve evidence, restore or intentionally change the target outside Supermover according to operator policy, rerun `drift list`, then retry resolve only when the same path and expected baseline is clean. Resolve does not repair files, rewrite manifests, suppress live detector output, authorize prune, or broadly reconcile drift. |
| `reconcile plan` exits non-zero or prints refusals | The selected ID is not a persisted drift record for the profile, artifacts are unreadable, published manifest evidence is missing, source evidence no longer matches, the target path is unsafe, or the drift class is outside the narrow missing regular-file/resolve-noop slice. | Exact command line, stderr/stdout, selected `.supermover/drift/<id>.json`, published receipt and manifest, profile source/target roots, current source file metadata/digest, and target path state. | Treat the plan as review evidence only. It is non-mutating. Fix profile/artifact/source issues or leave the drift for manual review; do not use live-only detector IDs or runtime target/state-dir overrides. |
| `reconcile apply` exits non-zero or prints refusals | The command is missing selected `--id`, explicit `--apply`, or `--reason`; the target changed after planning; source or published evidence no longer matches; the drift artifact changed; target paths are unsafe; or the drift class is unsupported. | Exact command line, stdout/stderr, prior `reconcile plan --format json`, selected drift artifact, published receipt/manifest, source file state, target file state, and any emitted reconcile receipt. | Preserve all evidence and inspect whether any file content changed. Rerun only after a fresh plan still selects the intended persisted IDs. Current apply does not write durable repair receipts, consume live-only drift, rewrite manifests, retry in the background, or run broad automatic reconcile. |
| `status` exits `1` and prints review-required evidence | The compact local status report was generated, but the current profile-selected target has warnings, soft deletes, unresolved persisted drift, live drift, verification findings, pairing evidence issues, recovery work, invalid artifacts, non-published or invalid network-transfer artifact issues, or no published manifest requiring review. | `status --profile <path> --format json`, `report --profile <path> --format json`, `health --profile <path>`, optional `drift list --profile <path> --format json`, selected manifests/receipts, profile target path, and target file state. | Preserve the emitted status and richer report evidence. Treat exit `1` as a successful review surface, not a CLI failure. `status` does not repair, persist live detector output, start a daemon, or prove network transfer readiness; use `drift record` when current live drift must become durable review evidence and `drift resolve` only for existing persisted records after a fresh detector no longer reports drift for the same path and expected baseline. |
| `status` exits `2` without a status report | Usage, profile loading, profile validation, target-selection, unsafe boundary, or report-generation failure prevented status from producing structured evidence. | Exact command line, stderr, profile file, target path permissions, and target `.supermover` listing if reachable. | Fix the input or target accessibility before using `status` as release evidence. Do not infer local target health, daemon state, encrypted transfer, sync, or network privacy status from an exit `2`. |
| Symlink publish conflict | Local push refuses to overwrite an existing non-symlink or divergent symlink target. | Warning with code `symlink_not_published`, target path state, previous manifest/soft-delete records. | Preserve the existing target as review evidence. Manually decide whether to prune or move it before rerunning from a new session. |
| Target lacks `.supermover` after dry run | `push --dry-run` intentionally does not write target state. | Command line containing `--dry-run`. | Run non-dry-run push when ready to publish. |
| Target lacks `.supermover` after non-dry-run push | Push failed before control artifacts, wrong target path, or permission issue. | stdout/stderr, exit code, target path, filesystem permissions. | Fix the failure and rerun with a new session ID or cleanly documented retry decision. |
| Session receipt status is not `published` | Interrupted or incomplete transaction state. | `.supermover/sessions/<session>/`, receipt/session files, command logs, `health --profile` output. | Preserve evidence. Use `health` for read-only classification, then `recover --profile` for the conservative local recovery subset. |
| Manifest exists but restored files are missing | Target was modified after publish, publish failed, or wrong target was inspected. | Manifest, session receipt, target path listing, drift notes if any. | Preserve `.supermover`; compare manifest paths to target; do not prune or delete evidence. |
| Warning files are deleted to "clean up" the target | Audit trail was damaged. | Backups, session output showing warning count. | Restore warning files from backup if possible. Treat the run as not fully auditable if warnings cannot be reconstructed. |
| Agent influence count is unexpected | Agent rule/state files were detected or omitted by profile/source rules. | `.supermover/agent/<session>-influence.json`, profile `agent_knowledge`, source tree. | Review whether these files should be migrated and cataloged. Edit profile rules if needed. |
| Discovery shows an unknown target | Current explicit-address discovery emits unauthenticated address hints; future LAN browsing must remain hint-only too. | Discovery advertisement fields and network context. | Do not trust or push to it. Pair only after explicit verification and identity pinning. |
| Discovery advertisement contains hostname, username, path, profile name, or inventory size | Discovery implementation leaked more than low-information hints. | Raw advertisement/TXT fields. | Block release of discovery. Advertisements must be limited to service, protocol, nonce, and minimal capabilities. |
| Pairing target identity differs from profile target | Device identity pinning caught a target mismatch. | Pairing receipt, profile target section, target device key/fingerprint. | Stop. Verify the physical/logical target. Do not update the profile unless the target change is intentional and reviewed. |
| Disk full or quota error during publish | Target filesystem cannot accept staged or final files. | stderr, free-space output, session state under `.supermover`. | Free space, preserve session evidence, then retry or recover according to session state. |
| Cross-device rename or promotion failure | Temporary and final files were not on the same filesystem. | Error text, source/target mount information, session state. | Keep staging on the same target filesystem; rerun after layout fix. |
| Operator wants to change delete, privacy, or metadata behavior for one run | Runtime override would break auditability. | Requested change and profile diff. | Change the profile, lint it, and keep the profile snapshot. |
| Operator wants to change the local target for one run | Target selection is profile-owned. | Requested target path and current profile target section. | Run `profile set-target --profile <path> --target <target>`, lint, then push. Do not change `target.target_id` unless the target identity changes intentionally. |

## Evidence Commands

Local/mounted release gate commands:

```bash
go mod tidy -diff
go test -count=1 ./...
go test -race -count=1 ./...
go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
go vet ./...
staticcheck ./...
golangci-lint run ./...
git diff --check
go run ./cmd/supermover help
go run ./cmd/supermover version
go run ./cmd/supermover profile help
go run ./cmd/supermover push --help
go run ./cmd/supermover verify --help
go run ./cmd/supermover deleted help
go run ./cmd/supermover health --help
go run ./cmd/supermover drift help
go run ./cmd/supermover drift record --help
go run ./cmd/supermover drift acknowledge --help
go run ./cmd/supermover drift resolve --help
go run ./cmd/supermover reconcile --help
go run ./cmd/supermover report --help
go run ./cmd/supermover status --help
go run ./cmd/supermover recover --help
go run ./cmd/supermover prune --help
go run ./cmd/supermover serve --help
go run ./cmd/supermover discover --help
go run ./cmd/supermover pair --help
```

Manual local smoke commands:

```bash
SMOKE_ROOT="$(mktemp -d)"
SRC="$SMOKE_ROOT/source"
DST="$SMOKE_ROOT/target"
PROFILE="$SMOKE_ROOT/supermover.profile.json"
SESSION="smoke-local"
BIN="$SMOKE_ROOT/supermover"
mkdir -p "$SRC/subdir" "$DST"
printf 'hello\n' > "$SRC/subdir/file.txt"
printf 'hidden\n' > "$SRC/.hidden"

go build -o "$BIN" ./cmd/supermover
"$BIN" profile init --profile "$PROFILE" --source "$SRC" --target "$DST"
"$BIN" profile lint --profile "$PROFILE"
"$BIN" push --profile "$PROFILE" --dry-run
"$BIN" push --profile "$PROFILE" --session "$SESSION"
"$BIN" verify --profile "$PROFILE" --session "$SESSION"
"$BIN" status --profile "$PROFILE"
rm "$SRC/subdir/file.txt"
"$BIN" push --profile "$PROFILE" --session "${SESSION}-delete"
"$BIN" deleted list --profile "$PROFILE"
"$BIN" health --profile "$PROFILE"
"$BIN" drift list --profile "$PROFILE"
"$BIN" report --profile "$PROFILE" --session "${SESSION}-delete" || test $? -eq 1
"$BIN" status --profile "$PROFILE" || test $? -eq 1
"$BIN" recover --profile "$PROFILE" --dry-run
```

Operational evidence commands:

```bash
go run ./cmd/supermover profile lint --profile ./supermover.profile.json
go run ./cmd/supermover profile set-target --profile ./supermover.profile.json --target /path/to/target
go run ./cmd/supermover scan --profile ./supermover.profile.json
go run ./cmd/supermover health --profile ./supermover.profile.json
go run ./cmd/supermover drift list --profile ./supermover.profile.json --format json
go run ./cmd/supermover drift record --profile ./supermover.profile.json --format json
go run ./cmd/supermover drift resolve --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<target restored reason>" --format json
go run ./cmd/supermover reconcile plan --profile ./supermover.profile.json --id <persisted-drift-id> --format json
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --id <persisted-drift-id> --apply --reason "<operator repair reason>" --format json
go run ./cmd/supermover report --profile ./supermover.profile.json --format json
go run ./cmd/supermover status --profile ./supermover.profile.json --format json
find /path/to/target/.supermover -maxdepth 4 -type f | sort
sed -n '1,160p' /path/to/target/.supermover/sessions/<session>/receipt.json
find /path/to/target/.supermover/warnings -type f -name '*.json' -maxdepth 1 2>/dev/null | sort
```

## Mainline Commands

Current local migration commands:

```bash
go run ./cmd/supermover recover --profile ./supermover.profile.json --session <session-id>
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <session-id>
go run ./cmd/supermover deleted list --profile ./supermover.profile.json
go run ./cmd/supermover drift list --profile ./supermover.profile.json [--session <session-id>] [--format text|json]
go run ./cmd/supermover drift record --profile ./supermover.profile.json [--session <session-id>] [--format text|json]
go run ./cmd/supermover drift resolve --profile ./supermover.profile.json --id <persisted-drift-id> --reason <text> [--format text|json]
go run ./cmd/supermover reconcile plan --profile ./supermover.profile.json --id <persisted-drift-id> [--format text|json]
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --id <persisted-drift-id> --apply --reason <text> [--format text|json]
go run ./cmd/supermover status --profile ./supermover.profile.json [--format text|json]
```

Current network trust skeletons and discovery hints:

```bash
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover discover --address 127.0.0.1:9000
go run ./cmd/supermover pair --profile ./supermover.profile.json --target <address> --verification-code <code>
```

`discover` emits untrusted explicit address hints only and does not browse LAN
services. `--address` values are operator-provided hint material and still leak
peer address metadata. `serve` exposes low-information discovery and gates
pairing bootstrap behind the target-console verification code; when the profile
is already paired and has complete receiver URL plus local TLS identity material,
it also mounts receiver upload routes over pinned mutual TLS. `pair` requires
the target-console verification code before writing pairing receipt/profile
pins. These commands do not advertise on LAN; non-dry-run `push --network` is
the separate source transfer command, while `push --network --dry-run` is
preflight-only.

Current reviewed prune command surface:

```bash
go run ./cmd/supermover prune --help
go run ./cmd/supermover prune --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover prune approve --profile ./supermover.profile.json --id <approval-id> --soft-delete <soft-delete-id> --reason "reviewed for prune" --reviewer <reviewer-id>
go run ./cmd/supermover prune --profile ./supermover.profile.json --apply --approval <id>
```

`prune --help` is command-surface evidence only. `prune --dry-run` validates
the profile prune policy, reads published soft-delete records, and emits
review-only candidates, refusals, and artifact problems without writing approval
or receipt artifacts or deleting target files. Candidates, refusals, and
artifact problems return exit `1` because operator review is required; an empty
clean dry-run returns exit `0`. Do not use dry-run candidates alone as
physical-prune acceptance evidence. Apply evidence must include the existing
approval artifact, command exit code/stdout/stderr, and the resulting
`.supermover/prune/receipts/<id>.json` receipt.

Future network product gates must stay separate from the local gate. The current
non-dry-run `push --network` path has evidence for authenticated pairing,
encrypted transport, bounded same-session source-interruption rerun after
receiver-accepted payload bytes, deterministic source-stop-after-progress
resume at the `networkrun` layer, receiver listener restart over preserved
target state, commit-only retry, idempotent published-session commit retry,
network transfer artifacts, and traffic privacy level 2 overhead evidence from
padding, batching, and bounded jitter. A rerun must use the same profile and
session and can rely on receiver status only when prior payload-overhead
evidence remains auditable. Missing, corrupt, mismatched, non-published, or
payload-empty prior transfer evidence should block with `needs_repair` /
`payload_overhead_missing`; this is correct fail-closed behavior. `recover` is
not a network recovery command. Level 2 traffic privacy is not anonymity and
does not hide total bytes, transfer duration, peer IP addresses, LAN presence,
or Supermover use. LAN browsing, daemon behavior, ongoing incremental sync,
broad arbitrary interruption recovery, broad resume acceptance, arbitrary
process-kill recovery, power-loss recovery, and receiver recovery UX remain
planned until command evidence says otherwise.
