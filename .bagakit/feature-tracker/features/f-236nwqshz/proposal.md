# Feature Proposal: f-236nwqshz

## Title

Broad repair retry policy

## Why

`f-233nwduwz` intentionally stops after narrow persisted reconcile apply
receipts and daemon persisted planned apply. Refusals include
`conflict_class` and `retry_advice`, but there is still no automatic or
background retry policy that decides when it is safe to try again.

Retry deserves its own feature because retrying mutation is a safety boundary,
not a formatting detail. A retry loop must consume durable evidence, respect
profile SSOT, preserve receipts, and stop on uncertainty instead of converting
operator guidance into silent target mutation.

## Goal

Define and implement automatic/background repair retry policy over durable
persisted drift records and reconcile receipts, without widening the current
narrow apply surface into broad scans, manifest rewrite, prune handoff, or
operator UX.

## Principle Layer

- What: retry policy for previously refused, partial, or pending persisted
  repair actions.
- Why: operators need repeatable retry behavior, but retries must not hide
  target conflicts or stale evidence.
- Intended generalization: a retry decision can be audited from profile,
  persisted drift records, reconcile receipts, conflict class, retry advice,
  target preflight, and fresh plan evidence.
- Failure boundary: if the policy cannot prove retry safety, it records or
  reports a refusal and waits for operator review.
- Behavior examples: retry a transient mutation failure only after fresh
  preflight still matches the selected durable action; refuse unsupported drift,
  source evidence loss, scope mismatch, or missing receipt history.
- Evidence refs: `f-233nwduwz` receipts, refusal taxonomy, and daemon persisted
  planned apply evidence.

## Scope

- In scope: retry eligibility model, backoff limits, receipt lineage, terminal
  refusal states, profile-backed configuration, daemon foreground integration
  if explicitly configured, tests, and docs.
- Out of scope: broad background scans, live-only detector persistence, manifest
  rewrite, drift-to-prune handoff, richer operator UX, bidirectional sync, LAN
  discovery, and target mutation without selected durable evidence.

## Acceptance Criteria

- Retry inputs are durable persisted drift records and reconcile receipts, not
  transient stdout or live-only detector IDs.
- Retry policy is profile-backed, bounded, and auditable.
- Every retry attempt writes durable receipt evidence or a durable refusal
  reason.
- Retry never bypasses target preflight, profile SSOT, selected durable action
  checks, or existing `.supermover` path protections.
- Docs and help distinguish retry policy from broad automatic repair.

## Transfer Checks

- Do not treat `retry_advice` strings as sufficient authorization.
- Do not retry destructive or unsupported actions without an explicit reviewed
  policy and fresh preflight.
- Preserve started/final receipt evidence across interruption.

## Impact

- Code paths: reconcile receipts, refusal taxonomy, profile validation, daemon
  foreground worker scheduling, report/status evidence.
- Tests: retry eligibility, refusal terminal states, receipt lineage,
  interruption evidence, and daemon stop/restart behavior.
- Rollout notes: this feature still does not implement broad scans, manifest
  rewrite, prune handoff, or broad background repair UX.
