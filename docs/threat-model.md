# Threat Model

Supermover v1 assumes the target machine is trusted and should receive
plaintext files.

## In Scope

- Content confidentiality and integrity in transit for non-dry-run
  `push --network`.
- TLS 1.3 channel authentication and encryption adapters around the receiver
  protocol.
- CLI/operator wiring for explicit first pairing and persistent device identity
  pinning, with TLS peer leaf SPKI hashes bound to those pins before transfer.
- Planned LAN discovery treated as unauthenticated address hints.
- Planned sparse discovery advertisements.
- Protocol-client traffic privacy level 2 padding, batching, and bounded timing
  jitter to reduce some record-size, burst, and timing signals.
- Audit records for privacy-relevant choices.

## Transport Decision Limits

The first encrypted transfer implementation uses TLS 1.3 over the existing HTTP
receiver protocol at the library layer. TLS is the channel security boundary; it
does not replace Supermover's application checks. Receiver begin requests,
pairing receipts, profile pins, session IDs, manifests, status offsets, and
commit receipts must still be validated as application state.

Non-dry-run `push --network` implements source-side encrypted file transfer
through the profile-selected pinned TLS 1.3 mTLS receiver. `push --network
--dry-run` is intentionally preflight-only and does not contact the receiver or
write target artifacts.

For accepted connections with matching pinned identities, TLS provides channel
authentication, confidentiality, and integrity for content and protocol
payloads. It does not stop denial-of-service, endpoint compromise, replay
attempts against APIs that later enable replayable early data, or traffic
metadata observation. It also does not hide total bytes, transfer duration, peer
IP addresses, LAN presence, or that Supermover is being used. The level 2
protocol-client path has bounded padding, batching, and timing jitter, but it
does not provide anonymity.

## Out Of Scope For V1

- Strong anonymity.
- Hiding total transfer size, transfer duration, peer IP addresses, LAN
  presence, or the fact that Supermover is being used.
- Protecting against endpoints with stolen private keys or against an operator
  who pairs the wrong target.
- Untrusted target encrypted repository mode.
- Protection from a compromised trusted endpoint.
