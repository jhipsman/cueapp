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
                        if let hero = heroOf(home, rows) {
                            Banner(card: hero)
                        }
                        ForEach(rows) { r in
                            CardRow(title: r.title, items: r.items, wide: r.key == "continue" || r.key == "new-episodes") { c in
                                openCard(c, cue: cue, nav: nav)
                            }
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

    private func heroOf(_ h: WatchHome, _ rows: [WatchRow]) -> WatchCard? {
        if let hero = h.hero, only == nil || hero.kind == only { return hero }
        return rows.flatMap(\.items).first { $0.backdropUrl != nil && $0.overview != nil && !$0.resumes }
    }
}

// The big banner at the top of Home: artwork edge to edge, the title, its
// story, Play and More Info.
@MainActor
struct Banner: View {
    @EnvironmentObject var cue: Cue
    @EnvironmentObject var nav: Nav
    let card: WatchCard

    var body: some View {
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
                    Button { nav.path.append(card) } label: {
                        Label("More Info", systemImage: "info.circle")
                    }
                }
                .padding(.top, 10)
            }
            .padding(.leading, 80)
            .padding(.bottom, 50)
        }
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
