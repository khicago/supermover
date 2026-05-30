import SwiftUI

struct PairingSectionView: View {
  let model: PairingSectionModel
  let localization: AppChromeLocalization
  var onRegenerateCode: (() -> Void)?
  var onApproveRequest: (() -> Void)?
  var onRejectRequest: (() -> Void)?
  var onCancel: () -> Void
  var onBack: () -> Void
  var onContinue: () -> Void

  init(
    model: PairingSectionModel,
    localization: AppChromeLocalization = AppChromeLocalization(language: .english),
    onRegenerateCode: (() -> Void)? = nil,
    onApproveRequest: (() -> Void)? = nil,
    onRejectRequest: (() -> Void)? = nil,
    onCancel: @escaping () -> Void = {},
    onBack: @escaping () -> Void = {},
    onContinue: @escaping () -> Void = {}
  ) {
    self.model = model
    self.localization = localization
    self.onRegenerateCode = onRegenerateCode
    self.onApproveRequest = onApproveRequest
    self.onRejectRequest = onRejectRequest
    self.onCancel = onCancel
    self.onBack = onBack
    self.onContinue = onContinue
  }

  var body: some View {
    DetailPageHost(
      header: .init(title: model.title, subtitle: model.subtitle),
      asideWidth: WorkbenchLayoutMetrics.pairingAsideWidth,
      headerAccessory: {
        headerBar
      },
      primary: {
        VStack(alignment: .leading, spacing: 18) {
          stepRail
          leftWorkbench
        }
      },
      aside: {
        inspector
          .fixedSize(horizontal: false, vertical: true)
      },
      footer: {
        footerActions
      }
    )
  }

  private var headerBar: some View {
    HStack(alignment: .center, spacing: 12) {
      if let badge = model.badge {
        statusBadge(badge)
      }

      Spacer(minLength: 12)

      if let updated = model.lastUpdatedLabel {
        Label(updated, systemImage: "clock")
          .font(.system(size: 12, weight: .medium))
          .foregroundStyle(SMColor.secondaryText)
      }
    }
  }

  private var stepRail: some View {
    WorkbenchToolbarStrip(padding: EdgeInsets(top: 10, leading: 14, bottom: 10, trailing: 14)) {
      HStack(spacing: 10) {
        ForEach(Array(model.steps.enumerated()), id: \.element.id) { index, step in
          HStack(spacing: 10) {
            stepMarker(step)
            Text(step.title)
              .font(.system(size: 13, weight: step.isCurrent ? .semibold : .medium))
              .foregroundStyle(step.isCurrent ? SMColor.primaryText : SMColor.secondaryText)
          }
          .frame(maxWidth: .infinity, alignment: .leading)

          if index < model.steps.count - 1 {
            Capsule()
              .fill(SMColor.hairline)
              .frame(width: 28, height: 2)
          }
        }
      }
    }
  }

  private var leftWorkbench: some View {
    VStack(alignment: .leading, spacing: 16) {
      endpointsPanel
      checklistPanel
      if let request = model.pendingRequest {
        pendingRequestPanel(request)
      }
      codePanel
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private var endpointsPanel: some View {
    WorkbenchPanel(title: localization.text("Trust Ceremony"), subtitle: model.summaryLine) {
      VStack(alignment: .leading, spacing: 18) {
        HStack(alignment: .top, spacing: 22) {
          endpointCard(model.source, roleLabel: localization.text("Source"))

          VStack(spacing: 12) {
            Spacer(minLength: 18)
            Image(systemName: model.transport.symbolName)
              .font(.system(size: 26, weight: .medium))
              .foregroundStyle(model.transport.state.color)
            Text(model.transport.title)
              .font(.system(size: 13, weight: .semibold))
              .foregroundStyle(SMColor.primaryText)
            Text(model.transport.detail)
              .font(.system(size: 12))
              .foregroundStyle(SMColor.secondaryText)
              .multilineTextAlignment(.center)
              .frame(maxWidth: 150)
            Spacer(minLength: 18)
          }
          .frame(maxWidth: .infinity)
          .overlay(alignment: .center) {
            Capsule()
              .fill(SMColor.hairline.opacity(0.8))
              .frame(height: 1)
              .padding(.horizontal, -18)
          }

          endpointCard(model.target, roleLabel: localization.text("Target"))
        }

        Divider()
          .overlay(SMColor.hairline)

        HStack(alignment: .top, spacing: 24) {
          transportSummaryColumn
          trustSummaryColumn
          expirySummaryColumn
        }
      }
    }
  }

  private func endpointCard(_ endpoint: PairingEndpointSummary, roleLabel: String) -> some View {
    RouteEndpointPane(
      roleLabel: roleLabel,
      title: endpoint.name,
      address: nil,
      iconName: endpoint.iconName,
      tint: endpoint.state.color,
      details: endpoint.metadata.map {
        RouteEndpointPaneDetail(id: $0.id, value: $0.value, emphasized: $0.emphasized)
      }
    )
  }

  private var transportSummaryColumn: some View {
    summaryColumn(title: localization.text("Transport")) {
      VStack(alignment: .leading, spacing: 8) {
        EvidenceChip(label: "path", value: model.transport.title, tint: model.transport.state.color)
        if let note = model.transport.note {
          supportingLine(note)
        }
      }
    }
  }

  private var trustSummaryColumn: some View {
    summaryColumn(title: localization.text("Security")) {
      VStack(alignment: .leading, spacing: 10) {
        ForEach(model.trustHighlights) { item in
          trustHighlightRow(item)
        }
      }
    }
  }

  private var expirySummaryColumn: some View {
    summaryColumn(title: localization.text("Pairing Expires")) {
      VStack(alignment: .leading, spacing: 8) {
        Label(model.expiry.label, systemImage: "clock")
          .font(.system(size: 13, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
        Text(model.expiry.detail)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }
    }
  }

  private func summaryColumn<Content: View>(title: String, @ViewBuilder content: () -> Content) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      Text(title)
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      content()
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private func trustHighlightRow(_ item: PairingTrustHighlight) -> some View {
    HStack(alignment: .top, spacing: 10) {
      Image(systemName: item.state.symbolName)
        .font(.system(size: 15, weight: .semibold))
        .foregroundStyle(item.state.color)
        .frame(width: 18)

      VStack(alignment: .leading, spacing: 2) {
        Text(item.title)
          .font(.system(size: 13, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
        Text(item.detail)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }
    }
  }

  private var checklistPanel: some View {
    WorkbenchPanel(title: localization.text("Trust Checklist"), subtitle: localization.text("Explicit checks keep discovery separate from trust.")) {
      VStack(spacing: 0) {
        ForEach(Array(model.checklist.enumerated()), id: \.element.id) { index, item in
          checklistRow(item, index: index + 1)

          if index < model.checklist.count - 1 {
            Divider()
              .overlay(SMColor.hairline)
              .padding(.leading, 38)
          }
        }
      }
    }
  }

  private func checklistRow(_ item: PairingChecklistItem, index: Int) -> some View {
    HStack(alignment: .top, spacing: 12) {
      ZStack {
        Circle()
          .stroke(item.state.color, lineWidth: 1.5)
          .background(Circle().fill(item.state == .current ? item.state.color.opacity(0.12) : Color.clear))
        Text("\(index)")
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(item.state.color)
      }
      .frame(width: 28, height: 28)

      VStack(alignment: .leading, spacing: 4) {
        Text(item.title)
          .font(.system(size: 13, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
        Text(item.detail)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }

      Spacer(minLength: 10)

      Text(item.state.label)
        .font(.system(size: 12, weight: .medium))
        .foregroundStyle(item.state.color)
    }
    .padding(.vertical, 12)
  }

  private var codePanel: some View {
    WorkbenchPanel(title: localization.text("Pairing Pin"), subtitle: model.code.caption) {
      VStack(alignment: .leading, spacing: 12) {
        HStack(spacing: 12) {
          Text(model.code.value)
            .font(.system(size: 22, weight: .medium, design: .monospaced))
            .foregroundStyle(SMColor.primaryText)
            .padding(.horizontal, 14)
            .padding(.vertical, 12)
            .frame(minWidth: 0, maxWidth: .infinity, alignment: .leading)
            .background(SMColor.cardElevated)
            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))

          if let onRegenerateCode {
            ActionButton(localization.text("Regenerate"), systemImage: "arrow.clockwise") {
              onRegenerateCode()
            }
          }
        }

        if let helper = model.code.helperText {
          supportingLine(helper)
        }
      }
    }
  }

  private func pendingRequestPanel(_ request: PairingPendingRequestModel) -> some View {
    WorkbenchPanel(title: request.title, subtitle: request.detail) {
      VStack(alignment: .leading, spacing: 14) {
        HStack(alignment: .top, spacing: 12) {
          Image(systemName: "person.crop.circle.badge.questionmark")
            .font(.system(size: 24, weight: .semibold))
            .foregroundStyle(request.state.color)
            .frame(width: 32)

          VStack(alignment: .leading, spacing: 8) {
            pairingRequestFact(label: localization.text("Source"), value: request.sourceLabel)
            pairingRequestFact(label: localization.text("Request ID"), value: request.requestID)
            pairingRequestFact(label: localization.text("Status"), value: request.statusLabel)
          }
        }

        if request.isActionable {
          HStack(spacing: 10) {
            PrimaryActionButton(request.approveTitle, systemImage: "checkmark") {
              onApproveRequest?()
            }
            ActionButton(request.rejectTitle, systemImage: "xmark") {
              onRejectRequest?()
            }
          }
        }
      }
    }
  }

  private func pairingRequestFact(label: String, value: String) -> some View {
    HStack(alignment: .firstTextBaseline, spacing: 8) {
      Text(label)
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
        .frame(width: 86, alignment: .leading)
      Text(value)
        .font(.system(size: 12, weight: .medium))
        .foregroundStyle(SMColor.primaryText)
        .textSelection(.enabled)
        .lineLimit(2)
        .minimumScaleFactor(0.82)
    }
  }

  private var inspector: some View {
    WorkbenchPanel(title: model.inspector.title, subtitle: model.inspector.subtitle) {
      VStack(alignment: .leading, spacing: 16) {
        VStack(alignment: .leading, spacing: 14) {
          ForEach(Array(model.inspector.instructions.enumerated()), id: \.offset) { index, item in
            HStack(alignment: .top, spacing: 12) {
              ZStack {
                Circle()
                  .stroke(SMColor.blue, lineWidth: 1.5)
                Text("\(index + 1)")
                  .font(.system(size: 12, weight: .semibold))
                  .foregroundStyle(SMColor.blue)
              }
              .frame(width: 28, height: 28)

              Text(item)
                .font(.system(size: 12))
                .foregroundStyle(SMColor.secondaryText)
                .fixedSize(horizontal: false, vertical: true)
            }
          }
        }

        VStack(alignment: .leading, spacing: 10) {
          Text(localization.text("Status"))
            .font(.system(size: 12, weight: .semibold))
            .foregroundStyle(SMColor.secondaryText)

          VStack(spacing: 0) {
            ForEach(Array(model.inspector.events.enumerated()), id: \.element.id) { index, event in
              inspectorEventRow(event)
              if index < model.inspector.events.count - 1 {
                Divider()
                  .overlay(SMColor.hairline)
                  .padding(.leading, 28)
              }
            }
          }
          .panelSurface(.notice, padding: EdgeInsets(top: 4, leading: 0, bottom: 4, trailing: 0))
        }

        if let notice = model.inspector.notice {
          HStack(alignment: .top, spacing: 12) {
            Image(systemName: notice.state.symbolName)
              .font(.system(size: 16, weight: .semibold))
              .foregroundStyle(notice.state.color)
              .frame(width: 20)

            VStack(alignment: .leading, spacing: 4) {
              Text(notice.title)
                .font(.system(size: 13, weight: .semibold))
                .foregroundStyle(notice.state.color)
              Text(notice.detail)
                .font(.system(size: 12))
                .foregroundStyle(SMColor.secondaryText)
                .fixedSize(horizontal: false, vertical: true)
            }
          }
          .panelSurface(
            .notice,
            padding: EdgeInsets(top: 14, leading: 14, bottom: 14, trailing: 14)
          )
        }
      }
    }
  }

  private func inspectorEventRow(_ event: PairingInspectorEvent) -> some View {
    HStack(alignment: .top, spacing: 12) {
      Image(systemName: event.state.symbolName)
        .font(.system(size: 15, weight: .semibold))
        .foregroundStyle(event.state.color)
        .frame(width: 18)

      VStack(alignment: .leading, spacing: 3) {
        Text(event.title)
          .font(.system(size: 13, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
        if let detail = event.detail {
          Text(detail)
            .font(.system(size: 11))
            .foregroundStyle(SMColor.secondaryText)
            .fixedSize(horizontal: false, vertical: true)
        }
      }

      Spacer(minLength: 8)

      Text(event.trailingLabel)
        .font(.system(size: 12, weight: .medium))
        .foregroundStyle(event.state.color)
    }
    .padding(.horizontal, 14)
    .padding(.vertical, 12)
  }

  private var footerActions: some View {
    HStack(spacing: 12) {
      ActionButton(model.cancelTitle, systemImage: "xmark") {
        onCancel()
      }

      Spacer(minLength: 12)

      ActionButton(model.backTitle, systemImage: "chevron.left") {
        onBack()
      }

      PrimaryActionButton(model.continueTitle, systemImage: "chevron.right") {
        onContinue()
      }
    }
  }

  private func stepMarker(_ step: PairingStep) -> some View {
    ZStack {
      Circle()
        .fill(step.isCurrent ? step.state.color : SMColor.card)
      Circle()
        .stroke(step.state.color.opacity(step.isCurrent ? 1 : 0.7), lineWidth: 1.5)
      if step.state == .complete {
        Image(systemName: "checkmark")
          .font(.system(size: 11, weight: .bold))
          .foregroundStyle(step.isCurrent ? SMColor.inverseText : step.state.color)
      } else {
        Text(step.indexLabel)
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(step.isCurrent ? SMColor.inverseText : step.state.color)
      }
    }
    .frame(width: 30, height: 30)
  }

  private func statusBadge(_ badge: PairingBadge) -> some View {
    StatusBadge(
      item: .init(
        icon: badge.state.symbolName,
        label: badge.title,
        tint: badge.state.color
      ),
      prominence: .softFill
    )
  }

  private func supportingLine(_ text: String) -> some View {
    Text(text)
      .font(.system(size: 12))
      .foregroundStyle(SMColor.secondaryText)
      .fixedSize(horizontal: false, vertical: true)
  }
}

struct PairingSectionModel {
  let title: String
  let subtitle: String
  let badge: PairingBadge?
  let lastUpdatedLabel: String?
  let steps: [PairingStep]
  let summaryLine: String
  let source: PairingEndpointSummary
  let target: PairingEndpointSummary
  let transport: PairingTransportSummary
  let trustHighlights: [PairingTrustHighlight]
  let expiry: PairingExpirySummary
  let checklist: [PairingChecklistItem]
  let code: PairingCodePanelModel
  let pendingRequest: PairingPendingRequestModel?
  let inspector: PairingInspectorModel
  let cancelTitle: String
  let backTitle: String
  let continueTitle: String

  init(
    title: String = "Pairing",
    subtitle: String,
    badge: PairingBadge? = nil,
    lastUpdatedLabel: String? = nil,
    steps: [PairingStep],
    summaryLine: String,
    source: PairingEndpointSummary,
    target: PairingEndpointSummary,
    transport: PairingTransportSummary,
    trustHighlights: [PairingTrustHighlight],
    expiry: PairingExpirySummary,
    checklist: [PairingChecklistItem],
    code: PairingCodePanelModel,
    pendingRequest: PairingPendingRequestModel? = nil,
    inspector: PairingInspectorModel,
    cancelTitle: String = "Cancel",
    backTitle: String = "Back",
    continueTitle: String = "Continue"
  ) {
    self.title = title
    self.subtitle = subtitle
    self.badge = badge
    self.lastUpdatedLabel = lastUpdatedLabel
    self.steps = steps
    self.summaryLine = summaryLine
    self.source = source
    self.target = target
    self.transport = transport
    self.trustHighlights = trustHighlights
    self.expiry = expiry
    self.checklist = checklist
    self.code = code
    self.pendingRequest = pendingRequest
    self.inspector = inspector
    self.cancelTitle = cancelTitle
    self.backTitle = backTitle
    self.continueTitle = continueTitle
  }
}

struct PairingBadge {
  let title: String
  let state: PairingVisualState
}

struct PairingStep: Identifiable {
  typealias ID = String

  let id: ID
  let indexLabel: String
  let title: String
  let state: PairingStepState
  let isCurrent: Bool
}

enum PairingStepState {
  case complete
  case current
  case upcoming

  var color: Color {
    switch self {
    case .complete, .current:
      return SMColor.blue
    case .upcoming:
      return SMColor.secondaryText
    }
  }
}

struct PairingEndpointSummary {
  let name: String
  let iconName: String
  let state: PairingVisualState
  let metadata: [PairingMetadataLine]
}

struct PairingMetadataLine: Identifiable {
  let id: String
  let value: String
  let emphasized: Bool

  init(id: String, value: String, emphasized: Bool = false) {
    self.id = id
    self.value = value
    self.emphasized = emphasized
  }
}

struct PairingTransportSummary {
  let title: String
  let detail: String
  let note: String?
  let symbolName: String
  let state: PairingVisualState
}

struct PairingTrustHighlight: Identifiable {
  let id: String
  let title: String
  let detail: String
  let state: PairingVisualState
}

struct PairingExpirySummary {
  let label: String
  let detail: String
}

struct PairingChecklistItem: Identifiable {
  let id: String
  let title: String
  let detail: String
  let state: PairingChecklistState
}

enum PairingChecklistState {
  case complete
  case current
  case pending

  var color: Color {
    switch self {
    case .complete:
      return SMColor.green
    case .current:
      return SMColor.blue
    case .pending:
      return SMColor.secondaryText
    }
  }

  var label: String {
    switch self {
    case .complete:
      return "Done"
    case .current:
      return "Current"
    case .pending:
      return "Pending"
    }
  }
}

struct PairingCodePanelModel {
  let value: String
  let caption: String
  let helperText: String?
}

struct PairingPendingRequestModel {
  let title: String
  let detail: String
  let sourceLabel: String
  let requestID: String
  let statusLabel: String
  let approveTitle: String
  let rejectTitle: String
  let state: PairingVisualState
  let isActionable: Bool
}

struct PairingInspectorModel {
  let title: String
  let subtitle: String
  let instructions: [String]
  let events: [PairingInspectorEvent]
  let notice: PairingInspectorNotice?
}

struct PairingInspectorEvent: Identifiable {
  let id: String
  let title: String
  let detail: String?
  let trailingLabel: String
  let state: PairingVisualState
}

struct PairingInspectorNotice {
  let title: String
  let detail: String
  let state: PairingVisualState
}

enum PairingVisualState {
  case neutral
  case info
  case success
  case warning
  case critical

  var color: Color {
    switch self {
    case .neutral:
      return SMColor.secondaryText
    case .info:
      return SMColor.blue
    case .success:
      return SMColor.green
    case .warning:
      return SMColor.amber
    case .critical:
      return SMColor.red
    }
  }

  var symbolName: String {
    switch self {
    case .neutral:
      return "circle"
    case .info:
      return "info.circle.fill"
    case .success:
      return "checkmark.circle.fill"
    case .warning:
      return "exclamationmark.triangle.fill"
    case .critical:
      return "xmark.octagon.fill"
    }
  }
}

#if DEBUG && canImport(PreviewsMacros)
#Preview {
  PairingSectionView(
    model: PairingSectionModel(
      subtitle: "Establish a trusted pairing between source and target. Pairing pins are required before transfer.",
      badge: PairingBadge(title: "Live", state: .success),
      lastUpdatedLabel: "Last updated: 1 min ago",
      steps: [
        PairingStep(id: "select", indexLabel: "1", title: "Select devices", state: .complete, isCurrent: false),
        PairingStep(id: "trust", indexLabel: "2", title: "Trust and pins", state: .current, isCurrent: true),
        PairingStep(id: "summary", indexLabel: "3", title: "Summary", state: .upcoming, isCurrent: false),
      ],
      summaryLine: "Discovery is only a hint; explicit pins and config validation establish trust.",
      source: PairingEndpointSummary(
        name: "MacBook Pro",
        iconName: "laptopcomputer",
        state: .success,
        metadata: [
          PairingMetadataLine(id: "os", value: "macOS 14.5 (23F79)", emphasized: true),
          PairingMetadataLine(id: "ip", value: "10.0.0.12"),
        ]
      ),
      target: PairingEndpointSummary(
        name: "Studio Storage",
        iconName: "externaldrive.connected.to.line.below",
        state: .success,
        metadata: [
          PairingMetadataLine(id: "host", value: "10.0.0.20", emphasized: true),
          PairingMetadataLine(id: "path", value: "/Volumes/StudioArchive"),
        ]
      ),
      transport: PairingTransportSummary(
        title: "LAN (10 GbE)",
        detail: "Pinned ceremony over local network",
        note: "Jumbo frames permitted for the selected path.",
        symbolName: "lock.shield",
        state: .success
      ),
      trustHighlights: [
        PairingTrustHighlight(
          id: "tls",
          title: "TLS pinned",
          detail: "TLS 1.3 with certificate pinning",
          state: .success
        ),
        PairingTrustHighlight(
          id: "profile",
          title: "Config sealed",
          detail: "Migration config is sealed before transfer",
          state: .success
        ),
      ],
      expiry: PairingExpirySummary(
        label: "24 hours",
        detail: "May 29, 2025 at 2:45 PM"
      ),
      checklist: [
        PairingChecklistItem(
          id: "open-target",
          title: "Open SuperMover on the target device",
          detail: "The target operator must enter pairing mode locally.",
          state: .complete
        ),
        PairingChecklistItem(
          id: "accept",
          title: "Accept pairing request",
          detail: "Confirm the source host and exported config identity.",
          state: .current
        ),
        PairingChecklistItem(
          id: "confirm-pin",
          title: "Confirm pairing pin",
          detail: "Enter the pairing code below to bind trust for this session.",
          state: .pending
        ),
      ],
      code: PairingCodePanelModel(
        value: "8734-9912-5541",
        caption: "Enter this pin on the target to establish trust.",
        helperText: "Regenerating the pin should invalidate the prior pending request."
      ),
      inspector: PairingInspectorModel(
        title: "On target: accept pairing",
        subtitle: "Use the target device to confirm the request and verify the current pin.",
        instructions: [
          "On the target device, open SuperMover.",
          "Go to Pairing and select \"Accept Pairing Request\".",
          "Enter the pairing pin below and confirm.",
        ],
        events: [
          PairingInspectorEvent(
            id: "request",
            title: "Pairing request received",
            detail: "macbook-pro • 2:45:01 PM",
            trailingLabel: "Received",
            state: .success
          ),
          PairingInspectorEvent(
            id: "pin",
            title: "Pin verified",
            detail: "2:45:05 PM",
            trailingLabel: "Verified",
            state: .success
          ),
          PairingInspectorEvent(
            id: "profile",
            title: "Config validated",
            detail: "2:45:06 PM",
            trailingLabel: "Valid",
            state: .success
          ),
          PairingInspectorEvent(
            id: "trust",
            title: "Trust established",
            detail: "2:45:07 PM",
            trailingLabel: "Trusted",
            state: .success
          ),
        ],
        notice: PairingInspectorNotice(
          title: "Trusted connection",
          detail: "You can now continue to the summary and start the transfer.",
          state: .info
        )
      )
    )
  )
  .padding(24)
  .background(SMColor.appBackground)
}
#endif
