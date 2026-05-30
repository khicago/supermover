# Feature Proposal: f-22ynwqndn

## Why

Explicit-address discovery exists, but v1 still needs true LAN browsing so an
operator can find available peers without typing addresses manually.

## Goal

Add LAN advertisement and browsing discovery UX with bounded low-info peer
discovery, ambiguity handling, and acceptance evidence without bundling pairing
trust, daemon lifecycle, or continuous sync.

## Implemented / Planned / Missing

| Status | Detail |
| --- | --- |
| Implemented | Explicit-address `discover`, `discover browse`, `discover advertise --profile <path>`, profile-backed mTLS `serve`, and non-dry-run network push. |
| Planned | Managed long-running advertisement ownership in a future daemon/sync slice, if needed. |
| Missing | No missing LAN browse/advertise behavior remains in this feature after the bounded UDP slice; discovery is still not trust. |

## Scope

- In scope: advertise/browse protocol surface, CLI UX, candidate record format, ambiguity/freshness handling, and discovery acceptance fixtures.
- Out of scope: treating discovery as trust, daemon lifecycle implementation, ongoing sync, and sensitive metadata disclosure before pairing.

## Acceptance Criteria

- Operators can browse for LAN candidates within a bounded timeout.
- Discovery output is low-information and does not expose sensitive file metadata.
- Duplicate or ambiguous peers are reported clearly instead of auto-selected.
- Docs distinguish explicit-address discover from true LAN browsing.

## Transfer Checks

- Discovery is not trust; pairing/authentication remains separate.
- Profile SSOT remains the source for advertised identity/config.
- LAN browsing must not add runtime overrides for transport auth.

## Impact

- Code paths: discover CLI, receiver advertisement path, profile/network config.
- Tests: unit tests for candidate parsing and ambiguity, plus LAN-like smoke fixtures where feasible.
- Rollout notes: daemon feature owns managed long-running lifecycle; this feature owns browse semantics.
