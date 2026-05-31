# Spec Delta: core

## ADDED Requirements

### Requirement: prune release readiness truth
The system SHALL expose read-only approval lifecycle and receipt-comparison
truth for prune release review without changing prune authorization semantics.

#### Scenario: approvals are summarized
- **WHEN** an operator reads prune release evidence
- **THEN** active, stale, expired, consumed, receipt-attention, or superseded
  approval states are reported from durable artifacts.

#### Scenario: stale approval is detected
- **WHEN** candidates differ from the approval evidence
- **THEN** release readiness reports a blocker or stale state before apply.

#### Scenario: receipt comparison is requested
- **WHEN** prune receipts exist after apply or interrupted apply
- **THEN** release output can compare approvals, current dry-run evidence, and
  the latest matching receipt for audit without mutating target state.

#### Scenario: compact status needs prune release truth
- **WHEN** compact `status` is requested
- **THEN** aggregate prune readiness counts and prune review action are exposed
  without listing full approval or receipt inventory.

## MODIFIED Requirements

### Requirement: prune review truth
The broader release workflow SHALL build on existing `prune review` rather than
listing read-only review as still missing.

## REMOVED Requirements

None.
