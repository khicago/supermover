import SwiftUI

struct DevicesSectionView: View {
  let model: DevicesSectionModel
  let onRefresh: () -> Void
  let onMoreActions: (() -> Void)?
  let onSelectFilter: (DevicesFilter.ID) -> Void
  let onSearchChange: (String) -> Void
  let onSelectDevice: (DeviceCardModel.ID) -> Void
  let onInspectorAction: () -> Void

  var body: some View {
    DetailPageHost(
      header: .init(title: model.title, subtitle: model.subtitle, prominence: .hero),
      headerAccessoryPlacement: .top,
      asideWidth: WorkbenchLayoutMetrics.devicesAsideWidth,
      headerAccessory: {
        headerAccessory
      },
      primary: {
        primaryColumn
      },
      aside: {
        inspectorColumn
      },
      footer: {
        EmptyView()
      }
    )
  }

  private var headerAccessory: some View {
    VStack(alignment: .trailing, spacing: 10) {
        HStack(spacing: 10) {
          ActionButton("Refresh", systemImage: "arrow.clockwise", action: onRefresh)

          if let onMoreActions {
            IconActionButton(systemImage: "ellipsis", action: onMoreActions)
          }
        }

      HStack(spacing: 10) {
        Text(model.lastUpdatedLabel)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)

        if let status = model.headerStatus {
          StatusBadge(
            item: .init(
              icon: status.systemImage,
              label: status.label,
              tint: status.tint
            ),
            prominence: .plain
          )
        }
      }
    }
  }

  private var primaryColumn: some View {
    VStack(alignment: .leading, spacing: 16) {
      filterBar
      deviceGrid
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private var inspectorColumn: some View {
    DevicesInspectorView(model: model.inspector, action: onInspectorAction)
      .frame(width: 300, alignment: .top)
  }

  private var filterBar: some View {
    WorkbenchToolbarStrip {
      HStack(spacing: 14) {
        ScrollView(.horizontal, showsIndicators: false) {
          HStack(spacing: 10) {
            ForEach(model.filters) { filter in
              FilterChip(filter: filter) {
                onSelectFilter(filter.id)
              }
            }
          }
          .padding(.vertical, 2)
        }

        Spacer(minLength: 8)

        if !model.searchText.isEmpty || !model.searchPlaceholder.isEmpty {
          WorkbenchSearchField(
            text: model.searchText,
            placeholder: model.searchPlaceholder,
            onChange: onSearchChange
          )
          .frame(width: 248)
        }
      }
    }
  }

  private var deviceGrid: some View {
    LazyVGrid(columns: [GridItem(.adaptive(minimum: 220, maximum: 270), spacing: 14)], spacing: 14) {
      ForEach(model.devices) { device in
        DeviceCardView(model: device) {
          onSelectDevice(device.id)
        }
      }
    }
  }
}

struct DevicesSectionModel {
  var title: String
  var subtitle: String
  var lastUpdatedLabel: String
  var headerStatus: DevicesHeaderStatus?
  var searchPlaceholder: String
  var searchText: String
  var filters: [DevicesFilter]
  var devices: [DeviceCardModel]
  var inspector: DevicesInspectorModel
}

struct DevicesHeaderStatus {
  var label: String
  var systemImage: String
  var tint: Color
}

struct DevicesFilter: Identifiable, Hashable {
  let id: String
  var title: String
  var countLabel: String
  var isSelected: Bool
}

struct DeviceCardModel: Identifiable, Hashable {
  enum Kind: String, Hashable {
    case laptop
    case desktop
    case rack
    case storage

    var systemImage: String {
      switch self {
      case .laptop:
        return "laptopcomputer"
      case .desktop:
        return "macmini"
      case .rack:
        return "server.rack"
      case .storage:
        return "externaldrive.connected.to.line.below"
      }
    }
  }

  enum Presence: Hashable {
    case online
    case offline

    var tint: Color {
      switch self {
      case .online:
        return SMColor.green
      case .offline:
        return SMColor.red
      }
    }

    var label: String {
      switch self {
      case .online:
        return "Online"
      case .offline:
        return "Offline"
      }
    }
  }

  enum Trust: Hashable {
    case trusted
    case untrusted
    case review

    var tint: Color {
      switch self {
      case .trusted:
        return SMColor.green
      case .untrusted:
        return SMColor.secondaryText
      case .review:
        return SMColor.amber
      }
    }

    var label: String {
      switch self {
      case .trusted:
        return "Trusted"
      case .untrusted:
        return "Untrusted"
      case .review:
        return "Review"
      }
    }
  }

  struct Metric: Hashable {
    var label: String
    var value: String
  }

  struct StorageUsage: Hashable {
    var summary: String
    var progress: Double
    var tint: Color
  }

  let id: String
  var name: String
  var detailLine: String
  var kind: Kind
  var presence: Presence
  var trust: Trust
  var address: String
  var primaryMetric: Metric
  var secondaryMetric: Metric
  var storage: StorageUsage?
  var lastSeen: String
  var isSelected: Bool
}

struct DevicesInspectorModel {
  struct StatusSummary {
    var title: String
    var value: String
    var tint: Color
    var note: String
  }

  struct CheckItem: Identifiable, Hashable {
    enum State: Hashable {
      case pass
      case warning
      case neutral

      var tint: Color {
        switch self {
        case .pass:
          return SMColor.green
        case .warning:
          return SMColor.amber
        case .neutral:
          return SMColor.secondaryText
        }
      }

      var systemImage: String {
        switch self {
        case .pass:
          return "checkmark.circle.fill"
        case .warning:
          return "exclamationmark.circle.fill"
        case .neutral:
          return "circle.fill"
        }
      }
    }

    let id: String
    var title: String
    var value: String
    var state: State
  }

  struct PairRow: Identifiable, Hashable {
    let id: String
    var label: String
    var value: String
  }

  var title: String
  var summary: StatusSummary
  var checks: [CheckItem]
  var overviewRows: [PairRow]
  var networkRows: [PairRow]
  var actionTitle: String
}

private struct FilterChip: View {
  let filter: DevicesFilter
  let action: () -> Void

  var body: some View {
    Button(action: action) {
      HStack(spacing: 8) {
        Text(filter.title)
          .font(.system(size: 13, weight: .medium))
        Text(filter.countLabel)
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(filter.isSelected ? SMColor.primaryText : SMColor.secondaryText)
          .padding(.horizontal, 7)
          .padding(.vertical, 3)
          .background(
            Capsule(style: .continuous)
              .fill(filter.isSelected ? SMColor.blue.opacity(0.10) : SMColor.input)
          )
      }
      .foregroundStyle(filter.isSelected ? SMColor.primaryText : SMColor.primaryText)
      .padding(.horizontal, 12)
      .padding(.vertical, 8)
      .background(filter.isSelected ? SMColor.card.opacity(0.95) : Color.clear)
      .clipShape(RoundedRectangle(cornerRadius: 11, style: .continuous))
    }
    .buttonStyle(.plain)
  }
}

private struct DeviceCardView: View {
  let model: DeviceCardModel
  let action: () -> Void

  var body: some View {
    Button(action: action) {
      VStack(alignment: .leading, spacing: 14) {
        header
        deviceVisual
        statusRow
        addressLine
        Divider()
          .overlay(SMColor.hairline.opacity(0.7))
        metricsBlock
      }
      .panelSurface(
        .panel,
        minHeight: 340,
        padding: EdgeInsets(top: 16, leading: 16, bottom: 16, trailing: 16)
      )
      .selectedCardStyle(model.isSelected)
    }
    .buttonStyle(.plain)
  }

  private var header: some View {
    HStack(alignment: .top, spacing: 10) {
      VStack(alignment: .leading, spacing: 4) {
        Text(model.name)
          .font(.system(size: 14, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
          .lineLimit(1)
        Text(model.detailLine)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
          .lineLimit(1)
      }

      Spacer(minLength: 8)
    }
  }

  private var deviceVisual: some View {
    WorkbenchMediaSlot(height: 100) {
      Image(systemName: model.kind.systemImage)
        .font(.system(size: 44, weight: .light))
        .foregroundStyle(SMColor.graphite.opacity(0.92))
    }
  }

  private var statusRow: some View {
    HStack(spacing: 8) {
      HStack(spacing: 8) {
        StatusDot(color: model.presence.tint)
        Text(model.presence.label)
          .font(.system(size: 13, weight: .medium))
          .foregroundStyle(model.presence.tint)
      }

      pill(title: model.trust.label, tint: model.trust.tint)

      Spacer(minLength: 0)
    }
  }

  private var addressLine: some View {
    Text(model.address)
      .font(.system(size: 12, weight: .medium))
      .foregroundStyle(SMColor.secondaryText)
      .lineLimit(1)
  }

  private var metricsBlock: some View {
    VStack(alignment: .leading, spacing: 10) {
      metricRow(model.primaryMetric)
      metricRow(model.secondaryMetric)

      if let storage = model.storage {
        VStack(alignment: .leading, spacing: 7) {
          Text(storage.summary)
            .font(.system(size: 12, weight: .medium))
            .foregroundStyle(SMColor.primaryText)
          ProgressRail(progress: storage.progress, tint: storage.tint)
            .frame(height: 6)
        }
      }

      VStack(alignment: .leading, spacing: 3) {
        Text("Last seen")
          .font(.system(size: 11, weight: .medium))
          .foregroundStyle(SMColor.secondaryText)
        Text(model.lastSeen)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.primaryText)
      }
    }
  }

  private func metricRow(_ metric: DeviceCardModel.Metric) -> some View {
    HStack(alignment: .firstTextBaseline) {
      Text(metric.label)
        .font(.system(size: 11, weight: .medium))
        .foregroundStyle(SMColor.secondaryText)
      Spacer(minLength: 10)
      Text(metric.value)
        .font(.system(size: 12, weight: .medium))
        .foregroundStyle(SMColor.primaryText)
        .multilineTextAlignment(.trailing)
    }
  }

  private func pill(title: String, tint: Color) -> some View {
    StatusBadge(item: .init(icon: "circle.fill", label: title, tint: tint), prominence: .softFill, compact: false)
  }
}

private struct DevicesInspectorView: View {
  let model: DevicesInspectorModel
  let action: () -> Void

  var body: some View {
    VStack(alignment: .leading, spacing: 14) {
      summaryCard
      checksCard
      rowsCard(title: "Trust overview", rows: model.overviewRows, includeButton: true)
      rowsCard(title: "Network", rows: model.networkRows, includeButton: false)
    }
  }

  private var summaryCard: some View {
    VStack(alignment: .leading, spacing: 10) {
      HStack(spacing: 10) {
        Image(systemName: "shield")
          .font(.system(size: 22, weight: .medium))
          .foregroundStyle(model.summary.tint)
        Text(model.title)
          .font(.system(size: 15, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
      }

      VStack(alignment: .leading, spacing: 4) {
        Text(model.summary.value)
          .font(.system(size: 20, weight: .bold))
          .foregroundStyle(model.summary.tint)
        Text(model.summary.note)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }
    }
    .panelSurface(.panel)
  }

  private var checksCard: some View {
    VStack(alignment: .leading, spacing: 0) {
      ForEach(Array(model.checks.enumerated()), id: \.element.id) { index, item in
        inspectorCheckRow(item)
        if index < model.checks.count - 1 {
          Divider()
            .overlay(SMColor.hairline.opacity(0.7))
        }
      }
    }
    .panelSurface(.panel, padding: EdgeInsets(top: 0, leading: 0, bottom: 0, trailing: 0))
  }

  private func rowsCard(title: String, rows: [DevicesInspectorModel.PairRow], includeButton: Bool) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      Text(title)
        .font(.system(size: 14, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)

      VStack(spacing: 0) {
        ForEach(Array(rows.enumerated()), id: \.element.id) { index, row in
          HStack(alignment: .firstTextBaseline) {
            Text(row.label)
              .font(.system(size: 12))
              .foregroundStyle(SMColor.secondaryText)
            Spacer(minLength: 12)
            Text(row.value)
              .font(.system(size: 12, weight: .medium))
              .foregroundStyle(SMColor.primaryText)
              .multilineTextAlignment(.trailing)
          }
          .padding(.vertical, 8)

          if index < rows.count - 1 {
            Divider()
              .overlay(SMColor.hairline.opacity(0.7))
          }
        }
      }

      if includeButton {
        Button(action: action) {
          Text(model.actionTitle)
            .font(.system(size: 12, weight: .medium))
            .frame(maxWidth: .infinity)
            .padding(.vertical, 8)
            .buttonSurface(.secondary)
        }
        .buttonStyle(.plain)
        .foregroundStyle(SMColor.primaryText)
      }
    }
    .panelSurface(.panel)
  }

  private func inspectorCheckRow(_ item: DevicesInspectorModel.CheckItem) -> some View {
    HStack(spacing: 10) {
      Image(systemName: item.state.systemImage)
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(item.state.tint)
        .frame(width: 16)

      Text(item.title)
        .font(.system(size: 12, weight: .medium))
        .foregroundStyle(SMColor.primaryText)

      Spacer(minLength: 12)

      Text(item.value)
        .font(.system(size: 12))
        .foregroundStyle(item.state.tint)
    }
    .padding(.horizontal, 14)
    .padding(.vertical, 12)
  }
}

#Preview {
  DevicesSectionView(
    model: DevicesSectionModel(
      title: "Devices",
      subtitle: "Discover and manage endpoints. All status is evidence-backed.",
      lastUpdatedLabel: "Last updated: 1 min ago",
      headerStatus: DevicesHeaderStatus(label: "Audited", systemImage: "clock", tint: SMColor.secondaryText),
      searchPlaceholder: "Search devices",
      searchText: "",
      filters: [
        DevicesFilter(id: "all", title: "All", countLabel: "5", isSelected: true),
        DevicesFilter(id: "sources", title: "Sources", countLabel: "1", isSelected: false),
        DevicesFilter(id: "targets", title: "Targets", countLabel: "1", isSelected: false),
        DevicesFilter(id: "receivers", title: "Receivers", countLabel: "1", isSelected: false),
        DevicesFilter(id: "offline", title: "Offline", countLabel: "2", isSelected: false),
      ],
      devices: [
        DeviceCardModel(
          id: "mbp",
          name: "MacBook Pro",
          detailLine: "Source  •  macOS 14.5",
          kind: .laptop,
          presence: .online,
          trust: .trusted,
          address: "10.0.0.12",
          primaryMetric: .init(label: "Role", value: "Source"),
          secondaryMetric: .init(label: "Storage", value: "1.02 TB / 2 TB (51%)"),
          storage: .init(summary: "51% allocated", progress: 0.51, tint: SMColor.blue),
          lastSeen: "1 min ago",
          isSelected: true
        ),
        DeviceCardModel(
          id: "storage",
          name: "Studio Storage",
          detailLine: "Target  •  macOS 14.5",
          kind: .storage,
          presence: .online,
          trust: .trusted,
          address: "10.0.0.20",
          primaryMetric: .init(label: "Role", value: "Target"),
          secondaryMetric: .init(label: "Storage", value: "6.12 TB / 8 TB (76%)"),
          storage: .init(summary: "76% allocated", progress: 0.76, tint: SMColor.green),
          lastSeen: "Just now",
          isSelected: false
        ),
        DeviceCardModel(
          id: "receiver",
          name: "Archive Receiver",
          detailLine: "Receiver  •  macOS 14.5",
          kind: .rack,
          presence: .online,
          trust: .trusted,
          address: "10.0.0.30",
          primaryMetric: .init(label: "Role", value: "Receiver"),
          secondaryMetric: .init(label: "Storage", value: "12 TB / 16 TB (75%)"),
          storage: .init(summary: "75% allocated", progress: 0.75, tint: Color.purple),
          lastSeen: "2 min ago",
          isSelected: false
        ),
        DeviceCardModel(
          id: "old-laptop",
          name: "Old Laptop",
          detailLine: "macOS 12.7",
          kind: .laptop,
          presence: .offline,
          trust: .untrusted,
          address: "10.0.0.99",
          primaryMetric: .init(label: "Role", value: "Unknown"),
          secondaryMetric: .init(label: "Storage", value: "—"),
          storage: nil,
          lastSeen: "1 day ago",
          isSelected: false
        ),
      ],
      inspector: DevicesInspectorModel(
        title: "Safety posture",
        summary: .init(
          title: "Safety posture",
          value: "Strong",
          tint: SMColor.green,
          note: "Migration configs are sealed and migration safety controls are enforced."
        ),
        checks: [
          .init(id: "sealed", title: "Config sealed", value: "Sealed", state: .pass),
          .init(id: "dry-run", title: "Dry-run passed", value: "Passed", state: .pass),
          .init(id: "preflight", title: "Target preflight clean", value: "Clean", state: .pass),
          .init(id: "warnings", title: "Warnings durable", value: "Durable", state: .pass),
          .init(id: "root", title: "Root comparison", value: "Completed", state: .pass),
          .init(id: "manual", title: "Reconcile manual", value: "Manual", state: .neutral),
        ],
        overviewRows: [
          .init(id: "trusted", label: "Trusted devices", value: "3"),
          .init(id: "untrusted", label: "Untrusted devices", value: "2"),
          .init(id: "change", label: "Last trust change", value: "Today, 2:45 PM"),
        ],
        networkRows: [
          .init(id: "transport", label: "Transport", value: "LAN (10 GbE)"),
          .init(id: "discovery", label: "Discovery", value: "5 devices"),
        ],
        actionTitle: "View trust records"
      )
    ),
    onRefresh: {},
    onMoreActions: {},
    onSelectFilter: { _ in },
    onSearchChange: { _ in },
    onSelectDevice: { _ in },
    onInspectorAction: {}
  )
  .padding(24)
  .background(SMColor.appBackground)
  .frame(width: 1320, height: 860)
}
