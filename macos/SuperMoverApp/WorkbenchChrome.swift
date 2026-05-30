import SwiftUI
#if os(macOS)
import AppKit
#endif

enum WorkbenchWindowMetrics {
  static let minimumContentSize = CGSize(
    width: max(1260, WorkbenchLayoutMetrics.minimumPinnedDetailWorkbenchWidth()),
    height: 820
  )

  static func clampedContentSize(for contentSize: CGSize) -> CGSize {
    CGSize(
      width: max(contentSize.width, minimumContentSize.width),
      height: max(contentSize.height, minimumContentSize.height)
    )
  }

  static func minimumWindowFrameSize(
    for frameSize: CGSize,
    contentLayoutSize: CGSize
  ) -> CGSize? {
    guard hasUsableContentLayoutSize(contentLayoutSize) else {
      return nil
    }

    return CGSize(
      width: minimumContentSize.width + max(0, frameSize.width - contentLayoutSize.width),
      height: minimumContentSize.height + max(0, frameSize.height - contentLayoutSize.height)
    )
  }

  static func clampedWindowFrameSize(
    for frameSize: CGSize,
    contentLayoutSize: CGSize
  ) -> CGSize? {
    guard let minimumFrameSize = minimumWindowFrameSize(
      for: frameSize,
      contentLayoutSize: contentLayoutSize
    ) else {
      return nil
    }

    return CGSize(
      width: max(frameSize.width, minimumFrameSize.width),
      height: max(frameSize.height, minimumFrameSize.height)
    )
  }

  static func zoomedFrame(
    for visibleFrame: CGRect,
    minimumWindowFrameSize: CGSize
  ) -> CGRect {
    let resolvedWidth = max(visibleFrame.width, minimumWindowFrameSize.width)
    let resolvedHeight = max(visibleFrame.height, minimumWindowFrameSize.height)
    return CGRect(
      x: visibleFrame.minX,
      y: visibleFrame.maxY - resolvedHeight,
      width: resolvedWidth,
      height: resolvedHeight
    )
  }

  static func hasUsableContentLayoutSize(_ size: CGSize) -> Bool {
    size.width.isFinite && size.height.isFinite && size.width > 0 && size.height > 0
  }
}

enum WorkbenchLayoutMetrics {
  static let sidebarWidth: CGFloat = 250
  static let sidebarDividerWidth: CGFloat = 1
  static let mainContentHorizontalPadding: CGFloat = 30
  static let mainContentTopPadding: CGFloat = 18
  static let mainContentVerticalPadding: CGFloat = 26
  static let fixedOwnerModeStripBottomPadding: CGFloat = 12
  static let fixedOwnerModeStripBodyGap: CGFloat = 8
  static let mainContentScrollSpace = "workbench.mainContentScroll"
  static let detailPageMinimumPrimaryWidth: CGFloat = 620
  static let detailPageSpacing: CGFloat = 18
  static let devicesAsideWidth: CGFloat = 300
  static let pairingAsideWidth: CGFloat = 330
  static let transferAsideWidth: CGFloat = 290
  static let evidenceAsideWidth: CGFloat = 208

  static var maximumPinnedAsideWidth: CGFloat {
    max(devicesAsideWidth, pairingAsideWidth, transferAsideWidth, evidenceAsideWidth)
  }

  static func minimumPinnedDetailWorkbenchWidth(
    asideWidth: CGFloat = maximumPinnedAsideWidth
  ) -> CGFloat {
    ceil(
      sidebarWidth
        + sidebarDividerWidth
        + (mainContentHorizontalPadding * 2)
        + detailPageMinimumPrimaryWidth
        + detailPageSpacing
        + asideWidth
    )
  }

  static func wrappedRowMetrics(
    for sizes: [CGSize],
    maxWidth: CGFloat,
    itemSpacing: CGFloat
  ) -> [WorkbenchWrappedRowMetrics] {
    guard !sizes.isEmpty else {
      return []
    }

    let resolvedMaxWidth =
      maxWidth.isFinite && maxWidth > 0 ? maxWidth : CGFloat.greatestFiniteMagnitude
    var rows: [WorkbenchWrappedRowMetrics] = []
    var currentIndices: [Int] = []
    var currentWidth: CGFloat = 0
    var currentHeight: CGFloat = 0

    func flushCurrentRow() {
      guard !currentIndices.isEmpty else {
        return
      }
      rows.append(
        WorkbenchWrappedRowMetrics(
          indices: currentIndices,
          width: currentWidth,
          height: currentHeight
        )
      )
      currentIndices.removeAll(keepingCapacity: true)
      currentWidth = 0
      currentHeight = 0
    }

    for (index, size) in sizes.enumerated() {
      let itemWidth = size.width
      let proposedWidth =
        currentIndices.isEmpty ? itemWidth : currentWidth + itemSpacing + itemWidth

      if !currentIndices.isEmpty && proposedWidth > resolvedMaxWidth {
        flushCurrentRow()
      }

      if currentIndices.isEmpty {
        currentIndices = [index]
        currentWidth = itemWidth
        currentHeight = size.height
      } else {
        currentIndices.append(index)
        currentWidth += itemSpacing + itemWidth
        currentHeight = max(currentHeight, size.height)
      }
    }

    flushCurrentRow()
    return rows
  }
}

struct WorkbenchWrappedRowMetrics: Equatable {
  let indices: [Int]
  let width: CGFloat
  let height: CGFloat
}

struct ScreenCard<Content: View>: View {
  let title: String
  let subtitle: String
  @ViewBuilder var content: Content

  var body: some View {
    VStack(alignment: .leading, spacing: 18) {
      PanelHeader(title: title, subtitle: subtitle, titleSize: 19, subtitleSize: 12, spacing: 5)
      content
    }
    .panelSurface(.sectionCard)
  }
}

struct WorkbenchPanel<Content: View>: View {
  let title: String
  let subtitle: String
  @ViewBuilder var content: Content

  var body: some View {
    VStack(alignment: .leading, spacing: 14) {
      PanelHeader(title: title, subtitle: subtitle, titleSize: 17, subtitleSize: 11, spacing: 4)
      content
    }
    .panelSurface(.panel)
  }
}

enum StatusBadgeProminence {
  case plain
  case softFill
}

struct StatusBadgeItem {
  let icon: String
  let label: String
  let tint: Color
}

struct StatusBadge: View {
  let item: StatusBadgeItem
  var prominence: StatusBadgeProminence = .softFill
  var compact: Bool = false

  var body: some View {
    HStack(spacing: compact ? 6 : 8) {
      Image(systemName: item.icon)
        .font(.system(size: compact ? 11 : 12, weight: .semibold))
        .foregroundStyle(item.tint)
      Text(item.label)
        .font(.system(size: compact ? 11 : 12, weight: .medium))
        .foregroundStyle(prominence == .plain ? item.tint : SMColor.primaryText)
        .lineLimit(1)
    }
    .padding(.horizontal, compact ? 0 : 9)
    .padding(.vertical, compact ? 0 : 6)
    .background(
      prominence == .plain ? Color.clear : item.tint.opacity(0.08)
    )
    .clipShape(RoundedRectangle(cornerRadius: compact ? 0 : 9, style: .continuous))
  }
}

struct WorkbenchSearchField: View {
  let text: String
  let placeholder: String
  let onChange: (String) -> Void

  private var textBinding: Binding<String> {
    Binding(
      get: { text },
      set: { newValue in
        onChange(newValue)
      }
    )
  }

  var body: some View {
    HStack(spacing: 8) {
      Image(systemName: "magnifyingglass")
        .font(.system(size: 13, weight: .medium))
        .foregroundStyle(SMColor.secondaryText)
      TextField(placeholder, text: textBinding)
        .textFieldStyle(.plain)
        .font(.system(size: 13))
        .foregroundStyle(SMColor.primaryText)
    }
    .padding(.horizontal, 12)
    .padding(.vertical, 8)
    .background(SMColor.cardElevated)
    .clipShape(RoundedRectangle(cornerRadius: 9, style: .continuous))
  }
}

struct WorkbenchToolbarStrip<Content: View>: View {
  var padding: EdgeInsets = EdgeInsets(top: 12, leading: 14, bottom: 12, trailing: 14)
  @ViewBuilder let content: Content

  init(
    padding: EdgeInsets = EdgeInsets(top: 12, leading: 14, bottom: 12, trailing: 14),
    @ViewBuilder content: () -> Content
  ) {
    self.padding = padding
    self.content = content()
  }

  var body: some View {
    content
      .panelSurface(.toolbarStrip, padding: padding)
  }
}

struct WorkbenchResponsiveBar<Leading: View, Trailing: View>: View {
  let alignment: VerticalAlignment
  let spacing: CGFloat
  let compactSpacing: CGFloat
  @ViewBuilder let leading: Leading
  @ViewBuilder let trailing: Trailing

  init(
    alignment: VerticalAlignment = .center,
    spacing: CGFloat = 16,
    compactSpacing: CGFloat = 12,
    @ViewBuilder leading: () -> Leading,
    @ViewBuilder trailing: () -> Trailing
  ) {
    self.alignment = alignment
    self.spacing = spacing
    self.compactSpacing = compactSpacing
    self.leading = leading()
    self.trailing = trailing()
  }

  var body: some View {
    ViewThatFits(in: .horizontal) {
      HStack(alignment: alignment, spacing: spacing) {
        leading
          .frame(maxWidth: .infinity, alignment: .leading)

        trailing
      }

      VStack(alignment: .leading, spacing: compactSpacing) {
        leading
          .frame(maxWidth: .infinity, alignment: .leading)

        HStack(spacing: 0) {
          Spacer(minLength: 0)
          trailing
        }
      }
    }
  }
}

enum WorkbenchWrapAlignment {
  case leading
  case trailing
}

struct WorkbenchWrappingLayout: Layout {
  var alignment: WorkbenchWrapAlignment = .leading
  var itemSpacing: CGFloat = 12
  var rowSpacing: CGFloat = 12

  struct Cache {
    var sizes: [CGSize] = []
    var proposalWidth: CGFloat?
    var rows: [WorkbenchWrappedRowMetrics] = []
    var totalSize: CGSize = .zero
  }

  func makeCache(subviews: Subviews) -> Cache {
    Cache()
  }

  func updateCache(_ cache: inout Cache, subviews: Subviews) {
    cache.sizes = subviews.map { $0.sizeThatFits(.unspecified) }
    cache.proposalWidth = nil
    cache.rows = []
    cache.totalSize = .zero
  }

  func sizeThatFits(
    proposal: ProposedViewSize,
    subviews: Subviews,
    cache: inout Cache
  ) -> CGSize {
    let sizes = cachedSizes(for: subviews, cache: &cache)
    let rows = wrappedRows(
      for: sizes,
      maxWidth: proposal.width,
      cache: &cache
    )

    let width = rows.map(\.width).max() ?? 0
    let height =
      rows.reduce(CGFloat(0)) { partial, row in
        partial + row.height
      } + max(0, CGFloat(rows.count - 1)) * rowSpacing
    let resolvedSize = CGSize(width: width, height: height)
    cache.totalSize = resolvedSize
    return resolvedSize
  }

  func placeSubviews(
    in bounds: CGRect,
    proposal: ProposedViewSize,
    subviews: Subviews,
    cache: inout Cache
  ) {
    let sizes = cachedSizes(for: subviews, cache: &cache)
    let rows = wrappedRows(
      for: sizes,
      maxWidth: bounds.width,
      cache: &cache
    )

    var y = bounds.minY
    for row in rows {
      let rowX: CGFloat
      switch alignment {
      case .leading:
        rowX = bounds.minX
      case .trailing:
        rowX = row.width < bounds.width ? bounds.maxX - row.width : bounds.minX
      }

      var x = rowX
      for index in row.indices {
        let size = sizes[index]
        let origin = CGPoint(
          x: x,
          y: y + (row.height - size.height) / 2
        )
        subviews[index].place(
          at: origin,
          anchor: .topLeading,
          proposal: ProposedViewSize(size)
        )
        x += size.width + itemSpacing
      }
      y += row.height + rowSpacing
    }
  }

  private func cachedSizes(
    for subviews: Subviews,
    cache: inout Cache
  ) -> [CGSize] {
    if cache.sizes.count != subviews.count {
      cache.sizes = subviews.map { $0.sizeThatFits(.unspecified) }
    }
    return cache.sizes
  }

  private func wrappedRows(
    for sizes: [CGSize],
    maxWidth: CGFloat?,
    cache: inout Cache
  ) -> [WorkbenchWrappedRowMetrics] {
    if cache.rows.isEmpty || cache.proposalWidth != maxWidth {
      cache.proposalWidth = maxWidth
      cache.rows = WorkbenchLayoutMetrics.wrappedRowMetrics(
        for: sizes,
        maxWidth: maxWidth ?? .greatestFiniteMagnitude,
        itemSpacing: itemSpacing
      )
    }
    return cache.rows
  }
}

struct WorkbenchWrappingRow<Content: View>: View {
  var alignment: WorkbenchWrapAlignment = .leading
  var spacing: CGFloat = 12
  var rowSpacing: CGFloat = 12
  @ViewBuilder let content: Content

  init(
    alignment: WorkbenchWrapAlignment = .leading,
    spacing: CGFloat = 12,
    rowSpacing: CGFloat = 12,
    @ViewBuilder content: () -> Content
  ) {
    self.alignment = alignment
    self.spacing = spacing
    self.rowSpacing = rowSpacing
    self.content = content()
  }

  var body: some View {
    WorkbenchWrappingLayout(
      alignment: alignment,
      itemSpacing: spacing,
      rowSpacing: rowSpacing
    ) {
      content
    }
    .frame(
      maxWidth: .infinity,
      alignment: alignment == .trailing ? .trailing : .leading
    )
  }
}

struct WorkbenchNotice: View {
  let title: String
  let detail: String
  let state: GateState

  var body: some View {
    HStack(alignment: .top, spacing: 10) {
      StatusDot(color: state.color)
      VStack(alignment: .leading, spacing: 4) {
        Text(title)
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
        Text(detail)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }
    }
    .panelSurface(.notice)
  }
}

struct WorkbenchFormField: View {
  let label: String
  let text: Binding<String>
  let placeholder: String

  var body: some View {
    VStack(alignment: .leading, spacing: 8) {
      Text(label)
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      TextField(placeholder, text: text)
        .textFieldStyle(.plain)
        .font(.system(size: 13))
        .padding(10)
        .background(SMColor.input)
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }
}

struct WorkbenchPageHeaderBadgeModel {
  let title: String
  let tint: Color
}

struct WorkbenchPageHeaderModel {
  enum Prominence {
    case standard
    case hero
  }

  let eyebrow: String?
  let title: String
  let subtitle: String
  var prominence: Prominence = .standard
  var badge: WorkbenchPageHeaderBadgeModel? = nil

  init(
    eyebrow: String? = nil,
    title: String,
    subtitle: String,
    prominence: Prominence = .standard,
    badge: WorkbenchPageHeaderBadgeModel? = nil
  ) {
    self.eyebrow = eyebrow
    self.title = title
    self.subtitle = subtitle
    self.prominence = prominence
    self.badge = badge
  }
}

struct WorkbenchPageHeaderBadge: View {
  let model: WorkbenchPageHeaderBadgeModel

  var body: some View {
    Text(model.title)
      .font(.system(size: 11, weight: .semibold))
      .foregroundStyle(model.tint)
      .lineLimit(1)
      .padding(.horizontal, 10)
      .padding(.vertical, 5)
      .background(
        RoundedRectangle(cornerRadius: 8, style: .continuous)
          .fill(model.tint.opacity(0.08))
      )
  }
}

struct WorkbenchPageHeaderValuePill: View {
  let icon: String
  let label: String
  let value: String
  let tint: Color

  var body: some View {
    HStack(spacing: 8) {
      Image(systemName: icon)
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(tint)
      Text(label)
        .font(.system(size: 11, weight: .medium))
        .foregroundStyle(SMColor.secondaryText)
      Text(value)
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)
        .lineLimit(1)
    }
    .fixedSize(horizontal: true, vertical: false)
    .padding(.horizontal, 11)
    .padding(.vertical, 7)
    .background(
      RoundedRectangle(cornerRadius: 8, style: .continuous)
        .fill(tint.opacity(0.08))
    )
  }
}

struct WorkbenchPageHeaderStatusItem: View {
  let icon: String
  let label: String
  let value: String
  let tint: Color

  var body: some View {
    HStack(spacing: 10) {
      Image(systemName: icon)
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(tint)

      VStack(alignment: .leading, spacing: 2) {
        Text(label.uppercased())
          .font(.system(size: 9, weight: .semibold))
          .foregroundStyle(SMColor.secondaryText)
        Text(value)
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
          .lineLimit(1)
      }
    }
    .fixedSize(horizontal: true, vertical: false)
    .padding(.horizontal, 11)
    .padding(.vertical, 8)
    .background(
      RoundedRectangle(cornerRadius: 8, style: .continuous)
        .fill(tint.opacity(0.08))
    )
  }
}

struct WorkbenchPageHeader: View {
  let model: WorkbenchPageHeaderModel

  private var titleSize: CGFloat {
    model.prominence == .hero ? 28 : 22
  }

  private var subtitleSize: CGFloat {
    model.prominence == .hero ? 13 : 12
  }

  private var textSpacing: CGFloat {
    model.prominence == .hero ? 5 : 4
  }

  var body: some View {
    VStack(alignment: .leading, spacing: 6) {
      if model.eyebrow != nil || model.badge != nil {
        HStack(alignment: .center, spacing: 10) {
          if let eyebrow = model.eyebrow {
            Text(eyebrow)
              .font(.system(size: 12, weight: .semibold))
              .foregroundStyle(SMColor.secondaryText)
          }

          if let badge = model.badge {
            WorkbenchPageHeaderBadge(model: badge)
          }
        }
      }

      PanelHeader(
        title: model.title,
        subtitle: model.subtitle,
        titleSize: titleSize,
        subtitleSize: subtitleSize,
        spacing: textSpacing
      )
    }
  }
}

struct WorkbenchHeaderBar<Leading: View, Accessory: View>: View {
  let accessoryPlacement: VerticalAlignment
  @ViewBuilder let leading: Leading
  @ViewBuilder let accessory: Accessory

  init(
    accessoryPlacement: VerticalAlignment = .center,
    @ViewBuilder leading: () -> Leading,
    @ViewBuilder accessory: () -> Accessory
  ) {
    self.accessoryPlacement = accessoryPlacement
    self.leading = leading()
    self.accessory = accessory()
  }

  var body: some View {
    WorkbenchResponsiveBar(alignment: accessoryPlacement) {
      leading
    } trailing: {
      accessory
    }
    .panelSurface(.toolbarStrip)
  }
}

extension WorkbenchHeaderBar where Leading == WorkbenchPageHeader {
  init(
    pageHeader: WorkbenchPageHeaderModel,
    accessoryPlacement: VerticalAlignment = .center,
    @ViewBuilder accessory: () -> Accessory
  ) {
    self.init(
      accessoryPlacement: accessoryPlacement,
      leading: {
        WorkbenchPageHeader(model: pageHeader)
      },
      accessory: accessory
    )
  }
}

struct DetailPageHost<HeaderAccessory: View, Primary: View, Aside: View, Footer: View>: View {
  let header: WorkbenchPageHeaderModel
  let headerAccessoryPlacement: VerticalAlignment
  let asideWidth: CGFloat?
  let asideLeading: Bool
  let spacing: CGFloat
  @ViewBuilder let headerAccessory: HeaderAccessory
  @ViewBuilder let primary: Primary
  @ViewBuilder let aside: Aside
  @ViewBuilder let footer: Footer

  init(
    header: WorkbenchPageHeaderModel,
    headerAccessoryPlacement: VerticalAlignment = .center,
    asideWidth: CGFloat? = nil,
    asideLeading: Bool = false,
    spacing: CGFloat = WorkbenchLayoutMetrics.detailPageSpacing,
    @ViewBuilder headerAccessory: () -> HeaderAccessory,
    @ViewBuilder primary: () -> Primary,
    @ViewBuilder aside: () -> Aside,
    @ViewBuilder footer: () -> Footer
  ) {
    self.header = header
    self.headerAccessoryPlacement = headerAccessoryPlacement
    self.asideWidth = asideWidth
    self.asideLeading = asideLeading
    self.spacing = spacing
    self.headerAccessory = headerAccessory()
    self.primary = primary()
    self.aside = aside()
    self.footer = footer()
  }

  var body: some View {
    LazyVStack(alignment: .leading, spacing: 0, pinnedViews: [.sectionHeaders]) {
      Section {
        bodyContent
          .padding(.top, spacing)
      } header: {
        headerBar
          .background(SMColor.appBackground)
          .zIndex(1)
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private var headerBar: some View {
    WorkbenchHeaderBar(
      pageHeader: header,
      accessoryPlacement: headerAccessoryPlacement
    ) {
      headerAccessory
    }
  }

  @ViewBuilder
  private var bodyContent: some View {
    VStack(alignment: .leading, spacing: spacing) {
      contentRow
      footer
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  @ViewBuilder
  private var contentRow: some View {
    if let asideWidth {
      ViewThatFits(in: .horizontal) {
        HStack(alignment: .top, spacing: spacing) {
          if asideLeading {
            aside
              .frame(width: asideWidth, alignment: .top)
            primary
              .frame(
                minWidth: WorkbenchLayoutMetrics.detailPageMinimumPrimaryWidth,
                maxWidth: .infinity,
                alignment: .leading
              )
          } else {
            primary
              .frame(
                minWidth: WorkbenchLayoutMetrics.detailPageMinimumPrimaryWidth,
                maxWidth: .infinity,
                alignment: .leading
              )
            aside
              .frame(width: asideWidth, alignment: .top)
          }
        }

        VStack(alignment: .leading, spacing: spacing) {
          if asideLeading {
            aside
              .frame(maxWidth: .infinity, alignment: .leading)
            primary
              .frame(maxWidth: .infinity, alignment: .leading)
          } else {
            primary
              .frame(maxWidth: .infinity, alignment: .leading)
            aside
              .frame(maxWidth: .infinity, alignment: .leading)
          }
        }
      }
    } else {
      primary
    }
  }
}

extension DetailPageHost where HeaderAccessory == EmptyView, Aside == EmptyView, Footer == EmptyView {
  init(
    header: WorkbenchPageHeaderModel,
    spacing: CGFloat = WorkbenchLayoutMetrics.detailPageSpacing,
    @ViewBuilder primary: () -> Primary
  ) {
    self.init(
      header: header,
      spacing: spacing,
      headerAccessory: { EmptyView() },
      primary: primary,
      aside: { EmptyView() },
      footer: { EmptyView() }
    )
  }
}

extension DetailPageHost where Aside == EmptyView, Footer == EmptyView {
  init(
    header: WorkbenchPageHeaderModel,
    headerAccessoryPlacement: VerticalAlignment = .center,
    spacing: CGFloat = WorkbenchLayoutMetrics.detailPageSpacing,
    @ViewBuilder headerAccessory: () -> HeaderAccessory,
    @ViewBuilder primary: () -> Primary
  ) {
    self.init(
      header: header,
      headerAccessoryPlacement: headerAccessoryPlacement,
      spacing: spacing,
      headerAccessory: headerAccessory,
      primary: primary,
      aside: { EmptyView() },
      footer: { EmptyView() }
    )
  }
}

extension DetailPageHost where Aside == EmptyView {
  init(
    header: WorkbenchPageHeaderModel,
    headerAccessoryPlacement: VerticalAlignment = .center,
    spacing: CGFloat = 18,
    @ViewBuilder headerAccessory: () -> HeaderAccessory,
    @ViewBuilder primary: () -> Primary,
    @ViewBuilder footer: () -> Footer
  ) {
    self.init(
      header: header,
      headerAccessoryPlacement: headerAccessoryPlacement,
      spacing: spacing,
      headerAccessory: headerAccessory,
      primary: primary,
      aside: { EmptyView() },
      footer: footer
    )
  }
}

extension DetailPageHost where HeaderAccessory == EmptyView, Footer == EmptyView {
  init(
    header: WorkbenchPageHeaderModel,
    asideWidth: CGFloat,
    asideLeading: Bool = false,
    spacing: CGFloat = 18,
    @ViewBuilder primary: () -> Primary,
    @ViewBuilder aside: () -> Aside
  ) {
    self.init(
      header: header,
      asideWidth: asideWidth,
      asideLeading: asideLeading,
      spacing: spacing,
      headerAccessory: { EmptyView() },
      primary: primary,
      aside: aside,
      footer: { EmptyView() }
    )
  }
}

extension DetailPageHost where Footer == EmptyView {
  init(
    header: WorkbenchPageHeaderModel,
    headerAccessoryPlacement: VerticalAlignment = .center,
    asideWidth: CGFloat,
    asideLeading: Bool = false,
    spacing: CGFloat = WorkbenchLayoutMetrics.detailPageSpacing,
    @ViewBuilder headerAccessory: () -> HeaderAccessory,
    @ViewBuilder primary: () -> Primary,
    @ViewBuilder aside: () -> Aside
  ) {
    self.init(
      header: header,
      headerAccessoryPlacement: headerAccessoryPlacement,
      asideWidth: asideWidth,
      asideLeading: asideLeading,
      spacing: spacing,
      headerAccessory: headerAccessory,
      primary: primary,
      aside: aside,
      footer: { EmptyView() }
    )
  }
}

extension DetailPageHost where HeaderAccessory == EmptyView, Aside == EmptyView {
  init(
    header: WorkbenchPageHeaderModel,
    spacing: CGFloat = 18,
    @ViewBuilder primary: () -> Primary,
    @ViewBuilder footer: () -> Footer
  ) {
    self.init(
      header: header,
      spacing: spacing,
      headerAccessory: { EmptyView() },
      primary: primary,
      aside: { EmptyView() },
      footer: footer
    )
  }
}

extension DetailPageHost where HeaderAccessory == EmptyView {
  init(
    header: WorkbenchPageHeaderModel,
    asideWidth: CGFloat,
    asideLeading: Bool = false,
    spacing: CGFloat = WorkbenchLayoutMetrics.detailPageSpacing,
    @ViewBuilder primary: () -> Primary,
    @ViewBuilder aside: () -> Aside,
    @ViewBuilder footer: () -> Footer
  ) {
    self.init(
      header: header,
      asideWidth: asideWidth,
      asideLeading: asideLeading,
      spacing: spacing,
      headerAccessory: { EmptyView() },
      primary: primary,
      aside: aside,
      footer: footer
    )
  }
}

struct WorkbenchMediaSlot<Content: View>: View {
  var height: CGFloat
  @ViewBuilder let content: Content

  init(height: CGFloat, @ViewBuilder content: () -> Content) {
    self.height = height
    self.content = content()
  }

  var body: some View {
    ZStack {
      RoundedRectangle(cornerRadius: 12, style: .continuous)
        .fill(SMColor.input.opacity(0.82))
      content
    }
    .frame(height: height)
  }
}

struct RouteEndpointPaneDetail: Identifiable, Hashable {
  let id: String
  var value: String
  var emphasized: Bool = false
}

struct RouteEndpointPane: View {
  let roleLabel: String
  let title: String
  let address: String?
  let iconName: String
  let tint: Color
  let details: [RouteEndpointPaneDetail]

  var body: some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 8) {
        StatusDot(color: tint)
        Text(roleLabel.uppercased())
          .font(.system(size: 10, weight: .semibold))
          .foregroundStyle(SMColor.tertiaryText)
      }

      Text(title)
        .font(.system(size: 17, weight: .bold))
        .foregroundStyle(SMColor.primaryText)
        .lineLimit(2)

      if let address, !address.isEmpty {
        Text(address)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
      }

      WorkbenchMediaSlot(height: 110) {
        Image(systemName: iconName)
          .font(.system(size: 42, weight: .light))
          .foregroundStyle(tint)
      }

      VStack(alignment: .leading, spacing: 4) {
        ForEach(details) { item in
          Text(item.value)
            .font(.system(size: item.emphasized ? 12 : 11, weight: item.emphasized ? .medium : .regular))
            .foregroundStyle(item.emphasized ? SMColor.primaryText : SMColor.secondaryText)
            .lineLimit(2)
        }
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }
}

struct ActionButton: View {
  let title: String
  let systemImage: String
  let action: () -> Void

  init(_ title: String, systemImage: String, action: @escaping () -> Void) {
    self.title = title
    self.systemImage = systemImage
    self.action = action
  }

  var body: some View {
    Button(action: action) {
      Label(title, systemImage: systemImage)
        .font(.system(size: 13, weight: .semibold))
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
        .buttonSurface(.secondary)
    }
    .buttonStyle(.plain)
    .foregroundStyle(SMColor.primaryText)
  }
}

struct IconActionButton: View {
  let systemImage: String
  let action: () -> Void
  var size: CGFloat = 36

  var body: some View {
    Button(action: action) {
      Image(systemName: systemImage)
        .font(.system(size: 14, weight: .semibold))
        .frame(width: size, height: size)
        .buttonSurface(.secondary)
    }
    .buttonStyle(.plain)
    .foregroundStyle(SMColor.primaryText)
  }
}

struct CompactActionButton: View {
  let title: String
  let systemImage: String
  let action: () -> Void
  var size: CGFloat = 34

  init(_ title: String, systemImage: String, action: @escaping () -> Void) {
    self.title = title
    self.systemImage = systemImage
    self.action = action
  }

  var body: some View {
    Button(action: action) {
      Image(systemName: systemImage)
        .font(.system(size: 13, weight: .semibold))
        .frame(width: size, height: size)
        .background(SMColor.cardElevated)
        .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 8, style: .continuous).stroke(SMColor.hairline))
    }
    .buttonStyle(.plain)
    .foregroundStyle(SMColor.primaryText)
    .help(title)
    .accessibilityLabel(Text(title))
  }
}

struct PrimaryActionButton: View {
  let title: String
  let systemImage: String
  let isEnabled: Bool
  let action: () -> Void

  init(_ title: String, systemImage: String, isEnabled: Bool = true, action: @escaping () -> Void) {
    self.title = title
    self.systemImage = systemImage
    self.isEnabled = isEnabled
    self.action = action
  }

  var body: some View {
    Button(action: action) {
      Label(title, systemImage: systemImage)
        .font(.system(size: 13, weight: .semibold))
        .padding(.horizontal, 16)
        .padding(.vertical, 11)
        .buttonSurface(isEnabled ? .primary : .secondary)
    }
    .buttonStyle(.plain)
    .foregroundStyle(isEnabled ? SMColor.inverseText : SMColor.secondaryText)
    .disabled(!isEnabled)
  }
}

struct EvidenceChip: View {
  let label: String
  let value: String
  let tint: Color

  var body: some View {
    HStack(spacing: 7) {
      StatusDot(color: tint)
      Text(label)
        .font(.system(size: 11, weight: .medium))
        .foregroundStyle(SMColor.secondaryText)
        .lineLimit(1)
      Text(value)
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)
        .lineLimit(1)
    }
    .fixedSize(horizontal: true, vertical: false)
    .padding(.horizontal, 10)
    .padding(.vertical, 7)
    .background(tint.opacity(0.08))
    .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
  }
}

struct StatusDot: View {
  let color: Color

  var body: some View {
    Circle()
      .fill(color)
      .frame(width: 8, height: 8)
  }
}

struct ProgressRail: View {
  let progress: Double
  let tint: Color

  var body: some View {
    GeometryReader { proxy in
      ZStack(alignment: .leading) {
        Capsule()
          .fill(SMColor.input)
        Capsule()
          .fill(tint)
          .frame(width: max(0, min(proxy.size.width, proxy.size.width * progress)))
      }
    }
    .frame(height: 6)
  }
}

struct TransferLine: View {
  var body: some View {
    ZStack {
      Capsule()
        .fill(SMColor.hairline)
        .frame(height: 2)
      Capsule()
        .fill(
          LinearGradient(
            colors: [SMColor.cyan.opacity(0.2), SMColor.blue, SMColor.green.opacity(0.7)],
            startPoint: .leading,
            endPoint: .trailing
          )
        )
        .frame(height: 3)
        .shadow(color: SMColor.cyan.opacity(0.3), radius: 7, x: 0, y: 0)
    }
  }
}

struct PanelHeader: View {
  let title: String
  let subtitle: String
  let titleSize: CGFloat
  let subtitleSize: CGFloat
  let spacing: CGFloat

  var body: some View {
    VStack(alignment: .leading, spacing: spacing) {
      Text(title)
        .font(.system(size: titleSize, weight: .bold))
        .foregroundStyle(SMColor.primaryText)
      Text(subtitle)
        .font(.system(size: subtitleSize))
        .foregroundStyle(SMColor.secondaryText)
        .fixedSize(horizontal: false, vertical: true)
    }
  }
}

enum PanelChrome {
  case sidebarCard
  case sectionCard
  case panel
  case runway
  case topBar
  case toolbarStrip
  case statusStrip
  case notice
  case metricTile
  case gateRow

  var padding: EdgeInsets {
    switch self {
    case .sidebarCard:
      return EdgeInsets(top: 14, leading: 14, bottom: 14, trailing: 14)
    case .sectionCard:
      return EdgeInsets(top: 20, leading: 20, bottom: 20, trailing: 20)
    case .panel:
      return EdgeInsets(top: 18, leading: 18, bottom: 18, trailing: 18)
    case .runway:
      return EdgeInsets(top: 18, leading: 18, bottom: 18, trailing: 18)
    case .topBar:
      return EdgeInsets(top: 16, leading: 18, bottom: 16, trailing: 18)
    case .toolbarStrip:
      return EdgeInsets(top: 12, leading: 14, bottom: 12, trailing: 14)
    case .statusStrip:
      return EdgeInsets(top: 4, leading: 0, bottom: 4, trailing: 0)
    case .notice:
      return EdgeInsets(top: 12, leading: 12, bottom: 12, trailing: 12)
    case .metricTile:
      return EdgeInsets(top: 16, leading: 16, bottom: 16, trailing: 16)
    case .gateRow:
      return EdgeInsets(top: 9, leading: 12, bottom: 9, trailing: 12)
    }
  }

  var background: Color {
    switch self {
    case .sidebarCard, .panel, .runway, .statusStrip:
      return SMColor.card
    case .sectionCard:
      return SMColor.cardElevated
    case .topBar, .toolbarStrip:
      return SMColor.card
    case .notice, .metricTile, .gateRow:
      return SMColor.cardElevated
    }
  }

  var cornerRadius: CGFloat {
    switch self {
    case .sidebarCard, .sectionCard, .panel:
      return 10
    case .runway:
      return 16
    case .topBar:
      return 14
    case .toolbarStrip:
      return 12
    case .statusStrip:
      return 14
    case .notice:
      return 12
    case .metricTile:
      return 14
    case .gateRow:
      return 10
    }
  }

  var shadowRadius: CGFloat {
    switch self {
    case .topBar, .toolbarStrip, .statusStrip, .sidebarCard, .sectionCard, .panel, .runway:
      return 0
    case .notice, .metricTile, .gateRow:
      return 2
    }
  }

  var shadowYOffset: CGFloat {
    switch self {
    case .topBar, .toolbarStrip, .statusStrip, .sidebarCard, .sectionCard, .panel, .runway:
      return 0
    case .notice, .metricTile, .gateRow:
      return 1
    }
  }
}

enum ButtonChrome: Equatable {
  case primary
  case secondary

  var background: Color {
    switch self {
    case .primary:
      return SMColor.graphite
    case .secondary:
      return SMColor.input
    }
  }

  var stroke: Color {
    switch self {
    case .primary:
      return SMColor.graphite
    case .secondary:
      return SMColor.hairline
    }
  }
}

extension View {
  func panelSurface(
    _ chrome: PanelChrome,
    minHeight: CGFloat? = nil,
    padding: EdgeInsets? = nil
  ) -> some View {
    self
      .padding(padding ?? chrome.padding)
      .frame(maxWidth: .infinity, minHeight: minHeight, alignment: .leading)
      .background(chrome.background)
      .clipShape(RoundedRectangle(cornerRadius: chrome.cornerRadius, style: .continuous))
      .shadow(color: SMColor.shadow, radius: chrome.shadowRadius, x: 0, y: chrome.shadowYOffset)
  }

  func buttonSurface(_ chrome: ButtonChrome) -> some View {
    self
      .background(chrome.background)
      .clipShape(RoundedRectangle(cornerRadius: 9, style: .continuous))
  }

  func selectedCardStyle(_ isSelected: Bool, tint: Color = SMColor.blue) -> some View {
    self
      .background(isSelected ? tint.opacity(0.08) : Color.clear)
      .overlay(alignment: .leading) {
        Rectangle()
          .fill(isSelected ? tint : .clear)
          .frame(width: 3)
      }
  }
}

enum SMColor {
  static let appBackground = dynamic(
    light: Color(red: 0.951, green: 0.958, blue: 0.972),
    dark: Color(red: 0.055, green: 0.064, blue: 0.080)
  )
  static let sidebar = dynamic(
    light: Color(red: 0.939, green: 0.947, blue: 0.964),
    dark: Color(red: 0.070, green: 0.081, blue: 0.102)
  )
  static let card = dynamic(
    light: Color(red: 0.986, green: 0.989, blue: 0.994),
    dark: Color(red: 0.105, green: 0.119, blue: 0.145)
  )
  static let cardElevated = dynamic(
    light: Color(red: 0.974, green: 0.980, blue: 0.990),
    dark: Color(red: 0.130, green: 0.146, blue: 0.176)
  )
  static let input = dynamic(
    light: Color(red: 0.946, green: 0.955, blue: 0.970),
    dark: Color(red: 0.082, green: 0.096, blue: 0.122)
  )
  static let hairline = dynamic(
    light: Color(red: 0.846, green: 0.868, blue: 0.904),
    dark: Color(red: 0.255, green: 0.288, blue: 0.342)
  )
  static let divider = dynamic(
    light: Color(red: 0.872, green: 0.887, blue: 0.916),
    dark: Color(red: 0.190, green: 0.220, blue: 0.270)
  )
  static let shadow = dynamic(light: Color.black.opacity(0.035), dark: Color.black.opacity(0.22))
  static let primaryText = dynamic(
    light: Color(red: 0.102, green: 0.118, blue: 0.145),
    dark: Color(red: 0.925, green: 0.940, blue: 0.965)
  )
  static let secondaryText = dynamic(
    light: Color(red: 0.392, green: 0.431, blue: 0.494),
    dark: Color(red: 0.670, green: 0.705, blue: 0.760)
  )
  static let tertiaryText = dynamic(
    light: Color(red: 0.500, green: 0.543, blue: 0.612),
    dark: Color(red: 0.505, green: 0.555, blue: 0.630)
  )
  static let inverseText = Color.white
  static let graphite = dynamic(
    light: Color(red: 0.149, green: 0.176, blue: 0.227),
    dark: Color(red: 0.155, green: 0.185, blue: 0.250)
  )
  static let blue = dynamic(
    light: Color(red: 0.145, green: 0.392, blue: 0.894),
    dark: Color(red: 0.365, green: 0.575, blue: 1.000)
  )
  static let cyan = dynamic(
    light: Color(red: 0.043, green: 0.596, blue: 0.702),
    dark: Color(red: 0.225, green: 0.780, blue: 0.885)
  )
  static let green = dynamic(
    light: Color(red: 0.145, green: 0.592, blue: 0.361),
    dark: Color(red: 0.310, green: 0.765, blue: 0.515)
  )
  static let amber = dynamic(
    light: Color(red: 0.824, green: 0.518, blue: 0.153),
    dark: Color(red: 0.980, green: 0.690, blue: 0.255)
  )
  static let red = dynamic(
    light: Color(red: 0.776, green: 0.224, blue: 0.204),
    dark: Color(red: 1.000, green: 0.430, blue: 0.405)
  )

  private static func dynamic(light: Color, dark: Color) -> Color {
#if os(macOS)
    Color(
      NSColor(name: nil) { appearance in
        if appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua {
          return NSColor(dark)
        }
        return NSColor(light)
      }
    )
#else
    light
#endif
  }
}
