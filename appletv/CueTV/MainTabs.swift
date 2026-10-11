import SwiftUI

// The app once signed in: the tab bar along the top, as in Apple's own TV
// apps, and whatever is playing full screen over it.
@MainActor
struct MainTabs: View {
    @EnvironmentObject var cue: Cue

    var body: some View {
        TabView {
            BrowseStack { HomeView(only: nil) }
                .tabItem { Label("Home", systemImage: "house") }
            BrowseStack { HomeView(only: "tv") }
                .tabItem { Label("Shows", systemImage: "tv") }
            BrowseStack { HomeView(only: "movie") }
                .tabItem { Label("Movies", systemImage: "film") }
            LiveView()
                .tabItem { Label("Live TV", systemImage: "dot.radiowaves.left.and.right") }
            BrowseStack { SearchView() }
                .tabItem { Label("Search", systemImage: "magnifyingglass") }
            BrowseStack { MyListView() }
                .tabItem { Label("My List", systemImage: "plus.square.on.square") }
            ProfileTab()
                .tabItem { Label(cue.profileName.isEmpty ? "Profile" : cue.profileName, systemImage: "person.crop.circle") }
        }
        .fullScreenCover(item: $cue.playing) { r in
            VODPlayerScreen(request: r).environmentObject(cue)
        }
    }
}

@MainActor
struct ProfileTab: View {
    @EnvironmentObject var cue: Cue

    var body: some View {
        VStack(spacing: 40) {
            Circle()
                .fill(cue.accentColor)
                .frame(width: 200, height: 200)
                .overlay(Text(String(cue.profileName.prefix(1))).font(.system(size: 90, weight: .bold)).foregroundStyle(.white))
            Text(cue.profileName).font(.title2.bold())
            Text(cue.server).foregroundStyle(.secondary)
            HStack(spacing: 40) {
                Button("Switch profile") { cue.switchProfile() }
                Button("Sign out") { Task { await cue.signOut() } }
            }
        }
    }
}
