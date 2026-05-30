# Feature Proposal: f-238nwybkh

## Title

Reconcile manifest rewrite decisions

## Why

Current reconcile repair restores selected files or resolves no-op cases, but it
does not rewrite published manifests. That is intentional: manifest mutation
changes future truth and must not be hidden inside repair apply.

This feature owns the reviewed decision flow for when, if ever, reconcile
outcomes should create new manifest evidence or supersede old manifest truth.

## Goal

Define and implement reviewed manifest rewrite decision flows for reconcile
outcomes, keeping manifest mutation separate from repair application and
requiring durable evidence, profile SSOT, target preflight, and receipts.

## Principle Layer

- What: explicit manifest rewrite review, approval, and receipt evidence.
- Why: manifest truth affects later drift detection, repair, sync, and prune.
- Intended generalization: manifest rewrites are auditable decisions derived
  from durable scan/repair evidence, not incidental side effects.
- Failure boundary: unsupported drift, missing source evidence, stale target
  state, or ambiguous operator intent produces refusal evidence.
- Behavior examples: propose a manifest update after reviewed target repair,
  refuse rewrite when source evidence is missing, and preserve superseded
  manifest evidence for audit.
- Evidence refs: `f-237nwzbyq` scan evidence, reconcile receipts, published
  manifests, and profile snapshots.

## Scope

- In scope: non-mutating rewrite proposal, explicit approval/apply split,
  manifest receipt evidence, stale-evidence preflight, and docs.
- Out of scope: direct repair apply, automatic retry, prune approval, broad
  repair scan implementation, background UX aggregation, and changing sync
  protocol semantics.

## Acceptance Criteria

- Operators can inspect proposed manifest changes before mutation.
- Manifest mutation requires explicit approval or apply intent and durable
  reason metadata.
- Superseded manifest evidence remains auditable.
- Rewrites never erase unresolved drift or repair refusal evidence.
- Docs distinguish manifest rewrite from target file restoration.

## Transfer Checks

- Do not make repair apply rewrite manifests implicitly.
- Do not treat manifest rewrite as proof that the target was physically
  restored.
- Do not discard old manifest/control-plane evidence.

## Impact

- Code paths: manifest store, reconcile receipts, scan evidence, report/status,
  health/verify consistency checks.
- Tests: proposal determinism, preflight refusal, approval/apply split, receipt
  durability, and stale target/source evidence handling.
- Rollout notes: this feature depends on broad scan inventory evidence and
  remains separate from prune handoff.
