# Verification Evidence

## Automated Checks

- Command: `GOTMPDIR=$PWD/.tmp/go-build go test -count=1 ./internal/cli -run 'TestPushNetworkReceiverRestartAcceptanceMatrix|TestPushNetworkBlocksReceiverResumeWithoutPriorPayloadEvidence|TestRecoverHelpStatesNetworkRecoveryIsUnwired'`
- Result: passed
- Evidence: command-level receiver listener restart over preserved target state resumes the same profile/session from receiver status, published retry uploads no chunks, missing prior payload-overhead evidence blocks as `needs_repair` / `payload_overhead_missing`, and `recover --help` states network recovery is unwired.
- Command: `GOTMPDIR=$PWD/.tmp/go-build go test -count=1 ./internal/networkpush -run 'TestRunPreservesPayloadEvidenceWhenRetryOnlyCommits|TestRunPreservesZeroBytePayloadEvidenceWhenRetryOnlyCommits|TestRunFailsClosedWhenPublishedRetryCannotReadPayloadOverhead|TestRunFailsClosedWhenPublishedRetryHasBadPriorPayloadEvidence'`
- Result: passed
- Evidence: commit-only retry, zero-byte commit-only retry, published retry without prior evidence, and bad-prior-evidence refusal.
- Command: `GOTMPDIR=$PWD/.tmp/go-build go test -count=1 ./internal/networkrun -run 'TestRunResumesAfterSourceStopWithInFlightProgressEvidence|TestRunFailsClosedWhenFailedRetryLacksPriorPayloadEvidence|TestRunFailsClosedWhenRemoteFailedRetryHasNoPriorArtifact|TestRunWritesInFlightProgressEvidenceBeforeClientCompletion'`
- Result: passed
- Command: `GOTMPDIR=$PWD/.tmp/go-build go test -count=1 ./internal/protocolclient -run 'TestClientRunResumesInterruptedUploadFromReceiverStatus|TestClientRunResumesAfterInterruptedMultiRecordBatch|TestClientRunTreatsAlreadyPublishedSessionAsComplete|TestClientRunRetriesStagedSessionCommitWithoutChunks'`
- Result: passed
- Command: `GOTMPDIR=$PWD/.tmp/go-build go test -count=1 ./internal/receiver -run 'TestFileStoreCommitRecoversAfterPublishBeforeReceipt|TestFileStoreZeroByteCommitRecoversAfterPublishBeforeReceipt|TestFileStorePublishedSessionReplayIsIdempotentAndRejectsChunks'`
- Result: passed
- Command: `GOTMPDIR=$PWD/.tmp/go-build go test -count=1 ./internal/cli ./internal/networkpush ./internal/networkrun ./internal/protocolclient ./internal/receiver ./internal/receiverserve ./internal/report ./internal/health`
- Result: passed
- Command: `GOTMPDIR=$PWD/.tmp/go-build go run ./cmd/supermover recover --help`
- Result: passed
- Command: `go test -count=1 ./...`
- Result: planned for implementation feature
- Command: `GOTMPDIR=$PWD/.tmp/go-build go vet ./...`
- Result: passed
- Command: `GOTMPDIR=$PWD/.tmp/go-build go mod tidy -diff`
- Result: passed
- Command: `git diff --check -- README.md docs/network-protocol.md docs/runbook.md docs/release-audit.md docs/v1-scope.md docs/plan.md docs/troubleshooting.md internal/cli/cli.go internal/cli/cli_test.go`
- Result: passed

## Manual Checks

- Step: Confirm f-22wnwd5pe/T-001 docs describe only deterministic `networkrun`
  source-stop-after-progress retry evidence.
- Outcome: passed; docs keep broad arbitrary interruption, daemon restart, and
  process-kill recovery as planned/unwired.
- Step: Confirm every recovery claim maps to a passing interruption fixture.
- Outcome: passed for the bounded T-002 matrix; current support maps to CLI receiver-restart/missing-prior/source-interruption tests, networkpush commit-only/published retry and bad-prior tests, networkrun source-stop-after-progress and prior-evidence tests, protocolclient receiver-status resume tests, and receiver publish-window recovery tests.
- Step: Confirm unsupported interruption modes remain documented as blockers.
- Outcome: passed; docs keep arbitrary process kill, power loss, daemon/OS-service restart recovery, network `recover`, broad retry policy, broad reconcile, and receiver crash UX outside the current acceptance claim.

## Residual Risks

- The current receiver restart fixture restarts the receiver listener in-process
  over preserved target state. It is command-level evidence for listener restart
  and same-session retry, not OS process crash, power loss, or daemon
  supervision evidence.
- Process and network interruption tests can be flaky; keep deterministic
  state-machine tests separate from slower smoke fixtures.
