# Architecture

Supermover v1 is organized around a durable control plane before transport
optimization.

## Product Shape

- Direction: one-way `source -> target`.
- Target trust: the target restores a plaintext, browsable file tree.
- Configuration: profiles are the SSOT; CLI actions update profiles instead of
  relying on broad runtime overrides.
- Deletion: source deletions become soft-delete records first; physical pruning
  is separate.
- Consistency: `live`, `strict`, and `snapshot` are explicit modes.
- Discovery: current discovery supports explicit address hints only and does not
  browse LAN. Pairing and pinned device identity establish trust.

## Control Plane

The target-side `.supermover` area is a first-class artifact surface. Current
wired command surfaces write:

- profile snapshots
- session receipts
- manifests
- audit warnings
- soft-delete records
- transaction recovery state
- target-drift records from refused managed updates or `drift record`
- pairing receipts from verified local pairing
- network transfer outcome artifacts from non-dry-run profile-backed network
  attempts that reach receiver begin

The schema and path foundation also covers planned history surfaces:

- history indexes

The control plane must be machine-readable and stable enough for current
`verify`, `deleted list`, `prune`, `health`, `drift list`, `drift record`,
`drift acknowledge`, `drift resolve`, `report`, `status`, and `recover`
commands plus planned broad drift reconcile/repair and agent-facing reporting
commands.

## Implementation Boundary

Supermover preserves and catalogs agent knowledge files, but it does not
interpret, merge, summarize, embed, or promote semantic memory. Downstream
agent or knowledge tools can consume manifests after sync.
