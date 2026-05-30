# apple-platform

## Track Contract

- track id: `apple-platform`
- parent pass: `pass-001`
- parent charter: `charter.md`

## Track Question

macOS app UX and permission constraints

## Required Source Types
- Apple official documentation or support security guidance.
- Repo-local macOS app implementation evidence where platform constraints
  affect current behavior.

## Preferred Sources
- Apple Platform Security and Apple Developer documentation.
- Local Swift app code for current process, permission, and packaging behavior.

## Disallowed Sources
- Unsourced app-store checklist blogs as primary evidence.
- Generic design inspiration that does not affect a Supermover acceptance gate.

## Source Id Range

`ap01`-`ap09`

## Owned Output Files
- tracks/apple-platform.md

## Minimum Evidence

At least file-access, local-network, and signing/notarization evidence before
claiming two-machine install readiness.

## Lead Policy

Open a lead only if a platform requirement affects a gate not already covered
by the plan.

## Drift Check

If a task claims source/target installation, verify it names file permissions,
Local Network/firewall/listen behavior, binary provenance, and failure UX.

## Merge Notes
- Result: macOS deployment/permission handling must move earlier than final
  packaging.
