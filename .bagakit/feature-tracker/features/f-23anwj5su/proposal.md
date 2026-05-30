# Feature Proposal: f-23anwj5su

## Title

Background repair operator UX

## Why

Once retry policy, scan inventory, manifest rewrite, and prune handoff are
separate evidence streams, operators need a clear UX that explains what is
recorded, what was retried, what mutated the target, what changed manifest
truth, and what still requires review.

This feature is deliberately late in the DAG. UX aggregation should reflect
wired behavior, not advertise planned repair capability before implementation.

## Goal

Define and implement operator-facing status, report, runbook, and review
surfaces for future background repair activity, separating evidence recording,
retry, mutation, manifest rewrite, and prune handoff states.

## Principle Layer

- What: operator-facing summary and review surfaces for background repair
  activity.
- Why: broad repair becomes unsafe if operators cannot distinguish evidence,
  retries, mutations, receipts, unresolved refusals, manifest changes, and
  prune handoffs.
- Intended generalization: every repair-related state maps to a durable
  artifact and a next safe action.
- Failure boundary: unknown, stale, partial, or unsupported states are shown as
  review-required, not success.
- Behavior examples: status shows retry stopped for review, report links
  manifest rewrite receipts, runbook explains handoff to prune, and review
  output lists safe next commands.
- Evidence refs: `f-236nwqshz`, `f-237nwzbyq`, `f-238nwybkh`,
  `f-239nwv337`, and current `f-233nwduwz` receipts.

## Scope

- In scope: status/report/review aggregation, runbook/troubleshooting updates,
  help honesty, JSON/text output fields, and acceptance checks.
- Out of scope: implementing retry policy, broad scan inventory, manifest
  rewrite, prune handoff, LAN/network discovery, or sync transport behavior.

## Acceptance Criteria

- Operator surfaces distinguish recorded evidence, planned actions, applied
  mutations, retries, refusals, manifest decisions, and prune handoffs.
- JSON/text output gives deterministic next-action categories.
- Docs state current wired behavior versus planned behavior.
- No UX wording claims broad repair success unless matching code and evidence
  are wired.
- Verification includes help smoke, report/status tests, and tracker/docs
  consistency checks.

## Transfer Checks

- Do not collapse multiple safety states into a single "healthy" or "repaired"
  label.
- Do not hide non-applied receipts, stale approvals, or unresolved drift.
- Do not count UX presence as feature implementation for underlying repair
  behaviors.

## Impact

- Code paths: report, status, reconcile review, health/verify if surfaced,
  runbook/troubleshooting, README.
- Tests: output contract, stale/partial states, help honesty, and doc/tracker
  alignment.
- Rollout notes: this feature should run after the underlying repair slices
  exist or explicitly document any still-planned surface.
