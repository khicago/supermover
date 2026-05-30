# Release Checklist

This checklist defines the minimum bar before Supermover can publish an
official tagged release or tell users to download release assets.

## Release State

Until every required gate below has passing evidence:

- do not publish a "stable" release claim
- do not tell users to download an app bundle as the recommended install path
- do not treat unsigned, ad-hoc signed, dirty, or unstapled app bundles as
  distribution readiness
- do not use same-machine packaged-app evidence as real two-machine acceptance

## Required Gates

### Source And CLI

- `go mod tidy -diff`
- `go test -count=1 ./...`
- `go test -race -count=1 ./...`
- `go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...`
- `go vet ./...`
- `staticcheck ./...`
- `golangci-lint run ./...`
- `git diff --check`
- `go run ./cmd/supermover help`
- `go run ./cmd/supermover version`

### macOS App

- `swift test --package-path macos`
- `macos/script/build-app.sh`
- `macos/script/audit-app.sh macos/dist/SuperMover.app`
- Developer ID signing evidence when distribution readiness is claimed
- notarization evidence from `macos/script/notarize-app.sh`
- stapler validation
- Gatekeeper assessment
- complete packaged provenance with `git_dirty=false`

### Acceptance

- packaged loopback acceptance evidence from
  `macos/script/acceptance-loopback.sh`
- real two-machine installed-app acceptance evidence from
  `macos/script/acceptance-two-machine.sh`
- operator evidence for Local Network prompt, firewall/listen readiness, and
  pairing confirmation when required by the two-machine lane
- final evaluation artifacts showing the source and target were distinct
  machines and that release evidence was current for both roles

### Documentation

- `README.md` current status matches release reality
- `CHANGELOG.md` has the release entry
- `docs/release-audit.md` current checkpoint is updated
- `docs/v1-scope.md` non-goals remain explicit
- `SECURITY.md` supported versions include the new release
- release notes call out limitations and upgrade guidance

## Tagging And Publishing

Only after the gates pass:

1. choose the version
2. update `CHANGELOG.md`
3. update supported versions in `SECURITY.md`
4. create a signed tag
5. publish release notes using the release template
6. attach only artifacts that match the audited, signed, notarized, and
   accepted build evidence
7. keep provenance, audit, notarization, and acceptance artifacts available for
   review

## Rollback

If release evidence is later found stale or incorrect:

- mark the release as withdrawn or pre-release
- remove or replace affected assets
- publish a clear advisory
- keep the failed evidence for audit
