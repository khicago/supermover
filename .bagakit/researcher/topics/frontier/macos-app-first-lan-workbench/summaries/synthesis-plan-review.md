# Synthesis: macOS app-first LAN workbench plan review

## What This Synthesizes

Plan review for implementing Supermover's native macOS app-first LAN workbench after local code inspection, subagent review, and platform/product research.

## Claim Refs
- c001
- c002
- c003
- c004
- c005
- c006

## Insight Refs
- i001

## Findings
- The plan direction is correct, but profile bootstrap, command coverage, structured events, process supervision, artifact readers, and macOS deployment gates must precede broad UI implementation.
- The current macOS app does not yet cover discover, pair, sync queue/run/loop/watch/network, daemon run/install/restart/stop, or app-owned profile setup.
- Merkle/root comparison must be conditional on a real CLI/control-plane artifact; otherwise the app should show unavailable.

## Open Risks
- A polished app shell could overstate readiness if unsupported progress, liveness, root comparison, or LAN trust states are shown as real.
- Two-machine installs can fail for file permissions, Local Network, firewall/listen, binary provenance, or missing bundled CLI unless those are explicit acceptance gates.

## Next Action

Execute T-001: write the command coverage matrix and app capability contract.
