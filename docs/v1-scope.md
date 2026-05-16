# v1 Scope And Non-Goals

Supermover v1 is a conservative one-way migration tool. The design favors
reviewable state, explicit trust, and recoverable operations over broad sync
magic.

## In Scope

- One-way migration from source to trusted target. Current local push supports
  first migration, idempotent reruns, additions, warning records, soft-delete
  records, and conservative recovery; changed-file incremental update is
  planned.
- Profile JSON as the configuration single source of truth.
- Target-side `.supermover` control plane for profile snapshots, pairing
  receipts, session receipts, manifests, warnings, history, target drift,
  soft-delete records, and recovery state.
- Local push vertical slice that copies supported regular files to a trusted
  local target and writes auditable control artifacts.
- Explicit warning records for recoverable or reviewable gaps.
- Soft-delete record and review flow before any physical target pruning.
- Ordinary file-tree fidelity with supplemental migration records for behavior
  that cannot be represented directly in the target filesystem.
- Agent knowledge files migrated as files and cataloged for downstream tools.
- Planned LAN discovery that exposes low-information address hints only.
- Planned pairing receipts and pinned device identity as the trust boundary.
- Planned bounded traffic metadata reduction: padding, batching, and jitter at
  defined policy levels.
- Recovery classification for interrupted or incomplete local sessions.

## Non-Goals

- Bidirectional sync.
- Automatic trust based on LAN discovery.
- Automatic physical deletion from the target when a source file disappears.
- Runtime flags that silently override profile policy for delete, privacy,
  metadata, or target identity behavior.
- Semantic merging, summarization, embedding, or rewriting of agent memory.
- Strong anonymity or hiding the fact that Supermover is in use.
- Hiding total transfer size, transfer duration, peer IP addresses, or LAN
  presence.
- Encrypted repository mode for untrusted targets.
- Protection from a compromised trusted endpoint.
- Network transport completeness in the local push vertical slice.

## Acceptance Principles

### Warning Auditability

A warning must be durable, machine-readable, and tied to a session. A successful
run with warnings is acceptable only when operators can inspect warning records
under `.supermover/warnings/` and decide whether to accept, rerun, or block.

Warnings must not be console-only. Console summaries may report warning counts,
but the target control plane is the evidence surface.

### Soft-Delete Review

Source-side deletion is not permission to delete the target copy immediately.
The target must first receive a soft-delete record with source path, target
path, profile/target/root scope, previous manifest evidence, detected time, and
reason. Physical pruning is a later action gated by profile policy and operator
review.

For v1, `delete_policy.mode: prune` without `require_review: true` is invalid.

### Profile SSOT

Profiles are the source of truth for migration behavior. Commands may select a
profile and may create or lint a profile, but policy changes belong in the
profile itself. Each successful run must preserve the profile snapshot used for
that run so target state can be audited later.

`profile lint` is a schema/safety gate. Implementation readiness still requires
the operational gates for the selected slice, such as `push --dry-run`, `verify`,
and `health`.

### Discovery Is Not Trust

Planned LAN discovery returns unauthenticated address hints. Discovery
advertisements must remain sparse and must not disclose identity, local layout,
profile data, or inventory size.

Trust starts only after planned explicit pairing verification writes a receipt
and pins device identity. A discovered endpoint without pairing is not a
migration target.

## Current Slice Versus Planned Surface

Implemented local slice:

```bash
go run ./cmd/supermover profile init --profile <path> --source <path> --target <path>
go run ./cmd/supermover profile lint --profile <path>
go run ./cmd/supermover profile set-target --profile <path> --target <path>
go run ./cmd/supermover scan --profile <path>
go run ./cmd/supermover push --profile <path> --dry-run
go run ./cmd/supermover push --profile <path> --session <session-id>
go run ./cmd/supermover verify --profile <path> --session <session-id>
go run ./cmd/supermover deleted list --profile <path>
go run ./cmd/supermover health --profile <path>
go run ./cmd/supermover recover --profile <path> --session <session-id>
```

Planned mainline surface:

```bash
go run ./cmd/supermover serve --profile <target-profile>
go run ./cmd/supermover discover
go run ./cmd/supermover pair --profile <path> --target <address-or-advertisement-id>
go run ./cmd/supermover status --profile <path>
go run ./cmd/supermover drift list --target <path> --profile <path>
go run ./cmd/supermover prune --target <path> --profile <path> --dry-run
```
