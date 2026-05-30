# Supermover

[![Go](https://github.com/khicago/supermover/actions/workflows/go.yml/badge.svg)](https://github.com/khicago/supermover/actions/workflows/go.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Status: pre-release](https://img.shields.io/badge/status-pre--release-orange.svg)](#current-status)

Supermover is a Go CLI for one-way, auditable file migration from a source
machine to a trusted target. It is profile-driven, conservative about target
mutation, and built around durable evidence under the target `.supermover`
control plane.

## Current Status

Supermover is **pre-release**.

- There is no official tagged binary release yet.
- The CLI is the authoritative execution surface today.
- The native macOS app is a CLI-backed operator workbench that can be built
  locally, but current local app bundles are not public release artifacts.
- Do not treat unsigned, ad-hoc signed, dirty, or unstapled app bundles as
  install-ready distribution builds.
- Real two-machine app-first acceptance, Developer ID signing, notarization,
  stapling, and Gatekeeper-ready release evidence remain planned.

Use Supermover from source if you are comfortable with explicit profiles,
operator review, and the documented safety boundaries. Wait for a tagged
release if you need a packaged end-user installer.

## Install

### From Source

```bash
git clone https://github.com/khicago/supermover.git
cd supermover
go run ./cmd/supermover version
go run ./cmd/supermover help
```

The module currently follows the Go version declared in `go.mod`.

### Local macOS App Build

```bash
macos/script/bootstrap-build-env.sh --for-build-app
macos/script/build-app.sh
open macos/dist/SuperMover.app
```

`build-app.sh` runs the build-environment bootstrap before compiling. The
bootstrap creates project-local build output directories and checks macOS build
tools such as Go, Swift/Xcode Command Line Tools, `sips`, `iconutil`, and
`codesign`. It does not silently install Go, signing certificates, keychain
items, or notarization credentials.

The build creates a local packaged app with a bundled CLI and provenance
manifest. Unless you also pass the release audit, Developer ID signing,
notarization, stapling, and two-machine acceptance gates, this app is local
review evidence only. It is not an official download release.

## Quick Start: Local Auditable Migration

Use an empty trusted target directory for the first run:

```bash
go run ./cmd/supermover profile init \
  --profile ./supermover.profile.json \
  --source ./example-source \
  --target ./example-empty-target

go run ./cmd/supermover profile lint --profile ./supermover.profile.json
go run ./cmd/supermover push --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover push --profile ./supermover.profile.json --session session-001
go run ./cmd/supermover verify --profile ./supermover.profile.json --session session-001
go run ./cmd/supermover status --profile ./supermover.profile.json
```

Review the target `.supermover` artifacts after each meaningful run. Command
stdout is useful for triage, but the durable audit trail lives under the target
control plane.

## Capability Snapshot

The shortest honest summary is this: Supermover already has a usable local
migration path, bounded low-information LAN browsing, a bounded profile-backed
network path, local incremental queue execution through `sync run` and
foreground `sync loop`, profile-enabled foreground-daemon local polling sync,
foreground OS watcher execution through `sync watch`, one bounded
profile-backed network queue run through `sync network run`, foreground
profile-backed network polling through `sync network loop`,
LAN-discovery-gated profile-backed network queue execution through
`sync network discover-run`, profile-enabled foreground-daemon source-side
network polling sync, profile-enabled foreground-daemon drift review evidence
recording, profile-enabled foreground-daemon persisted reconcile apply over
currently planned durable drift records, and a growing review surface. It does
not yet have a detached daemon, broad automatic repair, automatic discovery
endpoint selection, broad resumable network recovery, or anonymity.

## What Works Now

Today you can:

- define migration intent in a profile and treat that profile as the SSOT
- build and run the native macOS app as a CLI-backed operator workbench for
  profile setup, native discovery/pairing command orchestration, selected
  wired commands, sync queue/run/loop/watch/network controls, role-scoped
  foreground process supervision, structured evidence/problem display, and a
  native Evidence Vault with typed evidence cards, raw JSON envelope history,
  searchable target `.supermover` artifact catalog, explicit
  target-vs-manifest alignment scope, and evidence-bound review-metadata
  actions
- run first migration, idempotent reruns, additions, and managed changed-file
  updates for previously published regular files
- verify target state with `verify`, `report`, `status`, and the local-only
  `dashboard`
- review source-side deletions through `prune --dry-run`, author approval
  artifacts, inspect approval inventory, supersede old approvals, and physically
  prune only through `prune --apply --approval <id>`
- persist, acknowledge, expire, resolve, narrowly reconcile drift records, and
  review broad repair boundaries without mutation
- browse low-information LAN candidates, advertise sparse profile-backed
  presence, pair a source and target, run `serve`, and execute profile-backed
  `push --network`
- snapshot profile roots into a durable changed-file queue, inspect/cancel/fail
  queue entries, run one bounded local `sync run` pass, or run a foreground
  local `sync loop` polling pass sequence, or run foreground OS watcher
  execution through `sync watch`; each publishes ready entries through the
  existing local push safety path
- run one bounded `sync network run` pass that stores queue/run receipts under
  the profile-selected target control plane and publishes ready queue entries
  through a per-entry profile-backed mTLS network manifest
- run foreground `sync network loop` polling that repeats that same
  per-entry profile-backed network queue pass with generated session IDs and
  bounded `--max-runs` smoke support
- run one `sync network discover-run` pass that first requires a
  low-information LAN candidate matching the profile-selected
  `network.receiver_url`, then publishes through the same per-entry
  profile-pinned mTLS transfer
- run the current foreground daemon lifecycle surface as a supervisor-friendly
  wrapper over `serve`, optionally running profile-enabled local polling sync
  in the same foreground process, or source-side profile-enabled network
  polling sync as a worker-only foreground daemon mode
- record current live drift findings as durable review evidence from the
  foreground daemon when `repair.drift_recording` is enabled in the profile;
  this does not apply repair or reconcile mutations
- apply currently planned persisted reconcile actions from the foreground daemon
  when `repair.persisted_reconcile_apply` is enabled in the profile; this uses
  existing reconcile receipts and does not consume live-only detector findings

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
- `sync queue`, `sync run`, `sync loop`, and `sync watch` are local target control-plane
  surfaces. `sync run` is one bounded scan/enqueue/publish pass; `sync loop`
  is a foreground polling loop over the same local queue and run receipts;
  `sync watch` is a foreground OS watcher over existing source directories.
- `sync network run` is a bounded queue consumer and `sync network loop` is a
  foreground polling loop over that same profile-backed network queue pass.
  Both still require the profile-selected local target control plane for
  queue/run receipts, then publish only ready queue entries through a per-entry
  profile-backed mTLS network manifest. Regular-file replacement requires
  previous published manifest evidence and receiver-side target revalidation.
  They are not LAN discovery, automatic endpoint selection, OS file watchers,
  background daemons, broad repair, or a bidirectional sync engine.
- `sync network discover-run` adds a LAN discovery gate before the same
  profile-backed network queue pass. The LAN candidate must match
  `network.receiver_url`; discovery does not select, rewrite, or trust an
  endpoint.
- `dashboard` is a read-only target integrity page against the latest published
  manifest snapshot. It is not post-publish source comparison, Merkle-root
  proof, or synchronization.
- `daemon` is a foreground lifecycle surface only. When the profile enables
  `sync.local_polling`, it also runs the same local queue consumer and writes
  run receipts in that foreground process. When the profile enables
  `repair.drift_recording`, it records current live detector findings as
  durable drift review evidence without applying repair. When the profile enables
  `repair.persisted_reconcile_apply`, it applies only currently planned
  persisted drift records through the existing reconcile apply receipt path and
  stops after refusals for operator review. When the profile enables
  `sync.network_polling`, it runs a source-side network queue worker instead
  of serving pairing or receiver routes. It does not install
  launchd/systemd/Windows services, spawn a detached process, watch files,
  execute LAN-discovery-gated sync, select endpoints automatically, or run
  broad automatic repair, live-only repair apply, manifest rewrite, prune
  handoff, or automatic retry policy.
- `discover` emits explicit address hints and bounded low-information LAN
  datagram browse candidates. Discovery is not trust establishment.
- The native macOS app is not yet the complete app-first two-machine workflow.
  Discovery/pairing and sync controls are wired to current CLI commands, the
  app shows CLI provenance/readiness, and the packaging script can bundle the
  CLI with a provenance manifest plus optional code signing. Source profile
  creation in the app uses the recommended
  `~/.supermover/profile-local.json` path by default, while custom profile
  destinations remain an Advanced operator action. Final notarized
  distribution and two-machine acceptance evidence remain planned. The app now
  reads typed `verify --format json` target-vs-published-manifest evidence,
  catalogs target `.supermover` artifacts without following symlinks from the
  manually selected Target Root field, can run only bounded review-metadata
  actions from evidence cards, and exposes foreground daemon controls without
  claiming OS-managed detached service installation. Vault-side prune approval is
  single-candidate only. Merkle/root proof plus current-source comparison remain
  unavailable because no wired Merkle tree, content-root artifact, or
  current-source comparison artifact exists.

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
profile. `discover --address` emits untrusted explicit address hints;
`discover browse` listens for sparse LAN datagram advertisements; and
`discover advertise --profile <path>` sends sparse low-information
advertisements derived from the profile privacy policy. Browse/advertise output
is address-hint material only, not trust.
When the reviewed target profile explicitly sets
`discovery.advertise_receiver_hint=true`, `discover advertise` binds its UDP
advertisement source to the profile-selected `network.receiver_url` host:port
so `sync network discover-run` can require an exact hint match before using the
existing pinned mTLS network path. The payload remains sparse low-information
discovery data and still does not carry profile IDs, device IDs, paths, or file
metadata.

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
daemon sync, broad resume acceptance, arbitrary process-kill recovery, OS crash
recovery, or anonymity.

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

`drift acknowledge`, `drift expire`, and `drift resolve` operate only on
existing persisted drift records. `drift expire` retires stale persisted review
evidence without claiming the target is restored. `drift resolve` closes a
record only after a fresh profile-scoped live detector no longer reports drift
for the same path and expected baseline. Resolved or expired persisted records
stop counting as unresolved persisted drift, but current live drift remains
review-required.

`reconcile plan/review/apply` is a separate persisted-drift repair surface.
`plan` is non-mutating. `review` is also non-mutating: it summarizes the
current persisted reconcile plan, reports live-only detector findings that must
be recorded before selected apply, and marks background scans, manifest
rewrite, daemon/ongoing sync integration, drift-to-prune handoff, and
automatic retry policy as planned boundaries. `apply` requires explicit
selection intent, explicit `--apply`, and `--reason`: either one or more
`--id <persisted-drift-id>` values, or the gated
`--all-persisted-planned` mode that first reviews durable persisted evidence
and selects only currently planned persisted actions, or the gated
`--record-live` mode that first persists current live detector findings as
durable drift records and then applies only those resulting persisted planned
actions. It derives source and target only from the profile SSOT and currently
handles only missing regular-file restores from matching published/source
evidence plus resolve-noop for already-restored or already-absent targets.
`apply` writes durable
`.supermover/reconcile/receipts/<receipt-id>.json` audit evidence for applied,
partial, or refused outcomes; `report` and `status` surface reconcile receipt
counts and non-applied receipt issues. Reconcile refusals now include
`conflict_class` and `retry_advice` review fields so operators can distinguish
scope, target-state, artifact, published-evidence, source-evidence, unsafe
path, unsupported-drift, and mutation-failure cases without treating them as an
automatic retry policy.

When `repair.drift_recording` is enabled in the profile, `daemon run
--foreground` also runs a target-side drift recording worker. It periodically
persists the same current live detector findings that `drift record` would
write as `.supermover/drift/*.json` review evidence. This daemon worker records
evidence only; it does not apply `reconcile`, retry repair, rewrite manifests,
authorize prune, or claim the target is restored.

When `repair.persisted_reconcile_apply` is enabled in the profile, `daemon run
--foreground` also runs a target-side reconcile apply worker. It periodically
reviews persisted drift records, selects only currently planned persisted
actions, and applies them through the existing `reconcile apply` receipt path
using the profile's configured reason and optional reviewer. It does not record
live-only detector findings before apply, does not consume live-only IDs, does
not rewrite manifests or authorize prune, and stops after refused actions or
artifact problems so an operator can inspect the receipt before retrying. The
profile validator rejects enabling it together with `repair.drift_recording` to
avoid implicitly turning live-only detector findings into automatic apply input.

Broad automatic reconcile, background retry policy, background scans,
background live-only repair beyond explicit live-recording gates, manifest
rewrite, broad daemon repair retry/background policy, drift-to-prune
integration, and automatic LAN discovery endpoint selection
remain planned. The broad repair leftovers are split into proposal-only
follow-ups: retry policy, broad scan inventory, manifest rewrite decisions,
repair-to-prune handoff, and background repair operator UX.

### Incremental Sync Queue

`sync queue enqueue|status|list|ready|cancel|fail` stores durable changed-file queue
evidence under the profile-selected target. `sync run --profile <path>
--session <id>` first snapshots the profile roots into that queue, then marks
ready entries `in_flight`, invokes the existing local push path once, and
records `.supermover/incremental-sync/.../runs/<session>.json` evidence.
`sync loop --profile <path> --session-prefix <id>` repeats that local
scan/enqueue/publish pass in the foreground with generated session IDs; use
`--max-runs` for bounded smoke and release checks. `sync watch --profile
<path> --session-prefix <id>` arms recursive OS watchers for existing source
directories, runs one baseline pass, then coalesces file events into additional
local queue/run passes; use `--max-events` for bounded smoke and release
checks. `sync network run --profile <path> --session <id>` runs one bounded
network pass: it validates profile trust, network material, local TLS identity,
and the current network push contract before queue mutation, stores queue/run
receipts under the profile-selected target control plane, and publishes only
ready queue entries through a per-entry profile-backed mTLS network manifest.
Regular-file replacement requires previous published manifest evidence and
receiver-side target revalidation. `sync network loop --profile <path>
--session-prefix <id>` repeats that same network queue pass in the foreground
with generated session IDs; use `--max-runs` for bounded smoke and release
checks. `sync network discover-run --profile <path> --session <id>` first
browses for a low-information LAN candidate and only runs that same per-entry
network queue pass when the candidate address exactly matches the
profile-selected `network.receiver_url`; no queue/run receipt is written when
the discovery gate does not match. Idle queue passes write ordinary run
receipts and do not contact the receiver. A successful network pass marks
entries `done`; a refused publish records retry backoff instead of hiding the
work. `sync queue fail --id <entry-id> --reason
<text>` marks one entry as explicit terminal operator review evidence; it is
not a retry, repair, or target-restored claim. A later changed source
observation can enqueue new work for the same path. Hidden files and
dot-directories remain first-class queue entries.

`sync queue list` is the read-only per-entry inspection surface for persisted
queue details such as path, status, attempts, last error, next due time, failed
time, canceled time, done time, and metadata. `report` and `status` include
compact incremental sync evidence from the durable queue and run receipts.
Completed `done` entries and published run receipts remain audit evidence
without making a verified target dirty; queued, ready, `in_flight`, backoff,
failed entries, retrying runs, and corrupt queue/run artifacts are surfaced as
review evidence.

The same local queue consumer can also run inside `daemon run --foreground`
only when the profile enables `sync.local_polling`. The same profile-backed
network queue pass can run inside `daemon run --foreground` only when the
profile enables `sync.network_polling`; that mode is source-side worker-only
and does not serve pairing or receiver routes. These surfaces do not detach
into the background, add LAN-discovery-gated execution, add automatic endpoint
selection, or add bidirectional conflict resolution.

### Daemon

`daemon install`, `daemon run --foreground`, `daemon status`, `daemon logs`,
`daemon restart`, and `daemon stop` persist lifecycle evidence under
`.supermover/daemon` and normally wrap the same profile-backed `serve`
behavior.

The current daemon slice is a foreground, supervisor-friendly lifecycle surface
with durable status, redacted lifecycle events, stop intent, and restart
intent. Restart is consumed only by a running foreground daemon and restarts
serve listeners in that same process. If the profile enables
`sync.local_polling`, the restarted foreground daemon also restarts the local
polling queue consumer and continues generating durable incremental-sync run
receipts with fresh session IDs. If the profile enables
`sync.network_polling`, the foreground daemon runs a source-side network queue
worker instead of serve listeners, consumes stop/restart intents, and resumes
generated network run receipt numbering from durable receipts after a process
stop/start. This is still foreground-only and profile-backed; it is not OS file
watching, detached daemon execution, automatic discovery-selected sync,
automatic endpoint selection, or broad repair.

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

For local or mounted-target migrations, `profile init --target` creates a
complete profile in one step. For the native app's two-Mac preparation flow,
Source creates only its local side first:

```bash
go run ./cmd/supermover profile init --profile ./supermover.profile.json --source <source-dir> --source-only
go run ./cmd/supermover profile set-target --profile ./supermover.profile.json --target <target-dir>
```

Run `profile set-target` from the Target Mac after it chooses its own local save
folder. Migration and evidence commands still require `target.local_path`; a
source-only profile is a preparation artifact, not a publish-ready profile.

When you want a bounded local changed-file queue pass after source changes:

```bash
go run ./cmd/supermover sync queue enqueue --profile ./supermover.profile.json
go run ./cmd/supermover sync queue list --profile ./supermover.profile.json
go run ./cmd/supermover sync queue ready --profile ./supermover.profile.json
go run ./cmd/supermover sync run --profile ./supermover.profile.json --session sync-run-001
go run ./cmd/supermover sync loop --profile ./supermover.profile.json --session-prefix sync-loop --max-runs 2
go run ./cmd/supermover sync watch --profile ./supermover.profile.json --session-prefix sync-watch --max-events 1
```

When you want the foreground daemon to run the same local polling queue
consumer, enable it in the profile rather than passing a runtime override:

```json
"sync": {
  "local_polling": {
    "enabled": true,
    "interval_millis": 60000,
    "retry_backoff_millis": 60000,
    "session_prefix": "daemon-sync"
  }
}
```

Then run `supermover daemon run --foreground --profile <path>` under your own
supervisor.

When you want the foreground daemon to run source-side network polling, enable
the network polling worker in the source profile. `sync.local_polling` and
`sync.network_polling` are mutually exclusive profile modes:

```json
"sync": {
  "network_polling": {
    "enabled": true,
    "interval_millis": 60000,
    "retry_backoff_millis": 60000,
    "session_prefix": "daemon-network-sync"
  }
}
```

This daemon mode uses the same per-entry profile-backed mTLS network queue
publication as `sync network loop`; it is not LAN discovery driven and does not
select endpoints automatically.

When an operator needs to retire one queued observation as terminal review
evidence:

```bash
go run ./cmd/supermover sync queue fail --profile ./supermover.profile.json --id <entry-id> --reason "<operator terminal reason>"
```

When drift review is needed:

```bash
go run ./cmd/supermover drift record --profile ./supermover.profile.json --format json
go run ./cmd/supermover drift acknowledge --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<operator review reason>"
go run ./cmd/supermover drift expire --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<stale review reason>"
go run ./cmd/supermover drift resolve --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<target restored reason>"
go run ./cmd/supermover reconcile plan --profile ./supermover.profile.json --id <persisted-drift-id> --format json
go run ./cmd/supermover reconcile review --profile ./supermover.profile.json --format json
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --id <persisted-drift-id> --apply --reason "<operator repair reason>"
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --all-persisted-planned --apply --reason "<operator repair reason>"
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --record-live --apply --reason "<operator repair reason>"
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

- complete native macOS app-first source/target workflow acceptance evidence,
  including signed/notarized distribution readiness, two-machine LAN operation,
  and final closeout for workflows that currently remain unavailable such as
  Merkle/current-source proof
- OS-managed or detached daemon lifecycle
- automatic LAN discovery endpoint selection
- broad arbitrary interruption recovery, broad resume acceptance, and network
  `recover`
- broad automatic repair/reconcile follow-ups: retry policy, broad scan
  inventory, manifest rewrite decisions, background live-only repair beyond
  explicit live-recording gates, broad daemon repair retry/background policy,
  repair-to-prune handoff, and background repair operator UX
- broader operator-facing traffic privacy acceptance beyond the current
  profile-backed level 2 evidence path

## Operator Docs

- [Changelog](CHANGELOG.md): pre-release change log and release-history home
- [Support](SUPPORT.md): how to ask for help without leaking private data
- [Security policy](SECURITY.md): supported versions, vulnerability reporting,
  and current security boundaries
- [Contributing guide](CONTRIBUTING.md): validation, PR expectations, and
  safety-first change discipline
- [Release checklist](docs/release-checklist.md): gates before any official
  tagged release or downloadable app claim
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
