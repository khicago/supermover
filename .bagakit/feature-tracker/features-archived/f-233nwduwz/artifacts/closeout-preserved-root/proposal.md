# Feature Proposal: f-233nwduwz

## Why

Supermover has local recovery, narrow persisted-drift resolution, and a narrow
`reconcile plan/apply` CLI slice over persisted drift records, but v1 still
needs broader repair and reconcile flows that remain auditable and safe before
any target mutation.

## Goal

Extend the current narrow persisted-drift reconcile slice into auditable
planning, explicit application, durable receipts, conflict classes, and
preflight gates. Keep broader automatic/background repair as separately gated
follow-up work after the current safe mutation surface is honest.

## Implemented / Planned / Missing

| Status | Detail |
| --- | --- |
| Implemented | Local recovery, persisted-drift resolve, and narrow `reconcile plan/review/apply` exist. Current reconcile plan and review are non-mutating; apply requires persisted-drift selection intent, explicit `--apply`, and `--reason`; source/target come from the profile SSOT with no `--target` or `--state-dir` override. |
| Implemented narrowly | Persisted missing regular-file restore from matching published/source evidence, plus already-restored or already-absent resolve-noop. `reconcile apply` now writes durable `.supermover/reconcile/receipts/<receipt-id>.json` audit evidence, and report/status surface reconcile receipt counts plus non-applied receipt issues. |
| Implemented narrowly | Reconcile refusals include durable `conflict_class` and `retry_advice` review fields in JSON/text output and receipts. This is operator guidance, not automatic retry. |
| Implemented narrowly | `reconcile review` is a read-only broad repair boundary inventory. It reports persisted plan readiness, live-only detector findings that require `drift record` before selected apply, and planned boundaries for broad background scans, manifest rewrite, broad daemon repair retry/background policy, drift-to-prune handoff, and automatic retry policy. |
| Implemented narrowly | `reconcile apply --all-persisted-planned` is an explicit mutation gate that first reviews durable persisted evidence and selects only currently planned persisted actions. It does not consume live-only detector findings or planned broad boundaries. |
| Implemented narrowly | `reconcile apply --record-live` is an explicit mutation gate that first persists current live detector findings as durable drift records, then applies only the resulting persisted planned actions. It still requires `--apply` and `--reason` and is not background repair. |
| Implemented narrowly | Profile-backed foreground daemon `repair.drift_recording` periodically persists current live detector findings as durable drift review evidence. It records evidence only and does not apply repair, retry reconcile, rewrite manifests, authorize prune, or claim target restoration. |
| Implemented narrowly | Profile-backed foreground daemon `repair.persisted_reconcile_apply` periodically reviews persisted drift records, applies only currently planned persisted actions through the existing reconcile receipt path, and stops after refusals or artifact problems for operator review. It is mutually exclusive with `repair.drift_recording` and does not record live-only drift, consume live-only IDs, rewrite manifests, authorize prune, or implement retry policy. |
| Planned separately | `f-236nwqshz` broad repair retry policy, `f-237nwzbyq` broad repair scan inventory, `f-238nwybkh` reconcile manifest rewrite decisions, `f-239nwv337` repair-to-prune handoff, and `f-23anwj5su` background repair operator UX. |
| Missing in this feature by design | Automatic/background retry policy, broad background repair scans, background live-only repair beyond explicit live-recording gates, manifest rewrite, broad daemon repair retry/background policy, drift-to-prune integration, and broad background repair UX. These are not implemented or claimed by `f-233nwduwz`. |

## Scope

- In scope: broaden the existing selected-ID persisted reconcile slice with durable repair receipts, richer conflict classification, operator retry guidance, read-only boundary review, all-persisted planned selection, explicit live-recording selection, daemon drift review evidence recording, profile-backed daemon persisted planned apply, safety preflight, and explicit follow-up feature splits for remaining broad/background repair work.
- Out of scope: claiming broad automatic reconcile is already complete, automatic destructive target mutation without review, bidirectional sync, profile bypasses such as `--target`/`--state-dir`, and repair based on live-only ephemeral findings.

## Acceptance Criteria

- Operators can produce a reconcile plan without mutating the target.
- Operators can produce a reconcile boundary review without mutating the target.
- Current apply requires persisted-drift selection intent, explicit `--apply`, and `--reason`.
- Operators can choose `--record-live` to persist current live detector findings before applying only the resulting persisted planned actions.
- Operators can enable profile-backed foreground daemon drift recording to persist current live detector findings as durable review evidence without automatic repair.
- Operators can enable profile-backed foreground daemon persisted reconcile apply to apply only already persisted, currently planned actions through existing receipts without consuming live-only drift.
- Apply writes durable repair receipts for selected persisted-drift apply outcomes.
- Conflicts are classified with `conflict_class` and `retry_advice`, and unresolved conflicts do not silently mutate target state.
- The repair flow consumes durable drift evidence rather than transient stdout.
- Remaining broad/background repair surfaces are split into separate follow-up
  features with their own task gates and no implementation claim in this
  feature.

## Transfer Checks

- Preflight every knowable target mutation.
- Keep warning and future repair records durable and auditable.
- Preserve `.supermover` reserved control-plane protections.

## Impact

- Code paths: drift store, current reconcile planner/apply runner, boundary review, durable receipts, report/status.
- Tests: current plan determinism, durable receipt writing, preflight blockers, refusal taxonomy, read-only boundary review, all-persisted planned selection, explicit record-live selection, daemon drift recording evidence, and daemon persisted planned apply behavior exist; automatic/background retry behavior remains missing.
- Rollout notes: do not claim v1 complete or broad automatic repair complete;
  current implementation is narrow persisted drift repair plus explicit
  follow-up planning for the remaining broad repair boundaries.
