# Verification Evidence

## Automated Checks

- Command: `go test -count=1 ./...`
- Result: pass for the current uncommitted `T-003` slice using repo-local tmp
  dirs; includes the daemon foreign-state test assertion narrowing needed to
  keep full-suite evidence stable
- Command: `go run ./cmd/supermover help`
- Result: pass for the current uncommitted `T-003` slice using repo-local tmp
  dirs
- Command: `cd macos && swift build`
- Result: pass for committed `T-003` and current post-commit tracker truth alignment
- Command: `cd macos && ./script/build-app.sh`
- Result: pass for committed `T-003`; `macos/dist/SuperMover.app` rebuilt from the current tree
- Command: `git diff --check`
- Result: pass for committed `T-003`; rerun required after current tracker-truth edits before the next commit
- Command: `go test -count=1 ./...`
- Result: pass for the current uncommitted `T-004` read-only prune lifecycle slice using repo-local tmp dirs
- Command: `go run ./cmd/supermover help`
- Result: pass for the current uncommitted `T-004` read-only prune lifecycle slice
- Command: `cd macos && swift build`
- Result: pass for the current uncommitted `T-004` read-only prune lifecycle slice
- Command: `cd macos && ./script/build-app.sh`
- Result: pass for the current uncommitted `T-004` read-only prune lifecycle slice; `macos/dist/SuperMover.app` rebuilt from the current tree
- Command: `git diff --check`
- Result: pass for the current uncommitted `T-004` read-only prune lifecycle slice

## Current Slices

- Committed slice `ad4fea6`: native macOS operator shell, bundled CLI
  packaging, honest task naming, and tracked feature surfaces.
- Committed slice `e7d1736`: richer structured `status` / `report` /
  `prune review` cards.
- Committed slice `df2b100`: structured `health` and `drift list` cards.
- Committed slice `72b2441`: structured `daemon status` and `daemon logs`
  cards.
- Committed slice `832954b`: structured persisted drift follow-through cards
  for `drift acknowledge`, `drift resolve`, `reconcile plan`, and narrow
  `reconcile apply`, plus CLI-truth alignment so `reconcile plan` keeps
  optional ids and `reconcile apply` keeps repeatable ids.
- Current uncommitted slice: richer read-only prune lifecycle surfaces through
  full `prune review` and `prune approvals` inventory cards before approval
  artifact mutations, plus explicit `prune approve` and `prune supersede`
  artifact-mutation surfaces that still stop short of `prune --apply --approval`.

## Manual Checks

- Step: Launch the app and confirm task names use `Publish`, `Review`,
  `Network Push`, `Serve`, `Daemon`, and `Dashboard`, not `Sync` or `LAN`.
- Outcome: partial; labels and task names are implemented in code, but no GUI
  screenshot capture is recorded yet.
- Step: Confirm the app can open a profile, start a foreground `dashboard`
  task, and surface the emitted loopback URL.
- Outcome: partial; runtime parsing and button flow are implemented, but an
  end-to-end interactive smoke has not yet been captured in evidence.
- Step: Confirm the app renders structured status/report/health/drift/prune and
  daemon lifecycle evidence from JSON payloads instead of only raw stdout.
- Outcome: pass in code; no screenshot capture is recorded yet.
- Step: Confirm persisted drift follow-through surfaces remain narrow:
  acknowledge/resolve review metadata and persisted-drift reconcile only.
- Outcome: pass in code and fresh automated checks; the app still describes this
  as persisted drift review and narrow reconcile rather than broad repair
- Step: Confirm the app does not silently narrow current CLI reconcile inputs.
- Outcome: pass in code; `reconcile plan` now allows zero or more ids, while
  `reconcile apply` now accepts one or more ids, matching current CLI truth
- Step: Confirm the daemon foreign-state suite failure was a test-assertion
  precision issue rather than a product semantic regression.
- Outcome: pass; the status-line assertion now checks the `daemon_status` line
  directly instead of substring-matching `pid=99` across unrelated fields
- Step: Confirm the read-only prune lifecycle slice keeps `prune review` as
  full release-review truth rather than reducing it to approval inventory.
- Outcome: pass in code; current uncommitted app wiring now includes prune
  review authorization contract, candidates, refusals, approvals, receipts, and
  separate `prune approvals` inventory
- Step: Confirm the read-only prune lifecycle slice still does not expose
  approval mutation or target deletion actions.
- Outcome: pass in code; current uncommitted task surface adds `prune approvals`
  inventory and richer `prune review` only, leaving `prune approve`,
  `prune supersede`, and `prune --apply --approval <id>` outside this slice
- Step: Confirm prune approval mutation surfaces require dedicated approval and
  soft-delete selectors rather than overloading drift or session inputs.
- Outcome: pass in code; current uncommitted slice adds explicit approval id,
  soft-delete ids, reviewer, reason, and optional expiry fields for
  `prune approve` / `prune supersede`
- Step: Confirm prune approval mutation surfaces still do not imply target
  deletion or receipt writing.
- Outcome: pass in code; structured mutation results surface artifact-writing
  truth while leaving `prune --apply --approval <id>` outside this slice
- Step: Confirm prune approval mutation surfaces require dedicated approval and
  soft-delete selectors and do not overload drift or session semantics.
- Outcome: pass in code; current uncommitted slice adds dedicated approval id,
  soft-delete ids, reviewer, reason, and optional expiry inputs

## Residual Risks

- `serve` and `dashboard` still rely on stderr readiness lines. That remains an
  adapter layer, not the long-term app API.
- The app is still a single-child foreground runner. It should not be described
  as an OS-managed daemon supervisor.
- Persisted daemon status/logs are durable evidence views, not live liveness.
- The structured reconcile card now exposes more evidence, but it still
  summarizes rather than fully materializes every receipt field; raw stdout
  remains the full source of truth in-app.
- Approval-artifact mutation surfaces are still intentionally separate future
  work from target deletion; `prune --apply --approval <id>` remains outside the
  current macOS shell scope.
