# User Migration Guide

This guide describes the current local push vertical slice. It is written for a
trusted target directory on the same machine or mounted filesystem. Network
serve/discover/pair transfer is planned, but not required for this workflow.

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
  require explicit review and a profile policy that permits it.
- LAN discovery is address discovery only. A discovered endpoint is not trusted
  until pairing verifies and pins device identity.

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
go run ./cmd/supermover recover --profile ./supermover.profile.json --dry-run
```

`push --dry-run` scans and reports counts without writing target files or
control-plane artifacts. Warning output at this stage is mainly the count; full
warning JSON is written only after a published run can continue. The target
path comes from the profile. A non-dry-run local push copies supported regular
files, records manifests and warnings, and writes session control artifacts.
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
`recover` performs the conservative mutating recovery subset.

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
   `push --dry-run`, `verify`, and `health` as the operational readiness gates.

4. Confirm these fields are intentional:

   - `roots`: every source root that may be read.
   - `include` and `exclude`: local push currently supports the default
     include-all policy only; custom rules fail fast until rule evaluation is
     implemented.
   - `delete_policy.mode`: v1 defaults to recording deletes with review.
   - `delete_policy.require_review`: must be true before prune behavior.
   - `privacy_policy.allow_plaintext_restore`: confirms the target may receive
     plaintext files.
   - `privacy_policy.discovery_low_info`: required for traffic level 2.
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
overwrite existing files unless the existing file content is byte-identical to
the source. This makes reruns idempotent while preventing accidental replacement
of unrelated target data.

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
field to use in acceptance checks.

## Soft-Delete Review

The v1 policy is conservative: source-side deletion must become a reviewable
record before physical deletion from the target. The profile field
`delete_policy.require_review` is the safety gate. `delete_policy.mode: prune`
without review is invalid.

After a second or later push, source files that disappeared since the latest
published manifest for the same `profile_id`, `target_id`, and root are written
as `.supermover/deleted/*.json` records. The records include previous
session/manifest evidence, path, kind, size, and digest when available. List
them before any manual cleanup:

```bash
go run ./cmd/supermover deleted list --profile ./supermover.profile.json
go run ./cmd/supermover verify --profile ./supermover.profile.json --session session-001
```

Physical prune and approval commands are intentionally not implemented in the
current local push slice. Do not manually remove target files as a substitute
for reviewed pruning unless the manual action is tracked outside Supermover.

## LAN Discovery And Trust

Discovery advertisements are intentionally low information: service type,
protocol, nonce, and minimal capability flags. They must not include usernames,
hostnames, profile labels, paths, inventory sizes, or friendly names.

Discovery answers only "where might a target be reachable?" Trust requires
pairing, explicit verification, a pairing receipt, and pinned device identity in
the profile/control plane. Never treat a discovered address as proof that the
target is the intended restore destination.
