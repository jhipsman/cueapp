import SwiftUI

// Live TV as a programme guide: the groups down the left (Favorites first),
// then each channel's tile and its shows across a timeline, as wide as
// they are long, with a line at now. The show on now says LIVE and fills
// in as it goes. A channel's tile or its show on now plays it full screen;
// press and hold a tile to add it to (or take it off) Favorites.
//
// The guide is one list of rows (a channel's tile and its shows), built
// only as they come on screen, so a provider with thousands of channels
// opens at once. The timeline shows what fits across the screen and moves
// on an hour when focus reaches its right edge (back an hour at its left).

@MainActor
struct LiveView: View {
    @EnvironmentObject var cue: Cue
    @State private var channels: [Channel] = []
    @State private var categories: [Category] = []
    @State private var groups: [ChannelGroup] = []
    @State private var rows: [Channel] = []
    @State private var group = ""
    @State private var problem = ""
    @State private var loading = true
    @State private var playing: Channel?
    @State private var guide: [String: [Programme]] = [:]
    // What the guide covers (from half an hour back), and the part of it
    // on screen, from viewStart, spanMinutes wide.
    @State private var windowStart = LiveView.startOfWindow()
    @State private var viewStart = LiveView.startOfWindow()
    @State private var spanMinutes: Double = 120
    @State private var now = Date()
    @FocusState private var focusedGroup: String?
    @FocusState private var focusedShow: ShowKey?

    static let favorites = "★"
    static let all = "*"
    static let perMinute: CGFloat = 9 // points of timeline a minute
    static let rowHeight: CGFloat = 112
    static let tileWidth: CGFloat = 330
    static let gap: CGFloat = 14 // between a channel's tile and its shows
    static let hours = 5
    static let maxRows = 300

    var body: some View {
        HStack(spacing: 0) {
            groupsColumn
                .frame(width: 330)
            if loading && channels.isEmpty {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if !problem.isEmpty {
                VStack(spacing: 24) {
                    Text(problem).multilineTextAlignment(.center)
                    Button("Try again") { Task { await load() } }
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                grid
            }
        }
        .background(Color.cueBackground.ignoresSafeArea())
        .task { await load() }
        .task(id: group) { await loadGuide() }
        .task { await keepCurrent() }
        // Moving down the groups shows each in turn, once focus rests on
        // one: holding down on the remote doesn't redraw every group passed.
        .task(id: focusedGroup) {
            guard let g = focusedGroup, g != group else { return }
            try? await Task.sleep(for: .milliseconds(350))
            if !Task.isCancelled, focusedGroup == g { group = g }
        }
        .onChange(of: group) { _, _ in rows = shown }
        .onChange(of: focusedShow) { _, key in
            if let key { follow(key) }
        }
        .fullScreenCover(item: $playing) { ch in
            PlayerScreen(channel: ch).environmentObject(cue)
        }
    }

    static func startOfWindow(at date: Date = Date()) -> Date {
        // Half an hour back, on the half hour: what just ended shows too.
        let t = date.addingTimeInterval(-30 * 60).timeIntervalSince1970
        return Date(timeIntervalSince1970: (t / 1800).rounded(.down) * 1800)
    }

    // ---- The groups ----

    fileprivate struct ChannelGroup: Hashable {
        let id: String
        let name: String
        let count: Int
    }

    private func makeGroups() -> [ChannelGroup] {
        var out: [ChannelGroup] = []
        let favs = channels.filter(\.favorite).count
        if favs > 0 { out.append(ChannelGroup(id: LiveView.favorites, name: "Favorites", count: favs)) }
        out.append(ChannelGroup(id: LiveView.all, name: "All Channels", count: channels.count))
        var counts: [String: Int] = [:]
        for c in channels { counts[c.category, default: 0] += 1 }
        for c in categories where counts[c.id, default: 0] > 0 {
            out.append(ChannelGroup(id: c.id, name: c.name, count: counts[c.id, default: 0]))
        }
        return out
    }

    private var groupsColumn: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Live TV").font(.system(size: 40, weight: .bold)).padding(.leading, 12).padding(.bottom, 10)
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 4) {
                    ForEach(groups, id: \.id) { g in
                        Button { group = g.id } label: {
                            HStack {
                                if g.id == LiveView.favorites {
                                    Image(systemName: "star.fill").foregroundStyle(.yellow).font(.caption)
                                }
                                Text(g.name).lineLimit(1)
                                Spacer()
                                Text("\(g.count)").font(.caption).foregroundStyle(.secondary)
                            }
                            .font(.callout.weight(group == g.id ? .bold : .regular))
                            .padding(.horizontal, 12)
                        }
                        .buttonStyle(.plain)
                        .focused($focusedGroup, equals: g.id)
                    }
                }
                .padding(.vertical, 10)
            }
        }
        .padding(.leading, 50)
        .padding(.trailing, 16)
        .padding(.top, 30)
    }

    // ---- The guide ----

    private var shown: [Channel] {
        let list: [Channel]
        switch group {
        case LiveView.favorites: list = channels.filter(\.favorite)
        case LiveView.all, "": list = channels
        default: list = channels.filter { $0.category == group }
        }
        return Array(list.prefix(LiveView.maxRows))
    }

    private var viewEnd: Date { viewStart.addingTimeInterval(spanMinutes * 60) }

    private var grid: some View {
        GeometryReader { geo in
            let timeline = max(geo.size.width - LiveView.tileWidth - LiveView.gap - 20, 300)
            ScrollView(.vertical) {
                LazyVStack(alignment: .leading, spacing: 10) {
                    HStack(spacing: LiveView.gap) {
                        Text(dayLabel).font(.callout.weight(.semibold)).foregroundStyle(.secondary)
                            .frame(width: LiveView.tileWidth)
                        ruler.frame(width: timeline, alignment: .leading).clipped()
                    }
                    .frame(height: 50)
                    ForEach(rows) { ch in
                        HStack(spacing: LiveView.gap) {
                            ChannelTile(channel: ch) { playing = ch } onFavorite: { Task { await toggleFavorite(ch) } }
                                .frame(width: LiveView.tileWidth, height: LiveView.rowHeight)
                            GuideRow(channel: ch, shows: guide[ch.id] ?? [], start: viewStart, end: viewEnd, now: now,
                                     focus: $focusedShow) { playing = ch }
                                .frame(width: timeline, height: LiveView.rowHeight, alignment: .leading)
                        }
                    }
                }
                .padding(.vertical, 30)
                .padding(.trailing, 20)
            }
            .overlay(alignment: .topLeading) { nowLine(height: geo.size.height) }
            .onAppear { spanMinutes = Double(timeline / LiveView.perMinute) }
            .onChange(of: timeline) { _, w in spanMinutes = Double(w / LiveView.perMinute) }
        }
    }

    private static let dayFormat: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "EEE d"
        return f
    }()

    private var dayLabel: String { LiveView.dayFormat.string(from: viewStart) }

    // Times along the top, every half hour (viewStart is always on one).
    private var ruler: some View {
        HStack(spacing: 0) {
            ForEach(0..<(Int(spanMinutes / 30) + 1), id: \.self) { i in
                let t = viewStart.addingTimeInterval(Double(i) * 1800)
                let onHour = Calendar.current.component(.minute, from: t) == 0
                Text(Programme.clock.string(from: t))
                    .font(.caption.weight(onHour ? .bold : .regular))
                    .foregroundStyle(onHour ? .primary : .secondary)
                    .frame(width: 30 * LiveView.perMinute, alignment: .leading)
                    .fixedSize()
            }
        }
    }

    @ViewBuilder
    private func nowLine(height: CGFloat) -> some View {
        let minutes = now.timeIntervalSince(viewStart) / 60
        if minutes >= 0 && minutes <= spanMinutes {
            let x = LiveView.tileWidth + LiveView.gap + CGFloat(minutes) * LiveView.perMinute
            VStack(spacing: 0) {
                Image(systemName: "arrowtriangle.down.fill").font(.caption).foregroundStyle(.red)
                Rectangle().fill(Color.red).frame(width: 3)
            }
            .frame(height: height - 50)
            .offset(x: x - 8, y: 50)
            .allowsHitTesting(false)
        }
    }

    // Moves the timeline an hour on when the focused show reaches into the
    // last half hour on screen, and back an hour when it starts in the
    // first, so the next show over is always there to move to.
    private func follow(_ key: ShowKey) {
        guard let a = key.start, let b = key.stop else { return }
        let hour: TimeInterval = 3600, half: TimeInterval = 1800
        let span = spanMinutes * 60
        let covered = Double(LiveView.hours) * hour
        let latest = windowStart.addingTimeInterval(max(0, ((covered - span) / half).rounded(.up) * half))
        var s = viewStart
        while b > s.addingTimeInterval(span - half), s < latest, s.addingTimeInterval(hour) <= b.addingTimeInterval(-half) {
            s = min(s.addingTimeInterval(hour), latest)
        }
        if s == viewStart {
            while a < s.addingTimeInterval(half), s > windowStart {
                s = max(s.addingTimeInterval(-hour), windowStart)
            }
        }
        if s != viewStart {
            withAnimation(.easeInOut(duration: 0.25)) { viewStart = s }
        }
    }

    // ---- Loading ----

    private func load() async {
        loading = true
        problem = ""
        defer { loading = false }
        do {
            let live = try await cue.channels()
            guard live.configured else {
                problem = "Live TV isn't set up on this Cue yet: add your IPTV provider in Cue's settings."
                return
            }
            channels = live.channels ?? []
            categories = live.categories ?? []
            groups = makeGroups()
            let before = group
            if group.isEmpty {
                group = channels.contains(where: \.favorite) ? LiveView.favorites : LiveView.all
            }
            rows = shown
            // A new group loads its guide by itself (the task on group).
            if group == before { await loadGuide() }
        } catch {
            if Task.isCancelled { return }
            problem = error.localizedDescription
        }
    }

    private func loadGuide() async {
        let ids = shown.map(\.id)
        guard !ids.isEmpty else { return }
        let start = LiveView.startOfWindow()
        // A new window starts the guide afresh; the same one keeps what
        // other groups loaded, so going back to them draws at once.
        var all = start == windowStart ? guide : [:]
        for chunk in stride(from: 0, to: ids.count, by: 150) {
            let part = Array(ids[chunk..<min(chunk + 150, ids.count)])
            // An hour past the window, so the last screenful is filled.
            if let g = try? await cue.guide(ids: part, from: start, hours: LiveView.hours + 1) {
                all.merge(g.programmes) { _, new in new }
            }
            if Task.isCancelled { return }
        }
        if start != windowStart {
            windowStart = start
            if viewStart < start { viewStart = start }
        }
        guide = all
    }

    // Keeps LIVE, the line at now and the guide's window up to date while
    // the guide stays open.
    private func keepCurrent() async {
        while !Task.isCancelled {
            try? await Task.sleep(for: .seconds(30))
            if Task.isCancelled { return }
            now = Date()
            if LiveView.startOfWindow(at: now) != windowStart { await loadGuide() }
        }
    }

    private func toggleFavorite(_ ch: Channel) async {
        let on = !ch.favorite
        if (try? await cue.setFavorite(ch.id, on)) != nil, let i = channels.firstIndex(where: { $0.id == ch.id }) {
            channels[i].favorite = on
            groups = makeGroups()
            rows = shown
        }
    }
}

// A show in the guide, for following focus along the timeline.
struct ShowKey: Hashable {
    let channel: String
    let index: Int
    let start: Date?
    let stop: Date?
}

// A colour for each channel, so its shows stand apart in the guide.
func channelColor(_ id: String) -> Color {
    var h: UInt32 = 5381
    for b in id.utf8 { h = (h &* 33) &+ UInt32(b) }
    // Stirred, so channels numbered one after another (as providers number
    // them) get colours far apart, not shades of one.
    h ^= h >> 15
    h = h &* 0x2C1B3C6D
    h ^= h >> 12
    return Color(hue: Double(h % 360) / 360, saturation: 0.55, brightness: 0.36)
}

@MainActor
struct ChannelTile: View {
    let channel: Channel
    var onPlay: () -> Void
    var onFavorite: () -> Void

    var body: some View {
        Button(action: onPlay) {
            HStack(spacing: 16) {
                Logo(channel: channel)
                    .frame(width: 110, height: 64)
                VStack(alignment: .leading, spacing: 2) {
                    if channel.num > 0 {
                        Text("\(channel.num)").font(.caption2).foregroundStyle(.secondary)
                    }
                    Text(channel.name).font(.callout.weight(.semibold)).lineLimit(2)
                }
                Spacer(minLength: 0)
                if channel.favorite {
                    Image(systemName: "star.fill").font(.caption).foregroundStyle(.yellow)
                }
            }
            .padding(.horizontal, 14)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .buttonStyle(.card)
        .contextMenu {
            Button(channel.favorite ? "Remove from Favorites" : "Add to Favorites", action: onFavorite)
        }
    }
}

// One channel's shows across the part of the timeline on screen.
@MainActor
struct GuideRow: View {
    let channel: Channel
    let shows: [Programme]
    let start: Date
    let end: Date
    let now: Date
    var focus: FocusState<ShowKey?>.Binding
    var onPlay: () -> Void

    var body: some View {
        let tint = channelColor(channel.id)
        HStack(spacing: 0) {
            let cells = layout()
            if cells.isEmpty {
                ShowCell(title: channel.now?.title ?? "No guide for this channel", times: "", tint: tint, live: true, progress: 0, onPlay: onPlay)
                    .focused(focus, equals: ShowKey(channel: channel.id, index: -1, start: nil, stop: nil))
            } else {
                ForEach(cells, id: \.index) { c in
                    if c.gap > 0 { Color.clear.frame(width: c.gap) }
                    let live = (c.show.startDate ?? .distantFuture) <= now && now < (c.show.stopDate ?? .distantPast)
                    ShowCell(title: c.show.title, times: Programme.clock.string(from: c.show.startDate ?? start), tint: tint,
                             live: live, progress: live ? c.show.progress(at: now) : 0, onPlay: live ? onPlay : nil)
                        .frame(width: max(c.width - 8, 30))
                        .padding(.trailing, 8)
                        .focused(focus, equals: ShowKey(channel: channel.id, index: c.index, start: c.show.startDate, stop: c.show.stopDate))
                }
            }
            Spacer(minLength: 0)
        }
    }

    private struct Cell {
        let index: Int
        let show: Programme
        let gap: CGFloat
        let width: CGFloat
    }

    private func layout() -> [Cell] {
        var out: [Cell] = []
        var x: CGFloat = 0
        for (i, p) in shows.enumerated() {
            guard let a = p.startDate, let b = p.stopDate, b > start, a < end else { continue }
            let from = max(a, start), to = min(b, end)
            let left = CGFloat(from.timeIntervalSince(start) / 60) * LiveView.perMinute
            let w = CGFloat(to.timeIntervalSince(from) / 60) * LiveView.perMinute
            out.append(Cell(index: i, show: p, gap: max(left - x, 0), width: w))
            x = left + w
        }
        return out
    }
}

@MainActor
struct ShowCell: View {
    let title: String
    let times: String
    let tint: Color
    let live: Bool
    let progress: Double
    var onPlay: (() -> Void)?

    var body: some View {
        Button { onPlay?() } label: {
            ZStack(alignment: .leading) {
                tint.opacity(live ? 1 : 0.6)
                if live && progress > 0 {
                    Color.white.opacity(0.12).scaleEffect(x: progress, y: 1, anchor: .leading)
                }
                VStack(alignment: .leading, spacing: 6) {
                    Text(title).font(.callout.weight(.semibold)).lineLimit(2)
                    HStack(spacing: 8) {
                        if !times.isEmpty { Text(times).font(.caption2).foregroundStyle(.white.opacity(0.75)) }
                        if live {
                            Text("LIVE").font(.system(size: 14, weight: .heavy))
                                .padding(.horizontal, 6).padding(.vertical, 2)
                                .background(Color.red, in: RoundedRectangle(cornerRadius: 4))
                        }
                    }
                }
                .padding(.horizontal, 14)
                .opacity(live ? 1 : 0.8)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .buttonStyle(.card)
    }
}

// A channel's logo, through Cue (which fetches it from the provider).
@MainActor
struct Logo: View {
    @EnvironmentObject var cue: Cue
    let channel: Channel
    @State private var image: UIImage?

    var body: some View {
        ZStack {
            RoundedRectangle(cornerRadius: 10).fill(channelColor(channel.id).opacity(0.9))
            if let image {
                Image(uiImage: image).resizable().scaledToFit().padding(8)
            } else {
                Text(String(channel.name.prefix(3)).uppercased()).font(.caption.weight(.heavy)).foregroundStyle(.white)
            }
        }
        .task(id: channel.id) { await fetch() }
    }

    private func fetch() async {
        guard let path = channel.logo, let u = cue.url(path) else { return }
        if let hit = Logo.cache.object(forKey: u as NSURL) {
            image = hit
            return
        }
        var req = URLRequest(url: u)
        cue.headers.forEach { req.setValue($1, forHTTPHeaderField: $0) }
        guard let got = try? await URLSession.shared.data(for: req),
              (got.1 as? HTTPURLResponse)?.statusCode == 200 else { return }
        // Decoded away from the main thread, so scrolling stays smooth.
        let data = got.0
        guard let img = await Task.detached(operation: { UIImage(data: data)?.preparingForDisplay() }).value else { return }
        Logo.cache.setObject(img, forKey: u as NSURL)
        image = img
    }

    static let cache = NSCache<NSURL, UIImage>()
}
