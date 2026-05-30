# T-006 App Verification

This artifact records checks for the native discovery and pairing workflow.

## Implementation Evidence

- `AppStore` adds role-gated task kinds for Discover Address, Discover Browse,
  Discover Advertise, and Pair.
- Source role can browse LAN hints, check one explicit address hint, and run
  verification-code pairing through the existing CLI.
- Target role can advertise a bounded low-information hint and supervise
  target `serve`.
- Observer role remains read-only for pairing and cannot browse, advertise,
  serve, or pair devices.
- Discovery JSON stdout is decoded into app snapshots for explicit hints,
  browse results, and advertise results.
- The UI renders discovery candidates as untrusted evidence and surfaces
  duplicate/ambiguous candidate classification instead of auto-selecting or
  trusting an endpoint.
- `discover advertise` uses a separate optional advertise listen input. If that
  input is empty, the app omits `--listen`, preserving CLI default behavior and
  profile receiver-url inference.
- Task context now includes task-specific discovery/pairing inputs so stale
  discovery output or old pair summaries are not promoted for changed inputs.
- `macos/README.md`, `README.md`, `docs/plan.md`, and `docs/v1-scope.md` now
  state that native discovery/pairing orchestration is wired while sync
  controls, root comparison, packaging readiness, and final acceptance remain
  planned.

## Checks

- `swift build --package-path macos`
  - Result: pass.
- `swift test --package-path macos`
  - Result: pass; 5 XCTest tests cover command construction, advertise listen
    omission, role gates, task-context invalidation, and browse JSON decoding.
- `codexL exec --sandbox read-only --ephemeral ...`
  - Result: found two correctness issues:
    - `discover advertise` incorrectly reused the shared serve listen input and
      always passed `--listen`.
    - Discovery/pairing inputs were not included in stale-context matching.
  - Both were fixed before gate.
- `go test -p=1 -count=1 ./...`
  - Result: pass.
- `go vet ./...`
  - Result: pass.
- `go mod tidy -diff`
  - Result: pass.
- `git diff --check`
  - Result: pass.
- `go run ./cmd/supermover help`
  - Result: pass.
- `go run ./cmd/supermover version`
  - Result: pass.
- `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-006`
  - Result: pass in `artifacts/gate-T-006-r9-0001.log`.

## Remaining Boundaries

- No sync queue/run/loop/watch/network app controls were added.
- No Merkle/root-comparison evidence was added.
- No evidence-vault safe actions were added.
- No packaging, signing, notarization, Local Network permission, firewall, or
  bundled CLI provenance hardening was added.
