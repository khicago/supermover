# Feature Proposal: f-23dnwnzft

## Why
- The current Pairing surface is command-driven: the target exposes a pairing
  code, the source requests bootstrap material, and the source writes local
  pairing pins. That is not the same as a two-sided operator ceremony.
- Operators expect pairing to behave like a trust handshake: one side sends a
  request, the other side sees and approves or rejects it, and both sides get
  an unmistakable result.
- Discovery must stay a low-information hint. A found device must never become
  trusted until a target-side operator explicitly confirms the request.

## Goal
- Implement an app-first pairing ceremony where discovery remains an untrusted hint, source operators explicitly send a pairing request, target operators receive and approve or reject that request, both sides surface status notifications, and durable pairing receipts/profile pins are written only after explicit human confirmation.

## Principle Layer
- What: Convert pairing from one-sided bootstrap fetch into an explicit
  request/approval ceremony.
- Why: Trust is a human-confirmed relationship between two operators, not a
  side effect of LAN discovery or possession of a copied address.
- Intended generalization: Any future LAN trust operation should separate
  discovery, request, local operator confirmation, durable evidence, and
  transfer readiness.
- Failure boundary: no auto-accept, no target identity disclosure without the
  current verification code, no durable profile pins unless approval is fresh,
  single-use, and bound to the current request.
- Behavior examples:
  - Source browses LAN candidates, selects one, and sends a pairing request.
  - Target serve records a pending request and reports it through local app
    state without treating it as trusted.
  - Target operator approves or rejects; source sees success, rejection, or
    timeout.
  - Successful pairing writes source pins, exports a receipt, and exposes
    target-side adoption/receipt evidence.
- Evidence refs:
  - `internal/pairserve/server.go`
  - `internal/cli/pair_commands.go`
  - `internal/cli/serve_commands.go`
  - `macos/SuperMoverApp/ContentView.swift`
  - `macos/SuperMoverApp/AppStore.swift`

## Scope
- In scope:
  - request/approve/reject pairing protocol primitives
  - target serve readiness state for pending pairing requests
  - source-side pair command waiting for approval before writing durable pins
  - macOS app prompts/status for request sent, request received, approval,
    rejection, timeout, and success
  - focused CLI and app tests that pin the ceremony boundary
- Out of scope:
  - fully background daemon pairing outside the supervised app process
  - trusting discovery without verification code confirmation
  - OS-level push notifications when the app is not running
  - replacing later mTLS receiver transfer trust validation

## Acceptance Criteria
- Source can still browse LAN hints without trust.
- Source can send a pairing request to target serve with profile/device context
  and the current verification code.
- Target serve exposes a pending request only to local app state and does not
  return bootstrap material until the target operator approves.
- Target can reject a request and source surfaces rejection without writing
  profile pins or receipts.
- Approved pairing writes durable source profile pins and receipt evidence.
- App surfaces clear in-window notifications/events on both source and target
  paths.
- Existing direct pairing safety remains fail-closed for missing profile,
  missing verification code, wrong code, stale approval, and role gates.

## Transfer Checks
- `go test -count=1 ./internal/cli ./internal/pairserve ./internal/pairing`
- `swift test --package-path macos --filter Pairing`
- `swift test --package-path macos --filter AppStoreTests`
- `git diff --check`

## Impact
- Code paths:
  - CLI `serve`, `pair`, profile pairing receipt adoption
  - pairserve HTTP endpoints and ready-file evidence
  - macOS AppStore supervised process parsing and Pairing UI
- Tests:
  - Go protocol and CLI command tests for approve/reject/timeout
  - Swift app-state tests for pending request and notification surfaces
- Rollout notes:
  - Existing evidence should remain readable.
  - Discovery remains low-information and untrusted.
  - Pairing remains profile-backed and auditable.
