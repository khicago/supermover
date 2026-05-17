# Supermover Authority

This page defines where truth lives for the shared knowledge substrate.

## Completion Truth

Completion claims must be checked against all three:

1. original/user-stated requirement
2. current wired CLI/code behavior
3. validation evidence

Do not treat a tested vertical slice as the completed minimum product. For this
repository, the current local/mounted migration slice is implemented, while LAN
agent, encrypted communication, traffic-shape protection, changed-file
incremental sync, compact status UX, and physical prune remain planned or
partially designed. The read-only local migration `report` command is wired,
but it is not a substitute for LAN, transport, daemon status, incremental sync,
or prune support.

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
proposal-only features:

- `f-223nw49qj`: migration audit report UX; `report` is implemented and
  compact `status` remains planned
- `f-224nw98v7`: reviewed physical prune flow
- `f-225nwsa3h`: changed-file incremental local sync
- `f-226nwy2vy`: LAN agent discovery and pairing
- `f-227nw2p2n`: secure resumable transport integration
- `f-228nws66k`: traffic privacy level 2 implementation
- `f-229nwwybc`: failure injection and release hardening

Before implementation, assign the selected feature to `current_tree` or a
worktree and start the first task through feature-tracker commands when local
feature-tracker runtime is available. Do not cite ignored tracker state as the
only auditable evidence for checked-in commits.
