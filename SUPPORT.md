# Support

Supermover is currently pre-release software.

## Best-Effort Support

Use GitHub issues for:

- reproducible bugs
- documentation gaps
- release-readiness questions
- safety-boundary questions before running a migration

For security-sensitive reports, do not open a public issue. Follow
[SECURITY.md](SECURITY.md).

## Before Opening An Issue

Please include:

- operating system and filesystem type
- Supermover commit or version output
- command shape, with secrets and personal paths redacted
- whether the run used local push, network push, sync, prune, reconcile, or the
  macOS app
- relevant `.supermover` artifact names and statuses, with private paths and
  secrets redacted
- the expected behavior and the observed behavior

## Current Product Boundary

The CLI is the authoritative execution surface today. The macOS app is a local
operator workbench and packaging substrate; it is not yet an official notarized
end-user release.

Support requests that depend on unsigned local app bundles, ad-hoc signing,
unstapled apps, or incomplete two-machine acceptance evidence may be answered
as release-engineering or validation issues rather than end-user support.
