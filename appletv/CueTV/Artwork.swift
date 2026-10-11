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
