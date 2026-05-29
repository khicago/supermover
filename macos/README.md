# SuperMover macOS App

This directory contains a native macOS operator shell for SuperMover.

The app name is `SuperMover`. It is a front-end over the existing CLI and
target-side `.supermover` evidence. It does not introduce a second execution
engine.

## Current Scope

The mac app is currently a CLI-backed operator workbench, not yet the complete
app-first two-machine LAN migration product. The implemented app-first
foundation now covers:

- role selection for source, target, and observer workflows
- app-guided profile selection/create with friendly selected-profile labels in
  high-level workbench surfaces, advanced raw-path editing, source root, and
  target root setup
- profile init, lint, and target update actions through the CLI
- profile-backed network receiver/TLS material authoring through
  `profile set-network`
- source-side low-information discovery browse, explicit address hint checks,
  verification-code pairing plus source-side pairing receipt export through the CLI
- target-side bounded low-information discovery advertise and `serve`
  supervision through the CLI, plus target-side pairing receipt import through
  the CLI
- role-scoped foreground process supervision for `serve`, `dashboard`, and
  source-side sync loops/watchers
- foreground daemon install/run/restart/stop/status/log controls that write
  daemon lifecycle evidence and intents without claiming an OS-managed service
- sync queue status/list/ready read-only views for every role, source-owned
  queue enqueue/cancel/fail, bounded local/network sync runs, discovery-gated
  network run, and supervised foreground local/watch/network sync loops
- typed `verify --format json` comparator evidence for target-vs-published
  manifest checks, with Merkle/root proof shown as unavailable rather than
  inferred from profile root ids
- a native Evidence Vault with typed `verify`/`status`/`report`/`health`
  cards, raw JSON envelope history, CLI exit/freshness metadata, explicit
  target-vs-manifest alignment scope, searchable target `.supermover` artifact
  catalog over the manually selected Target Root field, malformed/symlink
  artifact problem surfacing, and evidence-bound review-metadata actions.
  Vault-side prune approval is single-candidate only.
- stale setup-context handling so old process output is not promoted as current
  green state
- structured app events and visible artifact-reader problems for expected JSON
  command output
- CLI provenance/readiness display for packaged versus development launch mode,
  including bundled manifest fields when present

The app wraps the following wired command surface:

- version
- profile lint
- profile init, profile set-target, profile set-network, and profile adopt-pairing
- local dry-run and publish
- verify JSON comparator, report, status, recover dry-run
- drift record, drift acknowledge, drift expire, drift resolve, reconcile plan/apply
- prune review, prune approvals inventory, prune approve, and prune supersede
- discover address, discover browse, discover advertise, and pair
- network push dry-run and bounded network push
- sync queue enqueue/status/list/ready/cancel/fail
- sync run, sync loop, sync watch
- sync network run, sync network discover-run, and sync network loop
- foreground `serve`
- foreground daemon install/run/status/logs/restart/stop
- loopback-only `dashboard`

The app intentionally does not yet claim:

- Merkle or root-comparison verification
- detached background sync
- OS-managed daemon installation
- OS-managed ongoing incremental sync
- broad automatic repair
- vault-side target mutation, transfer execution, pairing, publish, prune
  apply, reconcile apply, or Merkle/current-source proof
- notarized two-machine distribution readiness

For a real cross-device migration today, the CLI remains the authoritative
execution surface. The app can now launch and inspect the wired discovery,
pairing, serve, bounded transfer, sync queue/run/loop/watch/network,
verification, review commands, and the native Evidence Vault. It is not yet
sufficient as the only operator interface for a large LAN migration because
final two-machine acceptance evidence is still incomplete, the same-machine
acceptance harness now records a real CLI-generated `source.consistency.json`
proof while distinct-machine/operator-gated acceptance still fails closed on
any missing proof or release-readiness gate, and the vault intentionally
excludes target mutation and transfer execution.

## Build

From the repository root:

```bash
cd macos
swift build
swift run SuperMoverApp
```

The development app resolves the repo root automatically and supervises a
build-and-exec launch path: it builds the CLI to
`.tmp/macos-app/supermover-dev`, then `exec`s that binary in the same foreground
process. It does not require `cd` into the repository root before launch.

To build a packaged local app and record packaging evidence:

```bash
macos/script/build-app.sh
macos/script/audit-app.sh macos/dist/SuperMover.app > macos/dist/SuperMover.app.audit.json
```

The audit command emits JSON and exits nonzero when distribution evidence is
blocked. That is expected for unsigned, ad-hoc signed, dirty, or unstapled local
builds. Keep the JSON as review evidence; do not treat it as notarized
distribution readiness unless it passes with Developer ID signing, clean
provenance, Gatekeeper assessment, and stapler validation.

To drive a notarization/stapling pass with durable JSON evidence:

```bash
SUPERMOVER_CODESIGN_IDENTITY="Developer ID Application: Example Corp (TEAMID1234)" \
sh macos/script/build-app.sh

SUPERMOVER_NOTARY_KEYCHAIN_PROFILE="supermover-release" \
sh macos/script/notarize-app.sh \
  --app macos/dist/SuperMover.app
```

`script/notarize-app.sh` is fail-closed. It blocks when:
- no notary credentials are configured
- multiple auth modes are configured at once
- archive / submission / notary-log / stapler steps fail
- `notarytool submit` output is malformed, including a missing or non-UUID
  Apple submission id
- `notarytool log` output is malformed, does not report `status=Accepted`, or
  has an `issues` field that is neither absent/null nor an array
- post-staple `script/audit-app.sh` is still blocked
- on both pass and blocked exits it persists the structured result into
  `macos/dist/SuperMover.app.notary/notarization.json`, which is the same
  durable sidecar consumed by installed-app acceptance packaging/evaluation
  gates. It also persists the referenced post-staple audit next to that
  sidecar, so currentness does not depend on a custom temporary `--work-dir`
  surviving later cleanup. App-side acceptance also verifies that the sidecar's
  own `app_path`, canonical sibling `post-staple.audit.json`, and referenced
  post-staple audit contents still match the current packaged app and bundled
  provenance before it treats local notarization as current release evidence.
  Installed-app acceptance also requires that sidecar to retain a supported
  `auth_mode` (`keychain_profile`, `api_key`, or `apple_id`), a UUID-shaped
  Apple notary submission id, a missing/null `failure` record, and notary log
  path. App-side packaging collection and shell collection preflight refuse
  sibling `notary-log.json` that does not decode as accepted notary log JSON,
  and the bundle-local copied `*.notary-log.json` must retain that accepted
  shape. When stale notarization evidence is cleared, ordinary files and
  dangling symlink leaves are both removed, and shell plus app-side packaging
  collection refuse to overwrite existing bundle-local version, provenance,
  app-audit, notarization, or notary-log output leaves unless they are
  single-link regular files;
  `status=pass` plus
  `submission.status=Accepted` alone is review material, not release-ready proof.

This is now the repository’s first-class notarization/stapling script surface,
but it still depends on operator-supplied Apple credentials and a real
Developer ID identity. A passing local script run is release evidence; it is
not a substitute for the separate two-machine acceptance bundle.

## Implementation Notes

- The app uses `Process` to run the bundled `supermover` binary, or a
  supervised development launcher that builds and then `exec`s
  `.tmp/macos-app/supermover-dev` when no packaged CLI is present.
- High-level workbench surfaces and the generic CLI preview show the selected
  profile label instead of echoing the raw absolute profile path. Advanced
  setup retains the editable raw path because the profile file remains the
  SSOT for CLI execution and evidence authoring. Default draft ids no longer
  dominate the primary summary, and source profile identity authoring only
  surfaces when the operator is creating a new source profile. The Prepare page
  now separates role choice, migration config selection, root inputs for
  create/update actions, and lint/status checks so selecting an existing config
  cannot be confused with choosing a new config destination.
- Settings separates local Display Preferences from Command Inputs. Appearance
  and interface-language preferences are stored as UI-only defaults and applied
  at the app window; they do not alter profile files, CLI arguments, command
  previews, evidence bundles, or CLI stdout/stderr. The language switch now
  localizes bounded app-owned chrome such as the sidebar, Settings page,
  Display Preferences controls, and Prepare/setup role guidance. Task titles,
  command previews, evidence/proof values, artifact fields, and CLI output
  intentionally remain raw and audit-stable.
- The packaged `.app` bundles a built `supermover` binary under
  `Contents/Resources/bin/supermover`, writes
  `Contents/Resources/supermover-provenance.json`, and prefers that binary at
  runtime. The provenance manifest records the build profile, git commit,
  dirty-worktree bit including non-ignored untracked files, CLI version, bundled
  CLI path, build time, and signing mode. A packaged app must use the bundled
  CLI; if the bundle is missing `Contents/Resources/bin/supermover`, it fails
  clearly rather than falling back to the development launcher.
- Install Readiness treats missing, malformed, or incomplete packaged
  provenance as blocked. Unsigned, ad-hoc signed, or dirty packaged provenance
  is review-only local evidence, not a green source/target installation claim.
  `script/audit-app.sh` is the local release-engineering gate for signing,
  Gatekeeper, stapler, plist, provenance, CLI, git, and hash evidence; manifest
  text alone is not signing proof. When the canonical sibling
  `<AppName>.app.notary/notarization.json` sidecar is present, that audit now
  also fails closed if the sidecar is malformed or if its referenced
  post-staple audit/provenance no longer matches the current packaged app. Its
  sidecar `release_ready` bit also requires the same strict fields that
  installed-app acceptance requires: supported `auth_mode`, UUID-shaped Apple
  submission id, `failure` absent/null, accepted sibling notary-log JSON, and a
  post-staple `distribution_ready` audit.
- Long-running foreground tasks such as `serve`, `dashboard`, `sync loop`,
  `sync watch`, `sync network loop`, and `daemon run --foreground` remain
  foreground tasks in the app. Stopping them terminates the supervised process;
  daemon Stop/Restart buttons separately write scoped CLI stop/restart intents.
  During development builds, the launcher traps termination and asks the active
  `go build` child to stop; after `exec`, the actual CLI receives termination
  directly. Final stdout/stderr is drained before JSON evidence is promoted.
- `Info.plist` includes a Local Network usage description for explicit
  discovery, pairing, serve, and profile-pinned migration commands. The app also
  ships signing entitlements for network client/server and user-selected
  read-write file access. These are packaging intent, not proof that the user
  has approved macOS prompts or firewall rules.
- The custom titlebar chrome now also enforces the workbench minimum content
  size at the `NSWindow` layer and clamps any restored undersized frame back to
  the minimum before operators interact with the app, so a previously saved or
  transient small window does not leave the workbench narrower than its
  sidebar-plus-detail layout.
- `script/build-app.sh` builds the release app and CLI bundle. Set
  `SUPERMOVER_CODESIGN_IDENTITY` to a Developer ID identity, or `-` for ad-hoc
  local signing. When an identity is set, the script signs the nested bundled
  CLI first and then signs the app bundle without using signing-time `--deep`.
  `script/notarize-app.sh` then packages the signed app, submits it through
  `xcrun notarytool`, retrieves the notarization log, staples the ticket, and
  reruns `script/audit-app.sh`. It refuses malformed submit output, including
  non-UUID submission ids, before fetching logs or stapling, and refuses
  non-accepted notary logs before stapling, including malformed `issues`
  fields. Before that
  post-staple audit, it clears any previous sibling sidecar result so stale
  currentness evidence cannot block overwriting with fresh notarization output.
  Notarization still requires operator-provided Apple credentials and should be
  preserved with
  stapler/audit evidence before claiming distributed two-machine install
  readiness.
- The app does not create or store new key material. Pairing and transport
  secrets remain owned by the existing profile and `.supermover` control-plane
  artifacts.
- Discovery-gated sync treats LAN discovery as an availability gate only. The
  selected candidate must match the profile `network.receiver_url`; pairing and
  mTLS profile pins remain the trust boundary.
- Structured review commands prefer current JSON output where it already exists.

## Acceptance Checks

Run the packaged loopback acceptance after building the app:

```bash
macos/script/acceptance-loopback.sh
```

To reuse an already built app without rebuilding:

```bash
SUPERMOVER_ACCEPTANCE_SKIP_BUILD=1 macos/script/acceptance-loopback.sh
```

The script uses the bundled CLI inside `macos/dist/SuperMover.app`, checks that
packaged provenance matches the bundled CLI version and expected bundle path,
fails closed if the bundled CLI surface is stale for required packaged
acceptance commands such as `serve --ready-file`,
records `app-audit.json` from `script/audit-app.sh`,
migrates a disposable local source/target with a dot-prefixed file,
dot-directory payload, zero-byte file, and a larger regular file, checks
verify/status/report/health, exercises bounded local sync queue/run, and
confirms unpaired `push --network --dry-run` fails closed without changing
target files or control-plane evidence, then runs a same-machine packaged
pairing plus non-dry-run localhost mTLS transfer and preserves the resulting
receiver-side `network-transfer.json` evidence. It prints the preserved
evidence directory on success or failure. This is still local packaged-app
evidence only; unsigned, ad-hoc, dirty, or unstapled audit output is
review-only/blocking and it is not notarized two-machine LAN or Local
Network/firewall approval evidence.

For real two-machine installed-app collection, use:

```bash
sh macos/script/acceptance-two-machine.sh --help
```

The two-machine harness is phase-oriented and currently expects:

- `target-serve`: pairing-only target serve that emits durable ready evidence
- `source-pair`: source pairing with `pair --receipt-out`
- `target-import`: target-side `profile adopt-pairing --receipt-file`
- `target-serve`: paired target serve that emits receiver readiness
- `source-transfer`: source-side `profile set-network`, network dry-run, and
  non-dry-run `push --network`
- `evaluate`: evidence collection only

Each installed-app phase now also preflights the bundled CLI help surface it
depends on and exits `5` if the packaged app is stale for that role, rather
than letting phase execution fail later with weaker evidence.
In default `collection.mode=two_machine`, each installed-app phase also records
`source.app-audit.json` or `target.app-audit.json` first and exits `5` before
phase execution if that machine is not install-ready (`status=pass`,
`summary.pass_ready=true`, and `readiness=distribution_ready`). A `review_only`
audit is still packaging evidence to fix, not install-ready proof. This is
deliberate: ad-hoc,
dirty, unstapled, or otherwise blocked builds are not valid real two-machine
installed-app evidence substrates. The explicit `same_machine` harness remains
allowed as packaged-app wiring evidence only.
For the same real two-machine lane, the native app's installed-app launch
preview now also blocks when the current packaged app's local sibling
notarization evidence is missing or not release-ready, because shell phase
preflight would fail closed on that same current-app condition before phase
execution.
The Acceptance Bundle panel and bundle-load note/event now also derive their
top status from the recomputed workflow summary instead of trusting raw
`meta.status`, so a stale bundle that still has pending `next_actions` stays
in `review` even if an older `evaluation.json` already wrote
`status=evidence_collected`. That same currentness rule now also requires a
current bundle-local `evaluation.json` before the bundle can look complete: a
bare `meta.status=evidence_collected` no longer suppresses the final
`evaluate` action, and a strict two-machine lane also reopens `evaluate` when
the preserved evaluation artifact was written without
`require_operator_evidence=true`. Manual Evidence gate chips and missing-proof
facts likewise now follow the current evaluation mode instead of stale stored
evaluation metadata. Installed-app launch preview likewise stays in `review`
until that current strict evaluation both exists and still matches the current
phase/operator proof inputs. Preview/preflight only allow the matching
corrective launch when that reopened step is the sole remaining strict next
action. If multiple required steps reopen, or the reopened step is different
from the requested launch, preview/preflight stay blocked instead of
continuing to trust the stale `evaluation.json`, even after release evidence
and distinct-machine installed-app proof are otherwise ready.
Installed-app launch preview/preflight also stays blocked when the other
machine still lacks release-ready packaging evidence. A locally ready source
or target app cannot bypass missing `source.app-audit.json` /
`source.notarization.json` or `target.app-audit.json` /
`target.notarization.json` on its counterpart before phase execution.
When the current installed-app proof summary instead says the bundle needs
machine-identity correction, including missing role/machine-facts evidence, the
app still keeps target `serve` and source `pair` launchable as corrective tasks
so they can rewrite the matching role/machine-facts evidence the workflow
summary asks for; unrelated acceptance tasks remain blocked before packaging
evidence writes until the bundle is repaired.
Those corrective app writes now update the canonical `source.machine.json` /
`target.machine.json` artifacts plus `meta.json` `roles.*` and
`evidence.machine_facts.*` entries from the current installed app, matching
the shell phase authoring contract on that proof subset. This does not
substitute for the remaining verified bundle handoff, operator evidence, or
release-grade notarization / Gatekeeper gates.
The shared bundle also records per-machine `source.app-audit.json` and
`target.app-audit.json` so installed-app collection preserves packaging
evidence alongside transfer evidence; blocked local audit results remain local
review evidence, not distribution readiness.

The shell substrate no longer requires a shared writable `--bundle-root`
between the two Macs. The currently wired handoff flow is:

```bash
# on one machine
sh macos/script/acceptance-two-machine.sh pack-bundle \
  --bundle-root <local-bundle> \
  --archive <bundle.tgz>

# move <bundle.tgz> plus its generated <bundle.manifest.json> to the other machine, then there:
sh macos/script/acceptance-two-machine.sh unpack-bundle \
  --archive <bundle.tgz> \
  --manifest <bundle.manifest.json> \
  --bundle-root <incoming-bundle>

sh macos/script/acceptance-two-machine.sh merge-bundle \
  --bundle-root <current-bundle> \
  --incoming-bundle-root <incoming-bundle>

sh macos/script/acceptance-two-machine.sh workflow-status \
  --bundle-root <current-bundle>
```

`pack-bundle` now also emits a checksum manifest next to the archive, including
the exporting machine id/label already present in the bundle roles when that
local evidence was collected. `unpack-bundle` fail-closes if the manifest is
missing, malformed, refers to a different archive, the archive digest has
changed in transit, or the archive/staged tree contains symlinks, hardlinks, or
special files. Only after those checks does it record the importing machine
id/label into the verified handoff ledger in `meta.json` under
`.evidence.bundle_handoffs`.
`merge-bundle` validates the incoming bundle root before parsing incoming
metadata, rejects symlinks, hardlinks, and special files, and publishes staged
artifacts only after artifact, metadata, and `target.ready.json` conflicts have
been preflighted, so a rejected merge does not leave novel half-merged files in
the destination bundle for those preflightable conflicts. Publish-time artifact
copy failures before the final `meta.json` replacement also roll back newly
published artifact files and empty directories.
Operator-gated evaluation only treats that ledger as sufficient when it proves
a verified export/import crossing between the same source/target machine ids
already recorded in the acceptance bundle. A same-machine or wrong-machine
pack/unpack replay remains wiring evidence only.

`workflow-status` is a read-only coordination surface. It summarizes which
evidence-backed steps are already present in the bundle and what the next
source/target local action should be before the next handoff or final
evaluation. `installed_app_proof_ok` remains the proof-specific readiness bit;
the top-level `ok` bit only turns true after `evaluate` has already recorded
`evidence_collected` and no further `next_actions` remain.

With `--require-operator-evidence`, `workflow-status` now orders that advisory
surface as release packaging evidence first, then installed-app collection /
handoff correction, then `evaluate` before it falls back to older phase
reminders. Those legacy phase actions only come back after the strict
proof/release gates are already satisfied or the bundle is already
`evidence_collected`.

For older bundle metadata that omitted explicit `source_pair` /
`source_transfer` subpaths, the app and shell advisory surfaces now also read
the canonical bundle artifacts (`source.pair.json`, `source.pair.txt`,
`source.transfer.json`, `source.network-push.txt`, `source.verify.json`,
`source.status.json`, `source.report.json`, `source.health.json`,
`source.consistency.json`, and `source.baseline.json`) before deciding that
those lanes are still missing.

This is still only an installed-app evidence-handoff substrate. It does not by
itself prove distinct-machine LAN discovery, Local Network/firewall prompts,
operator pairing confirmation, or release-ready notarized distribution.
The native app Evidence Vault can now load that same bundle and record manual
operator evidence directly into `meta.json` under the shared `.evidence.operator`
contract for `local_network`, `firewall`, and `pairing_confirmation`, using the
same `.meta.lock` lock protocol as the harness. This is a durable recording
surface for real-device evidence, not an automatic acceptance pass. In the
strict two-machine lane, those pass records must also carry the current
machine-facts binding: Local Network and firewall evidence bind to
`target.machine.json`, and pairing confirmation binds to `source.machine.json`.
The shell recorder derives that `machine_id` from the canonical machine-facts
artifact and refuses strict pass records before machine facts exist; the app
uses the loaded bundle snapshot for the same binding.
The same Evidence Vault now also supports writing current app-side phase
artifacts into the bundle for `source.browse.json`, `target.advertise.json`,
`target.ready.json`, `target.ready.phase-<n>.json`, `source.pair.json`, and
`source.transfer.json`. App-side `source pair` also stages the validated
durable local receipt into `exported-receipts/<pairing_receipt_id>.json` and
records `source.pair.json.receipt_path` against that bundle-relative path, so
the authored bundle can replay through shell `target-import` after
pack/unpack/merge. Existing app-authored output leaves for machine facts,
discovery, target-ready, exported receipts, source-pair/source-transfer, phase
transcripts, source consistency, and evaluation must be single-link regular
files before the app overwrites them, so unsafe linked phase artifacts block
before partial bundle evidence or `meta.json` updates are published. When the current successful app run has retained stdout, the
app-side bundle authoring path also preserves `source.pair.txt`,
`target.adopt-pairing.txt`, and `source.network-push.txt` so the bundle keeps
phase transcripts alongside the JSON artifacts. These writes use current app
snapshots plus current profile SSOT where needed; they do not synthesize
missing CLI execution or replace target-side evaluation. This is still current
surface / receipt-path parity only, not real two-machine installed-app
acceptance proof.

App workflow summary now validates that staged exported receipt as a
single-link regular bundle artifact with a valid pairing-receipt schema before
advancing `source_pair` or `target_import`. `target_import` must also record a
non-empty `target_import.adopted` transcript path such as
`target.adopt-pairing.txt`, and shell/app advisory plus final evaluators require
that referenced transcript to remain a bundle-local single-link regular
artifact. Shell final `evaluate` also checks
required bundle-local proof artifacts as single-link regular files before
reading them, and shell plus Swift final evaluation now reject malformed
`source.status.json` / `source.health.json` transfer evidence instead of
accepting file presence. Shell `workflow-status`, shell final `evaluate`, the
Swift app workflow summary, and Swift final evaluation apply the same typed
transfer-proof checks, including non-negative Swift `Int`-decodable integer
readiness counters, before advancing `source_transfer` to `evaluate`. They
also require `source.verify.json`, `source.report.json`, `source.status.json`,
and `source.health.json` to carry present, normalized `target_root` evidence
for the selected/evaluated target root; after `evaluation.json` exists, shell
`workflow-status` reopens `source_transfer` if those source-side proof roots
are swapped to another target. Shell `workflow-status` and shell final
`evaluate`, plus Swift workflow/final evaluation, now also require canonical
`target.ready.json` to match `meta.json` `evidence.target_ready` before
target-serve readiness counts. Missing or malformed target-ready artifacts
reopen `target_serve_phase_1` or block `evaluation.json` instead of trusting
meta text alone. Once target readiness is valid, source-pair evidence must name
the same target address, and source-transfer evidence must name the same target
address, target mode, and receiver endpoint before the bundle can advance. A
pairing-only ready artifact keeps target serve visible as done, but transfer
does not count until canonical `target.ready.json` also proves receiver transfer
readiness with `receiver_address`, `receiver_routes=true`, `push_network=true`,
and `transfer=true`.
Shell `workflow-status` and shell final `evaluate` also reject
malformed optional `source.browse.json` / `target.advertise.json` discovery
artifacts instead of treating trusted/status bits alone as enough. Shell bundle
bootstrap rejects
symlinked/hardlinked/non-regular `meta.json`, Swift bundle access applies the
same rule to `meta.json`, and shell `workflow-status` uses that policy before
reading machine-facts and release-evidence artifacts. This prevents hardlinked
or malformed bundle artifacts from appearing greener than final evaluation.

The shared Swift script-test harness drains stdout and stderr concurrently
while child processes run. The two-machine script tests, app-audit/notarize
script tests, and skip-gated acceptance integration script launches reuse that
harness, so larger acceptance scripts cannot deadlock on pipe buffers before
their JSON evidence and stderr diagnostics can be captured.

The app can also record machine-local packaging evidence into the same bundle:
`source.version.txt` / `target.version.txt`,
`source.provenance.json` / `target.provenance.json`, and
`source.notarization.json` / `target.notarization.json`, and
`source.app-audit.json` / `target.app-audit.json`. This uses the current
packaged app resources plus local `script/audit-app.sh` execution and any
machine-local notarization evidence already present, and updates the shared
`.evidence.app_audit.<machine>` contract. It is local packaging evidence, not
release proof by itself.

This five-phase shape has been verified locally as a same-machine packaged-app
simulation. That is useful wiring evidence, but it is still not real
two-machine installed-app LAN acceptance, Local Network/firewall prompt
evidence, or notarized distribution readiness.
The app-side acceptance evaluator has also been replayed against a real
same-machine harness bundle, after deleting the harness-written
`evaluation.json`, to confirm the app can regenerate final evaluation from the
bundle plus target `.supermover` control-plane evidence alone.
The same built-app integration path now also covers the `AppStore` acceptance
actions for `Write Packaging`, `Write Import`, and `Write Evaluation`, against
a real same-machine bundle. The test deletes the harness-written packaging,
`target_import`, and `evaluation` artifacts first, then rewrites them through
the app surface, so the current app-first surface has local smoke evidence
above the lower-level coordinator tests.

For the local same-machine five-phase orchestration itself, use:

```bash
sh macos/script/acceptance-two-machine-same-machine.sh
```

This local orchestration reuses one migration profile identity across the
source/target profile files so `profile adopt-pairing --receipt-file` can
exercise the intended trust closure on one machine. Treat it as bundle-shape
validation, not as evidence about separate-operator profile authoring.
- Discovery browse/address/advertise output is parsed as untrusted
  low-information evidence. Pair remains text output plus durable profile and
  target control-plane evidence; run Status or Report after pairing to confirm
  pins and receipts.
- Missing or malformed JSON stdout for structured command surfaces is recorded
  as an app-visible artifact-reader problem instead of being silently ignored.
- `dashboard` and `serve` readiness still come from current stderr protocol
  lines; this is a temporary adapter layer, not a claim that stderr is the
  ideal long-term app API.

```bash
./script/build-app.sh
./script/audit-app.sh dist/SuperMover.app > dist/SuperMover.app.audit.json
open dist/SuperMover.app
```
