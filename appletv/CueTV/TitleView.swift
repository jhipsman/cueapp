import SwiftUI

// A film's or a show's page: the artwork big, Play (or Resume) and My List,
// a show's seasons and episodes, and more like it.
@MainActor
struct TitleView: View {
    @EnvironmentObject var cue: Cue
    @EnvironmentObject var nav: Nav
    let kind: String
    let tmdbId: Int
    @State private var title: WatchTitle?
    @State private var problem = ""
    @State private var season = 0
    @State private var episodes: [WatchEpisode] = []
    @State private var inList = false

    var body: some View {
        Group {
            if let t = title {
                ScrollView(.vertical) {
                    VStack(alignment: .leading, spacing: 50) {
                        header(t)
                        if kind == "tv", let seasons = t.seasons, !seasons.isEmpty {
                            seasonPicker(seasons)
                            episodeRow(t)
                        }
                        if let more = t.more, !more.isEmpty {
                            CardRow(title: "More Like This", items: Array(more.prefix(18))) { openCard($0, cue: cue, nav: nav) }
                        }
                    }
                    .padding(.bottom, 80)
                }
                .ignoresSafeArea(edges: [.top, .horizontal])
            } else if !problem.isEmpty {
                Text(problem)
            } else {
                ProgressView()
            }
        }
        .background(Color.cueBackground.ignoresSafeArea())
        .task { await load() }
        .task(id: season) { await loadEpisodes() }
    }

    // ---- The top ----

    private func header(_ t: WatchTitle) -> some View {
        ZStack(alignment: .bottomLeading) {
            Artwork(url: t.backdropUrl)
                .frame(maxWidth: .infinity)
                .frame(height: 900)
                .clipped()
            LinearGradient(colors: [Color.cueBackground.opacity(0.97), Color.cueBackground.opacity(0.6), .clear],
                           startPoint: .leading, endPoint: .init(x: 0.75, y: 0.5))
            LinearGradient(colors: [Color.cueBackground, .clear], startPoint: .bottom, endPoint: .init(x: 0.5, y: 0.5))
            VStack(alignment: .leading, spacing: 20) {
                Text(t.title)
                    .font(.system(size: 80, weight: .heavy))
                    .lineLimit(2)
                    .shadow(color: .black.opacity(0.5), radius: 12)
                if let tag = t.tagline, !tag.isEmpty {
                    Text(tag).font(.callout).italic().foregroundStyle(.white.opacity(0.7))
                }
                MetaLine(year: t.year, rating: t.rating, kind: t.kind, extra: [t.certification, runtime(t.runtime), t.genres?.prefix(3).joined(separator: ", ")]
                    .compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: "   "))
                if let o = t.overview {
                    Text(o).lineLimit(4).frame(maxWidth: 1100, alignment: .leading).foregroundStyle(.white.opacity(0.9))
                }
                HStack(spacing: 30) {
                    Button { play(t) } label: { Label(playLabel(t), systemImage: "play.fill") }
                    Button {
                        Task {
                            let on = !inList
                            if (try? await cue.setInList(t.kind, t.tmdbId, on)) != nil { inList = on }
                        }
                    } label: {
                        Label(inList ? "In My List" : "My List", systemImage: inList ? "checkmark" : "plus")
                    }
                }
                .padding(.top, 10)
            }
            .padding(.leading, 80)
            .padding(.bottom, 40)
        }
    }

    private func runtime(_ m: Int?) -> String? {
        guard let m, m > 0 else { return nil }
        return m >= 60 ? "\(m / 60)h \(m % 60)m" : "\(m)m"
    }

    private func playLabel(_ t: WatchTitle) -> String {
        if t.kind == "tv", let s = t.resumeSeason, let e = t.resumeEpisode {
            return "Resume S\(s):E\(e)"
        }
        if (t.resumePosition ?? 0) > 0 { return "Resume" }
        return "Play"
    }

    private func play(_ t: WatchTitle) {
        if t.kind == "tv" {
            let s = t.resumeSeason ?? t.seasons?.first?.number ?? 1
            let e = t.resumeEpisode ?? 1
            cue.playing = PlayRequest(kind: "tv", tmdbId: t.tmdbId, season: s, episode: e, title: t.title, subtitle: "S\(s):E\(e)", start: -1)
        } else {
            cue.playing = PlayRequest(kind: "movie", tmdbId: t.tmdbId, season: 0, episode: 0, title: t.title, subtitle: "", start: -1)
        }
    }

    // ---- Seasons and episodes ----

    private func seasonPicker(_ seasons: [SeasonInfo]) -> some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 24) {
                ForEach(seasons, id: \.number) { s in
                    Button { season = s.number } label: {
                        Text(s.name).fontWeight(season == s.number ? .bold : .regular)
                    }
                    .tint(season == s.number ? cue.accentColor : nil)
                }
            }
            .padding(.horizontal, 80)
            .padding(.vertical, 20)
        }
        .scrollClipDisabled()
    }

    private func episodeRow(_ t: WatchTitle) -> some View {
        ScrollView(.horizontal, showsIndicators: false) {
            LazyHStack(alignment: .top, spacing: 44) {
                ForEach(episodes) { e in
                    VStack(alignment: .leading, spacing: 12) {
                        Button {
                            cue.playing = PlayRequest(kind: "tv", tmdbId: t.tmdbId, season: e.season, episode: e.episode,
                                                      title: t.title, subtitle: "S\(e.season):E\(e.episode) · \(e.title)", start: -1)
                        } label: {
                            ZStack(alignment: .bottomLeading) {
                                Artwork(url: e.stillUrl, label: e.title)
                                    .frame(width: 440, height: 248)
                                    .clipped()
                                if let p = e.progress, p > 0, e.finished != true {
                                    GeometryReader { g in
                                        VStack {
                                            Spacer()
                                            ZStack(alignment: .leading) {
                                                Rectangle().fill(Color.white.opacity(0.25))
                                                Rectangle().fill(cue.accentColor).frame(width: g.size.width * p)
                                            }
                                            .frame(height: 6)
                                        }
                                    }
                                }
                                if e.finished == true {
                                    Image(systemName: "checkmark.circle.fill")
                                        .font(.title3)
                                        .foregroundStyle(cue.accentColor)
                                        .padding(12)
                                        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
                                }
                            }
                        }
                        .buttonStyle(.card)
                        Text("\(e.episode). \(e.title)").font(.callout.weight(.semibold)).lineLimit(1)
                        if let o = e.overview {
                            Text(o).font(.caption).foregroundStyle(.secondary).lineLimit(2)
                        }
                    }
                    .frame(width: 440, alignment: .leading)
                }
            }
            .padding(.horizontal, 80)
            .padding(.vertical, 30)
        }
        .scrollClipDisabled()
    }

    // ---- Loading ----

    private func load() async {
        guard title == nil else { return }
        do {
            let t = try await cue.title(kind, tmdbId)
            title = t
            inList = t.inList ?? false
            if kind == "tv" {
                season = t.resumeSeason ?? t.seasons?.first(where: { $0.number > 0 })?.number ?? t.seasons?.first?.number ?? 1
            }
        } catch {
            problem = error.localizedDescription
        }
    }

    private func loadEpisodes() async {
        guard kind == "tv", season > 0 || title?.seasons?.contains(where: { $0.number == 0 }) == true else { return }
        episodes = (try? await cue.season(tmdbId, season)) ?? []
    }
}
