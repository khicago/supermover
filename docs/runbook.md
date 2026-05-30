# Operations Runbook

This runbook is the operator procedure for the current Supermover checkpoint.
Use it to run the wired local and bounded network paths, capture evidence, and
avoid turning planned product surface into accidental release claims.

The document assumes three boundaries throughout:

- `report`, `status`, and `dashboard` are read-only review surfaces
- `daemon` is foreground lifecycle evidence around `serve`, not detached service
  management; when `sync.network_polling` is enabled it becomes a source-side
  worker-only foreground network queue daemon instead of serving routes
- `push --network` is a bounded profile-backed transfer path, not discovery,
  continuous sync, or broad recovery
- `sync run` is a bounded local queue consumer, `sync loop` is a foreground
  polling queue consumer, and `sync watch` is a foreground OS watcher feeding
  that same local queue consumer. `daemon run --foreground` can run local
  polling only when the profile enables `sync.local_polling`. `sync network
  run` is one bounded queue consumer over per-entry profile-backed mTLS
  network publication, `sync network discover-run` adds a bounded LAN-discovery
  gate that must match the profile receiver address before the same pinned mTLS
  path runs, and `sync network loop` repeats that same network queue pass in
  the foreground with generated session IDs.
  `daemon run --foreground` can run that same network queue pass only when the
  profile enables `sync.network_polling`. None of these are detached services,
  automatic endpoint selectors, broad repair, or OS file watcher daemons.
- The native macOS app is currently a CLI-backed workbench. It can help with
  profile setup, selected wired commands, foreground process supervision, and
  structured evidence/problem display, but it is not yet the release gate for a
  complete two-machine app-first LAN migration. The Prepare page calls the
  profile SSOT a migration config for operators, then separates role, config,
  root inputs for create/update actions, and setup checks without changing CLI
  `--profile` semantics. Source-side app profile creation defaults to the
  recommended `~/.supermover/profile-local.json` destination; custom profile
  file destinations are an Advanced action and must remain an explicit operator
  choice. Settings Display Preferences are local UI-only defaults for the
  app window; they do not modify migration config files, CLI arguments, command
  previews, evidence bundles, or CLI output. The language switch localizes
  bounded app-owned chrome such as the sidebar, Settings/Display Preferences
  labels, and Prepare/setup role guidance; task titles, command previews,
  evidence/proof values, artifact fields, and CLI output remain raw and
  audit-stable.

## App-First Acceptance Matrix

Use this matrix before claiming the macOS app is ready for a large
cross-device synchronization run. Keep loopback, packaged-app, and two-machine
evidence separate.

| Priority | Area | Positive acceptance | Negative acceptance | Evidence |
| --- | --- | --- | --- | --- |
| P0 | Packaged app provenance and audit | Packaged app includes an executable bundled CLI, complete internally consistent provenance, matching current git state, basic file hashes, Developer ID signing, Gatekeeper assessment, and stapler validation when claiming distribution readiness. | Missing CLI, malformed/incomplete/stale provenance, unsigned/ad-hoc/dirty/not-stapled app state must not render as green install readiness. | `macos/script/build-app.sh`, `macos/script/audit-app.sh`, `supermover-provenance.json`, bundled CLI `version`, app Settings Install Readiness. |
| P0 | Loopback local migration | Bundled CLI can create a profile, migrate a dot-prefixed file, dot-directory payload, zero-byte file, and a larger regular file, then verify/report/status/health clean. | Unsupported source/target state must not be hidden behind app green state. | `macos/script/acceptance-loopback.sh` summary plus generated JSON artifacts. |
| P0 | Sync workbench | Bundled CLI can enqueue changed files and run a bounded local sync pass with durable queue/run receipts. | Queue mutation commands require explicit durable ids/reasons where applicable. | `sync-queue-*.json`, `sync-run.json`, target file checks. |
| P0 | Network fail-closed baseline | Unpaired `push --network --dry-run` fails before transfer or target mutation. | Unpaired, missing network material, or wrong trust material must fail closed without changing target files or control-plane evidence. | `network-unpaired.err`, exit code, target before/after hash diff. |
| P0 | Two-machine LAN | Source and target installed apps can run target `serve`, source discovery/pairing, profile-backed dry-run, non-dry-run `push --network`, and post-run verify/status/report. | Wrong verification code, duplicate/ambiguous discovery, Local Network/firewall denial, and stale/missing evidence must not produce green state. Stale bundled CLI command surfaces must fail closed before phase execution. | Target serve ready JSON/stderr, pairing receipt/profile pins, `network-transfer.json`, verify/status/report JSON, per-machine `source.app-audit.json` / `target.app-audit.json`, app screenshots/logs. |
| P0 | Evidence Vault/app UI | App displays current structured evidence, raw JSON, artifact problems, and unavailable Merkle/current-source proof truthfully. | Stale stdout, missing evidence, malformed JSON, symlinked artifacts, and unsupported Merkle/current-source proof never pass. | App Evidence Vault state and `.supermover` artifact catalog. |
| P1 | Interruption/resume | Bounded same-session rerun resumes from receiver status only when prior accepted-payload evidence exists. | Do not claim arbitrary process-kill, power-loss, network `recover`, or daemon crash recovery. | Existing network recovery tests plus preserved `network-transfer.json`. |
| P1 | macOS permissions | File access, Local Network, firewall/listen, signing/notarization, and bundled provenance failures are actionable before the run. | Local ad-hoc, unsigned, or dirty builds are not distributed install readiness. | Settings Install Readiness, System Settings/firewall notes, codesign/notary evidence. |

### Loopback App-Bundled Acceptance

Run this on a single machine after building the app:

```bash
macos/script/acceptance-loopback.sh
```

To reuse an already built `macos/dist/SuperMover.app`:

```bash
SUPERMOVER_ACCEPTANCE_SKIP_BUILD=1 macos/script/acceptance-loopback.sh
```

The script uses the bundled CLI at
`macos/dist/SuperMover.app/Contents/Resources/bin/supermover`, not `go run`.
It writes a disposable profile/source/target under the system temp directory,
then preserves its artifacts there and prints the path on success or failure. It
proves:

- packaged CLI version and internally consistent provenance can be inspected;
  stale packaged CLI command surfaces required by acceptance fail closed before
  execution with explicit rebuild-required evidence;
  `macos/script/audit-app.sh` records local signing/Gatekeeper/stapler evidence;
  unsigned, ad-hoc, dirty, or unstapled local app state is recorded as
  review-only/blocking evidence, not distribution readiness
- profile init/lint works through the bundled CLI
- local push migrates a dot-prefixed file, dot-directory payload, zero-byte
  file, and a larger regular file
- verify/status/report/health report clean target evidence
- sync queue enqueue/list/ready plus bounded `sync run` publish changed-file
  evidence
- unpaired `push --network --dry-run` fails closed without changing target
  files or control-plane evidence
- packaged profile-backed pairing plus non-dry-run `push --network` can complete
  a same-machine localhost mTLS transfer and persist receiver-side
  `network-transfer.json` evidence through the bundled CLI path

It does not prove two-machine LAN pairing, Local Network prompt approval,
firewall readiness, two-machine installed-app mTLS transfer, signing/notarization,
detached daemon behavior, or Merkle/current-source proof.

### Manual Real-Device Evidence

For real two-machine installed-app runs, Local Network/firewall prompts and
physical pairing-code confirmation remain operator evidence, not automated
proof. Record them into the shared acceptance bundle through either surface:

- shell harness:
  `sh macos/script/acceptance-two-machine.sh record-operator-evidence --bundle-root <bundle> --kind local_network --status pass --detail 'accepted prompt on target'`
- native app:
  Evidence Vault -> Acceptance Bundle -> Manual Evidence

Both paths write the same `meta.json` contract under `.evidence.operator.*`
using the bundle `.meta.lock`. `evaluate --require-operator-evidence` remains
the fail-closed gate that requires each of these records to have `status:
pass`, non-empty `detail`, and a `machine_id` bound to the canonical installed
app machine-facts artifact for the relevant role:

- `local_network`: `target.machine.json`
- `firewall`: `target.machine.json`
- `pairing_confirmation`: `source.machine.json`

The shell `record-operator-evidence` command derives that binding from
`source.machine.json` / `target.machine.json` and refuses `pass` records for
these strict kinds until the required canonical machine-facts artifact exists.
The app uses the currently loaded bundle snapshot for the same binding and
keeps `pass` manual evidence blocked until machine facts are present.

App-side manual evidence recording is a durable bundle authoring surface only.
It does not turn the run green without the rest of the installed-app evidence.

For shell-driven distinct-machine collection, the bundle no longer needs to
live on a shared filesystem. The current shell substrate supports this explicit
handoff flow:

1. collect local phase evidence into a machine-local bundle root
2. archive that bundle for handoff:
   `sh macos/script/acceptance-two-machine.sh pack-bundle --bundle-root <local-bundle> --archive <bundle.tgz>`
3. move the archive to the other Mac by any operator-approved channel
4. restore it there:
   `sh macos/script/acceptance-two-machine.sh unpack-bundle --archive <bundle.tgz> --manifest <bundle.manifest.json> --bundle-root <incoming-bundle>`
5. merge it into the receiving machine's current bundle:
   `sh macos/script/acceptance-two-machine.sh merge-bundle --bundle-root <current-bundle> --incoming-bundle-root <incoming-bundle>`

To inspect what the current bundle still expects from source/target before the
next handoff or final evaluate, use:

- `sh macos/script/acceptance-two-machine.sh workflow-status --bundle-root <bundle>`
- `sh macos/script/acceptance-two-machine.sh workflow-status --bundle-root <bundle> --require-operator-evidence`

This is a bundle handoff substrate only. It preserves durable artifacts such
as `exported-receipts/<id>.json`, `source.pair.json`, `target.ready.json`,
`target.ready.phase-<n>.json`, `source.transfer.json`, and
packaging/notarization evidence without requiring the two devices to share a
writable bundle path.

`pack-bundle` also writes a sidecar checksum manifest with the exporting
machine id/label already recorded in the bundle. `unpack-bundle` verifies the
archive digest, rejects unsafe archive entries and unpacked staging entries
such as symlinks, hardlinks, and special files, requires the manifest export
identity to match the archive's internal export-identity artifact, records the
importing machine id/label into the verified handoff ledger in `meta.json`, and
only then publishes the restored bundle root. A malformed archive, missing
export identity, or unsafe archive tree fails closed from a staging directory
instead of deleting an existing incoming bundle. `merge-bundle` also validates
the incoming bundle root before parsing incoming metadata, rejects symlinks,
hardlinks, and special files, and stages publishable artifacts before replacing
`meta.json`; preflightable artifact, metadata, or `target.ready.json` conflicts
therefore fail without leaving novel half-merged files in the destination. If a
publish-time artifact copy fails before the final `meta.json` replacement,
`merge-bundle` rolls back newly published artifact files and empty directories.
This is integrity evidence for operator-controlled archive handoff, not proof
of real-device LAN behavior by itself.

The verified archive/manifests used during handoff are also retained in bundle
`meta.json` under `.evidence.bundle_handoffs`, so the final merged bundle can
show which archive crossings were verified along the way and which exporting
machine handed evidence to which importing machine. In the
`--require-operator-evidence` lane, evaluation now fails closed unless at least
one verified handoff proves a cross-machine export/import pairing between the
same source/target machine ids already recorded in the bundle; same-machine or
wrong-machine replays do not satisfy distinct-machine acceptance.

`workflow-status` is a read-only coordination surface. It does not execute
phases or mark the run complete; it summarizes which evidence-backed steps are
already present in the bundle and which machine-local action should happen
next, including shell command hints for that next step, so the distinct-machine
operator flow does not have to be reconstructed from README text alone. It now
derives installed-app readiness from the same proof verdict that
`evaluate --require-operator-evidence` uses, instead of from a weaker handoff
summary, so advisory and final-gate surfaces stay aligned on:

- collection mode and machine count
- role machine ids
- `evidence.machine_facts.*`
- `source.machine.json` / `target.machine.json`
- verified cross-machine `bundle_handoffs`

That shared shell proof summary now also persists
`installed_app_blocked_reason`, `missing_installed_app_requirements`,
`requires_machine_identity_correction`, `requires_bundle_handoff_proof`, and
the final-evaluate detail fields into both `workflow.summary.json` and
`meta.json` `.evidence.workflow_summary`. Direct shell
`evaluate --require-operator-evidence` now reports those details in the same
precedence the Swift evaluator uses: collection detail first, then
machine-facts detail, then bundle-handoff detail. Same-role-machine-id and
role-vs-machine-facts conflicts therefore fail with the same concrete text in
shell advisory, persisted workflow summary, and final evaluate.
`installed_app_proof_ok` remains the proof-specific readiness bit; the
top-level `workflow-status` `ok` bit now stays false until the bundle is
already `evidence_collected` and no further next action remains, so advisory
JSON does not turn green before final evaluate has actually run.

With `--require-operator-evidence`, `workflow-status` now orders that advisory
surface as release packaging evidence first, then installed-app collection /
handoff correction, then final `evaluate` before it falls back to legacy phase
reminders. Those older phase actions only
reappear after the strict proof/release gates are already satisfied or the
bundle is already `evidence_collected`.

The app can also write current phase artifacts into the same bundle for:

- `source.browse.json`
- `target.advertise.json`
- `target.ready.json`
- `target.ready.phase-<n>.json`
- `source.pair.json`
- `source.transfer.json`

For app-authored acceptance bundles, `target.ready.json` is the current
target-ready surface and `target.ready.phase-<n>.json` preserves the individual
serve phases. App-side `source pair` now also stages the validated durable
local pairing receipt into `exported-receipts/<pairing_receipt_id>.json` and
records `source.pair.json.receipt_path` against that bundle-relative path, so
merged bundles remain replayable through shell `target-import`. Before the app
publishes fresh phase evidence, existing bundle-local output leaves for machine
facts, discovery, target-ready, exported receipts, source-pair/source-transfer,
phase transcripts, source consistency, and evaluation must be single-link
regular files; unsafe leaves fail closed before partial phase artifacts or meta
updates are written. Shell
`workflow-status`, shell `evaluate`, app workflow summary, and Swift final
evaluate now all fail closed unless that recorded receipt path still resolves
to a bundle-local single-link regular file with a valid pairing-receipt schema,
so advisory no longer advances past `source_pair` / `target_import` on a stale,
unsafe, symlinked, hardlinked, missing, malformed, or non-regular receipt
artifact. `target_import` must also record a non-empty
`target_import.adopted` transcript path such as `target.adopt-pairing.txt`,
and that referenced path must remain a bundle-local single-link regular
artifact before shell/app advisory or final evaluation treats target import as
ready. Final evaluation also requires
`source.report.json` to report the same `pairing.receipt_id` as
`source.pair.json`, and it resolves the current-source baseline from
`source.consistency.json.baseline` before falling back to
`meta.json.evidence.source_consistency.baseline`. The app and shell workflow
advisory surfaces now use those same source-transfer checks before they advance
to `evaluate`: stale `source.report.json` pairing receipts, artifact-side
baseline drift, and non-regular source-transfer transcript/baseline files
reopen `source_transfer` instead of leaving advisory greener than the final
gate. This keeps shell and Swift final gates aligned on replayable pairing and
source-consistency evidence. Final evaluate also validates the raw
`pairing_receipt_id` and `session_id` values as single safe target control-plane
path segments before reading target `.supermover/pairings/<id>.json` or
`.supermover/sessions/<id>/network-transfer.json`; IDs with whitespace, `.`,
`..`, `~` prefixes, `/`, or `\` are rejected instead of being trimmed into a
different proof identity. The target control-plane pairing receipt and
network-transfer artifacts must also be regular non-symlink files. This prevents
bundle data from steering evaluation through traversal-like or non-regular
control-plane paths. This is bundle-authoring and source-transfer proof parity
only; it is not real two-machine installed-app acceptance proof.

The shell final evaluator now applies the same single-link regular-file
policy to required bundle-local proof artifacts before reading provenance, app
audit, source-pair/source-transfer evidence, verify/status/report/health,
source consistency, current-source baseline, machine facts, notarization, and
optional discovery artifacts. It also decodes `source.status.json` and
`source.health.json` as typed current-transfer proof instead of accepting mere
file presence. Shell final evaluate, shell `workflow-status`, Swift app
workflow summary, and Swift final evaluation apply matching typed
status/health checks, including non-negative Swift `Int`-decodable integer
readiness counters, before they advance `source_transfer` or write
`evaluation.json`. Those surfaces also require `source.verify.json`,
`source.report.json`, `source.status.json`, and `source.health.json` to carry
present, normalized `target_root` evidence for the selected/evaluated target
root; after `evaluation.json` exists, shell `workflow-status` reopens
`source_transfer` if the source-side proof roots are later swapped to another
target. The target-ready phase is also a durable artifact check now:
`target.ready.json` must be a bundle-local regular
artifact with required readiness fields, and its address/mode/verification
code must match `meta.json` `evidence.target_ready` before shell
`workflow-status`, shell final `evaluate`, app workflow summary, or Swift final
evaluation treats target serve as ready. After that target-ready artifact is
valid, `source.pair.json.target_address` must match it before source-pair /
target-import evidence counts, and `source.transfer.json` must match the same
target address, target mode, and recorded receiver endpoint before
`source_transfer` or final `evaluate` can pass. A pairing-ready target artifact
is not enough for transfer: `source_transfer` and final `evaluate` also require
canonical `target.ready.json` to include a non-empty `receiver_address`,
`receiver_routes=true`, `push_network=true`, and `transfer=true`. Optional
`source.browse.json`
and `target.advertise.json` artifacts are also decoded against the same required
discovery JSON shape that Swift uses before shell `workflow-status` marks the
steps done or shell final `evaluate` writes `evaluation.json`. Shell bundle
bootstrap plus Swift bundle access also reject symlinked, hardlinked, and
non-regular `meta.json` before using it as durable bundle truth. Shell
`workflow-status` summary helpers apply the same file policy before reading
machine-facts and release-evidence artifacts, so advisory proof does not get
greener than final evaluation for hardlinked local artifacts. Archive ingress
also rejects symlink, hardlink, and special-file entries before recording a
verified handoff or publishing an incoming bundle root. Direct `merge-bundle`
ingress applies the same linked/special-file rejection to the incoming root and
publishes only after artifact, metadata, and `target.ready.json` conflicts have
been preflighted; publish-time artifact copy failures remove newly published
artifact files before returning failure. Shared Swift script-test helpers now
drain stdout and stderr concurrently while child processes run, and the
remaining Swift test helpers / skip-gated integration script launches route
through that harness. This keeps larger script acceptance fixtures from
deadlocking on pipe buffers before they can emit durable proof. This is still
local proof-policy parity; the duplicated shell/Python helper code can still
be consolidated.

The native app now consumes the same installed-app collection proof family for
two-machine workflow-summary rendering and installed-app launch
preview/preflight. Release-ready packaging evidence alone stays review-only
until current machine-facts and verified cross-machine bundle-handoff proof are
present, contradictory machine-facts or verified-handoff evidence blocks phase
launch, and stale `workflow.summary.json` artifacts are ignored unless the
current merged bundle still agrees on bundle status, release-evidence booleans,
installed-app proof fields, `steps`, and `next_actions`. Local sibling
contradictory verified `bundle_handoffs` now also leave shell
`workflow-status` in the same `review_bundle_handoff` lane the app uses,
instead of suggesting a fresh pack/unpack/merge cycle against an already
conflicted merged bundle. Local sibling
`<AppName>.app.notary/notarization.json` evidence is also rejected unless its
referenced post-staple audit still matches the current packaged app
provenance, and canonical sibling sidecar / post-staple audit symlinks are
treated as unsafe evidence rather than as bundle-local proof. The shell-side
installed-app path now uses the same rule:
both app-side packaging collection and shell `record-packaging-evidence`
reject stale sibling sidecars and sibling `notary-log.json` files that do not
decode as accepted notary log JSON during collection, and `workflow-status`
plus `evaluate --require-operator-evidence` only treat release evidence as
ready after stale copied notarization / notary-log leaves, including dangling
symlink leaves, have been cleared. Shell and app-side packaging collection also
refuse to overwrite existing bundle-local version, provenance, app-audit,
notarization, or notary-log output leaves unless they are single-link regular
files. Bundle-local `*.app-audit.json`, `*.provenance.json`,
`*.notarization.json`, and referenced `*.notary-log.json` still agree on the
same current packaged app. The native app now consumes that same bundle-local
currentness contract for installed-app launch preview, launch preflight,
workflow summary, and Swift final evaluate instead of accepting copied
notarization artifacts on status/readiness alone. The Acceptance Bundle panel
plus store-level
bundle-load note/event now also derive from the recomputed workflow summary
instead of raw `meta.status`, so a bundle with stale
`status=evidence_collected` but current pending `next_actions` stays in
review. That same currentness rule now also requires a current
bundle-local `evaluation.json` before the bundle can look complete: a bare
`meta.status=evidence_collected` no longer suppresses the final `evaluate`
action, and the strict `--require-operator-evidence` lane also reopens
`evaluate` when the preserved evaluation artifact was recorded without
operator-evidence enforcement. Installed-app launch preview likewise stays in
`review` until that current strict evaluation both exists and still matches
the current phase/operator proof inputs. Preview/preflight only allow the
matching corrective launch when that reopened step is the sole remaining strict
next action. If multiple required steps reopen, or the reopened step is
different from the requested launch, preview/preflight stay blocked instead of
continuing to trust the stale `evaluation.json`, even after release evidence
and distinct-machine installed-app proof are otherwise ready. Manual
Evidence gate chips and missing-proof
facts now also follow the current evaluation mode instead of stale stored
evaluation metadata. Corrective installed-app `pair` / `serve`
launches also remain blocked until the other machine's release packaging
evidence is ready; the locally selected machine cannot bypass missing release
evidence on its counterpart. When the current bundle instead needs machine-identity
correction, including missing role/machine-facts evidence, app-side `source
pair` and `target serve` remain available as the corrective rewrites the
workflow summary requests, while unrelated acceptance tasks stay blocked before
packaging evidence writes until source/target machine identity evidence is
repaired. Swift final evaluate now also uses the same guarded
bundle-artifact access seam as the bundle reader for bundle-local artifacts, so
bundle `..` path escapes, symlinked bundle artifacts, and malformed
`source.consistency.json` proof files fail closed at final evaluate instead of
being accepted by direct raw rereads. That same bundle-artifact seam now also
rejects symlinked acceptance bundle roots and symlinked `meta.json` during
bundle reads, and app-side bundle/packaging authoring validates both the
bundle root and `meta.json` before any file writes, so a symlinked bundle path
cannot accumulate partial app-authored artifacts before metadata mutation is
rejected.

For older bundle metadata that omitted explicit `source_pair` /
`source_transfer` subpaths, the app and shell advisory surfaces now also read
the canonical bundle artifacts (`source.pair.json`, `source.pair.txt`,
`source.transfer.json`, `source.network-push.txt`, `source.verify.json`,
`source.status.json`, `source.report.json`, `source.health.json`,
`source.consistency.json`, and `source.baseline.json`) before they decide that
those legacy lanes are still missing.

Those corrective app launches now also author the same machine-identity proof
shape shell phases use: they rewrite canonical `source.machine.json` /
`target.machine.json`, update `evidence.machine_facts.source|target`, and
refresh `roles.source_pair` / `roles.target` with the current installed app's
machine id and label. This only closes the machine-identity gap; real
two-machine handoff proof, operator evidence, Developer ID notarization, and
Gatekeeper evidence still remain separate required gates.

Strict manual evidence is also machine-bound: Local Network and firewall records
must carry the current target `machine_id`, and pairing-confirmation records
must carry the current source `machine_id`. Old unbound or wrong-machine
operator records remain review material only and reopen the manual evidence
steps in `workflow-status` / final `evaluate`.

The same app-side authoring path now also writes the shell's current-ready
surface (`target.ready.json` plus `meta.json` `evidence.target_ready`) instead
of only the phase snapshot, and it stages `source pair` receipts into
bundle-local `exported-receipts/<pairing_receipt_id>.json` before recording the
bundle-relative `receipt_path`. That keeps app-authored bundles closer to shell
`merge-bundle`, `source-pair`, and `target-import` replay semantics without
changing the still-partial overall T-011 status.

When the current successful app run still has retained stdout for the same
context, the app-side authoring path also preserves:

- `source.pair.txt`
- `target.adopt-pairing.txt`
- `source.network-push.txt`

The app can also record machine-local packaging evidence into the same bundle:

- `source.version.txt` / `target.version.txt`
- `source.provenance.json` / `target.provenance.json`
- `source.app-audit.json` / `target.app-audit.json`
- `source.notarization.json` / `target.notarization.json` when a structured
  `notarize-app.sh` result exists next to the packaged app as
  `<AppName>.app.notary/notarization.json`

That app-side artifact writing is phase-local evidence capture. It depends on
the current app snapshot or current profile SSOT and must follow the real
underlying command execution; it is not a replacement for `evaluate`, target
control-plane artifacts, or two-device LAN proof.

For shell-driven real-device collection, `acceptance-two-machine.sh` now fails
earlier than `evaluate`: in default `collection.mode=two_machine`, each phase
records the current machine's `*.app-audit.json` first and exits `5` before
phase execution unless that audit is install-ready (`status=pass`,
`summary.pass_ready=true`, and `readiness=distribution_ready`). A `review_only`
audit is still packaging evidence to fix, not install-ready proof. This is
intentional. Ad-hoc,
dirty, unstapled, or otherwise blocked builds are not valid real two-machine
installed-app evidence substrates. The explicit `same_machine` harness remains
the separate packaged-app wiring path and can still collect local review
evidence without claiming real-device readiness.

### Local macOS App Audit

Run the local app audit after building `macos/dist/SuperMover.app`:

```bash
macos/script/build-app.sh
macos/script/audit-app.sh macos/dist/SuperMover.app > macos/dist/SuperMover.app.audit.json
```

The audit JSON validates `Info.plist`, packaged provenance schema and
`app_bundle_id`/`app_version` consistency, current git HEAD and dirty state,
bundled CLI path/version, basic hashes, codesign verification/details for the
app and bundled CLI, Gatekeeper assessment, and stapler validation. It exits
nonzero when distribution evidence is blocked. That is the correct result for
unsigned, ad-hoc signed, dirty, or unstapled local builds.
When the canonical sibling `<AppName>.app.notary/notarization.json` sidecar is
present, `macos/script/audit-app.sh` now also fails closed if that sidecar is
malformed, if its `audit.path` no longer points at the canonical sibling
`<AppName>.app.notary/post-staple.audit.json`, or if that post-staple
audit/provenance no longer matches the current packaged app. Symlinked
canonical sidecar or post-staple audit paths are also rejected as unsafe. The
local audit's sidecar `release_ready` bit is intentionally as strict as the
installed-app gate: it requires a supported `auth_mode`, UUID-shaped Apple
submission id, `failure` absent/null, accepted sibling notary-log JSON, and a
post-staple `distribution_ready` audit. Accepted notary-log evidence must be
bound to the sidecar's UUID-shaped `submission.id` by matching `jobId`; a stale
accepted log from a different Apple notary job is review material, not release
proof. A sidecar that is current but missing those release fields is review
material, not distribution readiness.
The native app's installed-app packaging/evaluation path uses that same
sibling sidecar contract and also requires the sidecar's own `app_path` to
still match the current packaged app before local notarization is treated as
current release evidence.
For real installed-app acceptance tasks in `collection.mode=two_machine`, the
native app now also blocks launch preview when the current packaged app's
local sibling notarization evidence is missing or not release-ready, because
shell phase preflight would fail closed on that same condition before phase
execution.

Treat this audit as local release-engineering evidence. A notarized
distribution claim still needs Developer ID signing, a UUID-shaped Apple notary
submission id, notary log evidence, successful stapler validation, and the
relevant two-machine acceptance evidence; manifest text alone is not signing or
notarization proof.

When a Developer ID-signed app is ready for notarization, use the repository
script surface instead of an undocumented manual shell sequence:

```bash
SUPERMOVER_CODESIGN_IDENTITY="Developer ID Application: Example Corp (TEAMID1234)" \
sh macos/script/build-app.sh

SUPERMOVER_NOTARY_KEYCHAIN_PROFILE="supermover-release" \
sh macos/script/notarize-app.sh \
  --app macos/dist/SuperMover.app
```

`macos/script/notarize-app.sh` is deliberately fail-closed:
- it refuses to run without exactly one configured auth mode
- it blocks when archive, notary submission, notary log retrieval, stapler, or
  post-staple audit fails
- it rejects malformed `notarytool submit` output, including a missing or
  non-UUID Apple submission id, before fetching logs or stapling the app
- it rejects malformed, non-accepted, or submission-mismatched `notarytool log`
  JSON before stapling the app; accepted logs must report `status=Accepted`,
  either omit `issues`, set it to null, or provide it as an array, and carry a
  `jobId` matching the submitted UUID
- it emits structured JSON evidence even on blocked exits
- it persists that JSON into `macos/dist/SuperMover.app.notary/notarization.json`
  so installed-app acceptance can consume the same durable sidecar
- it also persists the referenced post-staple audit into that sibling sidecar
  directory, so currentness does not depend on a custom temporary `--work-dir`
  surviving later cleanup
- before its post-staple audit it removes any previous sibling sidecar result,
  so stale currentness evidence cannot block overwriting with fresh notarization
  output on a later rerun

This closes the gap between “audit can detect a blocked release” and “the repo
has no first-class notarization workflow surface.” It does not eliminate the
external dependency on Apple credentials or on real two-machine installed-app
acceptance evidence.

When `evaluate --require-operator-evidence` is used for real installed-app
acceptance, distribution-ready `*.app-audit.json` is no longer sufficient by
itself. The acceptance bundle must also carry per-machine structured
`*.notarization.json` artifacts proving a supported auth mode
(`keychain_profile`, `api_key`, or `apple_id`), a UUID-shaped Apple notary
submission id, a missing/null `failure` record, bundle-local accepted notary
log JSON whose `jobId` matches that submission id, and a post-staple
`distribution_ready` audit. Missing or unsupported
`auth_mode`, missing or malformed `submission.id`, non-null `failure`, missing
`notary_log.path`, or a referenced `*.notary-log.json` that is not accepted
notary log JSON for the same submission id is review evidence only; shell
phase preflight, workflow status, and final evaluation treat it as not
release-ready.

## Release Gates

Run these gates for the current local/mounted migration slice before cutting a
release candidate:

```bash
go mod tidy -diff
go test -count=1 ./...
go test -race -count=1 ./...
go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
go vet ./...
staticcheck ./...
golangci-lint run ./...
git diff --check
go run ./cmd/supermover help
go run ./cmd/supermover version
go run ./cmd/supermover profile help
go run ./cmd/supermover push --help
go run ./cmd/supermover verify --help
go run ./cmd/supermover deleted help
go run ./cmd/supermover health --help
go run ./cmd/supermover drift help
go run ./cmd/supermover drift record --help
go run ./cmd/supermover drift acknowledge --help
go run ./cmd/supermover drift expire --help
go run ./cmd/supermover drift resolve --help
go run ./cmd/supermover reconcile --help
go run ./cmd/supermover reconcile review --help
go run ./cmd/supermover sync --help
go run ./cmd/supermover sync run --help
go run ./cmd/supermover sync loop --help
go run ./cmd/supermover sync watch --help
go run ./cmd/supermover sync network run --help
go run ./cmd/supermover sync network discover-run --help
go run ./cmd/supermover sync network loop --help
go run ./cmd/supermover report --help
go run ./cmd/supermover status --help
go run ./cmd/supermover recover --help
go run ./cmd/supermover daemon help
go run ./cmd/supermover prune --help
```

The current release gate is local-only. Passing it means the wired CLI supports
profile-driven one-way migration to a trusted local or mounted target, plus
local audit, health, report, compact status, verify, deleted-review,
drift recording, persisted drift acknowledgement, persisted drift expiry, and
conservative recovery surfaces, plus narrow persisted-drift reconcile for
selected missing-file repair and resolve-noop cases, read-only reconcile
boundary review, plus durable incremental
queue evidence, one bounded local `sync run` pass, foreground local
`sync loop` polling, foreground `sync watch`, bounded `sync network run`,
bounded LAN-discovery-gated `sync network discover-run`, foreground
`sync network loop`, plus profile-enabled
foreground-daemon local polling and profile-enabled source-side foreground
daemon network polling. The
profile-backed non-dry-run
`push --network` path requires the separate network smoke and network recovery
evidence below. The foreground `daemon` lifecycle gate covers only
`.supermover/daemon` install/state/stop-intent/restart-intent and redacted
lifecycle-event evidence around the existing serve behavior, optional
profile-enabled local polling queue consumption, and optional source-side
network polling worker mode. Passing the local gate does
not mean OS service-manager installation, detached background process
management, automatic LAN discovery endpoint selection, per-entry
network transport, broad network resume acceptance, broad drift reconcile workflow, background scanning, broad
repair, or broader prune release workflow automation is implemented. Current automated
network recovery evidence covers only the profile-backed same-session CLI/Runner
rerun path after receiver-accepted
payload bytes and a simulated transport failure, plus the earlier
receiver-status and published-session retry cases. That evidence is not
arbitrary process-kill recovery, general broad interruption/restart release
acceptance, or anonymity.
The command-surface gates for drift review are `drift record --help`,
`drift acknowledge --help`, `drift expire --help`, `drift resolve --help`, and
`reconcile --help`; behavior evidence comes from automated drift-review/
reconcile tests and from disposable persisted-record smoke below when release
operators need manual proof.
The wired `prune --dry-run` command validates
profile path, flags, target root, and profile prune policy, then reads published
soft-delete records and emits review-only candidates, refusals, and artifact
problems without mutating target files or writing approval or receipt artifacts.
Records still inside `delete_policy.retention_days` are surfaced as
`retention_window_active` refusals. `prune approve` writes durable approval
artifacts plus profile snapshots from fresh dry-run candidate evidence without
deleting target files or writing prune receipts.
`prune --apply --approval <id>` is the only physical prune path: it writes a
started receipt before deletion, re-runs the current prune plan including
retention checks, and records applied/partial/failed status after target-state
rechecks when finalization succeeds. `report` is read-only and exposes prune
candidates, refusals, current-scope approval evidence, existing receipts, and
receipt issues alongside other local evidence, while `status` exposes compact
prune release counts plus prune review status/action. `prune review` exposes the prune-only
release-review inventory without writing approvals, receipts, or target files;
none of these read-only commands applies prune decisions.
Include `supermover prune
--help` and `supermover prune review --help` in command-surface release gates,
a disposable `prune --dry-run` smoke for review evidence, a `prune review`
smoke for focused read-only release inventory, a `prune approve` smoke for
approval authoring, a `report` smoke before/after prune apply, and a separate
disposable `prune --apply --approval <id>` smoke when validating physical-prune
apply. `drift list`, `report`, `prune review`, and `status`
live detector surfaces stay read-only; use `drift record` to persist current
findings as `.supermover/drift` review records. `reconcile plan` can inspect
selected persisted drift IDs without mutation, `reconcile review` can inspect
persisted plan readiness and live-only record-required repair inputs without
mutation, and `reconcile apply` requires persisted-drift selection intent,
explicit `--apply`, and `--reason` before the current narrow missing
regular-file repair or already-restored/absent resolve-noop path can run.
Selection can be explicit `--id` values, `--all-persisted-planned` to select
only currently planned persisted actions after review, or `--record-live` to
first persist current live detector findings and then apply only the resulting
persisted planned actions. Protocol-client network runs have bounded level 2
padding, batching, and timing jitter evidence.

The wired compact local status contract lives in `docs/status.md`. Include
`supermover status --profile <path> [--format text|json]` in local release
smoke only as a read-only local profile/target evidence check, not as daemon,
LAN, encrypted-transfer, or sync status. Include
`sync queue`/`sync run`/`sync loop`/`sync watch` evidence only for the local queue slice:

```bash
go run ./cmd/supermover sync queue enqueue --profile ./target.profile.json
go run ./cmd/supermover sync queue status --profile ./target.profile.json
go run ./cmd/supermover sync queue list --profile ./target.profile.json
go run ./cmd/supermover sync queue ready --profile ./target.profile.json
go run ./cmd/supermover sync run --profile ./target.profile.json --session sync-run-001
go run ./cmd/supermover sync loop --profile ./target.profile.json --session-prefix sync-loop --max-runs 2
go run ./cmd/supermover sync watch --profile ./target.profile.json --session-prefix sync-watch --max-events 1
```

This smoke should preserve queue state and the
`.supermover/incremental-sync/.../runs/<session>.json` run receipts. It proves
local queue state, local scan/enqueue/publish passes, foreground OS watcher
event-triggered passes, and retry-backoff behavior when covered by tests. It
does not prove target repair, target synchronization, detached background
execution, source-side daemon network polling, automatic endpoint selection, or
bidirectional sync. Use
`sync queue list --profile ./target.profile.json --format json` for per-entry
queue lifecycle detail. Use
`sync queue fail --profile ./target.profile.json --id <entry-id> --reason "<operator terminal reason>"`
only when explicitly marking one queued observation as terminal operator review
evidence.

For a bounded profile-backed network queue smoke, first prepare the same paired
source/target profiles required by non-dry-run `push --network`, including
`network.receiver_url`, `network.local_tls_identity`, pairing receipt evidence,
traffic level 2 policy, and a profile-selected `target.local_path` available to
the source-side operator for queue/run receipts. Then run:

```bash
go run ./cmd/supermover sync network run --profile ./source.profile.json --session sync-network-001
go run ./cmd/supermover sync network discover-run --profile ./source.profile.json --session sync-network-discover-001
go run ./cmd/supermover sync network loop --profile ./source.profile.json --session-prefix sync-network --max-runs 2
```

These commands validate profile trust, network material, local TLS identity,
and the network push contract before queue mutation. They write ordinary
incremental queue/run receipts under the profile-selected target control plane
and publish ready queue entries through a per-entry profile-backed mTLS network
manifest. Regular-file replacement requires previous published manifest
evidence and receiver-side target revalidation. `sync network discover-run`
first requires a low-information LAN candidate that exactly matches the
profile-selected `network.receiver_url`; the match is an availability gate, not
trust or endpoint selection. `sync network loop` is foreground-only and uses
generated session IDs; idle queue passes write run receipts without contacting
the receiver. These commands are not automatic endpoint selection, OS file
watchers, broad repair, or detached daemon execution.

Foreground daemon local polling sync is profile-backed, not a runtime override.
To include it in a daemon smoke, add a reviewed profile stanza such as:

```json
"sync": {
  "local_polling": {
    "enabled": true,
    "interval_millis": 60000,
    "retry_backoff_millis": 60000,
    "session_prefix": "daemon-sync"
  }
}
```

Then run `daemon run --foreground --profile <path>` under a supervisor, mutate
or add a source file, and verify the target control plane contains
`.supermover/incremental-sync/.../runs/daemon-sync-000001.json` with the
expected profile/target scope. A `daemon restart` intent should be consumed by
the running foreground daemon; after restart, a new source change should create
a fresh generated run receipt, for example `daemon-sync-000002.json`, without
reusing or overwriting the earlier receipt. This proves only local polling queue
consumption inside the foreground daemon. It does not prove OS file watching,
detached service restart, crash supervision, source-side network polling, or
automatic LAN discovery endpoint selection.

Foreground daemon network polling sync is also profile-backed, not a runtime
override. To include it in a daemon smoke, enable the source profile worker
mode instead of `sync.local_polling`:

```json
"sync": {
  "network_polling": {
    "enabled": true,
    "interval_millis": 60000,
    "retry_backoff_millis": 60000,
    "session_prefix": "daemon-network-sync"
  }
}
```

Then run `daemon run --foreground --profile ./source.profile.json` under a
supervisor while the paired target `serve` receiver is running. Mutate or add a
source file, and verify the target control plane contains
`.supermover/incremental-sync/.../runs/daemon-network-sync-000001.json` plus
the receiver-side `network-transfer.json`. A foreground process stop/start
should resume generated run numbering from existing durable receipts, for
example `daemon-network-sync-000002.json`, without overwriting the earlier
receipt. This mode is source-side worker-only; it does not serve pairing or
receiver routes, browse the LAN, select endpoints automatically, or detach
from the foreground process.

Foreground daemon lifecycle evidence lives under `.supermover/daemon` and is
separate from compact migration `status`:

```bash
go run ./cmd/supermover daemon install --profile ./target.profile.json
go run ./cmd/supermover daemon status --profile ./target.profile.json
go run ./cmd/supermover daemon logs --profile ./target.profile.json --tail 20
go run ./cmd/supermover daemon run --foreground --profile ./target.profile.json
go run ./cmd/supermover daemon restart --profile ./target.profile.json --reason "operator restart"
go run ./cmd/supermover daemon stop --profile ./target.profile.json --reason "operator stop"
```

This gate proves profile-derived foreground lifecycle state, redacted lifecycle
events, stop intent, restart intent, and optional profile-enabled local polling
or network polling sync receipts, plus optional profile-enabled drift recording
review evidence and persisted reconcile apply receipts. `daemon restart` is
consumed by a running foreground daemon and restarts serve listeners plus the
profile-enabled local polling queue consumer, drift recording worker, and
persisted reconcile apply worker in that same process, or restarts the
source-side network polling worker in worker-only mode. Drift recording writes
durable `.supermover/drift/*.json` review evidence and does not repair target
files. Persisted reconcile apply uses only already persisted, currently planned
drift records and stops after refusals for operator review. The daemon does not
install launchd/systemd/Windows services, spawn a
detached daemon, supervise crash restart, browse LAN for endpoint selection,
watch files, run broad automatic repair, record live-only drift before apply,
rewrite manifests, authorize prune, retry repair in the background, or execute
automatic discovery-selected sync.

Current manual network smoke can exercise pairing, receiver route wiring,
dry-run preflight, and non-dry-run profile-backed mTLS transfer. The narrow
same-session interruption-rerun gate is automated test evidence, not a separate
operator CLI failure-injection command. Receiver-route evidence requires
preserving `serve` stderr from a paired target profile that has complete
`network.receiver_url` and `network.local_tls_identity`; it should show receiver
routes enabled without treating discovery as trust:

```bash
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover discover browse
go run ./cmd/supermover pair --profile ./source.profile.json --target <address> --verification-code <code> --receipt-out ./exported-receipts
go run ./cmd/supermover profile adopt-pairing --profile ./target.profile.json --receipt-file ./exported-receipts/<receipt-id>.json
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover profile set-network --profile ./source.profile.json --receiver-url https://<receiver-ip>:<receiver-port>
go run ./cmd/supermover push --network --profile ./source.profile.json --dry-run
go run ./cmd/supermover push --network --profile ./source.profile.json --session <network-session-id>
```

Current `push --network --dry-run` is preflight-only. An unpaired or invalid
profile exits earlier with a profile/pairing diagnostic. A paired profile that
lacks `network.receiver_url` or complete `network.local_tls_identity` exits
earlier with a profile network-material diagnostic. Only after paired profile,
receipt evidence, network material, local TLS identity files and pins, scan,
and manifest shape validate does it emit
`transfer=dry_run encrypted_transfer=profile_backed_mtls_validated
resume=not_attempted resume_authority=not_attempted
resume_outcome=not_attempted`. It sends no files, contacts no receiver, and
writes no `network-transfer.json`.

Current non-dry-run `push --network` connects to the profile-selected pinned
TLS 1.3 mTLS receiver, streams through `networkpush`/`networkrun`/
`protocolclient`, resumes same-session uploads from receiver status offsets
when a compatible partial receiver session already exists and auditable prior
`network-transfer.json` proves the earlier accepted payload overhead, retries
commit idempotently for already published sessions, and writes receiver-side
`network-transfer.json` only after receiver begin stores a session. For
published network transfers, that artifact is the proof surface for transfer
status and applied privacy overhead. Zero-byte regular files are supported on
this profile-backed path through an explicit final empty completion record from
the source protocol client; a clean publish should produce the target file,
receipt, and `network-transfer.json` evidence like other published network
files. Transport setup failures and begin-auth refusal can still leave no
network-transfer artifact.

For packaged-app evidence collection, `sh macos/script/acceptance-two-machine.sh --help`
documents the same five-phase operator shape:

1. `target-serve`
2. `source-pair`
3. `target-import`
4. `source-transfer`
5. `evaluate`

That five-phase packaged flow has been verified locally as a same-machine
simulation. Treat it as wiring evidence only. It is not real two-machine
installed-app LAN evidence, not Local Network/firewall prompt evidence, and
not notarized distribution evidence.
The app-side `evaluation` path has also been replayed against a real
same-machine harness bundle after removing the harness-generated
`evaluation.json`, so current app evaluation is verified against durable
bundle artifacts plus target control-plane evidence rather than only fixture
JSON.
The current built-app `AppStore` acceptance actions for packaging,
target-import, and evaluation have also been exercised against that
same-machine bundle shape. That app-first replay deletes the harness-written
packaging, `target_import`, and `evaluation` artifacts, then rewrites them
through the app surface against the same durable bundle and target
control-plane evidence. This gives the app-first authoring surface local smoke
evidence in addition to the lower-level evaluator and packaging collector
tests.

To run that same-machine five-phase orchestration end to end and preserve one
local bundle automatically:

```bash
sh macos/script/acceptance-two-machine-same-machine.sh
```

The local orchestrator reuses one migration `profile_id` across the source and
target profile files so exported receipt import can validate the intended trust
closure on one machine. This is still wiring evidence only.

Before starting a large first network run, ensure that the trusted target is
empty or intentionally contains only byte-identical payload. A new receiver
session fails at begin, before accepting chunks or creating session state, when
it can already see divergent target files, symlinks, or incompatible
directories. Commit still rechecks targets to catch changes during transfer.
This is a fail-fast conflict guard, not changed-file network synchronization;
do not use a new network session to update previously migrated files.

For target-side visual verification after a published migration, run the
loopback-only operator page on the target device:

```bash
go run ./cmd/supermover dashboard --profile ./target.profile.json
```

Open the emitted access-token URL on the target, or reach it through an SSH
local port forward while preserving that URL. The page is read-only and
performs one full `verify` plus a live scan for target paths outside the
selected manifest on page open and whenever the operator presses refresh.
This may read every regular target file, so it deliberately does not poll
continuously during a large transfer and refuses overlapping full-check
requests. A green page means the target matches its
latest published manifest snapshot and has no detected extra target paths or
review evidence; it does not prove that the source tree has not changed since
that publish, and it is not a Merkle-root comparison.
Current real two-machine acceptance now treats that gap as a hard gate, not
just explanatory copy. The bundle must carry `source.consistency.json` with an
explicit current-source proof verdict. The same-machine installed-app harness
now writes a real CLI-generated `status=pass` / `mode=current_source_verified`
artifact against the exact source baseline used for `push --network`. Distinct
machine installed-app acceptance is still incomplete because it also requires
separate operator-evidence, release-audit, and broader end-to-end proof gates;
any lane that lacks a real current-source verdict still fails closed instead of
implying end-to-end consistency.
Command output keeps `resume=receiver_status` for compatibility and adds
`resume_authority=receiver_status` plus `resume_outcome=fresh`, `resumed`, or
`published_retry`. Only `resume_outcome=resumed` with nonzero `resumed_bytes`
is proof that a rerun uploaded remaining payload bytes. A zero-payload
`published_retry` preserves the prior published payload overhead evidence.
Partial receiver-status retries also require prior transfer evidence whenever
the receiver already has accepted payload bytes. If the needed prior
`network-transfer.json` is missing, corrupt, mismatched, non-published where a
published retry is required, or lacks payload padding/batching counters, the
rerun writes `needs_repair` with
`error_code=payload_overhead_missing` and blocks the network privacy release
claim instead of fabricating applied-overhead proof.

The current automated interruption-rerun gate is deliberately bounded. It uses
a profile-backed same-session network run, lets the receiver accept payload
bytes, simulates a transport failure, then reruns the same profile/session
through the CLI/Runner path. Passing evidence is `resume_authority=receiver_status`,
`resume_outcome=resumed`, nonzero `resumed_bytes`, a published
`network-transfer.json` with retained/merged privacy overhead, clean
`health`/`status`/`report` review, and matching source/target hashes. This is
not a manual operator process-kill smoke, not an OS-daemon restart workflow, and
not general broad release acceptance. f-22wnwd5pe/T-001 adds an internal
`networkrun` fixture for a source stop immediately after durable in-flight chunk
progress evidence is written; the same-session rerun resumes from receiver
status, merges prior privacy-overhead evidence, and publishes matching target
content.

Keep a separate future acceptance gate for the remaining network product
surfaces. Do not mark OS service-manager daemon installation, detached
background process management, continuous watcher/network sync, or broad resume
acceptance passed until they have command-level validation evidence:

```bash
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover discover browse
go run ./cmd/supermover pair --profile ./supermover.profile.json --target <address> --verification-code <code>
go run ./cmd/supermover push --network --profile ./supermover.profile.json --session <network-session-id>
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <network-session-id>
go run ./cmd/supermover report --profile ./supermover.profile.json --session <network-session-id>
```

### f-22vnwgwjj Traffic Privacy Level 2 Gate

The traffic privacy reporting gate (f-22vnwgwjj) covers the
profile-backed network path and release checklist, but it does not close traffic
privacy level 2 as an anonymity claim or as completion of
LAN/OS-daemon/incremental-sync/broad-resume product work. Automated release-smoke
coverage now exercises profile lint, CLI `serve`, non-dry-run `push --network`,
`verify`, `health`, compact `status`, text `report`, JSON `status`, JSON
`report`, and receiver-side `network-transfer.json` evidence for a fresh
profile-backed mTLS transfer. The bounded interruption-rerun gate additionally
exercises receiver-accepted payload bytes followed by simulated transport
failure and same-session CLI/Runner recovery. Remaining acceptance work includes
receiver-side recovery UX, broad resume acceptance, arbitrary process-kill
recovery, OS-daemon behavior, continuous watcher/network sync, and manual
release-candidate evidence capture.

For this release boundary, use only currently wired surfaces:

```bash
go run ./cmd/supermover profile lint --profile ./source.profile.json
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover push --network --profile ./source.profile.json --dry-run --session <network-session-id>
go run ./cmd/supermover push --network --profile ./source.profile.json --session <network-session-id>
go run ./cmd/supermover verify --profile ./source.profile.json --session <network-session-id>
go run ./cmd/supermover health --profile ./source.profile.json
go run ./cmd/supermover status --profile ./source.profile.json
go run ./cmd/supermover status --profile ./source.profile.json --format json
go run ./cmd/supermover report --profile ./source.profile.json --session <network-session-id>
go run ./cmd/supermover report --profile ./source.profile.json --session <network-session-id> --format json
```

f-22vnwgwjj acceptance for the current slice is documentation, wired
operator smoke coverage, and evidence readiness:

- `profile lint` must reject level 2 profiles that omit required padding,
  batching, jitter, or low-information discovery settings.
- `health` and `report` must remain honest when no network transfer artifact
  exists. They must not imply encrypted transfer readiness or a completed
  network run.
- `report`, `status`, and `health` may surface non-published or invalid
  `network-transfer.json` artifacts as review issues. Clean published transfer
  overhead must still be preserved from the receiver-side artifact; these review
  commands do not need to print a `network_transfer` issue line for clean
  published sessions.
- `report` and compact `status` expose
  `traffic_privacy_acceptance`. A pass requires profile-backed mTLS plus a
  clean published level 2 `network-transfer.json` whose profile policy and
  device IDs match the configured profile and pairing receipt, with applied
  padding, batching, and jitter counters. Missing or mismatched evidence is a
  blocker, not an inferred pass.
- The automated fresh network release smoke is
  `TestPushNetworkReleaseSmokePublishesAndReportsViaCLI` in
  `internal/cli/cli_test.go`. It is an operator-facing CLI smoke, not an
  internal-only protocol-client assertion.
- The runbook must preserve the residual-leakage boundary: level 2 is bounded
  metadata reduction, not anonymity. Total bytes, transfer duration, peer IP
  addresses, LAN presence, and Supermover use remain observable.
- The profile remains the SSOT. Do not add CLI, environment, or one-off runtime
  privacy overrides for padding, batching, jitter, or traffic level.

Expected evidence snippets for a level 2 profile include:

```text
privacy policy=status=profile_contract_only ... traffic_level=2 ... claim=bounded_reduction_only ... residual_leakage=total_bytes,duration,peer_ip,lan_presence,supermover_use ... network_transfer=not_configured
privacy status=profile_contract_only ... traffic_level=2 ... overhead_status=not_applied ... local_push=traffic_shaping_not_applied ... network_transfer=not_configured
```

For a profile with complete network material and readable TLS identity files,
the profile-level line may instead report
`network_transfer=profile_backed_mtls_configured`. In both cases, applied
overhead is proven by `network-transfer.json`, not by profile policy alone.
`traffic_privacy_acceptance status=blocked ... blockers=applied_overhead_missing`
is expected until a published transfer artifact with applied overhead exists.
After a clean profile-backed level 2 transfer, `status` and `report` should
show:

```text
traffic_privacy_acceptance status=passed ... anonymity_claim=not_claimed ... observed_padding_bytes=<n> ... observed_jitter_budget_millis=<n>
```

When a non-published, failed, damaged, or otherwise review-required
`networkrun` artifact from non-dry-run `push --network` or another network
runner exists, `report` may show applied overhead fields such as:

```text
network_transfer session=<session> ... privacy_level=2 ... privacy_padding_bytes=<n> ... privacy_batch_frames=<n> ... privacy_jittered_requests=<n> ... privacy_overhead_jitter_budget_millis=<n>
```

Do not expect the `network_transfer` line from
`push --network --dry-run`; preflight writes no network session artifact. Also
do not require that line for a clean published transfer: clean published
network transfer overhead is proven from the receiver-side
`.supermover/sessions/<session>/network-transfer.json`, while
`health`/`status`/`report` should stay clean.

The network outcome artifact for non-dry-run attempts that reach `networkrun`
is:

```text
<target>/.supermover/sessions/<session>/network-transfer.json
```

When a real network attempt is run, the release gate must preserve
command output and exit code for every attempt. Preserve
`network-transfer.json` when the attempt reached a receiver session and
produced one; otherwise record its absence explicitly. Receiver-side artifacts
such as the session receipt, manifest, warning records, and
`network-session.json` should be preserved when present; for pre-begin failures
such as `auth_refused` or transport setup failure, record their absence instead
of treating absence as cleanup permission. Then review `health --profile ...`
and `report --profile ... --session <network-session-id>` for
`network_transfers`, artifact problems, pairing state, applied privacy policy,
applied overhead, and residual-leakage notes. The local push release gate does
not require `network-transfer.json` because local push does not use the network
runner.

The automated bounded interruption-rerun gate covers one CLI/Runner recovery
shape: a level 2 profile-backed same-session network transfer with no runtime
privacy overrides, receiver-accepted payload bytes, simulated transport failure,
and same-session CLI/Runner rerun recovery. Additional f-22wnwd5pe/T-001
internal evidence covers a deterministic `networkrun` source stop after durable
in-flight chunk progress evidence and a same-session receiver-status resume.
Treat the following as automated
gate output to preserve or summarize, not as a manual operator
failure-injection recipe:

- the profile used by `profile lint`;
- `health` output before and after recovery/resume;
- `report` output for the network session;
- `.supermover/sessions/<session>/network-transfer.json`;
- receiver-side receipt, manifest, warning, and network-session artifacts when
  they exist;
- command stdout, stderr, and exit codes for the original attempt, simulated
  transport failure, same-session rerun, and report review.

That bounded gate passes only when the preserved evidence shows configured
level 2 policy, applied padding, batching, and jitter overhead,
receiver-status resume for the same session, no claim of anonymity, and
explicit residual leakage for total bytes, transfer duration, peer IP
addresses, LAN presence, and Supermover use.

The f-22wnwd5pe/T-002 acceptance matrix adds command-level receiver restart and
fail-closed evidence to that bounded gate. Treat the following as current
supported evidence only when the same profile, same session, and same
profile-selected target control plane are used:

- source/network interruption after receiver-accepted payload bytes followed by
  `resume_outcome=resumed` and nonzero `resumed_bytes`;
- receiver listener restart over preserved target state followed by the same
  same-session resume evidence;
- commit-only and published-session retry with no chunk reupload and preserved
  prior payload-overhead evidence;
- missing, corrupt, mismatched, non-published, or payload-empty prior
  `network-transfer.json` evidence blocked as `needs_repair` with
  `error_code=payload_overhead_missing`.

Keep a separate future/manual gate for broad interruption/restart release
acceptance. That later gate must cover the intended operator workflow beyond
the simulated transport-failure seam, including receiver-side recovery UX,
arbitrary process-kill or process restart scenarios, and any manual
release-candidate evidence capture. Network `recover`, daemon/OS-service
restart recovery, power-loss recovery, automatic retry policy, and broad
reconcile integration remain unwired. An explicit non-resumable refusal is
useful diagnostic evidence, but it keeps broad resume acceptance blocked.

## Manual Smoke

Use a disposable source and target so the smoke can exercise publication,
verification, soft-delete review, health, report, and recovery dry-run without
touching production data:

```bash
SMOKE_ROOT="$(mktemp -d)"
SRC="$SMOKE_ROOT/source"
DST="$SMOKE_ROOT/target"
PROFILE="$SMOKE_ROOT/supermover.profile.json"
SESSION="smoke-local"
BIN="$SMOKE_ROOT/supermover"
mkdir -p "$SRC/subdir" "$DST"
printf 'hello\n' > "$SRC/subdir/file.txt"
printf 'hidden\n' > "$SRC/.hidden"

go build -o "$BIN" ./cmd/supermover
"$BIN" profile init --profile "$PROFILE" --source "$SRC" --target "$DST"
"$BIN" profile lint --profile "$PROFILE"
"$BIN" push --profile "$PROFILE" --dry-run
"$BIN" push --profile "$PROFILE" --session "$SESSION"
"$BIN" verify --profile "$PROFILE" --session "$SESSION"
"$BIN" status --profile "$PROFILE"
rm "$SRC/subdir/file.txt"
"$BIN" push --profile "$PROFILE" --session "${SESSION}-delete"
"$BIN" deleted list --profile "$PROFILE"
"$BIN" health --profile "$PROFILE"
"$BIN" report --profile "$PROFILE" --session "${SESSION}-delete" || test $? -eq 1
"$BIN" status --profile "$PROFILE" || test $? -eq 1
"$BIN" recover --profile "$PROFILE" --dry-run
```

Expected smoke evidence:

- the profile is created and linted from the profile SSOT;
- the smoke uses a built binary, not `go run`, for commands where the exact
  application exit code matters;
- the first push publishes files, including `.hidden`, to the local/mounted
  target;
- `verify` succeeds for the first session;
- the second push records the source-side deletion as a soft-delete review item;
- `deleted list` shows the deleted source path without physically pruning the
  target;
- `health` has no recovery work for a clean local run;
- `report` summarizes warnings, soft deletes, prune candidates/refusals,
  existing prune receipts, health, artifact, and verify state from the target
  `.supermover` evidence and returns non-zero because the smoke intentionally
  creates a soft-delete review item;
- the first `status` call returns zero for the clean profile-selected local
  target, and the post-delete `status` call emits review-required evidence and
  returns `1`;
- `recover --dry-run` reports intended recovery actions without mutating target
  state.

## Preflight

1. Confirm the source and target paths:

   ```bash
   test -d /path/to/source
   mkdir -p /path/to/target
   ```

2. Create or update the profile:

   ```bash
   go run ./cmd/supermover profile init --profile ./supermover.profile.json --source /path/to/source --target /path/to/target
   go run ./cmd/supermover profile lint --profile ./supermover.profile.json
   ```

   If the profile already exists, do not overwrite it with `profile init`.
   Review and edit the existing profile instead.
   If only the trusted local target path changed, use:

   ```bash
   go run ./cmd/supermover profile set-target --profile ./supermover.profile.json --target /path/to/target
   go run ./cmd/supermover profile lint --profile ./supermover.profile.json
   ```

3. Verify policy gates in the profile:

   - `delete_policy.require_review` is true.
   - `privacy_policy.allow_plaintext_restore` is true only for trusted targets.
   - `privacy_policy.padding_bucket_bytes`, `batch_max_bytes`,
     `batch_max_count`, and `jitter_budget_millis` are non-zero for traffic
     level 2.
   - `privacy_policy.discovery_low_info` is true for traffic level 2.
   - traffic level 2 is a schema/profile gate only in current CLI local push.
     Protocol-client network runs, including non-dry-run `push --network`,
     apply padding, batching, and bounded timing jitter, but this does not mean
     anonymity or LAN/daemon/incremental-sync support is wired.
   - `target.target_id` names the intended target identity and is not a local
     filesystem path.
   - `target.local_path` points at the trusted local restore directory.

   Changing `target.local_path` must not change `target.target_id` unless the
   operator is intentionally switching targets and passes `--target-id`.

## Dry-Run Gate

```bash
go run ./cmd/supermover push --profile ./supermover.profile.json --dry-run
```

Continue only if:

- entry counts match the migration expectation closely enough to explain;
- warning count is reviewed; full warning JSON is available only after a run is
  published;
- agent influence count is expected for the repository or home directory being
  moved;
- no operator expected runtime flags to override profile policy.

For JSON-style inspection before a real run, use
`go run ./cmd/supermover scan --profile ./supermover.profile.json --format json`.
After a real run, use the target control-plane artifacts. If the source scan
reports a `scan_error`, push is blocked before publish; fix source readability
and rerun the dry-run gate.

## Publish Gate

```bash
SESSION_ID=local-$(date -u +%Y%m%dT%H%M%SZ)
go run ./cmd/supermover push --profile ./supermover.profile.json --session "$SESSION_ID"
```

Capture the printed session ID. If a fixed session ID is used for acceptance
tests, choose a clean target directory or inspect existing session artifacts
before rerunning.

## Post-Run Evidence Checklist

```bash
find /path/to/target/.supermover -maxdepth 4 -type f | sort
```

Required:

- profile snapshot exists under `.supermover/profiles/`;
- session receipt exists under `.supermover/sessions/<session>/receipt.json`;
- manifest exists under `.supermover/sessions/<session>/manifest.json`;
- warning records exist when the run reported warnings;
- soft-delete records exist when the run reported deleted paths;
- agent influence record exists when the run reported influences.
- `network-transfer.json` exists for non-dry-run network attempts that reach
  receiver begin and store a session; it is not expected for local push
  sessions, network dry-run preflight, or pre-begin network failures. When
  present, inspect `privacy_policy` and
  `privacy_overhead` to compare configured level 2 bounds with applied
  padding, batching, and bounded jitter overhead.

Inspect the receipt:

```bash
sed -n '1,160p' /path/to/target/.supermover/sessions/<session>/receipt.json
```

Acceptance criteria:

- `status` is `published`;
- `profile_id` matches the profile used by the operator;
- `target_id` matches the intended target identity.

Inspect warnings:

```bash
find /path/to/target/.supermover/warnings -type f -name '*.json' -maxdepth 1 2>/dev/null | sort
```

Every warning must have an owner decision: accept, rerun with changed profile,
or block release.

Run the read-only aggregate report:

```bash
go run ./cmd/supermover report --profile ./supermover.profile.json
```

Use it to review the combined operator surface:

- warnings and profile suggestions;
- soft-delete records that need review;
- health and recovery issues;
- artifact problems such as missing or corrupt receipts, manifests, and
  profile snapshots;
- network transfer outcome artifacts, when present, including `auth_refused`,
  `interrupted`, `needs_repair`, `publish_failed`, and corrupted or mismatched
  `network-transfer.json` evidence;
- pairing state: `unpaired`, `paired_receipt_valid`, or receipt
  mismatch/missing/invalid states requiring review;
- encrypted transfer and network-transfer evidence for non-dry-run
  `push --network` sessions, and `not_wired`/absent evidence only for surfaces
  that remain unwired or attempts that did not reach receiver begin and a stored
  session;
- published-manifest verification state for the local push evidence;
- live target drift detected from the selected published manifest and target
  filesystem at report time.

For f-22cnwwseh T-004 network sessions, review `health`, compact `status`, and `report`
together. A published `network-transfer.json` with matching receipt/session
state proves the completed transfer and applied privacy overhead. Non-published
or damaged network transfer artifacts remain review evidence, not a completed
transfer claim, and can surface through those read-only commands.

The report is a summary over `.supermover` evidence plus read-only live target
drift detection. Preserve the underlying profile snapshot, receipts, manifests,
warnings, deleted records, prune approvals/receipts, and influence records as
the audit source of truth.
JSON reports expose live detector output under `live_target_drift`, with
summary counters such as `live_target_drifts` and
`live_target_drift_artifact_problems`. This live report evidence is read-only;
use `drift record` when current findings must be written to
`.supermover/drift/*.json` for review. `drift expire` can retire stale
persisted review evidence without claiming the target is restored; resolved or
expired persisted records no longer count as unresolved persisted drift, but
current live drift remains review-required. `drift resolve` can close existing
persisted records only after target restoration makes the same path and
expected baseline clean under a fresh detector. `reconcile plan/review/apply`
can repair only selected persisted missing-regular-file drift from matching
published/source evidence, or resolve already-restored/absent persisted
records; `review` remains a read-only boundary inventory and does not broadly
reconcile or prune drift. Reconcile refusal
`conflict_class`/`retry_advice` fields are review guidance for selecting a
fresh plan or manual repair path, not proof that repair is safe to run
automatically.
JSON reports expose physical-prune review evidence under `prune_review`:
pending candidates, refusals such as already-missing targets, current-scope
approval evidence, receipts, and non-applied receipt issues. Approval evidence
is read from durable `.supermover/prune/approvals/*.json` artifacts scoped to
the current profile/target. `prune review` exposes that prune inventory as a
focused read-only release-review surface, while `status` exposes only compact
counts plus prune review status/action. This evidence helps review
authored-but-unapplied approvals, stale or expired approvals, consumed
approvals, and receipt-attention states; the read-only surfaces do not author
approvals, supersede approvals, apply prune decisions, write receipts, delete
files or symlinks, repair/reconcile drift, make the target clean,
automatically release a migration, or close v1.

`report` exits non-zero when the generated report requires review, including
empty targets, warning records, soft deletes, prune candidates, prune refusals,
non-applied prune receipts, recovery issues, artifact problems, verification
findings, live target drift, or pairing receipt/profile mismatches. In shell
scripts, capture stdout before letting `set -e` abort so the review evidence is
not lost.

Run verify and treat any non-zero result as a release blocker until explained:

```bash
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <session-id>
```

`verify` checks regular files for size, `sha256:` digest, permission mode, and
modification time. It also checks directory entries as plain directories and
symlink entries by `readlink` target. The command exits non-zero for warning
findings as well as error findings, persisted warning records, soft-delete
records, unresolved persisted target-drift records, artifact problems, and
missing manifests.

Run target drift review when reviewing a published target:

```bash
go run ./cmd/supermover drift list --profile ./supermover.profile.json
go run ./cmd/supermover drift list --profile ./supermover.profile.json --session <session-id> --format json
go run ./cmd/supermover drift record --profile ./supermover.profile.json --session <session-id> --format json
go run ./cmd/supermover drift expire --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<stale review reason>" --format json
go run ./cmd/supermover drift resolve --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<target restored reason>" --format json
go run ./cmd/supermover reconcile plan --profile ./supermover.profile.json --id <persisted-drift-id> --format json
go run ./cmd/supermover reconcile review --profile ./supermover.profile.json --format json
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --id <persisted-drift-id> --apply --reason "<operator repair reason>" --format json
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --all-persisted-planned --apply --reason "<operator repair reason>" --format json
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --record-live --apply --reason "<operator repair reason>" --format json
```

`drift list` derives the target from the profile only and compares published
manifest evidence to the target filesystem. It exits non-zero for drift,
artifact problems, or no published manifest. It does not persist detector
output, acknowledge or resolve drift, mutate review state, run background scans,
repair drift, or prune drifted paths. When report generation succeeds, `report`
runs the same live detector under its independent report surface; compact
`status` now exposes a read-only current profile/target summary over the same
local evidence. `drift record` persists current live detector findings as
durable `.supermover/drift` records; it does not acknowledge, repair, prune,
suppress later live findings, run background scans, or broadly reconcile.
`drift expire` retires stale persisted review evidence without claiming target
restoration; a later redetection of the same logical finding can reopen that
record. `drift resolve` closes an existing persisted record only after a fresh
detector no longer reports drift for the same path and expected baseline. The
`status` contract is current-target only, has no `--session` flag, and keeps
`report --session` as the historical report surface. Use
`--format json` for automation or durable audit capture; text output is compact
operator review output with target-controlled values escaped.
`reconcile plan` is non-mutating and derives source and target only from the
profile. `reconcile review` is also non-mutating and reports persisted plan
readiness, live-only findings that must first be recorded, and planned broad
repair boundaries. `reconcile apply` accepts explicit persisted IDs or
`--all-persisted-planned`, or `--record-live`, and additionally requires
`--apply` and `--reason`; it has no `--target` or `--state-dir` override. The
all-persisted mode still selects only durable persisted planned actions after
review. The record-live mode first persists current live detector findings and
then applies only the resulting persisted planned actions. Current apply support
is limited to missing regular-file restores from published manifest evidence
and the current source file, plus resolving already-restored or already-absent
persisted records.

To record operator acknowledgement for an existing persisted target-drift
record, first capture a persisted drift ID from refused-push `verify`/`report`
JSON or from `drift record` output.
For a disposable release smoke, create a managed-file target drift through the
wired local push path rather than hand-writing `.supermover` artifacts:

```bash
SMOKE_ROOT="$(mktemp -d)"
SRC="$SMOKE_ROOT/source"
DST="$SMOKE_ROOT/target"
PROFILE="$SMOKE_ROOT/supermover.profile.json"
BIN="$SMOKE_ROOT/supermover"
mkdir -p "$SRC" "$DST"
printf 'old\n' > "$SRC/file.txt"

go build -o "$BIN" ./cmd/supermover
"$BIN" profile init --profile "$PROFILE" --source "$SRC" --target "$DST"
"$BIN" push --profile "$PROFILE" --session drift-smoke-one

printf 'new\n' > "$SRC/file.txt"
printf 'operator target edit\n' > "$DST/file.txt"
if "$BIN" push --profile "$PROFILE" --session drift-smoke-two; then
  echo "expected target drift refusal" >&2
  exit 1
fi

if "$BIN" verify --profile "$PROFILE" --session drift-smoke-two --format json > "$SMOKE_ROOT/verify-target-drifts.json"; then
  echo "expected verify to report target_drift evidence" >&2
  exit 1
fi
```

The failed second push leaves a scoped persisted drift artifact under
`$DST/.supermover/drift/`; the ID appears in the saved
`verify-target-drifts.json` `target_drifts` array. The persisted record belongs
to the refused attempt (`session_id=drift-smoke-two`) and carries the published
baseline it compared against in `expected.session_id` and
`expected.manifest_id`; acknowledgement rechecks that published baseline before
writing review metadata. Extract the drift ID manually from the JSON, or with a
local JSON parser:

```bash
DRIFT_ID="$(
  python3 - "$SMOKE_ROOT/verify-target-drifts.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    report = json.load(f)
print(report["target_drifts"][0]["id"])
PY
)"
```

Then acknowledge that persisted ID:

```bash
"$BIN" drift acknowledge --profile "$PROFILE" --id "$DRIFT_ID" --reason "disposable release smoke" --reviewer "release-smoke" --format json
```

Use a persisted ID from refused-push `target_drifts` or from `drift record`
output, not a live-only ID from `drift list` or `report.live_target_drift`.
`drift acknowledge` derives the target from the profile, requires a reason,
rechecks the persisted record against the published receipt/manifest/root
evidence named by its expected baseline, and writes acknowledgement metadata
only. It does not repair target files, resolve drift, suppress live
detector findings, authorize prune, or make `verify`, `report`, `health`, or
`status` clean.

To retire the same disposable persisted record as stale review evidence without
claiming target restoration:

```bash
"$BIN" drift expire --profile "$PROFILE" --id "$DRIFT_ID" --reason "stale review evidence for release smoke" --reviewer "release-smoke" --format json
```

`drift expire` writes `review_state=expired` plus review metadata only. It does
not repair target files, resolve drift, authorize prune, or suppress current
live detector output; current live drift can still keep `verify`, `report`,
`health`, or `status` review-required, and a later redetection can reopen the
record.

To close the same disposable persisted record, restore the target file to the
published baseline and run `drift resolve`:

```bash
printf 'old\n' > "$DST/file.txt"
"$BIN" drift resolve --profile "$PROFILE" --id "$DRIFT_ID" --reason "target restored for release smoke" --reviewer "release-smoke" --format json
```

`drift resolve` rechecks the persisted record against the published
receipt/manifest/root evidence and runs a fresh live detector before writing
`review_state=resolved`. It does not repair target files, rewrite manifests,
authorize prune, suppress future live detector findings, or perform broad
reconcile.

For a persisted missing-file record whose source still matches the published
manifest evidence, the narrow reconcile flow can plan and apply repair:

```bash
"$BIN" reconcile plan --profile "$PROFILE" --id "$DRIFT_ID" --format json
"$BIN" reconcile review --profile "$PROFILE" --format json
"$BIN" reconcile apply --profile "$PROFILE" --id "$DRIFT_ID" --apply --reason "restore missing regular file from source evidence" --reviewer "release-smoke" --format json
"$BIN" reconcile apply --profile "$PROFILE" --all-persisted-planned --apply --reason "restore all persisted planned actions from source evidence" --reviewer "release-smoke" --format json
"$BIN" reconcile apply --profile "$PROFILE" --record-live --apply --reason "record and restore current live planned actions from source evidence" --reviewer "release-smoke" --format json
```

Use `reconcile plan` first because it is non-mutating and shows the selected
action/refusal evidence. Use `reconcile review` to capture the broader
read-only boundary inventory; live-only findings must be durable before apply.
Use `reconcile apply` only with selected persisted drift IDs,
`--all-persisted-planned`, or `--record-live`; it refuses live-only IDs,
missing intent, missing reason, source or published evidence mismatch, unsafe
paths, and unsupported drift classes. The apply path writes durable
started/final receipts under
`.supermover/reconcile/receipts/<receipt-id>.json`; inspect non-applied
receipts through `report` or `status` before rerunning. Refusals include
`conflict_class` and `retry_advice` fields for operator review; the advice is
not automatic background retry. If `repair.drift_recording` is enabled, the
foreground daemon can make live detector findings durable before a later manual
reconcile review/apply, but it still does not apply repair automatically.
If `repair.persisted_reconcile_apply` is enabled, the foreground daemon can
apply only persisted planned actions through the same receipt path using the
profile reason and reviewer. It does not record live-only drift before apply,
does not consume live-only IDs, does not rewrite manifests or authorize prune,
and stops after refused actions or artifact problems for operator review.
Profile validation rejects enabling it with `repair.drift_recording` so
live-only detector findings cannot become implicit daemon apply input.

## Recovery Procedure

`health` exposes the current read-only recovery classifier and local target
review state. It reports interrupted or invalid local sessions, damaged control
artifacts, target-drift records from refused managed updates or `drift record`,
and scoped network transfer outcome artifacts when they exist. The package-level
drift detector exposed by `drift list` is not a `health` scan. It
returns non-zero when operator action is needed:

```bash
go run ./cmd/supermover health --profile ./supermover.profile.json
go run ./cmd/supermover report --profile ./supermover.profile.json
```

Use `health` for the focused recovery classifier and `report` for the broader
read-only operator aggregation. Neither command repairs state. Target-drift
records here are persisted review evidence surfaced through existing review
commands. `health` does not run the live detector; `report` does, under its
independent read-only live drift surface.
Network transfer statuses such as `auth_refused`, `interrupted`,
`needs_repair`, `publish_failed`, and `failed` require operator review and
retry planning; the current `recover` command is not a full network repair
command.

`recover` performs the conservative automated subset. It uses the profile SSOT
to find `target.local_path` and to write any repaired receipt.

```bash
go run ./cmd/supermover recover --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover recover --profile ./supermover.profile.json --session <session-id>
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <session-id>
```

For sessions that only reached `received` or `validated`, use an explicit
rollback decision:

```bash
go run ./cmd/supermover recover --profile ./supermover.profile.json --session <session-id> --rollback-incomplete
```

Preserve the target `.supermover` directory and command output before recovery.
If recovery reports `needs_repair`, do not delete staged session state; inspect
the manifest, receipt, target file, and `session.json` note.

## Soft-Delete Procedure

Physical pruning must be a separate reviewed action. Operators should not infer
that a missing source file authorizes immediate target deletion.

Current review command:

```bash
go run ./cmd/supermover deleted list --profile ./supermover.profile.json
go run ./cmd/supermover report --profile ./supermover.profile.json
```

Current prune dry-run review surface:

```bash
go run ./cmd/supermover prune --help
go run ./cmd/supermover prune --profile ./supermover.profile.json --dry-run
```

`prune --dry-run` keeps the profile as the SSOT, accepts only the command flags
defined by the CLI, requires `delete_policy.mode: prune`,
`delete_policy.require_review: true`, and
`delete_policy.allow_physical_prune: true`, and reads published soft-delete
records for the selected target. Output includes `schema:
supermover.prune_dry_run.v1`, profile/target IDs, the profile delete policy,
candidate/refusal counts, candidate soft-delete evidence, previous manifest
evidence, current target state, and artifact problems. Candidates require
operator review and return exit `1`; an empty clean dry-run returns exit `0`.
Refusals and artifact problems also return exit `1`. Soft deletes inside
`delete_policy.retention_days` are reported as `retention_window_active`
refusals until `detected_at + retention_days * 24h`. The dry-run command does
not delete target files, write approval records, apply an approval, or write
prune receipts.

Current prune approval-authoring surface:

```bash
go run ./cmd/supermover prune approve --profile ./supermover.profile.json --id <approval-id> --soft-delete <soft-delete-id> --reason "reviewed for prune" --reviewer "release-smoke" --format json
```

`prune approve` requires an approval ID, at least one `--soft-delete` ID, a
reason, and reviewer identity. `--approved-by` is an alias for `--reviewer`,
`--expires-at <RFC3339>` can set expiry, and `--format text|json` controls
output. The command reuses fresh dry-run evidence, writes
`.supermover/prune/approvals/<approval-id>.json` plus profile snapshot evidence,
and does not delete target files or write prune receipts. It writes approvals
only when the fresh dry-run has no refusals or artifact problems, and selected
IDs must be current dry-run candidates.

Current prune approval inventory and supersede surfaces:

```bash
go run ./cmd/supermover prune approvals --profile ./supermover.profile.json --format json
go run ./cmd/supermover prune supersede --profile ./supermover.profile.json --id <approval-id> --reason "replaced by newer approval" --reviewer "release-smoke"
```

`prune approvals` is read-only inventory over current-scope approval artifacts.
`prune supersede` updates one existing approval artifact to durable
`status=superseded` review metadata, keeps target files untouched, and does not
write prune receipts or apply prune decisions. Use it when an older approval
should no longer be treated as the current reviewed decision before any apply.

Current reviewed physical-prune apply surface:

```bash
go run ./cmd/supermover prune --profile ./supermover.profile.json --apply --approval <approval-id>
```

Apply requires an existing approval artifact at
`.supermover/prune/approvals/<approval-id>.json`. The approval must bind the
current profile ID, target ID, root ID, approved soft-delete items, current
profile delete policy, profile snapshot ID, and profile snapshot digest. Use
`prune approve` to create that artifact, then keep the approval JSON with the
release evidence.

`prune --apply --approval <id>` writes a durable started receipt before any
target mutation, takes the same target-wide lock used by local push, re-runs
the current prune plan including retention checks, rejects symlinked
approval/receipt/snapshot artifacts, rechecks each approved target immediately
before deletion, deletes only approved file/symlink targets through a
target-root confined filesystem handle whose opened identity must still match
the original target root, syncs the parent directory, and records
applied/partial/failed status in
`.supermover/prune/receipts/<id>.json` when finalization succeeds. If final
receipt writing is interrupted, the durable `started` receipt remains the
operator review evidence. Status `applied` exits `0`; `partial`, `failed`,
interrupted `started`, or invalid approval state exits non-zero and
requires receipt inspection before retry.

Required review evidence:

- source path;
- target path;
- session that detected the deletion;
- previous session and manifest evidence;
- current target state;
- refusal reason when unsafe;
- profile delete policy snapshot;
- approver and approval time.

Use `report` to confirm soft-delete records are visible in the same view as
warnings, health/recovery issues, artifact problems, prune candidates,
refusals, current-scope approval evidence, existing prune receipts, receipt
issues, and migration verification state. `report` returns non-zero when prune
candidates, refusals, authored-but-unapplied approvals, failed, partial, or
interrupted receipts still require operator review. A listed approval is not
physical prune authorization by itself; `prune --apply --approval <id>` still
performs the current policy, profile-snapshot, soft-delete, manifest, expiry,
target-identity, and target-state checks before mutation. An applied receipt is
audit evidence for an existing approval, not automatic release evidence. Use
`deleted list` for the itemized deletion review.

## Discovery And Pairing Procedure

LAN browsing is available as bounded sparse UDP discovery. Profile-backed
source-side encrypted transfer is available through non-dry-run
`push --network`. The foreground `daemon` command can persist
install/status/log/restart/stop lifecycle evidence and run the same serve
behavior under `daemon run --foreground`; it is not an OS service manager,
crash supervision, a detached background process, or continuous sync.
The
`serve` command validates the target profile/root and, for valid pairing-only
profiles, binds a low-information pairing listener: it
exposes discovery, returns pairing bootstrap only after the target-console
verification code is presented, and keeps pairing output untrusted. When the
profile is already paired and has complete `network.receiver_url` plus
`network.local_tls_identity`, `serve` also binds the receiver URL from the
profile and mounts receiver upload routes over pinned mutual TLS. Paired partial
receiver material fails closed before any listener reports ready. `pair`
requires that verification code before writing a local
pairing receipt, profile pins, and profile snapshot. `discover` can emit
untrusted explicit address hints with `--address`; with no source configured it
waits for the requested timeout and returns no hints. `discover browse` listens
for sparse LAN datagram advertisements and reports candidates without trust.
`discover advertise --profile <target-profile>` sends sparse profile-backed
advertisements containing only service, protocol, ephemeral nonce, and minimal
capability flags. `--address` and LAN browse output are operator hint material
and still expose peer address metadata.
The current trust-skeleton sequence is:

```bash
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover discover advertise --profile ./target.profile.json
go run ./cmd/supermover discover browse
go run ./cmd/supermover discover --address 127.0.0.1:9000
go run ./cmd/supermover pair --profile ./source.profile.json --target <address> --verification-code <code> --receipt-out ./exported-receipts
go run ./cmd/supermover profile adopt-pairing --profile ./target.profile.json --receipt-file ./exported-receipts/<receipt-id>.json
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover profile set-network --profile ./source.profile.json --receiver-url https://<receiver-ip>:<receiver-port>
go run ./cmd/supermover push --network --profile ./source.profile.json --dry-run
go run ./cmd/supermover push --network --profile ./source.profile.json --session <network-session-id>
```

Operational rule: discovery output is never an allowlist. It gives address
hints only. Pairing evidence begins after explicit verification writes a receipt
and pins device identity on the source profile. Target-side paired receiver
readiness additionally requires importing that receipt into target
`.supermover/pairings` through `profile adopt-pairing --receipt-file`.
Source-side receiver endpoint selection remains profile SSOT and should be
written with `profile set-network`, not passed as a transient runtime override.
The current source-side dry-run preflight is:

```bash
go run ./cmd/supermover push --network --profile ./source.profile.json --dry-run
```

Pairing report state is not transfer authorization by itself: a valid pairing
receipt only proves the profile pins and receipt agree. `serve` can use that
evidence plus profile-selected receiver URL and TLS identity material to mount
authenticated receiver routes. `push --network` reads the same
profile/pairing/network material, refuses unpaired profiles, mismatched
profiles, and paired profiles that lack that network material. Dry-run stops
there without contacting the receiver. Non-dry-run connects to the pinned TLS
1.3 mTLS receiver, transfers files through the protocol client, and writes
network transfer evidence after receiver begin stores a session. Same-session
reruns can use receiver status as the resume authority for already committed
bytes only when prior network-transfer evidence is auditable, and
published-session reruns retry commit idempotently without reuploading chunks
when the receiver reports the session complete. This is a bounded operator
`push --network` retry behavior, not LAN discovery, daemon sync, or arbitrary
process-kill recovery.

## Incident Response

For any failed or suspicious run, collect:

- profile file used by the command;
- complete target `.supermover` directory;
- command line, stdout, stderr, and exit code;
- source and target filesystem type if a promotion or rename failed;
- warning files and session receipt for the affected session;
- `health --profile ...` output for the affected target when available;
- `.supermover/sessions/<session>/network-transfer.json` when a network attempt
  artifact exists;
- `report` output when available, plus the artifacts it summarizes.

Do not "clean up" warnings, receipts, or manifests before triage. They are the
audit trail.
