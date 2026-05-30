# T-006 Discovery And Pairing Contract

This artifact records the native macOS app workflow added for current CLI
discovery and pairing behavior.

## Source Role

- Source role can run:
  - `discover browse --listen <udp-host:port> --timeout <duration> --format json`
  - `discover --address <host:port> --timeout <duration> --format json`
  - `pair --profile <path> --target <host:port|url> --verification-code <code> --method <method> --timeout <duration>`
- LAN browse candidates and explicit address hints are rendered as untrusted
  low-information evidence.
- Candidate cards show address, service, protocol, nonce, capability flags,
  expiration, duplicate count, and ambiguity reasons.
- "Use Address" only fills the target address field. It does not mark the
  candidate trusted and still requires target-side verification code entry.
- Pair remains text-output oriented. The app displays the latest successful
  `pair:` summary and instructs the operator to run Status or Report to confirm
  durable profile pins and target control-plane receipt evidence.

## Target Role

- Target role can run:
  - `discover advertise --profile <path> [--listen <udp-host:port>] --dest <udp-host:port> --interval <duration> --duration <duration> --format json`
  - `serve --profile <path> --listen <tcp-host:port>`
- Discovery advertise has its own optional listen input. When empty, the app
  omits `--listen` so the CLI can use its default or infer
  `network.receiver_url` when `discovery.advertise_receiver_hint=true`.
- Serve retains the supervised target foreground slot and displays current
  log-derived readiness text, including the target verification code line.

## Observer Role

- Observer role remains read-only.
- Observer cannot browse, advertise, serve, or pair devices from the app.
- Observer can inspect Status and Report evidence for pairing/network state.

## Context And Evidence Rules

- Discovery JSON snapshots decode only successful JSON stdout from the current
  app-launched command context.
- Task context includes task-specific discovery/pairing inputs, so changing a
  verification code, target address, browse listen/timeout, or advertise
  parameter prevents stale output from being promoted as current.
- Discovery input edits clear only the relevant discovery snapshot instead of
  wiping unrelated durable status/report/health evidence.
- Discovery `trusted=true` is treated as unexpected review evidence and never
  shown as a green trust state.

## Remaining Boundaries

- T-006 does not add sync queue/run/loop/watch/network app controls.
- T-006 does not add Merkle/root-comparison evidence.
- T-006 does not add signing, notarization, bundled CLI provenance, Local
  Network permission diagnostics, or firewall readiness checks.
- T-006 does not make LAN discovery a trust mechanism or implement automatic
  endpoint selection.
