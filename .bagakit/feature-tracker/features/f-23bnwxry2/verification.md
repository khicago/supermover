# Verification Evidence

## Automated Checks

- Task: `T-001`
  - Command: `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-001`
  - Result: pass in `artifacts/gate-T-001-r1-0001.log`.
  - Gate commands passed: `go test -p=1 -count=1 ./...`, `go vet ./...`,
    `go mod tidy -diff`, `git diff --check`,
    `go run ./cmd/supermover help`, and
    `go run ./cmd/supermover version`.
- Task: `T-001` supplemental help-surface smokes.
  - Result: pass for `profile --help`, `profile init --help`,
    `profile set-target --help`, `scan --help`, `deleted --help`,
    `recover --help`, `prune --help`, `reconcile --help`, `drift --help`,
    `discover --help`, `pair --help`, `sync --help`, `daemon --help`,
    `push --help`, and `push --network --help`.
- Task: `T-002`
  - Command: `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-002`
  - Result: pass in `artifacts/gate-T-002-r4-0001.log`.
  - Gate commands passed: `go test -p=1 -count=1 ./...`, `go vet ./...`,
    `go mod tidy -diff`, `git diff --check`,
    `go run ./cmd/supermover help`, and
    `go run ./cmd/supermover version`.
- Task: `T-002` supplemental app build and tracker checks.
  - `go test -count=1 ./...`: pass.
  - `swift build --package-path macos`: pass.
  - `feature-tracker validate-tracker --root .`: pass.
  - App-specific evidence is recorded in
    `artifacts/app-verification-T-002.md`.
- Task: `T-003` supplemental app build.
  - `swift build --package-path macos`: pass.
  - App-specific evidence is recorded in
    `artifacts/app-verification-T-003.md`.
- Task: `T-003`
  - Command: `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-003`
  - Result: pass in `artifacts/gate-T-003-r5-0001.log`.
  - Gate commands passed: `go test -p=1 -count=1 ./...`, `go vet ./...`,
    `go mod tidy -diff`, `git diff --check`,
    `go run ./cmd/supermover help`, and
    `go run ./cmd/supermover version`.
- Task: `T-004` supplemental app build.
  - `swift build --package-path macos`: pass.
  - App-specific evidence is recorded in
    `artifacts/app-verification-T-004.md`.
- Task: `T-004`
  - Command: `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-004`
  - Result: pass in `artifacts/gate-T-004-r6-0001.log`.
  - Gate commands passed: `go test -p=1 -count=1 ./...`, `go vet ./...`,
    `go mod tidy -diff`, `git diff --check`,
    `go run ./cmd/supermover help`, and
    `go run ./cmd/supermover version`.
- Task: `T-005`
  - Command: `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-005`
  - Result: pass in `artifacts/gate-T-005-r7-0001.log`.
  - Supplemental post-review validation passed for
    `swift build --package-path macos`, `go test -p=1 -count=1 ./...`,
    `go vet ./...`, `go mod tidy -diff`, `git diff --check`,
    `go run ./cmd/supermover help`, `go run ./cmd/supermover version`, and
    tracker validation. See `artifacts/app-review-fix-T-005.md`.
- Task: `T-006`
  - Command: `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-006`
  - Result: pass in `artifacts/gate-T-006-r9-0001.log`.
  - Gate commands passed: `go test -p=1 -count=1 ./...`, `go vet ./...`,
    `go mod tidy -diff`, `git diff --check`,
    `go run ./cmd/supermover help`, and
    `go run ./cmd/supermover version`.
- Task: `T-006` supplemental app checks.
  - `swift build --package-path macos`: pass.
  - `swift test --package-path macos`: pass; 5 XCTest tests.
  - App-specific evidence is recorded in
    `artifacts/app-discovery-pairing-T-006.md` and
    `artifacts/app-verification-T-006.md`.
- Task: `T-006` phase review fix.
  - `gpt-5.5/xhigh` review found one P1 stale text-output promotion risk for
    successful `pair` completions and one P2 tracker-summary overclaim.
  - Result: fixed. Successful text-output guidance now requires the finished
    run to match current task context, and the tracker summary now reflects the
    implemented CLI-backed boundary. See `artifacts/app-review-fix-T-006.md`.
- Task: `T-007`
  - Command: `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-007`
  - Result: pass in `artifacts/gate-T-007-r10-0001.log`.
  - Gate commands passed: `go test -p=1 -count=1 ./...`, `go vet ./...`,
    `go mod tidy -diff`, `git diff --check`,
    `go run ./cmd/supermover help`, and
    `go run ./cmd/supermover version`.
- Task: `T-007` supplemental app checks.
  - `swift build --package-path macos`: pass.
  - `swift test --package-path macos`: pass; 8 XCTest tests.
  - `feature-tracker validate-tracker --root .`: pass.
  - App-specific evidence is recorded in
    `artifacts/app-sync-execution-T-007.md` and
    `artifacts/app-verification-T-007.md`.
- Task: `T-010` supplemental packaging/readiness checks.
  - `swift test --package-path macos --filter 'AppStoreTests/testVersionAndForegroundDaemonCommandsUseExplicitBoundary|AppStoreTests/testCLIProvenanceReportsRunnableMode'`: pass; 2 XCTest tests.
  - `swift test --package-path macos --filter 'AppStoreTests/testPackagedCLIProvenanceBlocksMissingCLIWithoutDevelopmentFallback|AppStoreTests/testPackagedCLIProvenanceGatesMalformedUnsignedAdHocAndDirtyBundles|AppStoreTests/testCLIProvenanceReportsRunnableMode'`: pass; 3 XCTest tests.
  - `swift test --package-path macos`: pass; 44 XCTest tests.
  - `sh -n macos/script/build-app.sh`: pass.
  - `macos/script/build-app.sh`: pass; produced unsigned local app at
    `macos/dist/SuperMover.app`.
  - `jq . macos/dist/SuperMover.app/Contents/Resources/supermover-provenance.json`:
    pass; manifest reports schema `supermover.macos.provenance.v1`, build
    profile `local-release`, CLI version `supermover 0.1.0-dev`, and signing
    `unsigned`.
  - `test -x macos/dist/SuperMover.app/Contents/Resources/bin/supermover && macos/dist/SuperMover.app/Contents/Resources/bin/supermover version`:
    pass; bundled CLI prints `supermover 0.1.0-dev`.
  - `plutil -lint macos/script/Info.plist macos/script/SuperMover.entitlements`:
    pass.
  - `feature-tracker validate-tracker --root .`: pass.
  - `git diff --check`: pass.
  - App-specific evidence is recorded in
    `artifacts/app-packaging-readiness-T-010.md`.
- Task: `T-011` packaged loopback acceptance.
  - `SUPERMOVER_ACCEPTANCE_SKIP_BUILD=1 macos/script/acceptance-loopback.sh`:
    pass. The script used the bundled CLI under
    `macos/dist/SuperMover.app/Contents/Resources/bin/supermover`, checked that
    provenance matches the current `HEAD` short commit, bundled CLI version,
    and expected bundle path, and printed a disposable evidence directory.
  - Stale skipped-build provenance negative check: before rebuilding the stale
    app bundle, the same skipped-build command failed with shell exit code `5`
    and `jq` reported `incomplete packaged provenance`; after
    `macos/script/build-app.sh`, the bundle manifest reported
    `git_commit=6d4972ce9900` and acceptance passed.
  - `go run ./cmd/supermover reconcile scan --help`: expected fail with
    `reconcile: unknown subcommand "scan"`, confirming `f-237nwzbyq` must remain
    proposal-only.
  - App-specific acceptance evidence is recorded in
    `artifacts/app-acceptance-T-011.md`. The artifacts directory is ignored by
    default, so this acceptance artifact must be committed with an explicit
    `git add -f` for T-011.
  - `SUPERMOVER_CODESIGN_IDENTITY= macos/script/build-app.sh`: pass; produced an
    unsigned local app and preserved distribution readiness as blocked.
  - `macos/script/audit-app.sh macos/dist/SuperMover.app`: expected exit `1`
    against the unsigned app with `status=blocked`, `readiness=blocked`, and
    `blocking_checks=11`.
  - `SUPERMOVER_CODESIGN_IDENTITY=- macos/script/build-app.sh`: pass; signed the
    bundled CLI first and then the app bundle without signing-time `--deep`.
  - `macos/script/audit-app.sh macos/dist/SuperMover.app`: expected exit `1`
    against the ad-hoc app with `status=blocked`, `readiness=blocked`, and
    `blocking_checks=6`. The audit confirmed app and CLI hardened runtime plus
    entitlements, while ad-hoc identity, dirty worktree, Gatekeeper assessment,
    and stapler validation blocked distribution readiness.
  - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsRoleMatchedHandoffWhenMetaMachineFactsDisagree|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsWrongMachinePairHandoff|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMachineFactArtifactMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotTreatMetaMatchedHandoffAsProofWhenMachineFactArtifactsDisagree|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotTreatRoleMatchedHandoffAsProofWhenMachineFactsDisagree'`: pass; the shell distinct-machine proof surface now pins positive advance-to-evaluate plus wrong-pair, meta/artifact mismatch, and role-vs-meta machine-facts fail-closed branches.
  - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsProveTheWrongMachinePair|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsDoNotProveCrossMachineTransfer|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts'`: pass; 39 tests executed with 7 opt-in packaging tests skipped.
  - `swift build --package-path macos`: pass after the app-side launch gate
    consolidation and preview fail-closed tightening.
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchGateTests|AcceptancePackagingEvidenceTests|AppStoreTests|AcceptanceBundleAppOperationsIntegrationTests'`:
    pass; the app-side installed-app launch verdict now has focused coverage for
    packaged-app requirements, missing audit/notarization, schema-valid but
    release-incomplete notarization, and preview blocking when the local
    packaged app's notarization sidecar is missing, malformed, or not
    release-ready.
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchGateTests|AcceptancePackagingEvidenceTests|AcceptanceEvaluationTests|AppStoreTests'`:
    pass after moving local notarization preview inspection out of `AppStore`
    and into `AcceptanceBundleAppOperations` plus
    `AcceptancePackagingEvidenceCollector`, and after making Swift final
    evaluation reuse the same release-ready notarization helper as the app-side
    launch gate.
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppNotarizationSourceTests'`:
    pass after removing the unproven Swift-only
    `Contents/Resources/release/notary/notarization.json` fallback. The new
    tests pin that installed-app preview, preflight, and packaging evidence all
    ignore bundled fallback notarization artifacts and only trust sibling
    `<AppName>.app.notary/notarization.json`.
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppLaunchGateTests|AcceptancePackagingEvidenceTests|AcceptanceEvaluationTests|AppStoreTests'`:
    pass; app-side notarization-source alignment stayed green across preview,
    preflight, packaging evidence collection, and Swift final evaluation.
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppBundleContextTests|AcceptanceInstalledAppLaunchGateTests'`:
    pass after introducing a dedicated bundle-context resolver. Installed-app
    preview, preflight, and `runSelectedTask()` now fail closed when a
    configured acceptance bundle path cannot be loaded, instead of silently
    skipping the two-machine gate.
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppBundleContextTests|AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppLaunchGateTests|AcceptancePackagingEvidenceTests|AcceptanceEvaluationTests|AppStoreTests'`:
    pass; app-side installed-app launch now stays aligned across bundle-context
    failure, notarization-source selection, packaged-app/audit/notarization
    gate evaluation, packaging evidence collection, and Swift final evaluation.
  - `git diff --check -- macos/SuperMoverApp/AcceptanceInstalledAppLaunchBundleContext.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchGate.swift macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverApp/AcceptanceBundleAppOperations.swift macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift macos/SuperMoverApp/AppStore.swift macos/SuperMoverAppTests/AcceptanceInstalledAppBundleContextTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppLaunchGateTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppNotarizationSourceTests.swift macos/SuperMoverAppTests/AppStoreTests.swift`:
    pass.
  - `swift test --package-path macos --filter 'AcceptanceEvaluationPathSafetyTests'`:
    pass; Swift final evaluate now has focused coverage for bundle `..` path
    escape rejection, symlinked provenance rejection, and malformed
    `source.consistency.json` rejection even when meta still looks ready.
  - `swift test --package-path macos --filter 'AcceptanceBundleTests|AcceptanceInstalledAppLaunchGateTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceEvaluationTests|AcceptanceInstalledAppBundleContextTests|AcceptanceEvaluationPathSafetyTests|AppStoreTests/testAcceptanceTwoMachineLaunch'`:
    pass; 87 tests executed after unifying final evaluate with the shared
    bundle-artifact access seam.
  - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleArtifactAccess.swift macos/SuperMoverApp/AcceptanceBundleReader.swift macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift macos/SuperMoverAppTests/AcceptanceEvaluationPathSafetyTests.swift`:
    pass.
  - `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests'`:
    pass; focused coverage now pins symlinked `meta.json`, symlinked bundle
    root, launch-bundle-context unreadable-bundle failure, artifact-writer
    pre-write rejection, and packaging-evidence pre-write rejection.
  - `swift test --package-path macos --filter 'AcceptanceBundleTests|AcceptanceInstalledAppBundleContextTests|AcceptanceBundleArtifactWriterTests|AcceptancePackagingEvidenceTests|AcceptanceEvaluationPathSafetyTests|AcceptanceEvaluationTests|AppStoreTests/testAcceptanceTwoMachineLaunch'`:
    pass; 86 tests executed with 2 opt-in real-packaging tests skipped after
    wiring reader/writer/collector through the shared bundle root trust owner.
  - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleArtifactAccess.swift macos/SuperMoverApp/AcceptanceBundleReader.swift macos/SuperMoverApp/AcceptanceBundleArtifactWriter.swift macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverAppTests/AcceptanceBundleRootTrustTests.swift`:
    pass.
  - `sh macos/script/build-app.sh`:
    pass; rebuilt `macos/dist/SuperMover.app` from the tightened bundle-root
    trust-boundary slice as an unsigned local app.
  - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests|AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppCollectionProofParityScriptTests|AcceptanceInstalledAppLaunchGateTests|AcceptanceBundleTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceBundleAppOperationsIntegrationTests|AppStoreTests/testAcceptanceTwoMachineLaunch'`:
    pass; 105 tests executed with 9 opt-in real-app / real-packaging tests
    skipped after tightening three app-side fail-closed seams together:
    verified wrong-pair handoffs now stay in the `bundle_handoff` remediation
    lane instead of the contradictory blocked bucket, launch preflight no
    longer mutates the acceptance bundle once installed-app proof is already
    blocked, packaging evidence collection no longer leaves fresh
    version/provenance/app-audit artifacts behind when local notarization
    sidecar validation fails, and app-side preview/preflight now share one
    non-mutating local packaging-feasibility check that exercises bundled
    provenance, bundled CLI version probing, local audit helper execution, and
    sibling notarization currentness before phase launch.
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppNotarizationSourceTests|AppStoreTests/testAcceptanceTwoMachineLaunchPreview|AppStoreTests/testAcceptanceTwoMachineLaunchPreflight'`:
    pass; 45 tests executed after consolidating launch preview/preflight behind
    one typed current-app packaging probe, including the new fail-closed case
    where the loaded bundle still looks ready but the current app's fresh audit
    would make the local sibling notarization sidecar stale before preflight
    writes anything.
  - `swift test --package-path macos --filter 'AcceptanceMachineIdentityTests|AcceptanceBundleArtifactWriterTests|AppStoreTests'`:
    pass; 88 tests executed after wiring app-side acceptance authoring through
    a current-machine identity resolver and making corrective `pair` / `serve`
    rewrites update canonical `source.machine.json` / `target.machine.json`
    plus `meta.json` `roles.*` and `evidence.machine_facts.*` records. The new
    app/store tests pin that corrective auto-record now moves a stale-machine-id
    bundle out of the machine-identity blocker and down to the remaining
    verified `bundle_handoffs` gate instead of leaving correction as a
    preview-only advisory.
  - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests|AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppCollectionProofParityScriptTests|AcceptanceInstalledAppLaunchGateTests|AcceptanceBundleTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceBundleAppOperationsIntegrationTests|AppStoreTests/testAcceptanceTwoMachineLaunch'`:
    pass; 106 tests executed with 9 opt-in real-app / real-packaging tests
    skipped after moving preview/preflight onto that same typed probe owner.
  - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleAppOperations.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchCoordinator.swift macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverAppTests/AppStoreTests.swift`:
    pass.

## Manual Checks

- Step: Confirm an existing feat already covers only the native macOS operator
  shell.
- Outcome: `f-235nwsp4y` is archived and scoped to a CLI-truthful operator
  shell, not complete app-first LAN orchestration.

- Step: Confirm app-first LAN workbench scope needs a new feature.
- Outcome: pending implementation; current app redesign is an early UI slice,
  while discovery/pairing native flow, sync network controls, real progress,
  Merkle/root evidence, and full evidence browser remain future work.

- Step: Independent plan review.
- Outcome: subagent review found P0 gaps in app-owned profile bootstrap,
  explicit LAN/sync command coverage, event API ordering, and multi-process
  supervision. The task graph was revised to put those contracts before final
  transfer UI implementation.

- Step: Local implementation review.
- Outcome: current CLI exposes `profile init/lint/set-target`, `discover`,
  `pair`, `sync queue/run/loop/watch/network`, `daemon`, and `serve`; current
  macOS app task coverage only includes profile lint, local/review commands,
  network push, serve, daemon status/logs, and dashboard. The revised plan now
  requires a command coverage matrix before implementation.

- Step: Platform and comparable-product research.
- Outcome: Apple platform guidance makes file access, Local Network/firewall
  behavior, signing/notarization, and binary provenance relevant to a two-Mac
  install. Comparable sync/backup products reinforce compare-before-sync,
  explicit device identity, verification, and task history as core workflow
  surfaces rather than optional polish.

- Step: T-001 capability contract review.
- Outcome: `artifacts/capability-contract.md` now records the app-first role
  model, command coverage matrix, structured output needs, process supervision
  requirements, mutating action gates, unsupported capabilities, and acceptance
  gates.

- Step: T-001 subagent review.
- Outcome: review findings were incorporated. The contract now separates
  `missing`, `wired`, and `gate-compliant`; marks current app-launched flows as
  `wired but not gate-compliant` where session defaults, free-text ids,
  text-only output, or single-process supervision remain unsafe; adds
  `drift expire`, profile id/name flags, `reconcile apply --all-persisted-planned`,
  `reconcile apply --record-live`, and missing source/deleted/recovery/prune
  surfaces; and limits T-002 to setup/readiness until T-003/T-004 process and
  event contracts pass.

- Step: T-002 app-guided setup implementation.
- Outcome: macOS app now starts on a setup section with source/target/observer
  role choice, profile destination selection, source and target root browsing,
  app-side readable/writable directory readiness, profile id/name and target
  id/name inputs, CLI previews, and buttons for `profile init`,
  `profile set-target`, and `profile lint`. Long-running `serve` and
  `dashboard` launches are intentionally blocked until T-003/T-004.

- Step: T-002 subagent review fix.
- Outcome: setup readiness is now scoped to the current profile/root/identity
  context, stale structured evidence from earlier inputs is not promoted,
  source/target/observer roles are enforced before launching CLI tasks, observer
  setup is read-only, and serve/dashboard UI copy now labels those actions as
  blocked until T-003/T-004.

- Step: T-003 role-scoped process supervision implementation.
- Outcome: the macOS app now supervises foreground runs by slot: bounded action,
  target serve, and target dashboard. Starting a process in one slot no longer
  terminates another slot, duplicate starts in the same running slot are
  refused, stop actions target a named slot, and lifecycle events are visible in
  the app. Readiness remains log-derived until T-004.

- Step: T-003 subagent review fix.
- Outcome: supervised foreground liveness now distinguishes current-context
  `running` from `stale running` after profile, role, session, listen address,
  or setup input changes. Stale processes stay stoppable by slot but no longer
  render as current green liveness for the new setup context in either slot
  cards or focused run detail.

- Step: T-004 structured app event and artifact reader base.
- Outcome: the app now records structured app events, records visible artifact
  reader problems for missing or malformed JSON stdout, stores raw samples for
  inspection, and exposes those events/problems in the Evidence view. Text/stderr
  compatibility parsing remains non-authoritative until later structured event
  surfaces land.

- Step: T-004 implementation status and plan honesty refresh.
- Outcome: `README.md`, `macos/README.md`, `docs/plan.md`,
  `docs/runbook.md`, `docs/v1-scope.md`, and this feature proposal now state
  that the app is a CLI-backed operator workbench with T-001 through T-004
  implemented, while native discovery/pairing, sync controls, Merkle/root
  comparator evidence, evidence browser, packaging readiness, and acceptance
  evidence remain planned.

- Step: T-004 subagent review fix.
- Outcome: structured snapshot promotion is now gated by command completion
  semantics. Cancelled commands, non-zero exits with no structured stdout, and
  non-zero mutating command exits produce `structured output skipped` app events
  rather than artifact reader problems or promoted snapshots. Read-only
  review-required JSON exits remain promotable as review evidence because
  surfaces such as `status` can return review-required non-zero while still
  emitting valid JSON.

- Step: T-005 information architecture and design-system stabilization.
- Outcome: the app now renders a role runway before section content, exposes
  section availability as available/planned/read-only/role-gated in the sidebar
  and top bar, and makes pairing, transfer, and drift review pages role-aware.
  Source role sees planned native pairing and current wired transfer controls;
  target role owns supervised foreground serve; observer role remains read-only.
  Control Room, Settings, and the complete CLI task picker now hide execution
  controls when the selected role cannot run the selected command.
  Post-commit phase review fixes keep warning/artifact metrics pending until
  matching evidence exists, broaden evidence review detection across
  status/report/health problem fields, and hide profile creation from
  target/observer roles.
  The slice does not implement native discovery/pairing, sync controls,
  Merkle/root comparison, or packaging readiness.

- Step: T-006 native discovery and pairing workflow.
- Outcome: the app now exposes source-side Discover Browse, Discover Address,
  and Pair actions plus target-side Discover Advertise and Serve actions.
  Discovery snapshots are parsed from JSON and rendered as untrusted
  low-information evidence with duplicate/ambiguous candidate classification.
  Pairing requires the target verification code and remains backed by CLI
  receipt/profile-pin writes; the app directs operators to Status/Report for
  durable confirmation. Observer remains read-only. `codexL` small review found
  and the implementation fixed advertise-listen misuse and stale pairing input
  context gaps before gate.

- Step: T-006 phase review fix.
- Outcome: stale successful `pair` completions no longer promote old text
  summaries as current guidance after target address, verification code,
  method, or timeout changes. The T-006 tracker summary no longer claims
  challenge/hash display, profile pin status, ambiguity refusal, or LAN
  readiness diagnostics as implemented app behavior.

- Step: T-007 sync queue/run/loop/watch/network execution controls.
- Outcome: the app now exposes source-owned sync queue enqueue/cancel/fail,
  bounded `sync run`, `sync network run`, discovery-gated
  `sync network discover-run`, and supervised foreground `sync loop`,
  `sync watch`, and `sync network loop` controls. Target and Observer roles
  can inspect read-only queue status/list/ready only. Structured snapshots
  decode queue counters/entries, run receipts, network transfer outcomes,
  discovery gate evidence, and foreground loop/watch counters. Non-zero sync
  JSON exits are retained as review evidence where stdout contains a structured
  result, but success state still requires exit 0.

- Step: T-007 phase review fix.
- Outcome: development app launches now use a supervised build-and-exec path
  instead of supervising `go run`: the launcher builds
  `.tmp/macos-app/supermover-dev` and then `exec`s it in the same foreground
  process, so Stop has an immediate supervised process, asks the active build
  child to stop during build-phase termination, and targets the actual CLI after
  build succeeds. Process exit handling drains stdout/stderr before promoting
  structured evidence. Discovery-gated no-match results with Go zero-value
  `enqueue`/`run`/`network` fields no longer render an executed run.
  Sync-specific input changes clear promoted sync snapshots before new evidence
  is loaded.

- Step: T-008 verification comparator JSON evidence foundation.
  - Outcome: `verify` app launches now request `--format json`, decode typed
    target-vs-published-manifest evidence, and promote review-exit JSON as
    current verification evidence when the setup context still matches. The
    Verification screen shows manifest/session/root identity, file counts,
    findings, artifact problems, and explicit negative availability for Merkle
    proof and current-source comparison. `manifest.root_id` is labeled as profile
    root identity only; no Merkle tree or content-root proof is claimed.

- Step: T-008 phase review fix.
  - Outcome: post-commit subagent review found a P1 gate-semantics issue where
    clean verify-only evidence could make the broader target preflight gate
    green. The app now routes readiness through `EvidenceGateEvaluation`, so
    target preflight uses `status`/`report`/`health` evidence while verification
    uses `verify` evidence. Verification status copy is narrowed to
    target-vs-manifest wording, and warning, soft-delete, and target-drift
    details are inspectable in the comparator panel.

- Step: T-009 evidence vault browser and safe actions.
  - Outcome: the app now has a native Evidence Vault with typed cards for
    `verify`, `status`, `report`, and `health`; raw structured stdout envelope
    history with exit/freshness/stderr metadata; explicit
    target-vs-published-manifest alignment scope; issue-prioritized facts; a
    searchable target `.supermover` artifact catalog; malformed JSON and symlink
    catalog problems; and evidence-bound review-metadata actions. `drift record
    --format json` is captured as structured review evidence. The vault can run
    only bounded review-metadata commands whose selected ids resolve from loaded
    evidence: drift record/acknowledge/resolve/expire, sync queue cancel/fail,
    prune approve, and prune supersede. The catalog is explicitly a manual read
    of the selected Target Root field, not profile-derived target proof. Skipped
    queue rows and prune refusals cannot unlock actions. Vault-side
    `prune approve` is single-candidate only; multi-id approval remains outside
    the Evidence Vault run path. The vault still does not execute prune apply,
    reconcile apply, pairing, publish, network push, sync execution,
    Merkle/root proof, or current-source comparison. App-specific evidence is
    recorded in `artifacts/app-evidence-vault-T-009.md`.
  - Gate: `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-009`
    passed in `artifacts/gate-T-009-r14-0001.log`.
  - Supplemental checks passed: `swift build --package-path macos`,
    `swift test --package-path macos`, `feature-tracker validate-tracker --root .`,
    `staticcheck ./...`, and `golangci-lint run ./...`.
  - Supplemental race check did not pass: `go test -race -p=1 -count=1 ./...`
    failed in existing `internal/cli`
    `TestDaemonRunForegroundRunsProfileBackedLocalPollingSyncAcrossRestart`
    because `daemon stop` inspected a temporary daemon event file that had
    already disappeared. Treat race-clean daemon lifecycle as separate
    reliability work before release readiness.

- Step: T-010 macOS packaging, permissions, and daemon controls.
  - Outcome: Settings now exposes CLI provenance/readiness, manual
    `supermover version` verification, Local Network/firewall/file-access
    readiness guidance, and foreground daemon install/run/restart/stop/status/log
    controls. The build script bundles the CLI into the `.app`, writes
    `Contents/Resources/supermover-provenance.json`, supports optional
    code-signing verification, and records dirty provenance including
    non-ignored untracked files. Packaged app readiness now requires executable
    bundled CLI plus complete readable provenance, treats unsigned/dirty bundles
    as review-only local evidence, and refuses to fall back to the development
    launcher when a packaged bundle is missing its CLI. The daemon surface
    remains foreground-only and app-supervised; the app does not claim
    launchd/SMAppService installation, automatic permission approval detection,
    or new key material storage.
  - Supplemental checks passed: targeted Swift provenance/daemon command tests
    full Swift package tests, shell syntax validation for
    `macos/script/build-app.sh`, a real unsigned app packaging run, provenance
    manifest inspection, bundled CLI executable/version inspection, tracker
    validation, and diff whitespace validation.
  - Test-discovered fix: full Swift tests exposed that Evidence Vault
    review-metadata execution could report missing loaded evidence before
    detecting that the previously confirmed command preview no longer matched
    current app inputs. The app now checks role and preview/current-input match
    before reporting missing loaded evidence, while still reporting missing
    evidence when inputs are unchanged and evidence has been unloaded.
  - Stage review fix: gpt-5.5/xhigh spec and quality reviews blocked the first
    implementation because executable-only bundled CLI readiness could pass
    without usable provenance, unsigned/dirty manifest fields were hidden from
    readiness, broken packaged apps could fall back to the development launcher,
    target daemon restart/stop lacked a visible reason input, and the dirty bit
    ignored untracked files. These were fixed before commit.
  - Re-review fix: ad-hoc signing (`SUPERMOVER_CODESIGN_IDENTITY=-`) is now
    review-only local evidence, not pass-ready provenance. Deterministic Swift
    tests now cover packaged missing CLI/no fallback, malformed and incomplete
    provenance, unsigned, ad-hoc, dirty, and signed-clean provenance.

- Step: T-011 packaged loopback acceptance and tracker honesty.
  - Outcome: `macos/script/acceptance-loopback.sh` now provides a repeatable
    packaged-app loopback acceptance using the bundled CLI rather than `go run`.
    It validates local profile init/lint, local push, verify/status/report/health,
    a dot-prefixed file, dot-directory payload, zero-byte file, a larger
    regular file, bounded sync queue/run, and unpaired network dry-run
    fail-closed behavior without target data or control-plane mutation. The
    current packaged loopback path additionally exercises same-machine pairing,
    target profile pairing adoption, and non-dry-run localhost mTLS transfer
    through the bundled CLI, preserving receiver-side `network-transfer.json`
    evidence and a structured summary with `network_loopback.status=pass`. It
    writes a structured summary and prints the preserved evidence directory on
    success or failure, and records unsigned/dirty local provenance as
    review-only rather than distribution-ready.
    It does not claim signed/notarized distribution, two-machine LAN transfer,
    Local Network/firewall prompt approval, two-machine non-dry-run mTLS
    transfer, detached daemon behavior, or Merkle/current-source proof.
  - Outcome: `f-237nwzbyq` tracker truth was corrected back to proposal-only
    because current CLI does not expose `reconcile scan`.

- Step: T-011 local release-engineering audit.
  - Outcome: `macos/script/audit-app.sh` now emits JSON release-engineering
    evidence for plist, provenance, current git state, bundled CLI version/path,
    basic hashes, codesign app/CLI verification details, hardened runtime,
    entitlements, Gatekeeper assessment, and stapler validation.
  - Outcome: `macos/script/build-app.sh` now signs inside-out when a signing
    identity is provided: bundled CLI first, then app bundle, keeping `--deep`
    for verification rather than signing.
  - Outcome: unsigned and ad-hoc local builds correctly remain `blocked` for
    distribution readiness; ad-hoc builds provide local review evidence for
    runtime and entitlements but do not replace Developer ID/notary/stapler
    evidence.
- Step: T-011 trust-model and harness closure progress.
  - Outcome: `pair --receipt-out` now exports a transferable source-side pairing
    receipt, `profile adopt-pairing --receipt-file` now imports that receipt
    into target `.supermover/pairings` plus target profile pins, and
    `profile set-network` now provides a formal profile-SSOT command to update
    `network.receiver_url` and local TLS identity material instead of relying
    on runtime overrides.
  - Outcome: the native macOS app now exposes those same receipt export/import
    and profile network authoring surfaces directly in the source/target
    pairing workbench instead of leaving them script-only.
  - Outcome: the packaged two-machine harness is now structured as
    `target-serve`, `source-pair`, `target-import`, `source-transfer`, and
    `evaluate`. The packaged app path has been verified locally as a
    same-machine simulation of that five-phase flow, including paired receiver
    readiness and non-dry-run transfer. This remains local wiring evidence
    only; it does not claim real two-device installed-app LAN evidence, Local
    Network/firewall prompt approval, or notarized distribution readiness.
  - Outcome: the native macOS app Evidence Vault can now read the same
    acceptance bundle and durably author `.evidence.operator` records for
    `local_network`, `firewall`, and `pairing_confirmation` under the shared
    `.meta.lock` protocol. This closes the gap where real-device prompt/code
    evidence previously existed only as shell-side bundle edits.
  - Outcome: the native macOS app Evidence Vault can now also write current
    phase artifacts into that shared bundle for discovery browse/advertise,
    serve readiness phases, source pair, and source transfer, rather than
    leaving phase artifact authoring entirely to shell harness roles.
- Step: T-011 distinct-machine proof-summary alignment.
  - Outcome: shell `workflow-status` and
    `evaluate --require-operator-evidence` now consume the same installed-app
    proof verdict surface instead of separately interpreting weaker handoff
    booleans. The proof summary now requires collection mode, role machine ids,
    `evidence.machine_facts.*`, machine-facts artifact files, and verified
    cross-machine handoff evidence to agree before installed-app evaluation can
    proceed.
  - Outcome: direct shell tests now pin positive advance-to-evaluate plus
    wrong-pair handoff, role-vs-meta machine-facts mismatch, and
    meta-vs-artifact machine-facts mismatch. This closes the earlier advisory
    drift where `workflow-status` and `evaluate` could be refactored
    independently.
- Step: T-011 app fallback proof-parity alignment.
  - Outcome: when the shell-authored workflow summary artifact is missing, the
    app-side `AcceptanceBundleLoadedSnapshot.workflowSummary(...)` fallback now
    requires valid `supermover.acceptance.machine_facts.v1` artifacts before it
    treats verified bundle handoffs as installed-app proof.
  - Outcome: the fallback also now matches the shell lane split by advancing
    directly to `evaluate` in the default lane and reserving `bundle_handoff`
    for `require-operator-evidence` when installed-app proof is still missing.
  - Evidence: `swift test --package-path macos --filter 'AcceptanceBundleTests|AcceptanceEvaluationTests|AcceptanceTwoMachineScriptTests'`
    passed on 2026-06-03, including new coverage for invalid machine-facts
    schema and default-lane advance-to-evaluate behavior.
- Step: T-011 app advisory proof-parity tightening.
  - Outcome: the app-side installed-app launch preview/preflight path now
    consumes the same Swift-side distinct-machine collection proof family as the
    workflow-summary substrate instead of treating release-ready packaging
    evidence as sufficient. Incomplete machine-facts / handoff proof now stays
    `review`, contradictory machine-facts or verified-handoff proof now blocks
    phase launch, and `pass` requires a current bundle whose distinct-machine
    proof matches the shared proof owner.
  - Outcome: `workflow.summary.json` reuse is also narrower. In the
    two-machine lane, the app now ignores cached shell-authored default and
    `require-operator-evidence` workflow summaries unless installed-app proof
    fields, release-evidence booleans, `bundle_status`, and derived `steps` /
    `next_actions` still match the current merged bundle state.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchGateTests|AcceptanceInstalledAppWorkflowSummaryTests|AppStoreTests/testAcceptanceTwoMachineLaunchPreflightBlocksContradictoryInstalledAppProof|AppStoreTests/testAcceptanceTwoMachineLaunchPreflightAllowsDistributionReadyBundledAcceptanceTask|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewReviewsReadyAuditWhenDistinctMachineProofIsIncomplete|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewReviewsDistributionReadyAuditWhenDistinctMachineProofIsIncomplete|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewPassesWhenDistinctMachineProofIsReady|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewDoesNotTrustStaleMetaWhenSourceAppAuditArtifactIsMissing'`
    - `swift test --package-path macos --filter 'AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsProveTheWrongMachinePair|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsDoNotProveCrossMachineTransfer|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts'`
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryStartsWithTargetServe|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryRequiresHandoffBeforeEvaluate'`
    - `git diff --check -- macos/SuperMoverApp/AcceptanceInstalledAppCollectionProof.swift macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift macos/SuperMoverApp/AcceptanceBundleSnapshot.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchGate.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchBundleContext.swift macos/SuperMoverApp/AppStore.swift macos/SuperMoverAppTests/AcceptanceInstalledAppWorkflowSummaryTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppLaunchGateTests.swift macos/SuperMoverAppTests/AppStoreTests.swift`
- Step: T-011 local notarization sidecar currentness hardening.
  - Outcome: sibling `<AppName>.app.notary/notarization.json` evidence is no
    longer accepted on schema/status alone. The app now requires the sidecar's
    `audit.path` to stay anchored to the canonical sibling
    `<AppName>.app.notary/post-staple.audit.json`, requires that post-staple
    audit to exist, to match the sidecar's audit readiness fields, and to
    carry the same packaged-app provenance manifest as the current installed
    app before local notarization is treated as current release evidence.
  - Outcome: stale sibling sidecars now fail closed across all three relevant
    paths: packaging evidence collection refuses to copy them into the
    acceptance bundle, launch preview blocks with a currentness
    warning, and launch preflight blocks after removing stale notarization
    bundle evidence instead of silently trusting it.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppNotarizationSourceTests|AppStoreTests/testAcceptanceTwoMachineLaunchPreflightAcceptsNotarizeScriptSidecarWorkflow|AppStoreTests/testAcceptanceTwoMachineLaunchPreflightAllowsReadyBundledAcceptanceTask|AppStoreTests/testAcceptanceTwoMachineLaunchPreflightAllowsDistributionReadyBundledAcceptanceTask|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewReviewsLocalNotarizationThatIsNotReleaseReady|NotarizeAppScriptTests/testNotarizeScriptProducesDurableEvidenceOnSuccessfulFakeNotaryFlow|NotarizeAppScriptTests/testNotarizeScriptPersistsStructuredSidecarNextToAppOnSuccessfulFakeNotaryFlow'`
    - `git diff --check -- macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverApp/AcceptanceBundleAppOperations.swift macos/SuperMoverAppTests/AcceptanceInstalledAppNotarizationSourceTests.swift macos/SuperMoverAppTests/AppStoreTests.swift macos/SuperMoverAppTests/NotarizeAppScriptTests.swift`
- Step: T-011 shell-side notarization currentness parity.
  - Outcome: shell-side installed-app release evidence is no longer accepted on
    status/readiness alone. `record-packaging-evidence` now rejects sibling
    `<AppName>.app.notary/notarization.json` sidecars whose referenced
    post-staple audit no longer matches the current packaged app and bundled
    provenance, and shell `workflow-status` plus
    `evaluate --require-operator-evidence` now require bundle-local
    `*.app-audit.json`, `*.provenance.json`, and `*.notarization.json` to agree
    before the two-machine installed-app lane can advance to final evaluation.
  - Outcome: focused shell regression tests now pin all three fail-closed
    points: stale sidecar rejection during packaging evidence collection,
    workflow summary downgrade back to `source_packaging_evidence` /
    `target_packaging_evidence`, and direct `evaluate` failure when a copied
    notarization artifact no longer matches bundled release evidence.
  - Outcome: shell-side currentness now also canonicalizes provenance JSON
    before comparison, so semantically equal manifests with different key order
    no longer fail stale on raw string ordering alone.
  - Outcome: the shell script tests now share a smaller release-evidence fixture
    support module instead of continuing to hand-author status-only notarization
    JSON in each suite, which reduces one more path for test-only policy drift.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceAcceptsAuditProvenanceWithDifferentKeyOrder'`
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests|AcceptanceInstalledAppReleaseEvidenceScriptTests|AcceptanceInstalledAppNotarizationSourceScriptTests|AcceptanceInstalledAppReleaseEvidenceEvaluationScriptTests|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsProveTheWrongMachinePair|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsDoNotProveCrossMachineTransfer|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts'`
    - `sh -n macos/script/lib/acceptance-common.sh macos/script/lib/acceptance-two-machine.sh macos/script/acceptance-two-machine.sh`
- Step: T-011 local shell release-audit sidecar currentness.
  - Outcome: `macos/script/audit-app.sh` now shares the canonical sibling
    sidecar currentness rule when such evidence already exists. A malformed or
    stale `<AppName>.app.notary/notarization.json` sidecar now blocks the local
    release-engineering audit instead of being silently ignored.
  - Outcome: `macos/script/notarize-app.sh` now removes any previous sibling
    sidecar result before its post-staple audit, so stale sidecars do not block
    overwriting with fresh notarization evidence on a later rerun. The
    referenced post-staple audit is also persisted into the sibling sidecar
    directory, so a successful sidecar stays current even when a custom
    temporary `--work-dir` is cleaned up later.
  - Evidence:
    - `swift test --package-path macos --filter 'AppAuditTamperTests|NotarizeAppScriptTests|AcceptanceInstalledAppReleaseEvidenceScriptTests'`
    - `sh -n macos/script/audit-app.sh macos/script/notarize-app.sh`
    - `macos/script/audit-app.sh macos/dist/SuperMover.app`: expected exit `1` on the current unsigned local build
- Step: T-011 canonical sibling symlink fail-closed tightening.
  - Outcome: app-side packaging collection, app-side local notarization preview,
    the bundled packaged-app audit helper, shell `record-packaging-evidence`,
    and shell `audit-app.sh` now all reject symlinked canonical sibling
    `notarization.json` or canonical sibling `post-staple.audit.json` paths
    instead of following them as bundle-local proof.
  - Outcome: when current local notarization evidence becomes unsafe, copied
    `source|target.notarization.json` bundle artifacts are removed before the
    lane can continue, and preview text now reports unsafe symlinked local
    notarization evidence explicitly instead of falling back to a generic
    inspection failure.
  - Evidence:
    - `swift test --package-path macos --filter 'PackagedAppAuditorTests|AcceptancePackagingEvidenceTests|AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppNotarizationSourceScriptTests|AcceptanceInstalledAppReleaseEvidenceScriptTests|AppAuditTamperTests|NotarizeAppScriptTests'`
    - `sh -n macos/script/lib/acceptance-common.sh macos/script/audit-app.sh macos/script/notarize-app.sh`
    - `git diff --check -- macos/SuperMoverAppSupport/PackagedAppAuditor.swift macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverApp/AcceptanceBundleAppOperations.swift macos/script/lib/acceptance-common.sh macos/script/audit-app.sh macos/SuperMoverAppTests/PackagedAppAuditorTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppNotarizationSourceTests.swift macos/SuperMoverAppTests/AppAuditTamperTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppNotarizationSourceScriptTests.swift`
- Step: T-011 bundled helper sidecar currentness parity.
  - Outcome: `PackagedAppAuditor` now matches the shell-side bundle-local
    sidecar bootstrap/currentness split on the shared subset. A missing
    canonical sibling sidecar is no longer an automatic helper audit failure,
    but an existing stale sibling sidecar still blocks helper
    `currentness`/`release_ready`.
  - Outcome: helper-side `currentness` / `release_ready` now also fail closed
    when the sibling sidecar is decodable but invalid for the notarization
    schema contract. A malformed-yet-current-looking sidecar can no longer stay
    `current` just because its app path, audit path, and provenance still
    match.
  - Outcome: helper-side currentness now remains durable after a successful
    notarize run with a custom `--work-dir`; once the temp work dir is removed,
    the persisted sibling sidecar plus sibling post-staple audit still remain
    sufficient for helper-side currentness checks.
  - Evidence:
    - `swift test --package-path macos --filter 'PackagedAppAuditorTests|AcceptancePackagingEvidenceTests/testCollectorDefaultRunnerUsesBundledAuditHelperWhenPresent|AcceptancePackagingEvidenceTests/testCollectorCopiesStructuredNotarizationEvidenceWhenPresent'`
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests|AppAuditTamperTests|NotarizeAppScriptTests|AcceptancePackagingEvidenceTests|PackagedAppAuditorTests'`
- Step: T-011 helper/collector sidecar fail-closed tightening.
  - Outcome: the bundled helper and app-side packaging collector now reject one
    more stale-evidence class on the shared bundle-local proof subset. Helper
    currentness no longer overclaims on decodable-but-invalid sidecars, and the
    copied-built-app opt-in packaging fixture now writes and asserts the
    sibling `post-staple.audit.json` evidence it references, so the opt-in
    currentness path proves the same sidecar-plus-audit contract that
    production code now enforces.
  - Outcome: app-side packaging collection also now treats a mismatched
    top-level sidecar `app_path` as stale notarization evidence instead of only
    relying on the referenced post-staple audit's `app_path` / provenance
    fields, and helper / collector / shell currentness now all reject sidecars
    whose `audit.path` escapes the canonical sibling
    `.app.notary/post-staple.audit.json`.
  - Evidence:
    - `swift test --package-path macos --filter 'PackagedAppAuditorTests/testAuditorBlocksMalformedCanonicalSidecarEvenWhenAuditStillMatchesCurrentApp'`
    - `swift test --package-path macos --filter 'PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenPostStapleAuditSchemaIsInvalid|PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenPostStapleAuditEscapesSiblingSidecarDirectory|AcceptanceInstalledAppNotarizationSourceTests/testCollectorRejectsSiblingNotarizationWhenAuditPathEscapesSiblingSidecarDirectory|AcceptanceInstalledAppNotarizationSourceTests/testLaunchPreviewTreatsSiblingNotarizationWithExternalAuditPathAsCurrentnessReview|AcceptanceInstalledAppNotarizationSourceTests/testLaunchPreflightFailsClosedWhenSiblingNotarizationAuditPathEscapesSiblingSidecarDirectory|AppAuditTamperTests/testAppAuditBlocksCanonicalNotarizationSidecarWhoseAuditPathEscapesSiblingDirectory'`
    - `SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorUsesSiblingNotarizationSidecarForCopiedBuiltAppWhenEnabled'`
    - `swift test --package-path macos --filter 'PackagedAppAuditorTests|AcceptancePackagingEvidenceTests|AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppReleaseEvidenceScriptTests|AppAuditTamperTests|NotarizeAppScriptTests'`
    - `sh -n macos/script/lib/acceptance-common.sh macos/script/audit-app.sh macos/script/notarize-app.sh`
    - `git diff --check -- macos/script/lib/acceptance-common.sh macos/SuperMoverAppSupport/PackagedAppAuditor.swift macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverAppTests/PackagedAppAuditorTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppNotarizationSourceTests.swift macos/SuperMoverAppTests/AcceptancePackagingEvidenceTests.swift macos/SuperMoverAppTests/AppAuditTamperTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppReleaseEvidenceScriptTests.swift`
- Step: T-011 app-side installed-app release-evidence currentness parity.
  - Outcome: the native app no longer trusts installed-app release evidence on
    audit/notarization pass bits alone. Launch preview, launch preflight,
    workflow summary, and Swift final evaluate now all consume one
    bundle-local release-evidence owner that requires
    `source|target.provenance.json`, `source|target.app-audit.json`, and
    `source|target.notarization.json` to agree on the same current packaged app
    before installed-app release evidence is treated as ready.
  - Outcome: Swift now reads those canonical bundle-local release-evidence
    artifact names directly for this lane, so stale copied notarization
    artifacts no longer slip past app-side bundle loading just because
    `meta.json` still referenced older status/readiness values.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests|AcceptanceEvaluationTests|AcceptanceInstalledAppReleaseEvidenceScriptTests|AcceptanceInstalledAppNotarizationSourceScriptTests|AcceptanceInstalledAppReleaseEvidenceEvaluationScriptTests|AcceptanceInstalledAppLaunchGateTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceInstalledAppBundleContextTests|AcceptanceInstalledAppNotarizationSourceTests'`
    - `sh -n macos/script/lib/acceptance-common.sh macos/script/lib/acceptance-two-machine.sh macos/script/acceptance-two-machine.sh`
- Step: T-011 shell/app contradictory verified-handoff proof parity.
  - Outcome: shell distinct-machine installed-app proof no longer advances when
    a merged bundle contains one verified handoff for the recorded
    source/target machine pair plus additional verified handoffs for some other
    machine pair. `workflow-status` and
    `evaluate --require-operator-evidence` now treat that mixed state as
    `contradictory_verified_bundle_handoffs`, matching the Swift-side proof
    owner instead of leaving shell advisory/final gates more optimistic than
    app-side proof consumers.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppCollectionProofParityScriptTests|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsRoleMatchedHandoffWhenMetaMachineFactsDisagree|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsWrongMachinePairHandoff|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMachineFactArtifactMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotTreatMetaMatchedHandoffAsProofWhenMachineFactArtifactsDisagree|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotTreatRoleMatchedHandoffAsProofWhenMachineFactsDisagree|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsProveTheWrongMachinePair|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsDoNotProveCrossMachineTransfer|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts'`
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
- Step: T-011 app-side launch/preflight and packaging publication fail-closed tightening.
  - Outcome: the app-side installed-app collection proof now distinguishes a
    single verified handoff for the wrong machine pair from genuinely
    contradictory mixed verified handoffs; the former stays in the
    `bundle_handoff` remediation lane while the latter remains blocked for
    review-only cleanup.
  - Outcome: launch preflight now short-circuits non-correctable
    `.installedAppProofBlocked` states before authoring any packaging evidence,
    while still allowing app-side `source pair` / `target serve` when the
    shared proof owner says the next truthful step is machine-identity
    correction. Packaging evidence collection now stages outputs outside the
    acceptance bundle and only publishes them after the full local collection
    succeeds. Malformed or stale local notarization sidecars therefore clear
    stale copied notarization state without leaving fresh `*.version.txt`,
    `*.provenance.json`, or `*.app-audit.json` artifacts behind from the failed
    attempt.
  - Outcome: app-side preview and preflight now also share one non-mutating
    current-app packaging-feasibility owner. Preview no longer assumes that a
    `.bundled` CLI provenance record is sufficient by itself; it now blocks
    when the current packaged app cannot actually load bundled provenance, run
    its bundled CLI `version`, invoke its local audit helper, or safely trust a
    current sibling notarization sidecar. Missing or not-release-ready local
    notarization evidence now also blocks the launch advisory, so app preview
    no longer stays more optimistic than shell phase preflight on that
    installed-app lane.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests|AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppCollectionProofParityScriptTests|AcceptanceInstalledAppLaunchGateTests|AcceptanceBundleTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceBundleAppOperationsIntegrationTests|AppStoreTests/testAcceptanceTwoMachineLaunch'`
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppWorkflowSummaryTests|AppStoreTests/testAcceptanceTwoMachineLaunchPreview|AppStoreTests/testAcceptanceTwoMachineLaunchPreflight'`
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchGateTests|AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppNotarizationSourceTests|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewBlocksMissingLocalNotarizationEvidence|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewBlocksLocalNotarizationThatIsNotReleaseReady'`
- Step: T-011 closeout validation after session `019e7ef9-f666-7a21-b496-d7d49754e6cd`.
  - Outcome: the interrupted session stopped after a rebuilt app cleared 10/10
    repeated same-machine acceptance runs and then launched the final Swift
    acceptance checks without collecting their result.
  - Outcome: fresh verification on 2026-06-01 passed targeted Go package tests
    for the modified CLI/pairing/sync/report/status packages, full
    `go test -count=1 ./...`, shell syntax validation for the
    acceptance/build/audit scripts, a fresh unsigned app build, focused Swift
    acceptance tests, full `swift test --package-path macos`, and opt-in real
    built-app integrations. The opt-in integration exercises `AppStore` writing
    packaging, `target_import`, and `evaluation` artifacts against a real
    same-machine harness bundle after deleting the harness-written artifacts.
  - Boundary: this closes a local documentation and evidence loop only. It is
    still not real two-device installed-app LAN evidence, Local Network/firewall
    prompt evidence, notarized distribution evidence, or Merkle/current-source
    proof.
- Step: T-011 archive handoff restore staging after session
  `019e8912-91da-7b60-8a46-58a4549562b1`.
  - Outcome: `unpack-bundle` now restores archives into a staging directory,
    verifies the manifest digest, requires the manifest export identity to
    match the archive-local export identity artifact, records the verified
    handoff ledger, and only then publishes the requested bundle root. A
    digest-valid but malformed archive can no longer delete an existing
    incoming bundle root before failing closed.
  - Outcome: the same-machine archive-handoff harness remains green with the
    source-local bundle as the owner for source browse, pair, and transfer
    phases before explicit final aggregation.
  - Evidence:
    - `sh -n macos/script/lib/acceptance-two-machine.sh macos/script/acceptance-two-machine-same-machine.sh macos/script/acceptance-two-machine.sh`
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/(testTwoMachineUnpackBundlePreservesExistingBundleWhenArchiveMissingExportIdentity|testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityIsTampered|testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityFieldsAreMissing|testTwoMachineUnpackBundleRecordsVerifiedArchiveHandoffEvidence|testTwoMachinePackAndUnpackBundleRoundTripsEvidence)'`
    - `SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 swift test --package-path macos --filter 'AcceptanceEvaluationIntegrationTests/testSameMachineHarnessSupportsArchiveHandoffWhenEnabled'`
    - `git diff --check -- macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift`
  - Boundary: this is archive-handoff substrate hardening. It does not prove a
    real two-Mac installed-app run, release-grade notarization/stapling, or
    real Local Network/firewall/operator evidence.
- Step: T-011 proof-baseline refresh after shell notarization-currentness tightening.
  - Outcome: the handoff Minimal Takeover Route baseline remains green after
    re-running the pinned wrong-pair / machine-facts fail-closed checks, and
    the broader `AcceptanceTwoMachineScriptTests` suite is back on the current
    shell contract. The stale failures were in test fixtures, not in shell
    proof logic: the fixtures were still writing non-canonical
    notarization-sidecar `audit.path` values and one malformed-archive test no
    longer reached tar extraction after manifest export-identity validation was
    tightened.
  - Outcome: parity-focused app/shell suites also remain green for installed-app
    collection proof, workflow-summary reuse, and launch/evaluate alignment.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsRoleMatchedHandoffWhenMetaMachineFactsDisagree|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsWrongMachinePairHandoff|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMachineFactArtifactMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotTreatMetaMatchedHandoffAsProofWhenMachineFactArtifactsDisagree|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotTreatRoleMatchedHandoffAsProofWhenMachineFactsDisagree'`
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsProveTheWrongMachinePair|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsDoNotProveCrossMachineTransfer|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts'`
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppCollectionProofParityScriptTests|AcceptanceInstalledAppProofParityTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceBundleAppOperationsIntegrationTests'`
    - `sh -n macos/script/lib/acceptance-two-machine.sh macos/script/acceptance-two-machine.sh`
    - `git diff --check -- macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift`
  - Boundary: this keeps the local proof baseline and parity suites honest; it
    does not provide new real two-machine operator evidence or Developer ID /
    notary / staple / Gatekeeper proof.
- Step: T-011 contradictory bundle-handoff advisory alignment.
  - Outcome: shell `workflow-status` now treats
    `contradictory_verified_bundle_handoffs` the same way as the app-side
    workflow summary: both stay in `review_bundle_handoff` with no synthetic
    pack/unpack/merge commands instead of telling the operator to rerun a
    generic `bundle_handoff` step against an already-conflicted merged bundle.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppCollectionProofParityScriptTests|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenVerifiedHandoffsAlsoContainAnotherCrossMachinePair'`
    - `sh -n macos/script/lib/acceptance-two-machine.sh macos/script/acceptance-two-machine.sh`
    - `git diff --check -- macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceInstalledAppCollectionProofParityScriptTests.swift macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift`
  - Boundary: this is app/shell advisory-proof alignment only. It does not
    change final evaluate, real two-machine evidence, or release-grade
    notarization / Gatekeeper status.
- Step: T-011 contradictory installed-app proof parity across launch preview /
  preflight / evaluate.
  - Outcome: contradictory verified `bundle_handoffs` are now explicitly pinned
    as one fail-closed proof lane across app-side workflow summary, launch-gate
    verdict, launch preview, launch preflight, and Swift final evaluate. The
    new parity test also proves that launch preview/preflight do not try to
    probe or rewrite packaging evidence once contradictory handoff proof has
    already blocked the bundle.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppProofParityTests|AcceptanceInstalledAppLaunchGateTests/testGateBlocksContradictoryInstalledAppProofWhenVerifiedHandoffConflictsWithRecordedPair|AcceptanceInstalledAppCollectionProofParityScriptTests/testWorkflowStatusFailsClosedWhenVerifiedBundleHandoffsContainContradictoryMachinePairs|AcceptanceInstalledAppCollectionProofParityScriptTests/testEvaluateFailsClosedWhenVerifiedBundleHandoffsContainContradictoryMachinePairs|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsContainContradictoryMachinePairs'`
    - `git diff --check -- macos/SuperMoverAppTests/AcceptanceInstalledAppProofParityTests.swift`
  - Boundary: this is proof-surface regression coverage only. It does not add
    new real two-machine operator evidence, installed-app release artifacts, or
    Developer ID / notarization / staple / Gatekeeper proof.
- Step: T-011 machine-identity correction parity across workflow / corrective launches / final evaluate.
  - Outcome: same-machine installed-app bundles are now pinned as one shared
    correction lane across app-side workflow summary, corrective `source pair`
    / `target serve` launch preview-preflight, and Swift final evaluate.
    Evaluator machine-facts checking now surfaces the shared
    `finalEvaluationCollectionDetail` before the lower-level canonical
    same-machine-facts guard when both role ids and machine-facts collapse to
    the same machine, so final evaluate no longer drifts away from the
    workflow/launch correction reason for that bundle shape.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppProofParityTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenInstalledAppMachineFactsMatch|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts'`
    - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift macos/SuperMoverAppTests/AcceptanceInstalledAppProofParityTests.swift macos/SuperMoverAppTests/AcceptanceEvaluationTests.swift`
  - Boundary: this is machine-identity proof-surface alignment only. It does
    not create new real two-machine evidence, release notarization/stapling
    proof, or operator-permission proof.
- Step: T-011 shell final-evaluate detail parity and persisted workflow-summary refresh.
  - Outcome: shell distinct-machine installed-app proof now emits one richer
    verdict surface with `blocked_reason`, `missing_requirements`,
    `requires_machine_identity_correction`,
    `requires_bundle_handoff_proof`, and final-evaluate collection /
    machine-facts / bundle-handoff detail fields. `workflow-status`,
    persisted `workflow.summary.json`, and
    `evaluate --require-operator-evidence` now consume that same ordering
    instead of reconstructing weaker shell-only policy. Same-role-machine-id
    collapse, meta-vs-artifact machine-facts mismatch, and
    role-vs-machine-facts conflict now all fail with the same concrete detail
    text the Swift evaluator already used.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppCollectionProofParityScriptTests|AcceptanceInstalledAppProofParityTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenInstalledAppMachineFactsMatch|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenInstalledAppBundleHandoffsDoNotProveCrossMachineTransfer'`
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `sh -n macos/script/acceptance-two-machine.sh`
    - `git diff --check -- macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceInstalledAppCollectionProofParityScriptTests.swift`
  - Boundary: this is shell/app proof-surface alignment only. It does not add
    real two-Mac installed-app evidence, Developer ID notarization/stapling,
    or Gatekeeper proof.
- Step: T-011 profile-selection UX demotes raw path and keeps profile SSOT explicit.
  - Outcome: the macOS workbench now resolves one `ProfileSelectionContext`
    in `AppStore` and uses it across primary setup surfaces, so operators see
    profile identity first and raw file location only as advanced detail.
    Default draft `profile-local` / `Local profile` values no longer dominate
    the primary summary for a new source profile, while source profile identity
    editing only appears when the operator is actually creating a new source
    profile. Existing-profile and loaded-evidence paths still retain the
    profile file as SSOT and keep target identity editing available where it
    remains relevant.
  - Evidence:
    - `swift test --package-path macos --filter 'AppStoreTests|WorkbenchNavigationTests'`
    - `git diff --check -- macos/SuperMoverApp/AppStore.swift macos/SuperMoverApp/ContentView.swift macos/SuperMoverAppTests/AppStoreTests.swift macos/README.md`
  - Boundary: this is workbench state/IA simplification only. It does not add
    new LAN proof, two-machine installed-app evidence, or release-grade
    notarization / staple / Gatekeeper proof.
- Step: T-011 app-side stale-status and cross-machine release-gate tightening.
  - Outcome: the Acceptance Bundle panel top status/metrics now derive from a
    recomputed proof-aware summary instead of raw `meta.status`, so a stale
    bundle with `status=evidence_collected` but current pending
    `next_actions` stays in `review`. Installed-app launch preview/preflight
    also now fail closed when the other machine still lacks release-ready
    packaging evidence, so a locally ready source/target app cannot bypass
    missing `*.app-audit.json` / `*.notarization.json` on its counterpart
    before corrective or ordinary phase launch. `AppStore.refreshAcceptanceBundle()`
    now also emits `review` note/event state for the same stale-collected
    bundle shape instead of logging raw `meta.status` as if it were current
    proof truth. AppStore preview coverage now also explicitly pins the
    contradictory verified-handoff blocked lane.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceBundlePanelSummaryTests|AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceInstalledAppProofParityTests|AppStoreTests/testAcceptanceTwoMachineLaunch'`
    - `swift test --package-path macos --filter 'AcceptanceBundleTests|AcceptanceInstalledAppLaunchGateTests'`
    - `swift test --package-path macos --filter 'AcceptanceBundlePanelSummaryTests|AcceptanceInstalledAppLaunchCoordinatorTests|AppStoreTests/testRefreshAcceptanceBundleUsesProofAwareReviewStateWhenCollectedStatusIsStale|AppStoreTests/testAcceptanceTwoMachineLaunch'`
    - `git diff --check -- macos/SuperMoverApp/AcceptanceBundlePanel.swift macos/SuperMoverApp/AcceptanceBundlePanelSummary.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchBundleContext.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchCoordinator.swift macos/SuperMoverAppTests/AcceptanceBundlePanelSummaryTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppLaunchCoordinatorTests.swift macos/SuperMoverAppTests/AppStoreTests.swift`
  - Boundary: this is app-side proof-surface tightening only. It does not add
    new real two-machine operator evidence, release-grade notarization /
    stapling / Gatekeeper proof, or final T-011 closure.
- Step: T-011 current evaluation-artifact and operator-gate currentness tightening.
  - Outcome: app/shell workflow summary no longer treat bare
    `meta.status=evidence_collected` as current completion. The bundle now
    stays in `review` until a current bundle-local `evaluation.json` exists,
    and the strict two-machine lane reopens `evaluate` when that preserved
    evaluation artifact was recorded without
    `require_operator_evidence=true`. Manual Evidence gate chips, missing-proof
    facts, and workflow operator steps now also derive from the current
    evaluation mode / shared manual-evidence owner instead of stale stored
    evaluation metadata.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceOperatorEvidenceTests|AcceptanceBundlePanelSummaryTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceBundleTests'`
    - `swift test --package-path macos --filter 'AppStoreTests/testRefreshAcceptanceBundleUsesProofAwareReviewStateWhenCollectedStatusIsStale|AppStoreTests/testAcceptanceTwoMachineLaunch'`
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `git diff --check -- macos/SuperMoverApp/AcceptanceManualEvidenceGateSummary.swift macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift macos/SuperMoverApp/AcceptanceBundlePanelSummary.swift macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceOperatorEvidenceTests.swift macos/SuperMoverAppTests/AcceptanceBundlePanelSummaryTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppWorkflowSummaryTests.swift macos/SuperMoverAppTests/AcceptanceBundleTests.swift`
  - Boundary: this is still proof-surface and parity tightening only. It does
    not add new real two-machine operator evidence, non-dry-run distinct-machine
    transfer proof, or Developer ID notarization / staple / Gatekeeper closure.
- Step: T-011 installed-app launch preview current-evaluation parity tightening.
  - Outcome: app-side installed-app launch preview no longer flips to `pass`
    from release-ready packaging evidence plus distinct-machine installed-app
    proof alone. The launch gate now also requires the same current strict
    bundle-local `evaluation.json` truth that shell workflow summary / final
    evaluate require. Stored strict evaluation evidence only counts when the
    current bundle still satisfies the current phase/operator proof inputs.
    Same-step repair is only allowed when that reopened step is the sole
    remaining strict next action. If multiple required steps reopen, or if the
    reopened step is different from the requested launch, preview/preflight
    fail closed with the same blocking action that strict workflow summary
    reports. Preview returns to `pass` only after the merged bundle again
    satisfies current strict final-evaluation truth.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchGateTests|AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppProofParityTests|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewReviewsWhenDistinctMachineProofIsReadyButCurrentStrictEvaluationIsMissing|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewPassesWhenDistinctMachineProofAndCurrentStrictEvaluationAreReady'`
    - `git diff --check -- macos/SuperMoverApp/AcceptanceInstalledAppLaunchGate.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchCoordinator.swift macos/SuperMoverAppTests/AcceptanceInstalledAppLaunchGateTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppProofParityTests.swift macos/SuperMoverAppTests/AppStoreTests.swift docs/plan.md docs/runbook.md macos/README.md .bagakit/feature-tracker/features/f-23bnwxry2/verification.md`
  - Boundary: this is still advisory/parity tightening only. It does not add
    real two-machine installed-app acceptance evidence, canonical release app
    artifacts, or Developer ID notarization / staple / Gatekeeper closure.
- Step: T-011 app-side acceptance authoring receipt-path and current-ready parity.
  - Outcome: app-side acceptance authoring now stages validated durable local
    pairing receipts into bundle-local
    `exported-receipts/<pairing_receipt_id>.json`, records
    `source.pair.json.receipt_path` against that bundle-relative path, and
    writes current `target.ready.json` plus `meta.json`
    `evidence.target_ready` alongside `target.ready.phase-<n>.json`. Pairing
    serve authoring now also fails closed when the current readiness snapshot
    still lacks a verification code, and non-pairing current-ready rewrites no
    longer leave a stale verification code behind in the bundle metadata. That
    keeps app-authored bundles closer to shell `source-pair`, `merge-bundle`,
    and `target-import` replay semantics while preserving the same fail-closed
    proof contract.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceBundleArtifactWriterTests|AcceptanceTwoMachineCurrentStateMergeTests|AcceptanceTwoMachineScriptTests/testAppAuthoredTargetReadySupportsShellSourcePairWithoutExplicitAddressFlags'`
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `sh -n macos/script/acceptance-two-machine.sh`
    - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleArtifactWriter.swift macos/SuperMoverAppTests/AcceptanceBundleArtifactWriterTests.swift docs/handoff-app-first-lan-acceptance.md docs/runbook.md docs/plan.md macos/README.md .bagakit/feature-tracker/features/f-23bnwxry2/verification.md`
  - Boundary: this is still acceptance-authoring parity tightening only. It
    does not add real two-machine installed-app evidence, non-dry-run
    distinct-machine transfer proof, or real Developer ID notarization /
    stapling / Gatekeeper closure.

- Step: T-011 receipt-path proof parity across app workflow, Swift evaluate, shell workflow-status, and shell evaluate.
  - Outcome: the bundle-local staged pairing receipt is now part of the
    shared proof gate instead of an authoring-only convention. App workflow
    summary / launch gating, Swift final evaluate, shell `workflow-status`,
    and shell `evaluate` all fail closed unless `source.pair.json.receipt_path`
    still resolves to a bundle-local regular file. `target_import` also
    reopens when `source_pair` loses that durable receipt proof, so advisory
    no longer stays greener than replayable shell semantics.
  - Evidence:
    - `swift test --package-path macos --filter 'Acceptance(BundleTests|EvaluationPathSafetyTests|EvaluationTests|InstalledAppProofParityTests|TwoMachineScriptTests)'`
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift macos/SuperMoverApp/AcceptanceBundleReader.swift macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceInstalledAppLaunchGateTests.swift macos/SuperMoverAppTests/AcceptanceEvaluationModeTests.swift macos/SuperMoverAppTests/AcceptanceEvaluationTests.swift macos/SuperMoverAppTests/AcceptanceEvaluationPathSafetyTests.swift macos/SuperMoverAppTests/AcceptanceBundleTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppProofParityTests.swift macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift docs/runbook.md docs/handoff-app-first-lan-acceptance.md .bagakit/feature-tracker/features/f-23bnwxry2/verification.md`
  - Boundary: this is proof-policy parity tightening only. It does not prove
    real two-machine installed-app execution, real Local Network/firewall
    operator evidence, or release-grade notarized app distribution.

- Step: T-011 review follow-up for receipt regular-file, report receipt-id, and baseline authority parity.
  - Outcome: review found three remaining local proof-policy holes after the
    first receipt-path parity slice. Swift artifact access now treats
    non-regular bundle-local nodes as unreadable artifacts, so a directory at
    `exported-receipts/<pairing_receipt_id>.json` no longer keeps app
    workflow summary or Swift final evaluate green. Shell final `evaluate` now
    rejects a stale `source.report.json.pairing.receipt_id` that does not match
    `source.pair.json.pairing_receipt_id`, and it resolves the current-source
    baseline from `source.consistency.json.baseline` before falling back to
    `meta.json.evidence.source_consistency.baseline`, matching the Swift final
    gate.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceEvaluationPathSafetyTests/testEvaluationCoordinatorFailsClosedWhenSourcePairReceiptArtifactIsDirectory|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsDirectory|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceReportReceiptMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceConsistencyArtifactBaselineEscape|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingSourcePairReceiptArtifact'`
  - Boundary: this closes local proof-policy drift only. It still does not
    prove the real two-machine installed-app run, real operator evidence, or
    release-grade notarization/Gatekeeper closure.

- Step: T-011 source-transfer advisory parity for report receipt, consistency baseline, and regular artifacts.
  - Outcome: app workflow summary and shell `workflow-status` now apply the
    same source-transfer proof checks that final evaluate already required.
    `source.report.json.pairing.receipt_id` must match
    `source.pair.json.pairing_receipt_id`; `source.consistency.json.baseline`
    takes precedence over stale meta baseline pointers; and app-side readiness
    no longer treats source-transfer transcript or baseline directories as
    present artifacts. Advisory now reopens `source_transfer` instead of
    advancing to `evaluate` when those proof inputs drift.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptMismatchesSourcePair|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryUsesSourceConsistencyArtifactBaselineBeforeMeta|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourceReportReceiptMismatchesSourcePair|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusUsesSourceConsistencyArtifactBaselineBeforeMeta'`
    - `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptMismatchesSourcePair|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryUsesSourceConsistencyArtifactBaselineBeforeMeta|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferTranscriptIsDirectory|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceConsistencyBaselineIsDirectory'`
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `sh -n macos/script/acceptance-two-machine.sh`
    - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleReader.swift macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceBundleTests.swift macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift`
  - Boundary: this is app/shell advisory parity only. It still does not prove
    real two-machine installed-app execution, real operator evidence, or
    release-grade notarized app distribution.

- Step: T-011 target control-plane ID path-safety parity.
  - Outcome: subagent review found that final evaluation interpolated
    `pairing_receipt_id` and `session_id` directly into target `.supermover`
    control-plane paths. Swift final evaluate, shell final evaluate, and
    app/shell advisory readiness now reject those IDs unless they are single
    safe path segments before looking up
    `.supermover/pairings/<id>.json` or
    `.supermover/sessions/<id>/network-transfer.json`.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceEvaluationPathSafetyTests/testEvaluationCoordinatorFailsClosedWhenPairingReceiptIDIsUnsafeControlPlanePathSegment|AcceptanceEvaluationPathSafetyTests/testEvaluationCoordinatorFailsClosedWhenSessionIDIsUnsafeControlPlanePathSegment|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsUnsafePairingReceiptIDControlPlaneSegment|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsUnsafeSessionIDControlPlaneSegment|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptMismatchesSourcePair|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryUsesSourceConsistencyArtifactBaselineBeforeMeta'`
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `sh -n macos/script/acceptance-two-machine.sh`
    - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleReader.swift macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceBundleTests.swift macos/SuperMoverAppTests/AcceptanceEvaluationPathSafetyTests.swift macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift docs/runbook.md docs/handoff-app-first-lan-acceptance.md .bagakit/feature-tracker/features/f-23bnwxry2/verification.md`
  - Boundary: this closes a target control-plane path-safety hole in the local
    proof substrate only. It does not provide real two-machine installed-app
    execution, real operator evidence, or release-grade notarized app
    distribution.

- Step: T-011 raw control-plane ID and target artifact parity follow-up.
  - Outcome: post-implementation review found remaining drift where app/shell
    advisory surfaces could trim `source.report.json.pairing.receipt_id` or
    `source.consistency.json.session_id` into values that final evaluate would
    reject, and where target `.supermover` evidence could be a directory or
    symlink. Swift app workflow, Swift final evaluate, shell `workflow-status`,
    and shell final `evaluate` now require raw `pairing_receipt_id` and
    `session_id` values to be safe single path segments: empty, `.`, `..`, `~`
    prefixed, slash/backslash-bearing, or whitespace-bearing IDs fail closed
    instead of being normalized. Target
    `.supermover/pairings/<id>.json` and
    `.supermover/sessions/<id>/network-transfer.json` evidence must be regular
    non-symlink files.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptMismatchesSourcePair|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptHasWhitespace|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceConsistencySessionHasWhitespace|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairIDIsUnsafeButTransferArtifactsLookReady|AcceptanceEvaluationPathSafetyTests/testEvaluationCoordinatorFailsClosedWhenPairingReceiptIDIsUnsafeControlPlanePathSegment|AcceptanceEvaluationPathSafetyTests/testEvaluationCoordinatorFailsClosedWhenSessionIDIsUnsafeControlPlanePathSegment|AcceptanceEvaluationPathSafetyTests/testEvaluationCoordinatorFailsClosedForUnsafeControlPlaneIDMatrix|AcceptanceEvaluationPathSafetyTests/testEvaluationCoordinatorFailsClosedWhenTargetPairingReceiptIsDirectory|AcceptanceEvaluationPathSafetyTests/testEvaluationCoordinatorFailsClosedWhenTargetNetworkTransferIsSymlink|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourceReportReceiptMismatchesSourcePair|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourceReportReceiptHasWhitespace|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourceConsistencySessionHasWhitespace|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourcePairIDIsUnsafeButTransferArtifactsLookReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsUnsafePairingReceiptIDControlPlaneSegment|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsPairingReceiptIDWithWhitespaceControlPlaneSegment|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsUnsafeSessionIDControlPlaneSegment|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSessionIDWithWhitespaceControlPlaneSegment|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsTargetPairingReceiptDirectory|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsTargetNetworkTransferSymlink'`
    - `swift test --package-path macos --filter 'Acceptance(BundleTests|EvaluationPathSafetyTests|EvaluationTests|InstalledAppProofParityTests|TwoMachineScriptTests)'` (139 tests executed, 7 skipped, 0 failures)
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `sh -n macos/script/acceptance-two-machine.sh`
    - `git diff --check -- macos/SuperMoverApp/AcceptanceControlPlaneID.swift macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceBundleTests.swift macos/SuperMoverAppTests/AcceptanceEvaluationPathSafetyTests.swift macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift docs/runbook.md docs/handoff-app-first-lan-acceptance.md .bagakit/feature-tracker/features/f-23bnwxry2/verification.md`
  - Boundary: this is local proof-policy and advisory/final parity tightening
    only. It does not prove real two-machine installed-app execution, real
    operator evidence, or release-grade signed/notarized/stapled distribution.

- Step: T-011 reviewer-driven bundle-artifact hardlink, receipt-schema, and
  Task Dispatch gate follow-up.
  - Outcome: independent proof/UI/docs reviewers found that shell final
    `evaluate` still used plain `test -f` for many bundle-local artifacts,
    Swift advisory treated a present exported pairing receipt as ready without
    validating its receipt schema, `meta.json` was not under the same
    single-link regular-file policy as other bundle artifacts, and Task
    Dispatch could enable `Profile Init` against an existing profile file. The
    shell final gate now resolves required bundle-local proof artifacts through
    a bundle-relative single-link regular-file helper before reading
    provenance, app audit, source-pair/source-transfer artifacts, verify,
    status, report, health, source consistency, current-source baseline,
    machine facts, notarization, and optional discovery artifacts. Swift
    `AcceptanceBundleReader` now validates the exported receipt body before
    marking `source_pair` / `target_import` ready, and
    `AcceptanceBundleArtifactAccess` and `AcceptanceBundleMetaStore` apply the
    same single-link regular-file policy to `meta.json`. Shell bundle
    bootstrap now rejects linked or non-regular `meta.json`, and
    `workflow-status` summary helpers also reject hardlinked machine-facts and
    release-evidence artifacts before treating them as ready. App
    `taskRunGate` now blocks `Profile Init` unless the selected profile path is
    a new source-owned destination, and Task Dispatch opens focused on the
    current task category instead of the whole mixed command inventory.
  - Evidence:
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `sh -n macos/script/acceptance-two-machine.sh`
    - `swift test --package-path macos --filter 'AppStoreTests/testTaskRunGateBlocksProfileInitForExistingProfileFile|AppStoreTests/testTaskRunGateBlocksExistingProfileTasksForNewProfileDestination|AppStoreTests/testTaskRunGateBlocksPublishWithoutSessionID|AppStoreTests/testTaskRunGateBlocksReconcileApplyWithoutReason|WorkbenchNavigationTests|WorkbenchChromeTests/testDetailPageStickyHeaderStopsAtPaneTopWithoutOvershoot'` (9 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testOperatorEvidenceStoreRejectsHardlinkedMetaBeforeWriting|AcceptanceBundleRootTrustTests/testAcceptanceBundleReaderRejectsHardlinkedMetaFile|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMalformed|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsDirectory|AcceptanceEvaluationModeTests|AcceptanceInstalledAppLaunchGateTests'` (25 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsBundleLocalArtifactHardlinks|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsTargetControlPlaneHardlinks'` (2 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsLinkedBundleMeta|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsLinkedBundleMeta|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotUseHardlinkedMachineFactArtifactsAsProof|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotUseHardlinkedReleaseArtifactsAsReady|AcceptanceBundleRootTrustTests/testAcceptanceBundleReaderRejectsSpecialMetaFile|AcceptanceBundleRootTrustTests/testOperatorEvidenceStoreRejectsSpecialMetaBeforeWriting|AcceptanceBundleRootTrustTests/testAcceptanceBundleReaderRejectsHardlinkedMetaFile|AcceptanceBundleRootTrustTests/testOperatorEvidenceStoreRejectsHardlinkedMetaBeforeWriting|AcceptanceBundleTests/testAcceptanceBundleReaderLoadsEvidenceCollectedBundle|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMalformed'` (10 tests, 0 failures)
  - Historical evidence correction: older tracker/artifact entries that used a
    single command shaped like `sh -n a.sh b.sh ...` should not be read as
    proof that every listed script was syntax-checked. Shell `sh -n` parses the
    first script operand; current evidence uses separate `sh -n` commands for
    each script.
  - Follow-up evidence added after review: `unpack-bundle` now rejects unsafe
    archive members and unpacked staging entries, including symlinks,
    hardlinks, and special files, before recording a verified handoff or
    publishing the incoming bundle root.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachinePackAndUnpackBundleRoundTripsEvidence|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRecordsVerifiedArchiveHandoffEvidence|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityIsTampered|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityFieldsAreMissing|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedOnMalformedArchive|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundlePreservesExistingBundleWhenArchiveMissingExportIdentity|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedOnArchiveDigestMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRejectsSymlinkedArchiveEntriesBeforePublish|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRejectsHardlinkedArchiveEntriesBeforePublish|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRejectsSpecialArchiveEntriesBeforePublish'` (10 tests, 0 failures)
  - Follow-up evidence added after typed source status/health review: shell
    final `evaluate` and Swift final evaluation now reject malformed
    `source.status.json` and `source.health.json` transfer evidence instead of
    accepting mostly presence/schema-light proof.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedSourceStatusArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedSourceHealthArtifact'` (4 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceEvaluationTests'` (28 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceEvaluationScriptTests'` (1 test, 0 failures)
  - Follow-up evidence added after advisory status/health drift review:
    shell `workflow-status` and Swift app workflow summary now apply matching
    typed status/health checks before advancing `source_transfer` to
    `evaluate`; malformed-but-decodable status/health artifacts reopen
    `source_transfer` instead of leaving advisory greener than final evaluate.
  - Red-first evidence before the fix:
    - `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceStatusSessionMismatchesTransfer|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceHealthLacksTransferSession'` failed because `source_transfer` was `done=true` and next action was `evaluate`.
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceStatusSessionMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceHealthWithoutTransferSession'` failed for the same advisory-over-green reason.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceStatusSessionMismatchesTransfer|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceHealthLacksTransferSession'` (2 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceStatusSessionMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceHealthWithoutTransferSession'` (2 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryAdvancesToEvaluateWithoutOperatorEvidenceWhenInstalledAppProofIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptMismatchesSourcePair|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptHasWhitespace|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceStatusSessionMismatchesTransfer|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceHealthLacksTransferSession|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceConsistencySessionHasWhitespace|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryUsesSourceConsistencyArtifactBaselineBeforeMeta'` (7 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedSourceVerifyCounts|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourceReportReceiptMismatchesSourcePair|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourceReportReceiptHasWhitespace|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceStatusSessionMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceHealthWithoutTransferSession|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourceConsistencySessionHasWhitespace|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusUsesSourceConsistencyArtifactBaselineBeforeMeta|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact'` (10 tests, 0 failures)
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
  - Follow-up evidence added after final-evaluate counter-semantics review:
    shell final `evaluate` now matches shell `workflow-status`, Swift app
    workflow summary, and Swift final evaluation for decoded
    `source.status.json` / `source.health.json` readiness counters. Negative
    or fractional counters fail closed before `evaluation.json` is written.
  - Red-first evidence before the fix:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsNonIntegralOrNegativeSourceStatusCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsNonIntegralOrNegativeSourceHealthCounters'`
      failed because shell final `evaluate` exited `0` and wrote
      `evaluation.json` for negative and fractional status/health counters.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsNonIntegralOrNegativeSourceStatusCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsNonIntegralOrNegativeSourceHealthCounters'`
      (2 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceVerifyCounts|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists'`
      (4 tests, 0 failures)
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
  - Post-commit review follow-up:
    Shell `workflow-status` and final `evaluate` now bound decoded
    status/health counters to non-negative Swift `Int`-decodable integers, matching
    Swift decoder acceptance instead of allowing out-of-range JSON numbers or
    scientific integers that Swift rejects.
  - Red-first evidence before the follow-up:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsOutOfRangeSourceStatusAndHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsOutOfRangeSourceStatusAndHealthCounters'`
      failed because shell advisory advanced to `evaluate` and shell final
      `evaluate` exited `0` / wrote `evaluation.json` for out-of-`Int` range
      counters.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsOutOfRangeSourceStatusAndHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsOutOfRangeSourceStatusAndHealthCounters'`
      (2 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsNonIntegralOrNegativeSourceStatusCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsNonIntegralOrNegativeSourceHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsOutOfRangeSourceStatusAndHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsOutOfRangeSourceStatusAndHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceVerifyCounts|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists'`
      (8 tests, 0 failures)
  - Second post-commit review follow-up:
    Shell final `evaluate` no longer relies on `jq` floating-point predicates for
    status/health counter proof; it validates raw JSON number tokens with a
    Swift-compatible integer rule before accepting current transfer evidence.
    Shell `workflow-status` also compares operator `machine_id` bindings as raw
    strings against raw v1 machine-facts artifacts instead of trimming them.
  - Red-first evidence before the second follow-up:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsOutOfRangeSourceStatusAndHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsOutOfRangeSourceStatusAndHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusUsesRawOperatorMachineBinding|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateUsesRawOperatorMachineBinding'`
      failed because shell final `evaluate` wrote `evaluation.json` for
      `9223372036854775807.0` / `9.223372036854775807e18`, and shell
      `workflow-status` trimmed operator machine bindings while final `evaluate`
      used raw strings.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsOutOfRangeSourceStatusAndHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsOutOfRangeSourceStatusAndHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusUsesRawOperatorMachineBinding|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateUsesRawOperatorMachineBinding'`
      (4 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsOutOfRangeSourceStatusAndHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsOutOfRangeSourceStatusAndHealthCounters|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusUsesRawOperatorMachineBinding|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateUsesRawOperatorMachineBinding|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRequiresOperatorEvidenceMachineBinding|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRequiresOperatorEvidenceMachineBinding|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsNonIntegralOrNegativeSourceStatusCounters|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsNonIntegralOrNegativeSourceHealthCounters'`
      (8 tests, 0 failures)
  - UX clarity evidence:
    Manual Evidence now shows whether the currently recorded operator evidence
    row is valid for the strict two-machine final-evaluate gate. Existing
    `pass` records missing a role-bound `machine_id`, or bound to the wrong
    source/target machine, are shown as gate-invalid instead of only displaying
    the raw `pass` text.
  - UX evidence:
    - `swift test --package-path macos --filter 'AcceptanceOperatorEvidenceTests'`
      (13 tests, 0 failures)
  - UX post-commit review follow-up:
    Manual Evidence strict-gate status now also uses the raw operator
    `machine_id` string and requires matching v1 source/target machine-facts
    artifacts before showing a record as gate-valid, matching shell final
    `evaluate` edge cases for whitespace-padded bindings and malformed machine
    facts.
  - UX red-first evidence before the follow-up:
    - `swift test --package-path macos --filter 'AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusUsesRawMachineIDBinding|AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusRejectsMalformedMachineFactsSchema'`
      failed because the app displayed both cases as `gate valid`.
  - UX green evidence:
    - `swift test --package-path macos --filter 'AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusUsesRawMachineIDBinding|AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusRejectsMalformedMachineFactsSchema|AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusExplainsInvalidPassBinding'`
      (3 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceOperatorEvidenceTests'`
      (13 tests, 0 failures)
  - UX second post-commit review follow-up:
    Manual Evidence strict-gate status now compares operator evidence to raw v1
    source/target machine-facts `machine_id` values instead of trimming the
    canonical machine-facts side before comparison.
  - UX red-first evidence before the second follow-up:
    - `swift test --package-path macos --filter 'AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusUsesRawMachineFactsMachineID|AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusUsesRawMachineIDBinding|AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusRejectsMalformedMachineFactsSchema'`
      failed because a canonical `target.machine.json` with `machine_id` ending
      in a space was displayed with inverted gate-valid/gate-invalid statuses.
  - UX green evidence:
    - `swift test --package-path macos --filter 'AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusUsesRawMachineFactsMachineID|AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusUsesRawMachineIDBinding|AcceptanceOperatorEvidenceTests/testManualEvidenceRecordGateStatusRejectsMalformedMachineFactsSchema'`
      (3 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceOperatorEvidenceTests'`
      (13 tests, 0 failures)
  - Boundary:
    This is proof-parity and operator clarity only. It does not create real
    Local Network/firewall/pairing-confirmation evidence, real two-Mac
    installed-app execution, or release-grade Developer ID notarization /
    staple / Gatekeeper evidence.
  - Follow-up evidence added after script-helper pipe-drain review:
    `AcceptanceScriptTestSupport.runProcessAllowFailure` now drains stdout and
    stderr concurrently while child processes run, and
    the remaining Swift script/integration launch helpers delegate to that
    shared harness.
  - Evidence:
    - `swift test --package-path macos --filter 'AcceptanceScriptHarnessTests/testRunProcessAllowFailureDrainsStdoutAndStderrWhileProcessRuns'` red first with old helper: exit 9, stdoutBytes=65536, stderrBytes=61455, missing finished markers.
    - `swift test --package-path macos --filter 'AcceptanceScriptHarnessTests/testRunProcessAllowFailureDrainsStdoutAndStderrWhileProcessRuns'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact|AcceptanceTwoMachineScriptTests/testTwoMachinePackAndUnpackBundleRoundTripsEvidence'` (3 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceScriptHarnessTests|AppAuditTamperTests|NotarizeAppScriptTests'` (17 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceEvaluationIntegrationTests|AcceptanceBundleAppOperationsIntegrationTests'` (24 tests skipped by env gates, 0 failures)
    - `rg -n "waitUntilExit\(\)|readDataToEndOfFile\(\)" macos/SuperMoverAppTests -g '*.swift'` now reports only `AcceptanceScriptTestSupport.swift`, the shared concurrent-drain implementation.
  - Follow-up evidence added after discovery artifact proof drift review:
    shell `workflow-status` and shell final `evaluate` now validate optional
    `source.browse.json` / `target.advertise.json` artifacts against the same
    required JSON shape Swift decodes before treating discovery steps as done.
    Trusted/status bits alone are no longer enough to advance to `evaluate`.
  - Red-first evidence before the fix:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetAdvertiseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetAdvertiseArtifact'` failed because shell `workflow-status` marked malformed discovery artifacts done and shell final `evaluate` wrote `evaluation.json`.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetAdvertiseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetAdvertiseArtifact'` (4 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedSourceVerifyCounts|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceVerifyCounts|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact|AcceptanceInstalledAppCollectionProofParityScriptTests/testEvaluateFailsClosedWhenRecordedAlternateDiscoveryArtifactIsInvalid'` (6 tests, 0 failures)
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `sh -n macos/script/acceptance-two-machine.sh`
  - Follow-up evidence added after target-ready proof drift review:
    canonical `target.ready.json` is now the current target-serve readiness
    proof, and it must match `meta.json` `evidence.target_ready` before shell
    `workflow-status`, Swift workflow summary, shell final `evaluate`, or
    Swift final evaluate treats target serve as ready.
  - Red-first evidence before the fix:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMalformed|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetReadyArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedTargetReadyArtifact'` failed because workflow surfaces marked meta-only `target_ready` done and final evaluators wrote `evaluation.json` or did not throw.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMalformed|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetReadyArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedTargetReadyArtifact'` (8 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetAdvertiseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetAdvertiseArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorWritesEvaluationArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorCanonicalMachineFactsPreferCanonicalArtifactsOverAlternateMetaOutputs|AcceptanceEvaluationTests/testEvaluationCoordinatorExplicitNonCanonicalSourceArtifactsStayAlignedWithShellEvaluate'` (8 tests, 0 failures)
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `sh -n macos/script/acceptance-two-machine.sh`
  - Follow-up evidence added after source pair/transfer target-ready
    consistency review:
    shell `workflow-status`, shell final `evaluate`, Swift workflow summary,
    and Swift final evaluate now fail closed when source-pair or
    source-transfer artifacts point at a different target endpoint than the
    canonical `target.ready.json` artifact. `target.ready.json` remains the
    first repair lane when it is missing or malformed; once it is valid,
    `source.pair.json.target_address` plus `source.transfer.json`
    target address / target mode / receiver endpoint must match before
    advisory can advance or `evaluation.json` can be written.
  - Red-first evidence before the fix:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairTargetAddressMismatchesTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferReceiverMismatchesTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferReceiverMismatchWithTargetReady'` failed because shell/app advisory still marked mismatched source pair/transfer evidence ready, and final evaluators wrote `evaluation.json` or did not throw.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairTargetAddressMismatchesTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferReceiverMismatchesTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferReceiverMismatchWithTargetReady'` (8 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMalformed|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetReadyArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceEvaluationTests/testEvaluationCoordinatorWritesEvaluationArtifact'` (10 tests, 0 failures)
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `sh -n macos/script/acceptance-two-machine.sh`
  - Follow-up evidence added after receiver transfer-readiness review:
    pairing-ready `target.ready.json` is no longer enough to advance
    `source_transfer` or final evaluation. Shell `workflow-status`, shell final
    `evaluate`, Swift workflow summary, and Swift final evaluate now require
    the canonical target-ready artifact to include receiver-transfer proof:
    non-empty `receiver_address`, `receiver_routes=true`, `push_network=true`,
    and `transfer=true`.
  - Red-first evidence before the fix:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferLacksReceiverReadyTargetArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof'` failed because shell/app advisory advanced to `evaluate` from a pairing-ready target artifact and final evaluators wrote `evaluation.json` or did not throw.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferLacksReceiverReadyTargetArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof'` (4 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMalformed|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairTargetAddressMismatchesTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferReceiverMismatchesTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferLacksReceiverReadyTargetArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetReadyArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedTargetReadyArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceEvaluationTests/testEvaluationCoordinatorWritesEvaluationArtifact'` (22 tests, 0 failures)
  - Follow-up evidence added after target-import referenced-transcript review:
    when `target_import.adopted` references `target.adopt-pairing.txt`, shell
    `workflow-status`, shell final `evaluate`, Swift workflow summary, and
    Swift final evaluate now require that referenced transcript to remain a
    bundle-local regular artifact before target import can count. The target
    control-plane pairing receipt remains the stronger adoption proof, but the
    bundle cannot preserve a stale transcript reference while advancing or
    writing `evaluation.json`.
  - Red-first evidence before the fix:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingReferencedTargetImportArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingReferencedTargetImportArtifact|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenReferencedTargetImportArtifactIsMissing|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingReferencedTargetImportArtifact'` failed because shell/app advisory advanced to `evaluate`, and final evaluators wrote `evaluation.json` or did not throw.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingReferencedTargetImportArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingReferencedTargetImportArtifact|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenReferencedTargetImportArtifactIsMissing|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingReferencedTargetImportArtifact'` (4 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourcePairReceiptArtifactIsMissing|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourcePairReceiptArtifactIsMalformed|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingReferencedTargetImportArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingReferencedTargetImportArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenReferencedTargetImportArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairTargetAddressMismatchesTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingReferencedTargetImportArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorWritesEvaluationArtifact'` (14 tests, 0 failures)
  - Follow-up evidence added after target-import adopted-field review:
    Swift workflow summary and Swift final evaluate now also require the
    `target_import.adopted` field itself to be non-empty before target import
    can count. A `target_import` object with only the paired receipt id no
    longer advances app advisory or writes `evaluation.json`.
  - Red-first evidence before the follow-up fix:
    - `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetImportAdoptedTranscriptIsMissing|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetImportAdoptedTranscript'` failed because Swift workflow summary marked `target_import` done and advanced to `evaluate`, while Swift final evaluate did not throw.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetImportAdoptedTranscriptIsMissing|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetImportAdoptedTranscript'` (2 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenReferencedTargetImportArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetImportAdoptedTranscriptIsMissing|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingReferencedTargetImportArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetImportAdoptedTranscript'` (4 tests, 0 failures)
  - Follow-up evidence added after notary submission-id review:
    release-ready notarization evidence now requires `submission.id` to be a
    UUID-shaped Apple notary submission id, not merely a non-empty string.
    App release evidence, app workflow summary, shell phase preflight, shell
    workflow/evaluate, and Swift final evaluate all fail closed on a malformed
    placeholder such as `manual-pass`.
  - Red-first evidence before the fix:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceTests/testNotarizationIsReleaseReadyRejectsMalformedSubmissionID|AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMalformedSourceNotarizationSubmissionID|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenSourceNotarizationSubmissionIDIsMalformed'` failed because those surfaces accepted `submission.id="manual-pass"` as release-ready or wrote `evaluation.json`.
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedSubmissionID'` failed with the old phase-preflight jq condition restored to the previous non-empty-string rule.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceTests/testNotarizationIsReleaseReadyRejectsMalformedSubmissionID|AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMalformedSourceNotarizationSubmissionID|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenSourceNotarizationSubmissionIDIsMalformed'` (5 tests, 0 failures)
  - First-class script source-workflow follow-up:
    `macos/script/notarize-app.sh` now applies the same UUID-shaped submission
    id requirement before notary-log retrieval or stapling, so the source
    script cannot write a local `status=pass` sidecar for a submit response
    that installed-app acceptance will later reject.
  - Red-first evidence before the script fix:
    - `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenSubmissionIDIsNotUUID'` failed with exit code 0 / `status=pass` because `submission.id="manual-pass"` reached log retrieval, stapling, and post-staple audit.
  - Green evidence:
    - `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenSubmissionIDIsNotUUID'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'NotarizeAppScriptTests'` (8 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceTests/testNotarizationIsReleaseReadyRejectsMalformedSubmissionID|AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMalformedSourceNotarizationSubmissionID|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenSourceNotarizationSubmissionIDIsMalformed'` (5 tests, 0 failures)
    - `sh -n macos/script/notarize-app.sh`
    - `git diff --check`
  - First-class script notary-log follow-up:
    `macos/script/notarize-app.sh` now also validates the fetched notary log
    before stapling, so a log with `status` other than `Accepted` cannot write
    a local `status=pass` sidecar for evidence that installed-app acceptance
    will later reject.
  - Red-first evidence before the script fix:
    - `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenNotaryLogIsNotAccepted'` failed with exit code 0 / `status=pass` because `notarytool log` returned `status="Invalid"` and the script still stapled/audited.
  - Green evidence:
    - `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenNotaryLogIsNotAccepted'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'NotarizeAppScriptTests'` (9 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedNotaryLog|AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationBoundToTargetNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMissingBundleLocalNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMalformedBundleLocalNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsNotarizationBoundToOtherMachineNotaryLog|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleLocalNotaryLogIsMalformed'` (6 tests, 0 failures)
    - `sh -n macos/script/notarize-app.sh`
    - `git diff --check`
  - First-class script accepted-log shape follow-up:
    `macos/script/notarize-app.sh` now matches the final gate's accepted-log
    shape: `status=Accepted` and `issues` absent/null or array. A log with
    `issues` encoded as a string no longer reaches stapler or writes a pass
    sidecar.
  - Red-first evidence before the issues-shape fix:
    - `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenNotaryLogIssuesIsMalformed'` failed with exit code 0 / `status=pass` because `notarytool log` returned `status="Accepted"` and `issues="none"`.
  - Green evidence:
    - `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenNotaryLogIssuesIsMalformed'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'NotarizeAppScriptTests'` (10 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedNotaryLog|AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationBoundToTargetNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMissingBundleLocalNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMalformedBundleLocalNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsNotarizationBoundToOtherMachineNotaryLog|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleLocalNotaryLogIsMalformed'` (6 tests, 0 failures)
    - `sh -n macos/script/notarize-app.sh`
    - `git diff --check`
  - Phase-preflight accepted-log follow-up:
    `acceptance_require_ready_app_audit_for_collection` now rejects the
    current sibling `notary-log.json` unless it matches the same accepted-log
    shape before copying `*.notarization.json` / `*.notary-log.json` into the
    acceptance bundle. A malformed sibling log with `issues` encoded as a
    string no longer lets a two-machine phase start and no longer leaves copied
    release evidence behind for later gates to reject.
  - Red-first evidence before the phase-preflight fix:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotaryLog'` failed with exit code 0 because collection preflight accepted `notary-log.json` containing `{"status":"Accepted","issues":"none"}` and copied `target.notarization.json` / `target.notary-log.json` into the bundle.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotaryLog'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionAcceptsDistributionReadyAudit|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotaryLog|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsNotarizationWithoutSubmissionID'` (4 tests, 0 failures)
    - `sh -n macos/script/lib/acceptance-common.sh`
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `sh -n macos/script/acceptance-two-machine.sh`
  - App-side packaging accepted-log follow-up:
    `AcceptancePackagingEvidenceCollector` now rejects the current sibling
    `notary-log.json` unless it matches the accepted-log shape before staging
    app-authored `*.notarization.json` / `*.notary-log.json` artifacts. Stale
    bundle-local `*.notary-log.json` leaves are also removed when notarization
    evidence is cleared.
  - Red-first evidence before the app-collector fix:
    - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts'` failed because the collector did not throw, published fresh `source.app-audit.json`, `source.provenance.json`, `source.notarization.json`, and `source.notary-log.json`, and left the bundle release evidence present even though sibling `notary-log.json` contained `{"status":"Accepted","issues":"none"}`.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorCopiesStructuredNotarizationEvidenceWhenPresent|AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts|AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedStructuredNotarizationEvidenceWithoutPublishingFreshPackagingArtifacts|AcceptancePackagingEvidenceTests/testCollectorRejectsStaleStructuredNotarizationEvidenceWithoutPublishingFreshPackagingArtifacts|AcceptancePackagingEvidenceTests/testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing'` (5 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedNotaryLog|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleLocalNotaryLogIsMalformed|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotaryLog'` (3 tests, 0 failures)
    - `git diff --check`
  - App-side stale notary-log leaf cleanup follow-up:
    `AcceptancePackagingEvidenceCollector` now removes stale copied
    `*.notarization.json` and `*.notary-log.json` leaves when they exist as
    dangling symlinks, matching the shell collection cleanup path's `rm -f`
    semantics instead of leaving invisible stale proof nodes in the bundle.
  - Red-first evidence before the symlink-leaf cleanup fix:
    - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing'` failed because stale `source.notary-log.json` remained as a dangling symlink after local notarization evidence was cleared.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing|AcceptancePackagingEvidenceTests/testCollectorCopiesStructuredNotarizationEvidenceWhenPresent|AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts|AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedStructuredNotarizationEvidenceWithoutPublishingFreshPackagingArtifacts|AcceptancePackagingEvidenceTests/testCollectorRejectsStaleStructuredNotarizationEvidenceWithoutPublishingFreshPackagingArtifacts'` (5 tests, 0 failures)
  - Shell collection output-leaf safety follow-up:
    `acceptance_record_notarization_evidence_if_present` now applies the same
    single-link regular-file overwrite preflight to the bundle-local
    `*.notary-log.json` output leaf that it already applied to
    `*.notarization.json`, so shell `record-packaging-evidence` no longer
    overwrites a symlinked notary-log proof leaf and publishes workflow summary.
  - Red-first evidence before the shell notary-log output fix:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotaryLogOutputLeaf'` failed because shell `record-packaging-evidence` exited 0, replaced the symlinked `source.notary-log.json`, and wrote `workflow.summary.json`.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotaryLogOutputLeaf'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceWritesReadyArtifactsForSourceMachine|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotarizationOutputLeaf|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotaryLogOutputLeaf|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsStaleSiblingNotarizationWhoseAuditDoesNotMatchCurrentApp'`
      (4 tests, 0 failures)
    - `sh -n macos/script/lib/acceptance-common.sh`
    - `git diff --check`
  - App-side output-leaf hardlink follow-up:
    `AcceptancePackagingEvidenceCollector` now applies the same existing-output
    single-link regular-file preflight to app-authored bundle-local version,
    provenance, app-audit, notarization, and notary-log output leaves before
    staging or publishing fresh packaging evidence. A hardlinked
    `source.notary-log.json` output leaf no longer lets the app rewrite linked
    release evidence or leave partial fresh packaging artifacts behind.
  - Red-first evidence before the app-output hardlink fix:
    - `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsHardlinkedNotaryLogOutputLeafBeforeWriting'` failed because the collector did not throw, wrote fresh mandatory packaging outputs, and overwrote the hardlinked bundle-local notary-log leaf.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsHardlinkedNotaryLogOutputLeafBeforeWriting'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsHardlinkedNotaryLogOutputLeafBeforeWriting|AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsSymlinkedNotarizationOutputLeaf'` (2 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorCopiesStructuredNotarizationEvidenceWhenPresent|AcceptancePackagingEvidenceTests/testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing|AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts'` (3 tests, 0 failures)
  - Shell mandatory-output hardlink follow-up:
    `acceptance_record_app_audit` and its CLI fact helper now apply the same
    existing-output single-link regular-file preflight to bundle-local
    `*.version.txt`, `*.provenance.json`, and `*.app-audit.json` leaves before
    shell packaging collection writes fresh evidence. App-audit output now
    stages through a temporary file before publishing, so a hardlinked
    `source.app-audit.json` output leaf no longer rewrites external evidence or
    publishes `workflow.summary.json`.
  - Red-first evidence before the shell mandatory-output fix:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsHardlinkedBundleAppAuditOutputLeaf'` failed because shell `record-packaging-evidence` exited 0, overwrote the external hardlink target with app-audit JSON, and wrote `workflow.summary.json`.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsHardlinkedBundleAppAuditOutputLeaf'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceWritesReadyArtifactsForSourceMachine|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsHardlinkedBundleAppAuditOutputLeaf|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotarizationOutputLeaf|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotaryLogOutputLeaf'` (4 tests, 0 failures)
    - `sh -n macos/script/lib/acceptance-common.sh`
  - App-side phase artifact output-leaf hardlink follow-up:
    `AcceptanceBundleArtifactWriter` now applies the same existing-output
    single-link regular-file preflight to app-authored bundle-local phase
    artifact output leaves before writing machine facts, discovery,
    target-ready, exported receipt, source-pair/source-transfer, transcript,
    source-consistency, or evaluation evidence. A hardlinked `source.pair.json`
    output leaf no longer lets app-side source pairing rewrite external
    evidence or publish partial machine-facts/meta state.
  - Red-first evidence before the app-phase output hardlink fix:
    - `swift test --package-path macos --filter 'AcceptanceBundleArtifactWriterTests/testWriterRejectsHardlinkedSourcePairOutputLeafBeforeWritingPhaseArtifacts'` failed because `writeSourcePair` did not throw, wrote `source.machine.json`, `source.pair.txt`, and `exported-receipts/pair-1.json`, and updated source-pair metadata while `source.pair.json` was hardlinked outside the bundle.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceBundleArtifactWriterTests/testWriterRejectsHardlinkedSourcePairOutputLeafBeforeWritingPhaseArtifacts'` (1 test, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceBundleArtifactWriterTests/testWriterRecordsServePairAndTransferArtifacts|AcceptanceBundleArtifactWriterTests/testWriterRecordsDiscoveryArtifacts|AcceptanceBundleArtifactWriterTests/testWriterSourcePairRewritesCanonicalSourceMachineIdentityForInstalledAppProof|AcceptanceBundleArtifactWriterTests/testWriterServePhaseRewritesCanonicalTargetMachineIdentityForInstalledAppProof|AcceptanceBundleArtifactWriterTests/testWriterRejectsHardlinkedSourcePairOutputLeafBeforeWritingPhaseArtifacts'` (5 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testAcceptanceBundleArtifactWriterRejectsSymlinkedBundleRootBeforeWriting|AcceptanceBundleRootTrustTests/testAcceptanceBundleArtifactWriterRejectsSymlinkedMetaBeforeWriting|AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsHardlinkedNotaryLogOutputLeafBeforeWriting|AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsSymlinkedNotarizationOutputLeaf'` (4 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceBundleArtifactWriterTests'` (17 tests, 0 failures)
  - Open follow-ups from review: the release-evidence and machine-facts summary
    helpers now enforce the same hardlink fail-closed behavior, though their
    duplicated shell/Python helper code can still be consolidated.
  - Boundary: this is proof-policy and Task Dispatch correctness tightening
    only. It does not add real two-machine installed-app execution, real Local
    Network/firewall/operator evidence, or Developer ID notarization / staple /
    Gatekeeper closure.
  - Prepare-page migration-config clarity follow-up:
    `AppStore.setupGuide` now drives the Prepare page through Role, Migration
    Config, Root Inputs, and Config Checks. The UI labels the
    profile SSOT as Migration Config for operators while preserving CLI
    `--profile` semantics, and only shows `Update Existing Config Target` when
    the selected config path is an existing file.
  - Red-first evidence during implementation:
    - Focused setup-guide tests failed before implementation because
      `AppStore` had no `setupGuide` model.
  - Green evidence:
    - `swift test --package-path macos --filter 'AppStoreTests/testSetupGuideExplainsEmptySourcePreparationInUserOrder|AppStoreTests/testSetupGuideShowsNewConfigDestinationAsCreationStep|AppStoreTests/testSetupGuideShowsExistingTargetConfigWithoutCreationCTA|AppStoreTests/testProfileDestinationPlanInitializesNewSourceProfileWhenRootsAreReady|AppStoreTests/testProfileDestinationPlanStaysSelectionOnlyUntilRootsAreReady|AppStoreTests/testNewProfileDestinationDoesNotSatisfyExistingProfileTasks|AppStoreTests/testTaskRunGateBlocksExistingProfileTasksForNewProfileDestination|AppStoreTests/testTaskRunGateBlocksProfileInitForExistingProfileFile'`
      (8 tests, 0 failures)
    - `swift test --package-path macos --filter 'WorkbenchNavigationTests|WorkbenchChromeTests'`
      (12 tests, 0 failures)
    - `feature-tracker validate-tracker --root .`
    - `git diff --check`
  - Broad-check caveat:
    `swift test --package-path macos --filter 'AppStoreTests|WorkbenchNavigationTests|WorkbenchChromeTests'`
    executed 120 tests and failed 11 acceptance installed-app
    launch/preflight/notarization expectation tests tied to local release-ready
    packaging evidence state; the focused setup/profile/workbench tests above
    passed. T-011 remains partial.
  - Subagent review follow-up:
    A read-only post-commit reviewer found two Prepare-page semantic issues:
    `Read Status` success did not satisfy target/observer config validation,
    and `Roots Saved In Config` overstated root fields that are not loaded from
    existing config files. The follow-up now treats current successful
    `status` runs as validation for target/observer roles, renames the roots
    section to `Root Inputs`, and marks empty existing-config root inputs as
    neutral because lint/status still read the selected config file.
  - Green evidence after the review follow-up:
    - `swift test --package-path macos --filter 'AppStoreTests/testSetupGuideExplainsEmptySourcePreparationInUserOrder|AppStoreTests/testSetupGuideShowsNewConfigDestinationAsCreationStep|AppStoreTests/testSetupGuideShowsExistingTargetConfigWithoutCreationCTA|AppStoreTests/testSetupGuideCountsSuccessfulStatusAsObserverValidation|AppStoreTests/testSetupGuideCountsSuccessfulStatusAsTargetValidation|AppStoreTests/testSetupGuideClarifiesExistingConfigRootInputsAreNotLoadedRoots|AppStoreTests/testProfileDestinationPlanInitializesNewSourceProfileWhenRootsAreReady|AppStoreTests/testProfileDestinationPlanStaysSelectionOnlyUntilRootsAreReady|AppStoreTests/testNewProfileDestinationDoesNotSatisfyExistingProfileTasks|AppStoreTests/testTaskRunGateBlocksExistingProfileTasksForNewProfileDestination|AppStoreTests/testTaskRunGateBlocksProfileInitForExistingProfileFile'`
      (11 tests, 0 failures)
    - `swift test --package-path macos --filter 'WorkbenchNavigationTests|WorkbenchChromeTests'`
      (12 tests, 0 failures)
    - `feature-tracker validate-tracker --root .`
    - `git diff --check`
  - Settings display-preference clarity follow-up:
    Settings now has a right-side Display Preferences panel separate from
    Command Inputs. Appearance and interface-language choices are stored in a
    UI-only preferences store and applied at the app/window layer; they do not
    enter `AppStore`, profile files, `TaskInput`, command preview arguments,
    setup context signatures, task gates, evidence bundles, or CLI output.
    Dynamic color tokens now make light/dark appearance selection real while
    separating `SMColor.card` surface usage from `SMColor.inverseText` ink on
    primary/dark controls.
  - Red-first evidence during implementation:
    - Focused UI preference tests failed before implementation because
      `UIAppearancePreference`, `UILanguagePreference`, `UIPreferencesStore`,
      and the `WorkbenchWindowChrome.apply(... appearanceName:)` parameter did
      not exist.
  - Green evidence:
    - `swift test --package-path macos --filter 'UIPreferencesTests|WorkbenchChromeTests|WorkbenchNavigationTests|AppStoreTests/testUIPreferencesDoNotChangeCommandInputsOrPreviewContracts'`
      (18 tests, 0 failures)
  - Boundary:
    This is UI clarity and appearance/language preference infrastructure only.
    It does not prove full string localization, real two-machine installed-app
    execution, Local Network/firewall/operator evidence, Developer ID
    notarization, stapling, Gatekeeper acceptance, or final T-011 closure.
- Step: T-011 app-audit readiness parity for installed-app release evidence.
  - Outcome: app-side release evidence, launch gates, packaging probes, shell
    phase preflight, and shell workflow-status now require app audit artifacts
    to report `readiness=distribution_ready` in addition to `status=pass` and
    `summary.pass_ready=true`. A `review_only` audit remains a
    packaging-evidence correction lane instead of install-ready proof.
  - Red-first evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchGateTests/testGateFailsClosedWhenPackagingAuditPassesButIsNotDistributionReady|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellWorkflowStatusRejectsSourceAppAuditThatPassesButIsReviewOnly'`
      failed before implementation because the app launch gate returned
      `notarizationNotReady` and shell workflow-status only reported the
      notarization failure.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchGateTests|AcceptancePackagingEvidenceTests/testCollectorInspectionTreatsReviewOnlyAppAuditAsNotInstallReady|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellWorkflowStatusRejectsSourceAppAuditThatPassesButIsReviewOnly'`
      (21 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceTests|AcceptanceInstalledAppLaunchGateTests|AcceptanceInstalledAppReleaseEvidenceScriptTests|AcceptanceInstalledAppReleaseEvidenceEvaluationScriptTests|AcceptancePackagingEvidenceTests|AcceptanceInstalledAppBundleContextTests|AcceptanceBundleTests/testLoadedSnapshotDecodesAcceptanceArtifacts'`
      (61 tests, 0 failures, 2 opt-in real-packaging tests skipped)
  - Boundary:
    This does not create real two-Mac installed-app evidence, Local Network /
    firewall operator evidence, Developer ID notarization / staple /
    Gatekeeper proof, or final T-011 closure.
- Step: T-011 post-commit review coverage for phase-preflight app-audit
    readiness.
  - Outcome: `acceptance_require_ready_app_audit_for_collection` now has a
    direct negative regression for `status=pass`, `summary.pass_ready=true`,
    and `readiness=review_only`. The test requires phase preflight to stop at
    the install-ready app-audit blocker before falling through to the
    notarization gate.
  - Review source:
    post-commit quality review found this as a low-risk coverage gap after the
    `b1f3c21` readiness-parity commit.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsReviewOnlyAuditBeforeNotarizationGate'`
      (1 test, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionAcceptsDistributionReadyAudit|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsReviewOnlyAuditBeforeNotarizationGate|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotaryLog|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsNotarizationWithoutSubmissionID'`
      (5 tests, 0 failures)
    - `sh -n macos/script/lib/acceptance-common.sh`
    - `sh -n macos/script/lib/acceptance-two-machine.sh`
    - `git diff --check`
- Step: T-011 local app-audit notarization sidecar release-ready parity.
  - Outcome: `macos/script/audit-app.sh` and the Swift
    `PackagedAppAuditor` no longer mark an existing canonical notarization
    sidecar as release-ready on status bits alone. Existing sidecars must now
    retain the same strict local release fields required by installed-app
    acceptance: supported `auth_mode`, UUID-shaped Apple submission id,
    `failure` absent/null, accepted sibling notary-log JSON, and a
    distribution-ready post-staple audit.
  - Read-only audit source:
    a subagent release-proof review found the shell audit helper weaker than
    `acceptance_require_ready_app_audit_for_collection`,
    `workflow-status`/`evaluate`, and Swift installed-app release evidence on
    `auth_mode`, submission UUID, and `failure` handling.
  - Red-first evidence:
    - `swift test --package-path macos --filter 'PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenSubmissionIDIsNotUUID|PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenAuthModeIsMissing|PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenFailureIsRecorded|PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenNotaryLogIsNotAccepted|AppAuditTamperTests/testAppAuditBlocksCanonicalNotarizationSidecarWhenSubmissionIDIsNotUUID|AppAuditTamperTests/testAppAuditBlocksCanonicalNotarizationSidecarWhenAuthModeIsMissing|AppAuditTamperTests/testAppAuditBlocksCanonicalNotarizationSidecarWhenFailureIsRecorded|AppAuditTamperTests/testAppAuditBlocksCanonicalNotarizationSidecarWhenNotaryLogIsNotAccepted'`
      failed before implementation because both local audit paths accepted or
      failed to block current sidecars with malformed submission ids, missing
      auth mode, non-null failure, or malformed notary-log evidence.
  - Green evidence:
    - `swift test --package-path macos --filter 'PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenSubmissionIDIsNotUUID|PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenAuthModeIsMissing|PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenFailureIsRecorded|PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenNotaryLogIsNotAccepted|AppAuditTamperTests/testAppAuditBlocksCanonicalNotarizationSidecarWhenSubmissionIDIsNotUUID|AppAuditTamperTests/testAppAuditBlocksCanonicalNotarizationSidecarWhenAuthModeIsMissing|AppAuditTamperTests/testAppAuditBlocksCanonicalNotarizationSidecarWhenFailureIsRecorded|AppAuditTamperTests/testAppAuditBlocksCanonicalNotarizationSidecarWhenNotaryLogIsNotAccepted'`
      (8 tests, 0 failures)
    - `swift test --package-path macos --filter 'PackagedAppAuditorTests|AppAuditTamperTests|NotarizeAppScriptTests'`
      (42 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceTests|AcceptanceInstalledAppReleaseEvidenceScriptTests|AcceptanceInstalledAppReleaseEvidenceEvaluationScriptTests|AcceptancePackagingEvidenceTests|AcceptanceInstalledAppLaunchGateTests'`
      (54 tests, 0 failures, 2 opt-in real-packaging tests skipped)
    - `sh -n macos/script/audit-app.sh`
    - `sh -n macos/script/lib/acceptance-common.sh`
  - Boundary:
    This is local release-proof parity hardening only. It does not produce real
    Developer ID notarization/staple/Gatekeeper evidence, real two-Mac
    installed-app evidence, or final T-011 closure.
- Step: T-011 missing-machine-facts launch correction fail-closed parity.
  - Outcome: app-side installed-app launch preview/preflight now treats missing
    role/machine-facts evidence as the same constrained machine-identity
    correction lane that workflow summary already reports. `source pair` and
    `target serve` remain review/launchable corrective rewrites, while
    unrelated tasks such as `networkPush` block before packaging evidence is
    written.
  - Read-only audit source:
    a subagent proof-path review confirmed that
    `AcceptanceInstalledAppCollectionProofSummary.requiresMachineIdentityCorrection`
    was true for missing machine facts, but `machineIdentityCorrectionLaunch`
    only exposed corrections for concrete `blockedReason` conflicts. That let
    the launch coordinator treat `.installedAppProofIncomplete` as generally
    launchable for unrelated tasks.
  - Red-first evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppCollectionProofTests/testMachineIdentityCorrectionLaunchAllowsMissingMachineFactCorrection|AcceptanceInstalledAppLaunchCoordinatorTests/testPreviewReviewsPairAndServeWhenMachineFactsAreMissing|AcceptanceInstalledAppLaunchCoordinatorTests/testPreflightAllowsPairAndServeWhenMachineFactsAreMissing|AcceptanceInstalledAppLaunchCoordinatorTests/testPreflightBlocksUnrelatedTaskWhenMachineFactsAreMissing'`
      failed before implementation because missing-machine-facts correction
      returned nil from the proof owner, unrelated `networkPush` preflight
      returned nil, and preview did not present pair/serve as correction
      launches.
  - Green evidence:
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppCollectionProofTests/testMachineIdentityCorrectionLaunchAllowsMissingMachineFactCorrection|AcceptanceInstalledAppLaunchCoordinatorTests/testPreviewReviewsPairAndServeWhenMachineFactsAreMissing|AcceptanceInstalledAppLaunchCoordinatorTests/testPreflightAllowsPairAndServeWhenMachineFactsAreMissing|AcceptanceInstalledAppLaunchCoordinatorTests/testPreflightBlocksUnrelatedTaskWhenMachineFactsAreMissing'`
      (4 tests, 0 failures)
    - `swift test --package-path macos --filter 'AcceptanceInstalledAppCollectionProofTests|AcceptanceInstalledAppLaunchGateTests|AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppProofParityTests|AcceptanceInstalledAppWorkflowSummaryTests'`
      (88 tests, 0 failures)
  - Boundary:
    This is app-side proof/advisory parity only. It does not produce real
    two-Mac installed-app evidence, Local Network/firewall/operator evidence,
    Developer ID notarization/staple/Gatekeeper proof, or final T-011 closure.
- Step: T-011 bounded app chrome localization and packaged resource parity.
  - Outcome: the Settings language switch now localizes bounded app-owned
    chrome through packaged English and Simplified Chinese resources: sidebar
    labels, status/safety chrome, Settings page text, Display Preferences
    labels, and preference options. `System` language follows supported OS
    preferred languages, and lookup works from packaged app resource
    directories as well as SwiftPM resource bundles. Navigation identity, task
    titles, command previews, `TaskInput`-derived CLI arguments, context
    signatures, task gates, evidence/proof values, artifact fields, and CLI
    output stay raw and audit-stable. SwiftPM resources are processed with
    English as the default localization, and `build-app.sh` copies root-level
    `.lproj` directories into packaged app resources.
  - Read-only audit source:
    subagents independently identified the current language switch as
    infrastructure-only, recommended a small resource-backed app chrome seam,
    and warned that proof/command/evidence strings must remain unlocalized.
  - Red-first evidence:
    - `swift test --package-path macos --filter 'UIPreferencesTests|WorkbenchNavigationTests'`
      failed before implementation because `AppChromeLocalization`,
      localized navigation helpers, and localized resource lookup did not
      exist.
    - `swift test --package-path macos --filter 'AcceptanceScriptHarnessTests/testBuildAppResourceCopyIncludesProcessedLocalizationDirectories'`
      failed before implementation because the packaged-app resource copy
      helper did not exist and SwiftPM root-level `.lproj` directories were not
      copied into `Contents/Resources`.
  - Green evidence:
    - `swift test --package-path macos --filter 'UIPreferencesTests|WorkbenchNavigationTests|AppStoreTests/testUIPreferencesDoNotChangeCommandInputsOrPreviewContracts|AcceptanceScriptHarnessTests/testBuildAppResourceCopyIncludesProcessedLocalizationDirectories'`
      (15 tests, 0 failures)
    - `sh -n macos/script/build-app.sh`
    - `sh -n macos/script/lib/app-build-resources.sh`
    - `macos/script/build-app.sh`: pass; produced an unsigned local app at
      `macos/dist/SuperMover.app`.
    - `test -f macos/dist/SuperMover.app/Contents/Resources/en.lproj/Localizable.strings`:
      pass.
    - `test -f macos/dist/SuperMover.app/Contents/Resources/zh-hans.lproj/Localizable.strings`:
      pass; this confirms the SwiftPM-normalized Simplified Chinese resource
      directory is present in the packaged app.
  - Post-commit review follow-up:
    A read-only review found the first localization commit had helper-test and
    shell-syntax coverage, but no direct packaged `.app` inspection. The real
    unsigned build and two file checks above close that artifact-evidence gap.
  - Boundary:
    This is bounded UI and packaging polish only. It does not prove full app
    localization, two-Mac installed-app execution, Local Network/firewall/
    operator evidence, Developer ID notarization/staple/Gatekeeper proof, or
    final T-011 closure.
- Step: T-011 Prepare/setup app chrome localization.
  - Outcome: the language switch now also localizes the Prepare page header,
    role selector labels/summaries, setup guide titles/details/status/action
    labels, config/root/check cards, root labels/placeholders, setup checklist,
    visible config-state badges, visible role chrome in headers/task dispatch/
    transfer/control-room context, profile-selection guidance, and the
    Advanced disclosure through the same packaged English and Simplified
    Chinese resource seam. Raw `WorkbenchRole.title`,
    `WorkbenchRole.allowedSetup`, raw `SetupGuide`, task titles, command
    previews, `TaskInput`-derived CLI arguments, context signatures, task
    gates, profile/config paths, evidence/proof values, artifact fields, and
    CLI output remain raw and audit-stable.
  - Read-only audit source:
    subagents independently identified Prepare/onboarding and role/setup-guide
    strings as the next app-owned i18n gap, while warning that command,
    evidence, and proof values must not be localized.
  - Red-first evidence:
    - `swift test --package-path macos --filter 'UIPreferencesTests/testAppChromeLocalizationLoadsPrepareChromeResources|AppStoreTests/testLocalizedSetupGuideExplainsEmptySourcePreparationWithoutChangingRawGuide|AppStoreTests/testLocalizedSetupGuideShowsTargetValidationActions|AppStoreTests/testWorkbenchRoleLocalizedLabelsDoNotChangeRoleIdentity'`
      failed before implementation because `setup.*` localization keys,
      `AppStore.localizedSetupGuide(using:)`, and display-only
      `WorkbenchRole.localized*` accessors did not exist.
  - Green evidence:
    - `swift test --package-path macos --filter 'UIPreferencesTests/testAppChromeLocalizationLoadsPrepareChromeResources|AppStoreTests/testLocalizedSetupGuideExplainsEmptySourcePreparationWithoutChangingRawGuide|AppStoreTests/testLocalizedSetupGuideShowsTargetValidationActions|AppStoreTests/testWorkbenchRoleLocalizedLabelsDoNotChangeRoleIdentity'`
      (4 tests, 0 failures)
    - Post-review red/green:
      `swift test --package-path macos --filter 'UIPreferencesTests/testAppChromeLocalizationLoadsPrepareChromeResources|UIPreferencesTests/testVisibleWorkbenchRoleChromeDoesNotReadRawRoleTitles|AppStoreTests/testLocalizedProfileSelectionDisplayDoesNotChangeRawConfigValues'`
      failed first because the profile-selection display API and Advanced key
      were missing; after the review fix it passed with 3 tests, 0 failures.
    - `swift test --package-path macos --filter 'UIPreferencesTests|WorkbenchNavigationTests|AppStoreTests/testSetupGuide|AppStoreTests/testLocalizedSetupGuide|AppStoreTests/testLocalizedProfileSelectionDisplayDoesNotChangeRawConfigValues|AppStoreTests/testWorkbenchRoleLocalizedLabelsDoNotChangeRoleIdentity|AppStoreTests/testUIPreferencesDoNotChangeCommandInputsOrPreviewContracts|AcceptanceScriptHarnessTests/testBuildAppResourceCopyIncludesProcessedLocalizationDirectories'`
      (27 tests, 0 failures)
    - Python resource-key audit: English and Simplified Chinese
      `Localizable.strings` both have 136 keys, no duplicates, and no
      missing counterpart keys.
    - Final closeout rerun after replacing remaining raw visible role badge
      labels and transfer metadata prefixes:
      `swift test --package-path macos --filter 'UIPreferencesTests|WorkbenchNavigationTests|AppStoreTests/testSetupGuide|AppStoreTests/testLocalizedSetupGuide|AppStoreTests/testLocalizedProfileSelectionDisplayDoesNotChangeRawConfigValues|AppStoreTests/testWorkbenchRoleLocalizedLabelsDoNotChangeRoleIdentity|AppStoreTests/testUIPreferencesDoNotChangeCommandInputsOrPreviewContracts|AcceptanceScriptHarnessTests/testBuildAppResourceCopyIncludesProcessedLocalizationDirectories'`
      (27 tests, 0 failures)
    - Final closeout resource-key audit: English and Simplified Chinese
      `Localizable.strings` both have 136 keys, no duplicates, and no missing
      counterpart keys.
    - `swift build --package-path macos --product SuperMoverApp`: pass.
    - `feature-tracker validate-tracker --root .`:
      pass.
    - `git diff --check`: pass.
    - `sh macos/script/build-app.sh`: pass; produced an unsigned local app at
      `macos/dist/SuperMover.app`.
  - Review follow-up:
    A read-only reviewer found visible raw role labels and profile-selection
    guidance still leaking through app-owned Prepare/task/transfer/control-room
    chrome. The follow-up localized those display-only surfaces and added
    regression tests while leaving raw command/evidence contracts unchanged.
    A final closeout pass also made the static regression reject raw
    `label: "role"` badge labels and `Role •` metadata prefixes in app-owned
    chrome.
  - Boundary:
    This is bounded UI clarity and i18n polish only. It does not prove full app
    localization, Playwright/browser coverage for the native SwiftUI app,
    two-Mac installed-app execution, Local Network/firewall/operator evidence,
    Developer ID notarization/staple/Gatekeeper proof, or final T-011 closure.
- Step: T-011 final local closeout after session `019e8d41-f71d-7780-b30e-d31e0ddf06c5`.
  - Outcome: the interrupted i18n/UI-preferences closeout has been carried
    through to a local closed loop. Visible role chrome no longer uses raw
    `label: "role"` or `Role •` prefixes in app-owned display surfaces; the
    full Swift suite also exposed AppStoreTests fixture drift against the
    current release-proof contract, so the fixture now writes bundle-local
    notary logs, bundle-relative `notary_log.path` fields, and raw
    machine-bound operator evidence.
  - Green evidence:
    - `swift test --package-path macos --filter 'UIPreferencesTests|WorkbenchNavigationTests|AppStoreTests/testSetupGuide|AppStoreTests/testLocalizedSetupGuide|AppStoreTests/testLocalizedProfileSelectionDisplayDoesNotChangeRawConfigValues|AppStoreTests/testWorkbenchRoleLocalizedLabelsDoNotChangeRoleIdentity|AppStoreTests/testUIPreferencesDoNotChangeCommandInputsOrPreviewContracts|AcceptanceScriptHarnessTests/testBuildAppResourceCopyIncludesProcessedLocalizationDirectories'`
      (27 tests, 0 failures)
    - `swift test --package-path macos --filter 'AppStoreTests'`
      (116 tests, 0 failures)
    - `swift test --package-path macos`
      (692 tests, 33 skipped, 0 failures)
    - `go test -count=1 ./...`: pass.
    - Python resource-key audit: English and Simplified Chinese
      `Localizable.strings` both have 136 keys, no duplicates, and no missing
      counterpart keys.
    - `swift build --package-path macos --product SuperMoverApp`: pass.
    - `sh macos/script/build-app.sh`: pass; produced an unsigned local app at
      `macos/dist/SuperMover.app`.
  - Test-tiering note:
    `swift test --package-path macos` is useful as a release/hygiene gate but
    slow as a UI inner loop because it includes shell-backed acceptance,
    two-machine proof, packaging, notarization, symlink/hardlink, and
    fail-closed matrix tests. Future UI-only changes should start with the
    focused UI/resource/build subset, use AppStore/release-contract filters
    when touching packaging or launch proof, and reserve full Swift for
    closeout or pre-merge validation.
  - Boundary:
    This closes the current local implementation/documentation hygiene loop
    only. It still does not provide real two-Mac installed-app execution,
    Local Network/firewall/operator prompt evidence, Developer ID
    notarization/staple/Gatekeeper proof, or final T-011 release closure.
- Step: T-011 Prepare profile creation UX follow-up.
  - Outcome: the basic Source setup now presents a single recommended setup
    summary instead of competing config-file actions. Creating the setup still
    selects the recommended `~/.supermover/profile-local.json` path before
    running `profile init`, while existing config selection, custom destination
    selection, raw file paths, and identity fields live under Advanced options.
    Empty local directory fields no longer show orange access badges. Source
    setup now asks for a readable Source directory on this Mac plus a
    target-owned destination path string; it no longer asks the Source Mac to
    browse or validate the Target Mac's destination as a local writable
    directory. A follow-up copy audit also removed `target root` /
    `current roots` wording from Source profile-init summaries, profile
    selection guidance, role setup copy, and Task Dispatch input summaries.
  - Green evidence:
    - `swift test --package-path macos --filter 'AppStoreTests/testSetupGuideExplainsEmptySourcePreparationInUserOrder|AppStoreTests/testLocalizedSetupGuideExplainsEmptySourcePreparationWithoutChangingRawGuide|AppStoreTests/testSetupGuideShowsNewConfigDestinationAsCreationStep|AppStoreTests/testSetupGuideShowsExistingTargetConfigWithoutCreationCTA|AppStoreTests/testLocalizedSetupGuideShowsTargetValidationActions|AppStoreTests/testLocalizedProfileSelectionDisplayDoesNotChangeRawConfigValues|AppStoreTests/testSetupGuideClarifiesExistingConfigRootInputsAreNotLoadedRoots|UIPreferencesTests/testAppChromeLocalizationLoadsPrepareChromeResources'`
      (8 tests, 0 failures)
    - `swift test --package-path macos --filter 'AppStoreTests/testProfileDestinationPlanInitializesNewSourceProfileWhenSourceIsReadyAndTargetPathIsSet|AppStoreTests/testSetupGuideExplainsEmptySourcePreparationInUserOrder|AppStoreTests/testLocalizedSetupGuideExplainsEmptySourcePreparationWithoutChangingRawGuide|AppStoreTests/testLocalizedProfileSelectionDisplayDoesNotChangeRawConfigValues|AppStoreTests/testWorkbenchRoleLocalizedLabelsDoNotChangeRoleIdentity|AppStoreTests/testApplyProfileDestinationSelectionDoesNotAutoLaunchProfileCreation|AppStoreTests/testProfileInitDispatchCopyNamesTargetOwnedDestinationPath|UIPreferencesTests/testAppChromeLocalizationLoadsPrepareChromeResources'`
      (8 tests, 0 failures)
    - `swift build --package-path macos --product SuperMoverApp`: pass.
    - Python resource-key audit: English and Simplified Chinese
      `Localizable.strings` both have 142 keys, no duplicates, and no missing
      counterpart keys.
  - Boundary:
    This is Prepare-page UX simplification only. It keeps profile files as the
    CLI `--profile` SSOT, does not add runtime policy overrides, and does not
    close real two-Mac installed-app execution, Local Network/firewall prompt
    evidence, Developer ID notarization/staple/Gatekeeper proof, or final
    T-011 release closure.
- Step: T-011 Source-only profile creation correction.
  - Outcome: Source Prepare no longer collects, displays, gates on, or passes a
    Target-local save path. The CLI now has an explicit
    `profile init --source-only` mode that creates a valid source-side profile
    with `target.local_path` omitted; `profile lint` prints a pending
    `target.local_path` action for that state, and Target completes the same
    profile through `profile set-target` after choosing its own local save
    folder. The app profile-init command preview now uses `--source-only` and
    ignores stale `targetRootPath` state, while Target role remains the only
    setup surface that browses or validates a writable destination folder.
  - Green evidence:
    - `go test -count=1 ./internal/profile ./internal/cli -run 'Test(NewDefault|NewSourceOnly|ProfileInit|ProfileSetTarget)'`
      (pass)
    - `go test -count=1 ./internal/profile ./internal/cli`
      (pass)
    - `swift test --package-path macos --filter 'AppStoreTests/test(ProfileDestinationPlanInitializesNewSourceProfileWhenSourceIsReady|ProfileDestinationPlanStaysSelectionOnlyUntilRootsAreReady|SetupGuideExplainsEmptySourcePreparationInUserOrder|LocalizedSetupGuideExplainsEmptySourcePreparationWithoutChangingRawGuide|SetupGuideShowsNewConfigDestinationAsCreationStep|LocalizedProfileSelectionDisplayDoesNotChangeRawConfigValues|ApplyProfileDestinationSelectionDoesNotAutoLaunchProfileCreation|ProfileInitDispatchCopyIsSourceOnly|TaskRunGateAllowsProfileInitWithReadableSourceOnly|ProfileInitCommandPreviewUsesSourceOnlyAndIgnoresTargetPathInput|TaskRunGateBlocksProfileInitForExistingProfileFile)|UIPreferencesTests/testAppChromeLocalizationLoadsPrepareChromeResources'`
      (12 tests, 0 failures)
    - `swift test --package-path macos --filter 'AppStoreTests/test(SetupGuideExplainsEmptySourcePreparationInUserOrder|LocalizedSetupGuideExplainsEmptySourcePreparationWithoutChangingRawGuide|ProfileDestinationPlanInitializesNewSourceProfileWhenSourceIsReady|ProfileInitCommandPreviewUsesSourceOnlyAndIgnoresTargetPathInput|ProfileInitDispatchCopyIsSourceOnly)|UIPreferencesTests/testAppChromeLocalizationLoadsPrepareChromeResources'`
      (6 tests, 0 failures)
    - `swift test --package-path macos --filter 'AppStoreTests|UIPreferencesTests'`
      (130 tests, 0 failures)
    - `swift build --package-path macos --product SuperMoverApp`: pass.
    - `git diff --check`: pass.
    - `feature-tracker validate-tracker --root .`: pass.
    - Resource-key audit: English and Simplified Chinese
      `Localizable.strings` both have 142 keys and no duplicate keys.
    - CLI smoke: `profile init --source-only`, `profile lint`, `profile
      set-target`, and final `profile lint` all pass; lint reports pending
      `target.local_path` before `set-target` and no pending target path after
      `set-target`.
  - Boundary:
    This corrects Prepare/profile ownership only. It does not claim real
    two-Mac installed-app transfer success, pairing trust completion, Local
    Network/firewall prompt evidence, Developer ID notarization/staple, or
    final T-011 release closure.
- Step: T-011 app layout stability follow-up.
  - Outcome: Connect, Move, and Verify/Repair owner-page toolbars now live in
    the main workbench chrome outside the page body scroll view, while each
    selected detail surface remains scrollable below that fixed context. Sidebar
    navigation now scrolls independently, and the workstation footer uses a
    vertical fit fallback so constrained window heights keep core navigation and
    status visible before showing secondary safety details. A follow-up pass
    inspected the shared page host path for all top-level and merged workbench
    pages and split the main-content top inset from the bottom inset, so page
    headers and fixed owner toolbars sit closer to the window top without
    reducing bottom breathing room.
  - Green evidence:
    - `swift test --package-path macos --filter 'WorkbenchNavigationTests|WorkbenchChromeTests|AppStoreTests'`
      (136 tests, 0 failures)
  - Detail:
    Additional evidence is recorded in
    `artifacts/app-layout-stability-T-011.md`.
  - Boundary:
    This is app-shell layout stability only. It does not claim real two-Mac
    installed-app transfer success, Local Network/firewall prompt evidence,
    Developer ID notarization/staple/Gatekeeper proof, or final T-011 release
    closure.
- Step: T-011 recent issue root-cause sweep.
  - Outcome: `AGENTS.md` now requires root-cause analysis, sibling-surface
    search, in-scope fixes, validation evidence, and explicit closeout
    boundaries whenever a bug, confusing UX, inconsistent copy, unsafe state,
    or user-reported defect is found.
  - Outcome: the recent Prepare ownership, raw-looking button chrome, and
    workbench scroll/top-inset issues were rechecked under that rule. The
    Source/Target ownership and layout fixes remain aligned with the current
    code; no additional raw Browse/New button-style leak was found in active
    file-action surfaces.
  - Green evidence:
    - `swift test --package-path macos --filter 'AppStoreTests/test(ProfileDestinationPlanInitializesNewSourceProfileWhenSourceIsReady|ProfileInitCommandPreviewUsesSourceOnlyAndIgnoresTargetPathInput|TaskRunGateAllowsProfileInitWithReadableSourceOnly|SetupGuideExplainsEmptySourcePreparationInUserOrder|LocalizedSetupGuideExplainsEmptySourcePreparationWithoutChangingRawGuide)|WorkbenchChromeTests|WorkbenchNavigationTests'`
      (21 tests, 0 failures)
    - `go test -count=1 ./internal/profile ./internal/cli -run 'Test(NewDefault|NewSourceOnly|ProfileInit|ProfileSetTarget)'`
      (pass)
    - `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" validate-tracker --root .`
      (pass)
    - `git diff --check`: pass.
  - Detail:
    Additional sweep notes are recorded in
    `artifacts/recent-issue-root-cause-sweep-T-011.md`.
  - Boundary:
    This is a process and current-surface hygiene pass. It does not claim real
    two-Mac installed-app transfer success, Local Network/firewall prompt
    evidence, signed/notarized distribution readiness, Merkle/current-source
    proof, or final T-011 release closure.
- Step: T-011 app chrome, DRY, and localization closeout.
  - Outcome: shared page chrome and owner-mode policy now have explicit
    repository rules; owner-mode strips use a single implementation; sidebar,
    page headings, task display strings, major page sections, action labels,
    auxiliary panels, and empty states route through `AppChromeLocalization`
    or localized section helpers.
  - Outcome: `ProfileNetworkPanel`, `PairingReceiptPanel`,
    `AcceptanceBundlePanel`, and `AcceptanceOperatorEvidencePanel` now receive
    injected localization instead of hard-coding visible English chrome. Raw
    evidence, CLI output, paths, JSON/protocol fields, profile IDs, receipt IDs,
    and operator-entered text intentionally remain literal.
  - Green evidence:
    - `swift test --package-path macos --filter 'UIPreferencesTests|WorkbenchNavigationTests|WorkbenchChromeTests'`
      (31 tests, 0 failures)
    - `swift build --package-path macos --product SuperMoverApp`: pass.
    - `plutil -lint macos/SuperMoverApp/Resources/en.lproj/Localizable.strings macos/SuperMoverApp/Resources/zh-Hans.lproj/Localizable.strings`
      (pass)
    - Missing `AppChromeLocalization.text("...")` key scan: pass.
    - Duplicate localized resource key scans for English and Simplified
      Chinese: pass.
    - `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" validate-tracker --root .`
      (pass)
    - `git diff --check`: pass.
  - Detail:
    Additional closeout notes are recorded in
    `artifacts/app-chrome-i18n-dry-T-011.md`.
  - Boundary:
    This closes the shared chrome/i18n/DRY class of defects. It does not claim
    translated raw audit evidence, full visual QA across every window size,
    real two-Mac installed-app transfer success, Local Network/firewall prompt
    evidence, signed/notarized distribution readiness, Merkle/current-source
    proof, or final T-011 release closure.
- Step: T-011 shared detail-page header pinning correction.
  - Outcome: `DetailPageHost` now uses SwiftUI native pinned section headers
    instead of measuring scroll position and applying an offset. This fixes the
    Prepare page title disappearing during vertical scroll and applies through
    the shared detail host rather than a page-specific workaround.
  - Outcome: the obsolete `detailPageStickyHeaderOffset` helper,
    `DetailPageHeaderMinYPreferenceKey`, and `GeometryReader` plus offset shim
    were removed so future code cannot keep validating the failed approach.
  - Green evidence:
    - `swift test --package-path macos --filter 'WorkbenchChromeTests|WorkbenchNavigationTests|UIPreferencesTests'`
      (31 tests, 0 failures)
    - `git diff --check`: pass.
  - Detail:
    Updated notes are recorded in
    `artifacts/app-layout-stability-T-011.md` and
    `artifacts/recent-issue-root-cause-sweep-T-011.md`.
  - Boundary:
    This fixes shared macOS detail-page header pinning. It does not claim a
    full visual QA matrix across every window size, real two-Mac installed-app
    transfer success, Local Network/firewall prompt evidence,
    signed/notarized distribution readiness, Merkle/current-source proof, or
    final T-011 release closure.

## Residual Risks

- `f-237nwzbyq` is proposal-only. Do not use it as proof of implemented broad
  repair scan behavior until `reconcile scan` is actually wired and validated.
- This feature depends on accurate CLI/app truth. Any future UI slice must be
  reviewed against current command help, code paths, and validation evidence.
- Merkle/root comparison remains unavailable. Unless a real CLI/control-plane
  root artifact is implemented and tested, the app must continue to show Merkle
  proof as unavailable.
- The app-first LAN workflow remains incomplete. T-001 through T-010 have local
  implementation evidence and T-011 has partial packaged loopback evidence, but
  final two-machine acceptance evidence should not be claimed complete until a
  real two-device run, signing/notarization evidence, and permission evidence
  are recorded.
  Evidence Vault actions are intentionally limited to review metadata/control
  artifacts; target mutation, transfer, trust/pairing, publish, Merkle proof,
  and current-source proof remain excluded or unavailable.
- The specific daemon event temp-file boundary TOCTOU was fixed in `1c5c4cf`
  with deterministic tests and targeted race tests. Do not generalize that to
  all daemon or incremental sync release hardening: durable queue lost-update
  risks and final app foreground-daemon smoke evidence remain follow-up work.
