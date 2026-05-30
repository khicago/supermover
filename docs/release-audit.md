# Release Audit

This document is the current checkpoint truth for Supermover. It is not a full
project history dump. It answers four questions:

1. what is wired now
2. what evidence backs that claim
3. what remains intentionally out of scope
4. what must still pass before a release claim is honest

Detailed Bagakit runtime notes may exist under `.bagakit/`, but durable release
truth belongs here.

## Current Checkpoint

The current product checkpoint is broader than the original local push slice
and still narrower than the full v1 request.

### Implemented

Implemented now:

- one-way local publish, verify, report, status, and recover
- prune dry-run, approval authoring, approval inventory, approval supersede,
  focused prune review, and reviewed physical prune apply
- live drift detection, durable drift record, persisted drift acknowledge,
  persisted drift expire, persisted drift resolve, and narrow
  persisted-drift reconcile
- verification-code pairing, explicit-address discover hints, bounded sparse
  UDP LAN browse/advertise discovery, paired profile-backed `serve`, and
  non-dry-run profile-backed `push --network`
- bounded network recovery evidence for same-session resume, published-session
  retry, receiver listener restart over preserved target state, and fail-closed
  missing-prior-evidence handling
- loopback-only read-only `dashboard`
- foreground daemon lifecycle evidence around `serve`
- durable incremental queue evidence, read-only per-entry `sync queue list`,
  explicit operator failed-terminal queue state, one bounded local `sync run`
  pass, foreground local `sync loop` polling, and profile-enabled foreground
  daemon local polling sync, and foreground OS watcher execution through
  `sync watch`, plus one bounded profile-backed network queue pass through
  `sync network run`, bounded LAN-discovery-gated profile-backed network queue
  execution through `sync network discover-run`, and foreground profile-backed
  network polling through `sync network loop`

Not implemented now:

- detached or OS-managed daemon lifecycle
- automatic LAN discovery endpoint selection
- broad arbitrary interruption recovery
- broad automatic repair or reconcile
- network `recover`
- anonymity

## Current Evidence

The current checkpoint is backed by three evidence layers.

### Command Surface

First, command surfaces are wired:

- `profile`, `scan`, `push`, `verify`, `deleted list`, `health`, `report`,
  `status`, `recover`
- `drift list`, `drift record`, `drift acknowledge`, `drift expire`,
  `drift resolve`
- `reconcile plan`, `reconcile review`, `reconcile apply`
- `prune --dry-run`, `prune approve`, `prune approvals`, `prune supersede`,
  `prune review`, `prune --apply --approval <id>`
- `serve`, `discover`, `pair`
- `push --network`
- `dashboard`
- `daemon install`, `daemon run --foreground`, `daemon status`, `daemon logs`,
  `daemon restart`, `daemon stop`
- `sync queue enqueue`, `sync queue status`, `sync queue list`,
  `sync queue ready`, `sync queue cancel`, `sync queue fail`, `sync run`,
  `sync loop`, `sync watch`

### Durable Artifacts

Second, the target control plane is the durable evidence surface:

- manifests, receipts, warnings, soft deletes, drift records, prune approvals,
  prune receipts, daemon lifecycle state, incremental queue/run receipts, and
  network transfer outcomes all live under target-side `.supermover`
- `report`, `status`, `health`, `verify`, `drift list`, and `dashboard` read
  those artifacts or the current target filesystem; they do not invent a second
  truth source

### Automated Network Evidence

Third, automated evidence exists for the current bounded network path:

- same-session receiver-status resume when prior payload-overhead evidence is
  still auditable
- published-session retry without chunk reupload
- receiver listener restart over preserved target control-plane state
- `payload_overhead_missing` fail-closed behavior
- zero-byte regular-file transfer through explicit final empty completion

The newer checkpoint also adds two important constraints:

- new receiver sessions reject already-divergent target files, symlinks, or
  incompatible directories at begin, before payload upload
- `dashboard` verifies target state through a loopback-only, token-gated,
  read-only page instead of claiming sync or source comparison

## Release Gates

Use these commands as the release gate for the current checkpoint:

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
go run ./cmd/supermover push --network --help
go run ./cmd/supermover status --help
go run ./cmd/supermover drift help
go run ./cmd/supermover drift expire --help
go run ./cmd/supermover sync --help
go run ./cmd/supermover sync run --help
go run ./cmd/supermover sync loop --help
go run ./cmd/supermover recover --help
go run ./cmd/supermover prune --help
go run ./cmd/supermover dashboard --help
```

### Local Smoke

For a local smoke, preserve:

- `push`
- `verify`
- `report`
- `status`
- `sync queue status`
- `sync run`
- `sync loop`
- `.supermover/incremental-sync/.../queue.json`
- `.supermover/incremental-sync/.../runs/<session>.json`
- target `.supermover` artifacts

### Profile-Backed Network Smoke

For a profile-backed network smoke, preserve:

- target `serve` stderr
- source `push --network --dry-run`
- source `push --network --session <id>`
- target `verify`
- target `health`
- target `report`
- target `status`
- receiver-side `.supermover/sessions/<session>/network-transfer.json`

## Known Planned Surface

The current checkpoint is still intentionally incomplete.

- LAN discovery is bounded sparse UDP browse/advertise and remains
  unauthenticated address-hint material, not trust.
- Traffic privacy level 2 is release evidence for the current profile-backed
  path only. It is not an anonymity claim.
- Broad interruption acceptance remains planned beyond the current bounded
  same-session resume and retry matrix.
- Broader automatic repair, automatic/background retry, background live-only
  repair beyond explicit live-recording gates, manifest rewrite, broad daemon
  repair retry/background policy, and drift-to-prune integration remain
  planned. The
  current `reconcile review` surface is read-only boundary evidence,
  `reconcile apply --all-persisted-planned` is only a gated persisted-evidence
  selection path, and `reconcile apply --record-live` first persists current
  live detector findings before applying resulting persisted planned actions.
  Profile-backed foreground daemon drift recording can persist live detector
  findings as durable review evidence, but it does not apply repair.
  Profile-backed foreground daemon persisted reconcile apply can apply only
  already persisted, currently planned reconcile actions through existing
  receipts and stops after refusals for operator review. It is mutually
  exclusive with profile-backed drift recording so live-only detector findings
  cannot become implicit daemon apply input.
- Foreground daemon lifecycle evidence exists, but detached daemon management,
  crash supervision, and OS service-manager installation remain planned.
- Incremental queue evidence exists for read-only per-entry queue inspection,
  explicit queue failure review, one bounded local `sync run` pass, foreground
  local `sync loop` polling, and profile-enabled foreground-daemon local
  polling sync, foreground OS watcher `sync watch`, and bounded
  profile-backed `sync network run`, bounded LAN-discovery-gated
  `sync network discover-run`, foreground `sync network loop`, and
  profile-enabled source-side foreground-daemon network polling sync, including
  per-entry profile-backed mTLS network queue publication, but detached
  background execution and automatic LAN discovery endpoint selection remain
  planned.

## Safety Notes

- The migration-ready path still assumes an empty trusted target or an
  idempotent rerun where existing target content already matches the intended
  result.
- New network sessions fail fast on known target conflicts, but that does not
  turn network push into changed-file sync.
- `report`, `status`, and `dashboard` are review surfaces. They do not make the
  target clean, authorize prune, repair drift, or prove future source state.
- `recover` is conservative and local-first. It is not the broad network
  recovery interface.
- Approval artifacts and drift records are review truth, not silent mutation
  permission.

## Commit Trail

Recent commits that materially define the current checkpoint:

- `3e323cd` feature(f-234nwra8e): close prune release workflow with approval inventory and supersede
- `5c2a70a` feat(v1): wire bounded network and control-plane acceptance
- `0cc0075` feat(v1): add local integrity dashboard and fail-fast network preflight

Older history remains in Git. This document tracks the current release
boundary, not every historical step.
