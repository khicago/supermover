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
| `f-226nwy2vy` | LAN agent discovery and pairing. `serve` is wired as a low-information pairing listener and mounts authenticated receiver upload routes only for paired profiles with complete profile-selected network material; `pair` writes local receipt/profile pins after operator verification, and `discover` has low-information explicit address hints; LAN browsing remains planned. Non-dry-run `push --network` now uses profile-backed pinned TLS 1.3 mTLS transfer; dry-run remains preflight-only. |
| `f-227nw2p2n` | Secure resumable transport integration. |
| `f-228nws66k` | Traffic privacy level 2 implementation. |
| `f-229nwwybc` | Failure injection and release hardening. |
| `f-22bnwggww` | Compact local `status` UX over profile/control-plane evidence. Command is wired as a read-only local current profile/target view; release docs/audit polishing remains tracked in the feature. |
| `f-22anw4myc` | Target drift review UX and `drift list` surface. `drift list` remains read-only; `drift acknowledge` is wired for existing persisted target-drift records. |
| `f-22mnwgg64` | Durable live detector drift recording. `drift record` persists current live detector findings as `.supermover/drift` review records; repair/prune/reconcile integration and background scans remain planned. |
| `f-22qnwk3b3` | Persisted drift resolve. `drift resolve` is wired for existing persisted target-drift records after a fresh detector no longer reports the same path and expected baseline; broad reconcile/repair remains planned. |
| `f-233nwduwz` | Narrow persisted drift reconcile. `reconcile plan` is non-mutating for selected persisted drift evidence; `reconcile apply` requires selected IDs, explicit `--apply`, and `--reason`, and currently handles missing regular-file restores from published/source evidence plus already-restored/absent resolve-noop only. Broad automatic reconcile, repair receipts, retry policy, background scans, live-only repair, manifest rewrite, daemon sync, and ongoing sync remain planned. |

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
baseline. `reconcile plan/apply` now covers a narrow selected-ID persisted
drift repair slice: plan is non-mutating, apply requires explicit operator
intent, and repair is limited to missing regular files whose published manifest
evidence still matches the current source plus resolve-noop for already
restored or already absent persisted records. Broad automatic reconcile, durable
repair receipts, a conflict-class taxonomy beyond current refusals, retry
policy, background scans, live-only repair, manifest rewrite, daemon/ongoing
sync integration, and prune integration remain separate planned work.
`drift acknowledge` can add operator acknowledgement metadata only to existing
persisted `.supermover/drift/<id>.json` records, including records created by
`drift record`, surfaced as `target_drifts`.
The wired `status` slice is `supermover status --profile <path>
[--format text|json]` only, with no initial `--session` flag. It is a
read-only current profile/target view over profile SSOT, target `.supermover`
artifacts, and target files needed for verification/live drift detection;
`report --session` remains the historical report surface. It does not include
foreground daemon lifecycle state and does not imply LAN, encrypted transport,
or long-running sync status.
The wired `daemon` slice is foreground-only: `daemon install`, `daemon run
--foreground`, `daemon status`, `daemon logs`, `daemon restart`, and
`daemon stop` persist `.supermover/daemon` install/state/stop-intent/
restart-intent evidence and redacted lifecycle events around existing
profile-backed `serve` behavior. Restart is a foreground intent consumed by a
running daemon process and restarts serve listeners in that same process. OS
service-manager installation, detached background process management, crash
restart supervision, LAN browsing integration, file watching, and ongoing sync
remain future work.

Current feature dependency shape:

- `f-229nwwybc` hardens the current local slice and records separate future
  gates for network features.
- `f-226nwy2vy` first wires `serve`, `discover`, and `pair` command surfaces
  without trusting discovered endpoints. Current `discover` supports explicit
  address hints only, not LAN browsing. Non-dry-run `push --network` now
  transfers through the paired profile-backed mTLS receiver; dry-run does not
  contact the receiver or write target artifacts. Same-session `push --network`
  reruns can resume from receiver status offsets for compatible partial
  receiver sessions only when prior payload-overhead evidence remains
  auditable, and published-session reruns can retry commit without reuploading
  chunks while preserving prior published proof; broad process-kill/interruption
  recovery remains a separate acceptance gate.
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
- `f-233nwduwz` adds a narrow persisted-record `reconcile plan/apply` CLI
  slice. It consumes profile-scoped persisted drift IDs only; there is no
  `--target` or `--state-dir` override. Broad automatic reconcile, repair
  receipts, retry policy, background scans, live-only repair, manifest rewrite,
  daemon/ongoing sync, and drift-to-prune integration remain future work.
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

- Low-information discovery currently supports explicit-address hints; LAN
  browsing remains planned.
- Verification-code pairing with persistent pinned device identity is wired;
  identity generation, rotation, revocation, and broader lifecycle UX remain
  planned.
- Profile-backed pinned TLS 1.3 mTLS is wired for current `serve` and
  non-dry-run `push --network`; foreground daemon lifecycle state is wired
  around `serve`; OS service-manager and LAN-browsing integration remain
  planned.
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
  reconcile/repair, repair receipts, background scans, and prune integration
  remain planned.
- Recover and prune commands are partly wired: local recovery, `prune approve`
  approval-artifact authoring, `prune approvals`, focused read-only
  `prune review`, `prune supersede`, and reviewed `prune --apply --approval
  <id>` exist, while broader network recovery remains planned.

## Phase 7: Quality Bar

- Failure-oriented integration tests.
- Security and recovery documentation.
- CI, contribution guide, security policy, and release process.
