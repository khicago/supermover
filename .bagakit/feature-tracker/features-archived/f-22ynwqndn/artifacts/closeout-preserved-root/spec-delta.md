# Spec Delta: core

## ADDED Requirements

### Requirement: LAN browsing discovery
The system SHALL provide bounded LAN browsing for low-information peer
candidates without treating discovery as trust.

#### Scenario: browse returns candidates
- **WHEN** peers advertise on the LAN and an operator runs browse
- **THEN** the CLI returns bounded candidate metadata suitable for explicit
  pairing or connection selection.

#### Scenario: ambiguous peers are present
- **WHEN** multiple candidates conflict or cannot be distinguished safely
- **THEN** the CLI reports ambiguity and avoids automatic selection.

#### Scenario: no peers are found
- **WHEN** no candidates appear before the timeout
- **THEN** the CLI exits with a clear no-candidates result and does not imply a
  network or trust failure.

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
