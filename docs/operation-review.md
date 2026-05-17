# Operation Review

Supermover keeps migration review data in the target `.supermover` control
plane. Profiles remain the SSOT for configuration; review commands consume
artifacts and do not invent migration policy outside the profile.

## Verify Review

The `internal/verify` package builds an operator review report from:

- `.supermover/sessions/<session>/manifest.json`
- `.supermover/warnings/*.json`
- `.supermover/deleted/*.json`
- `.supermover/drift/*.json`

Default review only consumes sessions with a valid
`.supermover/sessions/<session>/receipt.json` whose status is `published`.
Manifests, warnings, and soft-delete records from sessions without a published
receipt are treated as recovery evidence rather than review/prune input. Use
`health` and `recover` first to bring the session to a terminal state or to mark
it `needs_repair`. Target-drift records are different: the current local push
flow writes them only when it refuses a managed changed-file update because the
target no longer matches trusted previous manifest evidence. They are scoped by
profile, target, root, and attempted session, and existing `verify`, `health`,
and `report` surfaces show unresolved records as review-required artifacts. The
`internal/verify` package has a read-only detector for comparing a published
manifest with the target filesystem, including missing, changed,
type-mismatched, and extra target-local paths. That detector is exposed by
`drift list` and by the independent live target drift section of `report`; it
is not exposed through `verify` or `health` scans, does not run in the
background, and is not persisted by those read-only surfaces. `drift record`
uses the same detector to persist current findings as durable
`.supermover/drift` review records.

`supermover drift list --profile <path> [--session <id>] [--format text|json]`
exposes the same detector as a read-only operator command. It derives the target
from the profile only, compares published manifest evidence to the target
filesystem, and returns review-required non-zero for detected drift, artifact
problems, or no published manifest. When report generation succeeds,
`report --profile` runs the same detector and exposes it separately from
persisted `target_drifts`: JSON uses
`live_target_drift` and summary counters such as `live_target_drifts` and
`live_target_drift_artifact_problems`, while text reports include equivalent
review evidence. Neither read-only live detector surface acknowledges or
resolves drift, repairs drift, mutates review state, prunes target paths, or
persists detector output. JSON output is the automation and durable audit
format; text output is compact operator review output with target-controlled
values escaped.

`supermover drift record --profile <path> [--session <id>] [--format
text|json]` is the explicit persistence path for current live detector findings.
It writes `.supermover/drift/<id>.json` records and returns review-required
non-zero when it records or observes drift/artifact problems. It is still
record-only: no repair, prune authorization, live detector suppression,
background scan, or broad reconcile is performed.

For the selected manifest, the shared verify report path checks regular target
files by safe relative target path, size, `sha256:` digest when present, and
permission mode or modification time when those fields are present. It also
checks directory entries as plain directories and symlink entries by `readlink`
target. Invalid JSON or unreadable artifacts are retained as report problems
instead of aborting the whole review.

`supermover verify --profile <path>` calls this package through the profile
target and renders either text or JSON. A non-zero exit means the selected
manifest has error findings, warning findings, warning records, soft-delete
records, unresolved persisted target-drift records, artifact problems, or no
manifest at all. Warning
findings include cases where a regular file cannot be fully verified because the
manifest digest is missing or uses an unsupported algorithm.

Current target-drift records contain the attempted session and scope, the
target-relative path, `detected_at`, `change`, structured `expected` manifest
evidence, structured `observed` target state, `review_state`, and reviewable
`evidence` strings. The current local push writer uses change categories such as
`content_mismatch`, `metadata_mismatch`, and `missing` when refusing a changed
managed regular file. The read-only detector can also produce package-level
records with changes such as `missing`, `content_mismatch`,
`metadata_mismatch`, `type_mismatch`, `symlink_mismatch`, `unsafe_parent`, and
`extra`; `drift list` renders those observations without persisting them, while
`drift record` can make the current observations durable artifacts. Drift review UX can compare what Supermover expected from the
previous/published manifest with what was actually present on the target.
Unresolved records shown by `verify`, `health`, and `report`, plus
observations from `drift list`, should be treated as review-required.

`supermover drift acknowledge --profile <path> --id <persisted-drift-id>
--reason <text> [--reviewer <id>] [--format text|json]` can add
acknowledgement metadata to one existing persisted target-drift record. The ID
must come from persisted `target_drifts` evidence, such as `verify --format
json`, `report --format json`, or a record written by `drift record`; live-only
detector IDs from `drift list` or `report.live_target_drift` are not durable
records and are refused. The command
derives target scope from the profile and rechecks the persisted record,
published receipt, manifest, root, and artifact boundary before writing
`review_state=acknowledged` plus review time/reviewer/reason metadata.
Acknowledgement is not reconciliation: it does not repair target files, rewrite
manifests, suppress live detector output, resolve records, authorize prune, or
make review-required reports clean.

`supermover drift resolve --profile <path> --id <persisted-drift-id> --reason
<text> [--reviewer <id>] [--format text|json]` can close one existing persisted
target-drift record. The ID must come from persisted `target_drifts` evidence,
not directly from live detector output. The command derives target scope from
the profile, rechecks the persisted record, published receipt, manifest, root,
and artifact boundary, then runs a fresh profile-scoped live detector for the
record's published baseline. It writes `review_state=resolved` plus review
time/reviewer/reason metadata only when the same path and expected baseline no
longer reports drift. Resolved records no longer make `verify`, `health`,
`report`, or `status` review-required, but live detector output remains
read-only review evidence. Resolve is not repair or broad reconciliation: it
does not mutate target files, rewrite manifests, authorize prune, suppress
future detector findings, or resume refused updates.

## Soft Delete Review

The `internal/deleted` package compares the previous manifest with the current
source scan and emits `control.SoftDelete` records for paths that disappeared
from the source. It records intent only:

- no target files are physically deleted
- directory entries are skipped
- record IDs are deterministic from session and paths
- records include profile, target, root, previous session, previous manifest,
  kind, size, and digest evidence when known
- timestamps are caller supplied for reproducible tests

The local push flow now:

1. Read the latest trusted manifest for the same profile ID, target ID, and
   root.
2. Scan the current source root from the profile.
3. Call `deleted.Generate`.
4. Persist records under `.supermover/deleted/<id>.json`.
5. Use `prune --dry-run` to produce non-mutating candidate/refusal evidence with
   previous manifest evidence and current target state.
6. Use `prune review` for focused read-only prune release aggregation: pending
   candidates, refusals, existing prune receipts, and non-applied receipt
   issues are surfaced without writing approvals, receipts, or target files.
7. Use `report` for broader read-only prune aggregation: pending candidates, refusals,
   existing prune receipts, and non-applied receipt issues are surfaced in the
   same operator report as warnings, health, drift, and verification evidence.
8. Use `prune approve --profile <path> --id <approval-id> --soft-delete <id>
   [--soft-delete <id>...] --reason <text> --reviewer <id>` to write a durable
   approval artifact under `.supermover/prune/approvals/<id>.json` from fresh
   dry-run evidence. `--approved-by` is an alias for `--reviewer`;
   `--expires-at <RFC3339>` and `--format text|json` are optional. Approval
   authoring does not delete target files or write prune receipts.
9. Use `prune --apply --approval <id>` for physical prune. Apply writes a
   durable started receipt before mutation, rechecks target state, and records
   applied/partial/failed status when finalization succeeds.

## Mainline Integration Points

Remaining integration:

- Reviewed physical prune now has narrow approval authoring through
  `prune approve`, physical deletion through `prune --apply --approval <id>`,
  durable receipts, read-only `prune review` inventory, and read-only report
  prune evidence. Broader release workflow surfaces remain planned.
- `verify` should eventually include richer directory metadata checks beyond
  current directory presence/type and regular-file metadata validation.
