import SwiftUI

// Live TV: groups down the left, their channels on the right with what's
// on now and next. Click plays it full screen; press and hold to add it to
// (or take it off) Favorites.

@MainActor
struct LiveView: View {
    @EnvironmentObject var cue: Cue
    @State private var channels: [Channel] = []
    @State private var categories: [Category] = []
    @State private var group = ""
    @State private var problem = ""
    @State private var loading = true
    @State private var playing: Channel?
    @State private var lastPlayed = ""
    @FocusState private var focusedGroup: String?

    private static let favorites = "★"
    private static let all = "*"

    var body: some View {
        HStack(spacing: 0) {
            groupsColumn
                .frame(width: 440)
            Divider()
            channelsColumn
        }
        .task { await load() }
        .fullScreenCover(item: $playing) { ch in
            PlayerScreen(channel: ch)
            .environmentObject(cue)
            .onDisappear { lastPlayed = ch.id }
        }
    }

    // ---- The groups ----

    private struct ChannelGroup: Hashable {
        let id: String
        let name: String
    }

    private var groups: [ChannelGroup] {
        var out: [ChannelGroup] = []
        if channels.contains(where: { $0.favorite }) { out.append(ChannelGroup(id: LiveView.favorites, name: "Favorites")) }
        out.append(ChannelGroup(id: LiveView.all, name: "All channels"))
        let used = Set(channels.map(\.category))
        for c in categories where used.contains(c.id) { out.append(ChannelGroup(id: c.id, name: c.name)) }
        return out
    }

    private var groupsColumn: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Live TV").font(.system(size: 44, weight: .bold))
                Spacer()
            }
            .padding(.bottom, 12)
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 4) {
                    ForEach(groups, id: \.id) { g in
                        Button {
                            group = g.id
                        } label: {
                            HStack {
                                Text(g.name).lineLimit(1)
                                Spacer()
                                if group == g.id {
                                    Circle().fill(cue.accentColor).frame(width: 12, height: 12)
                                }
                            }
                            .padding(.horizontal, 8)
                        }
                        .buttonStyle(.plain)
                        .focused($focusedGroup, equals: g.id)
                    }
                }
                .padding(.vertical, 20)
            }
            Spacer(minLength: 0)
            HStack(spacing: 16) {
                Button(cue.profileName.isEmpty ? "Profiles" : cue.profileName) { cue.switchProfile() }
                Button("Refresh") { Task { await load() } }
            }
            .font(.callout)
        }
        .padding(.leading, 60)
        .padding(.trailing, 24)
        .padding(.vertical, 40)
        .onChange(of: focusedGroup) { _, now in
            if let now { group = now }
        }
    }

    // ---- The channels ----

    private var shown: [Channel] {
        switch group {
        case LiveView.favorites: return channels.filter(\.favorite)
        case LiveView.all, "": return channels
        default: return channels.filter { $0.category == group }
        }
    }

    @ViewBuilder private var channelsColumn: some View {
        if loading && channels.isEmpty {
            ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
        } else if !problem.isEmpty {
            VStack(spacing: 24) {
                Text(problem).multilineTextAlignment(.center)
                Button("Try again") { Task { await load() } }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else {
            ScrollViewReader { scroller in
                ScrollView {
                    LazyVStack(spacing: 12) {
                        ForEach(shown) { ch in
                            Button {
                                playing = ch
                            } label: {
                                ChannelRow(channel: ch)
                            }
                            .buttonStyle(.card)
                            .id(ch.id)
                            .contextMenu {
                                Button(ch.favorite ? "Remove from Favorites" : "Add to Favorites") {
                                    Task { await toggleFavorite(ch) }
                                }
                            }
                        }
                    }
                    .padding(.horizontal, 48)
                    .padding(.vertical, 40)
                }
                .onChange(of: lastPlayed) { _, id in
                    if !id.isEmpty { scroller.scrollTo(id, anchor: .center) }
                }
            }
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
                group = channels.contains(where: { $0.favorite }) ? LiveView.favorites : LiveView.all
            }
            // The guide is still being fetched: what's on fills in shortly.
            if live.guideLoading == true {
                Task {
                    try? await Task.sleep(nanoseconds: 8_000_000_000)
                    if let again = try? await cue.channels(), let list = again.channels { channels = list }
                }
            }
        } catch {
            problem = error.localizedDescription
        }
    }

    private func toggleFavorite(_ ch: Channel) async {
        let on = !ch.favorite
        do {
            try await cue.setFavorite(ch.id, on)
            if let i = channels.firstIndex(where: { $0.id == ch.id }) { channels[i].favorite = on }
        } catch {
            problem = error.localizedDescription
        }
    }
}

@MainActor
struct ChannelRow: View {
    @EnvironmentObject var cue: Cue
    let channel: Channel

    var body: some View {
        HStack(spacing: 24) {
            Logo(channel: channel)
                .frame(width: 140, height: 80)
            VStack(alignment: .leading, spacing: 6) {
                HStack(spacing: 12) {
                    if channel.num > 0 {
                        Text("\(channel.num)").foregroundStyle(.secondary).monospacedDigit()
                    }
                    Text(channel.name).fontWeight(.semibold).lineLimit(1)
                    if channel.favorite {
                        Image(systemName: "star.fill").foregroundStyle(cue.accentColor).font(.caption)
                    }
                }
                if let now = channel.now {
                    Text(now.title).font(.callout).lineLimit(1)
                    ProgressView(value: now.progress())
                        .tint(cue.accentColor)
                        .frame(maxWidth: 520)
                }
                if let next = channel.next, let at = next.startDate {
                    Text("Next \(Programme.clock.string(from: at))  \(next.title)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 24)
        .padding(.vertical, 16)
        .frame(maxWidth: .infinity, alignment: .leading)
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
            RoundedRectangle(cornerRadius: 10).fill(Color.white.opacity(0.06))
            if let image {
                Image(uiImage: image).resizable().scaledToFit().padding(8)
            } else {
                Text(String(channel.name.prefix(3))).font(.caption).foregroundStyle(.secondary)
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
