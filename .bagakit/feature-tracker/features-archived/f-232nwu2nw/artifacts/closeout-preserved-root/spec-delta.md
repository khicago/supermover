# Spec Delta: core

## ADDED Requirements

### Requirement: ongoing incremental sync queue
The system SHALL maintain durable one-way changed-file queue state for ongoing
incremental synchronization.

#### Scenario: change is queued
- **WHEN** a watched or scheduled source change is detected
- **THEN** the change is recorded durably before transfer work begins.

#### Scenario: foreground polling pass runs
- **WHEN** an operator starts local foreground polling
- **THEN** each pass snapshots the profile roots, consumes ready queue entries
  through the local publish safety path, and writes a durable run receipt.

#### Scenario: daemon restarts
- **WHEN** incremental sync restarts after interruption
- **THEN** queued and in-flight changes are reconciled from durable state.

#### Scenario: item status is reported
- **WHEN** an operator requests status
- **THEN** queued, in-flight, published, failed, and retried items are reported
  without relying only on stdout.

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
