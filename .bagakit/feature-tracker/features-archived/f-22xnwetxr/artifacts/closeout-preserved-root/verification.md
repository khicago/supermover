# Verification Evidence

## Automated Checks

- Command: `go test -count=1 ./internal/driftreview ./internal/cli ./internal/report ./internal/health`
- Result: existing reopen, record, acknowledge/resolve, and report/status health integrations are already covered in current package tests
- Command: `go test -count=1 ./internal/localpush ./internal/driftstore`
- Result: existing automatic refusal-triggered target-drift artifact writes and resolved-record reopen semantics are already covered in current package tests
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/driftstore ./internal/driftreview ./internal/report ./internal/health ./internal/status`
- Result: pass for the current uncommitted `drift expire` slice after moving Go temp/build/cache into repo-local paths to survive low disk space
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli -run 'TestDriftExpireHelpIsHonest|TestDriftRecordHelpIsHonest|TestDriftAcknowledgeHelpIsHonest|TestDaemonStopDoesNotMutateForeignState'`
- Result: pass for current CLI help/status coverage under the same repo-local cache setup
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass for the full repository gate with repo-local Go temp/build/cache relocation
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass for the full repository vet gate under the same repo-local cache setup
- Command: `go mod tidy -diff`
- Result: pass
- Command: `git diff --check`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover help`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover drift expire --help`
- Result: pass

## Manual Checks

- Step: Confirm wording distinguishes existing explicit `drift record` from missing ongoing persistence.
- Outcome: done; README/docs/tracker now distinguish explicit/manual persistence, refusal-triggered persistence, and still-planned ongoing/background durability
- Step: Confirm already-implemented reopen and narrow reconcile behavior are not still counted as missing inside this feature.
- Outcome: done; tracker/docs keep narrow persisted-drift reconcile in `f-233nwduwz` and stop counting it as missing here
- Step: Confirm local push refusal paths already write durable `target_drift` artifacts and preserve/reopen logical findings.
- Outcome: done; tracker/docs now acknowledge explicit `drift record` plus current refusal-triggered durable writes as the wired automatic boundary
- Step: Confirm the new explicit `expired` drift lifecycle is implemented consistently across schema validation, drift review mutation, reopen behavior, and unresolved-report filtering.
- Outcome: done; schema validation, drift review mutation, reopen behavior, unresolved-report filtering, full `./...` tests, vet, and CLI help gates all pass

## Residual Risks

- Drift lifecycle semantics can create duplicate or stale records if identity is
  shallow; implementation should define stable keys before writing mutation code.
- There is still no general background or ongoing durable-persistence loop
  beyond explicit recording and current refusal-triggered artifact writes; that
  future surface should stay separate from this closed drift-evidence slice.
