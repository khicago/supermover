# Supermover

Supermover is a Go CLI for one-way, auditable file migration. The long-term
design includes ongoing incremental synchronization from a source machine to a
trusted target machine, but the current implementation is not a broad
incremental sync system or OS-managed background service.

The current implementation is a local push vertical slice. Available commands
are `profile`, `scan`, `push`, `verify`, `deleted list`, `prune`, `health`,
`drift list`, `drift record`, `drift acknowledge`, `drift resolve`,
`reconcile plan`, `reconcile apply`, `report`, `status`, `recover`, pairing
`serve`/`pair` surfaces, the profile-backed `serve` receiver surface,
foreground `daemon` lifecycle state, and a low-information
explicit-address `discover` adapter. The
local slice supports first migration, idempotent reruns, additions, managed
changed-file updates for previously published regular files, warning records,
soft-delete records, read-only operator reports, and conservative recovery.
`serve` validates a target profile and, for valid pairing-only profiles, binds a
low-information pairing HTTP listener that prints an operator verification code
and returns pairing bootstrap material only after that code is presented. When
the profile is already paired and has complete `network.receiver_url` plus
`network.local_tls_identity` material, `serve` also binds the receiver endpoint
from the profile and exposes upload routes over pinned mutual TLS. With no
receiver material, `serve` stays pairing-only. Once a paired profile has any
receiver material, `serve` refuses to start until the network receiver material
is complete and auditable. `pair` requires the verification code before it
writes a durable pairing receipt under the profile's target control plane and
pins target device identity in the profile. `discover` can emit untrusted
explicit address hints with `--address` and returns no results on timeout when
no source is configured.
An explicit address is operator-provided hint material and still reveals peer
address metadata; it does not browse LAN services or transfer files.
`daemon install`, `daemon run --foreground`, `daemon status`, `daemon logs`,
`daemon restart`, and `daemon stop` persist lifecycle evidence under the target
`.supermover/daemon` control-plane directory and wrap the same profile-backed
serve behavior. The current daemon slice is a foreground/supervisor-friendly
lifecycle surface with durable status, redacted lifecycle events, stop intent,
and restart intent; restart is consumed only by a running foreground daemon and
restarts serve listeners in that same process. It does not install
launchd/systemd/Windows services, spawn a detached process, provide crash
supervision, browse LAN services, watch files, or run ongoing sync.
Profiles now have a network SSOT shape for the operator network path:
`network.receiver_url` and `network.local_tls_identity` name the
profile-selected receiver endpoint and local certificate/key references. A
non-dry-run `push --network` uses that profile material with pairing receipt
pins to connect to the profile-selected TLS 1.3 mTLS receiver, stream files
through `networkpush`/`networkrun`/`protocolclient`, and write receiver-side
network transfer outcome evidence after receiver begin creates a session.
`push --network --dry-run` is preflight-only: it validates profile, pairing,
profile network material, local TLS identity files and pins, scan, and manifest
shape without contacting the receiver, writing target artifacts, or copying
files.
The `prune --dry-run` surface validates the profile prune policy, reads
published soft-delete records, and emits review-only candidates, refusals, and
artifact problems without deleting target files or writing prune approvals.
Active `delete_policy.retention_days` windows remain review-visible as
`retention_window_active` refusals rather than approval candidates.
`prune approve --profile <path> --id <approval-id> --soft-delete <id>
[--soft-delete <id>...] --reason <text> --reviewer <id>` authors a durable
approval artifact under `.supermover/prune/approvals/<id>.json` from fresh
dry-run candidate evidence, accepts `--approved-by` as an alias for
`--reviewer`, can set `--expires-at <RFC3339>`, and supports
`--format text|json`; it does not delete target files or write prune receipts,
and the fresh dry-run must be free of refusals or artifact problems before any
approval is written.
`prune --apply --approval <id>` remains the only physical prune path: it writes
a started prune receipt before target mutation, re-runs the current prune plan,
rechecks target evidence, and then records the final
applied/partial/failed status in the same receipt path when finalization
succeeds. If finalization is interrupted, the durable `started` receipt remains
review evidence. `prune review --profile <path>` is a focused read-only release
review surface over current prune candidates, approval inventory, and receipts.
`report` is also read-only and surfaces current profile/target prune approval
evidence from durable `.supermover/prune/approvals/*.json` artifacts, while
`status` exposes only compact approval counts and source breakdown. That
evidence helps review authored-but-unapplied approvals; it does not author
approvals, supersede approvals, apply prune decisions, write receipts, delete
files or symlinks, repair or reconcile drift, make the target clean,
automatically release a migration, or close v1. Broader recovery reconciliation,
LAN browsing, OS service-manager daemon installation, detached background
daemon process management, ongoing incremental sync, broad network resume
acceptance, broad automatic repair/reconcile, and broader prune release
workflow remain planned or unimplemented; anonymity is not claimed.
`drift record` can persist current
live detector findings as durable `.supermover/drift/<id>.json` review records,
`drift acknowledge` is wired only for existing persisted drift records with
operator reason evidence, and `drift resolve` can close an existing persisted
drift record only after a fresh profile-scoped live detector no longer reports
the same path and expected baseline.
`reconcile plan/apply` is a separate narrow persisted-drift repair slice:
`plan` is non-mutating, and `apply` requires selected persisted drift IDs,
explicit `--apply`, and `--reason`. It derives source and target only from the
profile SSOT, has no `--target` or `--state-dir` override, and currently
handles only missing regular-file restores from published manifest plus current
source evidence and resolve-noop cases where the target is already restored or
already absent. Broad automatic reconcile, durable repair receipts,
conflict-class taxonomy beyond current refusals, retry policy, background
scans, live-only repair, manifest rewrite, daemon sync, and ongoing sync remain
planned.
Profile-backed encrypted transfer is wired for
non-dry-run `push --network`, including zero-byte regular files through an
explicit final empty completion path in the protocol client/network runner/CLI.
The bounded source-interruption evidence is narrower than broad recovery:
during a profile-backed non-dry-run `push --network` attempt, after receiver
begin and accepted payload bytes, Supermover may persist in-flight
`network-transfer.json` evidence for a same-profile, same-session rerun. The
rerun can recover from authenticated receiver status only when the prior
payload-overhead evidence remains auditable. Current acceptance evidence also
covers a receiver listener restart over the same profile-selected target
control plane, a published-session retry that uploads no chunks, and a
missing-prior-evidence path that fails closed with `payload_overhead_missing`
instead of fabricating recovery. Internal `networkrun` acceptance covers a
deterministic source stop immediately after durable in-flight chunk progress
evidence, followed by same-session receiver-status resume and merged
privacy-overhead evidence. That support does not make `recover` a network
recovery command, and it does not complete LAN browsing, daemon behavior,
ongoing sync, broad arbitrary interruption recovery, broad resume acceptance,
arbitrary process-kill recovery, OS crash recovery, or anonymity.

## Quickstart

```bash
go run ./cmd/supermover profile init --profile ./supermover.profile.json --source /path/to/source --target /path/to/empty-target
go run ./cmd/supermover profile lint --profile ./supermover.profile.json
go run ./cmd/supermover push --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover push --profile ./supermover.profile.json --session session-001
go run ./cmd/supermover verify --profile ./supermover.profile.json --session session-001
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

The v1 direction is intentionally conservative:

- one-way `source -> trusted target`
- profile files as the configuration SSOT
- `.supermover` control-plane artifacts for receipts, manifests, warnings,
  previous-manifest evidence, soft deletes, target-drift refusal records,
  recovery, and network transfer outcomes from non-dry-run `push --network`
  attempts that reach the network runner; `report` and `drift list` can run the
  live detector without persisting their observations, `drift record` can
  persist current live detector findings as `.supermover/drift` review records,
  and `status` exposes
  compact current profile/target evidence without persisting live detector
  output; `report` also exposes prune candidates, refusals, current-scope
  approval evidence, existing receipts, and receipt issues as read-only review
  evidence. `prune review` exposes the same prune review evidence as a focused
  read-only release-review surface. `status` narrows that approval surface to
  counts and source breakdown. Approval evidence is scoped to the current
  profile/target and helps review authored-but-unapplied approvals only; it does not author
  approvals, supersede approvals, apply prune decisions, write receipts, delete
  files or symlinks, repair/reconcile drift, make the target clean,
  automatically release a migration, or close v1.
  `drift acknowledge` can add
  operator review evidence to existing persisted drift records only, and
  `drift resolve` can close existing persisted drift records only after a fresh
  detector no longer reports the same path and expected baseline.
  `reconcile plan/apply` adds a narrow selected-ID persisted drift repair path
  for missing regular-file restores and already-restored/absent resolve-noop
  cases only, while history writers, broad drift reconcile/repair, repair
  receipts, retry policy, background scans, drift-to-prune integration, and
  broader release workflow surfaces remain planned; wired
  `prune --dry-run` is a non-mutating soft-delete candidate/refusal review
  surface with retention-window refusals, `prune review` is a focused read-only
  prune release-review surface, `prune approve` authors durable approval
  artifacts only for current dry-run candidates without deleting target files or
  writing receipts, and `prune --apply --approval <id>` is the
  conservative physical prune apply path that rechecks retention and target
  state before deletion
- low-information explicit address discovery hints plus verification-code
  pairing that writes local receipts/profile pins; discovery is not trust, LAN
  browsing remains planned, and `serve` can mount authenticated receiver upload
  routes only from paired profile network material
- profile-backed `push --network` transfer over pinned TLS 1.3 mTLS for
  non-dry-run attempts, including zero-byte regular files through explicit
  final empty completion evidence; dry-run remains preflight-only and
  non-mutating
- bounded same-session `push --network` source-interruption evidence after
  accepted payload bytes, persisted in-flight network-transfer evidence, and a
  same-profile/same-session rerun, plus receiver-listener restart over
  preserved target state, published-session retry, missing-prior-evidence
  fail-closed behavior, and deterministic `networkrun`
  source-stop-after-progress resume evidence; broad arbitrary interruption
  recovery, network `recover`, broad resume acceptance, OS-managed daemon
  restart recovery, automatic crash restart, arbitrary process-kill recovery,
  and power-loss recovery remain planned or unwired
- traffic privacy level 2 overhead evidence from bounded padding, batching,
  and timing jitter on the protocol-client network path; this is not anonymity
  and does not hide total bytes, transfer duration, peer IP addresses, LAN
  presence, or Supermover use
- ordinary file-tree fidelity with auditable supplemental migration records
- agent knowledge files migrated as files and cataloged without semantic
  rewriting

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
