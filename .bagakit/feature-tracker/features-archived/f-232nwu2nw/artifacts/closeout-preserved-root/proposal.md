# Feature Proposal: f-232nwu2nw

## Why

Repeated `push` can move changed snapshots, but v1 calls for ongoing
incremental synchronization with durable queueing, status, and recovery across
long-running operation.
The current repository now has the first execution and visibility slices:
durable queue state, one bounded local `sync run` pass, foreground local
`sync loop` polling, profile-enabled foreground-daemon local polling sync,
foreground OS watcher `sync watch`,
one bounded profile-backed `sync network run`, foreground profile-backed
`sync network loop`, one bounded LAN-discovery-gated
`sync network discover-run`, profile-enabled source-side foreground-daemon network
polling sync,
read-only per-entry queue listing, explicit operator failed-terminal queue
evidence, and report/status aggregation for queue/run receipts. That is still
narrower than automatic LAN discovery endpoint selection, detached service
management, and broad repair.

## Goal

Add durable changed-file queueing and long-running incremental synchronization
with receipts, status, backoff, and cancellation after daemon lifecycle and
network recovery foundations are stable.

## Implemented / Planned / Missing

| Status | Detail |
| --- | --- |
| Implemented | Operators can run repeated push operations; status/report/recover expose run evidence. Durable `sync queue` can enqueue/status/list/ready/cancel/fail profile-scoped changed-file entries, including hidden files and dot-directories. `sync queue list` exposes all persisted queue entries with lifecycle fields for read-only per-entry review. `sync queue fail` records explicit failed-terminal operator review evidence for one observed queue entry without retrying, repairing, or claiming target restoration; a later changed source observation reopens the path as queued work. `sync run` snapshots the profile into the queue, marks ready entries `in_flight`, publishes once through the existing local push safety path, writes durable run receipts, marks clean entries `done`, records retry backoff on refused publish, and requeues stale in-flight entries before a new pass. `sync loop` repeats that same local pass in the foreground with generated session IDs, context-aware waits, and optional `--max-runs`. `sync watch` arms recursive OS watchers for existing source directories, writes one baseline local run receipt, coalesces OS file events into additional local queue/run passes, and supports bounded `--max-events` smoke runs. `sync network run` validates profile trust, network material, local TLS identity, and the current network push contract before queue mutation, writes ordinary queue/run receipts under the profile-selected target control plane, publishes ready queue entries through a per-entry profile-backed mTLS network manifest, carries previous published manifest evidence for regular-file replacements, marks clean entries `done`, records retry/backoff on runtime network refusal, and leaves empty queue runs as not-attempted network transfers. `sync network discover-run` validates the same profile-backed network material, requires a non-ambiguous low-information LAN candidate matching the profile-selected `network.receiver_url`, writes no queue/run receipt when the discovery gate does not match, and otherwise runs the same per-entry profile-pinned mTLS network queue pass. `sync network loop` repeats that same profile-backed per-entry network queue pass in the foreground with generated session IDs, context-aware waits, optional `--max-runs`, idle not-attempted passes, and the same retry/backoff behavior. `daemon run --foreground` runs the same local polling queue consumer only when the reviewed profile enables `sync.local_polling`, writes ordinary incremental-sync run receipts plus redacted `daemon_sync_*` lifecycle events, continues with fresh generated run receipt IDs after a foreground `daemon restart`, and resumes the sequence from existing durable run receipts after a foreground process stop/start. `daemon run --foreground` also runs a source-side worker-only network queue daemon only when the reviewed source profile enables `sync.network_polling`, validates network trust/material before queue mutation, writes ordinary incremental-sync run receipts plus redacted `daemon_sync_*` lifecycle events, publishes through the same per-entry profile-backed mTLS network queue path, and resumes generated run numbering from existing durable receipts after a foreground process stop/start. `report` and `status` aggregate queue/run receipt counts, distinguish clean completed evidence from review-required queued/backoff/failed/retrying evidence, and surface malformed queue/run artifacts. |
| Planned | Broader automatic LAN discovery endpoint selection beyond the explicit profile receiver-address match gate remains separate future work. |
| Missing | No T-003-scoped implementation gap remains after per-entry network queue publication; automatic endpoint selection, detached service management, broad repair, and bidirectional sync remain out of scope. |

## Scope

- In scope: one-way changed-file queue, scheduler/watcher loop, durable queue
  state, receipts, status, cancellation, and backoff. Current implementation
  covers durable queue, bounded local run receipts/backoff, foreground local
  polling, foreground OS watcher execution, profile-enabled foreground-daemon
  local polling, bounded profile-backed per-entry `sync network run`, foreground
  profile-backed per-entry `sync network loop`, profile-enabled source-side
  foreground-daemon network polling, bounded LAN-discovery-gated
  `sync network discover-run`, daemon restart
  acceptance for fresh local polling and network polling run receipts,
  per-entry network queue publication with previous published manifest evidence
  for regular-file replacement,
  read-only per-entry queue listing, explicit failed-terminal queue review
  evidence, and compact report/status aggregation; remaining slices should not
  re-count those as missing.
- Out of scope: bidirectional sync, conflict resolution as a primary mode,
  automatic LAN discovery endpoint selection, and daemon lifecycle
  implementation itself.

## Acceptance Criteria

- Changed files are detected or scheduled into a durable queue.
- Incremental sync can stop and resume without losing queued work.
- Receipts and status distinguish queued, in-flight, published, failed, and retried items.
- Hidden files and dot-directories remain first-class migration data.

## Transfer Checks

- Keep one-way source-to-target semantics.
- Reuse existing profile and transfer safety invariants.
- Do not bypass publish preflight or target control-plane protections.

## Impact

- Code paths: watcher/scheduler, queue store, push runner, status/report/recover.
- Tests: queue lifecycle, hidden-file changes, retry/backoff, cancellation, and restart recovery.
- Rollout notes: depends on daemon lifecycle and broad network recovery acceptance.
