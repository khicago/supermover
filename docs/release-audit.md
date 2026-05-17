# Release Audit

This document is the tracked audit surface for the current implementation
checkpoint. Detailed Bagakit commit-audit notes may exist under `.bagakit/`, but
that directory is intentionally ignored by Git.

## Implementation Status

Current completed slice:

- Go CLI commands: `profile`, `scan`, `push`, `verify`, `deleted list`,
  `health`, `report`, and `recover`.
- One-way local push from one source root to a trusted local or mounted target.
- Hidden files are included by default.
- Regular files are copied into session staging with source stability checks,
  SHA-256 manifests, no-replace final publish, and post-run verification.
- Publish is conservative: existing target files or symlinks are not overwritten
  unless they are already content-identical to the intended result.
- Profile JSON remains the SSOT; unsupported profile policies fail before push.
- Target `.supermover` records profile snapshots, transaction state, receipts,
  manifests, warnings, soft deletes, and agent influence records. Health is a
  read-only report over those artifacts.
- This feature adds `report` as a read-only operator aggregation command.
  `report` aggregates warnings, profile suggestions, soft deletes,
  health/recovery issues, artifact problems, and published-manifest
  verification state at report time.
- `recover` can replay safely staged local sessions, mark explicitly abandoned
  incomplete sessions as `rolled_back`, and mark non-automatable sessions as
  `needs_repair`.
- Receiver protocol library supports resumable chunk upload, integrity checks,
  idempotent begin/commit, and conservative publish semantics.

Known planned surface:

- End-to-end network CLI wiring (`serve`, `pair`, source client).
- Authenticated encrypted transport integration around the receiver protocol.
- Traffic-shaping implementation beyond the current profile policy model.
- Reviewed physical prune command.
- Broader automatic repair/reconcile command.
- Compact `status` command.
- Drift review command.

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

Current checkpoint: local migration publication and receiver store recovery now
cover process-level locks, no-replace file and symlink publish, shared symlink
target safety, published-artifact drift detection, and non-file verify checks.
Commit audit note for `b233f79`: lock-directory setup is intentionally performed
before the process file lock is acquired, with symlink-guarded directory
creation; the file lock protects subsequent session and target publication
mutations.

## Current Gate Results

Last full verification for this checkpoint:

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
go run ./cmd/supermover recover --help
GOOS=windows GOARCH=amd64 go test -c <each package from go list ./...>
GOOS=aix GOARCH=ppc64 go test -c ./internal/filelock
GOOS=solaris GOARCH=amd64 go test -c ./internal/filelock
```

Coverage is package-level and intentionally uneven: the CLI package exercises
flows through command integration tests, while lower-level packages carry the
bulk of behavior coverage. `cmd/supermover` has no direct tests because it is a
thin entrypoint over `internal/cli`.

## Safety Notes

- The current migration-ready path assumes an empty trusted target or an
  idempotent rerun where existing target content already matches the source.
- Divergent existing target files are refused rather than overwritten.
- Soft-delete records are review markers only; no target file is physically
  deleted by current commands.
- `health` is read-only. It reports incomplete transactions and damaged
  published artifacts. `recover` is the explicit mutating command for the safe
  local subset.
- `report` is also read-only. Its evidence source is the profile SSOT plus
  target `.supermover` control-plane artifacts, and its presence does not imply
  network transfer, encrypted transport, daemon status, or physical prune
  support. `report` returns non-zero when review is required, even when it
  successfully emits a parseable text or JSON report.
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
  artifact problems, and missing manifests. It verifies regular file size,
  `sha256:` digest, permission mode, and modification time, and verifies
  directory/symlink presence and symlink targets.
