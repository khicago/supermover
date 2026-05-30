# Feature Proposal: f-22tnwsds8

## Title

V1 remaining implementation backlog normalization

## Goal

Normalize the remaining v1 product gaps into explicit, scoped follow-up
features with implemented/planned/missing status, dependency order, non-goals,
acceptance gates, and honest release wording.

This feature is an umbrella planning feature. It is not an implementation
bucket and must not claim any future behavior as implemented.

## Gap Review

| V1 gap | Implemented now | Planned / partially wired | Missing closure surface |
| --- | --- | --- | --- |
| True LAN browsing | Explicit-address `discover` exists. Profile-backed TLS 1.3 mTLS is wired for `serve` and non-dry-run `push --network`. | Discovery status and low-info hints exist around explicit addresses. | LAN advertisement/browsing, bounded browse timeout UX, duplicate/ambiguous peer handling, and acceptance evidence on a LAN-like path. |
| Agent daemon | `serve` can run a receiver with profile-backed network auth. | Receiver lifecycle is manually started by the operator. | Managed daemon install/start/stop/status/log surfaces, profile SSOT daemon config, restart semantics, and safe background lifecycle tests/docs. |
| Ongoing incremental sync | Repeated `push` runs can publish changed snapshots. Existing recover/status/report surfaces expose run evidence. | Some changed-state and drift evidence exists after explicit commands. | Long-running watcher/scheduler, durable incremental receipts, changed-file queueing, backoff, cancellation, and operator status for continuous sync. |
| Broad network resume / arbitrary interruption recovery | Current resume evidence is bounded to known same-session paths and targeted network checks. In-flight resume evidence is now persisted for the current network path. | Recovery/status surfaces can describe some staged or persisted state. | Kill/restart/reboot/interrupted-transfer matrix, network receiver reconciliation, resumable retry protocol evidence, and durable acceptance fixtures before claiming broad interruption recovery. |
| Drift resolve/reconcile and durable live detector persistence | `drift record` can persist current live detector findings when invoked explicitly. Local push refusal paths can also write durable `target_drift` artifacts automatically. Persisted `target_drifts` can be listed, acknowledged, expired, resolved, and reopened when the same logical finding returns. Narrow `reconcile plan/apply` exists and is tracked separately under `f-233nwduwz` and writes durable repair receipts for selected persisted apply outcomes. Report/status/drift list expose live detector evidence. | Live detector output informs current commands, and profile-backed foreground daemon drift recording can persist live detector findings as durable review evidence. There is still no broad scan inventory or general background detector loop independent of explicit recording, daemon drift recording, or existing publish/refusal paths. | Automatic/ongoing live detector persistence beyond explicit/manual/profile-backed drift recording or refusal-triggered paths, broad scan inventory, and later-feature acceptance proving the remaining persistence boundary rather than re-counting already implemented reopen, expire, or narrow reconcile behavior. |
| Approval-authoring UX | `prune approve` authors approval artifacts for existing prune candidates. `prune --apply` consumes existing approval artifacts. `prune review` now provides read-only review evidence. | Report/status expose approval inventory evidence. | Completed in `f-234nwra8e` with `prune approvals`, `prune supersede`, stale approval detection, and release readiness counts that separate review, approval mutation, and target mutation. |
| Richer prune/release workflow | `prune --dry-run`, `prune approve`, `prune approvals`, `prune review`, `prune supersede`, `prune --apply`, approval receipts, and report/status evidence exist. | The read-only review command is implemented and archived in `f-22unwv5qs` / commit `d938d40`. | This L1 gap is complete; later release acceptance can build on the current approval and receipt surfaces without reopening this feature. |
| Broader automatic repair/reconcile | Local recovery, persisted-drift resolve, narrow selected persisted-drift `reconcile plan/review/apply`, durable reconcile receipts, refusal `conflict_class`/`retry_advice`, explicit `--all-persisted-planned`, explicit `--record-live`, profile-backed daemon drift recording, and profile-backed daemon persisted planned apply exist. | `f-233nwduwz` is the current narrow repair boundary: apply handles missing regular-file restore plus resolve-noop only, consumes durable persisted evidence, requires explicit intent/reason or profile policy, and stops after refusals for operator review. | Remaining broad/background repair surfaces are split into `f-236nwqshz` retry policy, `f-237nwzbyq` scan inventory, `f-238nwybkh` manifest rewrite decisions, `f-239nwv337` repair-to-prune handoff, and `f-23anwj5su` background repair operator UX. |
| Traffic privacy level 2 broader release acceptance | Level 2 padding/batching/jitter and applied-overhead evidence exist for the current profile-backed network path. | Reports expose current-path privacy evidence. | Broader release acceptance matrix, repeatable fixtures, operator-facing acceptance summary, and explicit wording that level 2 is bounded traffic-shape protection, not anonymity. |

## Follow-Up Features

| Feature | Gap | DAG layer | Dependency source | Implementation scope |
| --- | --- | --- | --- | --- |
| `f-22vnwgwjj` | Traffic privacy level 2 release acceptance | L1 | Depends on this umbrella. | Repeatable level 2 release acceptance evidence for the current profile-backed network path; no anonymity claim. |
| `f-22wnwd5pe` | Broad network recovery acceptance | L1 | Depends on this umbrella. | Interruption/resume acceptance matrix across bounded failure modes. |
| `f-22xnwetxr` | Durable drift detector persistence and reconcile | L1 | Depends on this umbrella. | Honest durable drift persistence closure, including already-implemented reopen truth plus any remaining automatic persistence boundary and expire semantics. |
| `f-22znw2utc` | Agent daemon lifecycle | L1 | Depends on this umbrella. | Managed daemon lifecycle around profile SSOT and existing `serve`. |
| `f-234nwra8e` | Richer prune and release workflow | L1 | Depends on this umbrella and follows archived `f-22unwv5qs` implementation evidence. | Approval inventory, supersede mutation, stale approval detection, readiness gates, and receipt comparison. |
| `f-22ynwqndn` | LAN browsing discovery | L2 | Depends on this umbrella and `f-22znw2utc`. | LAN advertise/browse UX, low-info peer candidates, ambiguity handling, and discovery acceptance. |
| `f-232nwu2nw` | Ongoing incremental sync | L2 | Depends on this umbrella, `f-22znw2utc`, and `f-22wnwd5pe`. | Durable changed-file queue, watcher/scheduler, receipts/status, backoff, and cancellation. |
| `f-233nwduwz` | Narrow persisted repair and reconcile boundary | L2 | Depends on this umbrella, `f-22xnwetxr`, and `f-234nwra8e`. | Safe selected persisted repair plan/review/apply, receipts, conflict classes, mutation preflight, profile-backed daemon drift recording, profile-backed daemon persisted planned apply, and follow-up split for broad/background repair. |
| `f-236nwqshz` | Broad repair retry policy | L3 | Depends on `f-233nwduwz`. | Automatic/background retry policy over durable persisted drift and reconcile receipt evidence; no broad scan, manifest rewrite, prune handoff, or UX aggregation. |
| `f-237nwzbyq` | Broad repair scan inventory | L3 | Depends on `f-233nwduwz`. | Non-mutating broad scan inventory that writes durable review evidence without applying repair. |
| `f-238nwybkh` | Reconcile manifest rewrite decisions | L4 | Depends on `f-237nwzbyq`. | Explicit reviewed manifest rewrite proposal/apply/receipt flow separate from target repair. |
| `f-239nwv337` | Repair to prune handoff | L3 | Depends on `f-233nwduwz` and `f-234nwra8e`. | Reviewed handoff evidence from drift/repair into prune workflows without granting prune authorization. |
| `f-23anwj5su` | Background repair operator UX | L5 | Depends on `f-236nwqshz`, `f-237nwzbyq`, `f-238nwybkh`, and `f-239nwv337`. | Operator-facing status/report/review/runbook aggregation after underlying repair slices are wired. |

## Recommended Next Blocking Slice

Open implementation work from `f-22vnwgwjj` first if the goal is the lowest-risk
release-acceptance closure: the traffic privacy level 2 mechanism already has
current-path evidence, and the missing work is to make acceptance repeatable and
operator-facing without widening product claims.

Open `f-22wnwd5pe` first if the priority is network resilience evidence rather
than release-acceptance polish. LAN browsing, daemon lifecycle, ongoing sync,
and broad repair should remain later slices because they carry larger
architecture and safety risk.

## Gates

- The backlog table covers every v1 gap named by the user.
- Each remaining v1 gap has a separate follow-up feature.
- `FEATURES_DAG.json` is generated from feature `state.json depends_on` values.
- The archived `prune review` implementation is recorded as implemented, not
  still recommended as the next missing slice.
- No planned behavior is described as implemented.
