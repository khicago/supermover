# Feature Proposal: f-22xnwetxr

## Why

SuperMover already has durable drift artifacts, explicit `drift record`,
persisted drift acknowledge/resolve, reopen-on-redetect, and a narrow persisted
drift reconcile CLI slice. What still lacks closure is an honest, unified
durable-persistence story for live detector findings plus any explicit expiry
policy for persisted drift review records.

The main risk now is not only missing code. Tracker/docs still understate
implemented reopen and narrow reconcile behavior, while over-claiming missing
closure around areas that have already moved into adjacent features such as
`f-233nwduwz`. This feature needs its own truth boundary repaired before the
remaining implementation can be scoped correctly.

## Goal

Close the durable drift detector persistence feature by keeping implemented
drift lifecycle truth honest, clarifying the current automatic durable-record
boundary, and adding any remaining explicit expiry or command-boundary
persistence behavior that belongs with durable drift evidence rather than with
broader repair/reconcile work.

## Implemented / Planned / Missing

| Status | Detail |
| --- | --- |
| Implemented | `drift record` persists current live detector findings explicitly. Persisted `target_drifts` can be listed, acknowledged, expired, resolved, and reopened when the same logical finding is detected again. Local push refusal paths already record durable target-drift artifacts through `driftstore.Put`, preserving review metadata on repeated observations. |
| Implemented narrowly | Reopen semantics, review-history carry-forward, and narrow persisted-drift `reconcile plan/apply` exist in code today, but the narrow reconcile slice is tracked separately under `f-233nwduwz` and should not be double-counted here as remaining closure. |
| Planned | There is still no general background scan/watcher surface that continuously persists live detector findings independent of explicit `drift record` or existing refusal-triggered writes. That broader ongoing durability surface belongs to later daemon/ongoing-sync work once current drift-evidence boundaries are already explicit and honest. |
| Missing | No additional feature-local drift-evidence mutation gap remains after explicit `drift expire` and boundary truth updates. Broad automatic repair, durable repair receipts, and general ongoing/background persistence remain separate later-feature work. |

## Scope

- In scope: durable drift artifact persistence semantics, reopen semantics,
  command-boundary or automatic artifact writing that belongs to drift evidence,
  explicit expiry policy if implemented, and tests/docs for those behaviors.
- Out of scope: broad automatic repair, durable repair receipts, conflict-class
  taxonomy beyond current narrow reconcile refusals, daemon/ongoing sync
  integration, manifest rewrite, prune integration, and broad operator repair
  UX. Those belong to `f-233nwduwz` or later features.

## Acceptance Criteria

- Tracker/docs separate implemented reopen and narrow reconcile behavior from
  still-missing durable persistence closure.
- Live detector findings can become durable through explicit or currently wired
  automatic paths without silently discarding prior review metadata for the same
  logical finding.
- Expiry semantics are implemented deterministically and covered by behavior
  tests.
- Existing `drift acknowledge` and `drift resolve` remain compatible with
  persisted records and reopen history.

## Transfer Checks

- Treat target drift records as durable audit artifacts.
- Keep `.supermover` target control-plane safety intact.
- Do not mutate target files from the detector persistence path.
- Do not claim broad automatic repair or broader reconcile closure from this
  feature.

## Impact

- Code paths: drift detector/store, local push drift-artifact writer,
  drift-record/review CLI, docs/tracker truth.
- Tests: reopen/lifecycle tests, automatic drift-artifact write tests, and
  expiry tests.
- Rollout notes: narrow persisted-drift reconcile remains tracked under
  `f-233nwduwz`; broader ongoing/background persistence remains later work and
  should not be silently re-opened inside this closed drift-evidence slice.
