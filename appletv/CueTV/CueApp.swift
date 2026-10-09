import SwiftUI

// Cue on Apple TV: Live TV from Cue's IPTV provider, played by the Apple
// TV's own player. Sign in, pick a profile, pick a channel.

@main
@MainActor
struct CueApp: App {
    @StateObject private var cue = Cue()

    var body: some Scene {
        WindowGroup {
            Root()
                .environmentObject(cue)
                .preferredColorScheme(.dark)
                .tint(cue.accentColor)
        }
    }
}

@MainActor
struct Root: View {
    @EnvironmentObject var cue: Cue

    var body: some View {
        ZStack {
            Color(red: 0.04, green: 0.05, blue: 0.07).ignoresSafeArea()
            switch cue.stage {
            case .starting:
                ProgressView().task { await cue.start() }
            case .signIn:
                SignInView()
            case .profiles:
                ProfilesView()
            case .live:
                LiveView()
            }
        }
    }
}

extension Cue {
    // The profile's color, or Cue's teal.
    var accentColor: Color { Color(hex: accent) ?? Color(red: 0.18, green: 0.83, blue: 0.75) }
}

extension Color {
    init?(hex: String) {
        var s = hex.trimmingCharacters(in: .whitespaces)
        if s.hasPrefix("#") { s.removeFirst() }
        guard s.count == 6, let v = UInt32(s, radix: 16) else { return nil }
        self.init(red: Double((v >> 16) & 0xFF) / 255, green: Double((v >> 8) & 0xFF) / 255, blue: Double(v & 0xFF) / 255)
    }
}
