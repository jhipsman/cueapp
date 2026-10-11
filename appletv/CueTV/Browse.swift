import SwiftUI

extension Color {
    static let cueBackground = Color(red: 0.04, green: 0.05, blue: 0.07)
}

// Where a tab has got to: the titles opened, one over another, so Back
// returns through them.
@MainActor
final class Nav: ObservableObject {
    @Published var path = NavigationPath()
}

// Opening a card: one being watched plays straight away (it carries on);
// anything else opens its page.
@MainActor
func openCard(_ card: WatchCard, cue: Cue, nav: Nav) {
    if card.resumes {
        cue.play(card)
    } else {
        nav.path.append(card)
    }
}

// A tab's own navigation: its page, and titles opened from it.
@MainActor
struct BrowseStack<Content: View>: View {
    @StateObject private var nav = Nav()
    @ViewBuilder var content: () -> Content

    var body: some View {
        NavigationStack(path: $nav.path) {
            content()
                .navigationDestination(for: WatchCard.self) { c in
                    TitleView(kind: c.kind, tmdbId: c.tmdbId)
                }
        }
        .environmentObject(nav)
    }
}

// Home (and the Shows and Movies tabs, with only set): a banner, then rows.
@MainActor
struct HomeView: View {
    @EnvironmentObject var cue: Cue
    @EnvironmentObject var nav: Nav
    let only: String?
    @State private var home: WatchHome?
    @State private var problem = ""

    var body: some View {
        Group {
            if let home {
                let rows = rowsOf(home)
                ScrollView(.vertical) {
                    LazyVStack(alignment: .leading, spacing: 30) {
                        let featured = featuredOf(home, rows)
                        if !featured.isEmpty {
                            Banner(cards: featured)
                        }
                        ForEach(Array(rows.enumerated()), id: \.element.id) { i, r in
                            CardRow(title: r.title, items: r.items, wide: r.key == "continue" || r.key == "new-episodes") { c in
                                openCard(c, cue: cue, nav: nav)
                            }
                            // What's on live, after Continue Watching (or first).
                            if only == nil && i == (rows.first?.key == "continue" ? 0 : -1) {
                                OnNowRow()
                            }
                        }
                        if only == nil && rows.first?.key != "continue" {
                            OnNowRow()
                        }
                    }
                    .padding(.bottom, 80)
                }
                .ignoresSafeArea(edges: [.top, .horizontal])
            } else if !problem.isEmpty {
                VStack(spacing: 30) {
                    Text(problem).multilineTextAlignment(.center)
                    Button("Try again") { Task { await load() } }
                }
            } else {
                ProgressView()
            }
        }
        .background(Color.cueBackground.ignoresSafeArea())
        .task { if home == nil { await load() } }
    }

    private func load() async {
        problem = ""
        do {
            home = try await cue.home()
        } catch {
            problem = error.localizedDescription
        }
    }

    private func rowsOf(_ h: WatchHome) -> [WatchRow] {
        h.rows.compactMap { r in
            let items = only == nil ? r.items : r.items.filter { $0.kind == only }
            return items.isEmpty ? nil : WatchRow(key: r.key, title: r.title, items: items)
        }
    }

    // The banner's titles: Home's own pick, then a few more with artwork
    // and a story from the rows (not ones being watched).
    private func featuredOf(_ h: WatchHome, _ rows: [WatchRow]) -> [WatchCard] {
        var out: [WatchCard] = []
        if let hero = h.hero, only == nil || hero.kind == only { out.append(hero) }
        for c in rows.flatMap(\.items) where out.count < 8 {
            if c.backdropUrl != nil, c.overview != nil, !c.resumes, !out.contains(where: { $0.kind == c.kind && $0.tmdbId == c.tmdbId }) {
                out.append(c)
            }
        }
        return out
    }
}

// The big banner at the top of Home: artwork edge to edge, the title, its
// story, Play and More Info. It moves on to the next featured title every
// few seconds (the dots show which), but not while Play or More Info is
// highlighted.
@MainActor
struct Banner: View {
    @EnvironmentObject var cue: Cue
    @EnvironmentObject var nav: Nav
    let cards: [WatchCard]
    @State private var index = 0
    @FocusState private var focused: Int?

    var body: some View {
        let card = cards[min(index, cards.count - 1)]
        ZStack(alignment: .bottomLeading) {
            Artwork(url: card.backdropUrl)
                .frame(maxWidth: .infinity)
                .frame(height: 820)
                .clipped()
            LinearGradient(colors: [Color.cueBackground.opacity(0.95), Color.cueBackground.opacity(0.55), .clear],
                           startPoint: .leading, endPoint: .init(x: 0.7, y: 0.5))
            LinearGradient(colors: [Color.cueBackground, .clear], startPoint: .bottom, endPoint: .init(x: 0.5, y: 0.55))
            VStack(alignment: .leading, spacing: 20) {
                Text(card.title)
                    .font(.system(size: 76, weight: .heavy))
                    .lineLimit(2)
                    .shadow(color: .black.opacity(0.5), radius: 12)
                MetaLine(year: card.year, rating: card.rating, kind: card.kind, extra: nil)
                if let o = card.overview {
                    Text(o).font(.body).lineLimit(3).frame(maxWidth: 1000, alignment: .leading).foregroundStyle(.white.opacity(0.9))
                }
                HStack(spacing: 30) {
                    Button { cue.play(card) } label: {
                        Label(card.resumes ? "Resume" : "Play", systemImage: "play.fill")
                    }
                    .focused($focused, equals: 0)
                    Button { nav.path.append(card) } label: {
                        Label("More Info", systemImage: "info.circle")
                    }
                    .focused($focused, equals: 1)
                }
                .padding(.top, 10)
                if cards.count > 1 {
                    HStack(spacing: 10) {
                        ForEach(cards.indices, id: \.self) { i in
                            Capsule().fill(Color.white.opacity(i == index ? 0.95 : 0.35))
                                .frame(width: i == index ? 34 : 12, height: 12)
                        }
                    }
                    .padding(.top, 14)
                }
            }
            .padding(.leading, 80)
            .padding(.bottom, 50)
            .id(index)
            .transition(.opacity)
        }
        .animation(.easeInOut(duration: 0.6), value: index)
        .task {
            while !Task.isCancelled {
                try? await Task.sleep(nanoseconds: 9_000_000_000)
                if focused == nil, cards.count > 1 { index = (index + 1) % cards.count }
            }
        }
    }
}

// What's on live now: a channel's logo and its show (with how far through)
// for Favorites, or the first channels. Selecting one plays it.
@MainActor
struct OnNowRow: View {
    @EnvironmentObject var cue: Cue
    @State private var channels: [Channel] = []
    @State private var playing: Channel?

    var body: some View {
        Group {
            if !channels.isEmpty {
                VStack(alignment: .leading, spacing: 18) {
                    HStack(spacing: 8) {
                        Text("On Now").font(.title3.weight(.semibold))
                        Image(systemName: "chevron.right").font(.caption.weight(.bold)).foregroundStyle(.secondary)
                    }
                    .padding(.leading, 80)
                    ScrollView(.horizontal, showsIndicators: false) {
                        LazyHStack(spacing: 30) {
                            ForEach(channels) { ch in
                                Button { playing = ch } label: { OnNowCard(channel: ch) }
                                    .buttonStyle(.card)
                            }
                        }
                        .padding(.horizontal, 80)
                        .padding(.vertical, 30)
                    }
                    .scrollClipDisabled()
                }
            }
        }
        .task {
            guard channels.isEmpty, let live = try? await cue.channels(), live.configured, let all = live.channels else { return }
            let favs = all.filter(\.favorite)
            channels = Array((favs.isEmpty ? all : favs).filter { $0.now != nil }.prefix(20))
        }
        .fullScreenCover(item: $playing) { ch in
            PlayerScreen(channel: ch).environmentObject(cue)
        }
    }
}

@MainActor
struct OnNowCard: View {
    @EnvironmentObject var cue: Cue
    let channel: Channel

    var body: some View {
        HStack(spacing: 18) {
            Logo(channel: channel).frame(width: 120, height: 70)
            VStack(alignment: .leading, spacing: 6) {
                Text(channel.name).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
                Text(channel.now?.title ?? "").font(.callout.weight(.semibold)).lineLimit(1)
                if let now = channel.now {
                    ProgressView(value: now.progress()).tint(cue.accentColor)
                }
                if let next = channel.next, let at = next.startDate {
                    Text("\(Programme.clock.string(from: at))  \(next.title)").font(.caption2).foregroundStyle(.secondary).lineLimit(1)
                }
            }
        }
        .padding(18)
        .frame(width: 520, height: 150, alignment: .leading)
        .background(channelColor(channel.id).opacity(0.5))
    }
}

// Year, score and kind, in a line.
struct MetaLine: View {
    let year: Int?
    let rating: Double?
    let kind: String
    let extra: String?

    var body: some View {
        HStack(spacing: 22) {
            if let y = year { Text(String(y)) }
            if let r = rating, r > 0 {
                Label(String(format: "%.1f", r), systemImage: "star.fill").labelStyle(.titleAndIcon)
            }
            Text(kind == "tv" ? "Series" : "Movie")
            if let e = extra, !e.isEmpty { Text(e) }
        }
        .font(.callout)
        .foregroundStyle(.white.opacity(0.8))
    }
}

// Search: Apple's own search keyboard, posters as you type.
@MainActor
struct SearchView: View {
    @EnvironmentObject var cue: Cue
    @EnvironmentObject var nav: Nav
    @State private var query = ""
    @State private var results: [WatchCard] = []

    var body: some View {
        ScrollView {
            PosterGrid(items: results) { openCard($0, cue: cue, nav: nav) }
        }
        .searchable(text: $query, prompt: "Movies and shows")
        .task(id: query) {
            let q = query.trimmingCharacters(in: .whitespaces)
            guard q.count >= 2 else { results = []; return }
            try? await Task.sleep(nanoseconds: 350_000_000)
            if Task.isCancelled { return }
            if let r = try? await cue.search(q) { results = r }
        }
    }
}

// My List: the titles saved for later.
@MainActor
struct MyListView: View {
    @EnvironmentObject var cue: Cue
    @EnvironmentObject var nav: Nav
    @State private var items: [WatchCard]?

    var body: some View {
        ScrollView {
            if let items, items.isEmpty {
                Text("Nothing here yet. Add titles with My List on their page.")
                    .foregroundStyle(.secondary)
                    .padding(80)
            } else if let items {
                PosterGrid(items: items) { openCard($0, cue: cue, nav: nav) }
            } else {
                ProgressView().padding(80)
            }
        }
        .task {
            if let h = try? await cue.home() {
                items = h.rows.first { $0.key == "my-list" }?.items ?? []
            } else {
                items = []
            }
        }
    }
}

@MainActor
struct PosterGrid: View {
    let items: [WatchCard]
    var onOpen: (WatchCard) -> Void

    var body: some View {
        LazyVGrid(columns: Array(repeating: GridItem(.fixed(250), spacing: 50), count: 6), spacing: 60) {
            ForEach(items) { c in
                PosterCard(card: c, onOpen: onOpen)
            }
        }
        .padding(60)
    }
}
