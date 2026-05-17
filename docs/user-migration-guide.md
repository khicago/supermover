# User Migration Guide

This guide describes the current local push vertical slice. It is written for a
trusted target directory on the same machine or mounted filesystem. Network
serve/discover/pair and non-dry-run `push --network` are wired separately, but
they are not required for the local workflow.

## Guarantees And Boundaries

- Migration is one-way: `source -> trusted target`.
- The profile JSON is the single source of truth for source roots, stable target
  identity, the current trusted local target path, consistency, delete policy,
  metadata policy, privacy policy, and agent knowledge handling.
- The target receives plaintext files. Do not use Supermover v1 to restore into
  an untrusted target.
- Warnings are audit records. A warning is not a quiet best-effort note; it is
  written to `.supermover/warnings/*.json` when a published run can continue
  but leaves a reviewable gap.
- Source deletions are recorded before any physical prune. Physical pruning must
  require explicit review, durable approval evidence, target-state checks, a
  durable prune receipt, and a profile policy that permits it. Source absence
  alone never authorizes deletion.
- Operator summaries are derived from the profile SSOT and target
  control-plane artifacts. Command stdout is useful for triage, but the durable
  evidence lives under `.supermover`.
- Discovery is address discovery only. A discovered endpoint is not trusted
  until pairing verifies and pins device identity. LAN browsing remains planned.

## Current Commands

```bash
go run ./cmd/supermover help
go run ./cmd/supermover profile init --profile ./supermover.profile.json --source /path/to/source --target /path/to/target
go run ./cmd/supermover profile lint --profile ./supermover.profile.json
go run ./cmd/supermover profile set-target --profile ./supermover.profile.json --target /path/to/target
go run ./cmd/supermover scan --profile ./supermover.profile.json
go run ./cmd/supermover push --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover push --profile ./supermover.profile.json --session session-001
go run ./cmd/supermover verify --profile ./supermover.profile.json --session session-001
go run ./cmd/supermover deleted list --profile ./supermover.profile.json
go run ./cmd/supermover health --profile ./supermover.profile.json
go run ./cmd/supermover drift list --profile ./supermover.profile.json
go run ./cmd/supermover drift record --profile ./supermover.profile.json
go run ./cmd/supermover drift resolve --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<target restored reason>"
go run ./cmd/supermover reconcile plan --profile ./supermover.profile.json --id <persisted-drift-id>
go run ./cmd/supermover report --profile ./supermover.profile.json
go run ./cmd/supermover status --profile ./supermover.profile.json
go run ./cmd/supermover recover --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover prune --help
go run ./cmd/supermover prune --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover prune review --profile ./supermover.profile.json
go run ./cmd/supermover prune --profile ./supermover.profile.json --apply --approval <approval-id>
```

Use `drift acknowledge` only after refused-push review output or
`drift record` has created a persisted drift ID and an operator has reviewed
it:

```bash
go run ./cmd/supermover drift acknowledge --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<operator review reason>"
```

Use `drift resolve` only after the target has been restored so a fresh live
detector no longer reports the same persisted path and expected baseline:

```bash
go run ./cmd/supermover drift resolve --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<target restored reason>"
```

`push --dry-run` scans and reports counts without writing target files or
control-plane artifacts. Warning output at this stage is mainly the count; full
warning JSON is written only after a published run can continue. The target
path comes from the profile. A non-dry-run local push copies supported regular
files, records manifests and warnings, and writes session control artifacts.
`prune --dry-run` validates the profile prune policy, reads published
soft-delete records, and emits review-only candidates, refusals, and artifact
problems without writing approvals or receipts, applying approvals, or deleting
files. Records still inside `delete_policy.retention_days` remain visible as
`retention_window_active` refusals instead of prune candidates.
Source scanner `scan_error` findings block push before publish instead of being
published as warning records.

`verify` checks published regular file payloads and metadata against the
manifest: size, `sha256:` digest, permission mode, and modification time. It
also verifies directory entries as plain directories and symlink entries by
`readlink` target. Unsupported manifest entry kinds and non-file mismatches are
reported as findings instead of being ignored. `verify` exits non-zero for
error findings, warning findings, artifact problems, or a missing manifest.
`deleted list` shows reviewable source-side deletions. `health` is read-only:
it reports incomplete or invalid session records and missing/corrupt published
artifacts under the target `.supermover` directory and does not repair them.
`drift list` is a read-only live detector over published manifest evidence and
the profile-selected target. `drift record` runs the same live detector and
persists current findings as durable `.supermover/drift/*.json` review records;
it does not acknowledge, repair, prune, suppress later detector findings, run
background scans, or broadly reconcile. `drift acknowledge` and `drift resolve`
are narrow persisted-record review commands:

```bash
go run ./cmd/supermover drift record --profile ./supermover.profile.json --format json
go run ./cmd/supermover drift acknowledge --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<operator review reason>" --format json
go run ./cmd/supermover drift resolve --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<target restored reason>" --format json
```

Use IDs from persisted `target_drifts` in `verify --format json` or
`report --format json`, or IDs returned by `drift record`. Live-only IDs from
`drift list` or `report.live_target_drift` must be recorded before they can be
acknowledged or resolved. Acknowledgement writes review metadata only and keeps
the persisted record review-required. Resolve writes review metadata only after
a fresh live detector no longer reports the same path and expected baseline.
Neither command repairs target files, suppresses live detector output,
authorizes prune, performs broad reconcile, or edits manifests. Valid resolved
persisted records no longer make status/report/health/verify review-required,
but current live detector drift remains review-required.

Use `reconcile plan/apply` only for the narrow persisted-drift repair slice:

```bash
go run ./cmd/supermover reconcile plan --profile ./supermover.profile.json --id <persisted-drift-id> --format json
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --id <persisted-drift-id> --apply --reason "<operator repair reason>" --format json
```

`plan` is non-mutating. `apply` requires one or more selected persisted drift
IDs, explicit `--apply`, and `--reason`; source and target paths still come
from the profile SSOT, with no `--target` or `--state-dir` override. Current
repair support is limited to missing regular-file restores when published
manifest evidence and the current source file match, plus resolve-noop when the
target already matches expected evidence or an expected-absent target path is
already absent. It does not consume live-only drift, run broad automatic
reconcile, write durable repair receipts, rewrite manifests, retry in the
background, or participate in daemon/ongoing sync.

`report` is a read-only aggregation command over the same evidence. It
summarizes warnings, profile suggestions, soft-delete records, persisted target
drift, prune candidates, prune refusals, current-scope prune approval evidence,
existing prune receipts, receipt issues, health/recovery issues, artifact
problems, pairing evidence state, and published-manifest verification state at
report time. It also runs the live target drift detector and reports
review-required drift separately from persisted `.supermover/drift/*.json`
records. `status` narrows prune approval evidence to compact prune release
counts plus prune review status/action. Prune report/status state is
evidence-only; report/status do not author approval artifacts, supersede
approvals, apply prune decisions, write receipts, delete files or symlinks,
repair/reconcile drift, make the target clean, automatically release a
migration, or close v1. Use
`prune approve` for durable approval authoring and
`prune --apply --approval <id>` for physical prune. Pairing state is evidence-only;
examples include `unpaired`, `paired_receipt_valid`,
`profile_invalid`, `paired_target_missing`, and
`paired_receipt_mismatch`/`paired_receipt_missing`/`paired_receipt_invalid`.
It is not live transport or daemon readiness, and it does not start or prove
network readiness by itself. It may surface persisted network-transfer evidence
after a non-dry-run network attempt reaches receiver begin and writes artifacts,
but pre-begin failures and dry runs can leave no network-transfer artifact. It
exits non-zero when the report requires operator review, including live target
drift, even when text or JSON output was produced successfully. Live report
drift is read-only evidence; use `drift record` to persist current findings
before persisted-record review. Report does not persist, acknowledge, resolve,
repair, reconcile, or prune drift. `recover` performs the conservative mutating
recovery subset.

`status` is wired as
`supermover status --profile <path> [--format text|json]`. It is a current
profile/target view over profile SSOT, target `.supermover` evidence, and
target files needed for verification/live drift detection, with no `--session`
flag and no repair, recover, prune, profile-update, background-scan, daemon,
LAN, encrypted transport, or long-running sync semantics. Use
`report --profile ... --session ...` for historical session-scoped evidence.

## Prepare A Profile

1. Choose a profile path that can be reviewed and versioned with your migration
   plan.
2. Initialize the profile:

   ```bash
   go run ./cmd/supermover profile init --profile ./supermover.profile.json --source /path/to/source --target /path/to/target
   ```

   `--target` sets `target.local_path`. The default `target.target_id` is a
   separate local identity; pass `--target-id` only when the intended target
   identity should change.

3. Review the profile before running migration:

   ```bash
   go run ./cmd/supermover profile lint --profile ./supermover.profile.json
   ```

   `profile lint` validates schema and safety invariants. It does not prove
   the profile is executable by the current local push implementation; use
   `push --dry-run`, `verify`, `health`, and the read-only `report` summary as
   the operational readiness gates.

4. Confirm these fields are intentional:

   - `roots`: every source root that may be read.
   - `include` and `exclude`: local push currently supports the default
     include-all policy only; custom rules fail fast until rule evaluation is
     implemented.
   - `delete_policy.mode`: v1 defaults to recording deletes with review.
   - `delete_policy.require_review`: must be true when `mode` is `prune`.
   - `delete_policy.allow_physical_prune`: must be true when `mode` is
     `prune`, and is invalid outside `mode: prune`.
   - `privacy_policy.allow_plaintext_restore`: confirms the target may receive
     plaintext files.
   - `privacy_policy.padding_bucket_bytes`, `batch_max_bytes`,
     `batch_max_count`, and `jitter_budget_millis`: required for traffic
     level 2.
   - `privacy_policy.discovery_low_info`: required for traffic level 2.
   - `traffic_level: 2`: a profile/schema declaration. Local push does not
     apply traffic shaping. Current operator `push --network` supports only
     traffic level 2 and applies bounded padding, batching, and timing jitter
     through the protocol client when supplied a level 2 policy. Levels 1 and 3
     remain schema/planning values for this network path. Level 2 only reduces
     some record-size, batch, and timing signals; it will not hide total bytes,
     transfer duration, peer IP addresses, LAN presence, or Supermover use.
   - `target.target_id`: the expected target identity, not a local path or
     transient discovery label.
   - `target.local_path`: trusted local restore directory for the local push
     slice.
   - `agent_knowledge.categories`: agent rule and state files to catalog.

   The current local push implementation fails fast if policy fields ask for
   behavior it does not implement, such as excluding hidden files, suppressing
   sensitive filenames, disabling permission/modtime preservation, preserving
   extended attributes, custom agent knowledge categories, or multiple roots.

Do not pass ad hoc runtime flags to change policy. If behavior should change,
edit the profile, lint it, and keep the changed profile with the run records.
Use `profile set-target` when the trusted local target path changes; it updates
`target.local_path` instead of bypassing the SSOT at push time. It keeps
`target.target_id` unchanged unless `--target-id` is explicitly supplied.

## Run A Dry Run

```bash
go run ./cmd/supermover push --profile ./supermover.profile.json --dry-run
```

Review the summary:

- `roots`: number of configured roots.
- `entries`: scanned filesystem entries.
- `warnings`: warning count expected if the run is published.
- `influences`: agent knowledge files detected for cataloging.

Stop and inspect the source tree if the counts are surprising. For a deeper
scan report:

```bash
go run ./cmd/supermover scan --profile ./supermover.profile.json
```

## Publish A Local Push

Use a stable session ID when you need deterministic evidence in acceptance
tests or runbooks:

```bash
go run ./cmd/supermover push --profile ./supermover.profile.json --session session-001
```

Expected success output:

```text
published session session-001: entries=<n> copied=<n> warnings=<n> influences=<n> deleted=<n>
```

The target should now contain restored files and a `.supermover` control
directory. Treat `.supermover` as part of the migration result, not disposable
cache.

Use an empty target directory for first migration. Current local push refuses to
overwrite unrelated existing target data. Reruns are idempotent when existing
files are byte-identical to the source. A changed regular file is replaced only
when the latest published manifest for the same profile, target, and root proves
Supermover published the previous target content and the target still matches
that previous size, `sha256:` digest, mode, and modification time. If the target
was edited manually after the last publish, `push` refuses the update and leaves
the target file intact. If the previous target file was deleted, the update is
also refused unless recovery can verify matching `previous` and `current`
replacement holds from the interrupted session. Avoid editing the target tree
while `push` or `recover` is running. Managed changed-file publish stores the
previous target snapshot in `.supermover/replacement-holds/<session>/previous/...`
and moves the current target into
`.supermover/replacement-holds/<session>/current/...` before publishing the
staged replacement with no-replace semantics after a final evidence check.

## Review Control-Plane Evidence

After a published run, inspect these files on the target:

```bash
find /path/to/target/.supermover -maxdepth 4 -type f | sort
```

Required evidence for the local push slice:

- `.supermover/profiles/profile-<session>.json`: embedded profile snapshot.
- `.supermover/sessions/<session>/receipt.json`: session status and target ID.
- `.supermover/sessions/<session>/manifest.json`: restored content manifest.
- `.supermover/warnings/*.json`: warning audit records, if warnings occurred.
- `.supermover/deleted/*.json`: soft-delete records, if source paths
  disappeared since the latest matching profile/target/root manifest.
- `.supermover/agent/<session>-influence.json`: agent knowledge catalog, if
  known agent files were present.

The profile snapshot is the audit anchor. If the operator cannot answer "which
profile produced this target state" from `.supermover/profiles/`, the run is not
acceptable.
`report` also decodes the embedded snapshot privacy policy so operators can see
the exact traffic-level bounds recorded for each session. For the current local
push slice, overhead is reported as `not_applied`; the fields are durable policy
evidence, not proof that padding, batching, or jitter changed local file copy
traffic.
Custom level-2 bounds can be valid profile declarations while still reporting
`local_push=unsupported_privacy_policy`, because current local push only accepts
the default plaintext/local privacy contract and does not apply traffic shaping.

Use the read-only operator summaries after checking the artifact inventory:

```bash
go run ./cmd/supermover report --profile ./supermover.profile.json
```

`report` is the review surface for combining warning records, profile
suggestions, soft-delete records, health/recovery issues, artifact problems,
prune candidates, prune refusals, existing prune receipts, receipt issues,
pairing evidence state, published-manifest verification state, and live target
drift into one audit-oriented view. Pairing state should be read as
target-identity evidence:
`unpaired` means no complete profile pins and receipt were found,
`paired_receipt_valid` means profile pins match the pairing receipt, and
receipt mismatch/missing/invalid states mean the target must not be treated as
trusted until reviewed. Pairing evidence alone is not a transfer attempt;
non-dry-run `push --network` still validates the pinned TLS peer and records
receiver-side network transfer evidence after receiver begin stores a session.
Pre-begin failures can leave no network-transfer artifact. Treat `report` as a
view over the profile and `.supermover` evidence plus the live detector, not as
a substitute for preserving the artifacts. JSON reports expose live drift as
`live_target_drift`, with counters such as `live_target_drifts` and
`live_target_drift_artifact_problems`; persisted `.supermover/drift/*.json`
records remain the separate `target_drifts` evidence. Use `drift record` when
current live findings should become durable review records; use `drift resolve`
only for existing persisted records after a fresh detector no longer reports
drift for the same path and expected baseline. In scripts, capture
the output before acting on a non-zero exit; non-zero means review is required
unless stderr says the report could not be generated.

`verify` and `deleted list` use published receipts as the review boundary. If a
session has a manifest or soft-delete artifact but no `published` receipt, those
artifacts are recovery evidence; run `health` and `recover` before treating them
as completed migration state.

## Warning Review

Warnings mean "published with evidence to review", not "ignored". For each
warning file:

1. Read `code`, `message`, `severity`, `paths`, `target_path`, `detected`, and
   `session_id`.
2. Decide whether the restored tree is acceptable without that behavior.
3. If policy should change, edit the profile and rerun from a new session.
4. Keep the warning record with the session evidence.

Examples of warning classes include symlink publish conflicts, where a source
symlink cannot replace an existing non-symlink target and is recorded as
`symlink_not_published`. Source scanner `scan_error` findings are different:
they block push before publish because source inventory and soft-delete
evidence are not reliable. The exact warning `code` is the machine-readable
field to use in acceptance checks. `report` groups warning records with profile
suggestions, but the warning JSON remains the durable decision artifact.

## Soft-Delete Review

The v1 policy is conservative: source-side deletion must become a reviewable
record before physical deletion from the target. The profile field
`delete_policy.require_review` is one safety gate. Current profile validation
requires `delete_policy.mode: prune`, `delete_policy.require_review: true`, and
`delete_policy.allow_physical_prune: true` to appear together. The
`prune --dry-run` command validates that profile policy, reads published
soft-delete records, and emits review-only candidates, refusals, and artifact
problems. Soft-delete records are candidates only after
`detected_at + retention_days * 24h`; while the window is active, dry-run emits
`retention_window_active` refusal evidence. It does not write approval records,
apply physical deletion, or write prune receipts.

After a second or later push, source files that disappeared since the latest
published manifest for the same `profile_id`, `target_id`, and root are written
as `.supermover/deleted/*.json` records. The records include previous
session/manifest evidence, path, kind, size, and digest when available. List
them before any manual cleanup:

```bash
go run ./cmd/supermover deleted list --profile ./supermover.profile.json
go run ./cmd/supermover prune --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover report --profile ./supermover.profile.json
go run ./cmd/supermover verify --profile ./supermover.profile.json --session session-001
```

`report` surfaces soft-delete records alongside published-manifest
verification state and health/recovery issues. `deleted list` remains the
itemized review command for source paths that disappeared. `prune --dry-run`
turns those records into non-mutating review evidence with previous manifest
evidence, current target state, and refusal reasons. Active retention windows
are refusal reasons and keep the record inspectable without making it
approvable.

Author approval artifacts from fresh dry-run evidence before physical deletion:

```bash
go run ./cmd/supermover prune approve --profile ./supermover.profile.json --id <approval-id> --soft-delete <soft-delete-id> --reason "<review reason>" --reviewer <reviewer-id>
```

`--approved-by` is an alias for `--reviewer`, `--expires-at <RFC3339>` can set
an expiry, and `--format text|json` controls output. Approval authoring writes
`.supermover/prune/approvals/<id>.json` plus profile snapshot evidence; it does
not delete target files or write prune receipts. The command only writes an
approval when the fresh dry-run has no refusals or artifact problems, and
selected IDs must be current dry-run candidates.

After authoring, `report` exposes current profile/target approval evidence from
`.supermover/prune/approvals/*.json`, while `status` exposes compact prune
release counts plus prune review status/action. This helps release review
distinguish authored-but-unapplied approvals, stale or expired approvals,
consumed approvals, and receipt-attention states from applied receipts, but it
is not deletion authorization or automatic release evidence by itself.
Use `prune approvals --profile ./supermover.profile.json` for read-only
inventory over current-scope approval artifacts, and
`prune supersede --profile ./supermover.profile.json --id <approval-id>
--reason <text> --reviewer <reviewer-id>` to mark an older approval superseded
without deleting target files or writing prune receipts.
Use `prune review --profile ./supermover.profile.json` for the focused
read-only prune release-review view; it reads candidates, approvals, and
receipts without writing approvals, receipts, or deleting target files.

Physical deletion is wired only through reviewed prune approval artifacts. When
an approval artifact exists at `.supermover/prune/approvals/<id>.json`, run:

```bash
go run ./cmd/supermover prune --profile ./supermover.profile.json --apply --approval <id>
```

The command writes `.supermover/prune/receipts/<id>.json`, records a started
receipt before target mutation, rechecks current target evidence, deletes only
approved file/symlink targets, and finalizes each item as `pruned`, `refused`,
or `failed`. Approval evidence must bind the profile, target, root,
profile-snapshot digest, soft-delete record, previous manifest evidence, target
path, reviewed policy, operator, timestamp, and decision. Receipts record the
approval, target-state check, outcome, and any refusal reason.

Current prune apply refuses when policy does not permit physical prune, approval
evidence is missing or invalid, target identity or path safety checks fail,
soft-delete or manifest evidence is missing, target state has drifted from the
reviewed evidence, the current prune plan reports an active retention window,
approval was superseded or expired, or the operator refuses. Manual target
deletion remains outside Supermover audit and must be tracked separately.

## LAN Discovery And Trust

Current `discover` uses explicit address hints and does not browse LAN
services. Discovery advertisements are intentionally low information: service
type, protocol, nonce, and minimal capability flags. They must not include
usernames, hostnames, profile labels, paths, inventory sizes, or friendly names.

Discovery answers only "where might a target be reachable?" Trust requires
pairing, explicit verification, a pairing receipt, and pinned device identity in
the profile/control plane. Never treat a discovered address as proof that the
target is the intended restore destination. Non-dry-run `push --network` is the
current profile-backed encrypted transfer command; `push --network --dry-run`
is preflight-only and writes no target artifacts.
