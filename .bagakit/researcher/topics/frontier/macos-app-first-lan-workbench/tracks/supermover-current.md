# supermover-current

## Track Contract

- track id: `supermover-current`
- parent pass: `pass-001`
- parent charter: `charter.md`

## Track Question

current CLI/app wiring and evidence gaps

## Required Source Types
- Current feature tracker proposal/tasks.
- Current CLI help/code surfaces.
- Current Swift app task/process implementation.

## Preferred Sources
- `f-23bnwxry2` proposal/tasks/verification.
- `internal/cli/cli.go`.
- `macos/SuperMoverApp/AppStore.swift`.
- Manual CLI help smoke output when useful.

## Disallowed Sources
- Design docs without verifying wired command behavior.
- Mock UI state as implementation evidence.

## Source Id Range

`sm01`-`sm09`

## Owned Output Files
- tracks/supermover-current.md

## Minimum Evidence

At least one plan artifact and two implementation references before changing a
task boundary.

## Lead Policy

Open a follow-up lead for any tracker inconsistency that could cause false
completion claims.

## Drift Check

Every proposed app screen must map to an existing CLI command/API or be called
out as a new implementation dependency.

## Merge Notes
- Result: app task coverage, structured events, and multi-process supervision
  are blocking plan gaps.
