# Operations Runbook

This runbook gives repeatable steps for a local push migration and the evidence
operators should preserve. It separates currently implemented commands from
planned mainline commands so acceptance work can proceed without implying
network features are complete. The read-only `report` command added by this
feature is an operator view over the profile SSOT and target control-plane
artifacts; it does not represent daemon, LAN agent, transport, or prune
completion.

## Release Gates

Run these gates for the current local/mounted migration slice before cutting a
release candidate:

```bash
go mod tidy -diff
go test -count=1 ./...
go test -race -count=1 ./...
go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
go vet ./...
staticcheck ./...
golangci-lint run ./...
git diff --check
go run ./cmd/supermover help
go run ./cmd/supermover version
go run ./cmd/supermover profile help
go run ./cmd/supermover push --help
go run ./cmd/supermover verify --help
go run ./cmd/supermover deleted help
go run ./cmd/supermover health --help
go run ./cmd/supermover report --help
go run ./cmd/supermover recover --help
```

The current release gate is local-only. Passing it means the wired CLI supports
profile-driven one-way migration to a trusted local or mounted target, plus
local audit, health, report, verify, deleted-review, and conservative recovery
surfaces. It does not mean LAN discovery, pairing, encrypted transport,
resumable network transfer, network daemon operation, privacy-preserving traffic
shape protection, drift review, status, or physical prune are implemented.

Keep separate future gates for the network slice. Do not mark these gates
passed until the commands are wired end to end and have their own validation
evidence:

```bash
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover discover
go run ./cmd/supermover pair --profile ./supermover.profile.json --target <address-or-advertisement-id>
go run ./cmd/supermover push --profile ./supermover.profile.json
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <network-session-id>
```

Future network acceptance must also prove authenticated pairing, encrypted
transport, resumable transfer across interruption, receiver-side recovery,
bounded metadata leakage for the selected privacy level, and audit artifacts
that distinguish discovery hints from trusted target identity.

## Manual Smoke

Use a disposable source and target so the smoke can exercise publication,
verification, soft-delete review, health, report, and recovery dry-run without
touching production data:

```bash
SMOKE_ROOT="$(mktemp -d)"
SRC="$SMOKE_ROOT/source"
DST="$SMOKE_ROOT/target"
PROFILE="$SMOKE_ROOT/supermover.profile.json"
SESSION="smoke-local"
mkdir -p "$SRC/subdir" "$DST"
printf 'hello\n' > "$SRC/subdir/file.txt"
printf 'hidden\n' > "$SRC/.hidden"

go run ./cmd/supermover profile init --profile "$PROFILE" --source "$SRC" --target "$DST"
go run ./cmd/supermover profile lint --profile "$PROFILE"
go run ./cmd/supermover push --profile "$PROFILE" --dry-run
go run ./cmd/supermover push --profile "$PROFILE" --session "$SESSION"
go run ./cmd/supermover verify --profile "$PROFILE" --session "$SESSION"
rm "$SRC/subdir/file.txt"
go run ./cmd/supermover push --profile "$PROFILE" --session "${SESSION}-delete"
go run ./cmd/supermover deleted list --profile "$PROFILE"
go run ./cmd/supermover health --profile "$PROFILE"
go run ./cmd/supermover report --profile "$PROFILE" --session "${SESSION}-delete" || test $? -eq 1
go run ./cmd/supermover recover --profile "$PROFILE" --dry-run
```

Expected smoke evidence:

- the profile is created and linted from the profile SSOT;
- the first push publishes files, including `.hidden`, to the local/mounted
  target;
- `verify` succeeds for the first session;
- the second push records the source-side deletion as a soft-delete review item;
- `deleted list` shows the deleted source path without physically pruning the
  target;
- `health` has no recovery work for a clean local run;
- `report` summarizes warnings, soft deletes, health, artifact, and verify
  state from the target `.supermover` evidence and returns non-zero because the
  smoke intentionally creates a soft-delete review item;
- `recover --dry-run` reports intended recovery actions without mutating target
  state.

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
- warning count is reviewed; full warning JSON is available only after a run is
  published;
- agent influence count is expected for the repository or home directory being
  moved;
- no operator expected runtime flags to override profile policy.

For JSON-style inspection before a real run, use
`go run ./cmd/supermover scan --profile ./supermover.profile.json --format json`.
After a real run, use the target control-plane artifacts. If the source scan
reports a `scan_error`, push is blocked before publish; fix source readability
and rerun the dry-run gate.

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
- soft-delete records exist when the run reported deleted paths;
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

Run the read-only aggregate report:

```bash
go run ./cmd/supermover report --profile ./supermover.profile.json
```

Use it to review the combined operator surface:

- warnings and profile suggestions;
- soft-delete records that need review;
- health and recovery issues;
- artifact problems such as missing or corrupt receipts, manifests, and
  profile snapshots;
- published-manifest verification state for the local push evidence.

The report is a summary over `.supermover` evidence. Preserve the underlying
profile snapshot, receipts, manifests, warnings, deleted records, and influence
records as the audit source of truth.

`report` exits non-zero when the generated report requires review, including
empty targets, warning records, soft deletes, recovery issues, artifact
problems, or verification findings. In shell scripts, capture stdout before
letting `set -e` abort so the review evidence is not lost.

Run verify and treat any non-zero result as a release blocker until explained:

```bash
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <session-id>
```

`verify` checks regular files for size, `sha256:` digest, permission mode, and
modification time. It also checks directory entries as plain directories and
symlink entries by `readlink` target. The command exits non-zero for warning
findings as well as error findings, artifact problems, and missing manifests.

## Recovery Procedure

`health` exposes the current read-only recovery classifier. It reports
interrupted or invalid local sessions and returns non-zero when operator action
is needed:

```bash
go run ./cmd/supermover health --profile ./supermover.profile.json
go run ./cmd/supermover report --profile ./supermover.profile.json
```

Use `health` for the focused recovery classifier and `report` for the broader
read-only operator aggregation. Neither command repairs state.

`recover` performs the conservative automated subset. It uses the profile SSOT
to find `target.local_path` and to write any repaired receipt.

```bash
go run ./cmd/supermover recover --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover recover --profile ./supermover.profile.json --session <session-id>
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <session-id>
```

For sessions that only reached `received` or `validated`, use an explicit
rollback decision:

```bash
go run ./cmd/supermover recover --profile ./supermover.profile.json --session <session-id> --rollback-incomplete
```

Preserve the target `.supermover` directory and command output before recovery.
If recovery reports `needs_repair`, do not delete staged session state; inspect
the manifest, receipt, target file, and `session.json` note.

## Soft-Delete Procedure

Physical pruning must be a separate reviewed action. Operators should not infer
that a missing source file authorizes immediate target deletion.

Current review command:

```bash
go run ./cmd/supermover deleted list --profile ./supermover.profile.json
go run ./cmd/supermover report --profile ./supermover.profile.json
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

Use `report` to confirm soft-delete records are visible in the same view as
warnings, health/recovery issues, artifact problems, and migration
verification state. Use `deleted list` for the itemized deletion review.

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
- `report` output when available, plus the artifacts it summarizes.

Do not "clean up" warnings, receipts, or manifests before triage. They are the
audit trail.
