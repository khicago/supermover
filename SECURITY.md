# Security Policy

Supermover is currently a local migration tool with a planned secure network
transport. Do not expose the receiver protocol directly on a network until an
authenticated encrypted transport is implemented and documented.

## Supported Versions

Security fixes target the `main` branch until the first tagged release. The
project currently requires Go 1.25 as declared in `go.mod`.

## Reporting A Vulnerability

Open a private security advisory if the hosting platform supports it. If that
is unavailable, contact the maintainers out of band before publishing exploit
details.

Please include:

- affected commit or version
- operating system and filesystem type
- profile and command shape, with secrets and personal paths redacted
- whether target files, `.supermover` artifacts, or source files were modified
- a minimal reproduction when possible

## Current Security Boundaries

- Local push assumes a trusted local or mounted target path.
- Existing target files are not overwritten unless content already matches.
- Source-side deletions are recorded as soft-delete evidence and are not
  physically pruned by current commands.
- Network receiver packages are protocol foundations, not a safe public
  listener without a future authenticated encrypted transport.

## Security Checks

Run the baseline gates before reporting a suspected regression:

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
staticcheck ./...
golangci-lint run ./...
git diff --check
```

`gosec` is tracked as a manual review aid for now because the current findings
need project-specific triage for controlled filesystem paths and atomic file
replacement patterns.
