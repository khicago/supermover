# Spec Delta: core

## ADDED Requirements

### Requirement: traffic privacy level 2 acceptance evidence
The system SHALL provide repeatable release acceptance evidence for traffic
privacy level 2 on the profile-backed network path.

#### Scenario: level 2 evidence is present
- **WHEN** an operator runs the acceptance surface against a level 2 profile
- **THEN** the output records shaping configuration, applied overhead evidence,
  and residual metadata leakage.

#### Scenario: anonymity is not claimed
- **WHEN** level 2 acceptance output is generated
- **THEN** it explicitly states that level 2 is bounded traffic-shape protection
  and not anonymity.

#### Scenario: evidence is missing
- **WHEN** required applied-overhead evidence is absent
- **THEN** the acceptance result fails or reports a blocker instead of passing
  by assumption.

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
