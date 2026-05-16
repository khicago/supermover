# Control Plane

The target-side `.supermover` directory stores machine-readable artifacts for
current `verify`, `deleted list`, and `health` commands and planned recover,
prune, history, drift, and agent-facing reporting commands. The Go schema
foundation lives in `internal/control`.

All control-plane documents use JSON with `version: 1`. Writers emit stable,
indented JSON and readers reject unknown fields so schema drift is visible.

## Paths

Path helpers resolve artifacts under the target root:

- profile snapshot: `.supermover/profiles/<id>.json`
- pairing receipt: `.supermover/pairings/<id>.json`
- session receipt: `.supermover/sessions/<session_id>/receipt.json`
- manifest: `.supermover/sessions/<session_id>/manifest.json`
- warning: `.supermover/warnings/<id>.json`
- target drift: `.supermover/drift/<id>.json`
- soft delete: `.supermover/deleted/<id>.json`
- history index: `.supermover/history/index.json`
- recovery state: `.supermover/recovery/state.json`

## Artifact Schemas

`profile_snapshot` captures the profile SSOT used for a run:

- `version`
- `id`
- `profile_id`
- `session_id`
- `captured_at`
- `profile`: embedded JSON profile payload

`pairing_receipt` records explicit target trust:

- `version`
- `id`
- `profile_id`
- `target_id`
- `device_public_key`
- `verified_at`

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
- `entries`: each entry has `path`, `kind`, optional `size`, `mod_time`,
  `digest`, and `target_path`

`warning` records audit-relevant issues:

- `version`
- `id`
- `session_id`
- `code`
- `message`
- `paths`
- `created_at`

`target_drift` records target-local changes detected after sync:

- `version`
- `id`
- `session_id`
- `path`
- `detected_at`
- `change`
- `evidence`

`soft_delete` records source-side deletions before physical pruning:

- `version`
- `id`
- `session_id`
- `source_path`
- `target_path`
- `detected_at`
- `reason`

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

## Validation Baseline

Validation catches missing required IDs and timestamps, invalid recovery
statuses, invalid embedded profile JSON, empty manifest entry paths/kinds, and
negative manifest entry sizes. Full sync semantics, digest verification,
transport, and recovery execution are intentionally outside this foundation.
