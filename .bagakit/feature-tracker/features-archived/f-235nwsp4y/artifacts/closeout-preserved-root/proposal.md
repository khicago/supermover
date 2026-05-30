# Feature Proposal: f-235nwsp4y

## Why

SuperMover already has enough wired CLI truth to justify a native macOS
operator surface: local publish and verify flows, bounded network push,
foreground `serve`, loopback-only `dashboard`, durable review artifacts, drift
review, prune review, and daemon lifecycle evidence. The missing layer is a
desktop control surface that reduces operator friction without forking the
execution engine.

The app still has to be named and scoped honestly. The current repo does not
implement ongoing incremental sync, LAN browsing, detached daemon lifecycle, or
broad automatic repair. So the app must remain an operator shell over current
CLI and artifact truth, not a sync client.

## Goal

Add a native macOS `SuperMover` desktop operator shell that launches and
reviews current CLI behavior truthfully, packages into `SuperMover.app`, and
exposes the most valuable existing review and lifecycle surfaces without
renaming bounded publish, bounded network transfer, or foreground daemon
behavior into broader product claims.

## Implemented / Planned / Missing

| Status | Detail |
| --- | --- |
| Implemented | Native SwiftUI app shell, authoritative CLI resolution in dev and packaged modes, bundled CLI packaging into `SuperMover.app`, honest task naming, structured `status` / `report` / `prune review` cards, structured `health` / `drift list` cards, structured `daemon status` / `daemon logs` cards, structured persisted drift follow-through surfaces for `drift acknowledge`, `drift resolve`, `reconcile plan`, and narrow `reconcile apply`, and current in-app task launch for wired local/network/foreground flows exist. |
| Planned | Broader app ergonomics can build on the shell next. Prune approval lifecycle is now being wired honestly in two slices: read-only `prune review` + `prune approvals` truth, then approval-artifact mutations such as `prune approve` and `prune supersede`. |
| Missing | No LAN browsing, no ongoing incremental sync, no OS-managed daemon install, no detached background service management, and no broad automatic repair UX should be claimed through this feature. |

## Principle Layer

- What:
  A native `SuperMover` macOS app that wraps the existing CLI with a foreground
  operator UI for profile selection, local publish/review, bounded network
  push, foreground `serve`, loopback-only `dashboard`, and structured evidence
  views backed by current JSON or durable control-plane artifacts.
- Why:
  The repo already has executable truth and durable artifacts. The missing
  layer is a desktop operator surface that makes those truths usable without
  asking operators to remember command shapes or manage multiple terminals.
- Intended generalization:
  App actions should map to stable CLI or artifact-backed behavior so the shell
  can deepen from log streaming into richer structured cards without redefining
  product semantics.
- Failure boundary:
  The app must not invent sync semantics, background daemon claims, trust
  decisions, or runtime policy overrides outside the profile SSOT.
- Behavior examples:
  - Run local `push --dry-run`, `push`, `verify`, `report`, and `status`
    without opening Terminal.
  - Launch foreground `serve` or `dashboard`, stream stdout/stderr, and expose
    emitted readiness lines without pretending the task is detached.
  - Review durable drift, prune, health, daemon, and reconcile evidence as
    structured cards instead of raw stdout where current JSON truth exists.
- Evidence refs:
  - `README.md`
  - `macos/README.md`
  - `internal/cli/cli.go`
  - `internal/operatorui/server.go`
  - `internal/driftreview`
  - `internal/networkrun`

## Scope

- In scope:
  - Native macOS app shell under `macos/`
  - Profile picker and task launcher for already wired CLI flows
  - Output and log display for short and long-running tasks
  - Packaging path that can produce `SuperMover.app`
  - Structured review and lifecycle cards for currently wired JSON surfaces
  - Honest product copy that preserves current CLI and code boundaries
- Out of scope:
  - Ongoing incremental sync UX or claims
  - LAN browsing UX or claims
  - Background daemon or service-manager install
  - Replacing core execution with a second native engine
  - GUI-owned policy overrides outside the profile file
  - Broad automatic repair or autonomous reconcile claims

## Acceptance Criteria

- The repo can build a native macOS app shell and package `SuperMover.app`.
- The app can launch at least one profile-backed local flow, one review flow,
  one bounded network flow, and one long-running foreground flow.
- The app can render current structured evidence for status/report/prune,
  health/drift, and daemon lifecycle surfaces without depending only on raw
  stdout.
- The app labels bounded publish, network push, `serve`, `dashboard`,
  persisted drift review, and narrow reconcile truthfully and does not call
  them sync, LAN browsing, detached daemon, or broad automatic repair.
- The app remains a CLI wrapper: command construction and durable truth still
  come from the existing CLI and target-side `.supermover` artifacts.

## Transfer Checks

- Do not rename `push --network` to sync.
- Do not rename foreground `serve` to detached daemon behavior.
- Do not present `discover` as LAN browsing.
- Do not add app-only runtime overrides for target identity, privacy policy, or
  pruning behavior.
- Do not present persisted daemon status as live liveness.
- Do not present narrow persisted-drift reconcile as broad automatic repair.
- Prefer structured JSON CLI outputs where already wired. Treat stderr parsing
  as a temporary readiness adapter, not the long-term contract.

## Impact

- Code paths:
  - `macos/Package.swift`
  - `macos/SuperMoverApp/**`
  - `macos/script/**`
  - `.gitignore`
  - `macos/README.md`
- Tests:
  - `go test -count=1 ./...`
  - `go run ./cmd/supermover help`
  - `cd macos && swift build`
  - `cd macos && ./script/build-app.sh`
  - `git diff --check`
- Rollout notes:
  - This feature complements `f-22znw2utc` foreground daemon lifecycle but does
    not subsume it.
  - This feature must not be used to imply closure of `f-22ynwqndn` LAN
    browsing or `f-232nwu2nw` ongoing incremental sync.
