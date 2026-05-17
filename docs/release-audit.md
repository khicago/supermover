# Release Audit

This document is the tracked audit surface for the current implementation
checkpoint. Detailed Bagakit runtime notes may exist under `.bagakit/`, but
that directory is intentionally ignored by Git; durable release evidence that
must survive repository checkout belongs in tracked docs. The current traffic
privacy level 2 evidence boundary is recorded in
`docs/traffic-privacy-level-2-verification.md`.

## Implementation Status

Current completed slice:

- Go CLI commands: `profile`, `scan`, `push`, `verify`, `deleted list`,
  `health`, `drift list`, `drift record`, persisted-record
  `drift acknowledge`, persisted-record `drift resolve`, `report`, `status`,
  `recover`, pairing `serve`/`pair` skeletons, low-information
  explicit-address `discover` hints, and non-mutating `prune --dry-run`
  candidate/refusal review.
- One-way local push from one source root to a trusted local or mounted target.
- Hidden files are included by default.
- Regular files are copied into session staging with source stability checks,
  SHA-256 manifests, and no-replace final publish. The separate `verify`
  command performs post-run restored-state verification.
- Publish is conservative: existing target files or symlinks are not overwritten
  unless they are already content-identical to the intended result.
- Profile JSON remains the SSOT; unsupported profile policies fail before push.
- Target `.supermover` records profile snapshots, transaction state, receipts,
  manifests, warnings, soft deletes, and agent influence records. Health is a
  read-only report over those artifacts.
- `report` is a read-only operator aggregation command.
  `report` aggregates warnings, profile suggestions, soft deletes,
  health/recovery issues, artifact problems, pairing evidence state, and
  published-manifest verification state at report time. It also runs the same
  read-only live target drift detector as `drift list` and exposes that result
  as independent report evidence, separate from persisted target-drift records.
- `drift list` is a read-only detector command over published manifest
  evidence and the profile-selected target filesystem. It supports optional
  session selection and text/JSON output, and returns review-required non-zero
  for drift, artifact problems, or no published manifest.
- `drift record` runs the live detector and persists current findings as
  durable `.supermover/drift/*.json` review records. It records evidence only:
  no repair, prune, suppression, background scan, or broad reconcile is
  implemented.
- `drift acknowledge` can review persisted drift records created by refused
  push attempts or by `drift record`.
- `drift resolve` can close persisted drift records created by refused push
  attempts or by `drift record`, but only after a fresh profile-scoped detector
  no longer reports drift for the same path and expected baseline. It writes
  review metadata only; it does not repair files, rewrite manifests, authorize
  prune, suppress future detector findings, or perform broad reconcile.
- `status` is a compact read-only current profile/target command over the
  profile SSOT, target control-plane artifacts, and target files needed for
  verification and live drift detection. It has text/JSON output and no session,
  target, policy, daemon, network, encrypted-transfer, sync, or repair override.
  It returns `0` for clean local evidence, `1` for generated review-required
  evidence, and `2` when no status report can be emitted.
- `recover` can replay safely staged local sessions, mark explicitly abandoned
  incomplete sessions as `rolled_back`, and mark non-automatable sessions as
  `needs_repair`.
- `prune --dry-run` exposes current help, flag parsing, profile loading,
  target-root resolution, profile delete-policy validation, and non-mutating
  candidate/refusal output over published soft-delete records. It does not write
  approvals or receipts, apply approvals, or delete files.
- `prune approve` writes durable approval artifacts plus profile snapshots from
  fresh dry-run evidence. It accepts `--approved-by` as an alias for
  `--reviewer`, optional `--expires-at <RFC3339>`, and `--format text|json`;
  it does not write prune receipts or delete files.
- `report` exposes read-only current profile/target prune approval evidence
  from `.supermover/prune/approvals/*.json`; `status` narrows that surface to
  counts and source breakdown, including authored-but-unapplied approval
  counts. This is audit evidence only; it does not author approvals, apply prune
  decisions, write receipts, delete files, make review-required targets clean,
  automatically release a migration, or close v1.
- `prune --apply --approval <id>` physically deletes approved file/symlink
  targets only over a durable approval artifact, after profile policy
  validation, path safety checks, a started receipt, and a current target-state
  recheck.
- `serve` exposes help, usage validation, target profile/root validation, and a
  low-information pairing HTTP listener that gates pairing bootstrap behind the
  target-console verification code. When the profile is already paired and has
  complete profile-selected receiver URL plus local TLS identity material,
  `serve` also mounts receiver upload routes on a pinned mutual-TLS listener.
  `pair` requires that verification code, writes local pairing
  receipt/profile-pin evidence, and does not start source-side transfer.
  `discover` can emit untrusted explicit address hints but does not run LAN
  browsing, authenticate encrypted peers, or transfer files.
- `push --network` is wired for non-dry-run profile-backed encrypted transfer.
  It validates profile policy, pairing receipt evidence, `network.receiver_url`,
  and `network.local_tls_identity`; loads the local TLS identity from the
  profile; connects to the profile-selected TLS 1.3 mTLS receiver; and drives
  `networkpush`/`networkrun`/`protocolclient`. Same-session reruns can resume
  from existing receiver committed offsets only when matching prior
  payload-overhead evidence remains auditable, and published-session reruns can
  retry commit idempotently without reuploading chunks while preserving prior
  published proof. Current automated evidence also covers the bounded
  profile-backed CLI/Runner source-interruption case where a same-session run
  reaches receiver begin, the receiver accepts payload bytes, the source-side
  network attempt is interrupted, in-flight `network-transfer.json` evidence may
  be persisted, and a same-profile, same-session rerun recovers from receiver
  status. f-22wnwd5pe/T-001 adds deterministic internal `networkrun` evidence
  for a source stop immediately after durable in-flight chunk progress evidence;
  the same-session rerun resumes from receiver status, merges the prior
  privacy-overhead evidence, and publishes matching target content. Zero-byte
  regular files are wired through explicit final empty completion evidence on this
  profile-backed path. The
  `push --network --dry-run` path is preflight-only and writes no target
  artifacts. This evidence stays profile-owned; it does not add endpoint,
  certificate, privacy, or recovery runtime overrides, and it does not make
  `recover` a network recovery command.
- Receiver protocol library supports resumable chunk upload, integrity checks,
  idempotent begin/commit, and conservative publish semantics.
- Internal TLS/mTLS transport adapters exist and now back the `serve` receiver
  mode. They
  configure TLS 1.3 mutual authentication, derive peer identity from leaf SPKI
  SHA-256 pins, check certificate validity, and inject TLS-derived authenticated
  peer context into receiver handlers. They are mounted by `serve` only when
  paired profile network material is complete. Non-dry-run `push --network`
  invokes the source-side TLS client configuration with the profile's local TLS
  identity and pinned target device ID.
- Network transfer attempts can now be represented by a validated
  `.supermover/sessions/<session>/network-transfer.json` outcome artifact when
  the `networkrun` wrapper has receiver-side artifact access after begin,
  including non-dry-run `push --network` sessions that reach receiver begin.
  Published artifacts prove the completed transfer and applied privacy overhead.
  `health`, `status`, and `report` can surface non-published outcomes such as
  `auth_refused`, `interrupted`, `needs_repair`, `publish_failed`, and damaged
  or mismatched transfer artifacts as review evidence. Dry-run network
  preflight writes no network session artifacts.
- The internal `networkrun` path requires its request privacy policy to match
  the caller-supplied profile policy before writing outcome evidence. The
  lower-level protocol client applies traffic privacy level 2 padding,
  batching, and bounded jitter from its transfer request policy. `networkrun`
  records the configured policy and applied overhead in
  `network-transfer.json` for `health` and `report` review. Non-dry-run
  operator `push --network` uses this path; dry-run does not.

Known planned surface:

- Operational LAN browsing beyond explicit address hints and
  verification-code pairing evidence.
- LAN browsing/agent daemon behavior, ongoing incremental sync, broad network
  resume acceptance, broad arbitrary interruption recovery, and arbitrary
  process-kill recovery acceptance. Current automated evidence is narrower:
  same-session receiver-status resume, idempotent published-session commit
  retry, and profile-backed CLI/Runner rerun after receiver-accepted payload
  bytes plus source-side network interruption. Network `recover` remains
  unwired.
- Operator-facing traffic privacy level 2 release acceptance beyond the current
  profile-backed network path. Protocol-client padding, batching, and bounded
  jitter overhead evidence is wired and documented for that path only. This is
  not anonymity; residual leakage includes total bytes, transfer duration, peer
  IP addresses, LAN presence, and Supermover use.
- Broader release workflow completion. `prune --dry-run`
  emits review-only candidate/refusal evidence over published soft-delete
  records. `prune approve` writes durable approval artifacts plus profile
  snapshots from fresh dry-run evidence without deleting files or writing prune
  receipts.
  `prune --apply --approval <id>` physically deletes approved file/symlink
  targets only after durable approval evidence, a started receipt, and a
  current target-state recheck. `prune review` now exposes focused read-only
  prune release inventory; `report` exposes approval evidence and `status`
  exposes approval counts/source breakdown as read-only audit evidence, but
  broader release workflow surfaces beyond that still need release acceptance.
- Broader automatic repair/reconcile command.
- Background scans and richer drift/prune workflow integration. `drift record`
  is the current durable detector-output write path. `drift acknowledge` and
  `drift resolve` are wired only for existing persisted target-drift records
  and are not repair or prune integration. `drift list`, `report`, and compact
  `status` expose live detector evidence read-only and do not mutate detector
  state.

Open-source governance now includes `LICENSE`, `SECURITY.md`,
`CONTRIBUTING.md`, and a GitHub Actions Go workflow.

## Commit Trail

- `84cc3fe feat(cli): add local migration push workflow`
- `8cf8758 feat(review): add verify and soft-delete review`
- `d1192c8 feat(network): add resumable receiver protocol library`
- `8b2b2fb fix(localpush): close source target alias gaps`
- `9088aff fix(publish): harden publish path and commit evidence`
- `8c668a4 fix(profile): separate target identity from local paths`
- `38a3a96 feat(health): expose read-only recovery diagnostics`
- `e5a8738 fix(compat): preserve legacy profile and manifest repair paths`
- `bb7dcf7 fix(safety): harden local migration publish evidence`
- `e350295 feat(recovery): add explicit local recover command`
- `4cfe63c fix(lint): remove stale local push helper`
- `64aec11 fix(recovery): verify staged payloads before recover publish`
- `9a88903 chore(governance): add open source project gates`
- `51906f9 fix(review): ignore unpublished artifacts in review`
- `4cb935c fix(audit): harden warning and control artifact edges`
- `c6be33d fix(control): reject trailing JSON documents`
- `8056164 fix(agentkb): unify default knowledge rules`
- `94649fd fix(cli): honor profile knowledge scan rules`
- `b233f79 fix(safety): harden migration publish and recovery`
- `a511f43 docs(audit): align operator guidance with recovery safety`
- `9ae3c59 docs(scope): clarify current migration slice boundaries`
- `d15bd70 feat(report): surface local pairing evidence state`
- `9438d40 feat(transport): add profile-pinned TLS channel adapter`
- `d5eb8f1 feat(networkrun): persist network transfer outcomes`
- `00967bc test(transport): cover TLS networkrun failure recovery`
- `959022a docs(audit): separate networkrun evidence from CLI scope`
- `cb16579 feat(privacy): surface level 2 leakage contract`
- `5da4c26 fix(privacy): correct report privacy contract`
- `1144718 feat(privacy): bind policy evidence to sessions`
- `7c4040e test(privacy): close audit gaps in level 2 evidence`
- `fcab254 feat(privacy): enforce level 2 padding evidence`
- `b8258bd fix(networkrun): retain padding evidence on warning write failure`
- `66a72d6 feat(privacy): add level 2 batching scheduler`
- `680faf3 docs(privacy): clarify wired level 2 batching scope`
- `0faf315 docs(privacy): finish level 2 batching doc cleanup`
- `2c814a8 feat(privacy): add level 2 bounded jitter evidence`
- `5543c21 fix(control): reject non-level2 jitter overhead`
- `5d5727e docs(agents): protect codex session evidence`

Current checkpoint: local migration publication and receiver store recovery now
cover process-level locks, no-replace file and symlink publish, shared symlink
target safety, published-artifact drift detection, and non-file verify checks.
Pairing evidence is now visible in `report`; TLS/mTLS transport adapters,
network transfer outcome artifacts, and TLS loopback failure/resume tests exist.
Internal traffic privacy level 2 padding, batching, and bounded jitter evidence
is wired for protocol-client/networkrun paths and visible in validated
artifacts, `health`, and `report`; non-dry-run operator `push --network` now
uses that profile-backed pinned TLS 1.3 mTLS path. f-22cnwwseh T-004 automated
evidence adds CLI-path resume behavior from an existing partial receiver
session using receiver status plus seeded prior payload-overhead evidence, and
idempotent rerun/commit retry for an already published network session.
f-22rnwcmaz/T-001 documents the additional bounded source-interruption gate:
profile-backed CLI/Runner same-session rerun after receiver begin,
receiver-accepted payload bytes, possible in-flight `network-transfer.json`
evidence, and a source-side network interruption, with retained/merged level 2
overhead evidence. LAN browsing, daemon behavior, ongoing incremental sync,
broad arbitrary interruption recovery, broad resume acceptance, arbitrary
process-kill recovery, anonymity, network `recover`, and operator-facing
receiver recovery UX remain unwired or not release-accepted.
Commit audit note for `b233f79`: lock-directory setup is intentionally performed
before the process file lock is acquired, with symlink-guarded directory
creation; the file lock protects subsequent session and target publication
mutations.
Commit audit note for `d15bd70`: review of the report pairing-evidence change
found no blockers; the remaining maintenance recommendation was to keep this
release-audit trail current.
Commit audit note for `2c814a8` and `5543c21`: f-228nws66k T-005 completed
the internal bounded jitter evidence slice and rejected impossible non-level-2
jitter overhead evidence, but f-228nws66k T-006 remains a release-boundary and
documentation gate, not an operator network-transfer completion claim.

## Release Gate Checklist

Use the runnable commands below for the current local/mounted migration release
gate. The cross-compile commands are expansion templates; replace the package
placeholder with concrete packages from `go list ./...` before treating them as
executed evidence. Record dated command output separately when cutting a release
candidate.

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
go run ./cmd/supermover scan --help
go run ./cmd/supermover push --network --help
go run ./cmd/supermover status --help
go run ./cmd/supermover drift help
go run ./cmd/supermover drift record --help
go run ./cmd/supermover recover --help
go run ./cmd/supermover prune --help
GOOS=windows GOARCH=amd64 go test -c <each package from go list ./...>
GOOS=aix GOARCH=ppc64 go test -c ./internal/filelock
GOOS=solaris GOARCH=amd64 go test -c ./internal/filelock
```

For a profile-backed local smoke, include
`supermover status --profile <path> [--format text|json]` after a successful
local `push`/`verify` and preserve the text or JSON output as read-only
profile/target evidence. A `status` smoke is not daemon, LAN,
encrypted-transfer, sync, or network privacy release evidence.

For a profile-backed network smoke, preserve `serve`, non-dry-run
`push --network --profile <path> --session <id>`, target `verify`,
`health`, `report`, and `status` output plus the receiver-side
`network-transfer.json`. Clean published transfer overhead is verified from
that artifact; `health`/`status`/`report` should be clean and may not print a
`network_transfer` issue line for the published session. The minimal smoke
proves the paired profile-backed mTLS transfer path only. f-22vnwgwjj
automated evidence is represented by `internal/cli/cli_test.go`
`TestPushNetworkReleaseSmokePublishesAndReportsViaCLI`, which runs profile lint,
CLI `serve`, non-dry-run `push --network`, target `verify`, `health`, compact
`status`, text `report`, JSON `status`, and JSON `report` in one fresh transfer
smoke. Its published-transfer overhead proof comes from direct receiver-side
`network-transfer.json` inspection, not from a required CLI-rendered
`network_transfer` issue line for clean published sessions.
f-22cnwwseh T-004 automated tests additionally prove same-session
receiver-status resume from a seeded partial receiver session with prior
payload-overhead evidence and idempotent published-session commit retry.
f-22rnwcmaz/T-001 release docs treat the newer CLI/Runner source-interruption
rerun coverage as a bounded gate only: receiver begin, accepted payload bytes,
possible in-flight network-transfer evidence, source-side network interruption,
same profile/session rerun, receiver-status recovery, nonzero resumed bytes,
clean health/status/report review, and preserved level 2 overhead evidence.
f-22wnwd5pe/T-001 adds deterministic `networkrun` source-stop-after-progress
acceptance with merged retry evidence and matching target content, but it still
does not close broad arbitrary interruption recovery, daemon restart, or
process-kill acceptance.
f-22wnwd5pe/T-002 adds a bounded acceptance matrix, not a broad recovery claim:
command-level evidence now covers receiver listener restart over preserved
target control-plane state, same-profile/same-session resume from receiver
status, clean post-retry `health`/`status`/`report`, and fail-closed
missing-prior-evidence behavior with `payload_overhead_missing`. Existing
networkpush evidence covers commit-only retry, published-session retry, and
bad-prior-evidence refusal. Arbitrary process kill, power loss, OS-daemon
restart recovery, network `recover`, automatic retry policy, broad reconcile,
and receiver recovery UX remain future work.
f-22dnw29gt T-004 updates the zero-byte network boundary: profile-backed
TLS/mTLS `push --network` now transfers zero-byte regular files through
explicit final empty completion in `protocolclient`, `networkpush`, and the CLI.
Receiver-side recovery UX, broad resume acceptance, LAN browsing, daemon
behavior, ongoing sync, arbitrary process-kill recovery, anonymity, and broader
privacy-product release acceptance remain separate gates until their tracked
tasks close.

Coverage is package-level and intentionally uneven: the CLI package exercises
flows through command integration tests, while lower-level packages carry the
bulk of behavior coverage. `cmd/supermover` has no direct tests because it is a
thin entrypoint over `internal/cli`.

f-22vnwgwjj privacy reporting and release-check acceptance is
intentionally scoped to release readiness for the current network evidence slice:
`profile lint`,
`health`, `status`, `report`, `docs/runbook.md`, and `docs/troubleshooting.md`
must preserve configured level, applied-overhead evidence when a
`network-transfer.json` exists, and residual leakage wording. `report` and
compact `status` now expose `traffic_privacy_acceptance`: it passes only for
profile-backed mTLS plus a clean published level 2 `network-transfer.json` whose
profile policy and device IDs match the configured profile and pairing receipt,
with applied padding, batching, and jitter counters. Otherwise it reports
blockers such as missing or mismatched evidence. The gate must also preserve the
release boundary that level 2 is bounded metadata reduction, not anonymity.
Current
`push --network --dry-run` requires profile-backed network material before it
emits a `transfer=dry_run` binding; it also validates the profile-selected
local TLS identity files and pins. It sends no files and writes no
`network-transfer.json`. Non-dry-run `push --network` writes receiver-side
`network-transfer.json` only after receiver begin stores a session. Zero-byte
regular files now use the same published transfer evidence path after the
protocol client sends an explicit final empty completion; a clean published
zero-byte transfer is not a review issue by itself. Transport setup failures
and begin-auth refusal can still occur before any receiver-side
network-transfer artifact exists.

f-22dnw29gt T-004 zero-byte completion evidence is represented by
`internal/protocolclient/client_test.go`,
`internal/networkpush/networkpush_test.go`, and `internal/cli/cli_test.go`
covering explicit final empty completion through the protocol client,
networkpush, and CLI path. f-22cnwwseh T-004 automated evidence is represented by
`internal/cli/cli_test.go` covering
operator `push --network` resume from an existing partial receiver session with
seeded prior payload-overhead evidence, `resume_authority=receiver_status`,
`resume_outcome=resumed`, nonzero `resumed_bytes`, published transfer status,
commit stage, applied privacy overhead, and clean `health`/`status`/`report`
review; and by `internal/networkpush/networkpush_test.go` covering
receiver-status resume after receiver restart plus published-session commit
retry without chunk reupload. Receiver-status retries that imply previously
accepted payload bytes require matching prior payload-overhead evidence.
Published zero-payload retries preserve the prior published payload overhead
artifact. If the needed prior artifact is missing, corrupt, mismatched,
non-published where a published retry is required, or lacks payload
padding/batching counters, the run records `needs_repair` with
`payload_overhead_missing` and blocks release instead of creating a clean
published privacy proof. The additional bounded CLI/Runner source-interruption
rerun gate covers only same-profile, same-session rerun after receiver begin,
receiver-accepted payload bytes, possible in-flight network-transfer evidence,
and source-side network interruption. It is not evidence for LAN discovery,
daemon workflow, ongoing sync, broad arbitrary interruption/restart acceptance,
network `recover`, arbitrary process-kill recovery, or anonymity.

## Safety Notes

- The current migration-ready path assumes an empty trusted target or an
  idempotent rerun where existing target content already matches the source.
- Divergent existing target files are refused rather than overwritten.
- Soft-delete records are review markers only. Current commands delete target
  files only through `prune --apply --approval <id>` over a durable approval
  artifact; source absence alone never triggers physical deletion.
- `health` is read-only. It reports incomplete transactions and damaged
  published artifacts. It also reports target-drift records from refused local
  managed updates or `drift record`. Live detector output belongs to
  `drift list`, `report`, and `status`, not `health`.
  `health` also reports scoped network transfer outcome artifacts when present;
  this is operator evidence only and does not imply a fresh network attempt,
  completed transfer, daemon status, or broad encrypted-transfer readiness.
  `recover` is the explicit mutating command for the safe local subset.
- `drift list` is read-only. It derives the target from `--profile`, compares
  published manifest evidence to the target filesystem, and returns non-zero
  when drift, artifact problems, or no published manifest require review. It
  does not persist detector output, acknowledge or resolve drift, mutate review
  state, run background scans, repair drift, or prune drifted paths. When
  report generation succeeds, `report` runs the same live detector under an
  independent report surface; compact `status` exposes a current local
  profile/target summary over the same read-only evidence. Use `drift record`
  to persist current live detector findings as `.supermover/drift` review
  records. `drift acknowledge` can review records created by refused push or
  by `drift record`; `drift resolve` can close those records only after a fresh
  detector no longer reports the same path and expected baseline. Repair,
  prune integration, broad reconcile, and background-scan workflows remain
  unimplemented.
- `report` is also read-only. Its evidence source is the profile SSOT plus
  target `.supermover` control-plane artifacts and live detector evidence, and
  its presence does not imply a fresh network transfer, daemon status, drift
  repair, prune apply authorization, or drift-prune workflow support. It
  surfaces prune candidates, refusals, current-scope approval evidence,
  existing receipts, and non-applied receipt issues as audit evidence. Persisted
  `.supermover/drift/*.json` records remain the `target_drifts` evidence
  surface; JSON reports expose non-persisted detector observations separately
  as `live_target_drift` with summary counters such as `live_target_drifts` and
  `live_target_drift_artifact_problems`. Use `drift record` when current live
  detector findings must become persisted review records. `report` may expose
  pairing evidence states, but those states are audit signals only:
  `paired_receipt_valid` does not imply a transfer attempt, daemon status, or
  file-transfer completion.
  `report` may also expose `network_transfers` from
  `.supermover/sessions/<session>/network-transfer.json` when such artifacts
  exist; those artifacts are not written by `push --network --dry-run`.
  `report` returns non-zero when review is required, even when it successfully
  emits a parseable text or JSON report.
- Publish reconciliation is intentionally conservative. If interruption happens
  during final publish, `recover` can replay staged files that still match the
  manifest, accept already-published identical targets, or report
  `needs_repair`; operators should preserve the target and review the
  manifest/staging evidence before rerunning.
- `push --dry-run` exposes warning counts, not complete warning JSON. The full
  warning artifacts are written only after a published run can continue. Source
  scanner `scan_error` findings now block push before publish because they make
  source inventory and soft-delete evidence unreliable.
- `verify` returns non-zero for warning findings as well as error findings,
  persisted warning records, soft-delete records, unresolved persisted
  target-drift records, artifact problems, and missing manifests. It verifies
  regular file size, `sha256:` digest, permission mode, and modification time,
  and verifies directory/symlink presence and symlink targets.
