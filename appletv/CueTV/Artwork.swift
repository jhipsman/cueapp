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
// style); wide cards are Continue Watching's, with a caption and progress.
@MainActor
struct PosterCard: View {
    @EnvironmentObject var cue: Cue
    let card: WatchCard
    var wide = false
    var onOpen: (WatchCard) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Button { onOpen(card) } label: {
                ZStack(alignment: .bottomLeading) {
                    Artwork(url: wide ? (card.backdropUrl ?? card.posterUrl) : card.posterUrl, label: card.title)
                        .frame(width: wide ? 460 : 250, height: wide ? 259 : 375)
                        .clipped()
                    if let p = card.progress, p > 0 {
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
            if wide {
                VStack(alignment: .leading, spacing: 4) {
                    Text(card.title).font(.callout.weight(.semibold)).lineLimit(1)
                    if !card.episodeLine.isEmpty {
                        Text(card.episodeLine).font(.caption).foregroundStyle(.secondary).lineLimit(1)
                    }
                }
                .frame(width: 460, alignment: .leading)
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
            Text(title).font(.title3.weight(.semibold)).padding(.leading, 80)
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
