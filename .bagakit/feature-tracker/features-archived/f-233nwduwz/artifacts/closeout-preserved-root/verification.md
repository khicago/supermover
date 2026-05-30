# Verification Evidence

## Automated Checks

- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli -run 'TestReconcileHelpIsHonest|TestReconcileApplyAllPersistedPlanned|TestReconcilePlanAndApplyMissingFileRestore|TestReconcileReviewReports'`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/reconcile ./internal/cli ./internal/report ./internal/status ./internal/verify ./internal/health`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass
- Command: `go mod tidy -diff`
- Result: pass
- Command: `git diff --check`
- Result: pass
- Command: `go run ./cmd/supermover reconcile --help`
- Result: pass
- Command: `go run ./cmd/supermover reconcile apply --help`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli -run 'TestReconcileHelpIsHonest|TestReconcileApplyRecordLive|TestReconcileApplyAllPersistedPlanned|TestReconcilePlanAndApplyMissingFileRestore|TestReconcileReviewReports'`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass
- Command: `go mod tidy -diff`
- Result: pass
- Command: `git diff --check`
- Result: pass
- Command: `go run ./cmd/supermover help`
- Result: pass
- Command: `go run ./cmd/supermover reconcile --help`
- Result: pass
- Command: `go run ./cmd/supermover reconcile apply --help`
- Result: pass
- Command: `go run ./cmd/supermover version`
- Result: pass (`supermover 0.1.0-dev`)
- Command: `feature-tracker.sh run-task-gate --root . --feature f-233nwduwz --task T-006`
- Result: pass (`artifacts/gate-T-006-r10-0001.log`)
- Command: `feature-tracker.sh finish-task --root . --feature f-233nwduwz --task T-006 --result done`
- Result: pass (`f-233nwduwz` returns to `ready` with T-007 still todo)
- Command: `feature-tracker.sh validate-tracker --root .`
- Result: pass
- Command: `feature-tracker.sh show-feature-dag --root .`
- Result: pass (`f-236nwqshz`, `f-237nwzbyq`, and `f-239nwv337` depend on
  `f-233nwduwz`; `f-238nwybkh` depends on scan inventory; `f-23anwj5su`
  depends on retry, scan, manifest rewrite, and prune handoff slices)
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/profile ./internal/cli -run 'TestValidateAcceptsProfileBackedPersistedReconcileApplyRepair|TestValidateRejectsInvalidProfiles|TestWriteReadRoundTrip|TestDaemonHelpIncludesLogsAndRestartWithoutOverclaiming|TestDaemonRunForegroundAppliesProfileBackedPersistedReconcileWithoutLiveRecord|TestDaemonRunForegroundPersistedReconcileApplyDoesNotConsumeLiveOnlyDrift|TestDaemonRunForegroundRecordsProfileBackedLiveDriftWithoutRepair'`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass
- Command: `go mod tidy -diff`
- Result: pass
- Command: `git diff --check`
- Result: pass
- Command: `go run ./cmd/supermover daemon --help`
- Result: pass
- Command: `go run ./cmd/supermover help`
- Result: pass
- Command: `go run ./cmd/supermover version`
- Result: pass (`supermover 0.1.0-dev`)
- Command: `feature-tracker.sh run-task-gate --root . --feature f-233nwduwz --task T-008`
- Result: pass (`artifacts/gate-T-008-r12-0001.log`)
- Command: `feature-tracker.sh finish-task --root . --feature f-233nwduwz --task T-008 --result done`
- Result: pass (`f-233nwduwz` returns to `ready` with T-009 still todo)
- Command: `feature-tracker.sh validate-tracker --root .`
- Result: pass
- Command: `feature-tracker.sh run-task-gate --root . --feature f-233nwduwz --task T-008`
- Result: not rerun after task close (`error: task T-008 must be in_progress before gate`); final supplemental full `go test`, `go vet`, `go mod tidy -diff`, `git diff --check`, daemon help, root help, version, and tracker validation evidence is recorded above.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/profile ./internal/cli -run 'TestValidateAcceptsProfileBackedPersistedReconcileApplyRepair|TestValidateRejectsInvalidProfiles|TestWriteReadRoundTrip|TestDaemonHelpIncludesLogsAndRestartWithoutOverclaiming|TestDaemonRunForegroundAppliesProfileBackedPersistedReconcileWithoutLiveRecord|TestDaemonRunForegroundPersistedReconcileApplyDoesNotConsumeLiveOnlyDrift|TestDaemonRunForegroundRecordsProfileBackedLiveDriftWithoutRepair'`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/profile ./internal/cli -run 'TestValidateAcceptsProfileBackedDriftRecordingRepair|TestValidateRejectsInvalidProfiles|TestWriteReadRoundTrip|TestDaemonHelpIncludesLogsAndRestartWithoutOverclaiming|TestDaemonRunForegroundRecordsProfileBackedLiveDriftWithoutRepair'`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/profile ./internal/cli -run 'TestValidateAcceptsProfileBackedDriftRecordingRepair|TestValidateRejectsInvalidProfiles|TestWriteReadRoundTrip|TestDaemonHelpIncludesLogsAndRestartWithoutOverclaiming|TestDaemonRunForegroundRecordsProfileBackedLiveDriftWithoutRepair|TestDaemonRunForegroundRunsProfileBackedLocalPollingSyncAcrossRestart|TestDaemonRunForegroundRunsProfileBackedNetworkPollingSync'`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass
- Command: `go mod tidy -diff`
- Result: pass
- Command: `git diff --check`
- Result: pass
- Command: `go run ./cmd/supermover daemon --help`
- Result: pass
- Command: `go run ./cmd/supermover help`
- Result: pass
- Command: `go run ./cmd/supermover version`
- Result: pass (`supermover 0.1.0-dev`)
- Command: `feature-tracker.sh run-task-gate --root . --feature f-233nwduwz --task T-007`
- Result: pass (`artifacts/gate-T-007-r11-0001.log`)
- Command: `feature-tracker.sh finish-task --root . --feature f-233nwduwz --task T-007 --result done`
- Result: pass (`f-233nwduwz` returns to `ready` with T-008 still todo)
- Command: `feature-tracker.sh validate-tracker --root .`
- Result: pass
- Command: `feature-tracker.sh run-task-gate --root . --feature f-233nwduwz --task T-005`
- Result: pass (`artifacts/gate-T-005-r9-0001.log`)
- Command: `feature-tracker.sh validate-tracker --root .`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/reconcile`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli -run 'TestReconcileHelpIsHonest|TestReconcileReviewReportsBoundariesWithoutMutation|TestReconcileReviewReportsLiveOnlyDriftAsRecordRequired|TestReconcilePlanAndApplyMissingFileRestore|TestReconcileTextRefusalIncludesConflictClassAndRetryAdvice'`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/reconcile ./internal/cli ./internal/report ./internal/status ./internal/verify ./internal/health`
- Result: pass
- Command: `feature-tracker.sh run-task-gate --root . --feature f-233nwduwz --task T-004`
- Result: pass (`artifacts/gate-T-004-r8-0001.log`)
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass
- Command: `go mod tidy -diff`
- Result: pass
- Command: `git diff --check`
- Result: pass
- Command: `go run ./cmd/supermover reconcile review --help`
- Result: pass
- Command: `go run ./cmd/supermover reconcile --help`
- Result: pass
- Command: `go run ./cmd/supermover help`
- Result: pass
- Command: `go run ./cmd/supermover version`
- Result: pass (`supermover 0.1.0-dev`)
- Command: `feature-tracker.sh validate-tracker --root .`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/reconcile ./internal/cli -run 'TestReconcileHelpIsHonest|TestReconcilePlanAndApplyMissingFileRestore|TestReconcileTextRefusalIncludesConflictClassAndRetryAdvice|TestPlanRefusesScopeMismatchWithoutMutation|TestApplyRequiresIntentAndPreflightRefusesChangedTarget|TestApplyWritesFailedReceiptWhenPreflightRefusesMutation|TestPlanExplicitIDKeepsSelectedMalformedArtifactProblem'`
- Result: pass
- Command: `feature-tracker.sh run-task-gate --root . --feature f-233nwduwz --task T-003`
- Result: pass (`artifacts/gate-T-003-r7-0001.log`)
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go mod tidy -diff`
- Result: pass
- Command: `git diff --check`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover reconcile --help`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover reconcile plan --help`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover reconcile apply --help`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover help`
- Result: pass
- Command: `feature-tracker.sh validate-tracker --root .`
- Result: pass
- Command: `GOTMPDIR=$PWD/.tmp/go-build go test -count=1 ./internal/cli ./internal/reconcile ./internal/verify ./internal/control ./internal/report ./internal/health`
- Result: pass
- Command: `GOTMPDIR=$PWD/.tmp/go-build go test -count=1 ./internal/reconcile ./internal/cli`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -count=1 ./internal/control ./internal/reconcile ./internal/cli ./internal/report ./internal/status`
- Result: pass
- Command: `GOTMPDIR=$PWD/.tmp/go-build go run ./cmd/supermover reconcile --help`
- Result: pass
- Command: `GOTMPDIR=$PWD/.tmp/go-build go run ./cmd/supermover reconcile plan --help`
- Result: pass
- Command: `GOTMPDIR=$PWD/.tmp/go-build go run ./cmd/supermover reconcile apply --help`
- Result: pass
- Command: `git diff --check -- README.md docs/control-plane.md docs/plan.md docs/runbook.md docs/status.md docs/troubleshooting.md docs/user-migration-guide.md internal/cli/cli.go internal/cli/cli_test.go internal/reconcile/reconcile.go internal/reconcile/reconcile_test.go`
- Result: pass
- Command: `feature-tracker.sh run-task-gate --root . --feature f-233nwduwz --task T-001`
- Result: pass (`artifacts/gate-T-001-r4-0001.log`; supplemental reconcile help checks are recorded above because the existing gate template still includes the prior drift-help smoke)
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -count=1 ./internal/reconcile ./internal/report ./internal/status ./internal/cli`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -count=1 ./...`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -race -count=1 ./internal/control ./internal/reconcile ./internal/report ./internal/status ./internal/cli`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass
- Command: `go mod tidy -diff`
- Result: pass
- Command: `git diff --check`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover help`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover reconcile --help`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover reconcile plan --help`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover reconcile apply --help`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover report --help`
- Result: pass
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go run ./cmd/supermover status --help`
- Result: pass
- Command: `feature-tracker.sh validate-tracker --root .`
- Result: pass

## Manual Checks

- Step: Confirm `reconcile plan` is read-only and `reconcile apply` requires selected persisted drift IDs, explicit `--apply`, and `--reason`.
- Outcome: pass, covered by CLI tests and help smoke.
- Step: Confirm current reconcile output is not documented as a durable repair receipt.
- Outcome: superseded by T-002; docs now state that selected-ID `reconcile apply` writes durable `.supermover/reconcile/receipts/<receipt-id>.json` receipts and that `report`/`status` surface receipt counts plus non-applied receipt issues.
- Step: Confirm stale `reconcile --help` wording no longer says apply receipts are not persisted.
- Outcome: pass, covered by `TestReconcileHelpIsHonest` and help smoke.
- Step: Confirm `report --session` does not count reconcile receipts from another session when the receipt-level session is empty but action/refusal evidence is session-scoped.
- Outcome: pass, covered by `TestBuildReportFiltersReconcileReceiptsBySessionEvidence`.
- Step: Confirm reconcile refusals now expose `conflict_class` and
  `retry_advice` in JSON/text output without adding automatic retry behavior.
- Outcome: pass, covered by reconcile package refusal taxonomy assertions,
  `TestReconcileTextRefusalIncludesConflictClassAndRetryAdvice`, and
  `TestReconcileHelpIsHonest`.
- Step: Confirm `reconcile review` is read-only and does not write reconcile
  receipts or mutate the target tree.
- Outcome: pass, covered by `TestReviewBoundariesReportsPersistedPlanWithoutCountingCoveredLiveDriftAsLiveOnly`,
  `TestReviewBoundariesReportsUnpersistedLiveOnlyDriftAsRecordRequired`, and
  `TestReconcileReviewReportsBoundariesWithoutMutation`.
- Step: Confirm `reconcile review` distinguishes persisted plan readiness from
  live-only detector findings, and requires live-only findings to be recorded
  before selected apply.
- Outcome: pass, covered by `TestReconcileReviewReportsLiveOnlyDriftAsRecordRequired`
  and the reconcile package boundary tests.
- Step: Confirm `reconcile review` keeps broad background scans, manifest
  rewrite, broad daemon repair retry/background policy, drift-to-prune handoff,
  and automatic retry policy as planned non-apply-capable boundaries.
- Outcome: pass, covered by boundary assertions in package and CLI tests plus
  help/doc smoke checks.
- Step: Confirm `reconcile apply --all-persisted-planned` applies only durable
  persisted planned reconcile actions and does not consume live-only detector
  findings directly.
- Outcome: pass, covered by `TestReconcileApplyAllPersistedPlannedAppliesOnlyDurablePlannedActions`
  and `TestReconcileApplyAllPersistedPlannedRequiresDurablePlannedActions`.
- Step: Confirm `--all-persisted-planned` cannot be mixed with explicit `--id`
  selection and still requires `--apply` plus `--reason`.
- Outcome: pass, covered by `TestReconcileApplyAllPersistedPlannedRejectsMixedExplicitIDs`,
  `TestReconcileApplyRequiresExplicitSelectionIntentAndReason`, and help smoke.
- Step: Confirm `reconcile apply --record-live` persists current live detector
  findings before apply and then applies only resulting persisted planned
  actions.
- Outcome: pass, covered by
  `TestReconcileApplyRecordLivePersistsThenAppliesLiveOnlyMissingFile`.
- Step: Confirm `--record-live` refuses clean live detector state and cannot be
  mixed with other selection modes.
- Outcome: pass, covered by `TestReconcileApplyRecordLiveRequiresLiveDrift`,
  `TestReconcileApplyRecordLiveRejectsMixedSelection`, and help smoke.
- Step: Confirm profile-backed `repair.drift_recording` lets the foreground
  daemon persist current live detector findings as durable drift review
  evidence without repairing the target.
- Outcome: pass, covered by
  `TestDaemonRunForegroundRecordsProfileBackedLiveDriftWithoutRepair`.
- Step: Confirm profile validation accepts drift recording config, rejects
  missing/negative intervals, and rejects combining target-side drift recording
  with source-side `sync.network_polling`.
- Outcome: pass, covered by
  `TestValidateAcceptsProfileBackedDriftRecordingRepair`,
  `TestValidateRejectsInvalidProfiles`, and `TestWriteReadRoundTrip`.
- Step: Confirm profile-backed `repair.persisted_reconcile_apply` lets the
  foreground daemon apply only persisted planned reconcile actions through
  durable receipts without recording live-only drift.
- Outcome: pass, covered by T-008 gate plus
  `TestDaemonRunForegroundAppliesProfileBackedPersistedReconcileWithoutLiveRecord`
  and
  `TestDaemonRunForegroundPersistedReconcileApplyDoesNotConsumeLiveOnlyDrift`.
- Step: Confirm `repair.persisted_reconcile_apply` remains profile SSOT and
  rejects unsafe combinations or incomplete review metadata.
- Outcome: pass, covered by T-008 gate plus
  `TestValidateAcceptsProfileBackedPersistedReconcileApplyRepair`,
  `TestValidateRejectsInvalidProfiles`, and `TestWriteReadRoundTrip`.
- Step: Confirm daemon persisted reconcile apply cannot be combined with daemon
  drift recording to create an implicit live-only-to-apply loop.
- Outcome: pass, covered by `TestValidateRejectsInvalidProfiles`, profile
  docs, README, runbook, troubleshooting, and status scope text.
- Step: Confirm broad/background repair leftovers are not still counted inside
  `f-233nwduwz`.
- Outcome: pass. `f-233nwduwz/T-009` splits the remaining work into
  `f-236nwqshz` retry policy, `f-237nwzbyq` broad scan inventory,
  `f-238nwybkh` manifest rewrite decisions, `f-239nwv337` repair-to-prune
  handoff, and `f-23anwj5su` background repair operator UX.

## Residual Risks

- Current reconcile repair is intentionally narrow: missing regular-file restore
  from matching published/source evidence plus already-restored or already-absent
  resolve-noop.
- `reconcile review` is read-only boundary evidence. `reconcile apply
  --all-persisted-planned` is a persisted-evidence selection gate only.
  `reconcile apply --record-live` first persists current live detector findings
  and then applies only the resulting persisted planned actions.
  `repair.drift_recording` records daemon-side durable review evidence only and
  does not apply repair. Broader automatic repair, background retry policy,
  background live-only repair beyond explicit live-recording gates, manifest
  rewrite, broad background repair scans, broad daemon repair retry/background
  policy, drift-to-prune integration, and broad background mutation UX remain
  planned.
- T-008 narrows the previous daemon automatic repair bucket to profile-backed
  persisted planned apply only. `repair.persisted_reconcile_apply` consumes
  already persisted planned actions and uses existing receipts, but does not
  record live-only drift, consume live-only IDs, retry in the background,
  rewrite manifests, authorize prune, or implement broad automatic repair. It
  is mutually exclusive with `repair.drift_recording` so live-only detector
  findings cannot become implicit daemon apply input.
- The broad repair follow-up features remain proposal-only until individually
  advanced, gated, and verified. Closing `f-233nwduwz` must not be read as
  completion of automatic/background retry, broad scans, manifest rewrite,
  repair-to-prune handoff, or broad repair UX.
