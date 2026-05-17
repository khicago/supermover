# Control Plane

The target-side `.supermover` directory stores machine-readable artifacts for
current `verify`, `deleted list`, `prune`, `health`, `drift list`, `drift
record`, `drift acknowledge`, `drift resolve`, narrow `reconcile plan/apply`,
`report`, `status`, `recover`, and foreground `daemon` lifecycle commands. The
Go schema/path foundation in
`internal/control` also
includes planned history and recovery-state artifacts plus schema validation
for pairing and target-drift artifacts. `drift record` writes persisted
target-drift artifacts from current live detector findings; `drift resolve`
can close existing persisted drift records when a fresh detector no longer
reports drift for the same path and expected baseline; `reconcile plan/apply`
can handle only selected persisted-drift missing regular-file restores and
already-restored/absent resolve-noop. Broad reconcile/repair UX remains
planned.

Documents owned by `internal/control` use JSON with `version: 1`. Writers emit
stable, indented JSON. Readers reject unknown fields and trailing JSON
documents, so each `internal/control` artifact path contains exactly one
schema-valid JSON document and schema drift is visible.

Transaction session records are separate `internal/transaction` JSON records.
They are not `version: 1` control documents; they are decoded and validated by
the transaction package.

## Paths

Current local `internal/control` writer paths under the target root:

- profile snapshot: `.supermover/profiles/<id>.json`
- session receipt: `.supermover/sessions/<session_id>/receipt.json`
- manifest: `.supermover/sessions/<session_id>/manifest.json`
- warning: `.supermover/warnings/<id>.json`
- soft delete: `.supermover/deleted/<id>.json`

Current local `internal/transaction` writer path:

- transaction session record: `.supermover/sessions/<session_id>/session.json`

Current conditional review artifact paths and planned foundation paths:

- history index: `.supermover/history/index.json`
- pairing receipt: `.supermover/pairings/<id>.json`
- target drift: `.supermover/drift/<id>.json`
- reconcile receipts: not yet durable control-plane artifacts; current
  `reconcile plan/apply` receipts are command output only
- recovery state: `.supermover/recovery/state.json`
- network transfer outcome:
  `.supermover/sessions/<session_id>/network-transfer.json`
- prune approval: `.supermover/prune/approvals/<id>.json`
- prune receipt: `.supermover/prune/receipts/<id>.json`
- daemon install plan: `.supermover/daemon/install.json`
- daemon status: `.supermover/daemon/state.json`
- daemon stop intent: `.supermover/daemon/stop-intent.json`
- daemon restart intent: `.supermover/daemon/restart-intent.json`
- daemon lifecycle events: `.supermover/daemon/events/<id>.json`

## Artifact Schemas

Current local `internal/control` writer schemas:

`daemon` lifecycle records are written by `internal/agentdaemon`, not by the
session transaction writer. They are target control-plane evidence for the
foreground daemon lifecycle surface:

- `install.json`: profile ID, target ID, absolute profile path, install time,
  `run_mode=foreground`, `service_manager=none`, and the foreground run command.
- `state.json`: profile ID, target ID, absolute profile path, status
  (`starting`, `running`, `stopping`, `stopped`, or `failed`), PID, mode,
  pairing/receiver addresses when ready, timestamps, stop intent summary, and
  last error when failed.
- `stop-intent.json`: profile ID, target ID, request time, optional reason, and
  requesting PID.
- `restart-intent.json`: profile ID, target ID, request time, optional reason,
  and requesting PID. A running foreground daemon may consume it and restart
  serve listeners in the same process; it is not an OS service restart request.
- `events/<id>.json`: scoped, redacted lifecycle events used by `daemon logs`
  and included in `daemon status`. These events intentionally do not persist raw
  stderr or pairing verification codes.

These records do not prove OS service installation, detached background process
management, crash supervision, LAN browsing, file watching, or ongoing sync.
`daemon run --foreground` polls `stop-intent.json` and `restart-intent.json`.
Stop exits through the same serve shutdown path; restart tears down and starts
serve listeners again in the same foreground process.

`profile_snapshot` captures the profile SSOT used for a run:

- `version`
- `id`
- `profile_id`
- `session_id`
- `captured_at`
- `profile`: embedded JSON profile payload

Reports decode the embedded `privacy_policy` from this payload and expose it
as profile-snapshot evidence. The current local push slice reports level 2
padding, batching, and jitter bounds as configured profile policy with
`overhead.status=not_applied`; it does not mean traffic shaping was applied to
local file copies.

`session_receipt` records a run:

- `version`
- `id`
- `profile_id`
- `target_id`
- `started_at`
- `ended_at`
- `status`

`network_transfer` records a network transfer attempt:

- peer/session identity and protocol version
- `privacy_policy`: the traffic privacy policy requested for that transfer
- `privacy_overhead`: applied traffic-shaping overhead counters when a network
  attempt applies level 2 behavior, including `padding-v1`
  frame counters (`frame_plain_bytes`, `frame_wire_bytes`, `padding_bytes`,
  `padded_chunks`, and `padding_bucket_bytes`), batch counters, and bounded
  timing jitter counters (`jittered_requests`, `jitter_delay_millis`,
  `max_jitter_delay_millis`, and `jitter_budget_millis`)
- status, stage, timestamps, error, and attempt history

Non-dry-run `push --network` writes this receiver-side artifact through
`networkrun` only after receiver begin stores a session. `push --network
--dry-run` writes no session artifacts. Failures that occur before a stored
receiver session exists, including local TLS setup errors, begin-auth refusal,
and transport setup failure, can leave no `network-transfer.json`. Zero-byte
regular files on the profile-backed `push --network` path are represented by an
explicit final empty completion and, when cleanly published, should have normal
receipt and network-transfer evidence rather than a pre-begin refusal.

`manifest` catalogs restored content:

- `version`
- `id`
- `session_id`
- `root_id`
- `created_at`
- `entries`: each entry has `path`, `kind`, optional `mode`, `size`,
  `mod_time`, `digest`, `target_path`, `symlink_target`, and
  previous-manifest evidence fields `previous_session_id`,
  `previous_manifest_id`, `previous_size`, `previous_digest`,
  `previous_mode`, and `previous_mod_time`

Strict manifest readers require `symlink_target` for symlink entries. The
compatibility reader used for historical review accepts legacy symlink manifest
entries without `symlink_target` so older control-plane data can still be used
for soft-delete review. Writers always emit `symlink_target` for symlink
entries.

Regular file entries may include previous-manifest evidence from the latest
published session for the same profile ID, target ID, and root ID. Local push
uses that evidence to replace a changed target file only when the existing
target still matches the previous `sha256:` digest, size, mode, and
modification time. Missing or partial previous evidence keeps overwrite refusal
conservative. If a staged replacement finds the target missing during recovery,
recovery refuses to publish the new file because the previous target evidence
can no longer be verified automatically unless the interrupted session retained
a matching pair of replacement holds. During an active changed-file publish,
Supermover rechecks the previous target evidence, creates a no-replace previous
snapshot under `.supermover/replacement-holds/<session>/previous/...`, moves the
current target into `.supermover/replacement-holds/<session>/current/...` with
no-replace semantics, then publishes the staged replacement with no-replace
semantics. The local target remains a single-writer surface during a run;
concurrent external writes to a target file are treated as outside the current
safety contract.

`warning` records audit-relevant issues:

- `version`
- `id`
- `session_id`
- `code`
- `message`
- `severity`
- `paths`
- `target_path`
- `detected`
- `suggested_profile_patch`
- `suggested_config`
- `created_at`

`soft_delete` records source-side deletions before physical pruning:

- `version`
- `id`
- `session_id`
- `profile_id`
- `target_id`
- `root_id`
- `previous_session_id`
- `previous_manifest_id`
- `source_path`
- `target_path`
- `kind`
- `size`
- `digest`
- `symlink_target`: emitted for symlink soft deletes when previous manifest
  evidence includes it
- `detected_at`
- `reason`

Soft-delete records are review inputs, not delete authority. Source absence
alone never authorizes deletion from the target.

Current local `internal/transaction` record:

`session.json` records live transaction checkpoints under
`.supermover/sessions/<session_id>/session.json`. It contains `id`, `state`,
`created_at`, `updated_at`, and optional `note`; see `docs/recovery.md` for the
runtime state machine.

Schema foundation for planned or partially wired artifact surfaces:

`history_index` points to known sessions:

- `version`
- `updated_at`
- `latest`
- `sessions`: each entry has `session_id`, `started_at`, and `receipt_ref`

`recovery_state` tracks interrupted or repairable work:

- `version`
- `session_id`
- `status`: one of `clean`, `interrupted`, or `repairing`
- `updated_at`
- `checkpoints`

`pairing_receipt` records explicit target trust:

- `version`
- `id`
- `profile_id`
- `target_id`
- `source_device_id`
- `target_device_id`
- `device_public_key`
- `method`: `sas`, `short_code`, `qr`, or `tofu`
- `verified_at`
- `verification_phrase` and/or `verification_hash`
- `protocol_version`

The durable receipt embeds the same trust event shape as
`transport.PairingReceipt` plus control-plane `version`, `id`, and `target_id`.
Schema validation requires distinct source and target device IDs, a supported
method, a machine-checkable verification timestamp and proof, and
`device_public_key == target_device_id`. The local `internal/pairing` validator
can read `.supermover/pairings/<id>.json` from a profile target and compare the
receipt to the profile SSOT. This is a local evidence check, not an operational
LAN pairing flow.

`target_drift` records target-local divergence from trusted manifest evidence.
Current local push writes these records when it refuses a managed changed-file
update because the target path no longer matches the previous manifest
evidence. The `internal/verify` package also has a detector for comparing
published manifest evidence with the target filesystem, including missing,
changed, type-mismatched, and extra target-local paths. There is not yet a
background detector or periodic scanner. `drift list --profile <path>
[--session <id>] [--format text|json]` exposes the detector as a read-only CLI
that derives the target from the profile and does not persist detector output.
`report` and `status` also expose live detector findings as read-only evidence.
`drift record --profile <path> [--session <id>] [--format text|json]` runs the
same detector and writes current findings as durable
`.supermover/drift/<id>.json` review records. Current `verify`, `health`, and
`report` consume `.supermover/drift/*.json` and surface matching unresolved
persisted records as review-required target state. Valid persisted records with
`review_state=resolved` are still decoded and validated, but they are excluded
from review-required persisted drift counts. `report` also runs the live detector
and exposes its observations under a separate report surface, such as JSON
`live_target_drift` with `live_target_drifts` and
`live_target_drift_artifact_problems` summary counters. Those report/status/list
observations are not written to `.supermover/drift/*.json` unless the operator
runs `drift record`. `drift acknowledge` can mutate only an existing persisted
`.supermover/drift/<id>.json` record after profile scope, published receipt,
manifest, root, and artifact-boundary checks. `drift resolve` uses the same
persisted-record safety boundary, runs a fresh profile-scoped live detector,
and writes `review_state=resolved` only when that detector no longer reports
drift for the same path and expected baseline. Broad reconcile, repair, prune
integration, and background scans remain planned.

Current emitted fields:

- `version`
- `id`
- `session_id`
- `profile_id`
- `target_id`
- `root_id`
- `path`
- `detected_at`
- `change`
- `expected`
- `observed`
- `review_state`
- `reviewed_at`
- `reviewed_by`
- `review_reason`
- `review_action`
- `evidence`

Current records encode the attempted session and scope, the target-relative
path, detection time, a machine category in `change`, structured
expected/observed evidence, `review_state`, and human-readable evidence strings.
The current writer uses `change` values such as
`content_mismatch`, `metadata_mismatch`, and `missing` for refused managed
changed-file updates.

The structured drift-review schema preserves the legacy fields and adds
explicit review evidence instead of relying on prose strings alone:

- expected manifest evidence: previous/published manifest identity and the
  expected file state used for comparison, including kind, size, digest, mode,
  modification time, and symlink target when known
- observed target state: what was found at the target path during refusal or
  drift detection, such as missing, regular file metadata, digest when computed,
  mode, modification time, safe symlink target text, or unsupported type from
  detector observations
- detection time: the durable `detected_at` timestamp for when Supermover
  observed the divergence
- change: the stable machine category for the divergence
- evidence: reviewable supporting facts, retained for text/report output even
  when structured expected/observed fields exist
- `review_state`: operator workflow state for the record
- review metadata: `reviewed_at`, `reviewed_by`, `review_reason`, and
  `review_action` when a wired review command records operator evidence

`review_state` semantics are review metadata, not automatic reconciliation.
`needs_review` means the target must be inspected before the migration can be
treated as clean. `acknowledged` means an operator has seen and intentionally
accepted the divergence as a known condition; it does not rewrite manifests or
resume refused updates by itself. `resolved` means `drift resolve` closed the
persisted record after a fresh profile-scoped live detector no longer reported
drift for the same path and expected baseline. The current local push writer
and `drift record` emit `needs_review`; older drift records without this field
are treated as review-required by `verify`, `health`, and `report`.

`drift acknowledge --profile <path> --id <persisted-drift-id> --reason <text>
[--reviewer <id>] [--format text|json]` is the current narrow CLI path that
mutates target-drift review metadata. It derives the target only from the
profile, accepts IDs from persisted `target_drifts` evidence, including records
created by `drift record`, refuses live-only detector IDs from `drift list`,
`report.live_target_drift`, or `status`, and writes
`review_state=acknowledged`, `reviewed_at`, optional `reviewed_by`,
`review_reason`, and `review_action=acknowledge`. It does not resolve the
record, repair target files, rewrite manifests, suppress live detector output,
resume refused updates, authorize prune, or make a review-required target
clean.

`drift resolve --profile <path> --id <persisted-drift-id> --reason <text>
[--reviewer <id>] [--format text|json]` is the narrow persisted drift closeout
command. It derives the target only from the profile, accepts IDs from
persisted `target_drifts` evidence, refuses live-only detector IDs, and rechecks
the same persisted record, published receipt, manifest, root, and
artifact-boundary evidence as acknowledgement. It additionally runs a fresh
live detector for the record's published baseline and writes
`review_state=resolved`, `reviewed_at`, optional `reviewed_by`,
`review_reason`, and `review_action=resolve` only when the same path and
expected baseline no longer reports drift. It does not repair target files,
rewrite manifests, authorize prune, suppress future detector findings, or
perform broad reconcile.

`reconcile plan --profile <path> --id <persisted-drift-id> [--id <id>...]
[--session <id>] [--format text|json]` is the current non-mutating planner for
selected persisted target-drift records. It derives source roots and target
root only from the profile SSOT and exposes no `--target` or `--state-dir`
override. The planner currently classifies only missing regular-file restore
actions when the persisted expected state is backed by a published manifest and
the current source file still matches that digest/size evidence, plus
resolve-noop actions for already-restored missing-file drift and already-absent
expected-missing paths. Other drift classes are refused.

`reconcile apply --profile <path> --id <persisted-drift-id> [--id <id>...]
--apply --reason <text> [--reviewer <id>] [--session <id>]
[--format text|json]` is the matching explicit mutation path. It takes the
target lock, replans before mutation, restores selected missing regular files
with no-replace target publish semantics, and marks the selected persisted drift
record resolved only after the target evidence matches. It can also resolve the
already-restored or already-absent noop cases without restoring file content.
The current receipt schema is emitted as command output; no durable
`.supermover/reconcile` repair receipt is written yet. Broad automatic
reconcile, conflict-class taxonomy beyond the refusal reason codes, retry
policy, live-only repair, manifest rewrite, background scans, daemon/ongoing
sync integration, and drift-to-prune integration remain planned.

`prune --dry-run` is a current CLI review surface, not a durable control-plane
artifact. It reads published `.supermover/deleted/*.json` records and prints
candidate/refusal evidence with previous manifest evidence and current target
state. It does not write `prune_approval` or `prune_receipt` artifacts.
Soft-delete records whose `delete_policy.retention_days` window is still active
remain visible as `retention_window_active` refusals rather than candidates.
`prune review` reads the same dry-run candidate/refusal evidence plus existing
approval and receipt inventory as a focused read-only release-review surface; it
does not introduce a new durable artifact.
`prune approve --profile <path> --id <approval-id> --soft-delete <id>
[--soft-delete <id>...] --reason <text> --reviewer <id>` is current wired CLI
behavior for authoring `prune_approval` artifacts from fresh dry-run candidate
evidence. `--approved-by` is an alias for `--reviewer`, `--expires-at
<RFC3339>` is optional, and `--format text|json` controls output. Approval
authoring writes the durable approval plus profile snapshot; it does not delete
target files, write `prune_receipt` artifacts, or approve selected IDs that are
not current candidates. The fresh dry-run must be free of refusals and artifact
problems before any approval is written. `prune --apply --approval <id>` is the
only current wired physical prune path.
`prune approvals --profile <path>` is current wired read-only inventory over
current-scope approval artifacts. `prune supersede --profile <path> --id
<approval-id> --reason <text> --reviewer <id>` is current wired approval
mutation for updating one existing approval artifact to durable
`status=superseded` review metadata without applying prune.

`prune_approval` is a current schema and apply input for reviewed physical
prune. `prune approve` authors valid approval artifacts by binding operator
intent to a specific profile, target, root, reviewed profile policy,
soft-delete records, trusted manifest evidence, and a profile snapshot before
any deletion can be attempted.

`report` also reads `.supermover/prune/approvals/*.json` as read-only approval
evidence for the current profile/target, and `status` exposes compact counts
and source breakdown derived from that read path. Parseable approval artifacts
outside the current profile, target, or session scope are skipped rather than
reported as current-profile `prune_approval` problems. Corrupt, unparseable,
symlinked, unreadable, or current-scope invalid approval artifacts may surface
as `prune_approval` artifact problems. This read path distinguishes
authored-but-unapplied approvals from approvals that already have linked prune
receipt evidence, and compact `status` narrows that read path to aggregate
counts plus prune review status/action. These read paths do not author
approvals, supersede approvals, apply prune decisions, write receipts, delete
files or symlinks, automatically release a migration, close v1, or duplicate the full apply-time
validation owned by `prune --apply --approval <id>`.

Fields:

- `version`
- `id`
- `profile_id`
- `target_id`
- `root_id`
- `created_at`
- `approved_by`: required when `status` is `approved`
- `approved_at`: required when `status` is `approved`
- `review_tool`
- `profile_snapshot_id`: required when `status` is `approved`
- `profile_snapshot_digest`: required when `status` is `approved`; SHA-256 over
  canonical JSON profile snapshot payload
- `profile_delete_policy`: snapshot of the prune-relevant profile fields
- `items`: required when `status` is `approved`; each entry has
  `soft_delete_id`, `soft_delete_ref`,
  `detected_session_id`, `previous_session_id`, `previous_manifest_id`,
  `root_id`,
  `source_path`, `target_path`, `kind`, `size`, `digest`, optional
  `symlink_target`, and `detected_at`
- `approval_scope_digest`: required when `status` is `approved`
- `expires_at`
- `status`: one of `approved`, `refused`, or `superseded`
- `approval_reason`
- `refusal_reason`: required unless `status` is `approved`

Approved artifact validation requires the embedded `profile_delete_policy`
snapshot to contain `mode: prune`, `require_review: true`, and
`allow_physical_prune: true`; profile files separately enforce the same gate
under `delete_policy`. Refusal states include profile policy not permitting
prune, active retention windows, missing or invalid soft-delete evidence,
missing or invalid previous manifest evidence, target identity mismatch, target
drift or changed target state, unsafe target path, expired or superseded
approval, and operator refusal. Manual target deletion outside this flow
remains outside the Supermover audit.

`prune_receipt` is current durable output for `prune --apply --approval <id>`.
It records what was rechecked and what happened after approval review. Current
`prune --dry-run` output is not a receipt, does not create receipt files, and
cannot imply deletion happened.

Fields:

- `version`
- `id`
- `prune_session_id`
- `approval_id`
- `profile_id`
- `target_id`
- `started_at`
- `ended_at`
- `status`: one of `planned`, `started`, `applied`, `partial`, or `failed`
- `dry_run`
- `approval_scope_digest`
- `items`: each entry has `soft_delete_id`, `target_path`,
  `intended_action`, `pre_prune_observed`, `result`, optional `error_code` /
  `error`, and optional `pruned_at`
- `refusals`: structured refusal reasons, not prose-only output

Apply writes a no-replace `started` receipt before any deletion. Final prune
receipts must show that target state was checked against trusted manifest and
soft-delete evidence immediately before mutation. `pruned` receipt items require
`pre_prune_observed` and `pruned_at`; `pruned_at` is invalid for non-`pruned`
outcomes. Top-level status must match item outcomes: `applied` means at least
one `pruned` result and no `failed`/`refused` result, `partial` means both
pruned and failed/refused outcomes, and `failed` means failed/refused outcomes
with no `pruned` result. A receipt item with `refused`, `skipped`, `failed`, or
dry-run/started `would_prune` is still audit evidence; it does not imply that
Supermover removed data.

## Validation Baseline

Validation catches missing required IDs and timestamps, invalid recovery
statuses, invalid embedded profile JSON, invalid session identifiers, empty
manifest entry paths/kinds, unsafe symlink targets, negative manifest entry
sizes, negative previous sizes, partial previous evidence, unsupported previous
digest algorithms, and malformed pairing receipts. Protocol validation,
publish, recovery, and verify layers reject reserved control-plane target paths;
that protection is not solely a schema-level manifest rule. Transport execution
remains outside this foundation.

Read-only health checks also treat published sessions as unhealthy when their
manifest or receipt artifact is missing or invalid. This keeps recovery status
from looking clean when the transaction record says published but the audit
surface is damaged.
