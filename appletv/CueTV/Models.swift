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

struct LivePlay: Decodable {
    let id: String
    let name: String
    let url: String      // through Cue
    let direct: String?  // the provider's own HLS address
}
