# Spec Delta: core

## ADDED Requirements

### Requirement: durable live drift detector persistence
The system SHALL persist live detector findings into durable drift records with
explicit lifecycle semantics.

#### Scenario: live finding is persisted
- **WHEN** the detector observes target drift through the persistence surface
- **THEN** a durable drift record is written or updated in the target
  control-plane evidence.

#### Scenario: repeated finding is reconciled
- **WHEN** a later detector run sees the same logical drift
- **THEN** the durable record is updated or reopened deterministically instead
  of creating ambiguous duplicates.

#### Scenario: reconcile stays read-only until apply
- **WHEN** reconcile planning is requested
- **THEN** output distinguishes observed drift, proposed action, and any later
  mutation gate.

## MODIFIED Requirements

### Requirement: drift record truth
The tracker and docs SHALL acknowledge that explicit `drift record` persistence
already exists and that this feature concerns ongoing/automatic durability plus
reconcile semantics.

## REMOVED Requirements

None.
