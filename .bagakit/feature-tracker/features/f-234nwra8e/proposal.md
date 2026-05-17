# Feature Proposal: f-234nwra8e

## Why

The prune workflow now has dry-run, approval authoring, read-only review, apply,
inventory, and receipts. The remaining release workflow gap is approval
lifecycle and release readiness, not the already implemented `prune review`
command.

## Goal

Complete the prune release workflow with approval inventory, supersede
mutation, compact release-readiness summaries, and approval-to-receipt
comparison while keeping target mutation gated behind existing approval/apply
paths.

## Implemented / Planned / Missing

| Status | Detail |
| --- | --- |
| Implemented | `prune --dry-run`, `prune approve`, `prune approvals`, `prune review`, `prune supersede`, `prune --apply`, approval inventory, stale/expired/consumed/superseded classification, receipt inventory, approval-to-receipt linkage, and compact status readiness counts exist. |
| Planned | Broader release acceptance gates or more elaborate approval-history surfaces can build on the current durable approval artifacts later. |
| Missing | No additional missing scope inside this feature; broader release gates and any future approval-history redesign should be tracked separately instead of inflating this slice. |

## Scope

- In scope: approval list/supersede mutation, receipt comparison, compact readiness counts, and release-oriented docs.
- Out of scope: changing physical prune safety gates, bypassing approval artifacts, or reimplementing read-only `prune review`.

## Acceptance Criteria

- Operators can inspect active, stale, superseded, consumed, and receipt-attention approval states.
- Operators can list current-scope approval artifacts and supersede an existing approval without deleting target files or writing prune receipts.
- Stale approvals are blocked or clearly marked before apply.
- Release readiness output separates blockers, warnings, approvals, receipts, and compact status counts.
- Existing `prune review` remains read-only.

## Transfer Checks

- Soft deletes remain easy to inspect before physical pruning.
- `prune --apply` continues to require valid approval artifacts.
- Approval lifecycle changes are durable and auditable.

## Impact

- Code paths: prune approval store, CLI, report/status, receipt comparison, and tracker/docs truth.
- Tests: stale/superseded approval fixtures, approval list/supersede CLI coverage, receipt-linked approval states, compact readiness counts, no-mutation review.
- Rollout notes: builds on archived feature `f-22unwv5qs` and commit `d938d40`.
