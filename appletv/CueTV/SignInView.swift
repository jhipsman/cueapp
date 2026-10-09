import SwiftUI

@MainActor
struct SignInView: View {
    @EnvironmentObject var cue: Cue
    @State private var server = ""
    @State private var username = ""
    @State private var password = ""
    @State private var busy = false
    @State private var problem = ""

    var body: some View {
        VStack(spacing: 28) {
            Text("Cue")
                .font(.system(size: 76, weight: .heavy))
                .foregroundStyle(cue.accentColor)
            Text("Sign in with your Cue account.")
                .foregroundStyle(.secondary)
            VStack(spacing: 18) {
                TextField("Server (like cuetv.me)", text: $server)
                    .textContentType(.URL)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                TextField("Username", text: $username)
                    .textContentType(.username)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                SecureField("Password", text: $password)
                    .textContentType(.password)
            }
            .frame(width: 760)
            if !problem.isEmpty {
                Text(problem).foregroundStyle(.red)
            }
            Button(busy ? "Signing in…" : "Sign in") {
                Task { await signIn() }
            }
            .disabled(busy || server.isEmpty || username.isEmpty || password.isEmpty)
        }
        .onAppear {
            if server.isEmpty { server = cue.server.isEmpty ? "cuetv.me" : cue.server }
        }
    }

    private func signIn() async {
        busy = true
        problem = ""
        defer { busy = false }
        do {
            try await cue.signIn(server: server, username: username, password: password)
        } catch {
            problem = error.localizedDescription
        }
    }
}
