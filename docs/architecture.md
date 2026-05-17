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
- Planned discovery: LAN discovery returns address hints only. Planned pairing
  and pinned device identity establish trust.

## Control Plane

The target-side `.supermover` area is a first-class artifact surface. The
current local slice writes:

- profile snapshots
- session receipts
- manifests
- audit warnings
- soft-delete records
- history indexes
- recovery state

Planned network and drift surfaces add:

- pairing receipts
- target-drift records

The control plane must be machine-readable and stable enough for current
`verify`, `deleted list`, `health`, `report`, and `recover` commands plus
planned prune, status, drift, and agent-facing reporting commands.

## Implementation Boundary

Supermover preserves and catalogs agent knowledge files, but it does not
interpret, merge, summarize, embed, or promote semantic memory. Downstream
agent or knowledge tools can consume manifests after sync.
