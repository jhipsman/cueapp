import AVKit
import SwiftUI

// A channel full screen, in the Apple TV's own player: the provider's HLS
// stream first (straight from the provider, as Strand and the TV's apps
// play it), then the same stream through Cue if that fails. Swipe down for
// what's on and the sound and subtitle choices; Back returns to the list.

@MainActor
struct PlayerScreen: View {
    @EnvironmentObject var cue: Cue
    @Environment(\.dismiss) private var dismiss
    let channel: Channel
    @State private var player: AVPlayer?
    @State private var problem = ""
    @State private var watcher: NSKeyValueObservation?

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            if let player {
                SystemPlayer(player: player).ignoresSafeArea()
            }
            if !problem.isEmpty {
                VStack(spacing: 24) {
                    Text(channel.name).font(.title2.bold())
                    Text(problem).multilineTextAlignment(.center)
                    Button("Back") { dismiss() }
                }
                .padding(60)
                .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 24))
            } else if player == nil {
                ProgressView()
            }
        }
        .task { await start() }
        .onDisappear {
            watcher?.invalidate()
            player?.pause()
            player = nil
        }
    }

    private func start() async {
        do {
            let play = try await cue.play(channel.id)
            var tries: [URL] = []
            if let d = play.direct, let u = URL(string: d) { tries.append(u) }
            if let u = cue.url(play.url) { tries.append(u) }
            attempt(tries)
        } catch {
            problem = error.localizedDescription
        }
    }

    // Plays the first address; when it fails, the next.
    private func attempt(_ tries: [URL]) {
        guard let first = tries.first else {
            problem = "This channel won't play right now."
            return
        }
        let rest = Array(tries.dropFirst())
        var options: [String: Any] = [:]
        if first.host == URL(string: cue.server)?.host {
            // Through Cue: signed in, and on this profile.
            options[AVURLAssetHTTPCookiesKey] = HTTPCookieStorage.shared.cookies ?? []
            options["AVURLAssetHTTPHeaderFieldsKey"] = cue.headers
        }
        let item = AVPlayerItem(asset: AVURLAsset(url: first, options: options))
        item.externalMetadata = metadata()
        watcher?.invalidate()
        watcher = item.observe(\.status, options: [.new]) { item, _ in
            guard item.status == .failed else { return }
            Task { @MainActor in attempt(rest) }
        }
        if let player {
            player.replaceCurrentItem(with: item)
        } else {
            let p = AVPlayer(playerItem: item)
            player = p
        }
        player?.play()
    }

    // What the player's info panel shows.
    private func metadata() -> [AVMetadataItem] {
        func item(_ id: AVMetadataIdentifier, _ value: String) -> AVMetadataItem {
            let m = AVMutableMetadataItem()
            m.identifier = id
            m.value = value as NSString
            m.extendedLanguageTag = "und"
            return m.copy() as! AVMetadataItem
        }
        let title = channel.num > 0 ? "\(channel.num)  \(channel.name)" : channel.name
        var out = [item(.commonIdentifierTitle, title)]
        if let now = channel.now {
            out.append(item(.iTunesMetadataTrackSubTitle, "\(now.title)  ·  \(now.times)"))
            if let d = now.desc, !d.isEmpty { out.append(item(.commonIdentifierDescription, d)) }
        }
        return out
    }
}

@MainActor
struct SystemPlayer: UIViewControllerRepresentable {
    let player: AVPlayer

    func makeUIViewController(context: Context) -> AVPlayerViewController {
        let vc = AVPlayerViewController()
        vc.player = player
        // Live: the picture's frame rate and range on the TV, as Strand does
        // when "Match Frame Rate" is on in the Apple TV's settings.
        vc.appliesPreferredDisplayCriteriaAutomatically = true
        return vc
    }

    func updateUIViewController(_ vc: AVPlayerViewController, context: Context) {
        if vc.player !== player { vc.player = player }
    }
}
