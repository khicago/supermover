# App-First LAN Acceptance Handoff

This document is the handoff for a strong engineer who does not already have
project context. Read it before changing T-011, app-first acceptance, macOS
packaging/notarization, or distinct-machine installed-app proof surfaces.

It is not a milestone brag sheet. It is a working map for finishing the real
objective:

- a genuinely installable native macOS app-first LAN one-way migration system
- two real Macs
- discovery, pairing, pinned TLS 1.3 mTLS transfer, observable sync, auditable
  evidence, end-to-end consistency verification
- release-grade signing/notarization/stapling acceptance
- every unverified capability fail-closed and recorded as incomplete

## First Principles

### 1. The product contract is evidence-first, not UI-first

Supermover is not a generic sync app and not a demo dashboard. It is a
one-way, auditable migration system. Operator truth must come from durable
artifacts, profile SSOT, and target control-plane evidence, not from transient
stdout or optimistic UI state.

Corollary:

- any green state without durable evidence is a bug
- any advisory surface that is more optimistic than the final gate is a bug
- any capability that is merely wired but not verified must remain blocked or
  explicitly marked unavailable/review-only

### 2. Discovery is not trust; packaging is not release; same-machine is not two-machine

This repository already contains many useful vertical slices. The main failure
mode has been over-crediting them.

Keep these boundaries hard:

- LAN discovery hints are not trust
- same-machine localhost mTLS is not real two-device LAN acceptance
- ad-hoc or unsigned app audit is not release readiness
- shell or app authoring of bundle artifacts is not proof unless the final gate
  accepts the same evidence contract

### 3. The real unit of progress is “more of the final claim becomes true”

Do not optimize for:

- prettier UI
- broader but weaker test suites
- new wrappers that make code look DRY but split policy across surfaces
- another same-machine substitute once a shell/app proof gap is the real risk

Optimize for:

- stronger fail-closed substrate
- app/shell/operator agreement on the same truth
- narrower distance between current evidence and a real two-Mac installed-app
  acceptance run

## Global Goal

Make SuperMover shippable as a native macOS app-first LAN migration system,
with proof strong enough to survive a requirement-by-requirement audit.

This means all of the following must become true and verified:

1. source and target installed apps run on two distinct real Macs
2. source discovers target, but trust still requires explicit pairing
3. pairing writes durable receipt/profile pin evidence
4. transfer uses pinned TLS 1.3 mTLS with durable receiver/session evidence
5. the app surfaces current evidence and missing proof honestly
6. end-to-end consistency is verified from durable artifacts
7. packaged app passes release-grade signing/notarization/staple/Gatekeeper
8. any missing proof remains blocked and recorded as incomplete

As of now, that full claim is not true.

## Action Principles

### Do

- preserve profile files as SSOT
- preserve `.supermover` control-plane evidence as authoritative
- keep app/shell/evaluator semantics aligned
- add tests at safety boundaries before or with the fix
- keep docs and tracker truth brutally honest
- prefer one deep proof/presenter module over several weak wrapper abstractions
- use same-machine and fake-notary only when they make the final contract more
  rigorous, not when they merely create more local green checks

### Do Not

- do not redefine success around the current strong slice
- do not claim two-machine acceptance from same-machine evidence
- do not claim release readiness from ad-hoc or unsigned audits
- do not let `workflow-status`, app preview, app preflight, and shell evaluate
  each drift into their own policy
- do not move shared acceptance proof logic into UI-only feature files
- do not touch `internal/reconcile/reconcile.go`

### Avoid

- shallow DRY wrappers that pass `AnyView`, booleans, or “summary-ish” structs
  around while the real policy stays split
- feature work that increases optimism without increasing proof
- large refactors that blur whether the system became safer or just different
- using ignored tracker state as the only record of truth; normalize durable
  facts into checked-in docs

## Current Context Panorama

### A. What is implemented and materially useful

1. **CLI / local migration substrate**
   - local one-way migration, report/status/health, drift record/ack/resolve,
     narrow reconcile plan/review/apply, sync queue/run/loop/watch/network, and
     current bounded network path are wired

2. **macOS app-first workbench**
   - app can drive setup, selected wired commands, discovery/pairing/sync
     orchestration, evidence vault, CLI provenance/readiness, and acceptance
     bundle authoring surfaces

3. **packaged app substrate**
   - `macos/script/build-app.sh`
   - `macos/script/audit-app.sh`
   - `macos/script/notarize-app.sh`
   - bundled CLI provenance manifest
   - unsigned/ad-hoc local audit correctly blocks distribution readiness

4. **same-machine acceptance substrate**
   - packaged loopback acceptance
   - same-machine five-phase two-machine simulation
   - bundle pack/unpack/merge handoff substrate
   - machine-facts artifacts
   - verified bundle handoff ledger

5. **distinct-machine proof hardening**
   - installed-app proof now requires agreement across:
     - collection mode / machine count
     - role machine ids
     - `evidence.machine_facts.*`
     - `source.machine.json` / `target.machine.json`
     - verified cross-machine `bundle_handoffs`
   - shell `workflow-status` and `evaluate --require-operator-evidence` now
     consume the same installed-app proof verdict surface

### B. What is still definitely incomplete

1. **Real two-device acceptance**
   - no fresh evidence yet of source/target installed apps on two distinct Macs
     completing the full app-first procedure

2. **Real release acceptance**
   - no fresh Developer ID + notarize + staple + Gatekeeper success evidence
     for the canonical packaged app

3. **Permission/operator evidence on real devices**
   - no fresh Local Network / firewall / physical pairing confirmation evidence
     from two actual Macs

4. **End-state proof chain**
   - no final audited chain yet that ties:
     - release-quality packaged apps
     - distinct-machine handoff
     - operator evidence
     - non-dry-run network transfer
     - end evaluation

### C. Known unknowns

1. **Where the first real two-machine run will fail**
   - likely candidates:
     - firewall / Local Network prompt timing
     - machine-facts/operator evidence bundle handling
     - packaged app launch preflight assumptions
     - real notarized sidecar availability at both ends

2. **How much of the app-side acceptance authoring path survives a real run**
   - current app surfaces are strong, and corrective installed-app
     `source-pair` / `target-serve` launches now rewrite canonical machine
     facts plus `roles.source_pair` / `roles.target` from the current installed
     app instead of only relaxing preview/preflight
   - that authoring path is still not yet proven on two real devices

3. **Whether any remaining advisory drift exists outside shell**
   - shell proof contract is tighter now
   - app-side advisory/preflight surfaces must still be re-checked against the
     same verdict semantics when pushing toward real-device execution

### D. Known but not yet fully explored areas

1. **App-side acceptance surfaces after the shell proof refactor**
   - `AppStore.acceptanceInstalledAppLaunchPreview`
   - `AppStore.acceptanceInstalledAppLaunchPreflightError`
   - `AcceptanceBundleLoadedSnapshot.workflowSummary(...)`
   - `AcceptanceBundleEvaluationCoordinator`

2. **Operator procedure itself**
   - the shell/archive substrate is present
   - the full two-machine runbook has not yet been driven end to end on real
     devices with release-quality apps

3. **Release pipeline closure**
   - the workflow surfaces exist, but canonical final artifacts are still absent

## Most Important Recent Work

The highest-value recent slice is not UI. It is shell distinct-machine proof
alignment.

### What changed

Files:

- `macos/script/lib/acceptance-two-machine.sh`
- `macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift`

Key result:

- `workflow-status` and `evaluate --require-operator-evidence` now share one
  installed-app proof verdict surface instead of separately interpreting weaker
  handoff/machine booleans.

That verdict includes:

- collection mode / machine count
- role machine ids
- machine-facts summary ids
- machine-facts artifact ids
- machine-facts consistency
- verified bundle handoffs
- whether the handoff proves the recorded source/target machine pair
- `blocked_reason / missing_requirements`
- `requires_machine_identity_correction / requires_bundle_handoff_proof`
- `final_evaluation_collection_detail / final_evaluation_machine_facts_detail /
  final_evaluation_bundle_handoff_detail`
- `installed_app_proof_ok / installed_app_proof_failures / failures /
  primary_failure / failure_message`

The proof-specific readiness bit is `installed_app_proof_ok`. The top-level
`workflow-status` `ok` bit now only turns true after `evaluate` has already
written `evidence_collected` and the bundle has no remaining `next_actions`, so
advisory JSON no longer looks complete while release, operator, or
pending-evaluate work still exists.

### Why it mattered

Previously the system risked a dangerous split:

- advisory shell surface could say “ready to evaluate”
- final evaluate surface could still apply a stricter policy

That is exactly the kind of optimism drift this product cannot afford.

### What is now directly tested

Focused shell coverage now pins:

- positive advance from `workflow-status` to `evaluate`
- persisted `workflow.summary.json` and `meta.json` workflow-summary refresh
  for the machine-identity correction lane
- same-machine handoff rejected
- wrong-machine-pair handoff rejected
- meta-vs-artifact machine-facts mismatch rejected
- role-vs-meta machine-facts disagreement rejected
- role-vs-machine-facts artifact disagreement rejected
- `evaluate` direct failure for:
  - same-role-machine-id collapse with
    `source_pair and target share machine_id=same-machine`
  - artifact mismatch with
    `roles.source_pair/target machine_id do not match source.machine.json and
    target.machine.json`
  - role-vs-machine-facts conflict with that same machine-facts detail
  - wrong-pair / contradictory handoff detail precedence after collection and
    machine-facts checks

Broader focused acceptance tests also passed for:

- `AcceptanceTwoMachineScriptTests`
- selected `AcceptanceEvaluationTests`

### What this did not do

- it did not produce real two-device evidence
- it did not make release signing/notary complete
- it did not eliminate the need to inspect app-side advisory surfaces for the
  same proof semantics

## Files and Surfaces You Must Understand

### Shell / operator substrate

- `macos/script/acceptance-two-machine.sh`
- `macos/script/lib/acceptance-two-machine.sh`
- `macos/script/acceptance-two-machine-same-machine.sh`
- `docs/runbook.md`

### App-side acceptance surfaces

- `macos/SuperMoverApp/AppStore.swift`
- `macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift`
- `macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift`
- `macos/SuperMoverApp/AcceptanceBundleAppOperations.swift`
- `macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift`

### Tests that currently define truth for this lane

- `macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift`
- `macos/SuperMoverAppTests/AcceptanceEvaluationTests.swift`
- `macos/SuperMoverAppTests/AcceptanceEvaluationIntegrationTests.swift`
- `macos/SuperMoverAppTests/AcceptanceBundleAppOperationsIntegrationTests.swift`
- `macos/SuperMoverAppTests/AcceptanceBundleTests.swift`

### Tracker / status / docs

- `.bagakit/feature-tracker/features/f-23bnwxry2/tasks.json`
- `.bagakit/feature-tracker/features/f-23bnwxry2/state.json`
- `.bagakit/feature-tracker/features/f-23bnwxry2/verification.md`
- `.bagakit/feature-tracker/features/f-23bnwxry2/artifacts/app-acceptance-T-011.md`
- `docs/plan.md`
- `docs/runbook.md`
- `README.md`

## Ownership Map

Use this map to avoid wandering across unrelated surfaces.

### Shell / operator substrate

- `macos/script/lib/acceptance-two-machine.sh`
  - authoritative shell-side distinct-machine acceptance logic
  - bundle handoff substrate
  - machine-facts and installed-app proof verdict
  - shell `workflow-status` and shell `evaluate`
- `macos/script/acceptance-two-machine.sh`
  - CLI wrapper / operator entry surface
- `macos/script/acceptance-two-machine-same-machine.sh`
  - same-machine simulation harness only

### App-side advisory and authoring surfaces

- `macos/SuperMoverApp/AppStore.swift`
  - app task orchestration
  - installed-app preview/preflight decisions
  - acceptance bundle record/load flows
- `macos/SuperMoverApp/AcceptanceBundleLoadedSnapshot.swift`
  - app-side loaded-bundle interpretation
  - workflow summary / advisory state
- `macos/SuperMoverApp/AcceptanceBundleEvaluationCoordinator.swift`
  - Swift-side final evaluation semantics
- `macos/SuperMoverApp/AcceptanceBundleAppOperations.swift`
  - app bundle authoring / packaging evidence write path
- `macos/SuperMoverApp/AcceptancePackagingEvidenceCollector.swift`
  - packaging/notarization evidence capture and stale cleanup

### Release packaging and notarization

- `macos/script/build-app.sh`
- `macos/script/audit-app.sh`
- `macos/script/notarize-app.sh`
- `macos/script/Info.plist`

### Current proof-defining tests

- `macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift`
- `macos/SuperMoverAppTests/AcceptanceInstalledAppCollectionProofParityScriptTests.swift`
- `macos/SuperMoverAppTests/AcceptanceInstalledAppProofParityTests.swift`
- `macos/SuperMoverAppTests/AcceptanceInstalledAppWorkflowSummaryTests.swift`
- `macos/SuperMoverAppTests/AcceptanceInstalledAppLaunchCoordinatorTests.swift`
- `macos/SuperMoverAppTests/AcceptanceEvaluationTests.swift`
- `macos/SuperMoverAppTests/AcceptanceBundleTests.swift`
- `macos/SuperMoverAppTests/AcceptanceBundleAppOperationsIntegrationTests.swift`
- `macos/SuperMoverAppTests/AcceptanceEvaluationIntegrationTests.swift`
- `macos/SuperMoverAppTests/NotarizeAppScriptTests.swift`

### Tracker / docs

- `.bagakit/feature-tracker/features/f-23bnwxry2/tasks.json`
- `.bagakit/feature-tracker/features/f-23bnwxry2/state.json`
- `.bagakit/feature-tracker/features/f-23bnwxry2/verification.md`
- `.bagakit/feature-tracker/features/f-23bnwxry2/artifacts/app-acceptance-T-011.md`
- `docs/plan.md`
- `docs/runbook.md`
- `README.md`

## Minimal Takeover Route

If you need the shortest path from zero context to useful execution, do this in
order and do not branch out before step 5:

1. Read:
   - `AGENTS.md`
   - `docs/must-guidebook.md`
   - `docs/must-authority.md`
   - `docs/plan.md`
   - `docs/runbook.md`
   - this handoff

2. Re-run the current proof baseline exactly:

```bash
sh -n macos/script/lib/acceptance-common.sh
sh -n macos/script/lib/acceptance-two-machine.sh
sh -n macos/script/acceptance-two-machine.sh
swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsLinkedBundleMeta|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusRejectsLinkedBundleMeta|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotUseHardlinkedMachineFactArtifactsAsProof|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotUseHardlinkedReleaseArtifactsAsReady|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsBundleLocalArtifactHardlinks|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsTargetControlPlaneHardlinks'
swift test --package-path macos --filter 'AcceptanceBundleRootTrustTests/testAcceptanceBundleReaderRejectsSpecialMetaFile|AcceptanceBundleRootTrustTests/testOperatorEvidenceStoreRejectsSpecialMetaBeforeWriting|AcceptanceBundleRootTrustTests/testOperatorEvidenceStoreRejectsHardlinkedMetaBeforeWriting|AcceptanceBundleRootTrustTests/testAcceptanceBundleReaderRejectsHardlinkedMetaFile|AcceptanceBundleTests/testAcceptanceBundleReaderLoadsEvidenceCollectedBundle|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMalformed|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMissing|AcceptanceBundleTests/testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsDirectory|AcceptanceEvaluationModeTests|AcceptanceInstalledAppLaunchGateTests'
swift test --package-path macos --filter 'AppStoreTests/testTaskRunGateBlocksProfileInitForExistingProfileFile|AppStoreTests/testTaskRunGateBlocksExistingProfileTasksForNewProfileDestination|AppStoreTests/testTaskRunGateBlocksPublishWithoutSessionID|AppStoreTests/testTaskRunGateBlocksReconcileApplyWithoutReason|WorkbenchNavigationTests|WorkbenchChromeTests/testDetailPageStickyHeaderStopsAtPaneTopWithoutOvershoot'
```

Run shell syntax checks as separate `sh -n` commands. Historical records that
used `sh -n a.sh b.sh ...` only prove the first script operand was parsed.

3. Inspect the exact proof ownership path:
   - shell verdict source:
     `acceptance_two_machine_installed_app_proof_summary`
   - shell advisory consumer:
     `acceptance_two_machine_compute_workflow_status`
   - shell final-gate consumer:
     `acceptance_two_machine_evaluate`
   - app advisory consumers:
     `AcceptanceBundleLoadedSnapshot.workflowSummary(...)`
     `AppStore.acceptanceInstalledAppLaunchPreview(...)`
     `AppStore.acceptanceInstalledAppLaunchPreflightError(...)`
   - Swift final gate:
     `AcceptanceBundleEvaluationCoordinator`

4. Check whether app-side advisory surfaces consume the same proof semantics as
   shell verdicts or are still more optimistic. The current local parity slice
   covers typed artifact paths, exported receipt schema validation,
   bundle-local hardlink rejection, linked/non-regular `meta.json` rejection,
   workflow-status machine-facts/release artifact hardlink rejection,
   archive-ingress symlink/hardlink/special-file rejection, target
   pairing/network-transfer schema checks, and Task Dispatch shared
   `taskRunGate`; shell and Swift final evaluators now also reject malformed
   `source.status.json` / `source.health.json` transfer evidence, and shell
   `workflow-status` plus Swift app workflow summary use the same typed
   status/health checks before advancing `source_transfer`. Shell
   `workflow-status` and shell final evaluate also validate optional
   `source.browse.json` / `target.advertise.json` discovery artifacts against
   the same required JSON shape Swift decodes before treating those steps as
   done. The same proof-parity rule now applies to target serve readiness:
   `target.ready.json` must be present, regular, well-formed, and match
   `meta.json` `evidence.target_ready` before shell/app workflow surfaces mark
   `target_serve_phase_1` done or final evaluators write `evaluation.json`.
   Once target readiness is valid, shell/app advisory and final evaluators also
   require `source.pair.json.target_address` plus `source.transfer.json`
   target address / target mode / receiver endpoint to match that same
   canonical readiness artifact. Transfer readiness is stricter than pairing
   readiness: `source_transfer` and final evaluation require canonical
   `target.ready.json` to include a non-empty `receiver_address`,
   `receiver_routes=true`, `push_network=true`, and `transfer=true`.
   Shared Swift script-test process helpers now drain stdout/stderr
   concurrently, and the remaining Swift script/integration launch helpers
   route through that harness; duplicated shell/Python helper consolidation
   remains follow-up.

5. Only after app/shell proof parity is verified, move to the first real
   two-machine installed-app run.

6. For that run, use the runbook’s explicit pack/unpack/merge procedure and
   record failures as substrate work, not operator mistakes.

## Ranked Likely Failure Points

1. **App/shell proof drift**
   - Shell proof semantics have been tightened faster than some app-side
     advisory surfaces.
   - Assume any “ready” app surface could still be more optimistic than shell
     `evaluate` until re-verified.

2. **Packaging/notarization substrate mismatch**
   - Same-machine and fake-notary slices are useful only if they do not hide the
     absence of real Developer ID / notarize / staple evidence.
   - Expect stale or missing canonical `.app.notary/notarization.json` to be a
     first-order blocker.

3. **Real-device permission gating**
   - Local Network, firewall, and physical pairing confirmation prompts can
     invalidate otherwise-correct orchestration.
   - Treat these as durable evidence authoring problems, not just “manual
     checklist” items.

4. **Machine identity / handoff evidence mismatch**
   - The system now expects role ids, meta machine-facts summaries, machine
     facts artifacts, and verified handoff ledgers to agree.
   - Real device runs are likely to expose mismatches that same-machine tests
     cannot.

5. **App-side acceptance authoring assumptions**
   - The app can write bundle/operator artifacts, but real two-device runs have
     not yet proven the full authoring path under release-quality packaged apps.
   - Assume this path is a likely source of first-run truth gaps until verified.

## Current T-011 Execution State

This is the shortest honest summary of where the feature stands now.

- Feature:
  - `f-23bnwxry2`
- Current task:
  - `T-011`
- Tracker state:
  - `in_progress`
- Last gate result:
  - `partial`

### What is most likely already aligned

- Shell-side distinct-machine installed-app proof semantics:
  - `acceptance_two_machine_installed_app_proof_summary`
  - `acceptance_two_machine_compute_workflow_status`
  - `acceptance_two_machine_evaluate`
- Focused shell/app parity coverage for workflow, persisted workflow summary,
  corrective lanes, and direct shell evaluate:
  - `AcceptanceInstalledAppCollectionProofParityScriptTests`
  - `AcceptanceInstalledAppProofParityTests`
  - `AcceptanceInstalledAppWorkflowSummaryTests`
  - `AcceptanceInstalledAppLaunchCoordinatorTests`
- Swift final-gate negative coverage around wrong machine pair, missing
  cross-machine proof, and role-vs-machine-facts mismatch:
  - `AcceptanceBundleEvaluationCoordinator`
  - `AcceptanceEvaluationTests`

### What is most likely still risky

- App-side advisory and preview/preflight surfaces may still be more optimistic
  than shell `evaluate` unless you re-check them against the latest shell proof
  verdict:
  - `AcceptanceBundleLoadedSnapshot.workflowSummary(...)`
  - `AppStore.acceptanceInstalledAppLaunchPreview(...)`
  - `AppStore.acceptanceInstalledAppLaunchPreflightError(...)`
- Current app-side launch preview/preflight is now aligned with the workflow
  summary for the missing role/machine-facts correction lane: `source pair` and
  `target serve` stay available as corrective rewrites, while unrelated
  acceptance tasks block before packaging evidence writes. This is still proof
  parity only, not real two-device evidence.
- Real two-device authoring may still expose gaps in:
  - operator-evidence writing
  - app-side phase artifact writing
  - packaging/notarization artifact availability on both machines
- Release-script and local app-audit source evidence are now aligned with the
  strict sidecar release policy: supported auth mode, UUID-shaped notary
  submission id, missing/null failure, accepted notary-log JSON whose `jobId`
  matches that submission id, and distribution-ready post-staple audit. No
  fresh real Developer ID notarization/staple/Gatekeeper evidence has been
  captured for T-011.
- App/shell installed-app release evidence now also requires
  `*.app-audit.json` to report `readiness=distribution_ready`; a
  `status=pass` / `summary.pass_ready=true` audit with `readiness=review_only`
  remains a packaging-evidence blocker, not install-ready proof.

### Current known-pass baseline

- Focused shell proof tests around:
  - correct advance to `evaluate`
  - same-machine rejection
  - wrong-machine-pair rejection
  - meta/artifact machine-facts mismatch rejection
  - role/meta machine-facts disagreement rejection
  - role/artifact machine-facts disagreement rejection
  - persisted `workflow.summary.json` / `meta.evidence.workflow_summary`
    refresh for the machine-identity correction lane
- Broader focused:
  - `AcceptanceInstalledAppCollectionProofParityScriptTests`
  - `AcceptanceInstalledAppProofParityTests`
  - `AcceptanceInstalledAppWorkflowSummaryTests`
  - `AcceptanceInstalledAppLaunchCoordinatorTests`
  - selected `AcceptanceEvaluationTests`
- Archive-handoff substrate checks:
  - manifest export identity tamper and field-stripping rejection
  - staged `unpack-bundle` restore preserving an existing bundle when an
    otherwise digest-valid archive lacks the internal export identity artifact
  - direct `merge-bundle` rejection for unsafe incoming roots plus artifact,
    metadata, and `target.ready.json` conflict atomicity
  - direct `merge-bundle` publish-copy failure rollback for newly published
    artifact files before final metadata replacement
  - same-machine archive-handoff harness with source-local bundle ownership
    through browse, pair, and transfer before explicit final aggregation
- Shell syntax checks for:
  - `macos/script/lib/acceptance-two-machine.sh`
  - `macos/script/acceptance-two-machine.sh`

### Current known-open end-state gaps

- no fresh real two-Mac installed-app end-to-end acceptance bundle
- no fresh canonical Developer ID + notarize + staple + Gatekeeper success
- no fresh real Local Network / firewall / physical pairing confirmation
  evidence
- no final audited artifact chain tying release-quality packaged apps,
  distinct-machine handoff, operator evidence, non-dry-run transfer, and final
  evaluation

## Canonical Completion Artifact Set

If you think T-011 is done, you should be able to point to all of these without
hand-waving.

### Release-quality app artifacts

- canonical packaged app:
  - `macos/dist/SuperMover.app`
- packaged provenance:
  - `macos/dist/SuperMover.app/Contents/Resources/supermover-provenance.json`
- structured notary sidecar:
  - `macos/dist/SuperMover.app.notary/notarization.json`
- local audit proving distribution-ready packaging:
  - `macos/dist/SuperMover.app.audit.json` or equivalent captured audit output

### Distinct-machine acceptance bundle artifacts

- bundle `meta.json` with:
  - collection mode `two_machine`
  - machine facts
  - verified `bundle_handoffs`
  - operator evidence
- `source.machine.json`
- `target.machine.json`
- `source.app-audit.json`
- `target.app-audit.json`
- `source.notarization.json`
- `target.notarization.json`
- phase artifacts proving actual source/target procedure:
  - `target.ready.json`
  - `target.ready.phase-<n>.json`
  - `source.pair.json`
  - `exported-receipts/<pairing_receipt_id>.json` referenced by
    `source.pair.json.receipt_path`
  - `source.transfer.json`
  - any current-source / consistency artifact required by the lane
- final:
  - `evaluation.json`

`source.pair.json.receipt_path` is not advisory decoration. Shell
`workflow-status`, shell `evaluate`, app workflow summary, and Swift final
evaluate should all reopen `source_pair` / `target_import` or fail closed when
that path is missing, unsafe, symlinked, non-regular, or no longer points at the
bundle-local staged receipt. Final evaluate should also reject a stale
`source.report.json` whose `pairing.receipt_id` no longer matches
`source.pair.json.pairing_receipt_id`, and should resolve the current-source
baseline from `source.consistency.json.baseline` before falling back to
`meta.json.evidence.source_consistency.baseline`. `target_import` must also
record a non-empty `target_import.adopted` transcript path, and that transcript
must remain a bundle-local regular artifact before advisory or final evaluation
can treat target import as ready.
`target.ready.json` is likewise not decoration. It is the canonical current
target-ready artifact, while `target.ready.phase-<n>.json` preserves phase
history. Shell `workflow-status`, shell `evaluate`, app workflow summary, and
Swift final evaluate should all reopen `target_serve_phase_1` or fail closed if
canonical target readiness is missing, unsafe, malformed, or does not match
`meta.json.evidence.target_ready`.
Once canonical target readiness is valid, `source.pair.json.target_address`
and the target address / mode / receiver endpoint in `source.transfer.json`
must still point at that same target. Advisory surfaces should reopen
`source_pair` or `source_transfer`, and final evaluators should fail closed,
instead of allowing pair/transfer proof from a different target endpoint to
write or preserve `evaluation.json`. Pairing readiness alone is still not
transfer readiness: source-transfer proof also requires canonical
`target.ready.json` to show a receiver endpoint plus receiver routes, push
network support, and transfer enabled.
The advisory surfaces should not wait until final evaluate to notice those
same source-transfer proof gaps: app workflow summary and shell
`workflow-status` must reopen `source_transfer` when `source.report.json`
points at a stale pairing receipt, when the consistency artifact names a
missing/unsafe baseline, or when required transcript/baseline artifacts are
non-regular nodes. The same rule applies to target-root binding:
`source.verify.json`, `source.report.json`, `source.status.json`, and
`source.health.json` must all carry present, normalized `target_root` evidence
for the same selected/evaluated target root. Shell and Swift final evaluate must
fail closed on proof from another target root, and shell `workflow-status` must
reopen `source_transfer` after `evaluation.json` if those source-side proof
roots are later swapped to another target.
Shell and Swift final evaluate must also reject unsafe `pairing_receipt_id` or
`session_id` values before using them as target `.supermover` control-plane
path segments. That check is against the raw ID value: whitespace-bearing IDs
must fail closed instead of being trimmed into a different proof identity. The
target `.supermover/pairings/<id>.json` and
`.supermover/sessions/<id>/network-transfer.json` evidence must also be regular
non-symlink files; directories or symlinked control-plane artifacts are not
durable acceptance proof.

### Operator evidence required for real installed-app completion

- durable `local_network`
- durable `firewall`
- durable `pairing_confirmation`

### Human-readable closeout proof

- updated `docs/runbook.md`
- updated `docs/plan.md`
- updated T-011 artifact and verification docs
- a short closeout note that explicitly says:
  - what was proven on two real Macs
  - what release-grade packaging evidence was used
  - which negative branches were still fail-closed at the time of completion

## Explicit To-Do / Not-To-Do / Avoid List

### Do next

1. **Audit app-side advisory drift against the new shell proof verdict**
   - Check whether app preview/preflight/workflow summary still use weaker or
     duplicate installed-app proof logic.
   - Prefer one shared proof owner over restating conditions in multiple
     surfaces.

2. **Drive the operator procedure toward a real two-Mac run**
   - Use the existing pack/unpack/merge substrate.
   - Keep the acceptance bundle as the durable center of truth.
   - Capture packaging, notarization, operator evidence, phase artifacts, and
     final evaluate evidence from actual distinct machines.

3. **Close release-quality packaged-app evidence**
   - Produce real `macos/dist/SuperMover.app.notary/notarization.json`
   - Re-run app audit on the real release-grade app
   - Feed those artifacts into the installed-app acceptance lane

4. **Keep docs/tracker honest after every slice**
   - especially T-011 artifact, verification, runbook, and plan

### Do not do

- Do not spend the next slice on UI polish unless it is directly blocking the
  acceptance path.
- Do not claim T-011 done because same-machine or shell focused tests are green.
- Do not add another “summary” helper if it leaves callers reinterpreting its
  internals.
- Do not weaken the proof contract just to make real-device bring-up easier.

### Avoid doing by accident

- Avoid using fake-notary or same-machine success as if it were release proof.
- Avoid pushing logic into `ContentView.swift` or other UI roots when the real
  problem is acceptance substrate ownership.
- Avoid broad cleanup that mixes with T-011 truth; it will make later audit and
  review weaker.

## Suggested Next Execution Order

If starting fresh with no conversation context, do this:

1. Read:
   - `AGENTS.md`
   - `docs/must-guidebook.md`
   - `docs/must-authority.md`
   - `docs/plan.md`
   - `docs/runbook.md`
   - this handoff

2. Re-validate the latest T-011 proof slice:
   - run the focused shell proof tests
   - run shell syntax check
   - run diff hygiene if there are local edits

3. Inspect app-side acceptance surfaces against the shell proof verdict
   - identify any duplicated or weaker installed-app proof policy

4. Pick the smallest real-mainline slice:
   - either app/shell proof alignment
   - or real two-machine operator procedure execution
   - or real release-notary evidence capture

5. After each slice:
   - update T-011 artifact / verification / runbook / plan
   - keep the goal active
   - do not redefine completion around what just turned green

## Validation Baseline For This Handoff

The recent relevant evidence includes:

```bash
swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsRoleMatchedHandoffWhenMetaMachineFactsDisagree|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsWrongMachinePairHandoff|AcceptanceTwoMachineScriptTests/testTwoMachineEvaluateRejectsMachineFactArtifactMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotTreatMetaMatchedHandoffAsProofWhenMachineFactArtifactsDisagree|AcceptanceTwoMachineScriptTests/testTwoMachineWorkflowStatusDoesNotTreatRoleMatchedHandoffAsProofWhenMachineFactsDisagree'
swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsProveTheWrongMachinePair|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenBundleHandoffsDoNotProveCrossMachineTransfer|AcceptanceEvaluationTests/testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts'
swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/(testTwoMachineUnpackBundlePreservesExistingBundleWhenArchiveMissingExportIdentity|testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityIsTampered|testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityFieldsAreMissing|testTwoMachineUnpackBundleRecordsVerifiedArchiveHandoffEvidence|testTwoMachinePackAndUnpackBundleRoundTripsEvidence)'
swift test --package-path macos --filter 'AcceptanceTwoMachineScriptTests/testTwoMachinePackAndUnpackBundleRoundTripsEvidence|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRecordsVerifiedArchiveHandoffEvidence|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityIsTampered|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityFieldsAreMissing|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedOnMalformedArchive|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundlePreservesExistingBundleWhenArchiveMissingExportIdentity|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleFailsClosedOnArchiveDigestMismatch|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRejectsSymlinkedArchiveEntriesBeforePublish|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRejectsHardlinkedArchiveEntriesBeforePublish|AcceptanceTwoMachineScriptTests/testTwoMachineUnpackBundleRejectsSpecialArchiveEntriesBeforePublish'
SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 swift test --package-path macos --filter 'AcceptanceEvaluationIntegrationTests/testSameMachineHarnessSupportsArchiveHandoffWhenEnabled'
sh -n macos/script/lib/acceptance-two-machine.sh
sh -n macos/script/acceptance-two-machine.sh
git diff --check -- macos/script/lib/acceptance-two-machine.sh macos/SuperMoverAppTests/AcceptanceTwoMachineScriptTests.swift
```

These prove the recent shell proof-alignment slice, not the final product
objective.

## Completion Standard

You are not done when:

- shell proof helpers are elegant
- same-machine harness is green
- app previews look consistent
- notarization workflow exists in principle

You are done only when current evidence proves:

- two real Macs
- installed apps
- release-grade signed/notarized/stapled packaging
- discovery, pairing, mTLS transfer, operator evidence, end evaluation
- all durable artifacts present
- any missing capability still fail-closed

Until then, keep T-011 as `partial / in_progress`.
