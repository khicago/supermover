# Verification Evidence

## Automated Checks

- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/protocol ./internal/protocolclient ./internal/networkpush ./internal/receiver ./internal/incrementalsync ./internal/cli -run 'TestFileStoreManagedReplacementRequiresPreviousPublishedEvidence|TestSyncNetworkRunPublishesQueuedHiddenFileViaNetwork|TestSyncNetworkRunHelpIsHonest|TestSyncNetworkDiscoverRunHelpIsHonest|TestSyncNetworkLoopHelpIsHonest|TestDaemonHelpIncludesLogsAndRestartWithoutOverclaiming|TestRunLoopbackPushesFromProfileOnly|TestRunRejectsDivergentTargetAtBeginWithoutUploadingPayload'`
- Result: pass for the T-003 per-entry network transport slice. It covers
  profile-backed per-entry network manifests for queued hidden-file changes,
  previous published manifest evidence in the second network run, receiver-side
  refusal without previous evidence, receiver-side managed regular-file
  replacement with matching previous evidence, tampered previous digest
  refusal, and updated help non-goals.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/receiver ./internal/cli -run 'TestFileStoreManagedReplacementRequiresPreviousPublishedEvidence|TestSyncNetworkRunPublishesQueuedHiddenFileViaNetwork'`
- Result: pass after hardening receiver managed replacement post-hold error
  recovery.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/control -run 'TestValidateArtifactLoadBoundary'`
- Result: pass after adding regression coverage for ignoring ordinary daemon
  event writer temp files while still rejecting symlink temp files.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=10 ./internal/cli -run TestDaemonRunForegroundLocalPollingSyncResumesSessionSequenceFromReceipts`
- Result: pass for the daemon receipt-sequence repro loop after daemon event
  temp-file boundary validation was narrowed.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/protocol ./internal/protocolclient ./internal/networkpush ./internal/receiver ./internal/incrementalsync ./internal/cli ./internal/profile ./internal/discovery ./internal/pairing ./internal/receiverserve ./internal/report ./internal/status ./internal/agentdaemon`
- Result: pass for the broader T-003 package set covering protocol previous
  evidence, protocolclient manifest generation, per-entry networkpush transfer,
  receiver replacement safety, CLI network queue execution, profile/discovery
  command compatibility, report/status aggregation, and daemon network polling
  compatibility.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass after wiring per-entry network queue publication, hardening
  receiver managed replacement recovery, ignoring ordinary daemon event temp
  files in boundary validation, and syncing docs/tracker truth.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -race -p=1 -count=1 ./...`
- Result: pass.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go mod tidy -diff`
- Result: pass.
- Command: `git diff --check`
- Result: pass.
- Command: `go run ./cmd/supermover sync network run --help`
- Result: pass; help describes per-entry profile-backed mTLS network manifest
  publication, previous published manifest evidence, receiver-side target
  revalidation, and non-goals.
- Command: `go run ./cmd/supermover sync network discover-run --help`
- Result: pass; help keeps discovery as a low-information availability gate and
  describes the same per-entry profile-pinned mTLS transfer after the gate.
- Command: `go run ./cmd/supermover sync network loop --help`
- Result: pass; help describes foreground network polling over per-entry
  profile-backed mTLS network manifests and excludes automatic endpoint
  selection, broad repair, detached daemon execution, and bidirectional sync.
- Command: `go run ./cmd/supermover sync network --help`
- Result: pass; help summarizes the sync-network surface as bounded/foreground
  per-entry profile-backed mTLS transfer with target control-plane receipts.
- Command: `go run ./cmd/supermover sync --help`
- Result: pass; help lists bounded network run, LAN-discovery-gated run, and
  foreground network loop without claiming automatic endpoint selection or
  broad repair.
- Command: `go run ./cmd/supermover daemon --help`
- Result: pass; help describes source-side foreground network polling as
  per-entry profile-backed mTLS queue publication and excludes automatic
  discovery-selected sync, endpoint selection, broad repair, and detached
  service management.
- Command: `go run ./cmd/supermover help`
- Result: pass.
- Command: `go run ./cmd/supermover version`
- Result: pass.
- Command: `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" validate-tracker --root .`
- Result: pass.

- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/profile ./internal/cli -run 'TestValidateRejectsInvalidProfiles|TestWriteReadRoundTrip|TestSyncNetworkDiscoverRunHelpIsHonest|TestDaemonHelpIncludesLogsAndRestartWithoutOverclaiming|TestSyncNetworkRunUsageErrors|TestSyncNetworkDiscoverRunPublishesAfterMatchingLANCandidate|TestSyncNetworkDiscoverRunNoMatchDoesNotMutateQueue|TestDiscoverAdvertiseWritesLowInfoDatagram|TestDiscoverAdvertiseReceiverHintRequiresPairedNetworkProfile'`
- Result: pass for the T-002 LAN-discovery-gated network slice. It covers
  `discovery.advertise_receiver_hint` profile validation and round-trip,
  `sync network discover-run` help and usage validation, low-information
  receiver-hint advertisement, refusal to advertise receiver hints from
  unpaired profiles, discovery gate match to profile `network.receiver_url`,
  hidden-file publication through the existing pinned mTLS network queue pass
  after the gate matches, no queue/run/session mutation when no LAN candidate
  matches, and no discovery trust claim.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli ./internal/profile ./internal/discovery ./internal/pairing ./internal/receiverserve ./internal/networkpush ./internal/incrementalsync ./internal/report ./internal/status ./internal/agentdaemon`
- Result: pass for the broader T-002 package set covering CLI command routing,
  profile schema validation, LAN discovery candidate handling, pairing trust,
  receiver serve material validation, profile-backed network push, durable
  queue/run receipts, report/status aggregation, and daemon artifact
  compatibility.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass after wiring `sync network discover-run`, profile
  `discovery.advertise_receiver_hint`, receiver-hint advertisement validation,
  and docs/tracker truth for the LAN-discovery-gated network slice.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -race -p=1 -count=1 ./...`
- Result: pass after wiring the LAN-discovery-gated network slice.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go mod tidy -diff`
- Result: pass.
- Command: `git diff --check`
- Result: pass.
- Command: `go run ./cmd/supermover sync network discover-run --help`
- Result: pass; help describes discovery as a low-information availability
  gate, requires a match to profile-selected `network.receiver_url`, keeps
  profile-pinned mTLS as the publication boundary, and excludes automatic
  endpoint selection, profile mutation, per-entry network transport,
  foreground watching, background daemon execution, and bidirectional sync.
- Command: `go run ./cmd/supermover sync network --help`
- Result: pass; help lists bounded network run, LAN-discovery-gated
  discover-run, and foreground network loop without claiming automatic
  endpoint selection or per-entry network transport.
- Command: `go run ./cmd/supermover discover advertise --help`
- Result: pass; help keeps advertisements low-information and profile-policy
  backed.
- Command: `go run ./cmd/supermover sync --help`
- Result: pass; help separates queue/run/loop/watch evidence from automatic
  endpoint selection and per-entry network transport.
- Command: `go run ./cmd/supermover daemon --help`
- Result: pass; help excludes LAN browsing, automatic discovery-selected sync,
  detached background execution, OS watchers, and per-entry network transport.
- Command: `go run ./cmd/supermover help`
- Result: pass.
- Command: `go run ./cmd/supermover version`
- Result: pass.
- Command: `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" validate-tracker --root .`
- Result: pass.
- Command: `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" run-task-gate --root . --feature f-232nwu2nw --task T-002`
- Result: pass (`artifacts/gate-T-002-r4-0001.log`). The tracker lifecycle
  gate used the repository's generic non-UI gate template that was still
  carrying the previous drift-slice command set, so the T-002-specific gate
  evidence is the focused, broader-package, full-suite, race, and help checks
  recorded above. The local ignored runtime policy was corrected for future
  local tracker runs, but it is not tracked evidence for this task.
- Command: `go test -count=1 ./internal/incrementalsync`
- Result: pass for durable queue lifecycle, bounded run receipts, in-flight
  recovery, retry backoff, cancellation, and hidden-file queue coverage.
- Command: `go test -count=1 ./internal/cli -run 'TestRunHelp|TestSyncRun|TestSyncQueue'`
- Result: pass for `sync queue` and `sync run` help, usage, local publish,
  refused-publish retry, and status evidence.
- Command: `go test -count=1 ./internal/cli -run 'TestSyncLoop|TestSyncRun|TestSyncQueue|TestRunHelp'`
- Result: pass for `sync loop` help, usage validation, generated run receipts,
  foreground polling across two passes, changed hidden-file detection between
  passes, local publish, and compact status evidence.
- Command: `go test -count=1 ./internal/control ./internal/incrementalsync ./internal/cli -run 'TestValidateArtifactLoadBoundaryRejectsUnsafeControlArtifacts|TestScheduler_RunOnce|TestSyncRun|TestSyncQueue|TestRunHelp'`
- Result: pass for incremental run receipt symlink-boundary validation plus
  focused queue/run behavior.
- Command: `go test -count=1 ./internal/incrementalsync ./internal/report ./internal/status ./internal/cli -run 'TestScheduler_RunResultsLoadsReceiptsAndReportsProblems|TestBuildReportSurfacesIncrementalSyncEvidence|TestBuildReportDoesNotRequireReviewForCompletedIncrementalSyncEvidence|TestBuildSurfacesIncrementalSyncCounts|TestSyncRunPublishesQueuedChangesAndMarksDone|TestSyncRunRetriesQueueWhenLocalPushRefusesTargetDrift'`
- Result: pass for run receipt loading, corrupt run receipt artifact problems,
  report/status incremental sync aggregation, completed run non-review behavior,
  retrying run review behavior, and CLI text surfacing of the compact counts.
- Command: `go test -p=1 -count=1 ./internal/incrementalsync ./internal/report ./internal/status ./internal/cli -run 'TestScheduler_(MarkFailedHidesEntryFromReady|FailedEntryReopensWhenSourceChanges)|TestSyncQueue(FailMarksTerminalReviewState|HelpIsHonest|UsageErrors)|TestBuildReportSurfacesFailedIncrementalSyncEvidence|TestBuildSurfacesFailedIncrementalSyncCounts'`
- Result: pass for explicit failed-terminal queue state, failed-at persistence,
  failed entries excluded from ready work, changed source observation reopening
  failed paths as queued work, CLI `sync queue fail`, and failed count
  propagation into report/status review surfaces.
- Command: `go test -p=1 -count=1 ./internal/cli -run 'TestSyncQueue(ListShowsAllEntryLifecycleDetails|HelpIsHonest|UsageErrors|EnqueueReadyStatusAndCancel|FailMarksTerminalReviewState)'`
- Result: pass for `sync queue list` help, usage validation, read-only
  all-entry listing, text lifecycle fields, JSON entry detail, and separation
  from the filtered `sync queue ready` executable subset.
- Command: `go test -p=1 -count=1 ./internal/profile ./internal/cli -run 'TestValidateAcceptsProfileBackedLocalPollingSync|TestWriteReadRoundTrip|TestValidateRejectsInvalidProfiles|TestDaemonRunForegroundRunsProfileBackedLocalPollingSyncAcrossRestart|TestDaemonRunForegroundLocalPollingSyncResumesSessionSequenceFromReceipts|TestDaemonHelpIncludesLogsAndRestartWithoutOverclaiming'`
- Result: pass for profile-backed `sync.local_polling` validation and
  round-trip, daemon help honesty, foreground daemon local polling publish of a
  hidden file, restart consumption, fresh generated run receipt IDs across
  restart, durable receipt-based session sequence recovery after foreground
  process stop/start, and `daemon_sync_*` lifecycle evidence.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli -run 'TestSyncWatch(HelpIsHonest|UsageErrors|PublishesChangedHiddenFileFromOSEvent)|TestSyncLoopHelpIsHonest|TestRunHelp'`
- Result: pass for foreground `sync watch` help honesty, usage validation,
  baseline run receipt, OS file-event-triggered run receipt, changed hidden-file
  publication from an existing watched dot-directory, generated session IDs,
  and boundaries excluding background daemon, continuous network sync, and LAN
  discovery claims.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli -run 'TestSyncNetworkRun'`
- Result: pass for `sync network run` help honesty, usage validation, hidden
  file publication through the existing profile-backed `push --network` mTLS
  transfer primitive, receiver-side network-transfer evidence, durable
  incremental queue/run receipts, idle queue not-attempted network output, and
  missing-network-material refusal before target control-plane mutation.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli -run 'TestSyncNetwork(Loop|Run)|TestRunHelp|TestSync(Watch|Run|Loop)HelpIsHonest'`
- Result: pass for `sync network loop` help honesty, usage validation,
  foreground two-pass profile-backed network queue execution, generated session
  IDs, first-pass hidden-file publication through the existing `push --network`
  mTLS transfer primitive, receiver-side network-transfer evidence for the
  attempted pass, second-pass idle not-attempted network output, and updated
  local run/loop/watch help boundaries.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli -run 'TestSyncNetwork(Loop|Run)|TestRunHelp|TestSync(Watch|Run|Loop)HelpIsHonest|TestDaemonHelpIncludesLogsAndRestartWithoutOverclaiming'`
- Result: pass after adding foreground profile-backed `sync network loop` and
  narrowing local run/loop/watch/daemon help non-goals from broad continuous
  network sync wording to daemon-integrated/automatic discovery-selected network sync
  boundaries.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli -run 'TestSyncNetworkRun(HelpIsHonest|UsageErrors|PublishesQueuedHiddenFileViaNetwork)|TestSync(Watch|Run|Loop)HelpIsHonest|TestRunHelp'`
- Result: pass for command-surface honesty around local run/loop/watch and the
  bounded profile-backed network queue run without claiming LAN discovery,
  detached daemon execution, continuous network sync, or per-entry network
  transport.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli ./internal/incrementalsync ./internal/networkpush ./internal/report ./internal/status`
- Result: pass for touched CLI, queue scheduler, profile-backed network push,
  report, and status packages after adding foreground profile-backed network
  queue loop execution.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass after adding foreground profile-backed `sync network loop`,
  updating command help, and syncing docs/tracker truth.
- Command: `go vet ./...`
- Result: pass after adding foreground profile-backed `sync network loop`.
- Command: `go mod tidy -diff`
- Result: pass.
- Command: `git diff --check`
- Result: pass.
- Command: `go run ./cmd/supermover sync network run --help`
- Result: pass; help describes one bounded network queue pass,
  profile-selected target control-plane receipts, profile-backed
  `push --network` transfer primitive, and excludes LAN discovery, per-entry
  transport, watcher, detached daemon, and bidirectional sync.
- Command: `go run ./cmd/supermover sync network loop --help`
- Result: pass; help describes a foreground network polling loop, generated
  session prefix, profile-selected target control-plane receipts,
  profile-backed `push --network` transfer primitive, bounded `--max-runs`
  smoke use, and excludes LAN discovery, per-entry network transport, OS file
  watcher, background daemon, and bidirectional sync.
- Command: `go run ./cmd/supermover sync --help`
- Result: pass; help lists `sync network run` and `sync network loop`, and
  separates profile-backed network queue execution from LAN discovery and
  per-entry network transport.
- Command: `go run ./cmd/supermover daemon --help`
- Result: pass; help describes profile-enabled foreground local polling and
  source-side foreground network polling, and excludes LAN discovery,
  automatic discovery-selected sync, per-entry network transport, detached background
  execution, and OS service management.
- Command: `go run ./cmd/supermover help`
- Result: pass; root help describes `sync` as durable queue plus bounded
  local/network runs, foreground loop, and watcher evidence.
- Command: `go run ./cmd/supermover version`
- Result: pass.
- Command: `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" validate-tracker --root .`
- Result: pass.
- Command: `go test -p=1 -count=1 ./internal/profile ./internal/cli ./internal/incrementalsync ./internal/report ./internal/status ./internal/agentdaemon`
- Result: pass for touched profile, daemon/CLI, queue scheduler, report/status,
  and daemon artifact packages after adding profile-enabled foreground-daemon
  local polling sync.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/profile ./internal/cli -run 'TestValidateAcceptsProfileBackedNetworkPollingSync|TestValidateRejectsMultipleDaemonPollingModes|TestWriteReadRoundTrip|TestDaemonHelpIncludesLogsAndRestartWithoutOverclaiming|TestDaemonRunForegroundRunsProfileBackedNetworkPollingSync'`
- Result: pass for profile-backed `sync.network_polling` validation and
  round-trip, local/network daemon polling mutual exclusion, daemon help
  honesty, source-side worker-only foreground daemon network polling, hidden
  file publication through the existing `push --network` mTLS transfer
  primitive, receiver-side network-transfer evidence, redacted `daemon_sync_*`
  lifecycle evidence, foreground stop intent, and durable receipt-based
  network session sequence recovery after foreground process stop/start.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -race -p=1 -count=1 ./internal/cli -run 'TestDaemonRunForegroundRunsProfileBackedLocalPollingSyncAcrossRestart|TestDaemonRunForegroundRunsProfileBackedNetworkPollingSync|TestDaemonRunForegroundLocalPollingSyncResumesSessionSequenceFromReceipts'`
- Result: pass after moving the local polling stdout assertion until after the
  foreground daemon goroutine exits; covers local and network daemon polling
  receipt sequence recovery under the race detector.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=20 -run TestDaemonRunForegroundLocalPollingSyncResumesSessionSequenceFromReceipts ./internal/cli`
- Result: pass after fixing the test clock to advance between foreground
  daemon process stop/start runs; the previous fixed `Now` could leave a stop
  intent dated after the next daemon startup clock and cancel the next pass.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=10 -run 'TestDaemonRunForeground(LocalPollingSyncResumesSessionSequenceFromReceipts|RunsProfileBackedLocalPollingSyncAcrossRestart|RunsProfileBackedNetworkPollingSync)' ./internal/cli`
- Result: pass for repeated local and network daemon polling smoke tests after
  the monotonic test-clock fix.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass after adding profile-enabled source-side foreground-daemon
  network polling and updating docs/tracker truth.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -race -p=1 -count=1 ./...`
- Result: pass after adding profile-enabled source-side foreground-daemon
  network polling.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- Result: pass.
- Command: `go mod tidy -diff`
- Result: pass.
- Command: `git diff --check`
- Result: pass.
- Command: `go run ./cmd/supermover daemon --help`
- Result: pass; help states `sync.network_polling` runs a source-side
  foreground network queue worker instead of serving receiver routes, and does
  not claim LAN browsing, automatic discovery-selected sync, per-entry network
  transport, detached background execution, or OS service installation.
- Command: `go run ./cmd/supermover daemon run --help`
- Result: pass.
- Command: `go run ./cmd/supermover sync --help`
- Result: pass; help keeps standalone bounded local/network sync commands
  separate from background daemon, bidirectional sync, LAN discovery, and
  per-entry network transport.
- Command: `go run ./cmd/supermover help`
- Result: pass.
- Command: `go run ./cmd/supermover version`
- Result: pass.
- Command: `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" validate-tracker --root .`
- Result: pass.
- Command: `sh "$BAGAKIT_LIVING_KNOWLEDGE_SKILL_DIR/scripts/bagakit-living-knowledge.sh" doctor --root .`
- Result: pass with three existing `.codex/.tmp` path-local `AGENTS.md`
  precedence warnings; no `.codex` files were modified or cleaned.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./internal/cli ./internal/incrementalsync ./internal/profile ./internal/report ./internal/status`
- Result: pass for touched CLI watcher, queue scheduler, profile, report, and
  status packages after adding foreground OS watcher `sync watch`.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass after adding foreground OS watcher `sync watch` and updating
  root/sync help text.
- Command: `go vet ./...`
- Result: pass.
- Command: `go mod tidy -diff`
- Result: pass.
- Command: `git diff --check`
- Result: pass.
- Command: `go run ./cmd/supermover sync watch --help`
- Result: pass; help describes foreground OS watcher behavior, baseline/event
  queue/run receipts, bounded `--max-events` smoke use, and excludes background
  daemon, bidirectional sync, continuous network sync, and LAN discovery.
- Command: `go run ./cmd/supermover sync network run --help`
- Result: pass; help describes one bounded network queue pass,
  profile-selected target control-plane receipts, profile-backed
  `push --network` transfer primitive, and excludes LAN discovery, per-entry
  transport, watcher, detached daemon, and bidirectional sync.
- Command: `go run ./cmd/supermover sync --help`
- Result: pass; help lists `sync watch` and `sync network run` alongside
  queue/run/loop and keeps continuous network sync and LAN discovery out of
  scope.
- Command: `go run ./cmd/supermover help`
- Result: pass; root help describes `sync` as durable queue, bounded run,
  foreground loop, and watcher evidence.
- Command: `go run ./cmd/supermover version`
- Result: pass.
- Command: `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" validate-tracker --root .`
- Result: pass.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- Result: pass after adding profile-enabled foreground-daemon local polling sync
  and durable receipt-based session sequence recovery.
- Command: `go vet ./...`
- Result: pass.
- Command: `go mod tidy -diff`
- Result: pass.
- Command: `git diff --check`
- Result: pass.
- Command: `go run ./cmd/supermover daemon --help`
- Result: pass; help describes profile-enabled local polling sync and
  explicitly excludes OS service management, detached background processes, OS
  file watching, automatic discovery-selected sync, and continuous network sync.
- Command: `go run ./cmd/supermover daemon run --help`
- Result: pass.
- Command: `go run ./cmd/supermover help`
- Result: pass.
- Command: `go run ./cmd/supermover version`
- Result: pass.
- Command: `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" validate-tracker --root .`
- Result: pass.
- Command: `go test -count=1 ./internal/cli ./internal/incrementalsync ./internal/control`
- Result: earlier bounded-run slice pass; superseded for this report/status
  slice by `go test -count=1 ./internal/cli ./internal/incrementalsync
  ./internal/report ./internal/status`, which passed.
- Command: `go test -p=1 -count=1 ./internal/incrementalsync ./internal/report ./internal/status ./internal/cli`
- Result: pass for all touched incremental sync, report/status, and CLI
  package tests after adding failed-terminal queue state.
- Command: `go test -race -count=1 ./internal/incrementalsync ./internal/cli`
- Result: superseded by `go test -race -count=1 ./internal/incrementalsync
  ./internal/cli ./internal/control`, which passed.
- Command: `go test -race -count=1 ./internal/incrementalsync ./internal/cli ./internal/report ./internal/status`
- Result: pass for touched queue, CLI, report, and status packages after
  adding explicit failed-terminal queue state.
- Command: `go test -count=1 ./...`
- Result: pass.
- Command: `go vet ./...`
- Result: pass.
- Command: `go mod tidy -diff`
- Result: pass.
- Command: `git diff --check`
- Result: pass.
- Command: `go run ./cmd/supermover sync --help`
- Result: pass; help describes durable queue plus bounded one-pass local
  consumer and explicitly excludes watcher/daemon/network sync.
- Command: `go run ./cmd/supermover sync queue --help`
- Result: pass; help lists enqueue/status/list/ready/cancel/fail and describes
  the profile-selected target queue boundary.
- Command: `go run ./cmd/supermover sync queue list --help`
- Result: pass; help describes read-only all-entry queue detail and excludes
  execution, copy, watcher, daemon, and ongoing sync behavior.
- Command: `go run ./cmd/supermover sync queue fail --help`
- Result: pass; help describes terminal operator review evidence and explicitly
  excludes source deletion, target mutation, retry, daemon, and ongoing sync.
- Command: `go run ./cmd/supermover sync run --help`
- Result: pass; help describes one bounded pass, durable run receipt, existing
  local push safety path, and non-goals.
- Command: `go run ./cmd/supermover sync loop --help`
- Result: pass; help describes foreground local polling, generated session IDs,
  optional `--max-runs`, durable run receipts, existing local push safety path,
  and non-goals.
- Command: `go run ./cmd/supermover status --help`
- Result: pass; help says network and incremental sync fields are local
  artifact evidence only and do not start synchronization.
- Command: `go run ./cmd/supermover report --help`
- Result: pass; help says report may surface incremental sync queue/run
  receipts but does not start daemon, transport, watcher, or repair work.
- Command: `go run ./cmd/supermover help`
- Result: pass; root help lists `sync` as durable changed-file queue and
  bounded run evidence.
- Command: `go run ./cmd/supermover version`
- Result: pass.
- Command: `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" run-task-gate --root . --feature f-232nwu2nw --task T-001`
- Result: pass; tracker gate log
  `.bagakit/feature-tracker/features/f-232nwu2nw/artifacts/gate-T-001-r3-0001.log`
  recorded the configured task gate. Note: the configured task gate still uses
  stale drift-focused commands, so sync-specific evidence above is the behavior
  evidence for this slice.
- Command: `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" validate-tracker --root .`
- Result: pass.

## Manual Checks

- Step: Confirm T-003 per-entry network publication is described as wired for
  queued network entries without expanding into automatic endpoint selection or
  broad repair.
- Outcome: README, v1 scope, control-plane docs, runbook, profile docs, user
  guide, troubleshooting, release audit, authority docs, plan, and tracker
  proposal now describe `sync network run`/`loop`/`discover-run` and
  `sync.network_polling` as per-entry profile-backed mTLS queue publication
  with previous published manifest evidence for regular-file replacement.
- Step: Confirm repeated push, bounded `sync run`, foreground `sync loop`,
  foreground `sync watch`, bounded `sync network run`, foreground
  `sync network loop`, profile-enabled foreground-daemon local polling sync,
  and profile-enabled source-side foreground-daemon network polling sync are
  not described as detached daemon, LAN discovery, automatic discovery-selected sync,
  automatic endpoint selection, or broad repair.
- Outcome: README, v1 scope, runbook, release audit, user guide, troubleshooting,
  and tracker proposal separate local queue/watch execution and bounded network
  queue execution from planned automatic discovery-selected network sync and
  broader repair.
- Step: Confirm `sync queue fail` is described as explicit terminal operator
  review evidence, not target repair, retry, publish, or synchronization.
- Outcome: README, control-plane docs, v1 scope, runbook, user guide,
  troubleshooting, release audit, and tracker proposal separate failed queue
  evidence from retry backoff and target-restored claims.
- Step: Confirm `sync queue list` is described as a read-only per-entry queue
  detail surface, while `sync queue ready` remains the executable subset.
- Outcome: README, v1 scope, runbook, user guide, troubleshooting, release
  audit, control-plane docs, and tracker proposal separate detailed queue
  inspection from execution and synchronization claims.
- Step: Confirm hidden files and dot-directories are included in queue fixtures.
- Outcome: automated queue/run tests include `.hidden/secret.txt` and verify it
  is queued, published, and marked done.

## Residual Risks

- Watcher behavior can be platform-dependent. The wired `sync watch` slice is a
  foreground OS watcher over existing source directories and keeps queue/run
  behavior auditable through durable receipts; detached service supervision and
  daemon-integrated watcher behavior remain out of scope.
- `sync run`, `sync loop`, `sync watch`, and profile-enabled foreground-daemon
  local polling currently delegate to local push as whole-profile publish
  passes, then mark the ready queue entries done on success. They are
  intentionally not detached background execution, automatic endpoint selection, or
  bidirectional conflict handling.
- `sync network run`, `sync network loop`, `sync network discover-run`, and
  profile-enabled foreground-daemon network polling publish ready queue entries
  through per-entry profile-backed mTLS network manifests. They still need
  profile-selected local target control-plane access for queue/run receipts,
  and regular-file replacement remains limited to previous published manifest
  evidence plus receiver-side target revalidation. They are not automatic
  endpoint selection, detached background execution, broad repair, or
  bidirectional sync.
- Report/status now aggregate compact queue/run receipt counts and malformed
  artifact problems. Operators use `sync queue list`, `sync queue ready`, and
  run receipt JSON for per-entry detail.
