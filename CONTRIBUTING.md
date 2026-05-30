# Contributing

Supermover is safety-first. Changes that affect file publication, recovery,
identity, or deletion semantics need tests and clear audit evidence.

Please also read:

- [README.md](README.md) for current release status and user-facing boundaries
- [SECURITY.md](SECURITY.md) for vulnerability reporting
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for project conduct
- [docs/release-checklist.md](docs/release-checklist.md) before changing
  release, packaging, signing, notarization, or acceptance behavior

## Development Setup

Install Go 1.25 or newer compatible with the module declaration:

```bash
go version
go test ./...
```

Optional local tools used by maintainers:

```bash
go install honnef.co/go/tools/cmd/staticcheck@2025.1.1
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
```

## Required Checks

Run the relevant subset during development. For non-trivial CLI changes, run:

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
```

For macOS app, packaging, or acceptance changes, also run the relevant subset:

```bash
swift test --package-path macos
macos/script/build-app.sh
macos/script/audit-app.sh macos/dist/SuperMover.app
```

Unsigned, ad-hoc signed, dirty, or unstapled local app audits are useful
review evidence, but they are not distribution readiness.

## Change Discipline

- Keep the profile JSON as the configuration SSOT.
- Preserve `.supermover` artifacts as audit evidence, not cache.
- Do not introduce overwrite, prune, or rollback behavior without explicit
  tests for interrupted runs and divergent targets.
- Do not expose network receiver code as a trusted listener without an
  authenticated encrypted transport and identity binding.
- Keep commits scoped by rollback boundary and include validation evidence in
  the commit message.
- Keep README, release audit, support, and security docs honest about
  implemented behavior versus planned behavior.

## Pull Requests

Good pull requests are small enough to review and roll back. Include:

- the behavior or documentation boundary being changed
- the tests or manual evidence used
- any release, signing, notarization, packaging, or acceptance effect
- explicit non-claims when the change is only a foundation slice

Do not include secrets, private keys, personal paths, or machine-local release
evidence in public commits, issues, or pull requests.
