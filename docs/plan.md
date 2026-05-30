# Implementation Plan

Supermover v1 should be built in small, reviewable slices. The implementation
order follows the local Bagakit feature plan, but this tracked document keeps
the public project direction visible without depending on ignored local
planning files.

## Current Execution Features

The original broad tracker feature has been archived as historical planning
evidence. Remaining work is split into narrower feature slices; local
feature-tracker state may mirror these IDs under `.bagakit/feature-tracker`,
but the table below is the checked-in execution summary:

| Feature | Purpose |
| --- | --- |
| `f-223nw49qj` | Migration audit report UX. `report` is implemented; compact `status` is tracked separately. |
| `f-224nw98v7` | Reviewed physical prune flow. |
| `f-225nwsa3h` | Changed-file incremental local sync. Local regular-file updates are implemented and the feature is archived; drift review UX remains separate. |
| `f-226nwy2vy` | LAN agent discovery and pairing. `serve` is wired as a low-information pairing listener and mounts authenticated receiver upload routes only for paired profiles with complete profile-selected network material; `pair` writes local receipt/profile pins after operator verification, and `discover` has low-information explicit address hints plus bounded sparse UDP LAN browse/advertise; discovery remains untrusted. Non-dry-run `push --network` now uses profile-backed pinned TLS 1.3 mTLS transfer; dry-run remains preflight-only. |
| `f-227nw2p2n` | Secure resumable transport integration. |
| `f-228nws66k` | Traffic privacy level 2 implementation. |
| `f-229nwwybc` | Failure injection and release hardening. |
| `f-22bnwggww` | Compact local `status` UX over profile/control-plane evidence. Command is wired as a read-only local current profile/target view; release docs/audit polishing remains tracked in the feature. |
| `f-22anw4myc` | Target drift review UX and `drift list` surface. `drift list` remains read-only; `drift acknowledge` is wired for existing persisted target-drift records. |
| `f-22mnwgg64` | Durable live detector drift recording. `drift record` persists current live detector findings as `.supermover/drift` review records; repair/prune/reconcile integration and background scans remain planned. |
| `f-22qnwk3b3` | Persisted drift resolve. `drift resolve` is wired for existing persisted target-drift records after a fresh detector no longer reports the same path and expected baseline; broad reconcile/repair remains planned. |
| `f-233nwduwz` | Narrow persisted drift reconcile plus read-only repair boundary review. `reconcile plan` is non-mutating for selected persisted drift evidence; `reconcile review` is non-mutating and reports persisted plan readiness, live-only record-required inputs, and planned broad repair boundaries; `reconcile apply` requires persisted-drift selection intent, explicit `--apply`, and `--reason`. Selection can be explicit `--id` values, the gated `--all-persisted-planned` mode that first reviews durable persisted evidence and selects only currently planned persisted actions, or the gated `--record-live` mode that first persists current live detector findings and then applies only resulting persisted planned actions. Apply currently handles missing regular-file restores from published/source evidence plus already-restored/absent resolve-noop only, writes durable apply receipts under `.supermover/reconcile/receipts`, and classifies refusal evidence with `conflict_class` plus `retry_advice`. Profile-backed foreground daemon drift recording can persist live detector findings as durable review evidence without applying repair. Profile-backed foreground daemon persisted reconcile apply can apply only already persisted, currently planned actions through existing reconcile receipts and stops after refusals for operator review. Remaining broad/background repair surfaces are split into `f-236nwqshz` retry policy, `f-237nwzbyq` broad scan inventory, `f-238nwybkh` manifest rewrite decisions, `f-239nwv337` repair-to-prune handoff, and `f-23anwj5su` background repair operator UX. |
| `f-236nwqshz` | Broad repair retry policy. Proposal-only follow-up for automatic/background retry over durable persisted drift records and reconcile receipts; it must not consume live-only detector IDs, broaden scans, rewrite manifests, authorize prune, or hide target conflicts. |
| `f-237nwzbyq` | Broad repair scan inventory. Proposal-only follow-up for non-mutating broad scan evidence that can feed later repair review without applying repair, rewriting manifests, or pruning target files. |
| `f-238nwybkh` | Reconcile manifest rewrite decisions. Proposal-only follow-up for explicit reviewed manifest rewrite proposal/apply/receipt flows separate from target repair. |
| `f-239nwv337` | Repair to prune handoff. Proposal-only follow-up for reviewed drift/repair-to-prune evidence linkage without granting physical prune authorization from repair evidence. |
| `f-23anwj5su` | Background repair operator UX. Proposal-only downstream UX feature for status/report/review/runbook aggregation after retry, scan, manifest rewrite, and prune handoff behavior are wired. |
| `f-232nwu2nw` | Ongoing incremental sync. Durable `sync queue` evidence, read-only per-entry queue listing, explicit operator failed-terminal queue state, one bounded local `sync run` pass, foreground local `sync loop` polling, foreground OS watcher `sync watch`, bounded profile-backed per-entry `sync network run`, foreground profile-backed per-entry `sync network loop`, one bounded LAN-discovery-gated `sync network discover-run` pass, profile-enabled foreground-daemon local polling sync, profile-enabled source-side foreground-daemon network polling sync, daemon restart acceptance for fresh local polling run receipts, durable receipt sequence recovery for local and network daemon polling, and report/status aggregation for queue/run receipts are wired; automatic LAN discovery endpoint selection remains planned. |
| `f-23bnwxry2` | Native macOS app-first LAN migration workbench. T-001 through T-010 have local implementation evidence across the app capability contract, setup/onboarding, role-scoped foreground process supervision, structured app event/artifact-reader problem surfaces, design-system stabilization, native discovery/pairing command orchestration, sync controls, typed verify evidence, Evidence Vault, packaging/readiness surfaces, and foreground daemon controls. Setup now labels the profile file as the migration config in the UI and separates role, config selection, root inputs for create/update actions, and config checks while preserving `--profile` as the CLI SSOT. Settings now separates local Display Preferences from Command Inputs; appearance and interface language defaults are UI-only, bounded app-owned chrome now includes localized sidebar, Settings/Display Preferences, and Prepare/setup role guidance, packaged apps carry the processed localization resources, and profile/config, CLI argument, command preview, evidence, and output contracts stay raw and audit-stable. T-011 is still in progress and partial: current work is tightening app/shell/evaluator proof parity and packaged-app acceptance substrate, but final release-grade closure still requires real two-machine installed-app acceptance, operator evidence, signed/notarized/stapled/Gatekeeper evidence, and final evaluation artifacts. |

Current checkpoint: `f-223nw49qj` has shipped the `report` command, and
`f-22bnwggww` wires compact local `status` as a read-only current
profile/target view. `f-225nwsa3h` implements managed
changed-file updates for local regular files by requiring previous published
manifest evidence from the same profile/target/root and rechecking target
content and metadata before publish or recovery replacement. The implemented
`report` command closes the main operator visibility gap for warnings, soft
deletes, recovery state, profile suggestions, and published-manifest
verification state at report time. It also runs the same live target drift
detector as `drift list` and reports that evidence separately from persisted
`.supermover/drift/*.json` target-drift records. The `drift list`, `report`,
and `status` live detector surfaces remain read-only over profile-selected
target state and published manifest evidence. `drift record` persists current
live detector findings as durable `.supermover/drift/<id>.json` review records
without resolving, repairing, pruning, or suppressing future detector output.
`drift resolve` can close existing persisted `.supermover/drift/<id>.json`
records after a fresh detector no longer reports the same path and expected
baseline. `reconcile plan/review/apply` now covers a narrow selected-ID
persisted drift repair surface: plan is non-mutating, review is non-mutating
and exposes live-only record-required inputs plus planned broad boundaries,
apply requires explicit operator intent, and selection is explicit IDs, the
gated `--all-persisted-planned` durable-persisted-evidence selection path, or
the gated `--record-live` live-recording path.
Repair is limited to missing regular files whose published manifest evidence
still matches the current source plus resolve-noop for already restored or
already absent persisted records. `apply` writes durable selected-ID repair receipts under
`.supermover/reconcile/receipts` and refusal evidence now has a conflict-class
taxonomy plus retry-advice review guidance. Profile-backed foreground daemon
persisted reconcile apply can run that persisted planned selection path from
profile policy and stops after refused receipts; it is mutually exclusive with
profile-backed drift recording to avoid implicit live-only-to-apply chaining.
Broad automatic reconcile,
background retry policy, background scans, background live-only repair beyond
the explicit `--record-live` gate, manifest rewrite, daemon/continuous sync
repair integration, and prune integration remain separate planned work under
`f-236nwqshz`, `f-237nwzbyq`, `f-238nwybkh`, `f-239nwv337`, and
`f-23anwj5su`.
`drift acknowledge` can add operator acknowledgement metadata only to existing
persisted `.supermover/drift/<id>.json` records, including records created by
`drift record`, surfaced as `target_drifts`.
The wired `status` slice is `supermover status --profile <path>
[--format text|json]` only, with no initial `--session` flag. It is a
read-only current profile/target view over profile SSOT, target `.supermover`
artifacts, and target files needed for verification/live drift detection;
`report --session` remains the historical report surface. It does not include
foreground daemon lifecycle state and optional profile-enabled local or
source-side network polling sync receipts; it does not imply LAN, encrypted
transport, detached daemon service, daemon-integrated watcher,
automatic LAN discovery endpoint selection, broad repair, or broad resumable
network recovery. Use `sync
queue`/`sync run`/`sync loop`/`sync watch` for the local incremental queue
surface and `sync network run`/`sync network loop`/`sync network discover-run`
for the profile-backed network queue passes.
The wired `daemon` slice is foreground-only: `daemon install`, `daemon run
--foreground`, `daemon status`, `daemon logs`, `daemon restart`, and
`daemon stop` persist `.supermover/daemon` install/state/stop-intent/
restart-intent evidence and redacted lifecycle events around existing
profile-backed `serve` behavior, plus profile-enabled local polling and
source-side network polling modes when configured. Restart is a foreground
intent consumed by a running daemon process and restarts serve listeners or
polling workers in that same process. OS service-manager installation,
detached background process management, crash restart supervision,
daemon-integrated file watching, automatic LAN discovery endpoint selection,
and automatic endpoint selection remain future work.

Current feature dependency shape:

- `f-229nwwybc` hardens the current local slice and records separate future
  gates for network features.
- `f-226nwy2vy` first wires `serve`, `discover`, and `pair` command surfaces
  without trusting discovered endpoints. Current `discover` supports explicit
  address hints and bounded sparse UDP LAN browse/advertise. Non-dry-run
  `push --network` now
  transfers through the paired profile-backed mTLS receiver; dry-run does not
  contact the receiver or write target artifacts. Same-session `push --network`
  reruns can resume from receiver status offsets for compatible partial
  receiver sessions only when prior payload-overhead evidence remains
  auditable, and published-session reruns can retry commit without reuploading
  chunks while preserving prior published proof; broad process-kill/interruption
  recovery remains a separate acceptance gate.
- `f-23bnwxry2` is the active native macOS app-first workbench feature. The
  current app can be built and used as a CLI-backed operator workbench for
  profile setup, including a Prepare page that labels the profile SSOT as the
  migration config and separates role choice, config selection, root inputs for
  create/update actions, and lint/status checks, plus local UI-only Display
  Preferences for app appearance and interface-language selection. Bounded
  app-owned chrome now localizes sidebar, Settings/Display Preferences, and
  Prepare/setup role guidance through packaged SwiftPM resources. Connect,
  Move, and Verify/Repair owner-page toolbars are fixed outside page-body
  scrolling, all page hosts share a tighter top inset with sidebar navigation
  scrolling independently under constrained window heights, while task
  titles, command previews, profile/config, CLI arguments, evidence/proof
  values, artifact fields, and CLI output stay raw and audit-stable. The app
  also has native discovery/pairing command orchestration, selected wired
  commands, sync queue/run/loop/watch/network controls, role-scoped
  foreground processes, structured app events, visible artifact decode
  problems, typed target-vs-published-manifest verification evidence with
  Merkle/root proof explicitly unavailable, a native Evidence Vault, CLI
  provenance/readiness display, and foreground daemon controls. The packaging
  script now bundles the CLI with a provenance manifest and optional code
  signing inputs, but final notarized distribution and two-machine acceptance
  evidence remain planned under T-011. Vault-side target mutation, transfer,
  trust/pairing, publish, and Merkle proof are still intentionally excluded or
  unavailable. Current same-machine acceptance now records a real
  `source.consistency.json` proof from the CLI baseline/compare path, while
  distinct-machine installed-app acceptance still fails closed on missing
  operator/release/proof gates. Shell `workflow-status` and
  `evaluate --require-operator-evidence` now share one installed-app proof
  verdict surface for collection mode, role machine ids,
  `evidence.machine_facts.*`, machine-facts artifacts, and verified
  cross-machine bundle handoff evidence. Strict operator evidence now also
  requires `pass`, non-empty detail, and a `machine_id` bound to the canonical
  source/target machine-facts artifact for that manual check, so unbound or
  wrong-machine Local Network/firewall/pairing-confirmation records no longer
  advance app/shell advisory or final evaluate. That shared shell proof summary now
  also publishes blocked reason, missing requirements, machine-identity vs
  bundle-handoff remediation flags, and final-evaluate detail precedence, so
  persisted `workflow.summary.json`, shell advisory, and direct evaluate all
  fail closed with the same concrete reasons for same-role-machine-id,
  role-vs-machine-facts, and contradictory handoff lanes. App-side
  installed-app launch preview /
  preflight, two-machine workflow-summary reuse, and Swift final evaluate now consume the same
  Swift-side proof family, so release-ready packaging evidence alone no longer
  renders `pass` and stale shell-authored two-machine workflow summaries are
  ignored unless current bundle status, proof fields, release evidence, steps,
  and next actions still match. Top-level shell `workflow-status` `ok=false`
  now also stays false until `evaluate` writes `evidence_collected`, even when
  `installed_app_proof_ok=true`, so the coordination JSON no longer reads
  greener than the final gate while release, operator, or pending-evaluate
  work remains. Contradictory verified `bundle_handoffs` now
  also keep shell `workflow-status` in the same `review_bundle_handoff` lane
  the app already used, instead of asking operators to rerun a generic
  `bundle_handoff` step against an already-conflicted merged bundle. Local
  strict advisory now also picks installed-app correction, release packaging
  evidence, and `evaluate` before it falls back to legacy phase reminders, and
  app/shell workflow summary code now consults canonical bundle
  `source_pair` / `source_transfer` artifacts even when older metadata omitted
  those explicit subpaths. App-side acceptance authoring now also stages the
  durable local pairing receipt into bundle-local
  `exported-receipts/<pairing_receipt_id>.json`, records
  `source.pair.json.receipt_path` against that bundle-relative path, and writes
  current `target.ready.json` plus `meta.json` `evidence.target_ready` so
  app-authored bundles stay replayable through shell `source-pair`,
  `merge-bundle`, and `target-import` semantics. Existing app-authored phase
  output leaves for machine facts, discovery, target-ready, exported receipts,
  source-pair/source-transfer, transcripts, source consistency, and evaluation
  now must also be single-link regular files before the app overwrites them, so
  a hardlinked phase artifact cannot leave partial machine-facts or meta
  evidence behind. Target-import metadata must now record a non-empty
  `target_import.adopted` transcript path such as
  `target.adopt-pairing.txt`, and shell/app advisory plus final evaluation
  require that transcript to remain a bundle-local regular artifact instead
  of treating the `target_import` object or pairing receipt id as proof. A bare
  `meta.status=evidence_collected` also no
  longer counts as current completion by itself: app/shell advisory now
  requires a current bundle-local `evaluation.json`, and the strict lane
  reopens `evaluate` when that preserved evaluation artifact was written
  without operator-evidence enforcement. Installed-app launch preview now
  likewise stays in `review` until that current strict evaluation both exists
  and still matches the current phase/operator proof inputs; preview/preflight
  only allow the matching corrective launch when that reopened step is the sole
  remaining strict next action. If multiple required steps reopen, or the
  reopened step is different from the requested launch, preview/preflight stay
  blocked instead of continuing to trust the stale `evaluation.json`. Even
  after release evidence and distinct-machine installed-app proof are ready,
  shell final evaluate, shell workflow-status summaries, shell bundle
  bootstrap, and Swift bundle access now also reject hardlinked bundle-local
  proof artifacts, linked/non-regular `meta.json`, malformed exported
  pairing receipts, unsafe archive-ingress entries, and malformed
  `source.status.json` / `source.health.json` transfer evidence instead of
  treating file presence as durable proof; shell final evaluate,
  shell `workflow-status`, Swift app workflow summary, and Swift final
  evaluation now all require their readiness counters to be non-negative
  Swift `Int`-decodable integers before advancing `source_transfer` or writing
  final evaluation evidence. They also require `source.verify.json`,
  `source.report.json`, `source.status.json`, and `source.health.json` to carry
  present, normalized `target_root` evidence for the selected/evaluated target
  root; after `evaluation.json` exists, shell `workflow-status` reopens
  `source_transfer` if those source-side proof roots are swapped to a different
  target. The target-serve phase now
  also comes from canonical `target.ready.json` plus matching
  `meta.json.evidence.target_ready`, so workflow advisory and final evaluate
  fail closed on missing or malformed target-ready artifacts instead of
  trusting meta text alone. App/shell workflow advisory and final evaluation
  now also require `source.pair.json.target_address` and
  `source.transfer.json` target address / mode / receiver endpoint to match
  that current target-ready artifact before pair or transfer evidence can
  advance to `evaluate`. Transfer readiness is stricter than pairing readiness:
  a valid pairing `target.ready.json` can keep `target_serve_phase_1` done, but
  `source_transfer` and final evaluation also require that same artifact to
  prove receiver transfer readiness with a non-empty `receiver_address`,
  `receiver_routes=true`, `push_network=true`, and `transfer=true`. Shell
  `workflow-status` and shell final evaluate
  also decode optional `source.browse.json` / `target.advertise.json`
  discovery artifacts against the same required shape Swift uses before
  treating those steps as complete. Task
  Dispatch is a standalone owner page and its run CTA uses the same
  `taskRunGate` as `runSelectedTask`, including blocking `Profile Init` unless
  the selected profile path is a new destination. These changes tighten local
  proof/advisory parity and profile UX; they do not close real two-Mac
  installed-app, operator-evidence, or Developer ID notarization gates. Local
  sibling notarization sidecars are also
  rejected unless their referenced post-staple audit still matches the current
  packaged app provenance, and canonical sibling sidecar / post-staple audit
  symlinks now fail closed instead of laundering external proof into a
  bundle-local path shape. Missing or not-release-ready local sibling
  notarization evidence for the current packaged app now also blocks the
  installed-app launch advisory, so app preview no longer stays more optimistic
  than shell phase preflight on that lane. When the shared proof owner instead
  says the bundle needs machine-identity correction, including missing
  role/machine-facts evidence as well as conflicting machine identity evidence,
  app-side `source pair` and `target serve` remain launchable as corrective
  rewrites while unrelated acceptance tasks stay blocked before packaging
  evidence writes. Those corrective app launches now also rewrite
  canonical `source.machine.json` / `target.machine.json`,
  `evidence.machine_facts.*`, and `roles.source_pair` / `roles.target` from
  the current installed app, so the bundle can move from a stale
  machine-identity blocker to the remaining handoff / operator / release gates
  instead of leaving correction as a preview-only promise. That currentness
  rule now also reaches the local
  release-engineering gate: `macos/script/audit-app.sh` blocks on stale
  sibling sidecars when they exist and only marks existing sidecars
  release-ready when they retain the same strict notarization fields required
  by installed-app acceptance: supported `auth_mode`, UUID-shaped Apple
  submission id, `failure` absent/null, accepted notary-log JSON, and a
  distribution-ready post-staple audit. `macos/script/notarize-app.sh` clears
  any previous sibling result before its post-staple audit so a rerun can
  replace stale evidence with fresh output while also persisting the referenced
  post-staple audit into the sibling sidecar directory. The bundled packaged-app
  audit helper now shares that same existing-sidecar currentness/release-ready
  rule without treating a missing sidecar as an automatic app-audit failure, and
  it now also refuses to mark a decodable-but-invalid sidecar as current or
  release-ready.
  App-side packaging collection likewise checks the sibling sidecar's top-level
  `app_path` and requires the sidecar's `audit.path` to stay anchored to the
  canonical sibling `.app.notary/post-staple.audit.json` in addition to the
  referenced post-staple audit/provenance before copying installed-app
  notarization evidence. It now also rejects symlinked canonical sidecar and
  post-staple audit leaves, so installed-app packaging collection no longer
  diverges from shell notarization bootstrap on that bundle-local proof subset.
  The app-side installed-app
  release-evidence owner now also reads the canonical bundle-local
  `source|target.provenance.json`,
  `source|target.app-audit.json`, and `source|target.notarization.json` files
  directly, so stale copied notarization artifacts no longer pass preview,
  workflow, or final evaluate on status bits alone. App-side packaging
  collection plus shell `record-packaging-evidence`, `workflow-status`, and
  `evaluate --require-operator-evidence` now apply the same currentness rule:
  stale sibling sidecars and sibling notary logs that do not decode as accepted
  notary log JSON bound by `jobId` to the sidecar `submission.id` are rejected
  during collection, and stale copied
  notarization / notary-log leaves are removed even when the stale leaf is a
  dangling symlink rather than a regular file. Existing bundle-local
  packaging output leaves for version, provenance, app-audit, notarization,
  and notary-log evidence must also be single-link regular files before shell
  or app-side packaging collection overwrites them.
  Bundle-local
  `*.app-audit.json`, `*.provenance.json`, and
  `*.notarization.json` must agree, with each referenced bundle-local
  `*.notary-log.json` decoding as accepted notary log JSON whose `jobId`
  matches that notarization artifact's UUID-shaped `submission.id`, each
  app-audit artifact reporting `readiness=distribution_ready` in addition to
  `status=pass` and `summary.pass_ready=true`; review-only app audits remain
  packaging-evidence blockers instead of install-ready proof, and each
  notarization artifact retaining a supported `auth_mode` from the first-class
  notarization script, a UUID-shaped Apple notary submission id, and `failure`
  absent/null, before installed-app release evidence can advance to
  `evaluate`. The first-class `macos/script/notarize-app.sh` source workflow
  also fails closed on missing or non-UUID submit ids before fetching notary
  logs, and on non-accepted or submission-mismatched notary-log JSON before
  stapling. Accepted log JSON means `status=Accepted`, `issues` absent/null or
  array, and `jobId` equal to the submitted UUID, so the source workflow cannot
  create a local `status=pass` sidecar that the later installed-app gates would
  reject. Swift final
  evaluate now also resolves bundle-local artifacts through one guarded
  bundle-artifact access seam shared with the bundle reader, and it rereads
  `source.consistency.json` strictly instead of trusting merged meta fallback.
  Distinct-machine evaluation now fails closed
  on bundle-local `..` path escapes, symlinked artifacts, and malformed
  current-source proof artifacts instead of accepting raw rereads from
  `Data(contentsOf:)`. That same seam now also rejects symlinked acceptance
  bundle roots and symlinked `meta.json` on read, and app-side artifact /
  packaging writers validate both the bundle root and `meta.json` before any
  file writes, so symlinked bundle paths no longer allow partial bundle
  mutation before later metadata rejection. The shell archive handoff substrate
  now stages `unpack-bundle` restores and direct `merge-bundle` publishes before
  making them visible at the requested bundle root. Malformed archives, archives
  missing the internal export identity artifact, incoming merge roots containing
  symlinks, hardlinks, or special files, and preflightable artifact /
  `meta.json` / `target.ready.json` merge conflicts now fail closed without
  wiping an existing incoming bundle or leaving novel half-merged artifacts in
  the destination. Direct `merge-bundle` also rolls back newly published artifact
  files and empty directories when a publish-time artifact copy fails before the
  final `meta.json` replacement. The same-machine archive-handoff harness keeps
  source phases on the source-local bundle before explicit pack/unpack/merge
  into the final aggregate mirror.
  This is still substrate hardening rather than real two-device acceptance
  completion.
- `f-22znw2utc` wires a foreground daemon lifecycle acceptance slice around the
  existing `serve` behavior. It persists install/status/log/restart/stop
  evidence under the target control plane and requires an external supervisor
  for long-running background process management.
- `f-22anw4myc` drift review starts with read-only `drift list`, live `report`
  evidence, compact `status` evidence, and persisted-only `drift acknowledge`.
- `f-22mnwgg64` adds `drift record` as durable live-detector evidence
  persistence only; prune/reconcile/repair workflows and background scans
  remain future work.
- `f-22qnwk3b3` adds persisted-record `drift resolve` after a fresh detector
  no longer reports drift for the same path and expected baseline; broad
  automatic reconcile/repair and drift-to-prune integration remain future work.
- `f-233nwduwz` adds a narrow persisted-record `reconcile plan/review/apply`
  CLI surface. It consumes profile-scoped persisted drift IDs only; there is no
  `--target` or `--state-dir` override. `review` is read-only and exposes
  live-only findings as record-required before apply. `apply` can select
  explicit IDs, use `--all-persisted-planned` to select only planned persisted
  actions after review, or use `--record-live` to persist current live detector
  findings and then apply only resulting persisted planned actions. It writes durable
  receipt evidence under `.supermover/reconcile/receipts` and classifies
  refusals with `conflict_class` plus `retry_advice`. Profile-backed foreground
  daemon persisted reconcile apply consumes only persisted planned actions and
  stops after refused receipts for operator review. Broad automatic
  reconcile, background retry policy, broad background repair scans,
  background live-only repair beyond explicit live-recording gates, manifest
  rewrite, broad daemon repair retry/background policy, and
  drift-to-prune integration remain future work split into `f-236nwqshz`,
  `f-237nwzbyq`, `f-238nwybkh`, `f-239nwv337`, and `f-23anwj5su`.
- `f-236nwqshz`, `f-237nwzbyq`, `f-238nwybkh`, `f-239nwv337`, and
  `f-23anwj5su` are proposal-only follow-ups for retry policy, broad repair
  scan inventory, manifest rewrite decisions, repair-to-prune handoff, and
  background repair operator UX. They are not implemented behavior today.
- `f-227nw2p2n` secure resumable transport depends on `f-226nwy2vy` pairing.
- `f-228nws66k` traffic privacy level 2 depends on `f-227nw2p2n` secure
  transport.

## Phase 1: Project Skeleton

- Go module and `cmd/supermover` entrypoint.
- Basic CLI help and version command.
- Architecture, profile, threat-model, and planning docs.
- Baseline `go test ./...` validation.

## Phase 2: Profile And Control Plane

- Profile schema as the configuration SSOT.
- `.supermover` control-plane schemas for profile snapshots, receipts,
  manifests, warnings, history, soft deletes, target drift, and recovery.
- Deterministic JSON encoding and validation.
- Profile lint and doctor checks.

## Phase 3: Scan, Audit, And Agent Knowledge

- Filesystem scanning for ordinary file-tree fidelity.
- Structured audit records for unsupported metadata and risky filesystem
  cases.
- Agent influence manifest for rule and memory files such as `AGENTS.md`,
  `CLAUDE.md`, `.cursor/rules/**`, and `.github/instructions/**`.

## Phase 4: Local Durability

- Target-side staging and atomic promotion.
- Session journals and recovery scanning.
- Read-only `health` diagnostics for interrupted or invalid sessions.
- Danger-pause rules for missing roots, root fingerprint changes, target drift,
  mass permission loss, and major policy changes.
- Soft-delete review and prune dry-run.

## Phase 5: Secure Transport

- Low-information discovery currently supports explicit-address hints and
  bounded sparse UDP LAN browse/advertise. Discovery is not trust.
- Verification-code pairing with persistent pinned device identity is wired;
  identity generation, rotation, revocation, and broader lifecycle UX remain
  planned.
- Profile-backed pinned TLS 1.3 mTLS is wired for current `serve` and
  non-dry-run `push --network`; foreground daemon lifecycle state is wired
  around `serve`; OS service-manager and automatic discovery-selected endpoint
  management remain planned.
- Privacy level 2 padding, batching, bounded timing jitter, and clear limits
  are wired for the profile-backed network path. This is not an anonymity
  claim.

## Phase 6: End-To-End Sync

- Managed local changed-file updates, bounded receiver-status resume, and
  deterministic `networkrun` source-stop-after-progress resume evidence are
  wired. The bounded network recovery acceptance matrix also covers receiver
  listener restart over preserved target state, commit-only retry,
  published-session retry, and fail-closed missing-prior-evidence behavior.
  Ongoing sync, network `recover`, broad resume acceptance, daemon restart
  recovery, power-loss recovery, and arbitrary process-kill recovery remain
  planned.
- `live`, `strict`, and `snapshot` consistency behavior remains planned beyond
  the current strict/profile-backed slices.
- Review commands are partly wired for deletes, live/persisted target drift,
  durable drift recording, persisted-record resolve, narrow selected-ID
  persisted drift reconcile, status, and report; broad automatic
  reconcile/repair, background scans, and prune integration remain planned.
- Recover and prune commands are partly wired: local recovery, `prune approve`
  approval-artifact authoring, `prune approvals`, focused read-only
  `prune review`, `prune supersede`, and reviewed `prune --apply --approval
  <id>` exist, while broader network recovery remains planned.

## Phase 7: Quality Bar

- Failure-oriented integration tests.
- Security and recovery documentation.
- CI, contribution guide, security policy, and release process.
