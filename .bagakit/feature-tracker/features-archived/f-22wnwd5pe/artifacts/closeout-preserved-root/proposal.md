# Feature Proposal: f-22wnwd5pe

## Why

Current resume evidence is useful but bounded. V1 still needs a defensible
network recovery acceptance surface before broad interruption recovery can be
claimed.

## Goal

Define and implement acceptance fixtures for bounded network interruption and
resume recovery across kill, restart, receiver crash, and interrupted transfer
cases without claiming arbitrary recovery before evidence exists.

## Implemented / Planned / Missing

| Status | Detail |
| --- | --- |
| Implemented | Bounded same-profile/same-session recovery, in-flight network resume evidence, deterministic `networkrun` source-stop-after-progress retry evidence, command-level receiver listener restart over preserved target state with prior auditable payload-overhead evidence, commit-only retry, published-session retry, and fail-closed missing/bad-prior-evidence cases. |
| Planned | Separate future command/process fixtures for arbitrary process kill, power loss, daemon or OS-service restart recovery, receiver crash UX, network `recover`, automatic retry policy, and broad retry/reconcile. |
| Missing | Broad arbitrary interruption recovery beyond the bounded same-profile/same-session matrix, receiver-side recovery UX, process-level crash/power-loss fixtures, and broad retry/reconcile integration. |

## Scope

- In scope: bounded acceptance matrix, same-profile/same-session fixtures, durable evidence checks, fail-closed unsupported-mode evidence, and docs that bound the claim.
- Out of scope: daemon lifecycle management, watcher-based sync, arbitrary process-kill or power-loss claims without fixtures, network `recover`, broad retry/reconcile, and bidirectional conflict semantics.

## Acceptance Criteria

- A matrix names every supported interruption mode and its expected recovery result.
- Passing fixtures prove resumed transfers do not corrupt target content or receipts.
- Unsupported interruption modes are explicitly reported as blockers or future work.
- Docs avoid saying arbitrary interruption recovery is implemented.

## Transfer Checks

- Keep `.supermover` control-plane safety checks intact.
- Verify target mutations remain preflighted and resumable evidence is durable.
- Avoid runtime overrides that bypass profile SSOT.

## Impact

- Code paths: network push, receiver/session state, recovery/status/report.
- Tests: integration-style failure fixtures plus targeted unit coverage.
- Rollout notes: broad recovery claim remains gated by the acceptance matrix.
