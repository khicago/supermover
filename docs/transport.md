# Transport, Discovery, Pairing, and Privacy Schemas

This document describes the transport, discovery, pairing, and privacy schema
foundation plus the f-227 transport decision and current network wiring. It
now includes the profile-backed non-dry-run `push --network` transfer path.
Dry-run network push remains preflight-only and non-mutating.

## f-227 Transport Security Decision

Decision for the first encrypted resumable transfer slice: use TLS 1.3 over the
existing HTTP receiver protocol, with mutual certificate authentication and
profile-pinned peer identity checks.

Rationale:

- TLS 1.3 is standardized for authenticated encrypted channels and is available
  in Go's standard `crypto/tls` package.
- The receiver protocol is already an `http.Handler`; TLS-over-HTTP preserves
  current begin/status/chunk/commit semantics and test coverage.
- The first transport slice needs a small, auditable trust boundary more than it
  needs a new stream protocol.

Rejected for the first implementation path:

- QUIC/TLS: keep as a future adapter option for flow-controlled streams, path
  migration, and traffic-shaping hooks, but do not add a UDP/HTTP3 dependency
  before the receiver authentication and resume gates are wired.
- Noise: keep as a future advanced option for custom static-key handshakes, but
  do not make Supermover own handshake framing, replay policy, and termination
  rules in the first encrypted transport slice.

Implementation gates:

- generate or import real device key material for CLI/operator flows;
  schema-only `sha256:...`
  placeholder device IDs are not live cryptographic identities;
- bind the TLS peer leaf SubjectPublicKeyInfo (SPKI) SHA-256 hash to pairing
  receipt and profile pins before enabling receiver upload endpoints;
- set the receiver TLS policy to TLS 1.3 and require client certificates;
- run profile/receipt trust checks before mounting the receiver upload handler;
- disable 0-RTT for mutating receiver operations;
- bind every session to profile ID, source device ID, target device ID, serving
  profile target scope, and protocol version;
- keep resume driven by receiver status offsets and published artifacts, not by
  transport ACKs.

Certificate and key lifecycle for the current profile-backed TLS path:

- each device owns a local signing key or TLS certificate key generated with
  system entropy, or imported from an explicitly trusted operator-controlled
  key file;
- private keys are stored on the owning device only, outside the target
  `.supermover` control plane, with owner-only filesystem permissions where the
  platform supports them;
- pairing receipts and profile pins store public identity material only: leaf
  SPKI SHA-256 hash as the stable device identity, optional certificate serial
  or DER fingerprint as diagnostic evidence, verified time, method, and
  protocol version;
- short-lived self-signed leaf certificates are acceptable when the profile pins
  the leaf SPKI hash directly; chain-of-trust validation may be added later, but
  the first trust boundary is profile pinning;
- certificate validity must be checked in addition to the pinned identity;
  `NotBefore` and `NotAfter` are evaluated against the local system clock at
  connection time, and not-yet-valid or expired certificates fail closed until
  an explicit re-pair or operator renewal flow resolves them;
- certificate renewal that preserves the same SPKI hash is not identity
  rotation; key rotation changes the SPKI hash and is never silent. The
  current loader enforces certificate validity and SPKI pins from the profile;
  operator UX for generation, renewal, rotation, revocation, and preserving
  superseded receipts remains planned;
- planned unpair/revoke UX must write durable revocation evidence, invalidate
  the profile pins, block transfer until a new pairing receipt is written, and
  prevent stale receipt reuse even if an older profile snapshot is restored;
- suspected lost or stolen private keys are handled as revocation plus explicit
  re-pairing, not as a background refresh;
- the source and receiver both fail closed if the profile pin, pairing receipt,
  live TLS peer SPKI hash, source device ID, target device ID, or profile target
  root scope disagree.

Primary sources used for this decision:

- RFC 8446 (TLS 1.3): https://www.rfc-editor.org/rfc/rfc8446.html
- Go `crypto/tls`: https://pkg.go.dev/crypto/tls
- Go `net/http`: https://pkg.go.dev/net/http
- RFC 9000 (QUIC): https://www.rfc-editor.org/rfc/rfc9000.html
- RFC 9001 (QUIC/TLS): https://www.rfc-editor.org/rfc/rfc9001.html
- Noise Protocol Framework: https://noiseprotocol.org/noise.html

Local research evidence is recorded under
`.bagakit/researcher/topics/frontier/secure-resumable-transport-decision/`.

## Device Identity

Device identity is represented as `transport.DeviceID`, a string wrapper for a
pinned public key or fingerprint encoding.

Schema-layer validation is intentionally conservative:

- values must be printable ASCII token strings
- whitespace and friendly labels are rejected
- values must resemble a public key or fingerprint, such as `sha256:...`,
  `ed25519:...`, `pubkey:...`, colon-separated fingerprints, or hex
  fingerprints

The schema validation is not cryptographic verification. Current mTLS identity
loading happens through the profile-selected TLS identity files; the TLS path
parses certificates, derives the leaf SubjectPublicKeyInfo (SPKI) SHA-256 pin,
and validates it against profile/pairing evidence at connection time. Operator
UX for generating, rotating, and revoking those identities remains planned.

## Pairing Receipt

`transport.PairingReceipt` records the trust event that connects a source device
to a target device for a profile.

Fields:

- source device ID
- target device ID
- profile ID
- target ID in the durable control-plane receipt
- method: `sas`, `short_code`, `qr`, or `tofu`
- verified time
- verification phrase and/or verification hash
- protocol version

Validation requires distinct source and target device IDs, a valid profile ID,
a supported method, a non-zero verified time, at least one verification proof,
and a protocol version such as `supermover/1`.

`control.PairingReceipt` persists this trust event under
`.supermover/pairings/<id>.json` with the same source/target device fields plus
control-plane `version`, `id`, `target_id`, and `device_public_key`. The
control receipt validator reuses the transport schema rules and additionally
requires `device_public_key` to equal `target_device_id`.

`internal/pairing` currently provides local control-plane evidence validation:
it loads the receipt referenced by `profile.target.pairing_receipt_id` from the
profile target path and checks profile ID, target ID, pinned device key,
receipt ID, and pairing timestamp. It does not contact a peer. Live peer
authentication happens only when the `serve` receiver and non-dry-run
`push --network` path validate TLS peer SPKI pins against profile and pairing
evidence.

## Privacy Policy

`transport.PrivacyPolicy` supports levels 1, 2, and 3 as typed values. Level 2
is the first schema-constrained traffic-shaping posture. Current operator
`push --network` supports only traffic level 2; it uses the protocol client and
network runner, so level 2 padding, batching, and bounded jitter apply on that
network path when the profile supplies a level 2 policy. Levels 1 and 3 remain
schema/planning values for this network path. Local push still reports traffic
shaping as not applied.

Level 2 requires:

- record padding enabled with a non-zero padding bucket
- batching enabled with non-zero byte and count limits
- low-information discovery enabled
- non-zero jitter budget

Level 2's privacy claim is limited to overhead evidence from bounded padding,
batching, and jitter reductions. It is not anonymity and does not hide total
bytes, transfer duration, peer IP addresses, LAN presence, or that Supermover
is being used.

The internal protocol client now has a deterministic `padding-v1` frame wrapper
for `/v1/chunks` and `/v1/chunk-batches` when a level 2 policy is supplied
through the library request. The receiver strips that frame before chunk digest
checks and manifest validation, so receiver status offsets, replay checks, final
file hashes, and resume remain based on plain payload bytes. `networkrun`
records applied padding, batching, and bounded jitter overhead for this network
path in `network-transfer.json`. `push --network --dry-run` does not call the
protocol client or write session artifacts.

`transport.DefaultPrivacyPolicy(transport.PrivacyLevel2)` returns the current
level 2 defaults:

- padding bucket: 64 KiB
- batch max bytes: 1 MiB
- batch max count: 64
- jitter budget: 250
- low-information discovery: enabled

Levels 1 and 3 are accepted as schema values without additional behavioral
requirements in this slice and are rejected by current operator `push
--network`.

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

Canonical values are derived from the typed advertisement fields. Extra
unauthenticated TXT values cannot override `svc`, `proto`, `nonce`, or `caps`.

Validation rejects unauthenticated TXT fields that disclose identity, local
layout, profile data, or inventory size. Rejected examples include usernames,
paths, hostnames, profile labels, file counts, and friendly names.

Discovery remains an address-hint mechanism only. Pairing receipts and pinned
device identity are durable trust evidence, but they do not authenticate a live
peer by themselves. Current `serve` and non-dry-run `push --network` use
profile and pairing evidence as gates, then require live pinned TLS 1.3 mTLS
peer validation before receiver upload routes are exposed or used.

## CLI Adapter Status

The `serve` command is wired as a low-information target listener for pairing
and, when a profile is already paired with complete profile-selected network
material, as an authenticated receiver listener. `pair` can consume the pairing
endpoint with an operator-entered verification code to write a local pairing
receipt and update profile target pins. `discover` also has a low-information
explicit-address adapter. `push --network` is wired as the source-side
profile-backed network path:

```bash
go run ./cmd/supermover serve --profile <target-profile>
go run ./cmd/supermover discover --timeout 2s
go run ./cmd/supermover discover --address 127.0.0.1:9000 --format json
go run ./cmd/supermover pair --profile <source-profile> --target <address> --verification-code <code>
go run ./cmd/supermover push --network --profile <source-profile> --dry-run
go run ./cmd/supermover push --network --profile <source-profile> --session <session-id>
```

Their help and usage validation are implemented so scripts can depend on the
shape of the flow. `serve` validates the target profile and target root, binds
the requested pairing listen address, exposes low-information `/v1/discovery`,
prints the verification code on the target console, and returns `/v1/pairing`
bootstrap material only when that code is presented. If profile network material
is absent, receiver upload routes remain disabled. If a paired profile has
complete `network.receiver_url` plus `network.local_tls_identity`, `serve`
derives the receiver listen address from that URL, loads the local certificate
and key from the profile, validates pairing trust and the target certificate
SPKI pin, then mounts receiver begin/status/chunk/commit routes behind pinned
TLS 1.3 mutual authentication. `discover` returns untrusted address hints only;
with no configured source it waits for the requested timeout and returns an
empty hint list. `--address` values are operator-provided hint material and
still disclose peer address metadata. `pair` validates the verification code,
writes a local `control.PairingReceipt`, updates the profile's pinned target
identity fields, and writes a profile snapshot for audit. The profile schema
defines `network.receiver_url` and `network.local_tls_identity` as the SSOT for
operator network connection material; these are references and pins, not a
runtime override surface. `push --network` reads the profile, pairing receipt
evidence, and required network material, refuses unpaired or mismatched profiles
and paired profiles without that material. Without `--dry-run`, it connects to
the profile-selected pinned mTLS receiver, streams the scanned source through
`protocolclient`, and writes network transfer evidence through `networkrun`.
With `--dry-run`, it stops after preflight and writes no target artifacts.

Receiver adapters now back the operator `serve` receiver mode:
`receiver.NewAuthenticatedHandlerFromProfile` validates profile trust evidence
and builds a `FileStore`-backed authenticated receiver handler. The lower-level
`receiver.NewAuthenticatedHandler` requires a transport-authenticated peer
context before begin/status/chunk/commit routes can reach the store, validates
session scope against durable receiver metadata, and rejects unsafe target roots.
`receiver.NewTLSReceiverFromProfile` validates profile/receipt trust, verifies
that the configured target certificate's leaf SPKI hash matches the profile's
pinned target device ID, installs a TLS 1.3 server config that requires a client
certificate, and wraps the authenticated handler with TLS-derived peer context.
`internal/receiverserve` is the CLI-facing adapter that loads the profile's
local TLS identity and runs this receiver on `network.receiver_url`. This
exposes authenticated receiver routes only when the profile trust and TLS
material validate.

The source protocol client and TLS client config helper back non-dry-run
`push --network`:
`internal/protocolclient` builds receiver begin requests from `scan.Result`,
skips reserved `.supermover` and unsupported special entries with warnings,
streams regular files from disk in bounded JSON chunks, wraps chunk requests in
deterministic `padding-v1` frames when level 2 padding is configured, resumes
from receiver status committed offsets, retries commit for staged/published
sessions, and commits through the receiver protocol. `transport.ClientTLSConfig`
verifies the target leaf SPKI pin and presents the source certificate for mTLS.
The protocol client still revalidates source evidence before commit, including
staged/published commit retries. It returns warning records to `networkrun`,
which persists them as target control-plane artifacts before exposing a
published network transfer. `networkrun` also records
`.supermover/sessions/<session>/network-transfer.json` and maps outcomes such
as `auth_refused`, `interrupted`, `needs_repair`, `publish_failed`, and
`failed`. The receiver protocol can represent zero-byte regular files as an
explicit final completion record with offset 0 and the empty-file SHA-256
digest, and the receiver requires staged evidence before publish. The
operator-facing profile-backed `push --network` path now sends that completion
through `protocolclient`, `networkpush`, and the CLI end to end. Automated
evidence covers the bounded same-session source-interruption case where
receiver begin succeeds, payload bytes are accepted, in-flight
network-transfer evidence may be persisted, and a same-profile/same-session
CLI/Runner rerun recovers from receiver status with preserved level 2 overhead
evidence. The bounded f-22wnwd5pe/T-002 matrix also covers receiver listener
restart over preserved target state, commit-only retry, published-session retry,
and missing/corrupt/mismatched prior transfer evidence that fails closed with
`payload_overhead_missing`. This does not make `recover` a network recovery
command and does not change the remaining limits around LAN browsing, daemon
workflow, ongoing sync, broad arbitrary interruption recovery, broad resume
acceptance, arbitrary process-kill recovery, power-loss recovery, or anonymity.

Current skeleton limits:

- `serve` does not browse or advertise on LAN or disclose inventory. Receiver
  upload endpoints are enabled only on the profile-selected mTLS listener after
  pairing and network material validate.
- `discover` does not browse LAN services, perform mDNS/DNS-SD, or trust any
  address.
- `pair` writes local pairing evidence only after operator verification. It is
  not a defense against active endpoint deception before a non-dry-run
  `push --network` validates the pinned TLS peer.

LAN browsing, a transfer daemon, ongoing incremental sync, broad arbitrary
interruption recovery, broad resume acceptance, network `recover`, automatic
retry/reconcile, power-loss recovery, and arbitrary process-kill recovery remain
planned future slices.
