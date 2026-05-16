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
```

`push --dry-run` scans and reports counts without writing target files or
control-plane artifacts. The target path comes from the profile. A non-dry-run
local push copies supported regular files, records manifests and warnings, and
writes session control artifacts.

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
- `warnings`: audit records expected if the run is published.
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
- `.supermover/agent/<session>-influence.json`: agent knowledge catalog, if
  known agent files were present.

The profile snapshot is the audit anchor. If the operator cannot answer "which
profile produced this target state" from `.supermover/profiles/`, the run is not
acceptable.

## Warning Review

Warnings mean "published with evidence to review", not "ignored". For each
warning file:

1. Read `code`, `message`, `paths`, and `session_id`.
2. Decide whether the restored tree is acceptable without that behavior.
3. If policy should change, edit the profile and rerun from a new session.
4. Keep the warning record with the session evidence.

Examples of warning classes include scan errors and local push features that
are not implemented yet, such as symlink copying. The exact `code` is the
machine-readable field to use in acceptance checks.

## Soft-Delete Review

The v1 policy is conservative: source-side deletion must become a reviewable
record before physical deletion from the target. The profile field
`delete_policy.require_review` is the safety gate. `delete_policy.mode: prune`
without review is invalid.

After a second or later push, source files that disappeared since the latest
published manifest are written as `.supermover/deleted/*.json` records. List
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
