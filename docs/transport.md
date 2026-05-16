# Transport, Discovery, Pairing, and Privacy Schemas

This document describes the T-006 schema foundation. It does not define real
networking behavior yet.

## Device Identity

Device identity is represented as `transport.DeviceID`, a string wrapper for a
future pinned public key or fingerprint encoding.

Schema-layer validation is intentionally conservative:

- values must be printable ASCII token strings
- whitespace and friendly labels are rejected
- values must resemble a public key or fingerprint, such as `sha256:...`,
  `ed25519:...`, `pubkey:...`, colon-separated fingerprints, or hex
  fingerprints

The validation is not cryptographic verification. Real key parsing belongs in a
later transport implementation.

## Pairing Receipt

`transport.PairingReceipt` records the trust event that connects a source device
to a target device for a profile.

Fields:

- source device ID
- target device ID
- profile ID
- method: `sas`, `short_code`, `qr`, or `tofu`
- verified time
- verification phrase and/or verification hash
- protocol version

Validation requires distinct source and target device IDs, a valid profile ID,
a supported method, a non-zero verified time, at least one verification proof,
and a protocol version such as `supermover/1`.

## Privacy Policy

`transport.PrivacyPolicy` supports levels 1, 2, and 3 as typed values. Level 2
is the first operationally constrained level.

Level 2 requires:

- record padding enabled with a non-zero padding bucket
- batching enabled with non-zero byte and count limits
- low-information discovery enabled
- non-negative jitter budget

`transport.DefaultPrivacyPolicy(transport.PrivacyLevel2)` returns the current
level 2 defaults:

- padding bucket: 64 KiB
- batch max bytes: 1 MiB
- batch max count: 64
- jitter budget: 250
- low-information discovery: enabled

Levels 1 and 3 are accepted as schema values without additional behavioral
requirements in this slice.

## Discovery Advertisement

`discovery.Advertisement` models unauthenticated LAN discovery data. It is
restricted to low-information hints:

- service type
- protocol version
- ephemeral nonce
- minimal capability flags

`Advertisement.TXT` emits only allowlisted TXT keys:

- `svc`
- `proto`
- `nonce`
- `caps`

Validation rejects unauthenticated TXT fields that disclose identity, local
layout, profile data, or inventory size. Rejected examples include usernames,
paths, hostnames, profile labels, file counts, and friendly names.

Discovery remains an address-hint mechanism only. Pairing receipts and pinned
device identity establish trust.
