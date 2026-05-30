# Verification Evidence

## Automated Checks

- Command: `go test -count=1 ./internal/cli ./internal/network ./internal/config`
- Result: superseded by focused LAN gate below; `./internal/network` and
  `./internal/config` were not directly touched by this slice.
- Command: `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -count=1 ./internal/cli ./internal/discovery`
- Result: pass. Covered explicit address compatibility, LAN browse loopback
  datagrams, JSON ambiguous-candidate reporting, low-info advertise datagram
  emission, and usage errors.
- Command: `go test -count=1 ./...`
- Result: pass.
- Command: `go vet ./...`
- Result: pass.
- Command: `go mod tidy -diff`
- Result: pass.
- Command: `git diff --check`
- Result: pass.
- Command: `go test -race -count=1 ./internal/cli ./internal/discovery`
- Result: pass.
- Command: `go run ./cmd/supermover discover --help`
- Result: pass; help lists explicit hints, `discover browse`, and
  `discover advertise`, and states discovery is not trust.
- Command: `go run ./cmd/supermover discover browse --help`
- Result: pass; help lists bounded UDP listen, timeout, strict mode, and
  text/json output.
- Command: `go run ./cmd/supermover discover advertise --help`
- Result: pass; help requires profile SSOT and lists low-information
  destination/listen/interval/duration flags.
- Command: `go run ./cmd/supermover help`
- Result: pass; overview describes discover as low-information explicit and
  LAN browse hints with no trust.
- Command: `feature-tracker run-task-gate --feature f-22ynwqndn --task T-001`
- Result: pass; tracker gate used its configured general gate command set.

## Manual Checks

- Step: Confirm discovery output does not expose sensitive file metadata.
- Outcome: focused tests assert text/JSON browse and advertise output omit
  profile IDs, target IDs, device keys, pairing receipt IDs, file counts, and
  hostnames. Sparse datagram decode accepts only service, protocol, ephemeral
  nonce, and capability flags.
- Step: Confirm docs separate LAN browsing from explicit-address discover and from trust.
- Outcome: README, docs/v1-scope.md, docs/transport.md, docs/runbook.md,
  docs/user-migration-guide.md, docs/plan.md, docs/release-audit.md,
  docs/troubleshooting.md, and authority summaries now describe
  `discover --address`, `discover browse`, and `discover advertise` as
  unauthenticated hint surfaces, not trust or transfer.

## Residual Risks

- LAN discovery can vary by platform and network environment; implementation
  should keep deterministic candidate logic separate from environment smoke tests.
- The current browse/advertise transport is sparse UDP datagrams, not
  mDNS/DNS-SD, and real LAN broadcast behavior still depends on local network
  policy. Loopback tests exercise deterministic CLI/protocol behavior.
