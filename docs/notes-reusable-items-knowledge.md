---
title: Reusable Items - Knowledge
sop:
  - Update this catalog when one note, index, or query pattern becomes worth reusing across tasks.
  - Keep source-of-truth links current and remove duplicate entries.
---

# Reusable Items - Knowledge

This catalog tracks reusable knowledge assets for the repository.

## Canonical Indexes

| Item | Level | When To Use | Source Of Truth |
| --- | --- | --- | --- |
| Current capability and release gate | MUST | Before claiming completion or release readiness | `docs/release-audit.md` |
| v1 scope boundary | MUST | Before deciding whether a feature is implemented or planned | `docs/v1-scope.md` |
| Remaining feature execution map | MUST | Before starting new implementation work | `docs/plan.md` |
| Operator local migration workflow | SHOULD | When running or documenting current local push migration | `docs/user-migration-guide.md` |
| Recovery and health behavior | SHOULD | When modifying recovery, publish, or health checks | `docs/recovery.md` |
| Control-plane schema map | SHOULD | When changing manifests, receipts, warnings, deleted records, or recovery state | `docs/control-plane.md` |
| Network receiver protocol | SHOULD | When wiring serve/client/resume behavior | `docs/network-protocol.md` |
| Transport and privacy schema | SHOULD | When changing discovery, pairing, device identity, or privacy policy | `docs/transport.md` |

## High-Signal Notes

| Item | Level | When To Use | Source Of Truth |
| --- | --- | --- | --- |
| Scope-truth lesson | MUST | Before status reporting or final answers | `docs/must-authority.md` |
| Warning auditability | MUST | When warning output, audit records, or profile suggestions change | `docs/v1-scope.md` |
| Soft-delete review | MUST | When delete/prune behavior changes | `docs/v1-scope.md` |

## Reusable Query Patterns

| Item | Level | When To Use | Source Of Truth |
| --- | --- | --- | --- |
| Capability recall | MUST | Find implemented/planned split | `docs/must-recall.md` |
| Recovery recall | SHOULD | Find recovery state and safety behavior | `docs/must-recall.md` |
| Transport recall | SHOULD | Find network/privacy plans and limits | `docs/must-recall.md` |
