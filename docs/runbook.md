# Operations Runbook

This runbook gives repeatable steps for a local push migration and the evidence
operators should preserve. It separates currently implemented commands from
planned mainline commands so acceptance work can proceed without implying LAN
agent, OS-managed daemon, detached background process, or ongoing sync features
are complete. The read-only `report` and compact local
`status` commands are operator views over the profile SSOT and target
control-plane artifacts; they do not represent daemon lifecycle, LAN agent,
transport, or prune completion.

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
go run ./cmd/supermover drift help
go run ./cmd/supermover drift record --help
go run ./cmd/supermover drift acknowledge --help
go run ./cmd/supermover drift resolve --help
go run ./cmd/supermover reconcile --help
go run ./cmd/supermover report --help
go run ./cmd/supermover status --help
go run ./cmd/supermover recover --help
go run ./cmd/supermover daemon help
go run ./cmd/supermover prune --help
```

The current release gate is local-only. Passing it means the wired CLI supports
profile-driven one-way migration to a trusted local or mounted target, plus
local audit, health, report, compact status, verify, deleted-review,
drift recording, persisted drift acknowledgement, and conservative recovery
surfaces, plus narrow persisted-drift reconcile for selected missing-file repair
and resolve-noop cases. The
profile-backed non-dry-run
`push --network` path requires the separate network smoke and network recovery
evidence below. The foreground `daemon` lifecycle gate covers only
`.supermover/daemon` install/state/stop-intent/restart-intent and redacted
lifecycle-event evidence around the existing serve behavior. Passing the local
gate does not mean LAN browsing, OS
service-manager installation, detached background process management, ongoing
incremental sync, broad network resume acceptance, broad drift reconcile
workflow, background scanning, broad repair, or broader prune release workflow
automation is implemented. Current automated
network recovery evidence covers only the profile-backed same-session CLI/Runner
rerun path after receiver-accepted
payload bytes and a simulated transport failure, plus the earlier
receiver-status and published-session retry cases. That evidence is not
arbitrary process-kill recovery, general broad interruption/restart release
acceptance, or anonymity.
The command-surface gates for drift review are `drift record --help`,
`drift acknowledge --help`, `drift resolve --help`, and `reconcile --help`;
behavior evidence comes from automated drift-review/reconcile tests and from
disposable persisted-record smoke below when release operators need manual
proof.
The wired `prune --dry-run` command validates
profile path, flags, target root, and profile prune policy, then reads published
soft-delete records and emits review-only candidates, refusals, and artifact
problems without mutating target files or writing approval or receipt artifacts.
Records still inside `delete_policy.retention_days` are surfaced as
`retention_window_active` refusals. `prune approve` writes durable approval
artifacts plus profile snapshots from fresh dry-run candidate evidence without
deleting target files or writing prune receipts.
`prune --apply --approval <id>` is the only physical prune path: it writes a
started receipt before deletion, re-runs the current prune plan including
retention checks, and records applied/partial/failed status after target-state
rechecks when finalization succeeds. `report` is read-only and exposes prune
candidates, refusals, current-scope approval evidence, existing receipts, and
receipt issues alongside other local evidence, while `status` exposes compact
approval counts and source breakdown. `prune review` exposes the prune-only
release-review inventory without writing approvals, receipts, or target files;
none of these read-only commands applies prune decisions.
Include `supermover prune
--help` and `supermover prune review --help` in command-surface release gates,
a disposable `prune --dry-run` smoke for review evidence, a `prune review`
smoke for focused read-only release inventory, a `prune approve` smoke for
approval authoring, a `report` smoke before/after prune apply, and a separate
disposable `prune --apply --approval <id>` smoke when validating physical-prune
apply. `drift list`, `report`, `prune review`, and `status`
live detector surfaces stay read-only; use `drift record` to persist current
findings as `.supermover/drift` review records. `reconcile plan` can inspect
selected persisted drift IDs without mutation, and `reconcile apply` requires
selected IDs, explicit `--apply`, and `--reason` before the current narrow
missing regular-file repair or already-restored/absent resolve-noop path can
run. Protocol-client network runs have bounded level 2 padding, batching, and
timing jitter evidence.

The wired compact local status contract lives in `docs/status.md`. Include
`supermover status --profile <path> [--format text|json]` in local release
smoke only as a read-only local profile/target evidence check, not as daemon,
LAN, encrypted-transfer, or sync status.

Foreground daemon lifecycle evidence lives under `.supermover/daemon` and is
separate from compact migration `status`:

```bash
go run ./cmd/supermover daemon install --profile ./target.profile.json
go run ./cmd/supermover daemon status --profile ./target.profile.json
go run ./cmd/supermover daemon logs --profile ./target.profile.json --tail 20
go run ./cmd/supermover daemon run --foreground --profile ./target.profile.json
go run ./cmd/supermover daemon restart --profile ./target.profile.json --reason "operator restart"
go run ./cmd/supermover daemon stop --profile ./target.profile.json --reason "operator stop"
```

This gate proves profile-derived foreground lifecycle state, redacted lifecycle
events, stop intent, and restart intent only. `daemon restart` is consumed by a
running foreground daemon and restarts serve listeners in that same process. It
does not install launchd/systemd/Windows services, spawn a detached daemon,
supervise crash restart, browse LAN, watch files, or run ongoing sync.

Current manual network smoke can exercise pairing, receiver route wiring,
dry-run preflight, and non-dry-run profile-backed mTLS transfer. The narrow
same-session interruption-rerun gate is automated test evidence, not a separate
operator CLI failure-injection command. Receiver-route evidence requires
preserving `serve` stderr from a paired target profile that has complete
`network.receiver_url` and `network.local_tls_identity`; it should show receiver
routes enabled without treating discovery as trust:

```bash
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover discover
go run ./cmd/supermover pair --profile ./supermover.profile.json --target <address> --verification-code <code>
go run ./cmd/supermover push --network --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover push --network --profile ./supermover.profile.json --session <network-session-id>
```

Current `push --network --dry-run` is preflight-only. An unpaired or invalid
profile exits earlier with a profile/pairing diagnostic. A paired profile that
lacks `network.receiver_url` or complete `network.local_tls_identity` exits
earlier with a profile network-material diagnostic. Only after paired profile,
receipt evidence, network material, local TLS identity files and pins, scan,
and manifest shape validate does it emit
`transfer=dry_run encrypted_transfer=profile_backed_mtls_validated
resume=not_attempted resume_authority=not_attempted
resume_outcome=not_attempted`. It sends no files, contacts no receiver, and
writes no `network-transfer.json`.

Current non-dry-run `push --network` connects to the profile-selected pinned
TLS 1.3 mTLS receiver, streams through `networkpush`/`networkrun`/
`protocolclient`, resumes same-session uploads from receiver status offsets
when a compatible partial receiver session already exists and auditable prior
`network-transfer.json` proves the earlier accepted payload overhead, retries
commit idempotently for already published sessions, and writes receiver-side
`network-transfer.json` only after receiver begin stores a session. For
published network transfers, that artifact is the proof surface for transfer
status and applied privacy overhead. Zero-byte regular files are supported on
this profile-backed path through an explicit final empty completion record from
the source protocol client; a clean publish should produce the target file,
receipt, and `network-transfer.json` evidence like other published network
files. Transport setup failures and begin-auth refusal can still leave no
network-transfer artifact.
Command output keeps `resume=receiver_status` for compatibility and adds
`resume_authority=receiver_status` plus `resume_outcome=fresh`, `resumed`, or
`published_retry`. Only `resume_outcome=resumed` with nonzero `resumed_bytes`
is proof that a rerun uploaded remaining payload bytes. A zero-payload
`published_retry` preserves the prior published payload overhead evidence.
Partial receiver-status retries also require prior transfer evidence whenever
the receiver already has accepted payload bytes. If the needed prior
`network-transfer.json` is missing, corrupt, mismatched, non-published where a
published retry is required, or lacks payload padding/batching counters, the
rerun writes `needs_repair` with
`error_code=payload_overhead_missing` and blocks the network privacy release
claim instead of fabricating applied-overhead proof.

The current automated interruption-rerun gate is deliberately bounded. It uses
a profile-backed same-session network run, lets the receiver accept payload
bytes, simulates a transport failure, then reruns the same profile/session
through the CLI/Runner path. Passing evidence is `resume_authority=receiver_status`,
`resume_outcome=resumed`, nonzero `resumed_bytes`, a published
`network-transfer.json` with retained/merged privacy overhead, clean
`health`/`status`/`report` review, and matching source/target hashes. This is
not a manual operator process-kill smoke, not an OS-daemon restart workflow, and
not general broad release acceptance. f-22wnwd5pe/T-001 adds an internal
`networkrun` fixture for a source stop immediately after durable in-flight chunk
progress evidence is written; the same-session rerun resumes from receiver
status, merges prior privacy-overhead evidence, and publishes matching target
content.

Keep a separate future acceptance gate for the remaining network product
surfaces. Do not mark LAN browsing, OS service-manager daemon installation,
detached background process management, ongoing incremental sync, or broad
resume acceptance passed until they have command-level validation evidence:

```bash
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover discover
go run ./cmd/supermover pair --profile ./supermover.profile.json --target <address> --verification-code <code>
go run ./cmd/supermover push --network --profile ./supermover.profile.json --session <network-session-id>
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <network-session-id>
go run ./cmd/supermover report --profile ./supermover.profile.json --session <network-session-id>
```

### f-22vnwgwjj Traffic Privacy Level 2 Gate

The traffic privacy reporting gate (f-22vnwgwjj) covers the
profile-backed network path and release checklist, but it does not close traffic
privacy level 2 as an anonymity claim or as completion of
LAN/OS-daemon/incremental-sync/broad-resume product work. Automated release-smoke
coverage now exercises profile lint, CLI `serve`, non-dry-run `push --network`,
`verify`, `health`, compact `status`, text `report`, JSON `status`, JSON
`report`, and receiver-side `network-transfer.json` evidence for a fresh
profile-backed mTLS transfer. The bounded interruption-rerun gate additionally
exercises receiver-accepted payload bytes followed by simulated transport
failure and same-session CLI/Runner recovery. Remaining acceptance work includes
receiver-side recovery UX, broad resume acceptance, arbitrary process-kill
recovery, LAN browsing, OS-daemon behavior, ongoing sync, and manual
release-candidate evidence capture.

For this release boundary, use only currently wired surfaces:

```bash
go run ./cmd/supermover profile lint --profile ./source.profile.json
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover push --network --profile ./source.profile.json --dry-run --session <network-session-id>
go run ./cmd/supermover push --network --profile ./source.profile.json --session <network-session-id>
go run ./cmd/supermover verify --profile ./source.profile.json --session <network-session-id>
go run ./cmd/supermover health --profile ./source.profile.json
go run ./cmd/supermover status --profile ./source.profile.json
go run ./cmd/supermover status --profile ./source.profile.json --format json
go run ./cmd/supermover report --profile ./source.profile.json --session <network-session-id>
go run ./cmd/supermover report --profile ./source.profile.json --session <network-session-id> --format json
```

f-22vnwgwjj acceptance for the current slice is documentation, wired
operator smoke coverage, and evidence readiness:

- `profile lint` must reject level 2 profiles that omit required padding,
  batching, jitter, or low-information discovery settings.
- `health` and `report` must remain honest when no network transfer artifact
  exists. They must not imply encrypted transfer readiness or a completed
  network run.
- `report`, `status`, and `health` may surface non-published or invalid
  `network-transfer.json` artifacts as review issues. Clean published transfer
  overhead must still be preserved from the receiver-side artifact; these review
  commands do not need to print a `network_transfer` issue line for clean
  published sessions.
- `report` and compact `status` expose
  `traffic_privacy_acceptance`. A pass requires profile-backed mTLS plus a
  clean published level 2 `network-transfer.json` whose profile policy and
  device IDs match the configured profile and pairing receipt, with applied
  padding, batching, and jitter counters. Missing or mismatched evidence is a
  blocker, not an inferred pass.
- The automated fresh network release smoke is
  `TestPushNetworkReleaseSmokePublishesAndReportsViaCLI` in
  `internal/cli/cli_test.go`. It is an operator-facing CLI smoke, not an
  internal-only protocol-client assertion.
- The runbook must preserve the residual-leakage boundary: level 2 is bounded
  metadata reduction, not anonymity. Total bytes, transfer duration, peer IP
  addresses, LAN presence, and Supermover use remain observable.
- The profile remains the SSOT. Do not add CLI, environment, or one-off runtime
  privacy overrides for padding, batching, jitter, or traffic level.

Expected evidence snippets for a level 2 profile include:

```text
privacy policy=status=profile_contract_only ... traffic_level=2 ... claim=bounded_reduction_only ... residual_leakage=total_bytes,duration,peer_ip,lan_presence,supermover_use ... network_transfer=not_configured
privacy status=profile_contract_only ... traffic_level=2 ... overhead_status=not_applied ... local_push=traffic_shaping_not_applied ... network_transfer=not_configured
```

For a profile with complete network material and readable TLS identity files,
the profile-level line may instead report
`network_transfer=profile_backed_mtls_configured`. In both cases, applied
overhead is proven by `network-transfer.json`, not by profile policy alone.
`traffic_privacy_acceptance status=blocked ... blockers=applied_overhead_missing`
is expected until a published transfer artifact with applied overhead exists.
After a clean profile-backed level 2 transfer, `status` and `report` should
show:

```text
traffic_privacy_acceptance status=passed ... anonymity_claim=not_claimed ... observed_padding_bytes=<n> ... observed_jitter_budget_millis=<n>
```

When a non-published, failed, damaged, or otherwise review-required
`networkrun` artifact from non-dry-run `push --network` or another network
runner exists, `report` may show applied overhead fields such as:

```text
network_transfer session=<session> ... privacy_level=2 ... privacy_padding_bytes=<n> ... privacy_batch_frames=<n> ... privacy_jittered_requests=<n> ... privacy_overhead_jitter_budget_millis=<n>
```

Do not expect the `network_transfer` line from
`push --network --dry-run`; preflight writes no network session artifact. Also
do not require that line for a clean published transfer: clean published
network transfer overhead is proven from the receiver-side
`.supermover/sessions/<session>/network-transfer.json`, while
`health`/`status`/`report` should stay clean.

The network outcome artifact for non-dry-run attempts that reach `networkrun`
is:

```text
<target>/.supermover/sessions/<session>/network-transfer.json
```

When a real network attempt is run, the release gate must preserve
command output and exit code for every attempt. Preserve
`network-transfer.json` when the attempt reached a receiver session and
produced one; otherwise record its absence explicitly. Receiver-side artifacts
such as the session receipt, manifest, warning records, and
`network-session.json` should be preserved when present; for pre-begin failures
such as `auth_refused` or transport setup failure, record their absence instead
of treating absence as cleanup permission. Then review `health --profile ...`
and `report --profile ... --session <network-session-id>` for
`network_transfers`, artifact problems, pairing state, applied privacy policy,
applied overhead, and residual-leakage notes. The local push release gate does
not require `network-transfer.json` because local push does not use the network
runner.

The automated bounded interruption-rerun gate covers one CLI/Runner recovery
shape: a level 2 profile-backed same-session network transfer with no runtime
privacy overrides, receiver-accepted payload bytes, simulated transport failure,
and same-session CLI/Runner rerun recovery. Additional f-22wnwd5pe/T-001
internal evidence covers a deterministic `networkrun` source stop after durable
in-flight chunk progress evidence and a same-session receiver-status resume.
Treat the following as automated
gate output to preserve or summarize, not as a manual operator
failure-injection recipe:

- the profile used by `profile lint`;
- `health` output before and after recovery/resume;
- `report` output for the network session;
- `.supermover/sessions/<session>/network-transfer.json`;
- receiver-side receipt, manifest, warning, and network-session artifacts when
  they exist;
- command stdout, stderr, and exit codes for the original attempt, simulated
  transport failure, same-session rerun, and report review.

That bounded gate passes only when the preserved evidence shows configured
level 2 policy, applied padding, batching, and jitter overhead,
receiver-status resume for the same session, no claim of anonymity, and
explicit residual leakage for total bytes, transfer duration, peer IP
addresses, LAN presence, and Supermover use.

The f-22wnwd5pe/T-002 acceptance matrix adds command-level receiver restart and
fail-closed evidence to that bounded gate. Treat the following as current
supported evidence only when the same profile, same session, and same
profile-selected target control plane are used:

- source/network interruption after receiver-accepted payload bytes followed by
  `resume_outcome=resumed` and nonzero `resumed_bytes`;
- receiver listener restart over preserved target state followed by the same
  same-session resume evidence;
- commit-only and published-session retry with no chunk reupload and preserved
  prior payload-overhead evidence;
- missing, corrupt, mismatched, non-published, or payload-empty prior
  `network-transfer.json` evidence blocked as `needs_repair` with
  `error_code=payload_overhead_missing`.

Keep a separate future/manual gate for broad interruption/restart release
acceptance. That later gate must cover the intended operator workflow beyond
the simulated transport-failure seam, including receiver-side recovery UX,
arbitrary process-kill or process restart scenarios, and any manual
release-candidate evidence capture. Network `recover`, daemon/OS-service
restart recovery, power-loss recovery, automatic retry policy, and broad
reconcile integration remain unwired. An explicit non-resumable refusal is
useful diagnostic evidence, but it keeps broad resume acceptance blocked.

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
BIN="$SMOKE_ROOT/supermover"
mkdir -p "$SRC/subdir" "$DST"
printf 'hello\n' > "$SRC/subdir/file.txt"
printf 'hidden\n' > "$SRC/.hidden"

go build -o "$BIN" ./cmd/supermover
"$BIN" profile init --profile "$PROFILE" --source "$SRC" --target "$DST"
"$BIN" profile lint --profile "$PROFILE"
"$BIN" push --profile "$PROFILE" --dry-run
"$BIN" push --profile "$PROFILE" --session "$SESSION"
"$BIN" verify --profile "$PROFILE" --session "$SESSION"
"$BIN" status --profile "$PROFILE"
rm "$SRC/subdir/file.txt"
"$BIN" push --profile "$PROFILE" --session "${SESSION}-delete"
"$BIN" deleted list --profile "$PROFILE"
"$BIN" health --profile "$PROFILE"
"$BIN" report --profile "$PROFILE" --session "${SESSION}-delete" || test $? -eq 1
"$BIN" status --profile "$PROFILE" || test $? -eq 1
"$BIN" recover --profile "$PROFILE" --dry-run
```

Expected smoke evidence:

- the profile is created and linted from the profile SSOT;
- the smoke uses a built binary, not `go run`, for commands where the exact
  application exit code matters;
- the first push publishes files, including `.hidden`, to the local/mounted
  target;
- `verify` succeeds for the first session;
- the second push records the source-side deletion as a soft-delete review item;
- `deleted list` shows the deleted source path without physically pruning the
  target;
- `health` has no recovery work for a clean local run;
- `report` summarizes warnings, soft deletes, prune candidates/refusals,
  existing prune receipts, health, artifact, and verify state from the target
  `.supermover` evidence and returns non-zero because the smoke intentionally
  creates a soft-delete review item;
- the first `status` call returns zero for the clean profile-selected local
  target, and the post-delete `status` call emits review-required evidence and
  returns `1`;
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
   - `privacy_policy.padding_bucket_bytes`, `batch_max_bytes`,
     `batch_max_count`, and `jitter_budget_millis` are non-zero for traffic
     level 2.
   - `privacy_policy.discovery_low_info` is true for traffic level 2.
   - traffic level 2 is a schema/profile gate only in current CLI local push.
     Protocol-client network runs, including non-dry-run `push --network`,
     apply padding, batching, and bounded timing jitter, but this does not mean
     anonymity or LAN/daemon/incremental-sync support is wired.
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
- `network-transfer.json` exists for non-dry-run network attempts that reach
  receiver begin and store a session; it is not expected for local push
  sessions, network dry-run preflight, or pre-begin network failures. When
  present, inspect `privacy_policy` and
  `privacy_overhead` to compare configured level 2 bounds with applied
  padding, batching, and bounded jitter overhead.

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
- network transfer outcome artifacts, when present, including `auth_refused`,
  `interrupted`, `needs_repair`, `publish_failed`, and corrupted or mismatched
  `network-transfer.json` evidence;
- pairing state: `unpaired`, `paired_receipt_valid`, or receipt
  mismatch/missing/invalid states requiring review;
- encrypted transfer and network-transfer evidence for non-dry-run
  `push --network` sessions, and `not_wired`/absent evidence only for surfaces
  that remain unwired or attempts that did not reach receiver begin and a stored
  session;
- published-manifest verification state for the local push evidence;
- live target drift detected from the selected published manifest and target
  filesystem at report time.

For f-22cnwwseh T-004 network sessions, review `health`, compact `status`, and `report`
together. A published `network-transfer.json` with matching receipt/session
state proves the completed transfer and applied privacy overhead. Non-published
or damaged network transfer artifacts remain review evidence, not a completed
transfer claim, and can surface through those read-only commands.

The report is a summary over `.supermover` evidence plus read-only live target
drift detection. Preserve the underlying profile snapshot, receipts, manifests,
warnings, deleted records, prune approvals/receipts, and influence records as
the audit source of truth.
JSON reports expose live detector output under `live_target_drift`, with
summary counters such as `live_target_drifts` and
`live_target_drift_artifact_problems`. This live report evidence is read-only;
use `drift record` when current findings must be written to
`.supermover/drift/*.json` for review. `drift resolve` can close existing
persisted records only after target restoration makes the same path and
expected baseline clean under a fresh detector. `reconcile plan/apply` can
repair only selected persisted missing-regular-file drift from matching
published/source evidence, or resolve already-restored/absent persisted records;
it does not broadly reconcile or prune drift.
JSON reports expose physical-prune review evidence under `prune_review`:
pending candidates, refusals such as already-missing targets, current-scope
approval evidence, receipts, and non-applied receipt issues. Approval evidence
is read from durable `.supermover/prune/approvals/*.json` artifacts scoped to
the current profile/target. `prune review` exposes that prune inventory as a
focused read-only release-review surface, while `status` exposes only the
related counts and source breakdown. This evidence helps review
authored-but-unapplied approvals; it does not author approvals, supersede
approvals, apply prune decisions, write receipts, delete files or symlinks,
repair/reconcile drift, make the target clean, automatically release a
migration, or close v1.

`report` exits non-zero when the generated report requires review, including
empty targets, warning records, soft deletes, prune candidates, prune refusals,
non-applied prune receipts, recovery issues, artifact problems, verification
findings, live target drift, or pairing receipt/profile mismatches. In shell
scripts, capture stdout before letting `set -e` abort so the review evidence is
not lost.

Run verify and treat any non-zero result as a release blocker until explained:

```bash
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <session-id>
```

`verify` checks regular files for size, `sha256:` digest, permission mode, and
modification time. It also checks directory entries as plain directories and
symlink entries by `readlink` target. The command exits non-zero for warning
findings as well as error findings, persisted warning records, soft-delete
records, unresolved persisted target-drift records, artifact problems, and
missing manifests.

Run target drift review when reviewing a published target:

```bash
go run ./cmd/supermover drift list --profile ./supermover.profile.json
go run ./cmd/supermover drift list --profile ./supermover.profile.json --session <session-id> --format json
go run ./cmd/supermover drift record --profile ./supermover.profile.json --session <session-id> --format json
go run ./cmd/supermover drift resolve --profile ./supermover.profile.json --id <persisted-drift-id> --reason "<target restored reason>" --format json
go run ./cmd/supermover reconcile plan --profile ./supermover.profile.json --id <persisted-drift-id> --format json
go run ./cmd/supermover reconcile apply --profile ./supermover.profile.json --id <persisted-drift-id> --apply --reason "<operator repair reason>" --format json
```

`drift list` derives the target from the profile only and compares published
manifest evidence to the target filesystem. It exits non-zero for drift,
artifact problems, or no published manifest. It does not persist detector
output, acknowledge or resolve drift, mutate review state, run background scans,
repair drift, or prune drifted paths. When report generation succeeds, `report`
runs the same live detector under its independent report surface; compact
`status` now exposes a read-only current profile/target summary over the same
local evidence. `drift record` persists current live detector findings as
durable `.supermover/drift` records; it does not acknowledge, repair, prune,
suppress later live findings, run background scans, or broadly reconcile.
`drift resolve` closes an existing persisted record only after a fresh detector
no longer reports drift for the same path and expected baseline. The `status`
contract is current-target only, has no `--session` flag,
and keeps `report --session` as the historical report surface. Use
`--format json` for automation or durable audit capture; text output is compact
operator review output with target-controlled values escaped.
`reconcile plan` is non-mutating and derives source and target only from the
profile. `reconcile apply` accepts persisted IDs only and additionally requires
`--apply` and `--reason`; it has no `--target` or `--state-dir` override.
Current apply support is limited to missing regular-file restores from
published manifest evidence and the current source file, plus resolving
already-restored or already-absent persisted records.

To record operator acknowledgement for an existing persisted target-drift
record, first capture a persisted drift ID from refused-push `verify`/`report`
JSON or from `drift record` output.
For a disposable release smoke, create a managed-file target drift through the
wired local push path rather than hand-writing `.supermover` artifacts:

```bash
SMOKE_ROOT="$(mktemp -d)"
SRC="$SMOKE_ROOT/source"
DST="$SMOKE_ROOT/target"
PROFILE="$SMOKE_ROOT/supermover.profile.json"
BIN="$SMOKE_ROOT/supermover"
mkdir -p "$SRC" "$DST"
printf 'old\n' > "$SRC/file.txt"

go build -o "$BIN" ./cmd/supermover
"$BIN" profile init --profile "$PROFILE" --source "$SRC" --target "$DST"
"$BIN" push --profile "$PROFILE" --session drift-smoke-one

printf 'new\n' > "$SRC/file.txt"
printf 'operator target edit\n' > "$DST/file.txt"
if "$BIN" push --profile "$PROFILE" --session drift-smoke-two; then
  echo "expected target drift refusal" >&2
  exit 1
fi

if "$BIN" verify --profile "$PROFILE" --session drift-smoke-two --format json > "$SMOKE_ROOT/verify-target-drifts.json"; then
  echo "expected verify to report target_drift evidence" >&2
  exit 1
fi
```

The failed second push leaves a scoped persisted drift artifact under
`$DST/.supermover/drift/`; the ID appears in the saved
`verify-target-drifts.json` `target_drifts` array. The persisted record belongs
to the refused attempt (`session_id=drift-smoke-two`) and carries the published
baseline it compared against in `expected.session_id` and
`expected.manifest_id`; acknowledgement rechecks that published baseline before
writing review metadata. Extract the drift ID manually from the JSON, or with a
local JSON parser:

```bash
DRIFT_ID="$(
  python3 - "$SMOKE_ROOT/verify-target-drifts.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    report = json.load(f)
print(report["target_drifts"][0]["id"])
PY
)"
```

Then acknowledge that persisted ID:

```bash
"$BIN" drift acknowledge --profile "$PROFILE" --id "$DRIFT_ID" --reason "disposable release smoke" --reviewer "release-smoke" --format json
```

Use a persisted ID from refused-push `target_drifts` or from `drift record`
output, not a live-only ID from `drift list` or `report.live_target_drift`.
`drift acknowledge` derives the target from the profile, requires a reason,
rechecks the persisted record against the published receipt/manifest/root
evidence named by its expected baseline, and writes acknowledgement metadata
only. It does not repair target files, resolve drift, suppress live
detector findings, authorize prune, or make `verify`, `report`, `health`, or
`status` clean.

To close the same disposable persisted record, restore the target file to the
published baseline and run `drift resolve`:

```bash
printf 'old\n' > "$DST/file.txt"
"$BIN" drift resolve --profile "$PROFILE" --id "$DRIFT_ID" --reason "target restored for release smoke" --reviewer "release-smoke" --format json
```

`drift resolve` rechecks the persisted record against the published
receipt/manifest/root evidence and runs a fresh live detector before writing
`review_state=resolved`. It does not repair target files, rewrite manifests,
authorize prune, suppress future live detector findings, or perform broad
reconcile.

For a persisted missing-file record whose source still matches the published
manifest evidence, the narrow reconcile flow can plan and apply repair:

```bash
"$BIN" reconcile plan --profile "$PROFILE" --id "$DRIFT_ID" --format json
"$BIN" reconcile apply --profile "$PROFILE" --id "$DRIFT_ID" --apply --reason "restore missing regular file from source evidence" --reviewer "release-smoke" --format json
```

Use `reconcile plan` first because it is non-mutating and shows the selected
action/refusal evidence. Use `reconcile apply` only with selected persisted
drift IDs; it refuses live-only IDs, missing intent, missing reason, source or
published evidence mismatch, unsafe paths, and unsupported drift classes. The
current output receipt is command output only, not a durable repair receipt
under `.supermover`.

## Recovery Procedure

`health` exposes the current read-only recovery classifier and local target
review state. It reports interrupted or invalid local sessions, damaged control
artifacts, target-drift records from refused managed updates or `drift record`,
and scoped network transfer outcome artifacts when they exist. The package-level
drift detector exposed by `drift list` is not a `health` scan. It
returns non-zero when operator action is needed:

```bash
go run ./cmd/supermover health --profile ./supermover.profile.json
go run ./cmd/supermover report --profile ./supermover.profile.json
```

Use `health` for the focused recovery classifier and `report` for the broader
read-only operator aggregation. Neither command repairs state. Target-drift
records here are persisted review evidence surfaced through existing review
commands. `health` does not run the live detector; `report` does, under its
independent read-only live drift surface.
Network transfer statuses such as `auth_refused`, `interrupted`,
`needs_repair`, `publish_failed`, and `failed` require operator review and
retry planning; the current `recover` command is not a full network repair
command.

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

Current prune dry-run review surface:

```bash
go run ./cmd/supermover prune --help
go run ./cmd/supermover prune --profile ./supermover.profile.json --dry-run
```

`prune --dry-run` keeps the profile as the SSOT, accepts only the command flags
defined by the CLI, requires `delete_policy.mode: prune`,
`delete_policy.require_review: true`, and
`delete_policy.allow_physical_prune: true`, and reads published soft-delete
records for the selected target. Output includes `schema:
supermover.prune_dry_run.v1`, profile/target IDs, the profile delete policy,
candidate/refusal counts, candidate soft-delete evidence, previous manifest
evidence, current target state, and artifact problems. Candidates require
operator review and return exit `1`; an empty clean dry-run returns exit `0`.
Refusals and artifact problems also return exit `1`. Soft deletes inside
`delete_policy.retention_days` are reported as `retention_window_active`
refusals until `detected_at + retention_days * 24h`. The dry-run command does
not delete target files, write approval records, apply an approval, or write
prune receipts.

Current prune approval-authoring surface:

```bash
go run ./cmd/supermover prune approve --profile ./supermover.profile.json --id <approval-id> --soft-delete <soft-delete-id> --reason "reviewed for prune" --reviewer "release-smoke" --format json
```

`prune approve` requires an approval ID, at least one `--soft-delete` ID, a
reason, and reviewer identity. `--approved-by` is an alias for `--reviewer`,
`--expires-at <RFC3339>` can set expiry, and `--format text|json` controls
output. The command reuses fresh dry-run evidence, writes
`.supermover/prune/approvals/<approval-id>.json` plus profile snapshot evidence,
and does not delete target files or write prune receipts. It writes approvals
only when the fresh dry-run has no refusals or artifact problems, and selected
IDs must be current dry-run candidates.

Current reviewed physical-prune apply surface:

```bash
go run ./cmd/supermover prune --profile ./supermover.profile.json --apply --approval <approval-id>
```

Apply requires an existing approval artifact at
`.supermover/prune/approvals/<approval-id>.json`. The approval must bind the
current profile ID, target ID, root ID, approved soft-delete items, current
profile delete policy, profile snapshot ID, and profile snapshot digest. Use
`prune approve` to create that artifact, then keep the approval JSON with the
release evidence.

`prune --apply --approval <id>` writes a durable started receipt before any
target mutation, takes the same target-wide lock used by local push, re-runs
the current prune plan including retention checks, rejects symlinked
approval/receipt/snapshot artifacts, rechecks each approved target immediately
before deletion, deletes only approved file/symlink targets through a
target-root confined filesystem handle whose opened identity must still match
the original target root, syncs the parent directory, and records
applied/partial/failed status in
`.supermover/prune/receipts/<id>.json` when finalization succeeds. If final
receipt writing is interrupted, the durable `started` receipt remains the
operator review evidence. Status `applied` exits `0`; `partial`, `failed`,
interrupted `started`, or invalid approval state exits non-zero and
requires receipt inspection before retry.

Required review evidence:

- source path;
- target path;
- session that detected the deletion;
- previous session and manifest evidence;
- current target state;
- refusal reason when unsafe;
- profile delete policy snapshot;
- approver and approval time.

Use `report` to confirm soft-delete records are visible in the same view as
warnings, health/recovery issues, artifact problems, prune candidates,
refusals, current-scope approval evidence, existing prune receipts, receipt
issues, and migration verification state. `report` returns non-zero when prune
candidates, refusals, authored-but-unapplied approvals, failed, partial, or
interrupted receipts still require operator review. A listed approval is not
physical prune authorization by itself; `prune --apply --approval <id>` still
performs the current policy, profile-snapshot, soft-delete, manifest, expiry,
target-identity, and target-state checks before mutation. An applied receipt is
audit evidence for an existing approval, not automatic release evidence. Use
`deleted list` for the itemized deletion review.

## Discovery And Pairing Procedure

LAN browsing is not available in the current slice. Profile-backed source-side
encrypted transfer is available through non-dry-run `push --network`. The
foreground `daemon` command can persist install/status/log/restart/stop
lifecycle evidence and run the same serve behavior under
`daemon run --foreground`; it is not LAN browsing, an OS service manager, crash
supervision, a detached background process, or ongoing sync.
The
`serve` command validates the target profile/root and, for valid pairing-only
profiles, binds a low-information pairing listener: it
exposes discovery, returns pairing bootstrap only after the target-console
verification code is presented, and keeps pairing output untrusted. When the
profile is already paired and has complete `network.receiver_url` plus
`network.local_tls_identity`, `serve` also binds the receiver URL from the
profile and mounts receiver upload routes over pinned mutual TLS. Paired partial
receiver material fails closed before any listener reports ready. `pair`
requires that verification code before writing a local
pairing receipt, profile pins, and profile snapshot. `discover` can emit
untrusted explicit address hints with `--address`; with no source configured it
waits for the requested timeout and returns no hints. `--address` is
operator-provided hint material and still exposes peer address metadata.
The current trust-skeleton sequence is:

```bash
go run ./cmd/supermover serve --profile ./target.profile.json
go run ./cmd/supermover discover --address 127.0.0.1:9000
go run ./cmd/supermover pair --profile ./supermover.profile.json --target <address> --verification-code <code>
go run ./cmd/supermover push --network --profile ./supermover.profile.json --dry-run
go run ./cmd/supermover push --network --profile ./supermover.profile.json --session <network-session-id>
```

Operational rule: discovery output is never an allowlist. It gives address
hints only. Pairing evidence begins after explicit verification writes a receipt
and pins device identity. The current source-side dry-run preflight is:

```bash
go run ./cmd/supermover push --network --profile ./supermover.profile.json --dry-run
```

Pairing report state is not transfer authorization by itself: a valid pairing
receipt only proves the profile pins and receipt agree. `serve` can use that
evidence plus profile-selected receiver URL and TLS identity material to mount
authenticated receiver routes. `push --network` reads the same
profile/pairing/network material, refuses unpaired profiles, mismatched
profiles, and paired profiles that lack that network material. Dry-run stops
there without contacting the receiver. Non-dry-run connects to the pinned TLS
1.3 mTLS receiver, transfers files through the protocol client, and writes
network transfer evidence after receiver begin stores a session. Same-session
reruns can use receiver status as the resume authority for already committed
bytes only when prior network-transfer evidence is auditable, and
published-session reruns retry commit idempotently without reuploading chunks
when the receiver reports the session complete. This is a bounded operator
`push --network` retry behavior, not LAN discovery, daemon sync, or arbitrary
process-kill recovery.

## Incident Response

For any failed or suspicious run, collect:

- profile file used by the command;
- complete target `.supermover` directory;
- command line, stdout, stderr, and exit code;
- source and target filesystem type if a promotion or rename failed;
- warning files and session receipt for the affected session;
- `health --profile ...` output for the affected target when available;
- `.supermover/sessions/<session>/network-transfer.json` when a network attempt
  artifact exists;
- `report` output when available, plus the artifacts it summarizes.

Do not "clean up" warnings, receipts, or manifests before triage. They are the
audit trail.
