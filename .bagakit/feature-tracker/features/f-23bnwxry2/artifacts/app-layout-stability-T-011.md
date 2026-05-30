# T-011 App Layout Stability Follow-up

This follow-up tightens the native macOS workbench shell after manual review
found that page titles and owner-page controls could move out of view during
vertical scrolling or window resizing.

## Scope

- Keep owner-page mode controls for Connect, Move, and Verify/Repair outside
  the page body scroll view so the operator can switch sub-surfaces without
  losing the section context.
- Keep detail-page content inside the body scroll view so long evidence panels
  remain scrollable without dragging the owner toolbar away.
- Let sidebar navigation scroll independently while preserving a compact
  workstation footer; at constrained heights the footer drops the secondary
  safety panel before it pushes navigation or status out of view.
- Record the fixed-owner-mode policy on `AppSection` rather than embedding that
  decision only inside the view tree.

## Non-goals

- This does not redesign the information architecture, add new migration
  behavior, or change CLI command inputs.
- This does not claim final two-machine installed-app acceptance, signed
  distribution readiness, notarization, stapling, Gatekeeper proof, or Local
  Network/firewall prompt evidence.

## Validation

- `swift test --package-path macos --filter 'WorkbenchNavigationTests|WorkbenchChromeTests|AppStoreTests'`:
  pass; 136 tests, 0 failures.
- `WorkbenchNavigationTests/testOwnerSectionsDeclareFixedModeStripPolicy` pins
  the owner-section policy for fixed mode strips.
- `WorkbenchChromeTests/testFixedOwnerModeStripLeavesDedicatedBodyGap` pins the
  body gap used when the fixed owner toolbar is present.
