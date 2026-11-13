import SwiftUI

@MainActor
struct ProfilesView: View {
    @EnvironmentObject var cue: Cue
    @State private var list: [Profile] = []
    @State private var problem = ""
    @State private var askingPin: Profile?
    @State private var pin = ""

    var body: some View {
        VStack(spacing: 48) {
            Text("Who's watching?")
                .font(.system(size: 56, weight: .bold))
            if list.isEmpty && problem.isEmpty {
                ProgressView()
            }
            HStack(spacing: 48) {
                ForEach(list) { p in
                    Button {
                        if p.hasPin {
                            pin = ""
                            askingPin = p
                        } else {
                            Task { await choose(p) }
                        }
                    } label: {
                        VStack(spacing: 16) {
                            Circle()
                                .fill(Color(hex: p.theme?.accent ?? "") ?? avatarColor(p.avatar))
                                .frame(width: 180, height: 180)
                                .overlay(face(p))
                            Text(p.name).font(.title3)
                        }
                        .padding(20)
                    }
                    .buttonStyle(.card)
                }
            }
            if !problem.isEmpty {
                Text(problem).foregroundStyle(.red)
                Button("Try again") { Task { await load() } }
            }
            Button("Sign out") { Task { await cue.signOut() } }
                .padding(.top, 40)
        }
        .task { await load() }
        .alert("PIN for \(askingPin?.name ?? "")", isPresented: Binding(get: { askingPin != nil }, set: { if !$0 { askingPin = nil } })) {
            SecureField("PIN", text: $pin)
                .keyboardType(.numberPad)
            Button("OK") {
                if let p = askingPin { Task { await choose(p, pin: pin) } }
            }
            Button("Cancel", role: .cancel) {}
        }
    }

    private func load() async {
        problem = ""
        do {
            let all = try await cue.profiles()
            list = all.profiles
            if all.liveTV == false {
                problem = "Live TV isn't set up on this Cue yet: add your IPTV provider in Cue's settings."
            }
            // One profile without a PIN: straight in.
            if list.count == 1, let only = list.first, !only.hasPin {
                await choose(only)
            }
        } catch {
            problem = error.localizedDescription
        }
    }

    private func choose(_ p: Profile, pin: String = "") async {
        do {
            try await cue.choose(p, pin: pin)
        } catch {
            problem = error.localizedDescription
        }
    }

    // A profile's picture (an avatar is "teal" or "teal/fox"), or its initial.
    private static let pictures: [String: String] = [
        "fox": "🦊", "cat": "🐱", "dog": "🐶", "panda": "🐼", "koala": "🐨", "lion": "🦁", "frog": "🐸", "monkey": "🐵",
        "unicorn": "🦄", "octopus": "🐙", "penguin": "🐧", "owl": "🦉", "rocket": "🚀", "star": "⭐", "game": "🎮",
        "ball": "⚽", "pizza": "🍕", "alien": "👾",
    ]

    @ViewBuilder
    private func face(_ p: Profile) -> some View {
        let parts = p.avatar.split(separator: "/", maxSplits: 1).map(String.init)
        if parts.count == 2, let pic = Self.pictures[parts[1]] {
            Text(pic).font(.system(size: 96))
        } else {
            Text(String(p.name.prefix(1))).font(.system(size: 80, weight: .bold)).foregroundStyle(.white)
        }
    }

    private func avatarColor(_ avatar: String) -> Color {
        switch avatar.split(separator: "/").first.map(String.init) ?? "" {
        case "red": return .red
        case "orange": return .orange
        case "yellow": return .yellow
        case "green": return .green
        case "blue": return .blue
        case "purple": return .purple
        case "pink": return .pink
        default: return .teal
        }
    }
}
