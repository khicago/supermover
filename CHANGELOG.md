# Changelog

Supermover has not published a tagged release yet.

This file follows the spirit of [Keep a Changelog](https://keepachangelog.com/)
and [Semantic Versioning](https://semver.org/) once tagged releases begin.

## Unreleased

### Added

- CLI-backed one-way local migration with durable `.supermover` target evidence.
- Profile-backed pairing, sparse LAN discovery hints, `serve`, and bounded
  profile-backed network transfer surfaces.
- Local incremental queue/run/loop/watch surfaces and bounded profile-backed
  network queue surfaces.
- Native macOS operator workbench for profile setup, wired command
  orchestration, foreground supervision, evidence review, UI localization, and
  local packaged-app provenance.
- Release-engineering scripts for local app packaging, audit, notarization
  evidence collection, and acceptance harnesses.
- Community and release-hygiene files for contribution, support, security,
  issue triage, pull requests, and release readiness.

### Changed

- README now presents the project as pre-release, separates source-build usage
  from official release distribution, and keeps the CLI as the authoritative
  execution surface.

### Security

- Current network execution is profile-backed TLS 1.3 mTLS; discovery remains
  untrusted address-hint material.
- Unsigned, ad-hoc signed, dirty, or unstapled local app bundles are explicitly
  treated as review-only evidence, not release-ready distribution builds.

### Not Yet Released

- No official tagged binary release exists yet.
- Developer ID signed, notarized, stapled, Gatekeeper-ready app distribution
  remains a release gate, not a current user-install claim.
