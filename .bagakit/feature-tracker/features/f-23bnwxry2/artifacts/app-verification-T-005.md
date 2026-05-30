# T-005 App Verification

This artifact records checks for the app-first information architecture and
design system stabilization slice.

## Implementation Evidence

- `ContentView` now renders a role runway for source, target, and observer
  roles before section content.
- `AppSection` declares role-specific availability as available, planned,
  read-only, or role-gated.
- Sidebar rows and top-bar chips expose section availability.
- Pairing, transfer, and drift review pages now avoid presenting unavailable
  role actions as primary live capability.
- Planned and blocked runway cards are non-navigating so they cannot route into
  a generic execution page that looks complete.
- Settings now hides source mutation inputs for target/observer roles, and the
  CLI task panel hides Run/Stop/Open URL controls when the selected role cannot
  execute the selected task.
- Post-commit review fixes keep warning/artifact metrics pending until matching
  evidence exists, broaden evidence review detection across status/report/health
  problem fields, and hide profile creation from target/observer roles.
- Planned native discovery/pairing, sync controls, Merkle/root comparison, and
  packaging readiness remain explicit planned/unavailable boundaries.

## Checks

- `swift build --package-path macos`
  - Result: pass.
- `git diff --check -- macos/SuperMoverApp/ContentView.swift ...`
  - Result: pass.
- `codexL exec --sandbox read-only --ephemeral ...`
  - Result: found P1 role/action mismatches in Control Room and Settings.
    Fixed before gate by role-gating the Dashboard shortcut, selected CLI task
    execution controls, planned/blocked runway navigation, and Settings mutation
    inputs.
- `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-005`
  - Result: pass in `artifacts/gate-T-005-r7-0001.log`.
  - Gate commands passed: `go test -p=1 -count=1 ./...`, `go vet ./...`,
    `go mod tidy -diff`, `git diff --check`,
    `go run ./cmd/supermover help`, and
    `go run ./cmd/supermover version`.
- `gpt-5.5/xhigh` phase review after commit `f73d645`
  - Result: found P1 unsupported green metric/evidence states and P2 profile
    creation affordance mismatch. Fixed in the follow-up review-fix commit.
- T-005 review-fix validation
  - Result: pass. See `artifacts/app-review-fix-T-005.md`.

## Remaining Boundaries

- No native discovery/pairing execution was added.
- No sync queue/run/loop/watch/network controls were added.
- No root-comparison evidence was added.
- No Swift model tests exist yet for role runway or section availability.
