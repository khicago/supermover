# T-003 App Verification

This artifact records checks and inspection evidence for role-scoped process
supervision in the native macOS workbench.

## Implementation Evidence

- `SupervisedProcessSlot` defines independent slots for bounded foreground
  actions, target serve, and target dashboard.
- `AppStore` stores active runs and `ProcessController` instances by slot, so
  starting a bounded action does not terminate target serve or dashboard.
- Starting a process in an already-running slot is refused with an operator
  note instead of silently replacing the process.
- Stop actions address a named slot and record a lifecycle event before sending
  process termination.
- Dashboard URL reveal reads from the target dashboard slot, not whichever run
  happens to be focused.
- Durable daemon status/logs remain evidence readers; active liveness comes
  only from app-owned foreground process slots.

## Checks

- `swift build --package-path macos`
  - Result: pass.
- `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-003`
  - Result: pass in `gate-T-003-r5-0001.log`.

## Review-Fix Checks

- `swift build --package-path macos`
  - Result: pass after stale-context supervision fix.
- `feature-tracker validate-tracker --root .`
  - Result: pass after stale-context supervision fix.
- `git diff --check`
  - Result: pass after stale-context supervision fix.
- `swift build --package-path macos`
  - Result: pass after focused-run stale detail fix.
- `feature-tracker validate-tracker --root .`
  - Result: pass after focused-run stale detail fix.
- `git diff --check`
  - Result: pass after focused-run stale detail fix.

## Remaining Boundaries

- `serve` and `dashboard` readiness remains log-derived until T-004 adds
  structured app events.
- Discovery advertise, daemon foreground run, sync loop, sync watch, and sync
  network loop are still future command surfaces for later tasks.
- A running foreground process whose profile, listen address, role, or other
  setup inputs no longer match the current app context is labeled stale and
  remains stoppable, but it is not green current liveness in slot cards or the
  focused run detail.
