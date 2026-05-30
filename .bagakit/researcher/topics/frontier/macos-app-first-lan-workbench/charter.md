# Topic Charter

## Core Question

What plan changes are needed before implementing Supermover's native macOS app-first LAN migration workbench?

## Decision Or Downstream Use

Revise `f-23bnwxry2` before implementation so the native app-first LAN
workbench is sliced by wired Supermover capability, macOS deployment reality,
and evidence-backed acceptance gates.

## Output Shape

Research source cards, source-bound summaries, claim ledger entries, one
cross-source insight, and a concrete feature-plan review.

## In Scope
- Current feature tracker plan and dependencies.
- Current CLI and macOS app command surfaces.
- macOS app permissions, local-network, signing, and distribution constraints.
- Comparable sync/backup UX patterns that affect plan sequencing.

## Out Of Scope
- Replacing the Supermover evidence model with a competitor product model.
- Designing new cryptographic protocol behavior beyond existing feature scope.
- Implementing the app feature during this review pass.

## Source Priority
- Repo-local wired code/help and feature tracker artifacts.
- Apple official docs for platform constraints.
- Official docs from comparable sync/backup products.
- Secondary commentary only if primary sources leave a gap.

## Evidence Threshold

At least one local implementation source, one local plan source, and two
external primary sources before changing the implementation plan.

## Stop Conditions
- Stop after the plan changes become clear enough to reorder tasks and define
  implementation gates.
- Defer deeper product benchmarking unless it changes a task boundary.

## Drift Sentinels
- Do not treat CLI roadmap copy as wired app behavior.
- Do not treat visual polish as evidence of transfer safety.
- Do not require Merkle/root comparison unless a wired artifact/API exists.
