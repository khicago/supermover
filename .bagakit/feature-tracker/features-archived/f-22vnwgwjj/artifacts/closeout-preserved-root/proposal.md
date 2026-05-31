# Feature Proposal: f-22vnwgwjj

## Why

Traffic privacy level 2 has current-path implementation evidence, but release
acceptance is still too implicit. Operators need a repeatable way to prove what
level 2 applies, what overhead was observed, and what leakage remains.

## Goal

Add repeatable release acceptance evidence and operator-facing summary for
traffic privacy level 2 on the current profile-backed network path, explicitly
documenting residual metadata leakage and that level 2 is bounded traffic-shape
protection, not anonymity.

## Implemented / Planned / Missing

| Status | Detail |
| --- | --- |
| Implemented | Level 2 padding, batching, jitter, and applied-overhead evidence exist for the profile-backed network transfer path. |
| Planned | Acceptance matrix and release summary that consume the existing evidence without widening the privacy claim. |
| Missing | Repeatable fixtures, CLI/report release summary, docs language that rejects anonymity, and gate evidence for representative transfer sizes. |

## Scope

- In scope: acceptance harness or command/report path, fixture coverage, overhead bounds, residual leakage wording, and release gate evidence.
- Out of scope: anonymity, onion routing, endpoint hiding, traffic analysis resistance beyond bounded shaping, and support for unwired transports.

## Acceptance Criteria

- Operators can run a repeatable check for level 2 on the profile-backed network path.
- Output records padding/batching/jitter configuration and observed overhead.
- Output explicitly says level 2 is not anonymity.
- Tests cover configured, disabled, and evidence-missing cases.

## Transfer Checks

- Keep profile files as SSOT.
- Do not add runtime overrides that bypass profile privacy settings.
- Do not imply protection for LAN browsing, daemon, or ongoing sync until those paths are wired.

## Impact

- Code paths: network transfer evidence, report/status or a focused acceptance command.
- Tests: unit fixtures plus at least one end-to-end smoke on the current network path.
- Rollout notes: release wording must separate traffic-shape protection from anonymity.
