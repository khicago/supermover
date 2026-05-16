# Supermover

Supermover is a planned Go CLI for one-way, auditable file migration and
incremental sync from a source machine to a trusted target machine.

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

The implementation plan lives in
`.bagakit/feature-tracker/features/f-222nwudju/proposal.md`.

## Development

```bash
go test ./...
go run ./cmd/supermover help
```

