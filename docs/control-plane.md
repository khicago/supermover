# Control Plane

The target-side `.supermover` directory stores machine-readable artifacts for
current `verify`, `deleted list`, `health`, `report`, and `recover` commands.
The Go schema/path foundation in `internal/control` also includes planned
history, recovery-state, pairing, and drift artifacts.

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

Schema/path foundation for planned artifact surfaces:

- history index: `.supermover/history/index.json`
- pairing receipt: `.supermover/pairings/<id>.json`
- target drift: `.supermover/drift/<id>.json`
- recovery state: `.supermover/recovery/state.json`

## Artifact Schemas

Current local `internal/control` writer schemas:

`profile_snapshot` captures the profile SSOT used for a run:

- `version`
- `id`
- `profile_id`
- `session_id`
- `captured_at`
- `profile`: embedded JSON profile payload

`session_receipt` records a run:

- `version`
- `id`
- `profile_id`
- `target_id`
- `started_at`
- `ended_at`
- `status`

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
- `detected_at`
- `reason`

Current local `internal/transaction` record:

`session.json` records live transaction checkpoints under
`.supermover/sessions/<session_id>/session.json`. It contains `id`, `state`,
`created_at`, `updated_at`, and optional `note`; see `docs/recovery.md` for the
runtime state machine.

Schema foundation for planned artifact surfaces:

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
- `device_public_key`
- `verified_at`

`target_drift` records target-local changes detected after sync:

- `version`
- `id`
- `session_id`
- `path`
- `detected_at`
- `change`
- `evidence`

## Validation Baseline

Validation catches missing required IDs and timestamps, invalid recovery
statuses, invalid embedded profile JSON, empty manifest entry paths/kinds,
unsafe symlink targets, negative manifest entry sizes, negative previous sizes,
partial previous evidence, and unsupported previous digest algorithms. Protocol
validation, publish, recovery, and verify layers reject reserved control-plane
target paths; that protection is not solely a schema-level manifest rule.
Transport execution remains outside this foundation.

Read-only health checks also treat published sessions as unhealthy when their
manifest or receipt artifact is missing or invalid. This keeps recovery status
from looking clean when the transaction record says published but the audit
surface is damaged.
