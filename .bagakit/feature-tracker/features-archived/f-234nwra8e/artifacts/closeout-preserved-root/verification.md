# Verification Evidence

## Automated Checks

- Command: `go test -count=1 ./internal/cli ./internal/prune ./internal/report`
- Result: covers approval inventory, supersede mutation, prune review/report receipt comparison, and superseded approval release-state reporting
- Command: `go test -count=1 ./internal/status ./internal/report`
- Result: validates compact prune readiness counts plus receipt-linked approval states
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build go test -count=1 ./internal/report ./internal/status ./internal/cli`
- Result: related package regression passed after moving temp output under repo-local `.tmp` because the system temp directory hit `no space left on device`
- Command: `go vet ./...`
- Result: last tracker gate recorded pass for `T-002`
- Command: `go mod tidy -diff`
- Result: last tracker gate recorded pass for `T-002`
- Command: `git diff --check`
- Result: pass on current tracked diff during `T-002` gate

## Manual Checks

- Step: Confirm `prune review` remains read-only and is not listed as missing.
- Outcome: wired and preserved after adding approval inventory and supersede mutation
- Step: Confirm stale approval handling blocks or clearly warns before apply.
- Outcome: wired in report/apply; compact `status` surfaces matching readiness counts
- Step: Confirm approval inventory and supersede mutation do not delete target files or write prune receipts.
- Outcome: covered by CLI tests for `prune approvals` and `prune supersede`
- Step: Confirm approval-to-receipt linkage stays read-only and does not imply new prune authorization.
- Outcome: covered by report/status/cli tests and preserved after supersede mutation wiring

## Residual Risks

- Approval lifecycle can confuse operators if states are too subtle; output
  should use explicit state names and separate review from mutation.
- Compact `status` must stay aggregate-only; do not let it drift into a second
  full inventory surface beside `prune review`.
- The current supersede mutation updates the durable approval artifact in place.
  If future requirements need full review-history replay rather than latest-state
  audit, that should be a separate feature instead of stretching this one.
