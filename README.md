# Supermover

Supermover is a Go CLI for one-way, auditable file migration from a source
machine to a trusted target. The long-term direction includes LAN-assisted
transfer and ongoing sync, but the current product is narrower on purpose:
profile-driven, target-auditable, and conservative about mutation.

The shortest honest summary is this: Supermover already has a usable local
migration path, a bounded profile-backed network path, and a growing review
surface. It does not yet have LAN browsing, a detached daemon, broad automatic
repair, ongoing sync, or anonymity.

## What Works Now

Today you can:

- define migration intent in a profile and treat that profile as the SSOT
- run first migration, idempotent reruns, additions, and managed changed-file
  updates for previously published regular files
- verify target state with `verify`, `report`, `status`, and the local-only
  `dashboard`
- review source-side deletions through `prune --dry-run`, author approval
  artifacts, inspect approval inventory, supersede old approvals, and physically
  prune only through `prune --apply --approval <id>`
- persist, acknowledge, resolve, and narrowly reconcile drift records
- pair a source and target, run `serve`, and execute profile-backed
  `push --network`
- run the current foreground daemon lifecycle surface as a supervisor-friendly
  wrapper over `serve`

## Boundaries

The important non-claims are part of the product contract:

- Supermover is one-way `source -> trusted target`, not bidirectional sync.
- Profile files remain the configuration SSOT. There are no runtime policy or
  target-identity overrides that bypass the audit trail.
- Audit evidence lives under target-side `.supermover` control-plane artifacts.
- Profile-backed TLS 1.3 mTLS protects the current network path. It does not
  provide anonymity. Residual leakage still includes total bytes, duration,
  peer IPs, LAN presence, and Supermover use.
- A new network session refuses already-divergent target files, symlinks, or
  incompatible directories at begin, before payload upload. That is fail-fast
  conflict rejection, not changed-file network sync.
- `dashboard` is a read-only target integrity page against the latest published
  manifest snapshot. It is not post-publish source comparison, Merkle-root
  proof, or synchronization.
- `daemon` is a foreground lifecycle surface only. It does not install
  launchd/systemd/Windows services, spawn a detached process, browse LAN
  services, watch files, or run ongoing sync.
- `discover` emits explicit address hints only. It is not LAN browsing and not
  trust establishment.

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
paths outside the selected manifest once on open and again on explicit refresh.
It refuses overlapping full checks and avoids re-reading declared file content
twice in the same integrity pass.

Open only the emitted access-token URL. Reach it remotely through SSH port
forwarding rather than binding it to a LAN interface.

### Pairing, Discover, and Serve

`serve` validates a target profile and, for valid pairing-only profiles, binds a
low-information pairing listener that prints an operator verification code and
returns pairing bootstrap material only after that code is presented.

When the profile is already paired and has complete `network.receiver_url` plus
`network.local_tls_identity` material, `serve` also binds the receiver endpoint
from the profile and exposes upload routes over pinned mutual TLS. With no
receiver material, `serve` stays pairing-only. Once a paired profile has any
receiver material, `serve` refuses to start until the receiver material is
complete and auditable.

`pair` requires the verification code before it writes a durable pairing
receipt under the target control plane and pins target device identity in the
profile. `discover` emits untrusted explicit address hints only.

### Network Transfer

Profiles carry the network SSOT for the current operator path:
`network.receiver_url` and `network.local_tls_identity` identify the selected
receiver endpoint and local certificate/key references.

`push --network --dry-run` is preflight-only. It validates profile, pairing,
network material, local TLS identity files and pins, scan, and manifest shape
without contacting the receiver, writing target artifacts, or copying files.

Non-dry-run `push --network` uses the profile material plus pairing receipt
pins to connect to the profile-selected TLS 1.3 mTLS receiver, stream files
through `networkpush` / `networkrun` / `protocolclient`, and write
receiver-side network transfer evidence after receiver begin creates a session.

The current recovery evidence is bounded, not broad. Same-profile, same-session
reruns can recover from authenticated receiver status only when prior
payload-overhead evidence remains auditable. Current acceptance evidence covers
receiver listener restart over the same profile-selected target control plane,
published-session retry with no chunk upload, fail-closed
`payload_overhead_missing`, and deterministic `networkrun`
source-stop-after-progress resume.

That does not make `recover` a network recovery command, and it does not close
LAN browsing, daemon sync, broad resume acceptance, arbitrary process-kill
recovery, OS crash recovery, or anonymity.

### Prune Review And Apply

`prune --dry-run` validates the profile prune policy, reads published
soft-delete records, and emits review-only candidates, refusals, and artifact
problems without deleting target files or writing prune approvals. Active
`delete_policy.retention_days` windows remain visible as
`retention_window_active` refusals rather than approval candidates.

`prune approve --profile <path> --id <approval-id> --soft-delete <id>
[--soft-delete <id>...] --reason <text> --reviewer <id>` authors a durable
approval artifact under `.supermover/prune/approvals/<id>.json` from fresh
dry-run candidate evidence. `--approved-by` is an alias for `--reviewer`,
`--expires-at <RFC3339>` is optional, and `--format text|json` is supported. It
does not delete target files or write prune receipts.

`prune approvals --profile <path>` lists current-scope approval artifacts
without mutating them. `prune supersede --profile <path> --id <approval-id>
--reason <text> --reviewer <id>` updates one approval artifact to durable
`superseded` review state without deleting target files or writing receipts.

`prune --apply --approval <id>` remains the only physical prune path. It writes
a started prune receipt before target mutation, re-runs the current prune plan,
rechecks target evidence, and records final `applied` / `partial` / `failed`
status in the same receipt path.

### Drift And Narrow Reconcile

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
SSOT and currently handles only missing regular-file restores from matching
published/source evidence plus resolve-noop for already-restored or
already-absent targets.

Broad automatic reconcile, durable repair receipts, retry policy, background
scans, live-only repair, manifest rewrite, daemon sync, and ongoing sync remain
planned.

### Daemon

`daemon install`, `daemon run --foreground`, `daemon status`, `daemon logs`,
`daemon restart`, and `daemon stop` persist lifecycle evidence under
`.supermover/daemon` and wrap the same profile-backed `serve` behavior.

The current daemon slice is a foreground, supervisor-friendly lifecycle surface
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
go run ./cmd/supermover report --profile ./supermover.profile.json
go run ./cmd/supermover status --profile ./supermover.profile.json
go run ./cmd/supermover recover --profile ./supermover.profile.json --dry-run
```

When drift review is needed:

```bash
go run ./cmd/supermover drift record --profile ./supermover.profile.json --format json
go run ./cmd/supermover drift acknowledge --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<operator review reason>"
go run ./cmd/supermover drift resolve --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<target restored reason>"
go run ./cmd/supermover reconcile plan --profile ./supermover.profile.json --id <persisted-drift-id> --format json
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --id <persisted-drift-id> --apply --reason "<operator repair reason>"
```

When prune review is needed:

```bash
go run ./cmd/supermover prune --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover prune review --profile ./supermover.profile.json
go run ./cmd/supermover prune approve --profile ./supermover.profile.json --id <approval-id> --soft-delete <id> --reason "<operator review reason>" --reviewer <reviewer>
go run ./cmd/supermover prune --profile ./supermover.profile.json --apply --approval <approval-id>
```

## Roadmap Truth

Still planned:

- LAN browsing beyond explicit address hints
- OS-managed or detached daemon lifecycle and ongoing incremental sync
- broad arbitrary interruption recovery, broad resume acceptance, and network
  `recover`
- broad automatic repair/reconcile, repair receipts, background scans, and
  richer drift-to-prune integration
- broader operator-facing traffic privacy acceptance beyond the current
  profile-backed level 2 evidence path

## Operator Docs

- [User migration guide](docs/user-migration-guide.md): current local push
  workflow, audit artifacts, and post-run checks
- [Operations runbook](docs/runbook.md): repeatable dry-run, publish, review,
  recovery, and incident procedures
- [Troubleshooting matrix](docs/troubleshooting.md): symptoms, likely causes,
  evidence to collect, and safe actions
- [Compact status contract](docs/status.md): local-only profile/target status
  fields, exit codes, and boundaries
- [v1 scope and non-goals](docs/v1-scope.md): product boundaries and non-claims
- [Release audit](docs/release-audit.md): current checkpoint, validation gates,
  and known planned surface

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
