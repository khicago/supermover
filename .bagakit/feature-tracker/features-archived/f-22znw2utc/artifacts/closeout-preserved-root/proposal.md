# Feature Proposal: f-22znw2utc

## Why

The receiver can be run manually with `serve`, but v1 agent behavior needs a
managed lifecycle so operators can start, stop, inspect, and recover the agent
without ad hoc process handling.

## Goal

Add managed agent daemon lifecycle surfaces for install, start, stop, status,
logs, and restart behavior around profile SSOT and existing `serve` behavior.

## Implemented / Planned / Missing

| Status | Detail |
| --- | --- |
| Implemented | `serve` runs a manually managed receiver using profile-backed network authentication. |
| Implemented | Foreground `daemon install`, `daemon run --foreground`, `daemon status`, `daemon logs`, `daemon restart`, and `daemon stop` wrap `serve` without alternate runtime configuration. They persist scoped install/state/stop-intent/restart-intent artifacts plus redacted lifecycle events under the profile-selected target control plane. |
| Implemented narrowly | Restart is a pending foreground intent until consumed by a running daemon, then restarts serve listeners in the same foreground process. It is not PID signaling, OS service restart, detached supervision, stale PID liveness proof, or crash recovery. |
| Missing | OS service-manager integration, detached background process management, automatic crash restart, stale PID health proof, LAN browsing, file watcher scheduling, and ongoing sync execution. |

## Scope

- In scope: CLI lifecycle commands, profile-driven daemon config, PID/state/log artifact handling, foreground restart intent behavior, and operator docs/tests.
- Out of scope: OS service managers, detached supervision, crash restart, LAN browsing semantics, file watcher scheduling, sync queueing, and runtime overrides that bypass profile SSOT.

## Acceptance Criteria

- Operators can start, stop, and inspect daemon status through explicit commands.
- Daemon behavior is derived from profile configuration.
- Logs and status are durable enough for audit and troubleshooting.
- Restart behavior is deterministic and documented.

## Transfer Checks

- Profile files remain the SSOT.
- Do not write daemon control data into migrated content paths.
- Keep `.codex` session/runtime/history untouched.

## Impact

- Code paths: CLI command tree, serve startup, process state, config/profile, status/report.
- Tests: command behavior, profile validation, stale PID/log handling, and smoke run where feasible.
- Rollout notes: this feature can unblock LAN advertisement lifecycle and ongoing incremental sync.
