# Feature Proposal: f-239nwv337

## Title

Repair to prune handoff

## Why

Repair and prune are different safety boundaries. Current prune workflows have
approval and receipt evidence, while current reconcile repair has selected
persisted drift receipts. No wired flow should let a repair finding or receipt
implicitly authorize physical deletion.

This feature owns the reviewed handoff between repair/drift evidence and prune
approval inputs.

## Goal

Define and implement the reviewed handoff between durable drift or repair
evidence and prune approval workflows without letting repair implicitly
authorize destructive prune.

## Principle Layer

- What: explicit handoff evidence from drift/repair review into prune
  candidate or approval review.
- Why: deletion requires stronger operator review than repair planning.
- Intended generalization: prune decisions can reference repair/drift evidence
  without inheriting its approval state.
- Failure boundary: ambiguous drift state, stale repair receipts, unresolved
  conflicts, or missing prune dry-run evidence blocks handoff.
- Behavior examples: show that a repair finding may require prune review,
  refuse handoff for unresolved drift, and require a fresh prune approval before
  physical deletion.
- Evidence refs: `f-233nwduwz` reconcile receipts, `f-234nwra8e` prune approval
  and release workflow, persisted drift records.

## Scope

- In scope: read-only handoff review, durable handoff artifacts if needed,
  prune-candidate linkage, freshness checks, and docs.
- Out of scope: physical prune implementation changes unless explicitly gated,
  automatic prune approval, broad scan implementation, manifest rewrite, retry
  policy, and background UX aggregation.

## Acceptance Criteria

- Handoff output is review evidence, not prune authorization.
- Physical prune still requires existing approval/apply semantics.
- Stale, unresolved, or conflicting repair/drift evidence blocks handoff.
- Report/status/runbook text separates repair completion from prune readiness.
- Tests cover refusal to infer deletion approval from repair evidence.

## Transfer Checks

- Do not let reconcile receipts approve prune.
- Do not hide soft-delete inspection windows.
- Do not bypass current prune dry-run, approval, receipt, or supersede controls.

## Impact

- Code paths: drift records, reconcile receipts, prune review/approval,
  report/status, runbook/troubleshooting.
- Tests: stale evidence, unresolved drift, approval separation, and receipt
  linkage.
- Rollout notes: depends on current prune workflow and narrow reconcile receipt
  evidence.
