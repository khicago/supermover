# Spec Delta: core

## ADDED Requirements

### Requirement: managed agent daemon lifecycle
The system SHALL provide managed daemon lifecycle commands around profile-backed
receiver behavior.

#### Scenario: daemon starts from profile
- **WHEN** an operator starts the daemon
- **THEN** daemon runtime configuration is derived from the selected profile and
  durable status evidence is written.

#### Scenario: daemon status is inspected
- **WHEN** an operator requests daemon status
- **THEN** the CLI reports persisted foreground lifecycle evidence and recent
  redacted lifecycle events, while avoiding liveness or OS-supervision claims.

#### Scenario: daemon logs are inspected
- **WHEN** an operator requests daemon logs
- **THEN** the CLI reports scoped redacted lifecycle events from the
  profile-selected target control plane.

#### Scenario: foreground restart is requested
- **WHEN** an operator requests daemon restart
- **THEN** a scoped restart intent is persisted as pending evidence and may be
  consumed by a running foreground daemon to restart serve listeners in the
  same process.

#### Scenario: daemon stops safely
- **WHEN** an operator stops the daemon
- **THEN** the daemon exits cleanly or reports why it cannot be stopped without
  corrupting transfer control-plane state.

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
