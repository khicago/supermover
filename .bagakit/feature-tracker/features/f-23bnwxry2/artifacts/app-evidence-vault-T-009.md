# T-009 Evidence Vault: Browser And Safe Actions

## Scope

This artifact records the implementation evidence for T-009.

Implemented in this slice:

- The Evidence screen is now an `Evidence Vault` surface with:
  - typed cards for `verify`, `status`, `report`, and `health`
  - raw structured stdout envelopes with task, exit code, freshness, and stderr
    sample metadata
  - envelope history, so stale completions and prior read surfaces remain
    auditable instead of disappearing from the vault
  - an explicit alignment-scope panel that says the wired truth is
    target-vs-published-manifest evidence, not current-source comparison
- Read-only structured commands no longer clear unrelated typed evidence on
  launch. Running `Status`, then `Report`, then `Health`, then `Verify` can
  accumulate multiple current evidence cards for the same setup context.
- Mutating or trust-changing commands still stale all promoted structured
  evidence before launch because target/control-plane truth may change.
- Non-zero JSON review exits are retained as review evidence and card facts now
  expose the CLI exit code instead of hiding it in raw output only.
- `drift record --format json` is captured as structured review evidence,
  including non-zero review exits that still emit JSON.
- Evidence-bound next actions now refuse review-metadata command previews or
  execution unless the selected drift, queue, soft-delete, or approval id is
  resolved from current loaded evidence rather than free-form text alone.
- The app has a searchable target `.supermover` Artifact Catalog covering
  profiles, pairings, sessions, network transfers, warnings, soft deletes,
  drift records, prune approvals/receipts, reconcile receipts, daemon
  artifacts/events, incremental-sync queue/run artifacts, agent influence,
  history, recovery, and unknown control-plane files.
- The catalog is explicitly a manual read of the selected Target Root field,
  not profile-derived target proof. Profile-backed `verify`/`status`/`report`
  remain the stronger target truth.
- Artifact catalog scanning preserves hidden files and dot-directories, refuses
  symlinks instead of following them, caps raw preview reads, surfaces malformed
  JSON as catalog problems, and keeps unknown control artifacts visible.
- Evidence Vault can run only bounded review-metadata actions from loaded
  evidence: `drift record`, `drift acknowledge`, `drift resolve`,
  `drift expire`, `sync queue cancel`, `sync queue fail`, `prune approve`, and
  `prune supersede`.
- `sync queue` skipped rows are deliberately read-only and cannot satisfy
  durable queue-entry verification for cancel/fail actions.
- Vault-side `prune approve` is single-candidate only: it requires exactly one
  loaded prune-review candidate soft-delete id, an explicit approval id, reason,
  and reviewer. Prune refusals, arbitrary verify soft-delete rows, and
  multi-id typed input cannot unlock approval execution.
- Raw JSON envelopes are pretty-printed when possible and include byte/line
  counts plus truncation state.
- Card fact rendering prioritizes identity plus highest-severity facts, so a
  red or amber card shows why it needs review.

Not implemented in this slice:

- Prune apply, reconcile apply, pairing, publish, network push, or sync
  execution from evidence cards. These mutate target files, transfer data, or
  change trust/transport state and remain outside Evidence Vault execution.
- Merkle tree construction, Merkle root comparison, or current-source tree
  comparison.

## Review Inputs

Three independent `gpt-5.5/xhigh` read-only reviews informed the slice:

- Correctness and robustness review found the original launch behavior erased
  unrelated evidence cards, and flagged non-zero JSON exits plus stale
  completions as evidence-retention gaps.
- Architecture and product-boundary review flagged duplicate task-to-artifact
  routing and required next-action previews to be evidence-bound, not just
  free-form input-bound.
- UI and self-explanation review required an explicit alignment-scope panel,
  issue-prioritized facts, more readable raw JSON, and user-facing preview
  safety labels.

The implementation fixes the T-009 blockers from those reviews. The separate
`f-237nwzbyq` broad repair scan tracker inconsistency was not part of this
commit and must not be staged or treated as implemented behavior.

## Tests Added

- `testEvidenceVaultRetainsMultipleReadSurfacesInSameContext`
- `testStaleStructuredCompletionIsRetainedAsStaleRawEnvelopeOnly`
- `testDriftRecordStructuredJSONIsRetainedAsReviewEvidence`
- `testEvidenceVaultCardsSurfaceNonZeroCLIExitAndPrioritizeIssueFacts`
- `testEvidenceNextActionPlannerRequiresLoadedEvidenceBeforePreviewingMutationCommands`
- `testEvidenceNextActionPlannerAllowsExecutionOnlyForCompleteReviewMetadataActions`
- `testPruneApprovePlanningRequiresLoadedSoftDeleteAndExplicitApprovalID`
- `testLoadedEvidenceResolversRejectSkippedQueueRowsAndPruneRefusals`
- `testEvidenceArtifactCatalogRefreshUsesTargetRootAndClearsOnSetupChange`
- `EvidenceArtifactCatalogTests` covering family classification, hidden/dot
  artifacts, malformed JSON, symlink refusal, and shared search/filter logic
- Existing next-action planner tests were tightened to require verified loaded
  evidence before previewing mutation command metadata.

## Validation

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
- `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-009`
  passed and wrote `artifacts/gate-T-009-r14-0001.log`
- `staticcheck ./...`
- `golangci-lint run ./...`

Supplemental `go test -race -p=1 -count=1 ./...` did not pass in the existing
Go suite: `internal/cli` failed in
`TestDaemonRunForegroundRunsProfileBackedLocalPollingSyncAcrossRestart` while
`daemon stop` inspected a temporary daemon event file that had already
disappeared. This app Evidence Vault slice did not modify that daemon code path;
the failure remains a separate reliability item before claiming race-clean
release readiness.
