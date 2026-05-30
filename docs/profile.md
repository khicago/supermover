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

The Go contract lives in `internal/profile`. Profiles are single JSON documents
with `version: 1` and deterministic, indented JSON read/write helpers. Readers
reject unknown fields and trailing JSON documents so runtime behavior cannot
silently fork from the reviewed profile.

Required top-level fields:

- `version`: schema version. Current value is `1`.
- `profile_id`: stable profile identifier.
- `name`: human-readable profile name.
- `roots`: non-empty source root list. Each root has `id` and `path`.
- `consistency`: one of `live`, `strict`, or `snapshot`.
- `delete_policy`: deletion handling policy.
- `metadata_policy`: metadata preservation policy.
- `privacy_policy`: plaintext/restoration privacy policy.
- `target`: pinned target identity, local reachability path, and pairing
  references.
- `agent_knowledge`: categories of agent knowledge files to preserve/catalog.

Optional top-level fields:

- `include`: glob-like include rules, each with `pattern`.
- `exclude`: glob-like exclude rules, each with `pattern`.
- `network`: profile-backed receiver and source-side network-transfer
  connection material.
- `sync`: profile-backed local incremental sync runtime settings.
- `supplemental_metadata`: string map for future migration annotations.

## Policy Fields

`delete_policy` fields:

- `mode`: one of `ignore`, `record`, or `prune`.
- `require_review`: must be true when `mode` is `prune`.
- `retention_days`: optional non-negative elapsed-day review/prune retention
  window. A soft-delete record is not a prune candidate until
  `detected_at + retention_days * 24h` has elapsed.
- `allow_physical_prune`: explicit opt-in required when `mode` is `prune`.

`prune --dry-run` is wired as non-mutating soft-delete review: it validates the
profile prune policy and emits candidate/refusal evidence without approvals,
receipts, or deletion. Active retention windows are emitted as
`retention_window_active` refusals, not candidates. `prune approve --profile
<path> --id <approval-id> --soft-delete <id> [--soft-delete <id>...]
--reason <text> --reviewer <id>` writes a durable approval artifact plus
profile snapshot from fresh dry-run candidate evidence; `--approved-by` is an
alias for `--reviewer`, `--expires-at <RFC3339>` is optional, and
`--format text|json` controls output. Approval authoring does not delete target
files or write prune receipts. It writes approvals only when the fresh dry-run
has no refusals or artifact problems, and selected IDs must be current dry-run
candidates.
`prune review --profile <path>` reads the same current candidate/refusal
evidence plus existing approval and receipt inventory as a focused read-only
release-review surface; it does not write approvals, receipts, or target files.
`prune --apply --approval <id>` is the only physical prune path; it rechecks
target state against trusted manifest and soft-delete evidence, writes a
started receipt before mutation, re-runs the current prune plan including the
retention gate, and records applied/partial/failed status when finalization
succeeds. If final receipt writing is interrupted, the durable `started`
receipt remains review evidence. `report` is read-only and surfaces prune
candidates, refusals, existing receipts, and receipt issues. The profile
validator reserves
the policy gate: `delete_policy.mode: prune` is valid only when
`delete_policy.require_review: true` and
`delete_policy.allow_physical_prune: true`; `allow_physical_prune: true` is
invalid outside `mode: prune`. Source absence alone never authorizes target
deletion.

`metadata_policy` fields:

- `mode`: one of `basic` or `preserve`.
- `preserve_permissions`: preserve file permissions when supported.
- `preserve_mod_time`: preserve modification timestamps when supported.
- `preserve_extended_attr`: preserve extended attributes when supported.

`privacy_policy` fields:

- `mode`: one of `plaintext` or `redacted`.
- `traffic_level`: declared traffic privacy posture. Project examples may use
  `2` to exercise the level 2 schema. Local push does not apply encrypted
  network transfer or traffic shaping. Current operator `push --network`
  supports only traffic level 2 and applies the protocol-client level 2
  padding, batching, and jitter path when the profile selects level 2. Levels 1
  and 3 remain schema/planning values for this network path.
- `allow_plaintext_restore`: must be true when `mode` is `plaintext`.
- `allow_hidden_files`: explicit hidden-file inclusion.
- `allow_sensitive_filenames`: explicit sensitive-filename inclusion.
- `padding_bucket_bytes`: required for traffic level 2.
- `batch_max_bytes`: required for traffic level 2.
- `batch_max_count`: required for traffic level 2.
- `jitter_budget_millis`: required bounded timing jitter budget for traffic
  level 2.
- `discovery_low_info`: must be true for traffic level 2.

The level 2 fields describe bounded padding, batching, and jitter policy. The
receiver protocol client applies padding, batching, and bounded timing jitter
for network runs and records the applied overhead in network transfer artifacts.
These reductions do not hide total bytes, transfer duration, peer IP addresses,
LAN presence, or Supermover use. Privacy behavior is profile-only: `push` and
`push --network` do not accept transient padding, jitter, batching, or privacy
flags that would bypass the profile snapshot audit trail.

`target` fields:

- `target_id`: required stable target identifier.
- `name`: optional human-readable target name.
- `local_path`: trusted local target directory for the local push slice.
  Commands update this through `profile set-target` instead of accepting an ad
  hoc push-time override.
  `local_path` is a reachability/write location, not a durable target identity;
  changing it must not change `target_id`. Once a profile is paired, target
  identity rotation requires an explicit re-pair flow, not `profile set-target
  --target-id`.
- `device_public_key`: optional pinned target device public key or fingerprint.
- `pairing_receipt_id`: optional `.supermover` pairing receipt reference.
- `paired_at`: optional RFC3339 pairing timestamp.

Paired target fields are atomic. If any of `device_public_key`,
`pairing_receipt_id`, or `paired_at` is set, all three must be set. The
`device_public_key` must pass schema-level device ID validation, and
`pairing_receipt_id` must be a safe control artifact id. These checks do not
authenticate a network peer by themselves; they only make the profile pins
machine-checkable for future pairing/transport gates.
Operator reports classify these pins as `unpaired` until all fields and the
referenced receipt validate, `paired_receipt_valid` when they match, and
receipt mismatch/missing/invalid states when profile pins and receipt evidence
disagree.

`network` fields:

- `receiver_url`: optional profile-selected receiver base URL. Paired target
  `serve` uses it for the receiver listener when the rest of
  `local_tls_identity` is complete; non-dry-run `push --network` uses it as the
  source-side receiver endpoint. It must be `https`, include an explicit IP host
  and port, and contain no userinfo, query, fragment, or path beyond `/`.
- `local_tls_identity.certificate_path`: optional path to the local device TLS
  certificate for the mTLS receiver/source transfer path.
- `local_tls_identity.private_key_path`: optional path to the local private key
  matching that certificate.

The certificate and key path fields are atomic: if either path is set, both
must be set. They must be absolute local file references, private keys must stay
on the owning device, and the paths must not use parent traversal, backslash
separators, or target `.supermover` control-plane space. `serve` uses complete
paired receiver material to mount authenticated receiver upload routes over
pinned mutual TLS. `push --network` refuses a paired profile that lacks
`network.receiver_url` or a complete `network.local_tls_identity`. Without
`--dry-run`, it loads the local TLS identity from the profile, connects to the
pinned TLS 1.3 mTLS receiver, and writes network transfer outcome artifacts
through the protocol client/network-run path. With `--dry-run`, it performs
preflight only and writes no target artifacts.

`discovery` fields:

- `advertise_receiver_hint`: optional boolean. When true, `discover advertise
  --profile <path>` binds its UDP advertisement source to the profile-selected
  `network.receiver_url` host:port so `sync network discover-run` can require a
  LAN candidate that matches the reviewed receiver endpoint before running the
  existing profile-pinned mTLS network queue pass.

`discovery.advertise_receiver_hint` requires complete `network.receiver_url`
and `network.local_tls_identity` material. It does not add identity trust to
LAN discovery, does not write or mutate profiles, and does not expose profile
IDs, device IDs, paths, usernames, hostnames, or file metadata in the
advertisement payload.

`sync` fields:

- `local_polling.enabled`: optional boolean. When true,
  `daemon run --foreground` runs the same local incremental queue consumer used
  by `sync loop` in the foreground daemon process.
- `local_polling.interval_millis`: positive polling interval required when
  local polling is enabled.
- `local_polling.retry_backoff_millis`: non-negative retry backoff duration.
  A positive value is required when local polling is enabled.
- `local_polling.session_prefix`: generated run receipt prefix required when
  local polling is enabled. It must be a safe Supermover session-id prefix.
- `network_polling.enabled`: optional boolean. When true,
  `daemon run --foreground` runs the same profile-backed network queue pass
  used by `sync network loop` as a source-side worker-only foreground daemon
  mode.
- `network_polling.interval_millis`: positive polling interval required when
  network polling is enabled.
- `network_polling.retry_backoff_millis`: non-negative retry backoff duration.
  A positive value is required when network polling is enabled.
- `network_polling.session_prefix`: generated network run receipt prefix
  required when network polling is enabled. It must be a safe Supermover
  session-id prefix.

`repair` fields:

- `drift_recording.enabled`: optional boolean. When true,
  `daemon run --foreground` runs a target-side worker that periodically records
  current live detector findings as durable `.supermover/drift/*.json` review
  evidence.
- `drift_recording.interval_millis`: positive polling interval required when
  drift recording is enabled.
- `persisted_reconcile_apply.enabled`: optional boolean. When true,
  `daemon run --foreground` periodically reviews persisted drift records and
  applies only currently planned persisted reconcile actions through the
  existing reconcile apply receipt path.
- `persisted_reconcile_apply.interval_millis`: positive polling interval
  required when persisted reconcile apply is enabled.
- `persisted_reconcile_apply.reason`: required review reason written to
  resolved drift records and reconcile receipts when persisted reconcile apply
  is enabled.
- `persisted_reconcile_apply.reviewer`: optional reviewer/operator identifier
  written to resolved drift records and reconcile receipts.

Daemon polling sync is profile-only. `sync.local_polling` and
`sync.network_polling` are mutually exclusive because they select different
publication paths. The daemon command does not accept a runtime `--sync`
override because migration behavior must remain auditable from the reviewed
profile and target-side `.supermover` artifacts. The foreground daemon writes
ordinary incremental-sync queue/run receipts and redacted `daemon_sync_*`
lifecycle events. A foreground restart consumes
`.supermover/daemon/restart-intent.json`, reloads the profile, and continues
with fresh generated run IDs; a foreground process stop/start resumes the next
generated run number from existing durable receipts. Network polling mode uses
per-entry profile-backed mTLS network queue publication and does not serve
pairing or receiver routes. `repair.drift_recording` is incompatible with
`sync.network_polling` because network polling is a source-side worker-only
mode. `repair.persisted_reconcile_apply` is also incompatible with
`sync.network_polling` for the same reason, and it is mutually exclusive with
`repair.drift_recording` so the daemon cannot implicitly turn live-only
detector findings into automatic apply input. Drift recording may run with the
normal foreground daemon and records review evidence only; it does not apply
reconcile, retry repair, rewrite manifests, authorize prune, or mark targets
restored. Persisted reconcile apply may run with the normal foreground daemon
and consumes only already persisted, currently planned drift records. It does
not call live recording, does not consume live-only detector findings, stops
after any refusal or artifact problem for operator review, and does not rewrite
manifests or authorize prune. These fields do not configure daemon-integrated
OS file watching, detached service management, LAN-discovery-gated sync,
automatic endpoint selection, or broad automatic repair. Use `sync watch` for
the wired foreground OS file watcher surface, `sync network run` or `sync
network loop` for explicit foreground network queue execution, and `sync
network discover-run` when a low-information LAN candidate must match the
profile-selected receiver address before the same network queue pass runs.

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
- invalid `repair.drift_recording` interval or incompatible use with
  `sync.network_polling`
- invalid `repair.persisted_reconcile_apply` interval/reason/reviewer or
  incompatible use with `sync.network_polling` or `repair.drift_recording`
- invalid paired target field shape or partial paired target fields
- invalid network receiver URL or partial TLS identity references
- invalid profile-backed local polling sync settings
- prune mode without review
- negative retention days
- plaintext privacy mode without explicit plaintext restore approval
- traffic level 2 without padding, batching, jitter, and low-information
  discovery
- empty include/exclude patterns
- unnamed agent knowledge categories
