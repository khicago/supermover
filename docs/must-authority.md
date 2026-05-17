# Supermover Authority

This page defines where truth lives for the shared knowledge substrate.

## Completion Truth

Completion claims must be checked against all three:

1. original/user-stated requirement
2. current wired CLI/code behavior
3. validation evidence

Do not treat a tested vertical slice as the completed minimum product. For this
repository, the current local/mounted migration slice is implemented, including
managed local regular-file updates from previous Supermover evidence.
`serve` is wired as a target listener with two profile-governed surfaces: a
low-information pairing listener for help, usage validation, target
profile/root validation, discovery, target-console verification code, and
verified pairing bootstrap; plus an authenticated receiver listener only when
the profile is already paired and has complete profile-selected network
material. `pair` is wired to require that verification code, write a durable
local pairing receipt, update profile target pins, and record a profile
snapshot. `discover` is wired for low-information explicit address hints only;
`--address` is operator-provided hint material and still leaks peer address
metadata. It does not browse LAN services or establish trust by itself.
Source-side non-dry-run `push --network` is wired for paired profile-backed
pinned TLS 1.3 mTLS transfer through the receiver protocol, and the current
operator path supports traffic privacy level 2 only. Its dry-run mode is still
preflight-only: it validates profile, pairing, network material, local TLS
identity, source scan, and manifest shape without contacting the receiver or
writing target artifacts. Operational LAN agent browsing, daemon behavior,
ongoing network sync, and broad operator resume/recovery acceptance remain
planned or partially designed. `drift record` is wired to persist current live
detector findings as durable `.supermover/drift/<id>.json` review records.
`drift acknowledge` is wired for existing persisted drift records, including
records created by `drift record`, surfaced as `target_drifts`; `drift
resolve` is wired for existing persisted drift records after a fresh detector
no longer reports the same path and expected baseline. Broad drift
reconcile/repair, drift-to-prune integration, and background scans remain
planned or partially designed.
The read-only local migration `report` command, read-only `drift list`
detector, compact local `status` command, `drift record`, `drift resolve`,
`prune --dry-run`, `prune approvals`, focused read-only `prune review`,
`prune approve`, `prune supersede`, and `prune --apply --approval <id>` are
wired, but they are not substitutes for LAN, daemon status, ongoing sync,
broad drift reconciliation, drift repair, background scans, or
drift-to-prune integration.

## Primary Status Sources

- `README.md`: public current capability summary.
- `docs/release-audit.md`: current checkpoint, validation gates, known planned
  surface, and safety notes.
- `docs/v1-scope.md`: product boundaries and implemented/planned command split.
- `docs/plan.md`: high-level v1 phase map.
- `.bagakit/feature-tracker`: optional local feature lifecycle state. It is
  useful during execution but is ignored from Git; checked-in docs must carry
  any status summary that future clones need to audit.

## Shared Checked-In Knowledge

- root:
  - `docs`
- `AGENTS.md` is only the bootstrap layer. `.bagakit/knowledge_conf.toml` may
  declare the same root for local Bagakit operators, but `.bagakit` is ignored
  runtime state and is not the only checked-in authority.

## Runtime Roots Declared By Protocol

- researcher:
  - `.bagakit/researcher`
- selector:
  - `.bagakit/skill-selector`
- evolver:
  - `.bagakit/evolver`

## Rules

- shared durable project knowledge belongs under the shared root
- current wired behavior wins over design prose when they differ
- release/status claims must cite `README.md`, `docs/release-audit.md`, or
  directly inspected code/CLI output
- path-local `AGENTS.md` may narrow execution guidance, but must not redefine the
  shared knowledge root
- shared pages, managed bootstrap text, and durable examples use repo-relative
  paths only
- absolute filesystem paths are forbidden in durable shared surfaces
- if one imported reference needs a durable handle, prefer a short opaque id
  such as `k-2ab7qxk9`
- do not carry forward timestamp-derived names, raw source file names, raw
  source file contents, or raw source-path/action-time metadata into shared
  knowledge pages
- research runtime is not shared knowledge by default
- evolver memory is not shared knowledge by default
- selector runtime is not shared knowledge by default

## Current Feature Tracker Caveat

The original broad feature tracker topic began as a full v1 plan and is now
archived as historical evidence. Active remaining work is split into narrower
features:

- `f-223nw49qj`: migration audit report UX; `report` is implemented
- `f-224nw98v7`: reviewed physical prune flow
- `f-225nwsa3h`: changed-file incremental local sync; implemented and archived
- `f-226nwy2vy`: LAN agent discovery and pairing
- `f-227nw2p2n`: secure resumable transport integration
- `f-228nws66k`: traffic privacy level 2 implementation
- `f-229nwwybc`: failure injection and release hardening
- `f-22bnwggww`: compact local status UX; `status` is wired as a read-only
  local profile/target evidence command
- `f-22anw4myc`: target drift review UX
- `f-22mnwgg64`: durable live detector drift recording; `drift record` is
  wired as evidence persistence only
- `f-22qnwk3b3`: persisted drift resolve; `drift resolve` closes existing
  persisted drift records only after a fresh detector no longer reports the
  same path and expected baseline

Dependency notes:

- secure resumable transport depends on pairing identity from
  `f-226nwy2vy`
- traffic privacy level 2 depends on secure transport from `f-227nw2p2n`
- release hardening must close the current local and profile-backed network
  gates and leave explicit future gates for LAN browsing, daemon behavior,
  ongoing sync, and broader operator recovery/privacy release acceptance

Before implementation, assign the selected feature to `current_tree` or a
worktree and start the first task through feature-tracker commands when local
feature-tracker runtime is available. Do not cite ignored tracker state as the
only auditable evidence for checked-in commits.
