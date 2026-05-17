# Supermover

Supermover is a Go CLI for one-way, auditable file migration. The long-term
design includes incremental sync from a source machine to a trusted target
machine.

The current implementation is a local push vertical slice. Available commands
are `profile`, `scan`, `push`, `verify`, `deleted list`, `health`, `report`,
and `recover`. It supports first migration, idempotent reruns,
additions, warning records, soft-delete records, read-only operator reports,
and conservative recovery. Changed-file incremental update, network receiver
CLI wiring, pairing, physical prune, broader recovery reconciliation, status,
discovery, and drift review commands are planned and may appear in design docs
before CLI wiring exists.

## Quickstart

```bash
go run ./cmd/supermover profile init --profile ./supermover.profile.json --source /path/to/source --target /path/to/empty-target
go run ./cmd/supermover profile lint --profile ./supermover.profile.json
go run ./cmd/supermover push --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover push --profile ./supermover.profile.json --session session-001
go run ./cmd/supermover verify --profile ./supermover.profile.json --session session-001
go run ./cmd/supermover deleted list --profile ./supermover.profile.json
go run ./cmd/supermover health --profile ./supermover.profile.json
go run ./cmd/supermover report --profile ./supermover.profile.json
go run ./cmd/supermover recover --profile ./supermover.profile.json --dry-run
```

Use an empty target directory for first migration. Current publish code refuses
to overwrite an existing target file or symlink unless the existing object is
content-identical, which keeps the migration path conservative for machine
replacement.

`push --dry-run` reports counts only; full warning JSON is written after a
published run. Source scanner `scan_error` findings block push instead of being
published as review warnings. `verify` checks published regular files for
size, SHA-256 digest, permissions, and modification time, and checks directory
and symlink entries for presence/type/target fidelity. It exits non-zero for
error findings, warning findings, artifact problems, or a missing manifest.
`report` is read-only and aggregates the profile SSOT plus target
`.supermover` artifacts into an operator view of warnings, profile suggestions,
soft deletes, health/recovery issues, artifact problems, and verification state
at report time. It returns non-zero when the report requires operator review,
even if the report itself was generated successfully.

The v1 direction is intentionally conservative:

- one-way `source -> trusted target`
- profile files as the configuration SSOT
- `.supermover` control-plane artifacts for receipts, manifests, warnings,
  history, target drift, soft deletes, and recovery
- planned explicit LAN pairing; discovery is not trust
- planned bounded traffic metadata reduction, not anonymity
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
