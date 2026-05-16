# Operations Runbook

This runbook gives repeatable steps for a local push migration and the evidence
operators should preserve. It separates currently implemented commands from
planned mainline commands so acceptance work can proceed without implying
network features are complete.

## Preflight

1. Confirm the source and target paths:

   ```bash
   test -d /path/to/source
   mkdir -p /path/to/target
   ```

2. Create or update the profile:

   ```bash
   go run ./cmd/supermover profile init --profile ./supermover.profile.json --source /path/to/source --target /path/to/target
   go run ./cmd/supermover profile lint --profile ./supermover.profile.json
   ```

   If the profile already exists, do not overwrite it with `profile init`.
   Review and edit the existing profile instead.
   If only the trusted local target path changed, use:

   ```bash
   go run ./cmd/supermover profile set-target --profile ./supermover.profile.json --target /path/to/target
   go run ./cmd/supermover profile lint --profile ./supermover.profile.json
   ```

3. Verify policy gates in the profile:

   - `delete_policy.require_review` is true.
   - `privacy_policy.allow_plaintext_restore` is true only for trusted targets.
   - `privacy_policy.discovery_low_info` is true for traffic level 2.
   - `target.target_id` names the intended target identity and is not a local
     filesystem path.
   - `target.local_path` points at the trusted local restore directory.

   Changing `target.local_path` must not change `target.target_id` unless the
   operator is intentionally switching targets and passes `--target-id`.

## Dry-Run Gate

```bash
go run ./cmd/supermover push --profile ./supermover.profile.json --dry-run
```

Continue only if:

- entry counts match the migration expectation closely enough to explain;
- warning count is reviewed;
- agent influence count is expected for the repository or home directory being
  moved;
- no operator expected runtime flags to override profile policy.

For JSON-style inspection, use the control-plane artifacts after a real run.
The current scan command prints text summaries.

## Publish Gate

```bash
SESSION_ID=local-$(date -u +%Y%m%dT%H%M%SZ)
go run ./cmd/supermover push --profile ./supermover.profile.json --session "$SESSION_ID"
```

Capture the printed session ID. If a fixed session ID is used for acceptance
tests, choose a clean target directory or inspect existing session artifacts
before rerunning.

## Post-Run Evidence Checklist

```bash
find /path/to/target/.supermover -maxdepth 4 -type f | sort
```

Required:

- profile snapshot exists under `.supermover/profiles/`;
- session receipt exists under `.supermover/sessions/<session>/receipt.json`;
- manifest exists under `.supermover/sessions/<session>/manifest.json`;
- warning records exist when the run reported warnings;
- agent influence record exists when the run reported influences.

Inspect the receipt:

```bash
sed -n '1,160p' /path/to/target/.supermover/sessions/<session>/receipt.json
```

Acceptance criteria:

- `status` is `published`;
- `profile_id` matches the profile used by the operator;
- `target_id` matches the intended target identity.

Inspect warnings:

```bash
find /path/to/target/.supermover/warnings -type f -name '*.json' -maxdepth 1 2>/dev/null | sort
```

Every warning must have an owner decision: accept, rerun with changed profile,
or block release.

## Recovery Procedure

`health` exposes the current read-only recovery classifier. It reports
interrupted or invalid local sessions and returns non-zero when operator action
is needed:

```bash
go run ./cmd/supermover health --profile ./supermover.profile.json
```

The public `recover` command is not implemented yet. Until it lands:

1. Preserve the target `.supermover` directory.
2. Preserve the exact profile file used for the failed run.
3. Record the command, session ID, stdout, stderr, and filesystem error.
4. Do not delete staged session state unless the operator has copied the
   evidence elsewhere.
5. Rerun only after deciding whether the prior session can be abandoned.

Planned command shape:

```bash
go run ./cmd/supermover recover --target /path/to/target --session <session-id>
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <session-id>
```

## Soft-Delete Procedure

Physical pruning must be a separate reviewed action. Operators should not infer
that a missing source file authorizes immediate target deletion.

Current review command:

```bash
go run ./cmd/supermover deleted list --profile ./supermover.profile.json
```

Planned physical-prune command shape:

```bash
go run ./cmd/supermover prune --target /path/to/target --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover prune --target /path/to/target --profile ./supermover.profile.json --apply
```

Required review evidence:

- source path;
- target path;
- session that detected the deletion;
- profile delete policy snapshot;
- approver and approval time, once approval support exists.

## Discovery And Pairing Procedure

Network discovery and pairing are not implemented in the current local push
slice. The intended sequence is:

```bash
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover discover
go run ./cmd/supermover pair --profile ./supermover.profile.json --target <address-or-advertisement-id>
go run ./cmd/supermover push --profile ./supermover.profile.json
```

Operational rule: discovery output is never an allowlist. It gives address
hints only. Trust begins after explicit pairing verification writes a receipt
and pins device identity.

## Incident Response

For any failed or suspicious run, collect:

- profile file used by the command;
- complete target `.supermover` directory;
- command line, stdout, stderr, and exit code;
- source and target filesystem type if a promotion or rename failed;
- warning files and session receipt for the affected session.

Do not "clean up" warnings, receipts, or manifests before triage. They are the
audit trail.
