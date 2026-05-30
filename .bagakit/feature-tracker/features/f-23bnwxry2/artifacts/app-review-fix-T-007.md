# T-007 Review Fix: Sync Foreground Control Hardening

## Scope

This artifact records the post-commit review fix for T-007 sync
queue/run/loop/watch/network execution controls.

The fix keeps the T-007 behavior foreground-only and CLI-backed. It does not
implement detached background sync, automatic endpoint selection, root
comparison, packaging, or evidence-vault actions.

## Findings Addressed

- Development runtime no longer launches `go run ./cmd/supermover ...` as the
  supervised process. When no packaged CLI binary is present, the app starts a
  supervised build-and-exec launcher that builds `.tmp/macos-app/supermover-dev`
  and then `exec`s that binary in the same foreground process. This keeps the UI
  responsive during build, gives Stop a supervised process immediately, and
  asks the active build child to stop during build-phase termination. After
  build succeeds, Stop targets the actual foreground SuperMover process.
- `ProcessController` now drains residual stdout/stderr before invoking exit
  handling and passes a complete output snapshot into structured artifact
  promotion. Late readability-handler updates are ignored after the run is no
  longer `.running`, preventing duplicate completed-output appends.
- Discovery-gated sync no-match JSON with zero-value `enqueue`, `run`, and
  `network` fields no longer renders an executed run panel. The app only treats
  discover-run as executed when the gate status is `matched` and the decoded run
  carries non-empty session/status evidence.
- Sync-specific inputs now clear promoted sync snapshots when changed, including
  queue entry id, session prefix, retry/interval/max settings, watch settings,
  discovery listen/timeout, and operator reason.

## Tests Added

- `testDiscoverRunNoMatchWithZeroValuePayloadDoesNotCountAsExecutedRun`
- `testDevelopmentCLIResolverUsesBuildThenExecLauncher`
- `testSyncInputChangesClearPromotedSyncSnapshots`

## Validation

Review-fix checks:

- `swift build --package-path macos`
- `swift test --package-path macos`
- `go mod tidy -diff`
- `git diff --check`
- `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- `go run ./cmd/supermover help`
- `go run ./cmd/supermover version`
- `feature-tracker validate-tracker --root .` through the installed Bagakit
  feature tracker script
- build-and-exec launcher smoke with the TERM trap returned
  `supermover 0.1.0-dev`
- stop-during-build trap simulation returned `trap status=143 child_stopped=<pid>`

`run-task-gate` was not rerun because T-007 had already been marked `done` and
the tracker requires the task to be `in_progress` before accepting another gate
run.
