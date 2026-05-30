# T-004 App Verification

This artifact records checks for the structured app event and artifact reader
base.

## Implementation Evidence

- `AppEvent` records app-owned structured events with severity, timestamp,
  title, and detail.
- `ArtifactReadProblem` records missing or malformed structured command output,
  including artifact kind, task, problem text, and raw stdout sample.
- `captureStructuredResult` no longer silently ignores JSON decode failures for
  structured command surfaces.
- Structured command completion now skips snapshot promotion for cancellation,
  non-zero exits with no structured stdout, and non-zero mutating command exits.
- Valid JSON from read-only review-required exits may still be promoted as
  review evidence rather than success evidence.
- Evidence view exposes structured app events and artifact reader problems.
- Stale structured output is not promoted when a command finishes for an older
  setup context.

## Checks

- `swift build --package-path macos`
  - Result: pass.
- `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-004`
  - Result: pass in `gate-T-004-r6-0001.log`.

## Remaining Boundaries

- Serve/dashboard readiness remains compatibility text until a structured event
  source is added.
- Transfer progress, ETA, throughput, and Merkle/root comparison remain
  unavailable until later task surfaces provide durable evidence.
- Raw non-UTF-8 stdout bytes are not preserved by the current app process
  adapter; structured command output is treated as JSON/UTF-8.
