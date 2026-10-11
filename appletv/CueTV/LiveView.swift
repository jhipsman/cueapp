import SwiftUI

// Live TV as a programme guide: the groups down the left (Favorites first),
// then each channel's tile and its shows across a timeline, as wide as
// they are long, with a line at now. The show on now says LIVE and fills
// in as it goes. A channel's tile or its show on now plays it full screen;
// press and hold a tile to add it to (or take it off) Favorites.

@MainActor
struct LiveView: View {
    @EnvironmentObject var cue: Cue
    @State private var channels: [Channel] = []
    @State private var categories: [Category] = []
    @State private var group = ""
    @State private var problem = ""
    @State private var loading = true
    @State private var playing: Channel?
    @State private var guide: [String: [Programme]] = [:]
    @State private var windowStart = LiveView.startOfWindow()
    @FocusState private var focusedGroup: String?

    static let favorites = "★"
    static let all = "*"
    static let perMinute: CGFloat = 9 // points of timeline a minute
    static let rowHeight: CGFloat = 112
    static let hours = 5

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
        .fullScreenCover(item: $playing) { ch in
            PlayerScreen(channel: ch).environmentObject(cue)
        }
    }

    static func startOfWindow() -> Date {
        // Half an hour back, on the half hour: what just ended shows too.
        let t = Date().addingTimeInterval(-30 * 60).timeIntervalSince1970
        return Date(timeIntervalSince1970: (t / 1800).rounded(.down) * 1800)
    }

    // ---- The groups ----

    private struct ChannelGroup: Hashable {
        let id: String
        let name: String
        let count: Int
    }

    private var groups: [ChannelGroup] {
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
        .onChange(of: focusedGroup) { _, now in
            if let now { group = now }
        }
    }

    // ---- The guide ----

    private var shown: [Channel] {
        switch group {
        case LiveView.favorites: return channels.filter(\.favorite)
        case LiveView.all, "": return channels
        default: return channels.filter { $0.category == group }
        }
    }

    private var grid: some View {
        let list = Array(shown.prefix(300))
        let width = CGFloat(LiveView.hours * 60) * LiveView.perMinute
        return ScrollView(.vertical) {
            HStack(alignment: .top, spacing: 14) {
                // Channel tiles.
                LazyVStack(spacing: 10) {
                    Text(dayLabel).font(.callout.weight(.semibold)).foregroundStyle(.secondary)
                        .frame(height: 50)
                    ForEach(list) { ch in
                        ChannelTile(channel: ch) { playing = ch } onFavorite: { Task { await toggleFavorite(ch) } }
                            .frame(width: 330, height: LiveView.rowHeight)
                    }
                }
                .frame(width: 330)
                // Shows across the timeline.
                ScrollView(.horizontal, showsIndicators: false) {
                    ZStack(alignment: .topLeading) {
                        LazyVStack(alignment: .leading, spacing: 10) {
                            ruler.frame(width: width, height: 50, alignment: .leading)
                            ForEach(list) { ch in
                                GuideRow(channel: ch, shows: guide[ch.id] ?? [], start: windowStart, width: width) { playing = ch }
                                    .frame(width: width, height: LiveView.rowHeight, alignment: .leading)
                            }
                        }
                        nowLine(height: CGFloat(list.count) * (LiveView.rowHeight + 10) + 60)
                    }
                    .padding(.trailing, 60)
                }
                .scrollClipDisabled()
            }
            .padding(.vertical, 30)
            .padding(.trailing, 20)
        }
    }

    private var dayLabel: String {
        let f = DateFormatter()
        f.dateFormat = "EEE d"
        return f.string(from: Date())
    }

    // Times along the top, every half hour.
    private var ruler: some View {
        HStack(spacing: 0) {
            ForEach(0..<(LiveView.hours * 2), id: \.self) { i in
                let t = windowStart.addingTimeInterval(Double(i) * 1800)
                Text(Programme.clock.string(from: t))
                    .font(.caption.weight(i % 2 == 0 ? .bold : .regular))
                    .foregroundStyle(i % 2 == 0 ? .primary : .secondary)
                    .frame(width: 30 * LiveView.perMinute, alignment: .leading)
            }
        }
    }

    private func nowLine(height: CGFloat) -> some View {
        TimelineView(.periodic(from: .now, by: 30)) { ctx in
            let x = CGFloat(ctx.date.timeIntervalSince(windowStart) / 60) * LiveView.perMinute
            VStack(spacing: 0) {
                Image(systemName: "arrowtriangle.down.fill").font(.caption).foregroundStyle(.red)
                Rectangle().fill(Color.red).frame(width: 3)
            }
            .frame(height: height)
            .offset(x: x - 8, y: 20)
            .allowsHitTesting(false)
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
            if group.isEmpty {
                group = channels.contains(where: \.favorite) ? LiveView.favorites : LiveView.all
            }
            await loadGuide()
        } catch {
            problem = error.localizedDescription
        }
    }

    private func loadGuide() async {
        let ids = shown.prefix(300).map(\.id)
        guard !ids.isEmpty else { return }
        windowStart = LiveView.startOfWindow()
        var all = guide
        for chunk in stride(from: 0, to: ids.count, by: 150) {
            let part = Array(ids[chunk..<min(chunk + 150, ids.count)])
            if let g = try? await cue.guide(ids: part, from: windowStart, hours: LiveView.hours) {
                all.merge(g.programmes) { _, new in new }
            }
        }
        guide = all
    }

    private func toggleFavorite(_ ch: Channel) async {
        let on = !ch.favorite
        if (try? await cue.setFavorite(ch.id, on)) != nil, let i = channels.firstIndex(where: { $0.id == ch.id }) {
            channels[i].favorite = on
        }
    }
}

// A colour for each channel, so its shows stand apart in the guide.
func channelColor(_ id: String) -> Color {
    var h: UInt32 = 5381
    for b in id.utf8 { h = (h &* 33) &+ UInt32(b) }
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

// One channel's shows across the timeline.
@MainActor
struct GuideRow: View {
    let channel: Channel
    let shows: [Programme]
    let start: Date
    let width: CGFloat
    var onPlay: () -> Void

    var body: some View {
        let end = start.addingTimeInterval(Double(LiveView.hours) * 3600)
        let tint = channelColor(channel.id)
        HStack(spacing: 0) {
            if shows.isEmpty {
                ShowCell(title: channel.now?.title ?? "No guide for this channel", times: "", tint: tint, live: true, progress: 0, onPlay: onPlay)
                    .frame(width: width - 8)
            } else {
                let cells = layout(end: end)
                ForEach(cells, id: \.index) { c in
                    if c.gap > 0 { Color.clear.frame(width: c.gap) }
                    let now = Date()
                    let live = (c.show.startDate ?? .distantFuture) <= now && now < (c.show.stopDate ?? .distantPast)
                    ShowCell(title: c.show.title, times: Programme.clock.string(from: c.show.startDate ?? start), tint: tint,
                             live: live, progress: live ? c.show.progress(at: now) : 0, onPlay: live ? onPlay : nil)
                        .frame(width: max(c.width - 8, 30))
                        .padding(.trailing, 8)
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

    private func layout(end: Date) -> [Cell] {
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
                    GeometryReader { g in
                        Color.white.opacity(0.12).frame(width: g.size.width * progress)
                    }
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
              (got.1 as? HTTPURLResponse)?.statusCode == 200,
              let img = UIImage(data: got.0) else { return }
        Logo.cache.setObject(img, forKey: u as NSURL)
        image = img
    }

    static let cache = NSCache<NSURL, UIImage>()
}
