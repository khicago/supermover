# Release Audit

This document is the tracked audit surface for the current implementation
checkpoint. Detailed Bagakit commit-audit notes may exist under `.bagakit/`, but
that directory is intentionally ignored by Git.

## Implementation Status

Current completed slice:

- Go CLI commands: `profile`, `scan`, `push`, `verify`, `deleted list`,
  `health`, and `recover`.
- One-way local push from one source root to a trusted local or mounted target.
- Hidden files are included by default.
- Regular files are copied into session staging with source stability checks,
  SHA-256 manifests, no-replace final publish, and post-run verification.
- Publish is conservative: existing target files or symlinks are not overwritten
  unless they are already content-identical to the intended result.
- Profile JSON remains the SSOT; unsupported profile policies fail before push.
- Target `.supermover` records profile snapshots, transaction state, receipts,
  manifests, warnings, soft deletes, agent influence records, and health
  diagnostics.
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
- Drift review command.
- Open-source governance files such as `LICENSE`, `SECURITY.md`,
  `CONTRIBUTING.md`, and CI workflow.

## Commit Trail

- `84cc3fe feat(cli): add local migration push workflow`
- `8cf8758 feat(review): add verify and soft-delete review`
- `d1192c8 feat(network): add resumable receiver protocol library`
- `8b2b2fb fix(localpush): close source target alias gaps`
- `9088aff fix(publish): harden publish path and commit evidence`
- `8c668a4 fix(profile): separate target identity from local paths`
- `38a3a96 feat(health): expose read-only recovery diagnostics`
- `e5a8738 fix(compat): preserve legacy profile and manifest repair paths`
- current checkpoint: `fix(safety): harden local migration publish evidence`
  covers no-replace publish, staged local push, profile SSOT fail-fast gates,
  and tracked release audit evidence.

## Current Gate Results

Last full verification for this checkpoint:

```bash
go test ./...
go test -race ./...
go test -cover ./...
go vet ./...
git diff --check
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
- Broader publish reconciliation is still planned. If interruption happens
  during final publish, `recover` can replay staged files or report
  `needs_repair`; operators should preserve the target and review the
  manifest/staging evidence before rerunning.
