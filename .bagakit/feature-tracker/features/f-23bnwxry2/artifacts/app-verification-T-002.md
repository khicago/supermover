# T-002 App Verification

This artifact records checks that are outside the default non-UI tracker gate.

## Checks

- `swift build --package-path macos`
  - Result: pass.
  - Scope: native macOS app package build.
- `go test -count=1 ./...`
  - Result: pass.
  - Scope: full Go package test suite, used to reproduce and clear the earlier
    gate failure.
- `feature-tracker validate-tracker --root .`
  - Result: pass.
  - Scope: feature-tracker schema and projection validity.

## Review-Fix Focus

- Setup readiness must be scoped to the current profile/root/identity context.
- Observer role must remain read-only; source and target roles must enforce the
  role contract before launching CLI tasks.
- Long-running serve and dashboard actions must remain blocked until T-003/T-004.

## Review-Fix Checks

- `swift build --package-path macos`
  - Result: pass after review fixes.
- `go vet ./...`
  - Result: pass after review fixes.
- `go mod tidy -diff`
  - Result: pass after review fixes.
- `go run ./cmd/supermover help`
  - Result: pass after review fixes.
- `go run ./cmd/supermover version`
  - Result: pass after review fixes.
- `feature-tracker validate-tracker --root .`
  - Result: pass after review fixes.
- `go test -count=1 ./internal/cli -run TestDaemonRunForegroundRunsProfileBackedLocalPollingSyncAcrossRestart -v`
  - Result: pass after one earlier full-suite run hit this daemon readiness test.
- `go test -count=1 ./...`
  - Result: pass on rerun after the daemon readiness test passed in isolation.
