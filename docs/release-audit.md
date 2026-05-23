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
  persisted drift resolve, and narrow persisted-drift reconcile
- verification-code pairing, explicit-address discover hints, paired
  profile-backed `serve`, and non-dry-run profile-backed `push --network`
- bounded network recovery evidence for same-session resume, published-session
  retry, receiver listener restart over preserved target state, and fail-closed
  missing-prior-evidence handling
- loopback-only read-only `dashboard`
- foreground daemon lifecycle evidence around `serve`

Not implemented now:

- LAN browsing
- detached or OS-managed daemon lifecycle
- ongoing incremental sync
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
- `drift list`, `drift record`, `drift acknowledge`, `drift resolve`
- `reconcile plan`, `reconcile apply`
- `prune --dry-run`, `prune approve`, `prune approvals`, `prune supersede`,
  `prune review`, `prune --apply --approval <id>`
- `serve`, `discover`, `pair`
- `push --network`
- `dashboard`
- `daemon install`, `daemon run --foreground`, `daemon status`, `daemon logs`,
  `daemon restart`, `daemon stop`

### Durable Artifacts

Second, the target control plane is the durable evidence surface:

- manifests, receipts, warnings, soft deletes, drift records, prune approvals,
  prune receipts, daemon lifecycle state, and network transfer outcomes all live
  under target-side `.supermover`
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

- LAN browsing remains planned beyond explicit address hints.
- Traffic privacy level 2 is release evidence for the current profile-backed
  path only. It is not an anonymity claim.
- Broad interruption acceptance remains planned beyond the current bounded
  same-session resume and retry matrix.
- Broader repair, reconcile, drift-to-prune integration, and durable repair
  receipts remain planned.
- Foreground daemon lifecycle evidence exists, but detached daemon management,
  crash supervision, and OS service-manager installation remain planned.

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
