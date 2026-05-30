# Capability Contract: Native macOS App-First LAN Workbench

Feature: `f-23bnwxry2`
Task: `T-001`
Status: execution contract

## Purpose

This contract freezes the first executable boundary for the native macOS
workbench. The app may feel like a control room, but every action must map to
one of these sources:

- profile SSOT data
- wired `supermover` CLI command output
- structured event/run-state API added by this feature
- durable target `.supermover` control-plane artifact

If an action cannot map to one of those sources, the UI must show it as
`unavailable`, `not configured`, `not run`, `blocked`, or `planned`, not as a
successful or live state.

## Evidence Snapshot

- CLI command inventory: `go run ./cmd/supermover help`
- CLI role setup parent help: `go run ./cmd/supermover profile --help`
- CLI profile create help: `go run ./cmd/supermover profile init --help`
- CLI profile target help: `go run ./cmd/supermover profile set-target --help`
- CLI source scan: `go run ./cmd/supermover scan --help`
- CLI deleted review: `go run ./cmd/supermover deleted --help`
- CLI recovery: `go run ./cmd/supermover recover --help`
- CLI prune: `go run ./cmd/supermover prune --help`
- CLI reconcile: `go run ./cmd/supermover reconcile --help`
- CLI discovery: `go run ./cmd/supermover discover --help`
- CLI pairing: `go run ./cmd/supermover pair --help`
- CLI sync: `go run ./cmd/supermover sync --help`
- CLI daemon: `go run ./cmd/supermover daemon --help`
- CLI network push: `go run ./cmd/supermover push --network --help`
- Current app task surface: `macos/SuperMoverApp/AppStore.swift`
- Research synthesis: `.bagakit/researcher/topics/frontier/macos-app-first-lan-workbench/summaries/synthesis-plan-review.md`

Current app coverage is partial. `SuperMoverTaskKind` currently wraps profile
lint, local push/verify/status/report/health, recovery dry-run, drift actions,
selected reconcile/prune review commands, `push --network`, `serve`, daemon
status/logs, and dashboard. It does not yet wrap app-owned profile creation,
top-level source scan, deleted review, recovery rollback, discovery, pairing,
sync queue, sync run/loop/watch, sync network run/discover-run/loop, daemon
install/run/restart/stop, physical prune apply, role-scoped process
supervision, or Merkle/root comparison evidence.

Coverage terms:

- `missing`: no native app action exists yet.
- `wired`: native app can launch the command, but it may still fall short of
  this contract's safety, evidence-selection, process, or structured-output
  gates.
- `gate-compliant`: native app launch path, disabled states, evidence source,
  and copy all satisfy this contract. No current row is gate-compliant until a
  later task proves it.

Do not treat `wired` as product-ready app-first behavior.

## Role Model

| Role | Primary job | Allowed action classes | Must stay disabled until |
| --- | --- | --- | --- |
| Source | Select source profile/roots, discover/pair target, run dry-run and bounded transfer/sync, review verification evidence | profile init/lint/set-target, discover browse, pair, push dry-run, push network, sync queue/run/loop/watch/network, verify/report/status | profile path is valid, source roots are readable, target identity is paired/pinned for network transfer, operator session inputs are present |
| Target | Select target root/profile, serve pairing and receiver, advertise low-info hints, expose dashboard/evidence | profile init/lint/set-target, serve, discover advertise, dashboard, status/report/health, daemon foreground controls | target root is readable/writable, `.supermover` control-plane guard passes, listen address is valid, Local Network/firewall readiness is explained |
| Observer | Inspect target evidence without mutation | status/report/health, verify, dashboard, evidence browser, raw JSON | target evidence exists and selected profile/target root matches the inspected control plane |

Discovery output is never identity. Pairing receipt and profile pins are the
trust boundary.

## Command Coverage Matrix

| Area | App action | Wired CLI/API | Current app coverage | Required app contract | Disabled or unavailable states | Tests |
| --- | --- | --- | --- | --- | --- | --- |
| Profile | Create profile | `profile init --profile --source (--target \| --source-only) [--target-id] [--id] [--name]` | Wired for Source prepare | Source role writes a source-only profile through CLI after verifying only this Mac's source folder; Target role owns destination selection through `profile set-target` and app-side writable-path checks | Missing source path, existing profile path, unreadable source, accidental `--source-only`/`--target` mix, target path not yet completed before migration commands | CLI source-only/init conflict tests; Swift source-only profile init and target-owned set-target tests |
| Profile | Validate profile | `profile lint --profile` | Wired but not gate-compliant | Show privacy/profile policy and actionable lint failures; green readiness requires decode or durable profile evidence, not text-only optimism | Missing profile, lint failure, stale selected path | Existing command smoke; Swift decode/status test |
| Profile | Change target root | `profile set-target --profile --target [--target-id] [--name]` | Missing | Native target selector updates profile through CLI; app or new CLI preflight must verify safe/writable target; paired target-id rotation is refused | Paired profile with target-id override, unsafe target, missing target, target name/pin ambiguity | CLI contract test; Swift disabled-state test |
| Source scan | Scan source roots | `scan --profile --format json` | Missing | Read source-root inventory without writing target state; useful for setup confidence and dry-run explanation | Missing profile, unreadable source root, unsupported entry type | Go scan tests; Swift decode test |
| Deleted review | Review source-side deleted records | `deleted list --profile` | Missing | Read source-side soft-delete evidence as operator context; no pruning implication | Missing profile, no deleted evidence, stale session | CLI smoke; evidence browser fixture |
| Discovery | Browse explicit/LAN hints | `discover`, `discover browse --format json` | Missing | Source shows candidates as untrusted hints with packet/source metadata | No candidates, invalid packets, duplicate/ambiguous candidates, Local Network blocked | Loopback discovery smoke; duplicate refusal test |
| Discovery | Advertise target hint | `discover advertise --profile ... --format json` | Missing | Target can advertise low-info pairing/service capability while showing trust warning | Invalid profile, listen/dest failure, no Local Network permission | Loopback advertise/browse smoke |
| Pairing | Pair source to target | `pair --profile --target --verification-code --method` | Missing | Source enters target code, shows verification method, writes receipt/pins through CLI | Wrong code, timeout, untrusted discovery-only candidate, missing profile | Wrong-code fail-closed test; pairing receipt decode test |
| Receiver | Start target receiver/pairing service | `serve --profile --listen` | Wired but not gate-compliant as single active task | Target starts foreground service, exposes pairing code and receiver readiness as role task after T-003/T-004 process/event gates | Invalid profile, target root unavailable, network material incomplete, listen failure | Readiness event test; serve smoke |
| Receiver | Target dashboard | `dashboard --profile --listen` | Wired but not gate-compliant as single active task | Observer/target opens read-only loopback dashboard and labels it not LAN sync after T-003/T-004 process/event gates | Missing profile, no dashboard URL, target evidence missing | URL parse compatibility test; dashboard smoke |
| Daemon | Install service evidence | `daemon install --profile` | Missing | Show as foreground lifecycle setup, not OS service manager unless CLI changes | Unsupported detached service expectations | CLI smoke; copy honesty test |
| Daemon | Run foreground agent | `daemon run --foreground --profile [--listen]` | Missing | Role-scoped long-running process; may wrap serve and profile-enabled polling | Missing foreground flag, invalid profile, stale daemon status mistaken as liveness | Multi-process supervisor test |
| Daemon | Read status/logs | `daemon status/logs --format json` | Wired but not gate-compliant | Show durable lifecycle evidence separately from active subprocess state | Missing evidence, stale status, malformed logs | JSON fixture tests |
| Daemon | Restart/stop intent | `daemon restart/stop --reason` | Missing | Require explicit reason and show intent receipt/status | Empty reason, no active matching foreground process | Negative Swift test; CLI smoke |
| Local publish | Dry-run local migration | `push --profile --dry-run` | Wired but not gate-compliant | Preflight whole publish plan before mutation; green state requires parsed output or durable evidence | Missing profile, known conflicts, reserved `.supermover` target path | Go publish tests; app command preview test |
| Local publish | Publish local migration | `push --profile --session` | Wired but not gate-compliant | Require operator-entered session id; never auto-fill mutating session | Empty/invalid session, failed dry-run evidence if policy requires | Swift disabled-state test; CLI smoke |
| Recovery | Review local recovery | `recover --profile --dry-run --format json` | Wired but not gate-compliant as Recover Dry Run | Show local target control-plane recovery actions without network retry implications | Missing profile, no incomplete sessions, malformed recovery evidence | Existing recovery tests; app decode test |
| Recovery | Roll back incomplete local sessions | `recover --profile --rollback-incomplete --format json` | Missing | Mutating recovery action must require explicit operator confirmation and session scoping when available | Missing profile, empty confirmation, network-transfer expectation | Negative Swift test; CLI contract test |
| Network push | Network dry-run | `push --network --profile --dry-run --format json` | Wired but not gate-compliant | Validate pairing, TLS identity, receiver URL, scan, manifest shape without contacting receiver | Unpaired profile, missing TLS identity, invalid receiver URL | Existing CLI tests; app decode test |
| Network push | Bounded network push | `push --network --profile --session --format json` | Wired but not gate-compliant | Source foreground transfer; completion comes from JSON/durable receiver evidence; session must be operator-entered | Empty session, unpaired target, receiver not ready, failed preflight | Loopback transfer smoke; interruption/resume test |
| Sync queue | Enqueue/status/list/ready | `sync queue enqueue/status/list/ready --format json` | Missing | Queue-only screens distinguish evidence from execution | Missing profile, target control plane absent, malformed queue | Queue fixture tests; command matrix test |
| Sync queue | Cancel/fail queue entry | `sync queue cancel/fail --id --reason --format json` | Missing | Require selected entry id and explicit reason; no source/target file mutation | Empty reason, live-only entry, missing id | Negative Swift test; CLI contract test |
| Sync local | Bounded local sync pass | `sync run --profile --session --format json` | Missing | Execute one queue consumer pass; show done/idle/retrying from structured result | Empty session, queue absent, target unavailable | CLI contract test; app decode test |
| Sync local | Foreground local loop | `sync loop --session-prefix ... --format json` | Missing | Clearly label as foreground polling loop, not background daemon | Empty prefix, invalid interval/max-runs, detached-service wording | Copy honesty test; cancellation test |
| Sync local | OS watcher | `sync watch --session-prefix ... --format json` | Missing | Clearly label watcher as foreground process | Empty prefix, watcher unsupported path, detached-service wording | Watch smoke where available; disabled-state test |
| Sync network | Bounded network sync pass | `sync network run --profile --session --format json` | Missing | Publish ready queue entries via profile-backed mTLS and receiver revalidation | Unpaired profile, no ready entries, receiver unavailable | Loopback network sync smoke |
| Sync network | Discovery-gated network run | `sync network discover-run --profile --session --format json` | Missing | Require low-info candidate matching profile `network.receiver_url`; no endpoint auto-selection | Candidate mismatch, duplicate candidate, unpaired target, discovery timeout | Discovery gate tests |
| Sync network | Foreground network loop | `sync network loop --session-prefix ... --format json` | Missing | Foreground loop only; no detached/background claim | Empty prefix, invalid interval/max-runs | Loop/cancel test |
| Status | Compact target status | `status --profile --format json` | Wired but not gate-compliant | Main readiness snapshot; stale/missing evidence never green | Missing profile, artifact problems, pairing issues | Fixture decode tests |
| Evidence | Operator report | `report --profile --format json` | Wired but not gate-compliant | Evidence browser summary and review-required state | Malformed artifacts, unsupported schema, stale session | Fixture decode tests |
| Evidence | Health | `health --profile --format json` | Wired but not gate-compliant | Control-plane health and recovery state | Invalid records, incomplete sessions, artifact problems | Fixture decode tests |
| Verify | File-level verification | `verify --profile [--session]` | Wired but not gate-compliant; text output | Comparator must use durable verify/status/report evidence; root comparison remains unavailable unless implemented | Missing manifests, mismatched session, artifact problems | Go verify tests; app unavailable-root test |
| Drift | Live drift list | `drift list --profile --format json` | Wired but not gate-compliant | Read-only live detector; output is not durable review state | Missing manifest/evidence, live-only ids cannot be ack/resolve | Fixture decode tests |
| Drift | Record/ack/resolve/expire persisted drift | `drift record`, `drift acknowledge`, `drift resolve`, `drift expire` | Wired but not gate-compliant except expire missing | Require durable persisted ids and reason/reviewer gates; ids must be selected from loaded evidence, not free text | Live-only id, missing reason, stale detector evidence, free-text id not loaded from evidence | Negative CLI/app tests |
| Reconcile | Review broad boundaries | `reconcile review --format json` | Missing | Read-only review of persisted plan readiness, live-only record-required inputs, and broad boundaries | Missing profile, stale/malformed drift evidence | CLI contract test; app read-only copy test |
| Reconcile | Plan/apply narrow repair | `reconcile plan`, `reconcile apply --apply --reason`, `reconcile apply --all-persisted-planned`, `reconcile apply --record-live` | Wired but not gate-compliant for selected plan/apply; bulk/live gates missing | Plan first; apply requires explicit operator intent and selected persisted ids; bulk/live convenience gates require separate confirmation and review summary | Empty reason, no selected ids, free-text ids not loaded from evidence, conflict/refusal, bulk/live gate not explicitly confirmed | Existing Go tests; app disabled-state tests |
| Prune | Dry-run physical prune review | `prune --profile --dry-run --format json` | Missing | Review physical prune candidates/refusals without deleting files | Missing profile, no soft-delete evidence, stale approval | Go prune tests; app review fixture |
| Prune | Review/approvals/approve/supersede | `prune review/approvals/approve/supersede --format json` | Wired but not gate-compliant | Soft-delete and approval artifacts are inspectable before physical pruning; ids must come from loaded evidence | Empty reason/reviewer, stale approval, missing soft-delete id, free-text id not loaded from evidence | Existing Go tests; app disabled-state tests |
| Prune | Apply approved physical prune | `prune --profile --apply --approval <id> --format json` | Missing | Must be a final explicit destructive action with durable approval evidence and receipt review | Missing approval id, stale/expired/superseded approval, empty confirmation, policy disallows physical prune | Negative CLI/app tests; manual release gate |

## Structured Output Contract

The app must prefer JSON or app-specific structured events for non-trivial
state. Human text may remain visible in logs, but it cannot be the source of a
green state except where explicitly marked as a compatibility bridge.

Rows that currently rely on text/stderr or incomplete parsing require new JSON,
event, or durable-artifact readers before they can drive green state:

- `profile init`, `profile lint`, and `profile set-target`
- local `push --dry-run` and `push --session`
- `pair`
- `serve` readiness
- `daemon install` and `daemon stop`
- `verify` command output

Compatibility parsing may expose "process printed X" as a neutral log fact. It
must not become trust, readiness, or successful verification without durable or
structured backing.

Required event classes for later tasks:

- `process_started`: role, command, pid, launch time, command preview
- `process_readiness`: role, command, readiness kind, listen URL/address, source
- `process_stopped`: role, command, exit code, cancellation flag, timestamps
- `profile_state`: profile path, profile id, target id, source roots, target root
- `pairing_state`: candidate id, verification method, receipt id, pin status
- `sync_queue_state`: queue state, counts, ready/in-flight/backoff/failed/done
- `transfer_state`: session id, stage, files/bytes/chunks when wired, retry/resume fields
- `verification_state`: session, manifests, files checked, warnings/errors, artifact problems
- `evidence_problem`: source, path, schema/version, parse error, stale/missing marker

Until these are wired, the app must suppress percent, throughput, ETA, live
root comparison, and broad "all clear" states.

## Process Supervision Contract

The current app has one `processController` and terminates the previous process
when launching another task. That is incompatible with the app-first LAN flow.

Required model:

- One supervised process slot per role-scoped long-running task type.
- `serve`, `discover advertise`, `dashboard`, `daemon run`, `sync loop`,
  `sync watch`, and `sync network loop` are independently stoppable foreground
  tasks.
- Durable daemon status/logs are displayed as evidence, not active liveness.
- A stop action must name the task it stops and show whether it sent process
  termination or wrote a CLI stop/restart intent.
- Starting a bounded source run may not terminate target `serve` or dashboard
  unless the operator explicitly stops it.

## Mutating Action Gates

The app must never generate hidden defaults for fields that express operator
intent.

Required typed inputs:

- `session_id`: required for publish, network push, bounded sync runs, and
  session-scoped verification when the command requires it. The current app's
  empty-session fallback to `session-001` is not gate-compliant for mutating
  commands.
- `session_prefix`: required for foreground loops/watchers.
- `reason`: required for drift/reconcile/prune/queue cancellation/failure and
  daemon restart/stop intents.
- `reviewer`: required where the CLI requires or semantically needs a reviewer.
- selected persisted ids: drift ids, prune approval ids, soft-delete ids, and
  queue entry ids must come from loaded durable evidence. Free-text id entry is
  not gate-compliant except as a debug-only command preview mode.

## Unsupported Or Conditional Capabilities

| Capability | Current status | Required UI behavior |
| --- | --- | --- |
| Merkle/root comparison | Not wired as a CLI/control-plane artifact in this contract | Show `unavailable` unless T-008 implements tested manifest-root evidence |
| Detached OS-managed daemon | CLI explicitly says current daemon does not install/manage detached background processes | Use foreground language only |
| Automatic endpoint trust/selection | Discovery is low-information and untrusted | Require pairing/profile pins before transfer |
| Broad automatic repair | Out of scope for this feature | Keep reconcile/prune actions selected, review-first, and explicit |
| Bidirectional sync/conflict resolution | Out of scope | Do not import two-way sync language |

## Acceptance Gate For T-001

T-001 is complete only when:

- this contract exists under feature artifacts
- the feature tracker task is in progress or done with evidence
- the contract names current app coverage and missing command coverage
- later tasks can point to this contract for implementation scope
- a subagent review checks for overclaiming, missing commands, unsafe gates, and
  task ordering risks
- `feature-tracker validate-tracker` passes
- help-surface smokes have been run for profile, scan, deleted, recover, prune,
  reconcile, discover, pair, sync, daemon, and network push
- current `SuperMoverTaskKind` coverage has been checked against the matrix
- every `wired` row is marked either gate-compliant or `wired but not
  gate-compliant`
- `git diff --check` passes for this artifact and tracker metadata
