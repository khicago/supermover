# Security Policy

Supermover's safe operator network path is profile-backed pinned TLS 1.3 mTLS
for `serve` and non-dry-run `push --network`. Lower-level receiver and protocol
handlers are internal foundations and must not be exposed directly as public,
unauthenticated network services.

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
- Network push assumes a paired profile with `network.receiver_url`,
  `network.local_tls_identity`, and pinned peer identity evidence. The profile
  remains the SSOT for operator network material.
- Existing target files are not overwritten unless content already matches.
- Source-side deletions are recorded as soft-delete evidence. Physical prune is
  wired only through reviewed `prune --apply --approval <id>` over existing
  durable approval artifacts; approval-authoring UX remains planned.
- Network receiver packages are protocol foundations. Safe exposure is the
  profile-backed pinned mTLS `serve` path, not unauthenticated public handlers.

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
