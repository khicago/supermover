# Contributing

Supermover is safety-first. Changes that affect file publication, recovery,
identity, or deletion semantics need tests and clear audit evidence.

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

Run these before submitting changes:

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

## Change Discipline

- Keep the profile JSON as the configuration SSOT.
- Preserve `.supermover` artifacts as audit evidence, not cache.
- Do not introduce overwrite, prune, or rollback behavior without explicit
  tests for interrupted runs and divergent targets.
- Do not expose network receiver code as a trusted listener without an
  authenticated encrypted transport and identity binding.
- Keep commits scoped by rollback boundary and include validation evidence in
  the commit message.
