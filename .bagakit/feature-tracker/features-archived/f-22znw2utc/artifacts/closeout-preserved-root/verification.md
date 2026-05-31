# Verification Evidence

## Automated Checks

- Command: `GOTMPDIR=$PWD/.tmp/go-build go test -count=1 ./internal/agentdaemon ./internal/control ./internal/cli`
- Result: pass
- Command: `GOTMPDIR=$PWD/.tmp/go-build go run ./cmd/supermover daemon --help`
- Result: pass
- Command: `GOTMPDIR=$PWD/.tmp/go-build go run ./cmd/supermover daemon logs --help`
- Result: pass
- Command: `GOTMPDIR=$PWD/.tmp/go-build go run ./cmd/supermover daemon restart --help`
- Result: pass
- Command: `git diff --check -- README.md docs/control-plane.md docs/plan.md docs/runbook.md docs/status.md docs/troubleshooting.md docs/v1-scope.md internal/agentdaemon/daemon.go internal/agentdaemon/daemon_test.go internal/cli/cli.go internal/cli/cli_test.go internal/control/control.go internal/control/control_test.go`
- Result: pass

## Manual Checks

- Step: Confirm daemon lifecycle does not introduce runtime overrides outside the profile.
- Outcome: pass; daemon commands derive target and runtime material from the selected profile and expose no endpoint/TLS/privacy runtime overrides.
- Step: Confirm restart semantics are foreground-only and do not claim OS service restart, PID signaling, stale PID liveness, or crash recovery.
- Outcome: pass; `daemon restart` writes pending restart-intent evidence and a running foreground daemon may consume it to restart serve listeners in the same process.
- Step: Confirm lifecycle logs avoid raw stderr and pairing verification codes.
- Outcome: pass; durable lifecycle events are redacted and tests cover forbidden raw stderr/pairing-code material.

## Residual Risks

- Stale `running` state remains persisted evidence, not a live-health proof.
  Future work should add explicit liveness/heartbeat/stale classification.
- Cross-platform OS service managers, detached process supervision, automatic
  crash restart, LAN browsing, file watching, and ongoing sync remain missing.
