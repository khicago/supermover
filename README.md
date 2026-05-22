# Supermover

Supermover is a Go CLI for one-way, auditable file migration from a source
machine to a trusted target. The product direction includes ongoing
incremental synchronization, but the current implementation is intentionally
conservative: profile-driven, target-auditable, and narrower than a broad sync
daemon.

> Current status
>
> - Implemented today: local publish/verify/recover, prune approval workflow,
>   durable drift record/acknowledge/resolve, narrow persisted-drift
>   reconcile, paired profile-backed network push, foreground daemon lifecycle,
>   and a loopback-only read-only dashboard.
> - Not implemented today: LAN browsing, OS-managed background service
>   installation, broad arbitrary interruption recovery, broad automatic
>   repair/reconcile, ongoing sync, and anonymity.

## What You Can Do Today

- Define migration intent in a profile and treat that profile as the SSOT.
- Run first migration, idempotent reruns, additions, and managed changed-file
  updates for previously published regular files.
- Verify target state with `verify`, `report`, `status`, and a local-only
  `dashboard`.
- Review source-side deletions through `prune --dry-run`, author approval
  artifacts, inspect approval inventory, supersede old approvals, and physically
  prune only through `prune --apply --approval <id>`.
- Persist, acknowledge, resolve, and narrowly reconcile drift records.
- Pair a source and target, run `serve`, and execute profile-backed
  `push --network`.
- Run the current foreground daemon lifecycle surface as a supervisor-friendly
  wrapper over `serve`.

## Product Boundaries

- One-way `source -> trusted target`, not bidirectional sync.
- Profile files are the configuration SSOT; there are no runtime policy or
  target-identity overrides that bypass the audit trail.
- Audit evidence lives under target-side `.supermover` control-plane artifacts.
- Network transfer is profile-backed TLS 1.3 mTLS, not anonymity. Residual
  leakage still includes total bytes, duration, peer IPs, LAN presence, and
  Supermover use.
- A new network session is conservative: the receiver refuses already-divergent
  target files, symlinks, or incompatible directories at begin, before payload
  upload. This is fail-fast conflict rejection, not changed-file network sync.
- The loopback-only `dashboard` verifies against the latest published manifest
  snapshot, not against post-publish source changes. It does not expose a
  Merkle/tree-root digest or execute synchronization.
- `daemon` is a foreground lifecycle surface only. It does not install
  launchd/systemd/Windows services, spawn a detached process, browse LAN
  services, watch files, or run ongoing sync.

## Current Workflows

### Local Publish

The local slice supports first migration, idempotent reruns, additions, managed
changed-file updates for previously published regular files, warning records,
soft-delete records, read-only operator reports, and conservative recovery.

Use an empty target directory for first migration. Current publish code refuses
to overwrite unrelated existing target files or symlinks. Changed regular files
are replaced only when the latest published manifest for the same
profile/target/root proves Supermover published the previous target content and
the target still matches that previous SHA-256, size, mode, and modification
time evidence. Concurrent external writes to the same target path are outside
the current safety contract.

### Dashboard

`dashboard --profile <path>` serves a target-side read-only HTML view on a
loopback-only listener. The page runs `verify` plus live detection of target
paths outside the selected manifest once when opened and on explicit refresh.
It refuses overlapping full-check requests and avoids re-reading declared file
content twice in the same integrity pass.

Open only the emitted access-token URL. Use SSH port forwarding rather than
exposing the page on a LAN interface.

### Pairing, Discover, and Serve

`serve` validates a target profile and, for valid pairing-only profiles, binds a
low-information pairing listener that prints an operator verification code and
returns pairing bootstrap material only after that code is presented.

When the profile is already paired and has complete `network.receiver_url` plus
`network.local_tls_identity` material, `serve` also binds the receiver endpoint
from the profile and exposes upload routes over pinned mutual TLS. With no
receiver material, `serve` stays pairing-only. Once a paired profile has any
receiver material, `serve` refuses to start until the network receiver material
is complete and auditable.

`pair` requires the verification code before it writes a durable pairing
receipt under the target control plane and pins target device identity in the
profile. `discover` emits untrusted explicit address hints only; it does not
browse LAN services or transfer files.

### Network Transfer

Profiles now have a network SSOT shape for the operator network path:
`network.receiver_url` and `network.local_tls_identity` name the
profile-selected receiver endpoint and local certificate/key references.

`push --network --dry-run` is preflight-only: it validates profile, pairing,
profile network material, local TLS identity files and pins, scan, and manifest
shape without contacting the receiver, writing target artifacts, or copying
files.

Non-dry-run `push --network` uses the profile material plus pairing receipt
pins to connect to the profile-selected TLS 1.3 mTLS receiver, stream files
through `networkpush`/`networkrun`/`protocolclient`, and write receiver-side
network transfer outcome evidence after receiver begin creates a session.

The current recovery evidence is bounded, not broad. Same-profile, same-session
reruns can recover from authenticated receiver status only when prior
payload-overhead evidence remains auditable. Current acceptance evidence covers:

- receiver listener restart over the same profile-selected target control plane
- published-session retry that uploads no chunks
- fail-closed missing-prior-evidence handling with `payload_overhead_missing`
- deterministic `networkrun` source-stop-after-progress resume

This does not make `recover` a network recovery command and does not complete
LAN browsing, daemon sync, broad resume acceptance, arbitrary process-kill
recovery, OS crash recovery, or anonymity.

### Prune Review and Apply

`prune --dry-run` validates the profile prune policy, reads published
soft-delete records, and emits review-only candidates, refusals, and artifact
problems without deleting target files or writing prune approvals. Active
`delete_policy.retention_days` windows remain visible as
`retention_window_active` refusals rather than approval candidates.

`prune approve --profile <path> --id <approval-id> --soft-delete <id>
[--soft-delete <id>...] --reason <text> --reviewer <id>` authors a durable
approval artifact under `.supermover/prune/approvals/<id>.json` from fresh
dry-run candidate evidence. `--approved-by` is an alias for `--reviewer`,
`--expires-at <RFC3339>` is optional, and `--format text|json` is supported.
It does not delete target files or write prune receipts.

`prune approvals --profile <path>` lists current-scope approval artifacts
without mutating them. `prune supersede --profile <path> --id <approval-id>
--reason <text> --reviewer <id>` updates one existing approval artifact to a
durable `superseded` review state without deleting target files or writing
prune receipts.

`prune --apply --approval <id>` remains the only physical prune path. It writes
a started prune receipt before target mutation, re-runs the current prune plan,
rechecks target evidence, and records final `applied` / `partial` / `failed`
status in the same receipt path. If finalization is interrupted, the durable
`started` receipt remains review evidence.

`prune review --profile <path>` is the focused read-only release-review surface
over current prune candidates, approval inventory, and receipts. `report` is
also read-only and surfaces current profile/target prune approval evidence from
durable `.supermover/prune/approvals/*.json` artifacts, while `status` exposes
compact prune release counts, prune review status/action, and artifact-problem
source breakdown.

### Drift and Narrow Reconcile

`drift list` is read-only. It compares published manifest evidence to the
target filesystem and exits non-zero when drift, artifact problems, or no
published manifest require review.

`drift record` persists current live detector findings as durable
`.supermover/drift/<id>.json` review records. It records evidence only: it does
not resolve, repair, prune, suppress future detector output, or run background
scans.

`drift acknowledge` and `drift resolve` operate only on existing persisted
drift records. `drift resolve` closes a record only after a fresh
profile-scoped live detector no longer reports drift for the same path and
expected baseline.

`reconcile plan/apply` is a separate narrow persisted-drift repair slice.
`plan` is non-mutating. `apply` requires selected persisted drift IDs, explicit
`--apply`, and `--reason`. It derives source and target only from the profile
SSOT, has no `--target` or `--state-dir` override, and currently handles only:

- missing regular-file restores from published manifest plus current source
  evidence
- resolve-noop cases where the target is already restored
- resolve-noop cases where an expected-absent target path is already absent

Broad automatic reconcile, durable repair receipts, conflict-class taxonomy
beyond current refusals, retry policy, background scans, live-only repair,
manifest rewrite, daemon sync, and ongoing sync remain planned.

### Daemon

`daemon install`, `daemon run --foreground`, `daemon status`, `daemon logs`,
`daemon restart`, and `daemon stop` persist lifecycle evidence under
`.supermover/daemon` and wrap the same profile-backed `serve` behavior.

The current daemon slice is a foreground/supervisor-friendly lifecycle surface
with durable status, redacted lifecycle events, stop intent, and restart
intent. Restart is consumed only by a running foreground daemon and restarts
serve listeners in that same process.

## Quickstart

```bash
go run ./cmd/supermover profile init --profile ./supermover.profile.json --source /path/to/source --target /path/to/empty-target
go run ./cmd/supermover profile lint --profile ./supermover.profile.json
go run ./cmd/supermover push --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover push --profile ./supermover.profile.json --session session-001
go run ./cmd/supermover verify --profile ./supermover.profile.json --session session-001
go run ./cmd/supermover dashboard --profile ./supermover.profile.json
go run ./cmd/supermover deleted list --profile ./supermover.profile.json
go run ./cmd/supermover health --profile ./supermover.profile.json
go run ./cmd/supermover drift list --profile ./supermover.profile.json
go run ./cmd/supermover drift record --profile ./supermover.profile.json
go run ./cmd/supermover reconcile plan --profile ./supermover.profile.json --id <persisted-drift-id>
go run ./cmd/supermover report --profile ./supermover.profile.json
go run ./cmd/supermover status --profile ./supermover.profile.json
go run ./cmd/supermover recover --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover daemon install --profile ./supermover.profile.json
go run ./cmd/supermover daemon status --profile ./supermover.profile.json
```

### Drift Follow-Up

If `verify --format json` or `report --format json` shows persisted
`target_drifts`, or `drift record --format json` returns `records[].id`,
acknowledge a reviewed persisted record separately:

```bash
go run ./cmd/supermover drift acknowledge --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<operator review reason>"
```

After the target has been restored so the live detector no longer reports that
same persisted path and expected baseline, close the persisted record
separately:

```bash
go run ./cmd/supermover drift resolve --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<target restored reason>"
```

For the narrow missing-file repair slice, inspect a non-mutating reconcile plan
for selected persisted drift evidence before applying:

```bash
go run ./cmd/supermover reconcile plan --profile ./supermover.profile.json --id <persisted-drift-id> --format json
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --id <persisted-drift-id> --apply --reason "<operator repair reason>"
```

Use this only for persisted drift IDs, not live-only detector IDs. Apply can
restore a missing regular file only when the published manifest evidence and
current source file still match; it can also resolve already-restored or
already-absent persisted records without additional target content mutation.
It is not a broad reconcile daemon and it does not rewrite manifests.

### Publish Safety

Use an empty target directory for first migration. Current publish code refuses
to overwrite an unrelated existing target file or symlink. Reruns are
idempotent when the existing object is content-identical. Changed regular files
are replaced only when the latest published manifest for the same
profile/target/root proves Supermover published the previous target content and
the target still matches that previous SHA-256, size, mode, and modification
time evidence. If that previous target file is missing or was edited outside
Supermover, the run refuses the update and leaves recovery to the operator. The
local target is expected to be under Supermover control during a run; concurrent
external writes to the same file are outside the current safety contract.
Managed changed-file publish first creates session-scoped replacement holds
under `.supermover/replacement-holds/<session>/previous/...` and
`.supermover/replacement-holds/<session>/current/...`, removes the previous
target only after rechecking previous evidence, and publishes the staged
replacement with no-replace semantics. Recovery can complete a held replacement
only when the target is absent and both holds still match previous evidence;
divergent target/hold state is marked `needs_repair`.

### Review Surfaces

`push --dry-run` reports counts only; full warning JSON is written after a
published run. Source scanner `scan_error` findings block push instead of being
published as review warnings. `verify` checks published regular files for
size, SHA-256 digest, permissions, and modification time, and checks directory
and symlink entries for presence/type/target fidelity. It exits non-zero for
error findings, warning findings, artifact problems, or a missing manifest.
`report` is read-only and aggregates the profile SSOT plus target
`.supermover` artifacts into an operator view of warnings, profile suggestions,
soft deletes, prune candidates/refusals, current-scope approval evidence,
receipts, health/recovery issues, artifact problems, pairing evidence state,
verification state, and the live target drift detector state at report time.
JSON reports expose this detector
under `live_target_drift`, with summary
counters such as `live_target_drifts` and
`live_target_drift_artifact_problems`; text reports include the same
review-required evidence. It returns non-zero when the report requires operator
review, even if the report itself was generated successfully. Live report drift
is not persisted by `report`, acknowledged, resolved, repaired, pruned, or
treated as compact status; use `drift record` when the operator intentionally
wants to materialize current live detector findings as durable
`.supermover/drift` review records.
`status` is a compact read-only current profile/target view. It derives the
target only from the profile, reads target control-plane artifacts plus target
files needed for verification and live drift detection, and returns `0` for a
clean local target, `1` when review-required evidence is emitted, and `2` when
no status report can be generated. It has no `--session`, `--target`, policy, or
network override; use `report --session` for historical session-scoped review.
Prune approval evidence in `status` is limited to current profile/target counts
and source breakdown from `.supermover/prune/approvals/*.json`; it does not list
full approval artifacts and is not prune authorization or target cleanup.

### Drift Commands

`drift list` is also read-only. It derives the target from the profile only,
compares published manifest evidence to the target filesystem, supports
`--session <id>` and `--format text|json`, and exits non-zero when drift,
artifact problems, or no published manifest require review. Use JSON output for
automation and durable audit capture; text output is compact operator review
output with target-controlled values escaped.
`drift record` derives the target from the profile only, runs the same detector,
and writes current detected findings as durable `.supermover/drift` review
records. It records evidence only: it does not resolve, repair, prune, suppress
future detector output, or run background scans.
`drift acknowledge` is the current narrow drift review-state mutation. It
derives the target from the profile, accepts only an existing persisted
`.supermover/drift/<id>.json` record, requires `--reason`, writes
`review_state=acknowledged` plus review time/reviewer/reason metadata, and
refuses live-only detector IDs from `drift list`, missing records, foreign
profile/target/root scope, missing published receipt/manifest evidence, and
already acknowledged or resolved records. Acknowledgement is review metadata
only: it does not repair target files, rewrite manifests, suppress live
detector drift, resolve records, authorize prune, or make a review-required
target clean.
`drift resolve` is the narrow persisted drift closeout path. It derives the
target from the profile, accepts only an existing persisted
`.supermover/drift/<id>.json` record, requires `--reason`, rechecks profile,
published session, manifest, root, and artifact-boundary evidence, then runs a
fresh live detector. It writes `review_state=resolved` plus review
time/reviewer/reason metadata only when the same target path and expected
baseline no longer reports drift. Valid resolved persisted records no longer
make `verify`, `health`, `report`, or `status` review-required, but live
detector output remains read-only review evidence. Resolve does not repair
target files, rewrite manifests, authorize prune, suppress future detector
findings, or perform broad reconcile.

### Narrow Reconcile

`reconcile plan/apply` is narrower than the roadmap reconcile feature. It works
only from persisted `.supermover/drift/<id>.json` evidence selected through the
profile, keeps `plan` non-mutating, and requires selected `--id` values plus
explicit `--apply` and `--reason` before `apply` can mutate target content or
mark a record resolved. Current apply support is limited to missing regular-file
restores from matching published manifest and current source evidence, plus
resolve-noop when the target already matches the expected missing-file evidence
or an expected-absent path is already absent. It does not consume live-only
drift IDs, perform broad automatic repair, write durable repair-receipt
artifacts, rewrite manifests, run background scans, or participate in daemon or
ongoing sync.

## Roadmap Truth

The current implementation is intentionally narrower than the product
direction. Still planned:

- LAN browsing beyond explicit address hints
- OS-managed or detached daemon lifecycle and ongoing incremental sync
- broad arbitrary interruption recovery, broad resume acceptance, and network
  `recover`
- broad automatic repair/reconcile, repair receipts, background scans, and
  richer drift-to-prune integration
- broader operator-facing traffic privacy acceptance beyond the current
  profile-backed level 2 evidence path

The current non-claims matter just as much:

- `discover` is not trust and not LAN browsing
- traffic privacy level 2 is not anonymity
- `dashboard` is read-only target verification against the latest published
  snapshot, not post-publish source comparison or synchronization
- `status` is compact review evidence, not release authorization, daemon
  supervision, or network-sync status

The tracked implementation outline lives in [docs/plan.md](docs/plan.md).
Local Bagakit planning and research artifacts may also exist under `.bagakit/`,
but that directory is intentionally ignored by Git.

## Operator Docs

- [User migration guide](docs/user-migration-guide.md): current local push
  workflow, audit artifacts, and post-run checks.
- [Operations runbook](docs/runbook.md): repeatable dry-run, publish, review,
  recovery, and incident procedures.
- [Troubleshooting matrix](docs/troubleshooting.md): symptoms, likely causes,
  evidence to collect, and safe actions.
- [Compact status contract](docs/status.md): local-only profile/target status
  output fields, exit codes, and boundaries.
- [v1 scope and non-goals](docs/v1-scope.md): product boundaries, including
  warning auditability, soft-delete review, profile SSOT, and discovery trust.
- [Release audit](docs/release-audit.md): tracked checkpoint status, validation
  gates, known planned surface, and safety notes.

## Development

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
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full contribution and validation
policy.
