# Spec Delta: core

## ADDED Requirements

### Requirement: Native macOS operator shell
The system SHALL provide a native macOS desktop operator shell that launches
current SuperMover CLI workflows without redefining the underlying product
contract.

#### Scenario: launch current local workflow
- **WHEN** an operator selects a valid profile and runs a local publish or
  review task from the app
- **THEN** the app launches the existing CLI command and presents its output
  without inventing a second execution engine

#### Scenario: launch current foreground workflow
- **WHEN** an operator starts `serve` or `dashboard` from the app
- **THEN** the app keeps the task foreground-attached, exposes output, and
  allows explicit stop behavior

#### Scenario: preserve product truth in labels
- **WHEN** the app presents current tasks
- **THEN** it does not label bounded publish or network push as ongoing sync,
  and it does not label foreground `serve` as detached daemon behavior

### Requirement: Structured operator review surfaces
The app SHALL prefer current structured CLI or artifact-backed truth over raw
stdout when current JSON surfaces already exist.

#### Scenario: render current status and report truth
- **WHEN** the operator runs `status`, `report`, `health`, `drift list`,
  `prune review`, `daemon status`, or `daemon logs`
- **THEN** the app renders structured summaries from the current JSON payloads
  instead of only showing raw command output

#### Scenario: render persisted drift follow-through truth
- **WHEN** the operator runs `drift acknowledge`, `drift resolve`,
  `reconcile plan`, or narrow `reconcile apply`
- **THEN** the app renders the returned persisted drift or reconcile result as
  structured review data without renaming it into broad automatic repair

### Requirement: CLI-truthful operator copy
The app SHALL preserve current SuperMover scope boundaries in product-facing
copy and task names.

#### Scenario: unsupported product claims remain blocked
- **WHEN** a current repo feature is still planned, such as LAN browsing,
  ongoing incremental sync, detached daemon install, or broad automatic repair
- **THEN** the app copy avoids presenting that behavior as available

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
