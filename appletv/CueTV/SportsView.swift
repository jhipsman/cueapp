import SwiftUI

// Sports: the next big game on top, then games on now and coming up, with
// the teams' logos and the live score or kickoff time (from ESPN, through
// Cue). It needs no TV provider; with Live TV set up, picking a game plays
// the channel it's on.

struct SportsTeam: Decodable, Hashable {
    let id: String
    let name: String
    let short: String?
    let logo: String?
    let score: String?
    let record: String?
}

struct SportsGame: Decodable, Identifiable, Hashable {
    let id: String
    let league: String
    let leagueName: String
    let start: String
    let state: String
    let detail: String
    let home: SportsTeam
    let away: SportsTeam
    let note: String?
    let networks: [String]?
    let channel: Channel?
    let streaming: String?
    let favorite: Bool?
    var key: String { league + "-" + id }
    var live: Bool { state == "in" }
    var whereOn: String {
        if let c = channel { return c.name }
        if let s = streaming { return "On \(s)" }
        if let n = networks?.first { return "On \(n)" }
        return ""
    }
    var when: String {
        if live { return detail }
        let iso = ISO8601DateFormatter()
        guard let d = iso.date(from: start) else { return detail }
        let f = DateFormatter()
        f.dateFormat = Calendar.current.isDateInToday(d) ? "'Today' h:mm a" : (Calendar.current.isDateInTomorrow(d) ? "'Tomorrow' h:mm a" : "EEE h:mm a")
        return f.string(from: d)
    }
}

struct SportsBoard: Decodable {
    let source: String?
    let now: [SportsGame]?
    let soon: [SportsGame]?
    let big: SportsGame?
}

@MainActor
struct SportsView: View {
    @EnvironmentObject var cue: Cue
    @State private var board: SportsBoard?
    @State private var problem = ""
    @State private var playing: Channel?
    @State private var note = ""

    var body: some View {
        Group {
            if let b = board, b.source == "espn" {
                ScrollView {
                    VStack(alignment: .leading, spacing: 40) {
                        if let g = b.big { BigGame(game: g) { watch(g) } }
                        if !note.isEmpty { Text(note).foregroundStyle(.secondary) }
                        row("On now", (b.now ?? []).filter { $0.key != b.big?.key })
                        row("Coming up", (b.soon ?? []).filter { $0.key != b.big?.key })
                        Text("Schedules and scores from ESPN.").font(.caption2).foregroundStyle(.tertiary)
                    }
                    .padding(60)
                }
            } else if board != nil {
                Text("No games on now or in the next day.").foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if !problem.isEmpty {
                Text(problem).foregroundStyle(.secondary).frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .background(Color.cueBackground.ignoresSafeArea())
        .task {
            // Scores change: read again every minute while the tab is open.
            while !Task.isCancelled {
                do { board = try await cue.get("/api/live/sports") } catch { if board == nil { problem = "Couldn't load the games." } }
                try? await Task.sleep(nanoseconds: 60_000_000_000)
            }
        }
        .fullScreenCover(item: $playing) { ch in
            PlayerScreen(channel: ch).environmentObject(cue)
        }
    }

    private func watch(_ g: SportsGame) {
        if let ch = g.channel { playing = ch; return }
        note = g.whereOn.isEmpty ? "This game isn't on any of your channels." : "\(g.away.short ?? g.away.name) at \(g.home.short ?? g.home.name) is \(g.whereOn.lowercased())."
    }

    @ViewBuilder private func row(_ title: String, _ games: [SportsGame]) -> some View {
        if !games.isEmpty {
            VStack(alignment: .leading, spacing: 16) {
                Text(title.uppercased()).font(.caption.bold()).foregroundStyle(.secondary)
                ScrollView(.horizontal) {
                    LazyHStack(spacing: 30) {
                        ForEach(games) { g in GameTile(game: g) { watch(g) } }
                    }
                    .padding(.vertical, 20)
                }
                .scrollClipDisabled()
            }
        }
    }
}

private struct TeamLogo: View {
    let team: SportsTeam
    let size: CGFloat
    var body: some View {
        Group {
            if let s = team.logo, let u = URL(string: s) {
                AsyncImage(url: u) { img in img.resizable().scaledToFit() } placeholder: { Color.clear }
            } else {
                Text(String((team.short ?? team.name).prefix(3)).uppercased()).font(.caption.bold())
            }
        }
        .frame(width: size, height: size)
    }
}

private struct SideRow: View {
    let team: SportsTeam
    let big: Bool
    var body: some View {
        HStack(spacing: big ? 20 : 12) {
            TeamLogo(team: team, size: big ? 80 : 40)
            VStack(alignment: .leading, spacing: 2) {
                Text(big ? team.name : (team.short ?? team.name)).font(big ? .title3.bold() : .callout.bold()).lineLimit(1)
                if let r = team.record, !r.isEmpty { Text(r).font(.caption2).foregroundStyle(.secondary) }
            }
            Spacer(minLength: 8)
            if let s = team.score, !s.isEmpty { Text(s).font(big ? .largeTitle.weight(.black) : .title3.weight(.heavy)).monospacedDigit() }
        }
    }
}

private struct BigGame: View {
    let game: SportsGame
    let action: () -> Void
    var body: some View {
        Button(action: action) {
            VStack(alignment: .leading, spacing: 18) {
                Text((game.live ? "● LIVE" : (game.favorite == true ? "★ YOUR TEAM" : "NEXT BIG GAME")) + " · " + game.leagueName + (game.note.map { " · \($0)" } ?? ""))
                    .font(.caption.bold()).foregroundStyle(game.live ? Color.red : Color.accentColor)
                HStack(spacing: 40) {
                    SideRow(team: game.away, big: true)
                    Text(game.live ? "–" : "at").font(.title3.bold()).foregroundStyle(.secondary)
                    SideRow(team: game.home, big: true)
                }
                Text([game.when, game.whereOn].filter { !$0.isEmpty }.joined(separator: " · "))
                    .foregroundStyle(game.live ? Color.red : .secondary)
            }
            .padding(36)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .buttonStyle(.card)
    }
}

private struct GameTile: View {
    let game: SportsGame
    let action: () -> Void
    var body: some View {
        Button(action: action) {
            VStack(alignment: .leading, spacing: 10) {
                Text((game.favorite == true ? "★ " : "") + game.leagueName).font(.caption2.bold()).foregroundStyle(.secondary)
                SideRow(team: game.away, big: false)
                SideRow(team: game.home, big: false)
                Text(game.when).font(.caption).foregroundStyle(game.live ? Color.red : .secondary)
                if !game.whereOn.isEmpty { Text(game.whereOn).font(.caption2).foregroundStyle(.secondary).lineLimit(1) }
            }
            .padding(24)
            .frame(width: 420, alignment: .leading)
        }
        .buttonStyle(.card)
    }
}
