import Foundation

// Talking to Cue: the server's address, signing in (a cookie, kept by the
// system), and the profile this Apple TV is on (sent with every request,
// as Watch in a browser does).

struct CueError: LocalizedError {
    let message: String
    var errorDescription: String? { message }
}

@MainActor
final class Cue: ObservableObject {
    enum Stage { case starting, signIn, profiles, live }

    @Published var stage: Stage = .starting
    @Published var server: String
    @Published var profileName: String
    @Published var accent: String
    private(set) var profileToken: String

    static let userAgent = "CueAppleTV/1"
    private let defaults = UserDefaults.standard

    init() {
        let saved = UserDefaults.standard
        server = saved.string(forKey: "server") ?? ""
        profileToken = saved.string(forKey: "profileToken") ?? ""
        profileName = saved.string(forKey: "profileName") ?? ""
        accent = saved.string(forKey: "accent") ?? ""
    }

    // On opening: signed in already, and on a profile?
    func start() async {
        guard !server.isEmpty else { stage = .signIn; return }
        do {
            let _: Me = try await get("/api/auth/me")
            stage = profileToken.isEmpty ? .profiles : .live
        } catch {
            stage = .signIn
        }
    }

    // "cuetv.me" becomes "https://cuetv.me"; no slash at the end.
    static func tidy(_ address: String) -> String {
        var s = address.trimmingCharacters(in: .whitespacesAndNewlines)
        if !s.lowercased().hasPrefix("http://") && !s.lowercased().hasPrefix("https://") {
            s = "https://" + s
        }
        while s.hasSuffix("/") { s.removeLast() }
        return s
    }

    func signIn(server address: String, username: String, password: String) async throws {
        server = Cue.tidy(address)
        defaults.set(server, forKey: "server")
        let _: Me = try await send("POST", "/api/auth/login", body: ["username": username, "password": password])
        stage = .profiles
    }

    func profiles() async throws -> Profiles { try await get("/api/profiles") }

    func choose(_ p: Profile, pin: String = "") async throws {
        let picked: Picked = try await send("POST", "/api/profiles/\(p.id)/select", body: ["pin": pin])
        profileToken = picked.token
        profileName = picked.profile.name
        accent = picked.profile.theme?.accent ?? ""
        defaults.set(profileToken, forKey: "profileToken")
        defaults.set(profileName, forKey: "profileName")
        defaults.set(accent, forKey: "accent")
        stage = .live
    }

    func switchProfile() {
        profileToken = ""
        defaults.removeObject(forKey: "profileToken")
        stage = .profiles
    }

    func signOut() async {
        let _: Empty? = try? await send("POST", "/api/auth/logout", body: [:])
        HTTPCookieStorage.shared.cookies?.forEach { HTTPCookieStorage.shared.deleteCookie($0) }
        switchProfile()
        stage = .signIn
    }

    // ---- Live TV ----

    func channels() async throws -> LiveChannels { try await get("/api/live/channels") }

    func play(_ id: String) async throws -> LivePlay { try await get("/api/live/play/\(escape(id))") }

    func setFavorite(_ id: String, _ on: Bool) async throws {
        let _: Empty? = try await send(on ? "PUT" : "DELETE", "/api/live/favorites/\(escape(id))", body: nil)
    }

    // ---- Requests ----

    func url(_ path: String) -> URL? { URL(string: server + path) }

    // Headers for anything fetched from Cue (pictures, the player).
    var headers: [String: String] {
        var h = ["User-Agent": Cue.userAgent]
        if !profileToken.isEmpty { h["X-Cue-Profile"] = profileToken }
        return h
    }

    private func escape(_ s: String) -> String {
        s.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed.subtracting(CharacterSet(charactersIn: "/"))) ?? s
    }

    func get<T: Decodable>(_ path: String) async throws -> T { try await send("GET", path, body: nil) }

    func send<T: Decodable>(_ method: String, _ path: String, body: [String: String]?) async throws -> T {
        guard let u = url(path) else { throw CueError(message: "That server address doesn't look right.") }
        var req = URLRequest(url: u, timeoutInterval: 30)
        req.httpMethod = method
        headers.forEach { req.setValue($1, forHTTPHeaderField: $0) }
        if let body {
            req.setValue("application/json", forHTTPHeaderField: "Content-Type")
            req.httpBody = try JSONSerialization.data(withJSONObject: body)
        }
        let data: Data
        let resp: URLResponse
        do {
            (data, resp) = try await URLSession.shared.data(for: req)
        } catch {
            throw CueError(message: "Couldn't reach Cue at \(server).")
        }
        let code = (resp as? HTTPURLResponse)?.statusCode ?? 0
        if code == 401 && path != "/api/auth/login" {
            stage = .signIn
        }
        if code == 403, path != "/api/profiles", (try? JSONDecoder().decode(Problem.self, from: data))?.error.lowercased().contains("profile") == true {
            switchProfile()
        }
        guard (200..<300).contains(code) else {
            let msg = (try? JSONDecoder().decode(Problem.self, from: data))?.error
            throw CueError(message: msg ?? "Cue answered \(code).")
        }
        if T.self == Empty?.self || data.isEmpty || data == Data("null".utf8) {
            if let none = Empty?.none as? T { return none }
        }
        do {
            return try JSONDecoder().decode(T.self, from: data)
        } catch {
            throw CueError(message: "Couldn't read Cue's answer.")
        }
    }
}

struct Empty: Decodable {}
struct Problem: Decodable { let error: String }
struct Me: Decodable { let username: String? }
struct Picked: Decodable {
    let token: String
    let profile: Profile
}
