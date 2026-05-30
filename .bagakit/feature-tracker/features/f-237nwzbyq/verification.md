# Verification Evidence

## Automated Checks
- Not started. This feature is proposal-only follow-up evidence from
  `f-233nwduwz/T-009`.
- Honesty check: `go run ./cmd/supermover reconcile scan --help` currently
  fails with `reconcile: unknown subcommand "scan"`. Do not claim scan
  inventory as implemented until the CLI is wired and validated.

## Manual Checks
- Step: Confirm the proposal is read-only scan inventory and not repair apply.
- Outcome: pending future implementation; current CLI exposes
  `reconcile plan`, `reconcile review`, and `reconcile apply`, but not
  `reconcile scan`.

## Residual Risks
- Broad repair scan inventory remains unimplemented until this feature is
  advanced from proposal to implementation.
