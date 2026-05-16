# Supermover

Supermover is a planned Go CLI for one-way, auditable file migration and
incremental sync from a source machine to a trusted target machine.

The current implementation is a local push vertical slice. Available commands
are `profile`, `scan`, `push`, `verify`, and `deleted list`. Network receiver,
pairing, physical prune, recovery, status, discovery, health, and drift review
commands are planned and may appear in design docs before CLI wiring exists.

The v1 direction is intentionally conservative:

- one-way `source -> trusted target`
- profile files as the configuration SSOT
- `.supermover` control-plane artifacts for receipts, manifests, warnings,
  history, target drift, soft deletes, and recovery
- explicit LAN pairing; discovery is not trust
- bounded traffic metadata reduction, not anonymity
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

## Development

```bash
go test ./...
go run ./cmd/supermover help
```
