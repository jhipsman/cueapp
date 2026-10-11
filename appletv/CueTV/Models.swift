import Foundation

// What Cue sends, as in Watch's own pages (web/src/api.ts).

struct Theme: Decodable {
    let accent: String?
}

struct Profile: Decodable, Identifiable {
    let id: Int
    let name: String
    let avatar: String
    let hasPin: Bool
    let theme: Theme?
}

struct Profiles: Decodable {
    let profiles: [Profile]
    let active: Int?
    let liveTV: Bool?
}

struct Programme: Decodable, Hashable {
    let title: String
    let desc: String?
    let start: String
    let stop: String

    var startDate: Date? { Programme.date(start) }
    var stopDate: Date? { Programme.date(stop) }

    // How far through it is, 0 to 1.
    func progress(at now: Date = Date()) -> Double {
        guard let a = startDate, let b = stopDate, b > a else { return 0 }
        return min(1, max(0, now.timeIntervalSince(a) / b.timeIntervalSince(a)))
    }

    var times: String {
        guard let a = startDate, let b = stopDate else { return "" }
        return "\(Programme.clock.string(from: a)) – \(Programme.clock.string(from: b))"
    }

    private static let plain = ISO8601DateFormatter()
    private static let fractional: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f
    }()
    static let clock: DateFormatter = {
        let f = DateFormatter()
        f.timeStyle = .short
        f.dateStyle = .none
        return f
    }()

    static func date(_ s: String) -> Date? { plain.date(from: s) ?? fractional.date(from: s) }
}

struct Channel: Decodable, Identifiable, Hashable {
    let id: String
    let num: Int
    let name: String
    let category: String
    let logo: String?
    var favorite: Bool
    let now: Programme?
    let next: Programme?
}

struct Category: Decodable, Hashable {
    let id: String
    let name: String
}

struct LiveChannels: Decodable {
    let configured: Bool
    let categories: [Category]?
    let channels: [Channel]?
    let guideLoading: Bool?
}

struct LiveGuide: Decodable {
    let programmes: [String: [Programme]]
}

struct LivePlay: Decodable {
    let id: String
    let name: String
    let url: String      // through Cue
    let direct: String?  // the provider's own HLS address
}

// ---- Movies and shows ----

struct WatchCard: Decodable, Hashable, Identifiable {
    let kind: String // "movie" or "tv"
    let tmdbId: Int
    let title: String
    let year: Int?
    let overview: String?
    let posterUrl: String?
    let backdropUrl: String?
    let rating: Double?
    let progress: Double? // 0 to 1, on Continue Watching
    let season: Int?
    let episode: Int?
    let episodeTitle: String?
    let isNew: Bool?

    enum CodingKeys: String, CodingKey {
        case kind, tmdbId, title, year, overview, posterUrl, backdropUrl, rating, progress, season, episode, episodeTitle
        case isNew = "new"
    }

    var id: String { "\(kind)-\(tmdbId)-\(season ?? 0)-\(episode ?? 0)" }
    // A title being watched: playing it carries on.
    var resumes: Bool { progress != nil || episode != nil }
    var episodeLine: String {
        guard let s = season, let e = episode else { return "" }
        return "S\(s):E\(e)" + (episodeTitle.map { " · \($0)" } ?? "")
    }
}

struct WatchRow: Decodable, Identifiable {
    let key: String
    let title: String
    let items: [WatchCard]
    var id: String { key }
}

struct WatchHome: Decodable {
    let hero: WatchCard?
    let rows: [WatchRow]
}

struct SeasonInfo: Decodable, Hashable {
    let number: Int
    let name: String
    let episodes: Int
}

struct WatchTitle: Decodable {
    let kind: String
    let tmdbId: Int
    let title: String
    let year: Int?
    let overview: String?
    let posterUrl: String?
    let backdropUrl: String?
    let rating: Double?
    let tagline: String?
    let runtime: Int?
    let genres: [String]?
    let certification: String?
    let seasons: [SeasonInfo]?
    let inList: Bool?
    let resumeSeason: Int?
    let resumeEpisode: Int?
    let resumePosition: Double?
    let more: [WatchCard]?
}

struct WatchEpisode: Decodable, Hashable, Identifiable {
    let season: Int
    let episode: Int
    let title: String
    let overview: String?
    let runtime: Int?
    let stillUrl: String?
    let aired: Bool?
    let progress: Double?
    let finished: Bool?
    var id: String { "\(season)-\(episode)" }
}

struct WatchProgress: Decodable {
    let position: Double
    let duration: Double
    let finished: Bool
}

struct PlayAnswer: Decodable {
    let title: String?
    let url: String
    let streamUrl: String?
    let release: String?
    let quality: String?
    let option: Int?
    let options: Int?
    let youtube: String?
    let convert: String?
}

struct ConvertInfo: Decodable {
    let duration: Double
    let hls: String
}

// A film or episode to play. start < 0: where it was left off, if anywhere.
struct PlayRequest: Identifiable, Hashable {
    let id = UUID()
    let kind: String
    let tmdbId: Int
    let season: Int
    let episode: Int
    let title: String
    let subtitle: String
    let start: Double
}
