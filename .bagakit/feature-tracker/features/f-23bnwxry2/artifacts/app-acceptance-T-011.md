# T-011 App-First Acceptance Evidence

## Scope

This artifact records the final acceptance strategy and current local evidence
for the native macOS app-first LAN migration workbench.

Current implemented evidence:

- `macos/script/acceptance-loopback.sh` runs through the packaged app's bundled
  CLI at `macos/dist/SuperMover.app/Contents/Resources/bin/supermover`.
- The loopback script covers packaged provenance consistency with the bundled
  CLI version and expected bundle path, profile init/lint, local push, verify,
  status, report, health, a dot-prefixed file, dot-directory payload, zero-byte
  file, a larger regular file, sync queue enqueue/list/ready, bounded local
  `sync run`, unpaired network dry-run fail-closed behavior without target
  data or control-plane mutation, and a same-machine packaged pairing plus
  non-dry-run localhost mTLS transfer that leaves receiver-side
  `network-transfer.json` evidence.
- `docs/runbook.md` now contains an app-first acceptance matrix that separates
  loopback packaged-app evidence from two-machine LAN, permission, signing, and
  final release evidence.
- `macos/script/audit-app.sh` records local release-engineering evidence for
  `Info.plist`, provenance, current git state, bundled CLI path/version, basic
  hashes, app/CLI codesign verification, hardened runtime, entitlements,
  Gatekeeper assessment, and stapler validation.
- `macos/script/build-app.sh` signs inside-out when
  `SUPERMOVER_CODESIGN_IDENTITY` is set: bundled CLI first, then the app bundle;
  signing-time `--deep` is not used.
- `macos/script/acceptance-two-machine.sh` now exposes a five-phase harness:
  `target-serve`, `source-pair`, `target-import`, `source-transfer`, and
  `evaluate`. The same packaged app and bundled CLI path has been exercised
  locally as a same-machine simulation of that five-phase trust and transfer
  shape. The shared evidence bundle now also preserves per-machine
  `source.app-audit.json` and `target.app-audit.json` alongside CLI provenance
  and transfer artifacts.
- The shell distinct-machine installed-app proof contract is now less split and
  more fail-closed: `workflow-status` and
  `evaluate --require-operator-evidence` both consume the same installed-app
  proof verdict surface rather than independently interpreting weaker handoff
  booleans. That verdict now requires collection mode, role machine ids,
  `evidence.machine_facts.*`, machine-facts artifact files, and verified
  cross-machine handoff evidence to agree before the bundle can advance to
  final installed-app evaluation.
- The shell archive handoff restore path now stages `unpack-bundle` before
  publishing to the requested incoming bundle root. A digest-valid archive that
  is missing the internal export identity artifact now fails closed without
  deleting an existing bundle root, and valid restores still record the
  verified archive handoff in `meta.json` before the staged root is published.
- That shell-side verdict now also matches the app-side contradictory-handoff
  edge. A merged bundle is no longer allowed to advance just because it
  contains one verified cross-machine handoff for the recorded source/target
  pair; if the same bundle also carries any additional verified cross-machine
  handoff for a different machine pair, shell `workflow-status` and
  `evaluate --require-operator-evidence` now fail closed with the same
  `contradictory_verified_bundle_handoffs` outcome the Swift-side proof owner
  already used.
- The app-side `AcceptanceBundleLoadedSnapshot.workflowSummary(...)` fallback is
  now less optimistic when the shell-authored workflow summary artifact is
  absent: it requires valid `supermover.acceptance.machine_facts.v1` artifacts
  before treating machine-pair handoffs as installed-app proof, and it matches
  the shell by reserving `bundle_handoff` as a next action for the
  `require-operator-evidence` lane instead of demanding it in the default lane.
- The app-side workflow-summary reader is now also less willing to trust stale
  shell-authored `workflow.summary.json` artifacts: in the two-machine lane it
  only accepts either the default or `require-operator-evidence` summary when
  the current bundle still agrees on installed-app proof fields, release-ready
  packaging evidence, `bundle_status`, and the derived `steps` /
  `next_actions` surface. A cached two-machine summary can no longer keep
  showing `evaluate` after the merged bundle has drifted back to an earlier or
  blocked state.
- The app-side installed-app launch advisory and preflight surfaces are now
  tighter and share more of the same proof owner. `AcceptanceInstalledAppLaunchGate`
  owns the two-machine packaged-app/audit/notarization verdict, `AppStore`
  preview and preflight both consume that gate, and local notarization preview
  inspection now lives under `AcceptanceBundleAppOperations` plus
  `AcceptancePackagingEvidenceCollector` rather than leaving bundle-layout
  details in `AppStore`. Preview now fails closed to `blocked` when the local
  packaged app's notarization sidecar is missing, malformed, schema-valid but
  still not release-ready, or no longer matches the current packaged app's
  post-staple audit/provenance, instead of showing a stale optimistic `pass`
  from the loaded bundle alone. Swift final evaluation now reuses the same
  release-ready notarization helper, reducing one more app-side policy fork.
- App-side installed-app launch preview/preflight now also consume the shared
  Swift-side distinct-machine collection proof summary instead of treating
  release-ready packaging evidence as sufficient. The launch surface now splits
  three states more honestly: incomplete machine-facts / handoff proof stays
  `review`, contradictory machine-facts / verified-handoff proof blocks phase
  launch, and only a current bundle whose distinct-machine proof matches the
  shared proof owner can render `pass`.
- That installed-app proof split is now narrower and less misleading around
  verified `bundle_handoffs`. A bundle that only contains a verified handoff
  for the wrong machine pair is treated as missing pair proof and continues to
  request `bundle_handoff`; only bundles that contain both a matched verified
  handoff and additional verified handoffs for other machine pairs stay in the
  contradictory blocked bucket. This keeps app-side workflow advice aligned
  with the stricter shell verdict without flattening both failure shapes into
  one review-only state.
- App-side launch preflight now also stops earlier and writes less. When the
  installed-app proof is already blocked by an unrelated or non-correctable
  condition, preflight returns that blocker before attempting to author any
  packaging evidence; but when the shared proof owner says the next honest step
  is machine-identity correction, app-side `source pair` and `target serve`
  remain launchable so they can rewrite the matching role/machine-facts
  evidence the workflow summary already requests. Local packaging output leaves
  that are unsafe still fail closed before any bundle mutation starts.
- That corrective app-side authoring path now really rewrites the machine
  identity proof it advertises: source `pair`, target `serve`, source
  `transfer`, target `import`, source `browse`, and target `advertise` all
  refresh canonical `source.machine.json` / `target.machine.json` and update
  the matching `meta.json` `roles.*` plus `evidence.machine_facts.*` records
  from the current installed app's machine id/label. App-side machine-identity
  correction can now advance a bundle out of the stale-machine-id blocker and
  down to the remaining verified `bundle_handoffs`, operator-evidence, and
  release-evidence gates instead of staying as a preview-only promise.
- The app-side notarization source contract is now tighter and matches the
  shell/docs release workflow: installed-app acceptance only consumes sibling
  `<AppName>.app.notary/notarization.json` evidence. Swift no longer treats a
  bundled `Contents/Resources/release/notary/notarization.json` artifact as
  installed-app proof, and it now rejects sibling sidecars whose referenced
  post-staple audit/provenance no longer matches the current packaged app, and
  it rejects symlinked canonical sidecar / post-staple audit leaves as unsafe
  local evidence, so preview and preflight now fail closed when only an
  unproven fallback, a stale sibling sidecar, or laundered external proof is
  present.
- The shell-side release-evidence contract now matches that same currentness
  rule. `record-packaging-evidence` refuses sibling sidecars whose referenced
  post-staple audit no longer matches the current packaged app and bundled
  provenance, and shell `workflow-status` plus
  `evaluate --require-operator-evidence` now require bundle-local
  `*.app-audit.json`, `*.provenance.json`, and `*.notarization.json` to agree
  before installed-app release evidence can advance to final evaluation. The
  local release-engineering gate now also shares part of that contract:
  `macos/script/audit-app.sh` blocks when an existing canonical sibling
  sidecar is stale for the current app, and `macos/script/notarize-app.sh`
  clears any previous sibling result before its post-staple audit so stale
  sidecars do not block a fresh rerun. It also persists the referenced
  post-staple audit into the sibling sidecar directory, so later currentness
  checks do not depend on a custom temporary `--work-dir`.
- The bundled packaged-app audit helper now matches that same bundle-local
  sidecar rule on the shared subset: a missing sibling sidecar is no longer an
  automatic helper failure, but an existing stale sibling sidecar still blocks
  currentness/release-ready checks, including after a successful notarize run
  whose temporary `--work-dir` was later removed. Decodable-but-invalid sidecar
  documents and wrong-schema post-staple audits also now fail closed for helper
  `current` / `releaseReady` instead of only surfacing as blocked sub-checks.
  Canonical sidecars are now anchored to the sibling
  `<AppName>.app.notary/post-staple.audit.json` path rather than accepting
  arbitrary external audit files, reject symlinked canonical sidecar /
  post-staple audit leaves, and the shell-side currentness check now
  canonicalizes provenance JSON before comparison so semantically identical
  manifests do not fail stale on key-order drift. This reduces one concrete
  helper-vs-shell drift in installed-app packaging collection without yet
  resolving the broader repo/git contract mismatch.
- The native app now uses that same bundle-local currentness contract for
  installed-app launch preview, launch preflight, workflow summary, and Swift
  final evaluate. Copied notarization artifacts that no longer match
  `source|target.app-audit.json` and `source|target.provenance.json` now stay
  review/blocked in-app instead of looking ready on status/readiness bits
  alone, and Swift release-evidence loading now reads the canonical
  `source|target.{provenance,app-audit,notarization}.json` files directly for
  this lane. App-side packaging collection now also rejects sibling sidecars
  whose top-level `app_path` no longer matches the current packaged app, and
  whose `audit.path` escapes the canonical sibling
  `.app.notary/post-staple.audit.json`. The copied-built-app opt-in fixture now
  writes and asserts the sibling `post-staple.audit.json` evidence it
  references instead of depending on a status-only sidecar stub.
- Swift final evaluate now also shares one guarded bundle-artifact access seam
  with the bundle reader instead of rebuilding bundle paths ad hoc. Bundle-local
  `..` path escapes, symlinked provenance/audit/machine-facts artifacts, and
  malformed `source.consistency.json` proof artifacts now fail closed at final
  evaluation rather than being accepted by direct `Data(contentsOf:)` rereads
  or by merged source-consistency meta fallback.
- That same bundle-artifact access owner now also guards acceptance bundle root
  and `meta.json` trust boundaries more tightly. `AcceptanceBundleReader`
  rejects symlinked bundle roots and symlinked `meta.json`, and app-side
  artifact / packaging writers now validate both the bundle root and
  `meta.json` before any file writes, so a symlinked bundle path can no longer
  leave partial
  `source.version.txt`, `source.provenance.json`, `source.app-audit.json`, or
  other app-authored artifacts behind before later metadata mutation fails.
- App-side packaging evidence publication is now also less prone to
  half-published local truth. `AcceptancePackagingEvidenceCollector` stages the
  current app's version/provenance/audit/notarization outputs outside the
  acceptance bundle first and only publishes them into the bundle after the
  full collection succeeds. A malformed or stale local notarization sidecar now
  clears stale copied notarization state without leaving fresh
  `source|target.version.txt`, `source|target.provenance.json`, or
  `source|target.app-audit.json` artifacts behind from the failed attempt.
- App-side installed-app launch preview is now less optimistic about whether
  the current packaged app can actually author fresh packaging evidence.
  `AcceptanceBundleAppOperations` now drives preview/preflight through a shared
  non-mutating current-app packaging feasibility check that exercises bundled
  provenance loading, bundled CLI version probing, local audit helper
  execution, output path safety, and sibling notarization currentness before
  phase launch is offered. Missing or malformed bundled provenance, unavailable
  local audit helpers, unsafe output leaves, and stale/malformed sibling
  notarization sidecars now block preview at the same seam preflight uses, and
  missing or not-release-ready local notarization evidence now also blocks the
  launch advisory when shell phase preflight would fail closed on that same
  current packaged-app condition.
- That current-app packaging seam is now also less split internally. Preview
  and preflight no longer classify fresh local packaging feasibility through
  separate audit/notarization paths; they now share one typed current-app
  packaging probe that inspects the fresh local audit result together with the
  sibling `.app.notary/notarization.json` sidecar. A launch surface can no
  longer stay `pass`/`review` when the current app's fresh audit would make the
  local notarization sidecar stale before preflight writes anything into the
  bundle. Missing, not-release-ready, malformed, symlinked, escaped, or
  fresh-audit-mismatched local notarization sidecars now block preview at the
  same owner preflight relies on, so the advisory surface no longer implies a
  phase launch can proceed when shell collection would exit before execution.
- The app-side installed-app launch gate now also treats a configured but
  unreadable acceptance bundle as a hard blocker instead of silently downgrading
  into “not in two-machine mode.” A non-empty bundle path whose `meta.json` is
  missing or malformed now yields a blocked preview, a blocked preflight, and
  no launched task until the operator clears or replaces that bundle path.
- The native macOS app can now load that same acceptance bundle and write
  durable manual operator evidence for `local_network`, `firewall`, and
  `pairing_confirmation` into `meta.json` under the same `.meta.lock` protocol
  used by the shell harness, instead of leaving real-device prompt/code
  evidence as shell-only authoring.
- The native macOS app can also write current app-side phase artifacts into the
  same bundle for discovery browse/advertise, serve readiness phases, source
  pair, target import, source transfer, per-machine packaging audit, and final
  evaluation. The real built-app integration deletes harness-written
  packaging, `target_import`, and `evaluation` artifacts from a same-machine
  bundle and rewrites them through `AppStore` surfaces.
- The Settings language switch now localizes bounded app-owned Prepare/setup
  chrome in addition to sidebar, Settings, Display Preferences, and preference
  options, including visible role badge labels, role metadata prefixes, and
  profile-selection guidance. Raw role identity, task titles, command
  previews, profile/config paths, evidence/proof values, artifact fields, and
  CLI output remain raw and audit-stable; this is UI clarity only, not release
  acceptance evidence.
- `f-237nwzbyq` tracker state was corrected back to proposal-only because the
  current CLI exposes `reconcile plan`, `reconcile review`, and
  `reconcile apply`, but not `reconcile scan`.

Still not complete:

- Signed/notarized distribution evidence.
- Two-machine app-installed LAN acceptance with Local Network/firewall prompt
  evidence.
- Two-machine non-dry-run mTLS network transfer evidence driven from installed
  apps.
- A real source-machine bundle handoff, pack/unpack/merge, and final
  `evaluate --require-operator-evidence` run across two distinct Macs using
  release-quality packaged apps.
- Merkle/current-source proof, which remains unavailable and must not be shown
  as pass.
- The specific daemon event temp-file TOCTOU was fixed in `1c5c4cf` with
  targeted tests. Broader foreground daemon and incremental sync queue
  concurrency should still be treated as release-hardening work until final
  release gates cover them.

## Validation

- `SUPERMOVER_ACCEPTANCE_SKIP_BUILD=1 macos/script/acceptance-loopback.sh`
  passed and printed a disposable evidence directory. The script now also
  prints and writes the evidence path on failure. Its summary records
  `provenance_readiness=local_review_only`, `app_audit_status=blocked`,
  `app_audit_exit_code=1`, and `app_audit_blocking_checks=11` for the local
  unsigned app, while also recording `network_loopback.status=pass`,
  `network_loopback.transfer=tls13_mtls`, `paired_target_profile=true`, and a
  receiver-side `network-transfer.json` artifact for the same-machine packaged
  mTLS run rather than treating that as two-machine release evidence.
- Bundled CLI surface negative check: with an out-of-date packaged app whose
  bundled CLI did not yet expose `serve --ready-file`,
  `SUPERMOVER_ACCEPTANCE_SKIP_BUILD=1 macos/script/acceptance-loopback.sh`
  failed closed with shell exit code `5` and an explicit rebuild-required
  diagnostic instead of timing out later in serve readiness.
- `SUPERMOVER_CODESIGN_IDENTITY= macos/script/build-app.sh` passed and produced
  an unsigned local app.
- `macos/script/audit-app.sh macos/dist/SuperMover.app` against the unsigned app
  exited `1` with `status=blocked`, `readiness=blocked`, and `blocking_checks=11`.
- `SUPERMOVER_CODESIGN_IDENTITY=- macos/script/build-app.sh` passed and produced
  an ad-hoc signed app with explicitly signed bundled CLI and app bundle.
- `macos/script/audit-app.sh macos/dist/SuperMover.app` against the ad-hoc app
  exited `1` with `status=blocked`, `readiness=blocked`, and `blocking_checks=6`;
  app and CLI hardened runtime plus entitlements were present, while ad-hoc
  identity, dirty worktree, Gatekeeper assessment, and stapler validation blocked
  distribution readiness.
- Stale skipped-build provenance negative check: after adding the current-HEAD
  provenance gate and before rebuilding `macos/dist/SuperMover.app`,
  `SUPERMOVER_ACCEPTANCE_SKIP_BUILD=1 macos/script/acceptance-loopback.sh`
  failed with shell exit code `5`, `jq` reported `incomplete packaged
  provenance`, and the script printed a preserved failure evidence directory.
  Rebuilding with `macos/script/build-app.sh` refreshed
  `supermover-provenance.json` to `git_commit=6d4972ce9900`, after which the
  same skipped-build acceptance passed.
- `go run ./cmd/supermover reconcile scan --help` failed with
  shell exit code `1` and output containing `reconcile: unknown subcommand
  "scan"`, confirming the f-237 tracker must remain proposal-only until that CLI
  is actually implemented.
- Local same-machine packaged five-phase simulation passed after rebuilding the
  bundled app with the updated CLI surface. The simulated flow covered:
  `target-serve` pairing readiness, `source-pair` with `--receipt-out`,
  `target-import` via `profile adopt-pairing --receipt-file`, paired
  `target-serve` receiver readiness, `source-transfer` with
  `profile set-network` followed by dry-run and non-dry-run `push --network`,
  and `evaluate`. This is local wiring evidence only and is deliberately kept
  separate from real two-machine installed-app acceptance.
- Fresh closeout checks on 2026-06-01 passed:
  - `go test -count=1 ./...`
  - `go test -count=1 ./internal/cli ./internal/pairing ./internal/pairserve ./internal/incrementalsync ./internal/networkpush ./internal/profile ./internal/report ./internal/status`
  - `sh -n macos/script/acceptance-loopback.sh macos/script/acceptance-two-machine.sh macos/script/acceptance-two-machine-same-machine.sh macos/script/audit-app.sh macos/script/build-app.sh`
  - `swift test --package-path macos --filter 'AcceptanceBundleArtifactWriterTests|AcceptanceBundleAppOperationsIntegrationTests|AcceptanceEvaluationIntegrationTests|AcceptanceEvaluationTests|AcceptanceBundleTests'`
  - `swift test --package-path macos`
  - `sh macos/script/build-app.sh`
  - `SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION=1 SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 swift test --package-path macos --filter 'AcceptanceBundleAppOperationsIntegrationTests/testAppStoreRecordsPackagingAndEvaluationAgainstSameMachineBundleWhenEnabled|AcceptanceEvaluationIntegrationTests/testEvaluationCoordinatorRewritesSameMachineHarnessBundleWhenEnabled|AcceptancePackagingEvidenceTests/testCollectorDefaultRunnersAgainstBuiltAppWhenEnabled'`
- Distinct-machine proof-summary closeout checks on 2026-06-02 passed:
  - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsRoleMatchedHandoffWhenMetaMachineFactsDisagree|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsWrongMachinePairHandoff|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMachineFactArtifactMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotTreatMetaMatchedHandoffAsProofWhenMachineFactArtifactsDisagree|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotTreatRoleMatchedHandoffAsProofWhenMachineFactsDisagree'`
  - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsProveTheWrongMachinePair|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsDoNotProveCrossMachineTransfer|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts'`
  - `sh -n macos/script/lib/acceptance-two-machine.sh macos/script/acceptance-two-machine.sh`
  - `git diff --check -- macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift`
- App-side fallback proof-parity checks on 2026-06-03 passed:
  - `swift test --package-path macos --filter 'AcceptanceBundleTests|AcceptanceEvaluationTests|AcceptanceTwoMachineScriptTests'`
  - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift macos/SuperMoverAppTests/AcceptanceBundleTests.swift`
- App-side launch-gate proof-owner checks on 2026-06-03 passed:
  - `swift build --package-path macos`
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchGateTests|AcceptancePackagingEvidenceTests|AppStoreTests|AcceptanceBundleAppOperationsIntegrationTests'`
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchGateTests|AcceptancePackagingEvidenceTests|AcceptanceEvaluationTests|AppStoreTests'`
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppNotarizationSourceTests'`
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppLaunchGateTests|AcceptancePackagingEvidenceTests|AcceptanceEvaluationTests|AppStoreTests'`
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppBundleContextTests|AcceptanceInstalledAppLaunchGateTests'`
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppBundleContextTests|AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppLaunchGateTests|AcceptancePackagingEvidenceTests|AcceptanceEvaluationTests|AppStoreTests'`
  - `git diff --check -- macos/SuperMoverApp/AcceptanceInstalledAppLaunchBundleContext.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchGate.swift macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverApp/AcceptanceBundleAppOperations.swift macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift macos/SuperMoverApp/AppStore.swift macos/SuperMoverAppTests/AcceptanceInstalledAppBundleContextTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppLaunchGateTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppNotarizationSourceTests.swift macos/SuperMoverAppTests/AppStoreTests.swift`
- App-side installed-app proof-parity tightening on 2026-06-03 passed:
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchGateTests|AcceptanceInstalledAppWorkflowSummaryTests|AppStoreTests/testAcceptanceTwoMachineLaunchPreflightBlocksContradictoryInstalledAppProof|AppStoreTests/testAcceptanceTwoMachineLaunchPreflightAllowsDistributionReadyBundledAcceptanceTask|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewReviewsReadyAuditWhenDistinctMachineProofIsIncomplete|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewReviewsDistributionReadyAuditWhenDistinctMachineProofIsIncomplete|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewPassesWhenDistinctMachineProofIsReady|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewDoesNotTrustStaleMetaWhenSourceAppAuditArtifactIsMissing'`
  - `swift test --package-path macos --filter 'AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsProveTheWrongMachinePair|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsDoNotProveCrossMachineTransfer|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts'`
  - `git diff --check -- macos/SuperMoverApp/AcceptanceInstalledAppCollectionProof.swift macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift macos/SuperMoverApp/AcceptanceBundleSnapshot.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchGate.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchBundleContext.swift macos/SuperMoverApp/AppStore.swift macos/SuperMoverAppTests/AcceptanceInstalledAppWorkflowSummaryTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppLaunchGateTests.swift macos/SuperMoverAppTests/AppStoreTests.swift`
- Default two-machine workflow-summary stale-artifact guard on 2026-06-03
  passed:
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryStartsWithTargetServe|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryRequiresHandoffBeforeEvaluate'`
  - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift macos/SuperMoverAppTests/AcceptanceInstalledAppWorkflowSummaryTests.swift`
- Final-evaluate bundle path-safety tightening on 2026-06-03 passed:
  - `swift test --package-path macos --filter 'AcceptanceEvaluationPathSafetyTests'`
  - `swift test --package-path macos --filter 'AcceptanceBundleTests|AcceptanceInstalledAppLaunchGateTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceEvaluationTests|AcceptanceInstalledAppBundleContextTests|AcceptanceEvaluationPathSafetyTests|AppStoreTests/testAcceptanceTwoMachineLaunch'`
  - `swift test --package-path macos --filter 'AcceptanceEvaluationIntegrationTests'`
  - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleArtifactAccess.swift macos/SuperMoverApp/AcceptanceBundleReader.swift macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift macos/SuperMoverAppTests/AcceptanceEvaluationPathSafetyTests.swift`
- Acceptance-bundle root/meta trust-boundary tightening on 2026-06-03 passed:
  - `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests'`
  - `swift test --package-path macos --filter 'AcceptanceBundleTests|AcceptanceInstalledAppBundleContextTests|AcceptanceBundleArtifactWriterTests|AcceptancePackagingEvidenceTests|AcceptanceEvaluationPathSafetyTests|AcceptanceEvaluationTests|AppStoreTests/testAcceptanceTwoMachineLaunch'`
  - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleArtifactAccess.swift macos/SuperMoverApp/AcceptanceBundleReader.swift macos/SuperMoverApp/AcceptanceBundleArtifactWriter.swift macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverAppTests/AcceptanceBundleRootTrustTests.swift`
  - `sh macos/script/build-app.sh`
- Local notarization sidecar currentness guard on 2026-06-03 passed:
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppNotarizationSourceTests|AppStoreTests/testAcceptanceTwoMachineLaunchPreflightAcceptsNotarizeScriptSidecarWorkflow|AppStoreTests/testAcceptanceTwoMachineLaunchPreflightAllowsReadyBundledAcceptanceTask|AppStoreTests/testAcceptanceTwoMachineLaunchPreflightAllowsDistributionReadyBundledAcceptanceTask|AppStoreTests/testAcceptanceTwoMachineLaunchPreviewReviewsLocalNotarizationThatIsNotReleaseReady|NotarizeAppScriptTests/testNotarizeScriptProducesDurableEvidenceOnSuccessfulFakeNotaryFlow|NotarizeAppScriptTests/testNotarizeScriptPersistsStructuredSidecarNextToAppOnSuccessfulFakeNotaryFlow'`
  - `git diff --check -- macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverApp/AcceptanceBundleAppOperations.swift macos/SuperMoverAppTests/AcceptanceInstalledAppNotarizationSourceTests.swift macos/SuperMoverAppTests/AppStoreTests.swift macos/SuperMoverAppTests/NotarizeAppScriptTests.swift`
- Helper/collector sidecar fail-closed tightening on 2026-06-03 passed:
  - `swift test --package-path macos --filter 'PackagedAppAuditorTests/testAuditorBlocksMalformedCanonicalSidecarEvenWhenAuditStillMatchesCurrentApp'`
  - `swift test --package-path macos --filter 'PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenPostStapleAuditSchemaIsInvalid|PackagedAppAuditorTests/testAuditorBlocksCanonicalSidecarWhenPostStapleAuditEscapesSiblingSidecarDirectory|AcceptanceInstalledAppNotarizationSourceTests/testCollectorRejectsSiblingNotarizationWhenAuditPathEscapesSiblingSidecarDirectory|AcceptanceInstalledAppNotarizationSourceTests/testLaunchPreviewTreatsSiblingNotarizationWithExternalAuditPathAsCurrentnessReview|AcceptanceInstalledAppNotarizationSourceTests/testLaunchPreflightFailsClosedWhenSiblingNotarizationAuditPathEscapesSiblingSidecarDirectory|AppAuditTamperTests/testAppAuditBlocksCanonicalNotarizationSidecarWhoseAuditPathEscapesSiblingDirectory'`
  - `SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorUsesSiblingNotarizationSidecarForCopiedBuiltAppWhenEnabled'`
  - `swift test --package-path macos --filter 'PackagedAppAuditorTests|AcceptancePackagingEvidenceTests|AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppReleaseEvidenceScriptTests|AppAuditTamperTests|NotarizeAppScriptTests'`
  - `sh -n macos/script/lib/acceptance-common.sh macos/script/audit-app.sh macos/script/notarize-app.sh`
  - `git diff --check -- macos/script/lib/acceptance-common.sh macos/SuperMoverAppSupport/PackagedAppAuditor.swift macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverAppTests/PackagedAppAuditorTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppNotarizationSourceTests.swift macos/SuperMoverAppTests/AcceptancePackagingEvidenceTests.swift macos/SuperMoverAppTests/AppAuditTamperTests.swift macos/SuperMoverAppTests/AcceptanceInstalledAppReleaseEvidenceScriptTests.swift`
- Shared current-app packaging probe parity on 2026-06-03 passed:
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppNotarizationSourceTests|AppStoreTests/testAcceptanceTwoMachineLaunchPreview|AppStoreTests/testAcceptanceTwoMachineLaunchPreflight'`
  - `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests|AcceptanceInstalledAppLaunchCoordinatorTests|AcceptanceInstalledAppNotarizationSourceTests|AcceptanceInstalledAppCollectionProofParityScriptTests|AcceptanceInstalledAppLaunchGateTests|AcceptanceBundleTests|AcceptanceInstalledAppWorkflowSummaryTests|AcceptanceBundleAppOperationsIntegrationTests|AppStoreTests/testAcceptanceTwoMachineLaunch'`
  - `git diff --check -- macos/SuperMoverApp/AcceptanceBundleAppOperations.swift macos/SuperMoverApp/AcceptanceInstalledAppLaunchCoordinator.swift macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift macos/SuperMoverAppTests/AppStoreTests.swift`
- Shell-side notarization currentness parity on 2026-06-03 passed:
  - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests|AcceptanceInstalledAppReleaseEvidenceScriptTests|AcceptanceInstalledAppNotarizationSourceScriptTests|AcceptanceInstalledAppReleaseEvidenceEvaluationScriptTests|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsProveTheWrongMachinePair|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsDoNotProveCrossMachineTransfer|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts'`
  - `sh -n macos/script/lib/acceptance-common.sh macos/script/lib/acceptance-two-machine.sh macos/script/acceptance-two-machine.sh`
- Archive handoff restore staging on 2026-06-03 passed:
  - `sh -n macos/script/lib/acceptance-two-machine.sh macos/script/acceptance-two-machine-same-machine.sh macos/script/acceptance-two-machine.sh`
  - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/(testTwoMachineUnpackBundlePreservesExistingBundleWhenArchiveMissingExportIdentity|testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityIsTampered|testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityFieldsAreMissing|testTwoMachineUnpackBundleRecordsVerifiedArchiveHandoffEvidence|testTwoMachinePackAndUnpackBundleRoundTripsEvidence)'`
  - `SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 swift test --package-path macos --filter 'AcceptanceEvaluationIntegrationTests/testSameMachineHarnessSupportsArchiveHandoffWhenEnabled'`
  - `git diff --check -- macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift`
- Reviewer-driven bundle-artifact hardlink, receipt-schema, and Task Dispatch
  gate follow-up on 2026-06-04 passed:
  - `sh -n macos/script/lib/acceptance-two-machine.sh`
  - `sh -n macos/script/acceptance-two-machine.sh`
  - `swift test --package-path macos --filter 'AppStoreTests/testTaskRunGateBlocksProfileInitForExistingProfileFile|AppStoreTests/testTaskRunGateBlocksExistingProfileTasksForNewProfileDestination|AppStoreTests/testTaskRunGateBlocksPublishWithoutSessionID|AppStoreTests/testTaskRunGateBlocksReconcileApplyWithoutReason|WorkbenchNavigationTests|WorkbenchChromeTests/testDetailPageStickyHeaderStopsAtPaneTopWithoutOvershoot'`
    - 9 tests, 0 failures.
  - `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testOperatorEvidenceStoreRejectsHardlinkedMetaBeforeWriting|AcceptanceBundleRootTrustTests/testAcceptanceBundleReaderRejectsHardlinkedMetaFile|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMalformed|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsDirectory|AcceptanceEvaluationModeTests|AcceptanceInstalledAppLaunchGateTests'`
    - 25 tests, 0 failures.
  - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsBundleLocalArtifactHardlinks|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsTargetControlPlaneHardlinks'`
    - 2 tests, 0 failures.
  - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsLinkedBundleMeta|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsLinkedBundleMeta|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotUseHardlinkedMachineFactArtifactsAsProof|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotUseHardlinkedReleaseArtifactsAsReady|AcceptanceBundleRootTrustTests/testAcceptanceBundleReaderRejectsSpecialMetaFile|AcceptanceBundleRootTrustTests/testOperatorEvidenceStoreRejectsSpecialMetaBeforeWriting|AcceptanceBundleRootTrustTests/testAcceptanceBundleReaderRejectsHardlinkedMetaFile|AcceptanceBundleRootTrustTests/testOperatorEvidenceStoreRejectsHardlinkedMetaBeforeWriting|AcceptanceBundleTests/testAcceptanceBundleReaderLoadsEvidenceCollectedBundle|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMalformed'`
    - 10 tests, 0 failures.
  - Coverage notes:
    - shell final `evaluate` now requires required bundle-local artifacts to
      be single-link regular files before reading them;
    - shell bundle bootstrap rejects linked or non-regular `meta.json` before
      treating it as durable bundle truth;
    - shell `workflow-status` summary helpers reject hardlinked machine-facts
      and release-evidence artifacts before treating them as ready;
    - Swift advisory validates exported pairing receipt schema/content before
      marking `source_pair` / `target_import` ready;
    - `meta.json` is covered by the same single-link regular-file bundle
      access policy for reads and direct operator-evidence writes;
    - Task Dispatch run CTA and `runSelectedTask` share the same `taskRunGate`,
      including the inverse `Profile Init` guard for existing profile files.
  - Historical evidence correction: older artifact entries shaped like
    `sh -n a.sh b.sh ...` only prove the first script operand was parsed by
    `sh -n`; current syntax evidence is recorded as separate `sh -n` commands.
- Archive ingress fail-closed follow-up on 2026-06-04 passed:
  - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachinePackAndUnpackBundleRoundTripsEvidence|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRecordsVerifiedArchiveHandoffEvidence|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityIsTampered|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityFieldsAreMissing|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedOnMalformedArchive|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundlePreservesExistingBundleWhenArchiveMissingExportIdentity|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedOnArchiveDigestMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRejectsSymlinkedArchiveEntriesBeforePublish|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRejectsHardlinkedArchiveEntriesBeforePublish|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRejectsSpecialArchiveEntriesBeforePublish'`
    - 10 tests, 0 failures.
  - Coverage notes:
    - `unpack-bundle` rejects unsafe tar members before extraction;
    - the extracted staging tree is rechecked before any verified handoff is
      recorded or published to the requested incoming bundle root;
    - symlink, hardlink, and FIFO/special-file entries fail closed from staging
      and leave the requested bundle root unpublished.
- Typed status/health final-evaluator follow-up on 2026-06-04 passed:
  - `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedSourceStatusArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedSourceHealthArtifact'`
    - 4 tests, 0 failures.
  - `swift test --package-path macos --filter 'AcceptanceEvaluationTests'`
    - 28 tests, 0 failures.
  - `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceEvaluationScriptTests'`
    - 1 test, 0 failures.
  - Coverage notes:
    - shell final `evaluate` rejects malformed `source.status.json` and
      `source.health.json` before writing `evaluation.json`;
    - Swift final evaluation applies the same typed transfer-proof checks;
    - adjacent evaluator fixtures now carry valid target pairing and network
      transfer control-plane evidence so each negative test reaches its
      intended failure mode.
- Typed status/health advisory parity follow-up on 2026-06-04 passed:
  - Red-first evidence before the fix:
    - `AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceStatusSessionMismatchesTransfer` and
      `AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceHealthLacksTransferSession`
      failed because Swift workflow summary still marked `source_transfer`
      done and advanced to `evaluate`.
    - `AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceStatusSessionMismatch` and
      `AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceHealthWithoutTransferSession`
      failed for the same shell `workflow-status` drift.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceStatusSessionMismatchesTransfer|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceHealthLacksTransferSession'`
    - 2 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceStatusSessionMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceHealthWithoutTransferSession'`
    - 2 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryAdvancesToEvaluateWithoutOperatorEvidenceWhenInstalledAppProofIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptMismatchesSourcePair|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptHasWhitespace|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceStatusSessionMismatchesTransfer|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceHealthLacksTransferSession|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceConsistencySessionHasWhitespace|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryUsesSourceConsistencyArtifactBaselineBeforeMeta'`
    - 7 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedSourceVerifyCounts|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourceReportReceiptMismatchesSourcePair|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourceReportReceiptHasWhitespace|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceStatusSessionMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceHealthWithoutTransferSession|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourceConsistencySessionHasWhitespace|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusUsesSourceConsistencyArtifactBaselineBeforeMeta|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact'`
    - 10 tests, 0 failures.
    `sh -n macos/script/lib/acceptance-two-machine.sh`
    - pass.
  - Coverage notes:
    - shell `workflow-status` now validates `source.status.json` session
      identity, required non-empty status fields, and non-negative counts
      before marking `source_transfer` done;
    - shell `workflow-status` and Swift workflow summary now require
      `source.health.json.network_transfers` to include the current session
      with a non-empty status before advancing to final evaluation.
- Discovery artifact proof parity follow-up on 2026-06-04 passed:
  - Red-first evidence before the fix:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetAdvertiseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetAdvertiseArtifact'`
    - failed because shell `workflow-status` marked malformed
      `source.browse.json` / `target.advertise.json` artifacts done and shell
      final `evaluate` wrote `evaluation.json`.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetAdvertiseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetAdvertiseArtifact'`
    - 4 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedSourceVerifyCounts|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceVerifyCounts|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact|AcceptanceInstalledAppCollectionProofParityScriptTests/testEvaluateFailsClosedWhenRecordedAlternateDiscoveryArtifactIsInvalid'`
    - 6 tests, 0 failures.
    `sh -n macos/script/lib/acceptance-two-machine.sh`
    - pass.
    `sh -n macos/script/acceptance-two-machine.sh`
    - pass.
  - Coverage notes:
    - shell `workflow-status` now decodes optional `source.browse.json` and
      `target.advertise.json` against the same required shape Swift decodes
      before marking discovery steps done;
    - shell final `evaluate` rejects malformed optional discovery artifacts
      before writing `evaluation.json`;
    - this is local proof-policy parity only, not real two-device discovery or
      Local Network/firewall evidence.
- Target-ready proof parity follow-up on 2026-06-04 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMalformed|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetReadyArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedTargetReadyArtifact'`
    - failed because shell/app workflow surfaces still treated
      `meta.json` `evidence.target_ready` as enough target-serve proof, and
      final evaluators did not require canonical `target.ready.json`.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMalformed|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetReadyArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedTargetReadyArtifact'`
    - 8 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetAdvertiseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceBrowseArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetAdvertiseArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorWritesEvaluationArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorCanonicalMachineFactsPreferCanonicalArtifactsOverAlternateMetaOutputs|AcceptanceEvaluationTests/testEvaluationCoordinatorExplicitNonCanonicalSourceArtifactsStayAlignedWithShellEvaluate'`
    - 8 tests, 0 failures.
    `sh -n macos/script/lib/acceptance-two-machine.sh`
    - pass.
    `sh -n macos/script/acceptance-two-machine.sh`
    - pass.
  - Coverage notes:
    - shell `workflow-status`, shell final `evaluate`, Swift workflow summary,
      and Swift final evaluate now require canonical `target.ready.json` to be
      present, regular, well-formed, and consistent with
      `meta.json.evidence.target_ready`;
    - missing or malformed target-ready proof reopens `target_serve_phase_1`
      or blocks `evaluation.json`;
    - this is local proof-policy parity only, not real two-Mac target-serve,
      Local Network/firewall, Developer ID signing/notarization, or final
      T-011 completion evidence.
- Source pair/transfer target-ready consistency follow-up on 2026-06-04
  passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairTargetAddressMismatchesTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferReceiverMismatchesTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferReceiverMismatchWithTargetReady'`
    - failed because shell/app advisory still marked mismatched
      source-pair/source-transfer evidence ready, and final evaluators accepted
      the bundle or wrote `evaluation.json`.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairTargetAddressMismatchesTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferReceiverMismatchesTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferReceiverMismatchWithTargetReady'`
    - 8 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMalformed|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetReadyArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceEvaluationTests/testEvaluationCoordinatorWritesEvaluationArtifact'`
    - 10 tests, 0 failures.
    `sh -n macos/script/lib/acceptance-two-machine.sh`
    - pass.
    `sh -n macos/script/acceptance-two-machine.sh`
    - pass.
  - Coverage notes:
    - shell `workflow-status`, shell final `evaluate`, Swift workflow summary,
      and Swift final evaluate now require `source.pair.json.target_address`
      and `source.transfer.json` target address / target mode / receiver
      endpoint to match canonical `target.ready.json` once target-ready proof
      itself is valid;
    - missing or malformed target-ready proof still reopens
      `target_serve_phase_1` as the first repair lane;
    - this is local proof-policy parity only, not real two-Mac transfer,
      Local Network/firewall/operator proof, Developer ID notarization, or
      final T-011 completion evidence.
- Receiver transfer readiness follow-up on 2026-06-04 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferLacksReceiverReadyTargetArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof'`
    - failed because shell/app advisory advanced to `evaluate` from a
      pairing-ready `target.ready.json` without receiver transfer proof, and
      final evaluators wrote `evaluation.json` or did not throw.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferLacksReceiverReadyTargetArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof'`
    - 4 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMalformed|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairTargetAddressMismatchesTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferReceiverMismatchesTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferLacksReceiverReadyTargetArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetReadyArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMalformedTargetReadyArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceEvaluationTests/testEvaluationCoordinatorWritesEvaluationArtifact'`
    - 22 tests, 0 failures.
  - Coverage notes:
    - shell `workflow-status`, shell final `evaluate`, Swift workflow summary,
      and Swift final evaluate now require canonical `target.ready.json` to
      prove receiver transfer readiness before `source_transfer` can count;
    - the receiver-transfer proof is non-empty `receiver_address`,
      `receiver_routes=true`, `push_network=true`, and `transfer=true`;
    - pairing-ready target proof can still keep `target_serve_phase_1` done,
      but it no longer lets transfer evidence write or preserve
      `evaluation.json`;
    - this is local proof-policy parity only, not real two-Mac transfer,
      Local Network/firewall/operator proof, Developer ID notarization, or
      final T-011 completion evidence.
- Target-import referenced transcript follow-up on 2026-06-04 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingReferencedTargetImportArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingReferencedTargetImportArtifact|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenReferencedTargetImportArtifactIsMissing|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingReferencedTargetImportArtifact'`
    - failed because shell/app advisory advanced to `evaluate` when
      `meta.json` referenced `target.adopt-pairing.txt` but that transcript was
      missing, and final evaluators wrote `evaluation.json` or did not throw.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingReferencedTargetImportArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingReferencedTargetImportArtifact|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenReferencedTargetImportArtifactIsMissing|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingReferencedTargetImportArtifact'`
    - 4 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourcePairReceiptArtifactIsMissing|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusFailsClosedWhenSourcePairReceiptArtifactIsMalformed|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsMissingReferencedTargetImportArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMissingReferencedTargetImportArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsSourceTransferReceiverMismatchWithTargetReady|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenReferencedTargetImportArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairTargetAddressMismatchesTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingReferencedTargetImportArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsSourcePairTargetAddressMismatchWithTargetReady|AcceptanceEvaluationTests/testEvaluationCoordinatorWritesEvaluationArtifact'`
    - 14 tests, 0 failures.
  - Coverage notes:
    - shell `workflow-status`, shell final `evaluate`, Swift workflow summary,
      and Swift final evaluate now require a non-empty
      `target_import.adopted` reference to resolve to a bundle-local regular
      artifact before target import can count;
    - target control-plane pairing receipt remains the strong adoption proof,
      but the bundle can no longer preserve stale transcript references while
      writing or preserving `evaluation.json`;
    - this is local proof-policy parity only, not real two-Mac import,
      Local Network/firewall/operator proof, Developer ID notarization, or
      final T-011 completion evidence.
- Target-import adopted transcript field follow-up on 2026-06-05 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetImportAdoptedTranscriptIsMissing|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetImportAdoptedTranscript'`
    - failed because Swift workflow summary marked `target_import` done and
      advanced to `evaluate` when `target_import` omitted `adopted`, while
      Swift final evaluate wrote `evaluation.json`.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetImportAdoptedTranscriptIsMissing|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetImportAdoptedTranscript'`
    - 2 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenReferencedTargetImportArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetImportAdoptedTranscriptIsMissing|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingReferencedTargetImportArtifact|AcceptanceEvaluationTests/testEvaluationCoordinatorRejectsMissingTargetImportAdoptedTranscript'`
    - 4 tests, 0 failures.
  - Coverage notes:
    - Swift workflow summary and Swift final evaluate now require
      `target_import.adopted` to be non-empty and resolve to a bundle-local
      regular artifact before target import can count;
    - this extends the earlier referenced-transcript check to the field itself
      and does not close real two-Mac import, operator proof, Developer ID
      signing/notarization/stapling, Gatekeeper proof, or final T-011.
- Notary submission-id hardening on 2026-06-05 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceTests/testNotarizationIsReleaseReadyRejectsMalformedSubmissionID|AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMalformedSourceNotarizationSubmissionID|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenSourceNotarizationSubmissionIDIsMalformed'`
    - failed before the fix because app release evidence, app workflow summary,
      shell workflow/evaluate, and Swift final evaluate accepted
      `submission.id="manual-pass"` as release-ready notarization evidence or
      wrote `evaluation.json`.
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedSubmissionID'`
    - failed with the old phase-preflight jq condition restored to the previous
      non-empty-string rule, proving `acceptance_require_ready_app_audit_for_collection`
      also accepted `submission.id="manual-pass"` before the UUID-shaped check.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceTests/testNotarizationIsReleaseReadyRejectsMalformedSubmissionID|AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMalformedSourceNotarizationSubmissionID|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenSourceNotarizationSubmissionIDIsMalformed'`
    - 5 tests, 0 failures.
  - Coverage notes:
    - installed-app release evidence now requires a UUID-shaped Apple notary
      submission id before accepting notarization as release-ready;
    - shell phase preflight, shell workflow status, shell final evaluate, app
      workflow summary, and Swift final evaluate fail closed on malformed
      `submission.id` instead of accepting a hand-written placeholder;
    - this is release-evidence proof-policy hardening only, not real Developer
      ID notarization, staple, Gatekeeper, or two-Mac installed-app acceptance
      evidence.
- First-class notarization script submission-id hardening on 2026-06-05 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenSubmissionIDIsNotUUID'`
    - failed before the fix because `macos/script/notarize-app.sh` accepted
      `submission.id="manual-pass"`, fetched the notary log, stapled the app,
      ran the post-staple audit, and emitted `status=pass`.
  - Green evidence:
    `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenSubmissionIDIsNotUUID'`
    - 1 test, 0 failures.
    `swift test --package-path macos --filter 'NotarizeAppScriptTests'`
    - 8 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceTests/testNotarizationIsReleaseReadyRejectsMalformedSubmissionID|AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMalformedSourceNotarizationSubmissionID|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenSourceNotarizationSubmissionIDIsMalformed'`
    - 5 tests, 0 failures.
    `sh -n macos/script/notarize-app.sh`
    - passed.
    `git diff --check`
    - passed.
  - Coverage notes:
    - the repository's first-class notarization script now fails closed before
      notary-log retrieval or stapling when `notarytool submit` output lacks a
      UUID-shaped Apple submission id;
    - blocked exits still persist the structured work-dir result and canonical
      sibling `.app.notary/notarization.json` sidecar, so downstream app/shell
      acceptance sees a durable blocked result instead of a local pass that the
      final evaluator would later reject;
    - this closes a source-workflow proof drift only. It does not add real
      Apple notary credentials, Developer ID signing, staple, Gatekeeper, or
      two-Mac installed-app acceptance evidence.
- First-class notarization script notary-log hardening on 2026-06-05 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenNotaryLogIsNotAccepted'`
    - failed before the fix because `macos/script/notarize-app.sh` accepted a
      fetched notary log with `status="Invalid"`, stapled the app, ran the
      post-staple audit, and emitted `status=pass`.
    `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenNotaryLogIssuesIsMalformed'`
    - failed before the fix because the same script accepted a fetched notary
      log with `status="Accepted"` but `issues` encoded as a string, stapled
      the app, ran the post-staple audit, and emitted `status=pass`.
  - Green evidence:
    `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenNotaryLogIsNotAccepted'`
    - 1 test, 0 failures.
    `swift test --package-path macos --filter 'NotarizeAppScriptTests/testNotarizeScriptFailsClosedWhenNotaryLogIssuesIsMalformed'`
    - 1 test, 0 failures.
    `swift test --package-path macos --filter 'NotarizeAppScriptTests'`
    - 10 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedNotaryLog|AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationBoundToTargetNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMissingBundleLocalNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMalformedBundleLocalNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsNotarizationBoundToOtherMachineNotaryLog|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleLocalNotaryLogIsMalformed'`
    - 6 tests, 0 failures.
    `sh -n macos/script/notarize-app.sh`
    - passed.
    `git diff --check`
    - passed.
    `swift test --package-path macos --filter 'NotarizeAppScriptTests'`
    - 9 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedNotaryLog|AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationBoundToTargetNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMissingBundleLocalNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsMalformedBundleLocalNotaryLog|AcceptanceTwoMachineScriptTests/testTwoMachineStrictAcceptanceRejectsNotarizationBoundToOtherMachineNotaryLog|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleLocalNotaryLogIsMalformed'`
    - 6 tests, 0 failures.
    `sh -n macos/script/notarize-app.sh`
    - passed.
    `git diff --check`
    - passed.
  - Coverage notes:
    - the repository's first-class notarization script now fails closed after
      notary-log retrieval but before stapling when `notarytool log` output is
      malformed, does not report `status=Accepted`, or carries an `issues`
      field that is neither absent/null nor an array;
    - blocked exits still persist the structured work-dir result and canonical
      sibling `.app.notary/notarization.json` sidecar, so downstream app/shell
      acceptance sees a durable blocked result instead of a local pass that the
      final evaluator would later reject;
    - this closes another source-workflow proof drift only. It does not add
      real Apple notary credentials, Developer ID signing, staple, Gatekeeper,
      or two-Mac installed-app acceptance evidence.
- Two-machine phase-preflight notary-log hardening on 2026-06-05 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotaryLog'`
    - failed before the fix because
      `acceptance_require_ready_app_audit_for_collection` accepted a current
      sibling `notary-log.json` with `status="Accepted"` but `issues` encoded
      as a string, copied `target.notarization.json` / `target.notary-log.json`
      into the bundle, and exited `0`.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotaryLog'`
    - 1 test, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionAcceptsDistributionReadyAudit|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotaryLog|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedSubmissionID|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsNotarizationWithoutSubmissionID'`
    - 4 tests, 0 failures.
    `sh -n macos/script/lib/acceptance-common.sh`
    - passed.
    `sh -n macos/script/lib/acceptance-two-machine.sh`
    - passed.
    `sh -n macos/script/acceptance-two-machine.sh`
    - passed.
  - Coverage notes:
    - two-machine phase preflight now rejects sibling notary logs unless they
      match the same accepted-log shape (`status=Accepted` and `issues`
      absent/null/array) before copying notarization evidence into the bundle;
    - malformed sibling notary logs are cleared from bundle-local
      notarization evidence and fail with exit `5`, so real phase execution
      does not start on release evidence the later workflow/final gates would
      reject;
    - this is release-evidence collection parity only. It does not add real
      Developer ID credentials, notarization, staple, Gatekeeper, or two-Mac
      installed-app acceptance evidence.
- App-side packaging collection notary-log hardening on 2026-06-05 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts'`
    - failed before the fix because `AcceptancePackagingEvidenceCollector`
      accepted a current sibling `notary-log.json` with `status="Accepted"` but
      `issues` encoded as a string, published fresh `source.app-audit.json`,
      `source.provenance.json`, `source.notarization.json`, and
      `source.notary-log.json`, and left release evidence present.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts'`
    - 1 test, 0 failures.
    `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorCopiesStructuredNotarizationEvidenceWhenPresent|AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts|AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedStructuredNotarizationEvidenceWithoutPublishingFreshPackagingArtifacts|AcceptancePackagingEvidenceTests/testCollectorRejectsStaleStructuredNotarizationEvidenceWithoutPublishingFreshPackagingArtifacts|AcceptancePackagingEvidenceTests/testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing'`
    - 5 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceInstalledAppWorkflowSummaryTests/testWorkflowSummaryRejectsSourceNotarizationWithMalformedNotaryLog|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleLocalNotaryLogIsMalformed|AcceptanceTwoMachineScriptTests/testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotaryLog'`
    - 3 tests, 0 failures.
    `git diff --check`
    - passed.
  - Coverage notes:
    - app-side packaging collection now rejects sibling notary logs unless they
      match the same accepted-log shape before staging notarization evidence;
    - clearing stale notarization evidence now also removes the copied
      `source|target.notary-log.json` leaf, so orphan release evidence does not
      linger after a malformed/missing sidecar path;
    - this is app/shell collection parity only. It does not add real Developer
      ID credentials, notarization, staple, Gatekeeper, or two-Mac installed-app
      acceptance evidence.
- App-side stale notary-log symlink cleanup on 2026-06-05 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing'`
    - failed before the fix because clearing local notarization evidence removed
      `source.notarization.json` but left `source.notary-log.json` as a
      dangling symlink leaf in the bundle.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing'`
    - 1 test, 0 failures.
    `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing|AcceptancePackagingEvidenceTests/testCollectorCopiesStructuredNotarizationEvidenceWhenPresent|AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts|AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedStructuredNotarizationEvidenceWithoutPublishingFreshPackagingArtifacts|AcceptancePackagingEvidenceTests/testCollectorRejectsStaleStructuredNotarizationEvidenceWithoutPublishingFreshPackagingArtifacts'`
    - 5 tests, 0 failures.
  - Coverage notes:
    - app-side stale copied notarization cleanup now treats dangling symlink
      leaves as existing bundle-local evidence nodes and removes them, matching
      shell collection cleanup behavior;
    - this keeps malformed/missing local sidecar cleanup from leaving hidden
      stale proof artifacts. It does not add real Developer ID credentials,
      notarization, staple, Gatekeeper, or two-Mac installed-app acceptance
      evidence.
- Shell packaging collection notary-log output safety on 2026-06-05 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotaryLogOutputLeaf'`
    - failed before the fix because `record-packaging-evidence` exited 0,
      replaced an existing symlinked `source.notary-log.json` output leaf, and
      wrote `workflow.summary.json`.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotaryLogOutputLeaf'`
    - 1 test, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceWritesReadyArtifactsForSourceMachine|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotarizationOutputLeaf|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotaryLogOutputLeaf|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsStaleSiblingNotarizationWhoseAuditDoesNotMatchCurrentApp'`
    - 4 tests, 0 failures.
    `sh -n macos/script/lib/acceptance-common.sh`
    - passed.
  - Coverage notes:
    - shell collection now refuses to overwrite existing bundle-local notary-log
      output leaves unless they are single-link regular files, matching its
      existing notarization output preflight;
    - this is release-evidence collection hardening only. It does not add real
      Developer ID credentials, notarization, staple, Gatekeeper, or two-Mac
      installed-app acceptance evidence.
- App-side packaging collection output-leaf hardlink safety on 2026-06-05
  passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsHardlinkedNotaryLogOutputLeafBeforeWriting'`
    - failed before the fix because `AcceptancePackagingEvidenceCollector`
      succeeded, published fresh `source.version.txt`,
      `source.provenance.json`, and `source.app-audit.json`, and overwrote an
      existing hardlinked bundle-local `source.notary-log.json` output leaf.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsHardlinkedNotaryLogOutputLeafBeforeWriting'`
    - 1 test, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsHardlinkedNotaryLogOutputLeafBeforeWriting|AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsSymlinkedNotarizationOutputLeaf'`
    - 2 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptancePackagingEvidenceTests/testCollectorCopiesStructuredNotarizationEvidenceWhenPresent|AcceptancePackagingEvidenceTests/testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing|AcceptancePackagingEvidenceTests/testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts'`
    - 3 tests, 0 failures.
  - Coverage notes:
    - app-side packaging collection now preflights existing bundle-local
      version, provenance, app-audit, notarization, and notary-log output
      leaves as single-link regular files before staging or publishing fresh
      evidence;
    - this closes one app/shell/evaluator proof-policy drift around linked
      release-evidence output leaves. It does not add real Developer ID
      credentials, notarization, staple, Gatekeeper, or two-Mac installed-app
      acceptance evidence.
- Shell packaging collection mandatory-output hardlink safety on 2026-06-05
  passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsHardlinkedBundleAppAuditOutputLeaf'`
    - failed before the fix because shell `record-packaging-evidence` exited
      0, rewrote an external hardlinked `source.app-audit.json`, and wrote
      `workflow.summary.json`.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsHardlinkedBundleAppAuditOutputLeaf'`
    - 1 test, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceWritesReadyArtifactsForSourceMachine|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsHardlinkedBundleAppAuditOutputLeaf|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotarizationOutputLeaf|AcceptanceInstalledAppReleaseEvidenceScriptTests/testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotaryLogOutputLeaf'`
    - 4 tests, 0 failures.
    `sh -n macos/script/lib/acceptance-common.sh`
    - passed.
  - Coverage notes:
    - shell packaging collection now preflights existing bundle-local version,
      provenance, and app-audit output leaves with the same single-link regular
      policy already used for notarization/notary-log outputs;
    - shell app-audit output is written through a temporary file before
      publishing, so malformed or unsafe output leaves do not mutate external
      hardlink targets;
    - this is release-evidence collection hardening only. It does not add real
      Developer ID credentials, notarization, staple, Gatekeeper, or two-Mac
      installed-app acceptance evidence.
- App-side phase artifact output-leaf hardlink safety on 2026-06-05 passed:
  - Red-first evidence:
    `swift test --package-path macos --filter 'AcceptanceBundleArtifactWriterTests/testWriterRejectsHardlinkedSourcePairOutputLeafBeforeWritingPhaseArtifacts'`
    - failed before the fix because `writeSourcePair` did not throw, wrote
      `source.machine.json`, `source.pair.txt`, and
      `exported-receipts/pair-1.json`, and updated source-pair metadata while
      `source.pair.json` was hardlinked outside the bundle.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceBundleArtifactWriterTests/testWriterRejectsHardlinkedSourcePairOutputLeafBeforeWritingPhaseArtifacts'`
    - 1 test, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceBundleArtifactWriterTests/testWriterRecordsServePairAndTransferArtifacts|AcceptanceBundleArtifactWriterTests/testWriterRecordsDiscoveryArtifacts|AcceptanceBundleArtifactWriterTests/testWriterSourcePairRewritesCanonicalSourceMachineIdentityForInstalledAppProof|AcceptanceBundleArtifactWriterTests/testWriterServePhaseRewritesCanonicalTargetMachineIdentityForInstalledAppProof|AcceptanceBundleArtifactWriterTests/testWriterRejectsHardlinkedSourcePairOutputLeafBeforeWritingPhaseArtifacts'`
    - 5 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testAcceptanceBundleArtifactWriterRejectsSymlinkedBundleRootBeforeWriting|AcceptanceBundleRootTrustTests/testAcceptanceBundleArtifactWriterRejectsSymlinkedMetaBeforeWriting|AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsHardlinkedNotaryLogOutputLeafBeforeWriting|AcceptanceBundleRootTrustTests/testPackagingEvidenceCollectorRejectsSymlinkedNotarizationOutputLeaf'`
    - 4 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceBundleArtifactWriterTests'`
    - 17 tests, 0 failures.
  - Coverage notes:
    - app-side phase authoring now preflights existing bundle-local machine
      facts, discovery, target-ready, exported receipt, source-pair /
      source-transfer, transcript, source-consistency, and evaluation output
      leaves as single-link regular files before writing fresh evidence;
    - app-side copied phase artifacts stage through a temporary file before
      publication, so unsafe destination leaves do not mutate external hardlink
      targets;
    - this is app-authored bundle phase hardening only. It does not add real
      Developer ID credentials, notarization, staple, Gatekeeper, or two-Mac
      installed-app acceptance evidence.
- Script helper pipe-drain follow-up on 2026-06-04 passed:
  - Red-first evidence with old helper:
    `swift test --package-path macos --filter 'AcceptanceScriptHarnessTests/testRunProcessAllowFailureDrainsStdoutAndStderrWhileProcessRuns'`
    - failed with exit 9 after the watchdog killed the child process;
    - captured only stdoutBytes=65536 and stderrBytes=61455;
    - missing `stdout-finished` / `stderr-finished` markers.
  - Green evidence:
    `swift test --package-path macos --filter 'AcceptanceScriptHarnessTests/testRunProcessAllowFailureDrainsStdoutAndStderrWhileProcessRuns'`
    - 1 test, 0 failures.
  - Adjacent evidence:
    `swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact|AcceptanceTwoMachineScriptTests/testTwoMachinePackAndUnpackBundleRoundTripsEvidence'`
    - 3 tests, 0 failures.
  - Broader helper consolidation evidence:
    `swift test --package-path macos --filter 'AcceptanceScriptHarnessTests|AppAuditTamperTests|NotarizeAppScriptTests'`
    - 17 tests, 0 failures.
    `swift test --package-path macos --filter 'AcceptanceEvaluationIntegrationTests|AcceptanceBundleAppOperationsIntegrationTests'`
    - 24 tests skipped by env gates, 0 failures.
    `rg -n "waitUntilExit\(\)|readDataToEndOfFile\(\)" macos/SuperMoverAppTests -g '*.swift'`
    - only `AcceptanceScriptTestSupport.swift` remains, as the shared
      concurrent-drain implementation.
  - Coverage notes:
    - shared `AcceptanceScriptHarness.runProcessAllowFailure` drains stdout
      and stderr concurrently while the child process runs;
    - Swift script and acceptance integration launch helpers now delegate to
      that shared harness instead of hand-rolling wait-then-read pipe capture.
- Final local i18n/UI-preferences closeout on 2026-06-05 passed:
  - Visible app-owned role chrome in Prepare headers, Task Dispatch, transfer
    metadata, and Control Room context now uses localized role labels instead
    of raw `label: "role"` badges or `Role •` prefixes.
  - Raw role identity, command previews, profile/config paths,
    TaskInput-derived CLI arguments, evidence/proof values, artifact fields,
    and CLI output remain unlocalized and audit-stable.
  - The full Swift-suite closeout also aligned AppStoreTests fixtures with the
    current strict release-proof contract: bundle-local notary logs,
    bundle-relative `notary_log.path` fields, and raw machine-bound operator
    evidence are now present in the fixture evidence.
  - Green evidence:
    `swift test --package-path macos --filter 'UIPreferencesTests|WorkbenchNavigationTests|AppStoreTests/testSetupGuide|AppStoreTests/testLocalizedSetupGuide|AppStoreTests/testLocalizedProfileSelectionDisplayDoesNotChangeRawConfigValues|AppStoreTests/testWorkbenchRoleLocalizedLabelsDoNotChangeRoleIdentity|AppStoreTests/testUIPreferencesDoNotChangeCommandInputsOrPreviewContracts|AcceptanceScriptHarnessTests/testBuildAppResourceCopyIncludesProcessedLocalizationDirectories'`
    - 27 tests, 0 failures.
    `swift test --package-path macos --filter 'AppStoreTests'`
    - 116 tests, 0 failures.
    `swift test --package-path macos`
    - 692 tests, 33 skipped, 0 failures.
    `go test -count=1 ./...`
    - passed.
  - Coverage notes:
    - this is local UI/i18n clarity plus test-fixture drift closeout only;
    - the slow full Swift suite remains useful as a release/hygiene gate, but
      future UI-only work should start with focused UI/resource/build checks and
      reserve the shell-backed full acceptance matrix for closeout or
      pre-merge validation;
    - this does not add real Developer ID credentials, notarization, staple,
      Gatekeeper, Local Network/firewall prompt evidence, or real two-Mac
      installed-app acceptance evidence.
- Prepare profile creation UX follow-up:
  - Source-side basic setup now defaults to the recommended
    `~/.supermover/profile-local.json` migration config destination. The
    primary setup action can select that recommended path before running
    `profile init`, while `Use Recommended Config` can preselect it without
    launching the CLI.
  - Manual profile destination selection remains available only under the
    Advanced disclosure as `Choose Custom Location`; the ordinary config card
    no longer asks operators to choose where a new config file should live.
  - Green evidence:
    `swift test --package-path macos --filter 'AppStoreTests/testSetupGuideExplainsEmptySourcePreparationInUserOrder|AppStoreTests/testLocalizedSetupGuideExplainsEmptySourcePreparationWithoutChangingRawGuide|AppStoreTests/testLocalizedProfileSelectionDisplayDoesNotChangeRawConfigValues|AppStoreTests/testRecommendedProfileDestinationSelectsSuperMoverProfileWithoutAutoLaunching|AppStoreTests/testApplyProfileDestinationSelectionDoesNotAutoLaunchProfileCreation|UIPreferencesTests/testAppChromeLocalizationLoadsPrepareChromeResources'`
    - 6 tests, 0 failures.
    Python resource-key audit: English and Simplified Chinese
    `Localizable.strings` both have 141 keys, no duplicates, and no missing
    counterpart keys.
  - Boundary:
    This is basic Prepare UX simplification only. It keeps profile files as the
    CLI `--profile` SSOT, does not add runtime policy overrides, and does not
    close real two-Mac, Local Network/firewall, Developer ID notarization,
    Gatekeeper, or final release gates.
- Open follow-ups from static review: consolidation of duplicated shell/Python
  regular-artifact helper code after behavior is already fail-closed.
- This artifact is under the ignored feature-tracker artifacts directory and
  must be committed with:
  `git add -f .bagakit/feature-tracker/features/f-23bnwxry2/artifacts/app-acceptance-T-011.md`.

## Acceptance Boundary

The local app is now buildable and locally testable as a packaged CLI-backed
workbench. Local packaged evidence now includes a same-machine non-dry-run
profile-backed mTLS transfer through the installed bundle, but it is not yet a
final distributed two-machine release until signed and notarized packaging plus
two-machine LAN acceptance evidence are recorded.
