# Implementation Plan

Supermover v1 should be built in small, reviewable slices. The implementation
order follows the local Bagakit feature plan, but this tracked document keeps
the public project direction visible without depending on ignored local
planning files.

## Current Execution Features

The original broad tracker feature has been archived as historical planning
evidence. Remaining work is split into narrower proposal-only features under
`.bagakit/feature-tracker/features/`:

| Feature | Purpose |
| --- | --- |
| `f-223nw49qj` | Migration audit report and status UX. |
| `f-224nw98v7` | Reviewed physical prune flow. |
| `f-225nwsa3h` | Changed-file incremental local sync. |
| `f-226nwy2vy` | LAN agent discovery and pairing. |
| `f-227nw2p2n` | Secure resumable transport integration. |
| `f-228nws66k` | Traffic privacy level 2 implementation. |
| `f-229nwwybc` | Failure injection and release hardening. |

Recommended next implementation feature: `f-223nw49qj`, because it closes the
operator visibility gap for warnings, soft deletes, recovery state, profile
suggestions, and migration completeness.

## Phase 1: Project Skeleton

- Go module and `cmd/supermover` entrypoint.
- Basic CLI help and version command.
- Architecture, profile, threat-model, and planning docs.
- Baseline `go test ./...` validation.

## Phase 2: Profile And Control Plane

- Profile schema as the configuration SSOT.
- `.supermover` control-plane schemas for profile snapshots, receipts,
  manifests, warnings, history, soft deletes, target drift, and recovery.
- Deterministic JSON encoding and validation.
- Profile lint and doctor checks.

## Phase 3: Scan, Audit, And Agent Knowledge

- Filesystem scanning for ordinary file-tree fidelity.
- Structured audit records for unsupported metadata and risky filesystem
  cases.
- Agent influence manifest for rule and memory files such as `AGENTS.md`,
  `CLAUDE.md`, `.cursor/rules/**`, and `.github/instructions/**`.

## Phase 4: Local Durability

- Target-side staging and atomic promotion.
- Session journals and recovery scanning.
- Read-only `health` diagnostics for interrupted or invalid sessions.
- Danger-pause rules for missing roots, root fingerprint changes, target drift,
  mass permission loss, and major policy changes.
- Soft-delete review and prune dry-run.

## Phase 5: Secure Transport

- Low-information LAN discovery.
- Explicit pairing with persistent pinned device identity.
- Secure transport with profile-bound session context.
- Privacy level 2 padding, batching, bounded timing jitter, and clear limits.

## Phase 6: End-To-End Sync

- Incremental sync and resumable large-file transfer.
- `live`, `strict`, and `snapshot` consistency behavior.
- Review commands for deletes and target drift.
- Recover and prune commands.

## Phase 7: Quality Bar

- Failure-oriented integration tests.
- Security and recovery documentation.
- CI, contribution guide, security policy, and release process.
