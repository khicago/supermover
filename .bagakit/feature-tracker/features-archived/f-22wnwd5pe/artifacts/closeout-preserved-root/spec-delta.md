# Spec Delta: core

## ADDED Requirements

### Requirement: network interruption acceptance matrix
The system SHALL provide a repeatable acceptance matrix for supported network
interruption and resume cases.

Current bounded matrix rows are:

| Mode | Acceptance |
| --- | --- |
| Source/network interruption after receiver-accepted payload | Supported for the same profile/session when receiver status reports partial committed bytes and prior payload-overhead evidence remains auditable. |
| Receiver listener restart after receiver-accepted payload | Supported when the receiver is restarted over the same profile-selected target/control-plane state and the same profile/session is retried with prior auditable payload-overhead evidence. |
| Payload complete but commit not completed | Supported as commit-only retry at the `networkpush` layer with no payload reupload and preserved prior overhead evidence. |
| Already published same-session retry | Supported as `published_retry` with no chunk upload and preserved published overhead evidence. |
| Missing, corrupt, mismatched, non-published, or payload-empty prior transfer evidence | Blocked as `needs_repair` / `payload_overhead_missing`; the system does not fabricate recovery or privacy overhead. |
| Arbitrary OS process kill, power loss, daemon/OS-service restart recovery, network `recover`, broad retry policy, broad reconcile, receiver crash UX | Future work unless separate command/process fixtures are added. |

#### Scenario: interrupted transfer resumes
- **WHEN** a supported transfer interruption is injected
- **THEN** a later retry resumes or reconciles from durable evidence without
  corrupting target content or receipts.

#### Scenario: unsupported recovery mode
- **WHEN** an interruption mode has no durable recovery proof
- **THEN** the acceptance surface reports it as unsupported or blocked rather
  than passing.

#### Scenario: recovery claim is bounded
- **WHEN** docs or release output describe recovery
- **THEN** they name the covered failure modes and avoid arbitrary recovery
  claims.

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
