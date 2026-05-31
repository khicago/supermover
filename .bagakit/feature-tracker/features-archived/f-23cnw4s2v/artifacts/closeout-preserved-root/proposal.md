# Feature Proposal: f-23cnw4s2v

## Why
- The app currently defaults to `Control Room`, but `Control Room` is rendered as
  a peer inside the same `Sections` list as `Setup`, `Devices`, `Pairing`,
  `Transfer`, `Sync`, `Verification`, `Evidence`, `Drift Review`, and
  `Settings`.
- That makes the first screen look like one workflow tab among many, while its
  actual job is broader: orient the operator, summarize safety gates, expose the
  next action, and deep-link into the correct owner page.
- Several sections overlap in the user's mental model:
  - `Devices` and `Pairing` are both connection/trust state.
  - `Transfer` and `Sync` are both source-owned movement execution.
  - `Verification` and `Drift Review` are both correctness/repair.
  - `Setup` and `Settings` blur profile-backed migration inputs with app
    preferences.
- Many surfaces explain state with paragraph text where a compact icon, state
  color, and hover/help affordance would preserve the main path more clearly.

## Goal
- Simplify the native app workbench information architecture by separating Control Room/Home from workflow sections, merging overlapping pages, and reducing text-heavy surfaces into icon/tooltip/detail-on-demand patterns while preserving evidence-first migration safety.

## Principle Layer
- What: Treat `Control Room` as `Home`, not as a workflow section. Workflow pages
  become owner pages for one operator intent each.
- Why: A migration workbench needs one obvious place to understand current state
  and one obvious place to act. If every page both summarizes and operates the
  same concepts, the user cannot tell where truth lives.
- Intended generalization: Every concept gets one write owner and may have
  read-only summary appearances elsewhere. Summary appearances must deep-link to
  the owner instead of duplicating controls.
- Failure boundary: Do not simplify by hiding safety gates, evidence freshness,
  role gates, profile SSOT, or unsupported states. A green status still requires
  durable evidence.
- Behavior examples:
  - `Home` shows a compact profile gate icon. Editing profile paths happens in
    `Prepare`.
  - `Home` shows a receiver/trust gate icon. Pairing and receiver state are owned
    by `Connect`.
  - `Home` shows movement readiness and last run. Queue mutation and run controls
    are owned by `Move`.
  - `Home` shows verification freshness. Drift, reconcile, and prune decisions
    are owned by `Verify & Repair`.
- Evidence refs:
  - `macos/SuperMoverApp/ContentView.swift` currently defaults to
    `selectedSection = .controlRoom`.
  - `macos/SuperMoverApp/SidebarNavigation.swift` currently puts
    `controlRoom` inside `AppSection.allCases` with workflow pages.
  - `macos/SuperMoverApp/ControlRoomView.swift` already composes the cockpit-like
    panels, so the product concept is present but the navigation taxonomy is not.

## Scope
- In scope:
  - Navigation taxonomy and sidebar grouping.
  - Page ownership map for profile, connection, movement, verification, evidence,
    and settings.
  - Low-density text reduction using SF Symbols, tooltips, disclosure, and
    inspector/detail affordances.
  - Design drafts for the next SwiftUI implementation pass.
  - Static design board:
    `artifacts/workbench-ia-design-board.html`.
  - Rendered design board PNG:
    `artifacts/workbench-ia-design-board.png`.
  - Corrected high-fidelity design target based on the current implemented app
    language:
    `artifacts/workbench-ia-current-language.html` and
    `artifacts/workbench-ia-current-language.png`.
- Out of scope:
  - Changing CLI behavior or profile-file SSOT.
  - Claiming new LAN, crypto, traffic-shape, or release readiness.
  - Removing raw evidence access.
  - Reworking the T-011 acceptance harness.

## Proposed Taxonomy
- Home:
  - `Control Room` becomes the single default landing surface and is visually
    separated from workflow sections.
  - It summarizes the current run posture and exposes deep links, not duplicate
    owner controls.
- Workflow:
  - `Prepare`: former `Setup`; owns role, profile selection, source/target root
    authoring, and profile lint.
  - `Connect`: merges `Devices` and `Pairing`; owns device discovery, target
    trust, receiver readiness, and connection diagnostics.
  - `Move`: merges `Transfer` and `Sync`; owns dry run, bounded push, queue,
    watch/loop state, and run supervision.
  - `Verify & Repair`: merges `Verification` and `Drift Review`; owns verify,
    drift review, reconcile, prune, and repair decisions.
- Evidence:
  - `Evidence Vault`: owns durable receipts, reports, raw JSON, health,
    lifecycle logs, and export/share actions.
- System:
  - `Settings`: app preferences and command defaults only. It should not become a
    second profile editor.

## Ownership Rules
- Profile paths and role: owner `Prepare`; summaries in `Home` and sidebar.
- Device and trust state: owner `Connect`; summaries in `Home`.
- Movement execution and queue mutation: owner `Move`; summaries in `Home` and
  `Evidence Vault`.
- Verification, drift, reconcile, prune: owner `Verify & Repair`; summaries in
  `Home` and `Evidence Vault`.
- Durable artifacts and raw evidence: owner `Evidence Vault`; summaries
  everywhere else must link back to it.
- App preferences: owner `Settings`; never override profile truth casually.

## Design Draft A: Home-First Minimal Cockpit
```
Sidebar
  SuperMover
  [Home]                         <- separated, selected by default

  Workflow
    Prepare
    Connect
    Move
    Verify & Repair

  Evidence
    Evidence Vault

  System
    Settings

Home
  [Run posture: Ready / Blocked / Needs evidence]
  [Profile] [Trust] [Receiver] [Move] [Verify] [Evidence]   <- icon gates

  Next action: Start dry run / Fix profile / Verify target
  Recent run:  state, time, command icon, evidence link

  Activity strip: last command, last artifact, last warning
  Inspector drawer opens for explanations and raw details
```

Strength: cleanest mental model. Risk: may feel sparse unless the right drawer
and recent-run affordances are present.

## Design Draft B: Phase Rail Workbench
```
Sidebar
  [Home]

  Workflow
    1 Prepare
    2 Connect
    3 Move
    4 Verify & Repair

  Evidence
    Vault

Home
  Horizontal phase rail:
    Prepare -> Connect -> Move -> Verify -> Evidence

  Main pane:
    selected phase summary + one primary action

  Right rail:
    active process, safety gates, recent evidence
```

Strength: makes the migration path obvious without pretending every migration is
linear. Risk: needs careful copy so `Sync` does not look like a mandatory fourth
step when it is a movement mode.

## Design Draft C: Evidence-First Operator Console
```
Sidebar
  [Home]
  Workflow: Prepare / Connect / Move / Verify & Repair
  Evidence: Vault
  System: Settings

Home
  Evidence freshness header:
    profile hash, target pin, last run id, verify receipt

  Required evidence lane:
    missing profile lint / missing pair receipt / stale verify

  Action lane:
    only actions unlocked by current evidence

  Bottom:
    compact event console and raw artifact shortcuts
```

Strength: best for safety-critical runs and audits. Risk: too technical for the
first-run operator unless paired with a clear next action.

## Recommended Direction
- Do not use the first static design board as a visual target. It is retained as
  a low-fidelity IA sketch only; it is weaker than the current implemented app
  design and should not guide visual implementation.
- Use `artifacts/workbench-ia-current-language.html` and
  `artifacts/workbench-ia-current-language.png` as the corrected visual target.
- Keep the current implemented app language: light native workbench shell,
  compact sidebar status, large Control Room migration surface, safety gate
  panel, metric strip, stage rail, console, and explicit task dispatch.
- Apply Draft B's phase rail idea only as an IA simplification inside that
  existing language, plus Draft A's separated `Home`.
- Keep Draft C's evidence freshness model as the status logic behind the phase
  rail, not as the dominant first-screen visual.
- Use `artifacts/workbench-ia-design-board.html` as the handoff visual for the
  three drafts and the recommended hybrid; use
  `artifacts/workbench-ia-design-board.png` for quick visual review of the
  rejected low-fidelity exploration.
- Rename visible concepts in the app:
  - `Control Room` remains the page heading, but nav label becomes `Home`.
  - `Setup` becomes `Prepare`.
  - `Devices` + `Pairing` become `Connect`.
  - `Transfer` + `Sync` become `Move`, with mode controls inside the page.
  - `Verification` + `Drift Review` become `Verify & Repair`.
  - `Evidence` becomes `Evidence Vault`.
- Replace low-value subtitles and repeated explanatory paragraphs with:
  - icon plus state label
  - tooltip/help popover for the why
  - detail drawer for commands, evidence paths, and failure explanations
  - raw JSON only in `Evidence Vault` or explicit debug disclosure

## Acceptance Criteria
- `Home` is selected by default and is visually outside the workflow section
  group.
- Sidebar has no flat `Sections` bucket containing all pages as peers.
- Every high-level concept has one owner page and documented read-only summary
  locations.
- `Devices`, `Pairing`, `Transfer`, `Sync`, `Verification`, and `Drift Review`
  are no longer exposed as six separate top-level destinations.
- Low-density status surfaces use icon-first presentation with hover/help
  affordances, while safety-critical failure text remains visible at the moment
  it blocks action.
- No UI status becomes greener or more optimistic than the underlying durable
  evidence supports.
- Existing CLI/app behavior stays wired through current command contracts; no
  runtime overrides are introduced as a design shortcut.

## Transfer Checks
- Swift UI ownership tests or model tests cover the sidebar grouping and default
  selected page.
- Manual smoke:
  - app opens to `Home`
  - workflow pages are grouped as `Prepare`, `Connect`, `Move`,
    `Verify & Repair`
  - old owner concepts are reachable through merged pages
  - evidence/raw artifact access remains available
- Documentation or feature tracker notes distinguish design proposal, wired UI,
  and migration capability status.

## Impact
- Code paths:
  - `macos/SuperMoverApp/SidebarNavigation.swift`
  - `macos/SuperMoverApp/ContentView.swift`
  - `macos/SuperMoverApp/ControlRoomView.swift`
  - likely page hosts for setup/devices/pairing/transfer/sync/verification/drift
- Tests:
  - Add navigation taxonomy tests where the SwiftUI model can be tested without
    snapshot fragility.
  - Add or update manual app smoke notes after implementation.
- Rollout notes:
  - Land as UI taxonomy cleanup first; do not combine with CLI behavior changes.
  - Keep existing page internals reachable during the first implementation pass,
    even if merged under new owners.
