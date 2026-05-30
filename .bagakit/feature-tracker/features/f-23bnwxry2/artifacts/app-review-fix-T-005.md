# T-005 Review Fix Evidence

This artifact records the follow-up fixes after the first phase review of
commit `f73d645`.

## Findings Addressed

- `gpt-5.5/xhigh` phase review found unsupported green warning/artifact metric
  states before matching evidence exists.
- `gpt-5.5/xhigh` phase review found evidence runway pass states could ignore
  review-worthy target evidence beyond warning/artifact counts.
- `gpt-5.5/xhigh` phase review found the shared profile picker exposed `New`
  in target/observer roles despite those roles needing existing profile
  selection.
- `codexL` side review found multi-snapshot precedence could hide problem
  evidence from another loaded status/report/health snapshot.

## Fixes

- Warning and artifact metrics now render `not checked` with review tint until
  a corresponding structured evidence count exists.
- Warning/artifact count evidence aggregates loaded status/report/health
  snapshots rather than taking the first available snapshot.
- Evidence pass/review state checks every loaded status/report/health snapshot
  and treats warning, drift, prune receipt, recovery, artifact, verification,
  unhealthy, attention, and review states as review-worthy.
- Profile creation is source-only in the picker.
- The Control Room integrity tile now uses status tinting instead of fixed
  green.

## Checks

- `swift build --package-path macos`
  - Result: pass.
- `git diff --check`
  - Result: pass.
- `go test -p=1 -count=1 ./...`
  - Result: pass.
- `go vet ./...`
  - Result: pass.
- `go mod tidy -diff`
  - Result: pass.
- `go run ./cmd/supermover help`
  - Result: pass.
- `go run ./cmd/supermover version`
  - Result: pass.
- `feature-tracker validate-tracker --root .`
  - Result: pass.
