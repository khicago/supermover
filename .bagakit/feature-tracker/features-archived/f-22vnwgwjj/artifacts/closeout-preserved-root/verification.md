# Verification Evidence

## Automated Checks

- Command: `go test -count=1 ./internal/cli ./internal/network ./internal/report`
- Result: planned for implementation feature
- Command: `go vet ./...`
- Result: planned for implementation feature
- Command: `go mod tidy -diff`
- Result: planned for implementation feature
- Command: `git diff --check`
- Result: planned for implementation feature

## Manual Checks

- Step: Confirm level 2 output says traffic-shape protection is not anonymity.
- Outcome: planned for implementation feature
- Step: Confirm release acceptance blocks when overhead evidence is missing.
- Outcome: planned for implementation feature

## Residual Risks

- Network timing evidence can be noisy; acceptance should use bounded,
  deterministic fixtures where possible and keep measured claims narrow.
