import SwiftUI

// Browse: a page per streaming service (its rows), collections across
// services, and the big film series in order.

struct BrowseHomeData: Decodable {
    struct Service: Decodable, Hashable, Identifiable {
        let key: String
        let name: String
        let logo: String?
        var id: String { key }
    }
    struct Collection: Decodable, Hashable, Identifiable {
        let key: String
        let title: String
        let blurb: String
        var id: String { key }
    }
    struct Franchise: Decodable, Hashable, Identifiable {
        let id: Int
        let name: String
        let poster: String?
    }
    let services: [Service]
    let collections: [Collection]
    let franchises: [Franchise]
}

struct BrowseServiceData: Decodable {
    struct Row: Decodable, Identifiable {
        let key: String
        let title: String
        let items: [WatchCard]
        var id: String { key }
    }
    let name: String
    let logo: String?
    let rows: [Row]
}

struct BrowseListData: Decodable {
    let items: [WatchCard]
}

// Where Browse can go: a service, a collection or a film series.
enum BrowseTarget: Hashable {
    case service(BrowseHomeData.Service)
    case collection(BrowseHomeData.Collection)
    case franchise(BrowseHomeData.Franchise)
}

@MainActor
struct CollectionsView: View {
    @EnvironmentObject var cue: Cue
    @State private var home: BrowseHomeData?
    @State private var problem = ""

    var body: some View {
        Group {
            if let home {
                ScrollView(.vertical) {
                    VStack(alignment: .leading, spacing: 50) {
                        shelf("Streaming services") {
                            ForEach(home.services) { s in
                                NavigationLink(value: BrowseTarget.service(s)) {
                                    VStack(spacing: 14) {
                                        Artwork(url: s.logo, label: s.name)
                                            .frame(width: 110, height: 110)
                                            .clipShape(RoundedRectangle(cornerRadius: 22))
                                        Text(s.name).font(.caption.weight(.semibold)).lineLimit(1)
                                    }
                                    .frame(width: 220, height: 200)
                                }
                                .buttonStyle(.card)
                            }
                        }
                        shelf("Collections") {
                            ForEach(home.collections) { c in
                                NavigationLink(value: BrowseTarget.collection(c)) {
                                    VStack(alignment: .leading, spacing: 6) {
                                        Spacer()
                                        Text(c.title).font(.title3.bold()).lineLimit(1)
                                        Text(c.blurb).font(.caption).foregroundStyle(.white.opacity(0.8)).lineLimit(2)
                                    }
                                    .padding(24)
                                    .frame(width: 420, height: 200, alignment: .leading)
                                    .background(LinearGradient(colors: collectionColors(c.key), startPoint: .topLeading, endPoint: .bottomTrailing))
                                }
                                .buttonStyle(.card)
                            }
                        }
                        shelf("Film series") {
                            ForEach(home.franchises) { f in
                                VStack(alignment: .leading, spacing: 12) {
                                    NavigationLink(value: BrowseTarget.franchise(f)) {
                                        Artwork(url: f.poster, label: f.name).frame(width: 250, height: 375).clipped()
                                    }
                                    .buttonStyle(.card)
                                    Text(f.name).font(.caption).foregroundStyle(.secondary).lineLimit(1).frame(width: 250, alignment: .leading)
                                }
                            }
                        }
                    }
                    .padding(.vertical, 40)
                }
            } else {
                Text(problem.isEmpty ? "Loading…" : problem).foregroundStyle(.secondary)
            }
        }
        .navigationDestination(for: BrowseTarget.self) { t in
            switch t {
            case .service(let s): ServiceBrowseView(service: s)
            case .collection(let c): BrowseGridView(title: c.title, blurb: c.blurb, path: "/api/watch/browse/collection/\(c.key)")
            case .franchise(let f): BrowseGridView(title: f.name, blurb: "In the order they came out.", path: "/api/watch/browse/franchise/\(f.id)")
            }
        }
        .task {
            do { home = try await cue.get("/api/watch/browse") } catch { problem = error.localizedDescription }
        }
    }

    @ViewBuilder
    private func shelf<Content: View>(_ title: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 18) {
            Text(title).font(.title3.weight(.semibold)).padding(.leading, 80)
            ScrollView(.horizontal, showsIndicators: false) {
                LazyHStack(alignment: .top, spacing: 44) { content() }
                    .padding(.horizontal, 80)
                    .padding(.vertical, 30)
            }
            .scrollClipDisabled()
        }
    }

    private func collectionColors(_ key: String) -> [Color] {
        switch key {
        case "holiday": return [Color(red: 0.70, green: 0.15, blue: 0.12), Color(red: 0.23, green: 0.05, blue: 0.04)]
        case "halloween": return [Color(red: 0.91, green: 0.42, blue: 0.10), Color(red: 0.23, green: 0.09, blue: 0.02)]
        case "acclaimed": return [Color(red: 0.79, green: 0.64, blue: 0.15), Color(red: 0.24, green: 0.18, blue: 0.02)]
        case "family": return [Color(red: 0.19, green: 0.64, blue: 0.42), Color(red: 0.05, green: 0.20, blue: 0.13)]
        case "short": return [Color(red: 0.00, green: 0.57, blue: 1.00), Color(red: 0.02, green: 0.16, blue: 0.30)]
        case "gems": return [Color(red: 0.56, green: 0.31, blue: 0.78), Color(red: 0.16, green: 0.07, blue: 0.25)]
        default: return [Color(white: 0.3), Color(white: 0.09)]
        }
    }
}

// A streaming service's page: its rows, as on Home.
@MainActor
struct ServiceBrowseView: View {
    @EnvironmentObject var cue: Cue
    @EnvironmentObject var nav: Nav
    let service: BrowseHomeData.Service
    @State private var page: BrowseServiceData?
    @State private var problem = ""

    var body: some View {
        ScrollView(.vertical) {
            LazyVStack(alignment: .leading, spacing: 30) {
                Text(service.name).font(.title.bold()).padding(.leading, 80).padding(.top, 30)
                if let page {
                    ForEach(page.rows) { r in
                        CardRow(title: r.title, items: r.items) { c in openCard(c, cue: cue, nav: nav) }
                    }
                } else {
                    Text(problem.isEmpty ? "Loading…" : problem).foregroundStyle(.secondary).padding(.leading, 80)
                }
            }
        }
        .task {
            do { page = try await cue.get("/api/watch/browse/service/\(service.key)") } catch { problem = error.localizedDescription }
        }
    }
}

// A collection or film series: its titles in a grid.
@MainActor
struct BrowseGridView: View {
    @EnvironmentObject var cue: Cue
    @EnvironmentObject var nav: Nav
    let title: String
    let blurb: String
    let path: String
    @State private var items: [WatchCard]?
    @State private var problem = ""

    var body: some View {
        ScrollView(.vertical) {
            VStack(alignment: .leading, spacing: 10) {
                Text(title).font(.title.bold())
                Text(blurb).foregroundStyle(.secondary)
                if let items {
                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 250), spacing: 44)], alignment: .leading, spacing: 50) {
                        ForEach(items) { c in
                            PosterCard(card: c) { openCard($0, cue: cue, nav: nav) }
                        }
                    }
                    .padding(.top, 30)
                } else {
                    Text(problem.isEmpty ? "Loading…" : problem).foregroundStyle(.secondary)
                }
            }
            .padding(.horizontal, 80)
            .padding(.vertical, 30)
        }
        .task {
            do {
                let list: BrowseListData = try await cue.get(path)
                items = list.items
            } catch { problem = error.localizedDescription }
        }
    }
}
