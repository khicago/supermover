# Profile SSOT

Profiles are the single source of truth for sync behavior.

Runtime commands should edit or select a profile rather than silently overriding
important policy. Each successful run should store a profile snapshot in the
target control plane so the resulting state can be audited later.

Planned profile sections:

- roots
- include and exclude rules
- consistency mode
- delete policy
- metadata policy
- privacy policy
- pairing and target identity
- supplemental migration rules
- agent knowledge categories

## JSON Schema Foundation

The Go contract lives in `internal/profile`. Profiles are JSON documents with
`version: 1` and deterministic, indented JSON read/write helpers.

Required top-level fields:

- `version`: schema version. Current value is `1`.
- `profile_id`: stable profile identifier.
- `name`: human-readable profile name.
- `roots`: non-empty source root list. Each root has `id` and `path`.
- `consistency`: one of `live`, `strict`, or `snapshot`.
- `delete_policy`: deletion handling policy.
- `metadata_policy`: metadata preservation policy.
- `privacy_policy`: plaintext/restoration privacy policy.
- `target`: pinned target identity and pairing references.
- `agent_knowledge`: categories of agent knowledge files to preserve/catalog.

Optional top-level fields:

- `include`: glob-like include rules, each with `pattern`.
- `exclude`: glob-like exclude rules, each with `pattern`.
- `supplemental_metadata`: string map for future migration annotations.

## Policy Fields

`delete_policy` fields:

- `mode`: one of `ignore`, `record`, or `prune`.
- `require_review`: must be true when `mode` is `prune`.
- `retention_days`: optional non-negative review/prune retention window.
- `allow_physical_prune`: explicit opt-in for future physical pruning.

`metadata_policy` fields:

- `mode`: one of `basic` or `preserve`.
- `preserve_permissions`: preserve file permissions when supported.
- `preserve_mod_time`: preserve modification timestamps when supported.
- `preserve_extended_attr`: preserve extended attributes when supported.

`privacy_policy` fields:

- `mode`: one of `plaintext` or `redacted`.
- `traffic_level`: required traffic privacy level. Project examples should use
  `2` unless the profile intentionally chooses a different privacy posture.
- `allow_plaintext_restore`: must be true when `mode` is `plaintext`.
- `allow_hidden_files`: explicit hidden-file inclusion.
- `allow_sensitive_filenames`: explicit sensitive-filename inclusion.
- `padding_bucket_bytes`: required for traffic level 2.
- `batch_max_bytes`: required for traffic level 2.
- `batch_max_count`: required for traffic level 2.
- `jitter_budget_millis`: bounded timing jitter budget.
- `discovery_low_info`: must be true for traffic level 2.

`target` fields:

- `target_id`: required stable target identifier.
- `name`: optional human-readable target name.
- `local_path`: trusted local target directory for the local push slice.
  Commands update this through `profile set-target` instead of accepting an ad
  hoc push-time override.
- `device_public_key`: optional pinned target device public key.
- `pairing_receipt_id`: optional `.supermover` pairing receipt reference.
- `paired_at`: optional pairing timestamp.

`agent_knowledge.categories` entries:

- `name`: required category name, for example `codex` or `claude`.
- `paths`: optional path patterns belonging to the category.
- `manifest`: whether matching files should be highlighted in manifests.

## Validation Baseline

Validation intentionally catches contract and safety errors before sync logic
runs:

- missing `profile_id`, `name`, target ID, root IDs, or root paths
- empty root list
- invalid consistency, delete, metadata, or privacy modes
- invalid traffic privacy level
- prune mode without review
- negative retention days
- plaintext privacy mode without explicit plaintext restore approval
- traffic level 2 without padding, batching, and low-information discovery
- empty include/exclude patterns
- unnamed agent knowledge categories
