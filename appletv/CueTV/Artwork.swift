import SwiftUI

// Pictures from TMDB (posters, backdrops, stills), kept by URLCache, with a
// quiet placeholder while they load.
@MainActor
struct Artwork: View {
    let url: String?
    var label: String = ""

    var body: some View {
        if let s = url, let u = URL(string: s) {
            AsyncImage(url: u, transaction: Transaction(animation: .easeOut(duration: 0.25))) { phase in
                switch phase {
                case .success(let img):
                    img.resizable().aspectRatio(contentMode: .fill)
                default:
                    placeholder
                }
            }
        } else {
            placeholder
        }
    }

    private var placeholder: some View {
        ZStack {
            LinearGradient(colors: [Color.white.opacity(0.10), Color.white.opacity(0.04)], startPoint: .top, endPoint: .bottom)
            if !label.isEmpty {
                Text(label).font(.caption).multilineTextAlignment(.center).foregroundStyle(.secondary).padding(12)
            }
        }
    }
}

// A poster that lifts and catches the light when focused (Apple's card
// style), its name underneath; wide cards are Continue Watching's: the
// artwork with the name over it, and the progress.
@MainActor
struct PosterCard: View {
    @EnvironmentObject var cue: Cue
    let card: WatchCard
    var wide = false
    var onOpen: (WatchCard) -> Void

    var body: some View {
        let w: CGFloat = wide ? 460 : 250
        VStack(alignment: .leading, spacing: 12) {
            Button { onOpen(card) } label: {
                ZStack(alignment: .bottomLeading) {
                    Artwork(url: wide ? (card.backdropUrl ?? card.posterUrl) : card.posterUrl, label: card.title)
                        .frame(width: w, height: wide ? 259 : 375)
                        .clipped()
                    if wide {
                        LinearGradient(colors: [.black.opacity(0.85), .clear], startPoint: .bottom, endPoint: .center)
                        VStack(alignment: .leading, spacing: 2) {
                            Text(card.title).font(.title3.weight(.bold)).lineLimit(1)
                            if !card.episodeLine.isEmpty {
                                Text(card.episodeLine).font(.caption).foregroundStyle(.white.opacity(0.8)).lineLimit(1)
                            }
                        }
                        .padding(.horizontal, 18)
                        .padding(.bottom, 20)
                    }
                    if let p = card.progress, p > 0 {
                        VStack {
                            Spacer()
                            ZStack(alignment: .leading) {
                                Rectangle().fill(Color.white.opacity(0.25))
                                Rectangle().fill(cue.accentColor).frame(width: w * min(p, 1))
                            }
                            .frame(height: 6)
                        }
                    }
                    if card.isNew == true {
                        Text("NEW EPISODE")
                            .font(.caption2.bold())
                            .padding(.horizontal, 10).padding(.vertical, 4)
                            .background(cue.accentColor, in: RoundedRectangle(cornerRadius: 6))
                            .padding(12)
                            .frame(maxHeight: .infinity, alignment: .top)
                    }
                }
            }
            .buttonStyle(.card)
            if !wide {
                Text(card.title).font(.caption).foregroundStyle(.secondary).lineLimit(1)
                    .frame(width: w, alignment: .leading)
            }
        }
    }
}

// A titled row that scrolls sideways.
@MainActor
struct CardRow: View {
    let title: String
    let items: [WatchCard]
    var wide = false
    var onOpen: (WatchCard) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 18) {
            HStack(spacing: 8) {
                Text(title).font(.title3.weight(.semibold))
                Image(systemName: "chevron.right").font(.caption.weight(.bold)).foregroundStyle(.secondary)
            }
            .padding(.leading, 80)
            ScrollView(.horizontal, showsIndicators: false) {
                LazyHStack(alignment: .top, spacing: 44) {
                    ForEach(items) { c in
                        PosterCard(card: c, wide: wide, onOpen: onOpen)
                    }
                }
                .padding(.horizontal, 80)
                .padding(.vertical, 30)
            }
            .scrollClipDisabled()
        }
    }
}

// A title's name as its logo artwork, as Apple's TV app draws it, or in big
// letters when it has none. Until the lookup answers, a quiet space, so the
// name doesn't flash before the logo replaces it.
struct TitleLogo: View {
    @EnvironmentObject var cue: Cue
    let kind: String
    let tmdbId: Int
    let title: String
    var known: String? = nil // "" when the server said there is none
    var size: CGFloat = 76
    var maxWidth: CGFloat = 760
    var maxHeight: CGFloat = 220
    @State private var url: String?

    var body: some View {
        Group {
            if let u = url, !u.isEmpty, let link = URL(string: u) {
                AsyncImage(url: link) { phase in
                    if let img = phase.image {
                        img.resizable().scaledToFit()
                            .frame(maxWidth: maxWidth, maxHeight: maxHeight, alignment: .bottomLeading)
                            .shadow(color: .black.opacity(0.5), radius: 16)
                    } else if phase.error != nil {
                        name
                    } else {
                        Color.clear.frame(width: 1, height: maxHeight * 0.6)
                    }
                }
            } else if url != nil {
                name
            } else {
                Color.clear.frame(width: 1, height: maxHeight * 0.6)
            }
        }
        .task(id: "\(kind)-\(tmdbId)") {
            url = known
            if known == nil { url = await cue.logo(kind, tmdbId) }
        }
    }

    private var name: some View {
        Text(title)
            .font(.system(size: size, weight: .heavy))
            .lineLimit(2)
            .shadow(color: .black.opacity(0.5), radius: 12)
    }
}

// IMDb, Rotten Tomatoes and Metacritic, each with its own badge.
struct ScoresLine: View {
    let ratings: Ratings?

    var body: some View {
        HStack(spacing: 18) {
            if let s = ratings?.imdb { score("IMDb", s, Color(red: 0.96, green: 0.77, blue: 0.09), .black) }
            if let s = ratings?.rottenTomatoes { score("RT", s, Color(red: 0.98, green: 0.2, blue: 0.04), .white) }
            if let s = ratings?.metacritic { score("MC", s, Color(white: 0.25), .white) }
        }
    }

    private func score(_ badge: String, _ value: String, _ bg: Color, _ fg: Color) -> some View {
        HStack(spacing: 8) {
            Text(badge).font(.system(size: 18, weight: .black)).foregroundStyle(fg)
                .padding(.horizontal, 7).padding(.vertical, 2)
                .background(bg, in: RoundedRectangle(cornerRadius: 5))
            Text(value).font(.callout.weight(.bold))
        }
    }
}

// TintGlow: the main colour of a title's picture, glowing softly from the
// lower left of a banner, so every title brings its own mood.
struct TintGlow: View {
    @EnvironmentObject var cue: Cue
    let kind: String
    let tmdbId: Int
    @State private var colour: Color?

    var body: some View {
        RadialGradient(colors: [(colour ?? .clear).opacity(0.45), .clear], center: .bottomLeading, startRadius: 0, endRadius: 1100)
            .blendMode(.screen)
            .allowsHitTesting(false)
            .animation(.easeInOut(duration: 0.8), value: colour)
            .task(id: "\(kind)-\(tmdbId)") { colour = await cue.colour(kind, tmdbId) }
    }
}
