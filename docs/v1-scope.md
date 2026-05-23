# v1 Scope And Non-Goals

Supermover v1 is a conservative one-way migration tool. The product favors
reviewable state, explicit trust, and recoverable operations over broad sync
magic.

Read this document as a scope contract, not a roadmap pitch. If a behavior is
not listed in scope, or is listed only as planned, it should not be described
elsewhere as implemented.

## In Scope

- One-way migration from source to trusted target. Current local push supports
  first migration, idempotent reruns, additions, warning records, soft-delete
  records, managed changed-file updates for previously published regular files,
  read-only local operator reports, and conservative recovery.
- Profile JSON as the configuration single source of truth.
- Target-side `.supermover` control plane. The current local slice stores
  profile snapshots, session receipts, manifests, warnings, soft-delete
  records, previous-manifest replacement evidence, and transaction recovery
  state. Local push can also write target-drift records, but only when it
  refuses a managed changed-file update because the target no longer matches
  trusted previous manifest evidence. `verify`, `health`, and `report` consume
  and validate those records, with unresolved records treated as
  review-required artifacts. The `internal/verify` package now has a read-only
  detector that compares the selected published manifest with
  the target filesystem. `drift list`, `report`, and `status` expose live
  detector evidence as read-only surfaces. `drift record` persists current live
  detector findings as `.supermover/drift/*.json` review records without
  resolving, repairing, pruning, or suppressing future detector output. `drift
  acknowledge` is wired for existing persisted drift records, including records
  created by `drift record`, surfaced as `target_drifts`; `drift resolve` is
  wired for existing persisted drift records after a fresh detector no longer
  reports the same path and expected baseline. Broad drift reconcile/repair,
  drift-to-prune integration, and background scans remain planned.
  History surfaces remain planned. Operator-facing non-dry-run
  `push --network` is wired through profile-backed pinned TLS 1.3 mTLS and
  network transfer outcome artifacts. Current recovery evidence includes a
  bounded same-session CLI/Runner rerun after receiver-accepted payload bytes
  and simulated transport failure. A foreground daemon lifecycle surface is
  wired for install/run/status/stop evidence around existing `serve` behavior.
  LAN browsing, OS service-manager daemon installation, detached background
  process management, ongoing incremental sync, and broad resume acceptance
  remain planned.
- Local push vertical slice that copies supported regular files to a trusted
  local target and writes auditable control artifacts.
- Explicit warning records for recoverable or reviewable gaps.
- Soft-delete record and review flow before any physical target pruning.
- Ordinary file-tree fidelity with supplemental migration records for behavior
  that cannot be represented directly in the target filesystem.
- Agent knowledge files migrated as files and cataloged for downstream tools.
- Wired `serve` command that validates a target profile/root and binds a
  low-information pairing listener for valid pairing-only profiles. When the
  profile is already paired and has complete `network.receiver_url` plus
  `network.local_tls_identity`, `serve` also mounts receiver upload routes on
  the profile-selected mTLS listener. Paired profiles with partial receiver
  material fail closed before any listener reports ready.
- Wired `pair` command skeleton that requires the verification code, writes a
  durable local pairing receipt, updates profile target pins, and records a
  profile snapshot for audit.
- Wired `push --network` source-side transfer. It reads profile, pairing receipt
  evidence, and profile-backed network material (`network.receiver_url` plus
  `network.local_tls_identity`), refuses unpaired or mismatched profiles and
  paired profiles missing that material, and, without `--dry-run`, connects to
  the pinned TLS 1.3 mTLS receiver through `networkpush`, `networkrun`, and
  `protocolclient`. Current operator `push --network` supports traffic level 2
  only. `--dry-run` is preflight-only: it validates profile/pairing/network
  material, local TLS identity files and pins, source scan, and manifest shape
  without contacting the receiver or writing target artifacts. Zero-byte regular
  files are supported for non-dry-run `push --network` through explicit final
  empty completion evidence, with normal receipt and network-transfer evidence
  when the transfer publishes cleanly.
- Source protocol client. It streams regular files to the receiver protocol in
  bounded JSON chunks from existing `scan.Result` evidence, skips reserved
  control-plane/special entries with warnings, resumes from receiver status
  committed offsets, retries commit for staged/published receiver sessions, and
  commits through the receiver protocol. It returns warning records to
  `networkrun`, which persists them as durable target warning artifacts before
  claiming a published network transfer. Receiver-status resume exists for
  same-session network retries, including the bounded automated CLI/Runner gate
  where an accepted-payload run fails with simulated transport failure and a
  same profile/session rerun recovers. Current acceptance also covers receiver
  listener restart over preserved target state, commit-only retry, published
  retry, and fail-closed missing-prior-evidence behavior. Broad resume
  acceptance, network `recover`, receiver-side recovery UX, automatic retry
  policy, daemon/OS-service restart recovery, power-loss recovery, and
  arbitrary process-kill recovery remain planned. The receiver protocol can
  represent zero-byte regular files through explicit completion evidence, and the
  operator-facing profile-backed `push --network` path now sends that evidence
  through `protocolclient`, `networkpush`, and the CLI. This does not complete
  LAN browsing, daemon workflow, ongoing sync, broad resume acceptance, or
  arbitrary process-kill recovery.
- Internal TLS/mTLS transport adapter library. It derives device IDs from leaf
  certificate SPKI SHA-256 hashes, configures TLS 1.3 mTLS with client
  certificate requirements and certificate validity checks, validates peer pins,
  and can wrap the receiver handler with TLS-derived authenticated peer context.
  A profile-bound receiver adapter validates pairing receipt evidence and the
  target certificate pin before exposing the handler. The handler is mounted by
  `serve` only for paired profiles with complete profile-selected network
  material; non-dry-run `push --network` uses the corresponding source-side
  TLS client configuration.
- Low-information explicit-address `discover` adapter. It emits untrusted
  operator-provided address hints, returns no hints on timeout with no source,
  and does not browse LAN services. Address hints are still peer-address
  metadata leakage, not privacy protection.
- Foreground daemon lifecycle commands: `daemon install`, `daemon run
  --foreground`, `daemon status`, `daemon logs`, `daemon restart`, and
  `daemon stop`. They use the profile SSOT, write
  `.supermover/daemon/{install,state,stop-intent,restart-intent}.json` plus
  redacted `.supermover/daemon/events/*.json`, and wrap the same serve
  pairing/receiver behavior. Restart is a foreground intent consumed by a
  running daemon process, not OS process supervision. They do not install an OS
  service manager, spawn a detached process, recover crashes, browse LAN
  services, watch files, or run ongoing sync.
- Local-only read-only `dashboard --profile <path> [--listen <loopback-ip:port>]`.
  It serves a target-side operator page that runs existing `verify` and live
  extra-path detection against the latest published manifest on page load or
  explicit refresh, without digest-reading expected files twice. It refuses
  overlapping full checks and non-loopback listening, and requires its emitted
  per-process access-token URL before serving the active page. It does not
  persist detector output, repair files, execute synchronization, compare a
  post-publish source tree, or provide a Merkle/tree-root digest.
- Planned LAN discovery that exposes low-information address hints only.
- Pairing receipts and pinned device identity as the trust boundary.
- Internal protocol-client bounded traffic metadata reduction now covers
  padding, batching, and bounded jitter at defined policy levels;
  non-dry-run `push --network` uses that protocol-client path for traffic level
  2 only, while dry-run remains preflight-only.
- Recovery classification for interrupted or incomplete local sessions.

## Non-Goals

- Bidirectional sync.
- Automatic trust based on LAN discovery.
- Automatic physical deletion from the target when a source file disappears.
- Automatic physical prune from review evidence. `prune --dry-run` is
  review-only candidate/refusal evidence and never deletes files; `prune
  approve` writes durable approval artifacts plus profile snapshots without
  deleting files or writing prune receipts; `prune approvals` is read-only
  approval inventory; `prune supersede` updates one existing approval artifact
  to durable superseded review metadata without applying prune; `prune review`
  and `report` surface current profile/target approval evidence, prune
  candidates, refusals, existing receipts, and receipt issues as read-only
  evidence; `status` exposes compact prune release counts plus prune review
  status/action for authored-but-unapplied, stale, expired, consumed, and
  receipt-attention states;
  `prune --apply --approval <id>` remains the only physical prune path.
- Runtime flags that silently override profile policy for delete, privacy,
  metadata, or target identity behavior.
- Semantic merging, summarization, embedding, or rewriting of agent memory.
- Strong anonymity or hiding the fact that Supermover is in use.
- Hiding total transfer size, transfer duration, peer IP addresses, or LAN
  presence.
- Encrypted repository mode for untrusted targets.
- Protection from a compromised trusted endpoint.
- Network transport completeness in the local push vertical slice.

## Acceptance Principles

### Warning Auditability

A warning must be durable, machine-readable, and tied to a session. A successful
run with warnings is acceptable only when operators can inspect warning records
under `.supermover/warnings/` and decide whether to accept, rerun, or block.

Warnings must not be console-only. Console summaries may report warning counts,
but the target control plane is the evidence surface.

### Soft-Delete Review

Source-side deletion is not permission to delete the target copy immediately.
The target must first receive a soft-delete record with source path, target
path, profile/target/root scope, previous manifest evidence, detected time, and
reason. Physical pruning is a later action gated by profile policy and operator
review.

For v1, the profile schema defines the prune gate:
`delete_policy.mode: prune` without `require_review: true` and
`allow_physical_prune: true` is invalid, and `allow_physical_prune: true`
outside `mode: prune` is invalid. `delete_policy.retention_days` is enforced as
an elapsed window from soft-delete `detected_at`; active windows are refusal
evidence rather than prune candidates. Current `prune --apply --approval <id>`
additionally requires durable approval evidence, target state checks against
trusted manifest and soft-delete evidence immediately before mutation, a fresh
retention check, and a prune receipt after the attempt. Source absence alone
never authorizes deletion. Approval artifacts live under
`.supermover/prune/approvals/<id>.json`; apply writes receipts under
`.supermover/prune/receipts/<id>.json`. `prune approve` authors approval
artifacts plus profile snapshots from fresh dry-run candidate evidence without
deleting target files or writing prune receipts. `prune approvals` is read-only
approval inventory, and `prune supersede` updates one existing approval
artifact to durable superseded review metadata without applying prune. `report`
and compact `status` read approval artifacts as audit evidence only; `status`
narrows this to compact prune release counts plus prune review status/action.
These read-only surfaces do not apply, supersede, or validate approvals for
mutation, and they do not automatically release a migration or close v1.

Refusal remains the default when policy, approval evidence, target identity,
soft-delete evidence, previous manifest evidence, or current target state does
not match the reviewed plan. Manual target deletion is outside Supermover audit
and must be tracked separately.

### Target-Drift Review

Target drift means target-local divergence from trusted published manifest
evidence. Local push can persist drift when it refuses a managed changed-file
update, and `drift record` can persist current live-detector findings from
published manifest evidence. Dry-run preflight can detect refused-update
conditions but does not mutate the target control plane. Current records include
session/profile/target/root scope, target-relative path, detection time,
`change`, structured expected manifest evidence, structured observed target
state, `review_state`, and evidence strings so review UX can distinguish
unreviewed, acknowledged, and resolved records without implying automatic
reconciliation.

Unresolved persisted drift records surfaced by `verify`, `health`, and the
report `target_drifts` view are review-required. Acknowledged records remain
review-required metadata; valid resolved records are excluded from
review-required counts. `drift list --profile
<path> [--session <id>] [--format text|json]` is read-only and derives the
target from the profile only. It classifies missing, changed, type-mismatched,
and extra target-local paths from published manifest evidence; default
extra-path scans compare against the latest published manifest. Explicit
latest-session review also scans extras; explicit historical session review
stays bounded to that session's manifest entries so later published content is
not treated as historical drift. `report --profile` exposes the same live
detector under an independent report surface, with JSON `live_target_drift` and
summary counters such as `live_target_drifts` and
`live_target_drift_artifact_problems`. `drift list` and `report` both return
review-required non-zero for live drift or artifact problems and do not persist
their live detector output. An unscoped report with no published manifest is a
generated empty-target report. An explicit `report --session <id>` can still
produce a structured report when scoped persisted drift or artifact evidence
exists without a selected manifest; an explicit missing-session request with no
scoped report evidence remains a stderr-only selection error.

`drift record --profile <path> [--session <id>] [--format text|json]` runs the
same live detector and writes current findings as durable `.supermover/drift`
review records with `review_state=needs_review`. Existing matching records are
reported as existing and keep prior review metadata. It records evidence only:
there is no current background target scan, and `drift record` does not
resolve drift, repair drift, prune drifted paths, reconcile manifests, suppress
future detector findings, or edit live detector observations through the
read-only `drift list`, `report`, or `status` surfaces.

`drift acknowledge --profile <path> --id <persisted-drift-id> --reason <text>
[--reviewer <id>] [--format text|json]` is wired as the narrow persisted drift
review-state mutation. It derives target scope from the profile, accepts only
an existing `.supermover/drift/<id>.json` record that remains visible through
published profile/session evidence, and writes acknowledgement review metadata.
The ID should come from persisted `target_drifts` in `verify --format json`,
`report --format json`, or `drift record` output, not directly from live-only
`drift list`, `report.live_target_drift`, or `status` detector output.
Acknowledgement does not repair files, resolve/reconcile the record, rewrite
manifests, suppress future detector findings, resume refused updates, authorize
prune, or mark the target clean.

`drift resolve --profile <path> --id <persisted-drift-id> --reason <text>
[--reviewer <id>] [--format text|json]` is wired as the narrow persisted drift
closeout path. It derives target scope from the profile, accepts only an
existing `.supermover/drift/<id>.json` record that remains visible through
published profile/session evidence, and reruns the live detector for the
record's published baseline. It writes `review_state=resolved` plus review
metadata only when the same target path and expected baseline no longer reports
drift. Resolved persisted records no longer make `verify`, `health`, `report`,
or `status` review-required. Resolve does not repair files, rewrite manifests,
suppress future detector findings, resume refused updates, authorize prune, or
provide broad reconcile.

### Compact Local Status

`status` is wired as `supermover status --profile <path> [--format text|json]`.
The surface has no `--session` flag, derives current local target state from the
profile SSOT, target `.supermover` artifacts, and target files needed for
verification/live drift detection, and is read-only. It reuses report, health,
verify, and drift evidence. It does not include foreground daemon lifecycle
state; use `daemon status` for `.supermover/daemon` install/state/stop-intent
evidence. It does not imply LAN, encrypted transport, or long-running sync
status; pairing and network fields are evidence only.
Session-scoped historical review remains
`report --profile <path> --session <id>`.

### Profile SSOT

Profiles are the source of truth for migration behavior. Commands may select a
profile and may create or lint a profile, but policy changes belong in the
profile itself. Each successful run must preserve the profile snapshot used for
that run so target state can be audited later.

`profile lint` is a schema/safety gate. Implementation readiness still requires
the operational gates for the selected slice, such as `push --dry-run`, `verify`,
`health`, and `report`.

### Discovery Is Not Trust

Current `serve` validates the target profile/root and, for valid pairing-only
profiles, binds a low-information pairing listener. It returns low-information discovery
responses, prints a verification code on the target console, and returns pairing
bootstrap only after that code is presented. When the profile is already paired
and includes complete profile-selected receiver URL plus local TLS identity
material, `serve` also binds that receiver URL and mounts authenticated receiver
routes over pinned mutual TLS. Paired partial receiver material fails closed
before any listener reports ready. It does not browse or advertise on LAN.
Current `pair` requires the verification code before writing a local
receipt/profile pins.
`discover` has a low-information explicit-address adapter; with no configured
source it waits for the requested timeout and returns no untrusted hints.

Planned LAN browsing still remains unwired. Discovery returns unauthenticated
address hints only. Discovery advertisements must remain sparse and must not
disclose identity, local layout, profile data, or inventory size.

Pairing evidence starts only after explicit pairing verification writes a
receipt and pins device identity. A discovered endpoint without pairing is not a
migration target. A paired endpoint becomes an authenticated transfer channel
only when non-dry-run `push --network` validates the pinned TLS peer through the
profile-selected receiver URL and local TLS identity.

## Current Slice Versus Planned Surface

Implemented local slice:

```bash
go run ./cmd/supermover profile init --profile <path> --source <path> --target <path>
go run ./cmd/supermover profile lint --profile <path>
go run ./cmd/supermover profile set-target --profile <path> --target <path>
go run ./cmd/supermover scan --profile <path>
go run ./cmd/supermover push --profile <path> --dry-run
go run ./cmd/supermover push --profile <path> --session <session-id>
go run ./cmd/supermover verify --profile <path> --session <session-id>
go run ./cmd/supermover deleted list --profile <path>
go run ./cmd/supermover health --profile <path>
go run ./cmd/supermover drift list --profile <path>
go run ./cmd/supermover drift list --profile <path> --session <session-id>
go run ./cmd/supermover drift record --profile <path> [--session <session-id>]
go run ./cmd/supermover drift acknowledge --profile <path> --id <persisted-drift-id> --reason <text>
go run ./cmd/supermover drift resolve --profile <path> --id <persisted-drift-id> --reason <text>
go run ./cmd/supermover report --profile <path>
go run ./cmd/supermover report --profile <path> --session <session-id>
go run ./cmd/supermover status --profile <path> [--format text|json]
go run ./cmd/supermover prune --help
go run ./cmd/supermover prune --profile <path> --dry-run
go run ./cmd/supermover prune approvals --profile <path> [--format text|json]
go run ./cmd/supermover prune approve --profile <path> --id <approval-id> --soft-delete <soft-delete-id> --reason <text> --reviewer <reviewer-id>
go run ./cmd/supermover prune review --profile <path> [--session <session-id>] [--format text|json]
go run ./cmd/supermover prune supersede --profile <path> --id <approval-id> --reason <text> --reviewer <reviewer-id> [--format text|json]
go run ./cmd/supermover prune --profile <path> --apply --approval <approval-id>
go run ./cmd/supermover recover --profile <path> --session <session-id>
go run ./cmd/supermover daemon install --profile <target-profile>
go run ./cmd/supermover daemon run --foreground --profile <target-profile>
go run ./cmd/supermover daemon status --profile <target-profile> [--format text|json]
go run ./cmd/supermover daemon logs --profile <target-profile> [--tail <n>] [--format text|json]
go run ./cmd/supermover daemon restart --profile <target-profile> [--reason <text>]
go run ./cmd/supermover daemon stop --profile <target-profile> [--reason <text>]
```

Wired network trust skeleton:

```bash
go run ./cmd/supermover serve --profile <target-profile>
go run ./cmd/supermover discover
go run ./cmd/supermover pair --profile <path> --target <address> --verification-code <code>
go run ./cmd/supermover push --network --profile <path> --dry-run
go run ./cmd/supermover push --network --profile <path> --session <session-id>
```

These commands do not yet advertise on LAN. `serve` exposes low-information
discovery, gates pairing bootstrap behind the operator verification code, and
mounts receiver upload routes only on a paired profile's profile-selected mTLS
receiver listener. `push --network --dry-run` validates
profile/pairing/network-material evidence, local TLS identity files and pins,
source scan, and manifest shape, and writes no target artifacts;
non-dry-run `push --network` transfers over the profile-selected pinned mTLS
receiver and records network transfer evidence after receiver begin stores a
session.

The current operator CLI exposes `prune --dry-run` for profile-policy
validation and non-mutating candidate/refusal evidence over published
soft-delete records, including `retention_window_active` refusals while
retention is active. It exposes `prune approve` for durable approval authoring
from fresh dry-run candidates, `prune approvals` for read-only approval
inventory, `prune supersede` for durable approval supersede metadata, and
conservative physical prune apply through `prune --apply --approval <id>` when
the approval artifact exists under the target control plane. Apply re-runs the
prune plan before deletion, so retention-active approved items produce refusal
receipts instead of deletion. `prune review` and `report` surface prune
candidates, refusals, current-scope approval evidence, existing receipts, and
receipt issues as read-only audit evidence, while `status` exposes compact
prune release counts plus prune review status/action.
