import AVKit
import SwiftUI

// A film or an episode full screen, in the Apple TV's own player. The file
// plays as it is when Apple's player can open it (MP4, or Premiumize's MP4
// copy); otherwise Cue converts it as it plays, keeping the picture and the
// surround (the server's Apple TV mode). It carries on where it was left,
// saves where you are every 15 seconds, and at the end of an episode the
// next one starts.

@MainActor
final class Playback: ObservableObject {
    @Published var player: AVPlayer?
    @Published var problem = ""
    @Published var status = "Finding a version…"

    private weak var cue: Cue?
    private(set) var request: PlayRequest
    private var option = 1
    private var options = 1
    private var offset: Double = 0 // where a conversion started: its time 0
    private var knownDuration: Double = 0
    private var watcher: NSKeyValueObservation?
    private var ticker: Any?
    private var endObserver: NSObjectProtocol?
    private var skips: Skips?
    private weak var controller: AVPlayerViewController?
    private var shownAction = ""

    init(request: PlayRequest) {
        self.request = request
    }

    func start(cue: Cue) async {
        self.cue = cue
        Task { skips = await cue.skips(request) }
        var start = request.start
        if start < 0 {
            start = 0
            if let p = await cue.progress(request), !p.finished, p.position > 30, p.duration - p.position > 60 {
                start = p.position
            }
        }
        await open(at: start)
    }

    // The current version (option), from start seconds.
    private func open(at start: Double) async {
        guard let cue else { return }
        problem = ""
        status = "Finding a version…"
        do {
            let a = try await cue.playAnswer(request, option: option)
            options = max(a.options ?? 1, 1)
            if a.youtube != nil {
                try await next(at: start, why: "That version is a YouTube video, which this app can't play yet.")
                return
            }
            var url: URL?
            var throughCue = false
            if Playback.applePlays(a.url) {
                url = URL(string: a.url)
                offset = 0
            } else if let s = a.streamUrl, let u = URL(string: s) {
                url = u
                offset = 0
            } else if let token = a.convert, !token.isEmpty {
                status = "Getting it ready…"
                let info = try await cue.convertInfo(token)
                knownDuration = info.duration
                var hls = info.hls + "&apple=1"
                if start > 1 { hls += "&t=\(Int(start))" }
                url = cue.url(hls)
                offset = start > 1 ? Double(Int(start)) : 0
                throughCue = true
            }
            guard let url else {
                try await next(at: start, why: "This version can't play on Apple TV.")
                return
            }
            var opts: [String: Any] = [:]
            if throughCue || url.host == URL(string: cue.server)?.host {
                opts[AVURLAssetHTTPCookiesKey] = HTTPCookieStorage.shared.cookies ?? []
                opts["AVURLAssetHTTPHeaderFieldsKey"] = cue.headers
            }
            let item = AVPlayerItem(asset: AVURLAsset(url: url, options: opts))
            item.externalMetadata = metadata()
            watch(item, start: start)
            let p = player ?? AVPlayer()
            p.replaceCurrentItem(with: item)
            if offset == 0 && start > 1 {
                _ = await p.seek(to: CMTime(seconds: start, preferredTimescale: 600))
            }
            if player == nil {
                player = p
                track(p)
            }
            p.play()
        } catch {
            problem = error.localizedDescription
        }
    }

    // The next version, when this one won't play.
    private func next(at start: Double, why: String) async throws {
        if option < options {
            option += 1
            status = "Trying another version…"
            await open(at: start)
        } else {
            throw CueError(message: why)
        }
    }

    private func watch(_ item: AVPlayerItem, start: Double) {
        watcher?.invalidate()
        watcher = item.observe(\.status, options: [.new]) { [weak self] item, _ in
            guard item.status == .failed else { return }
            Task { @MainActor in
                guard let self else { return }
                do {
                    try await self.next(at: start, why: "None of the versions would play on Apple TV.")
                } catch {
                    self.problem = error.localizedDescription
                }
            }
        }
    }

    // Saving where you are, and what happens at the end.
    private func track(_ p: AVPlayer) {
        ticker = p.addPeriodicTimeObserver(forInterval: CMTime(seconds: 1, preferredTimescale: 1), queue: .main) { [weak self] t in
            Task { @MainActor in
                guard let self else { return }
                self.updateActions()
                if t.seconds.isFinite, Int(t.seconds) % 15 == 0 { await self.save() }
            }
        }
        endObserver = NotificationCenter.default.addObserver(forName: .AVPlayerItemDidPlayToEndTime, object: nil, queue: .main) { [weak self] note in
            Task { @MainActor in
                guard let self, let item = note.object as? AVPlayerItem, item === self.player?.currentItem else { return }
                await self.ended()
            }
        }
    }

    var position: Double {
        guard let t = player?.currentTime().seconds, t.isFinite else { return offset }
        return offset + t
    }

    var duration: Double {
        if knownDuration > 0 { return knownDuration }
        if let d = player?.currentItem?.duration.seconds, d.isFinite, d > 0 { return d + offset }
        return 0
    }

    func save() async {
        guard let cue, duration > 0, position > 5 else { return }
        await cue.saveProgress(request, position: position, duration: duration)
    }

    // ---- The player's own controls, with Cue's extras ----

    func configure(_ vc: AVPlayerViewController) {
        controller = vc
        // Speeds in the player's menu (0.5x to 2x).
        vc.speeds = AVPlaybackSpeed.systemDefaultSpeeds
        var items: [UIMenuElement] = []
        items.append(UIAction(title: "Other Version", image: UIImage(systemName: "rectangle.stack")) { [weak self] _ in
            Task { @MainActor in await self?.otherVersion() }
        })
        vc.transportBarCustomMenuItems = items
    }

    // Skip Intro (or Recap) while one plays, Next Episode at the credits.
    private func updateActions() {
        guard let vc = controller else { return }
        let pos = position
        var want = ""
        if let i = skips?.intro, pos >= i.start, pos < i.end - 1 { want = "intro" }
        if let r = skips?.recap, pos >= r.start, pos < r.end - 1 { want = "recap" }
        if request.kind == "tv", duration > 0 {
            let credits = skips?.creditsStart ?? skips?.creditsFromEnd.map { duration - $0 } ?? (duration - 40)
            if pos >= credits { want = "next" }
        }
        guard want != shownAction else { return }
        shownAction = want
        switch want {
        case "intro", "recap":
            let seg = want == "intro" ? skips?.intro : skips?.recap
            vc.contextualActions = [UIAction(title: want == "intro" ? "Skip Intro" : "Skip Recap", image: UIImage(systemName: "forward.fill")) { [weak self] _ in
                guard let self, let end = seg?.end else { return }
                Task { @MainActor in
                    _ = await self.player?.seek(to: CMTime(seconds: max(end - self.offset, 0), preferredTimescale: 600))
                }
            }]
        case "next":
            vc.contextualActions = [UIAction(title: "Next Episode", image: UIImage(systemName: "forward.end.fill")) { [weak self] _ in
                Task { @MainActor in await self?.ended() }
            }]
        default:
            vc.contextualActions = []
        }
    }

    private func otherVersion() async {
        let at = position
        if option < options {
            option += 1
            status = "Trying another version…"
            await open(at: at)
        } else {
            option = 1
            await open(at: at)
        }
    }

    private func ended() async {
        guard let cue else { return }
        let total = duration > 0 ? duration : position
        await cue.saveProgress(request, position: total, duration: total)
        guard request.kind == "tv", let n = try? await cue.next(request.tmdbId, season: request.season, episode: request.episode) else {
            cue.playing = nil
            return
        }
        request = PlayRequest(kind: "tv", tmdbId: request.tmdbId, season: n.season, episode: n.episode,
                              title: request.title, subtitle: "S\(n.season):E\(n.episode) · \(n.title)", start: 0)
        skips = await cue.skips(request)
        shownAction = ""
        controller?.contextualActions = []
        option = 1
        knownDuration = 0
        await open(at: 0)
    }

    func stop() {
        Task { await save() }
        watcher?.invalidate()
        if let t = ticker { player?.removeTimeObserver(t) }
        if let e = endObserver { NotificationCenter.default.removeObserver(e) }
        player?.pause()
    }

    private func metadata() -> [AVMetadataItem] {
        func item(_ id: AVMetadataIdentifier, _ value: String) -> AVMetadataItem {
            let m = AVMutableMetadataItem()
            m.identifier = id
            m.value = value as NSString
            m.extendedLanguageTag = "und"
            return m.copy() as! AVMetadataItem
        }
        var out = [item(.commonIdentifierTitle, request.title)]
        if !request.subtitle.isEmpty { out.append(item(.iTunesMetadataTrackSubTitle, request.subtitle)) }
        return out
    }

    // Files Apple's player opens as they are.
    static func applePlays(_ link: String) -> Bool {
        guard let path = URL(string: link)?.path.lowercased() else { return false }
        return path.hasSuffix(".mp4") || path.hasSuffix(".m4v") || path.hasSuffix(".mov")
    }
}

@MainActor
struct VODPlayerScreen: View {
    @EnvironmentObject var cue: Cue
    @Environment(\.dismiss) private var dismiss
    let request: PlayRequest
    @StateObject private var playback: Playback

    init(request: PlayRequest) {
        self.request = request
        _playback = StateObject(wrappedValue: Playback(request: request))
    }

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            if let p = playback.player {
                SystemPlayer(player: p) { vc in playback.configure(vc) }.ignoresSafeArea()
            }
            if !playback.problem.isEmpty {
                VStack(spacing: 24) {
                    Text(request.title).font(.title2.bold())
                    Text(playback.problem).multilineTextAlignment(.center)
                    Button("Back") { dismiss() }
                }
                .padding(60)
                .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 24))
            } else if playback.player == nil {
                VStack(spacing: 24) {
                    ProgressView()
                    Text(request.title).font(.title3.bold())
                    Text(playback.status).foregroundStyle(.secondary)
                }
            }
        }
        .task { await playback.start(cue: cue) }
        .onDisappear { playback.stop() }
    }
}
