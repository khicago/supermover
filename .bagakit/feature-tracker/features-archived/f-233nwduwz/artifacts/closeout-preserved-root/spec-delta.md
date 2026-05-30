# Spec Delta: core

## ADDED Requirements

### Requirement: repair reconcile plan/apply
The system SHALL provide a safe repair and reconcile workflow with explicit
plan/apply separation and durable receipts.

#### Scenario: plan is generated
- **WHEN** durable drift or recovery evidence is available
- **THEN** the system can produce a reconcile plan without mutating target data.

#### Scenario: apply is blocked by conflict
- **WHEN** a repair plan contains unresolved conflict classes
- **THEN** apply refuses to mutate the target and reports the blockers.

#### Scenario: apply succeeds
- **WHEN** an approved repair plan is applied
- **THEN** target mutations are preflighted and durable repair receipts are
  written.

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
