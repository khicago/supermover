# Threat Model

Supermover v1 assumes the target machine is trusted and should receive
plaintext files.

## In Scope

- Content confidentiality and integrity in transit.
- Explicit first pairing and persistent device identity pinning.
- LAN discovery treated as unauthenticated address hints.
- Sparse discovery advertisements.
- Padding, batching, and bounded timing jitter to reduce obvious metadata
  leakage.
- Audit records for privacy-relevant choices.

## Out Of Scope For V1

- Strong anonymity.
- Hiding total transfer size, transfer duration, peer IP addresses, LAN
  presence, or the fact that Supermover is being used.
- Untrusted target encrypted repository mode.
- Protection from a compromised trusted endpoint.

