# Compact Local Status Contract

`status` is wired as an intentionally local-only compact operator view:

```bash
supermover status --profile <path> [--format text|json]
```

There is no `--session` flag in the initial `status` contract. Historical,
session-scoped review remains the job of:

```bash
supermover report --profile <path> --session <id>
```

`status` is a compact current-target view derived from the profile single source
of truth, target `.supermover` artifacts, and target files needed for
verification and live drift detection. It is read-only. It does not repair,
recover, prune, rewrite profiles, acknowledge review state, persist live
detector output, or run background scans.

`status` also exposes compact current profile/target prune approval counts,
prune receipt counts/issues, prune review status/action, and artifact-problem
source breakdown from durable `.supermover/prune/approvals/*.json` and
`.supermover/prune/receipts/*.json` artifacts. It does not list the full
approval or receipt inventory; use `prune review` for the focused read-only
prune release inventory. These counts are read-only audit evidence for review
of authored-but-unapplied approvals, stale or expired approvals, consumed
approvals, and receipt-attention states; they do not authorize prune, author
approvals, supersede approvals, apply prune decisions, write prune receipts,
delete files or symlinks, repair or reconcile drift, make a review-required
target clean, automatically release a migration, or close v1.

Persisted target-drift review state is managed by separate commands:
`supermover drift acknowledge --profile <path> --id <persisted-drift-id>
--reason <text>` and `supermover drift resolve --profile <path> --id
<persisted-drift-id> --reason <text>`. `status` may continue to show
acknowledged persisted drift as review-required evidence because
acknowledgement is not repair or reconciliation. Valid persisted drift records
closed by `drift resolve` are excluded from status review counts, but current
live detector findings remain review-required read-only evidence.
The separate `reconcile plan/apply` command surface may repair only a narrow
selected persisted-drift slice: missing regular-file restores from matching
published manifest and current source evidence, plus already-restored or
already-absent resolve-noop cases. `status` does not run that planner or
apply repair; it only reports the resulting persisted drift state and current
live detector evidence.

`status` reuses the same evidence classes as `report`, `health`, `verify`, and
`drift list` instead of inventing a second truth source. Pairing and network
fields are evidence only. Output does not include foreground daemon lifecycle
state; use `supermover daemon status --profile <path>` for
`.supermover/daemon` install/state/stop-intent/restart-intent evidence and
recent redacted lifecycle events, or `supermover daemon logs --profile <path>`
for the scoped event history. `status` output does not imply daemon health,
LAN browsing, encrypted transport readiness, or long-running sync status.

## Output Fields

Text output is compact operator output. Any target-controlled values, including
paths, session IDs, profile IDs, target IDs, warning IDs, drift paths, and
artifact paths, must be escaped before printing so control characters cannot
forge lines or fields.

JSON output is deterministic structured data. Object keys and list ordering must
be stable for the same evidence set. The current top-level fields are:

- `schema`: status output schema/version identifier.
- `scope`, `target_root`, `profile_id`, and `target_id`: profile-selected local
  target scope.
- `overall`: compact `status` of `clean` or `review_required`, plus
  `target_status` carrying the detailed report status such as
  `local_target_verified`, `local_target_attention`, or
  `local_target_unhealthy`.
- `issues`: report issue tokens when review is required.
- `latest_session`: selected latest published-manifest evidence and
  completeness summary.
- `counts`: counts for warnings, soft deletes, persisted target drift, live
  target drift, prune approvals, authored-but-unapplied prune approvals,
  active/stale/expired/consumed prune approvals, prune receipts, prune receipt
  issues, recovery issues, invalid health records, artifact problems,
  verification findings, pairing evidence issues, and network-transfer
  evidence.
- `prune_review`: compact prune release-review status/action copied from the
  broader report/review evidence without expanding to full item inventory.
- `pairing`: local pairing/profile-pin evidence state.
- `privacy`: profile privacy evidence and current local/network wiring status.
- `traffic_privacy_acceptance`: level 2 acceptance evidence for the
  profile-backed network path. It reports `passed` only when the profile is
  configured for profile-backed mTLS and a clean published
  `network-transfer.json` carries matching profile policy, matching source and
  target device IDs, and applied padding, batching, and jitter counters. It
  reports blockers such as missing or mismatched evidence instead of assuming a
  pass, and it carries `anonymity_claim=not_claimed`.
- `network`: local network-transfer artifact evidence status, including damaged
  network-transfer artifact counts when no transfer summary can be emitted.
- `review_required`: boolean copy of the compact review decision.

The field set may grow with implementation, but it must remain a profile/target
status surface. It must not accept runtime policy overrides.

## Exit Codes

- `0`: status was generated and the current local target is verified and clean.
- `1`: status was generated and operator review is required.
- `2`: usage, profile loading, profile validation, target-selection error, or
  status generation error prevented a status report from being emitted.

Review-required status includes warnings, soft deletes, unresolved persisted
target drift, live target drift, authored-but-unapplied prune approvals,
recovery issues, invalid or unreadable control artifacts, verification
warning/error findings, pairing evidence issues, network artifact issues, or no
published manifest for the current
profile-selected target.
Clean published network transfers, including zero-byte regular files published
through explicit final empty completion, should not create a `status` review
issue solely because the file had size `0`.

`status` must return `1` for review-required evidence even when text or JSON was
produced successfully.
