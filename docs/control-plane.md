# Control Plane

The target-side `.supermover` directory stores machine-readable artifacts for
current `verify`, `deleted list`, `health`, `report`, and `recover` commands.
The Go schema/path foundation in `internal/control` also includes planned
history, recovery-state, pairing, drift, and agent-facing reporting surfaces.

All control-plane documents use JSON with `version: 1`. Writers emit stable,
indented JSON. Readers reject unknown fields and trailing JSON documents, so
each artifact path contains exactly one schema-valid JSON document and schema
drift is visible.

## Paths

Current local writer paths under the target root:

- profile snapshot: `.supermover/profiles/<id>.json`
- session receipt: `.supermover/sessions/<session_id>/receipt.json`
- manifest: `.supermover/sessions/<session_id>/manifest.json`
- warning: `.supermover/warnings/<id>.json`
- soft delete: `.supermover/deleted/<id>.json`
- transaction recovery state: `.supermover/sessions/<session_id>/session.json`

Schema/path foundation for planned artifact surfaces:

- history index: `.supermover/history/index.json`
- pairing receipt: `.supermover/pairings/<id>.json`
- target drift: `.supermover/drift/<id>.json`
- recovery state: `.supermover/recovery/state.json`

## Artifact Schemas

Current local writer schemas:

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
  `mod_time`, `digest`, `target_path`, and `symlink_target`

Strict manifest readers require `symlink_target` for symlink entries. The
compatibility reader used for historical review accepts legacy symlink manifest
entries without `symlink_target` so older control-plane data can still be used
for soft-delete review. Writers always emit `symlink_target` for symlink
entries.

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

`transaction recovery state` records live transaction checkpoints under
`.supermover/sessions/<session_id>/session.json`; see
`docs/recovery.md` for the runtime state machine.

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
unsafe symlink targets, and negative manifest entry sizes. Protocol validation,
publish, recovery, and verify layers reject reserved control-plane target paths;
that protection is not solely a schema-level manifest rule. Transport execution
remains outside this foundation.

Read-only health checks also treat published sessions as unhealthy when their
manifest or receipt artifact is missing or invalid. This keeps recovery status
from looking clean when the transaction record says published but the audit
surface is damaged.
