import SwiftUI

struct TransferSectionView: View {
  let model: TransferSectionModel
  let primaryControl: TransferSectionControl?
  let secondaryControl: TransferSectionControl?
  let supportingModel: TransferSupportingModel?
  let onSelectStage: ((TransferStageModel.ID) -> Void)?
  let onOpenTerminal: (() -> Void)?
  let onOpenEvidence: (() -> Void)?
  let onInspectorAction: (() -> Void)?

  init(
    model: TransferSectionModel,
    primaryControl: TransferSectionControl? = nil,
    secondaryControl: TransferSectionControl? = nil,
    supportingModel: TransferSupportingModel? = nil,
    onSelectStage: ((TransferStageModel.ID) -> Void)? = nil,
    onOpenTerminal: (() -> Void)? = nil,
    onOpenEvidence: (() -> Void)? = nil,
    onInspectorAction: (() -> Void)? = nil
  ) {
    self.model = model
    self.primaryControl = primaryControl
    self.secondaryControl = secondaryControl
    self.supportingModel = supportingModel
    self.onSelectStage = onSelectStage
    self.onOpenTerminal = onOpenTerminal
    self.onOpenEvidence = onOpenEvidence
    self.onInspectorAction = onInspectorAction
  }

  var body: some View {
    DetailPageHost(
      header: .init(title: model.title, subtitle: model.subtitle),
      headerAccessoryPlacement: .top,
      asideWidth: WorkbenchLayoutMetrics.transferAsideWidth,
      headerAccessory: { headerBar },
      primary: { primaryColumn },
      aside: { inspectorColumn },
      footer: { footerActions }
    )
  }

  private var headerBar: some View {
    HStack(alignment: .top, spacing: 16) {
      if let badge = model.headerBadge {
        statusBadge(badge)
      }

      Spacer(minLength: 12)

      VStack(alignment: .trailing, spacing: 10) {
        HStack(spacing: 10) {
          if let primaryControl {
            Button(action: primaryControl.action) {
              Label(primaryControl.title, systemImage: primaryControl.systemImage)
                .font(.system(size: 13, weight: .semibold))
                .padding(.horizontal, 16)
                .padding(.vertical, 11)
                .buttonSurface(primaryControl.prominence.buttonChrome)
            }
            .buttonStyle(.plain)
            .foregroundStyle(primaryControl.prominence.foregroundStyle)
          }

          if let secondaryControl {
            ActionButton(
              secondaryControl.title,
              systemImage: secondaryControl.systemImage,
              action: secondaryControl.action
            )
          }
        }

        HStack(spacing: 10) {
          if let updated = model.lastUpdatedLabel {
            Label(updated, systemImage: "clock")
              .font(.system(size: 12, weight: .medium))
              .foregroundStyle(SMColor.secondaryText)
          }

          if let note = model.headerNote {
            Text(note)
              .font(.system(size: 12))
              .foregroundStyle(SMColor.secondaryText)
          }
        }
      }
    }
  }

  private var primaryColumn: some View {
    VStack(alignment: .leading, spacing: 16) {
      routePanel
      activityPanel
      logPanel
      supportingSectionPanel
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private var inspectorColumn: some View {
    TransferInspectorView(model: model.inspector, action: onInspectorAction)
      .frame(width: 290, alignment: .top)
      .fixedSize(horizontal: false, vertical: true)
  }

  private var routePanel: some View {
    WorkbenchPanel(title: "Transfer Route", subtitle: model.route.summaryLine) {
      VStack(alignment: .leading, spacing: 18) {
        HStack(alignment: .center, spacing: 18) {
          endpointCard(model.route.source, roleLabel: "Source")

          VStack(spacing: 10) {
            Text(model.overview.progressLabel)
              .font(.system(size: 42, weight: .bold))
              .foregroundStyle(SMColor.primaryText)

            Text(model.overview.stateLabel.uppercased())
              .font(.system(size: 12, weight: .semibold))
              .foregroundStyle(model.overview.tint)

            ProgressRail(progress: model.overview.progress, tint: model.overview.tint)
              .frame(height: 6)
              .frame(maxWidth: 420)

            Text(model.overview.detail)
              .font(.system(size: 13))
              .foregroundStyle(SMColor.secondaryText)

            if !model.overview.highlights.isEmpty {
              HStack(spacing: 8) {
                ForEach(model.overview.highlights) { item in
                  EvidenceChip(label: item.label, value: item.value, tint: item.tint)
                }
              }
            }
          }
          .frame(maxWidth: .infinity)

          endpointCard(model.route.target, roleLabel: "Target")
        }

        Divider()
          .overlay(SMColor.hairline)

        LazyVGrid(columns: [GridItem(.adaptive(minimum: 130), spacing: 0)], spacing: 0) {
          ForEach(model.metrics) { metric in
            transferMetric(metric)
          }
        }
      }
    }
  }

  private func endpointCard(_ endpoint: TransferEndpointModel, roleLabel: String) -> some View {
    RouteEndpointPane(
      roleLabel: roleLabel,
      title: endpoint.name,
      address: endpoint.address,
      iconName: endpoint.symbolName,
      tint: endpoint.statusTint,
      details: endpoint.metadata.map {
        RouteEndpointPaneDetail(id: $0.id, value: $0.value, emphasized: $0.emphasized)
      }
    )
  }

  private func transferMetric(_ metric: TransferMetricModel) -> some View {
    VStack(alignment: .leading, spacing: 10) {
      Text(metric.title)
        .font(.system(size: 12, weight: .medium))
        .foregroundStyle(SMColor.secondaryText)

      Text(metric.value)
        .font(.system(size: 18, weight: .semibold))
        .foregroundStyle(metric.tint ?? SMColor.primaryText)

      if let detail = metric.detail {
        Text(detail)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
      }

      if let progress = metric.progress {
        ProgressRail(progress: progress, tint: metric.tint ?? SMColor.blue)
          .frame(height: 5)
      } else if let sparkline = metric.sparkline {
        TransferSparkline(values: sparkline, tint: metric.tint ?? SMColor.blue)
          .frame(height: 28)
      } else {
        Spacer(minLength: 5)
      }
    }
    .padding(.horizontal, 18)
    .frame(maxWidth: .infinity, minHeight: 108, alignment: .leading)
    .overlay(alignment: .trailing) {
      Rectangle()
        .fill(SMColor.hairline.opacity(0.45))
        .frame(width: 1)
        .padding(.vertical, 10)
        .opacity(metric.id == model.metrics.last?.id ? 0 : 1)
    }
  }

  private var activityPanel: some View {
    WorkbenchPanel(title: "Current Activity", subtitle: model.activity.subtitle) {
      HStack(alignment: .top, spacing: 16) {
        currentFileColumn

        Divider()
          .overlay(SMColor.hairline)

        stageColumn
      }
    }
  }

  private var currentFileColumn: some View {
    VStack(alignment: .leading, spacing: 12) {
      Text("Current file")
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)

      Text(model.activity.currentFile.path)
        .font(.system(size: 13))
        .foregroundStyle(SMColor.secondaryText)
        .lineLimit(2)

      Text(model.activity.currentFile.progressLabel)
        .font(.system(size: 13, weight: .medium))
        .foregroundStyle(SMColor.primaryText)

      ProgressRail(progress: model.activity.currentFile.progress, tint: model.activity.currentFile.tint)
        .frame(height: 6)

      HStack(spacing: 18) {
        activityMeta(label: "Started", value: model.activity.currentFile.startedAt)
        activityMeta(label: "Receipt", value: model.activity.currentFile.receiptID)
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private var stageColumn: some View {
    VStack(alignment: .leading, spacing: 16) {
      Text(model.activity.stageSummary)
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)

      HStack(alignment: .top, spacing: 0) {
        ForEach(Array(model.activity.stages.enumerated()), id: \.element.id) { index, stage in
          stageColumnNode(stage, index: index)
        }
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  @ViewBuilder
  private func stageColumnNode(_ stage: TransferStageModel, index: Int) -> some View {
    let content = VStack(alignment: .leading, spacing: 8) {
      HStack(spacing: 0) {
        if index > 0 {
          Capsule()
            .fill(stageConnectorColor(before: stage))
            .frame(height: 2)
        }

        stageMarker(stage)

        Capsule()
          .fill(stageConnectorColor(after: stage, at: index))
          .frame(height: 2)
          .opacity(index == model.activity.stages.count - 1 ? 0 : 1)
      }
      .frame(height: 24)

      Text(stage.title)
        .font(.system(size: 13, weight: stage.state == .current ? .semibold : .medium))
        .foregroundStyle(stage.state == .pending ? SMColor.secondaryText : SMColor.primaryText)

      Text(stage.timeLabel)
        .font(.system(size: 12))
        .foregroundStyle(SMColor.secondaryText)

      Text(stage.statusLabel)
        .font(.system(size: 12, weight: .medium))
        .foregroundStyle(stage.state.tint)
    }
    .frame(maxWidth: .infinity, alignment: .leading)
    .contentShape(Rectangle())

    if let onSelectStage {
      Button {
        onSelectStage(stage.id)
      } label: {
        content
      }
      .buttonStyle(.plain)
    } else {
      content
    }
  }

  private func activityMeta(label: String, value: String) -> some View {
    VStack(alignment: .leading, spacing: 4) {
      Text(label)
        .font(.system(size: 11, weight: .medium))
        .foregroundStyle(SMColor.secondaryText)
      Text(value)
        .font(.system(size: 12))
        .foregroundStyle(SMColor.primaryText)
        .lineLimit(1)
    }
  }

  private func stageMarker(_ stage: TransferStageModel) -> some View {
    ZStack {
      Circle()
        .fill(stage.state.fillColor)
        .frame(width: 18, height: 18)
      Circle()
        .stroke(stage.state.tint, lineWidth: stage.state == .current ? 3 : 2)
        .frame(width: 26, height: 26)
      if let symbol = stage.state.symbolName {
        Image(systemName: symbol)
          .font(.system(size: 11, weight: .bold))
          .foregroundStyle(stage.state.symbolTint)
      }
    }
    .frame(width: 34, height: 24)
  }

  private func stageConnectorColor(before stage: TransferStageModel) -> Color {
    switch stage.state {
    case .complete, .current:
      return stage.state.tint
    case .pending:
      return SMColor.hairline
    case .warning:
      return SMColor.amber
    }
  }

  private func stageConnectorColor(after stage: TransferStageModel, at index: Int) -> Color {
    guard index < model.activity.stages.count - 1 else { return .clear }
    switch stage.state {
    case .complete:
      return stage.state.tint
    case .current:
      return SMColor.hairline
    case .pending:
      return SMColor.hairline
    case .warning:
      return SMColor.amber
    }
  }

  private var logPanel: some View {
    WorkbenchPanel(title: "Live Log", subtitle: model.log.subtitle) {
      VStack(alignment: .leading, spacing: 12) {
        VStack(alignment: .leading, spacing: 6) {
          ForEach(model.log.entries) { entry in
            HStack(alignment: .firstTextBaseline, spacing: 12) {
              Text(entry.timestamp)
                .font(.system(size: 12, design: .monospaced))
                .foregroundStyle(SMColor.secondaryText)
                .frame(width: 64, alignment: .leading)

              Text(entry.message)
                .font(.system(size: 12, design: .monospaced))
                .foregroundStyle(entry.tint ?? SMColor.secondaryText)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
          }
        }

        if let footer = model.log.footerNote {
          HStack {
            Text(footer)
              .font(.system(size: 11))
              .foregroundStyle(SMColor.secondaryText)
            Spacer()
            StatusDot(color: SMColor.green)
          }
        }
      }
    }
  }

  @ViewBuilder
  private var supportingSectionPanel: some View {
    if let supportingModel {
      ScreenCard(
        title: "Transfer Supporting Surfaces",
        subtitle: "Advanced task wiring and network/profile inputs that are not yet folded into the primary transfer desk."
      ) {
        TransferSupportingSurfaces(model: supportingModel)
      }
    }
  }

  private var footerActions: some View {
    HStack(spacing: 10) {
      if let onOpenTerminal {
        ActionButton("Open in Terminal", systemImage: "terminal", action: onOpenTerminal)
      }

      Spacer(minLength: 12)

      if let onOpenEvidence {
        PrimaryActionButton("Review Evidence", systemImage: "doc.text.magnifyingglass", action: onOpenEvidence)
      }
    }
  }

  private func statusBadge(_ badge: TransferStatusBadge) -> some View {
    StatusBadge(
      item: .init(
        icon: badge.systemImage,
        label: badge.label,
        tint: badge.tint
      ),
      prominence: .softFill
    )
  }
}

struct TransferSupportingModel {
  struct ActionCard: Identifiable {
    let id: String
    let title: String
    let subtitle: String
    let primary: Bool
    let action: () -> Void
  }

  let gateNotice: TransferSupportingNotice?
  let actionCards: [ActionCard]
  let profileNetworkPanel: AnyView?
  let sessionID: Binding<String>?
  let listenAddress: Binding<String>?
}

struct TransferSupportingNotice {
  let title: String
  let detail: String
  let state: GateState
}

private struct TransferSupportingSurfaces: View {
  let model: TransferSupportingModel

  var body: some View {
    VStack(alignment: .leading, spacing: 16) {
      if let gateNotice = model.gateNotice {
        WorkbenchNotice(
          title: gateNotice.title,
          detail: gateNotice.detail,
          state: gateNotice.state
        )
      }

      if !model.actionCards.isEmpty {
        HStack(spacing: 16) {
          ForEach(model.actionCards) { card in
            TransferSupportingActionCard(model: card)
          }
        }
      }

      if let profileNetworkPanel = model.profileNetworkPanel {
        profileNetworkPanel
      }

      if let sessionID = model.sessionID, let listenAddress = model.listenAddress {
        HStack(spacing: 14) {
          WorkbenchFormField(
            label: "Session ID",
            text: sessionID,
            placeholder: "Required for publish / network push"
          )
          WorkbenchFormField(
            label: "Listen Address",
            text: listenAddress,
            placeholder: "Serve / dashboard / daemon bind address"
          )
        }
      }
    }
  }
}

private struct TransferSupportingActionCard: View {
  let model: TransferSupportingModel.ActionCard

  var body: some View {
    Button(action: model.action) {
      VStack(alignment: .leading, spacing: 8) {
        HStack {
          Text(model.title)
            .font(.system(size: 15, weight: .semibold))
            .foregroundStyle(model.primary ? SMColor.inverseText : SMColor.primaryText)
          Spacer()
          Image(systemName: model.primary ? "arrow.right.circle.fill" : "arrow.right.circle")
            .foregroundStyle(model.primary ? SMColor.inverseText : SMColor.blue)
        }
        Text(model.subtitle)
          .font(.system(size: 12))
          .foregroundStyle(model.primary ? SMColor.inverseText.opacity(0.8) : SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }
      .padding(16)
      .frame(maxWidth: .infinity, alignment: .leading)
      .background(model.primary ? SMColor.graphite : SMColor.card)
      .clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
      .overlay(
        RoundedRectangle(cornerRadius: 16, style: .continuous)
          .stroke(model.primary ? SMColor.graphite : SMColor.hairline)
      )
    }
    .buttonStyle(.plain)
  }
}


struct TransferSectionModel {
  var title: String
  var subtitle: String
  var headerBadge: TransferStatusBadge?
  var lastUpdatedLabel: String?
  var headerNote: String?
  var route: TransferRouteModel
  var overview: TransferOverviewModel
  var metrics: [TransferMetricModel]
  var activity: TransferActivityModel
  var log: TransferLogModel
  var inspector: TransferInspectorModel
}

struct TransferSectionControl {
  enum Prominence {
    case primary
    case secondary

    var buttonChrome: ButtonChrome {
      switch self {
      case .primary:
        return .primary
      case .secondary:
        return .secondary
      }
    }

    var foregroundStyle: Color {
      switch self {
      case .primary:
        return SMColor.card
      case .secondary:
        return SMColor.primaryText
      }
    }
  }

  var title: String
  var systemImage: String
  var prominence: Prominence
  var action: () -> Void
}

struct TransferStatusBadge {
  var label: String
  var systemImage: String
  var tint: Color
}

struct TransferRouteModel {
  var summaryLine: String
  var source: TransferEndpointModel
  var target: TransferEndpointModel
}

struct TransferEndpointModel {
  struct MetadataLine: Identifiable {
    let id: String
    var value: String
    var emphasized: Bool

    init(id: String, value: String, emphasized: Bool = false) {
      self.id = id
      self.value = value
      self.emphasized = emphasized
    }
  }

  var name: String
  var address: String
  var symbolName: String
  var statusTint: Color
  var metadata: [MetadataLine]
}

struct TransferOverviewModel {
  struct Highlight: Identifiable {
    let id: String
    var label: String
    var value: String
    var tint: Color

    init(id: String, label: String, value: String, tint: Color) {
      self.id = id
      self.label = label
      self.value = value
      self.tint = tint
    }
  }

  var progress: Double
  var progressLabel: String
  var stateLabel: String
  var detail: String
  var tint: Color
  var highlights: [Highlight]
}

struct TransferMetricModel: Identifiable {
  let id: String
  var title: String
  var value: String
  var detail: String?
  var progress: Double?
  var sparkline: [Double]?
  var tint: Color?
}

struct TransferActivityModel {
  struct CurrentFile {
    var path: String
    var progressLabel: String
    var progress: Double
    var tint: Color
    var startedAt: String
    var receiptID: String
  }

  var subtitle: String
  var currentFile: CurrentFile
  var stageSummary: String
  var stages: [TransferStageModel]
}

struct TransferStageModel: Identifiable {
  enum State {
    case complete
    case current
    case pending
    case warning

    var tint: Color {
      switch self {
      case .complete:
        return SMColor.green
      case .current:
        return SMColor.blue
      case .pending:
        return SMColor.hairline
      case .warning:
        return SMColor.amber
      }
    }

    var fillColor: Color {
      switch self {
      case .complete:
        return SMColor.green.opacity(0.14)
      case .current:
        return SMColor.blue.opacity(0.12)
      case .pending:
        return SMColor.card
      case .warning:
        return SMColor.amber.opacity(0.12)
      }
    }

    var symbolName: String? {
      switch self {
      case .complete:
        return "checkmark"
      case .current:
        return nil
      case .pending:
        return nil
      case .warning:
        return "exclamationmark"
      }
    }

    var symbolTint: Color {
      switch self {
      case .complete:
        return SMColor.green
      case .current:
        return SMColor.blue
      case .pending:
        return .clear
      case .warning:
        return SMColor.amber
      }
    }
  }

  let id: String
  var title: String
  var timeLabel: String
  var statusLabel: String
  var state: State
}

struct TransferLogModel {
  struct Entry: Identifiable {
    let id: String
    var timestamp: String
    var message: String
    var tint: Color?
  }

  var subtitle: String
  var entries: [Entry]
  var footerNote: String?
}

struct TransferInspectorModel {
  struct SummaryRow: Identifiable {
    let id: String
    var label: String
    var value: String
    var tint: Color
  }

  struct Note: Identifiable {
    enum Style {
      case good
      case neutral
      case warning

      var tint: Color {
        switch self {
        case .good:
          return SMColor.green
        case .neutral:
          return SMColor.blue
        case .warning:
          return SMColor.amber
        }
      }

      var symbolName: String {
        switch self {
        case .good:
          return "checkmark.seal"
        case .neutral:
          return "info.circle"
        case .warning:
          return "exclamationmark.triangle"
        }
      }
    }

    let id: String
    var title: String
    var detail: String
    var style: Style
  }

  var title: String
  var subtitle: String
  var actionTitle: String?
  var summaryRows: [SummaryRow]
  var notes: [Note]
}

struct TransferInspectorView: View {
  let model: TransferInspectorModel
  let action: (() -> Void)?

  var body: some View {
    WorkbenchPanel(title: model.title, subtitle: model.subtitle) {
      VStack(alignment: .leading, spacing: 16) {
        VStack(alignment: .leading, spacing: 10) {
          ForEach(model.summaryRows) { row in
            HStack(alignment: .firstTextBaseline) {
              Text(row.label)
                .font(.system(size: 12))
                .foregroundStyle(SMColor.secondaryText)
              Spacer(minLength: 12)
              Text(row.value)
                .font(.system(size: 12, weight: .semibold))
                .foregroundStyle(row.tint)
            }
          }
        }
        .panelSurface(.notice, padding: EdgeInsets(top: 12, leading: 12, bottom: 12, trailing: 12))

        VStack(alignment: .leading, spacing: 10) {
          ForEach(model.notes) { note in
            HStack(alignment: .top, spacing: 10) {
              Image(systemName: note.style.symbolName)
                .font(.system(size: 14, weight: .semibold))
                .foregroundStyle(note.style.tint)
                .frame(width: 16)

              VStack(alignment: .leading, spacing: 3) {
                Text(note.title)
                  .font(.system(size: 12, weight: .semibold))
                  .foregroundStyle(SMColor.primaryText)
                Text(note.detail)
                  .font(.system(size: 12))
                  .foregroundStyle(SMColor.secondaryText)
                  .fixedSize(horizontal: false, vertical: true)
              }
            }
          }
        }

        if let action, let actionTitle = model.actionTitle {
          ActionButton(actionTitle, systemImage: "slider.horizontal.3", action: action)
        }
      }
    }
  }
}

struct TransferSparkline: View {
  let values: [Double]
  let tint: Color

  var body: some View {
    GeometryReader { proxy in
      if values.count > 1 {
        Path { path in
          let minValue = values.min() ?? 0
          let maxValue = values.max() ?? 1
          let range = max(maxValue - minValue, 0.001)

          for (index, value) in values.enumerated() {
            let x = proxy.size.width * CGFloat(index) / CGFloat(values.count - 1)
            let normalized = (value - minValue) / range
            let y = proxy.size.height * (1 - CGFloat(normalized))

            if index == 0 {
              path.move(to: CGPoint(x: x, y: y))
            } else {
              path.addLine(to: CGPoint(x: x, y: y))
            }
          }
        }
        .stroke(tint, style: StrokeStyle(lineWidth: 1.5, lineCap: .round, lineJoin: .round))
      }
    }
  }
}
