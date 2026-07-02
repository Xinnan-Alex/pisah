import SwiftUI

// MARK: - Palette / fonts
// ponytail: Bricolage Grotesque -> system rounded, Figtree -> system default.
// To use the real fonts: drop the .ttf files in this folder, add UIAppFonts to
// Info.plist (GENERATE_INFOPLIST_FILE -> INFOPLIST_KEY) and swap F.d / F.t below.
extension Color {
    init(hex: UInt) {
        self.init(.sRGB,
                  red: Double((hex >> 16) & 0xff) / 255,
                  green: Double((hex >> 8) & 0xff) / 255,
                  blue: Double(hex & 0xff) / 255,
                  opacity: 1)
    }
}

enum P {
    static let ink = Color(hex: 0x221B14)
    static let orange = Color(hex: 0xF25C3B)
    static let brown = Color(hex: 0x6B5E50)
    static let mut = Color(hex: 0x9A8B79)
    static let cream = Color(hex: 0xF6F1EA)
    static let cream2 = Color(hex: 0xF3ECE2)
    static let paper = Color(hex: 0xFBF7F2)
    static let green = Color(hex: 0x18A07A)
    static let purple = Color(hex: 0x7C5CFF)
    static let amber = Color(hex: 0xE8A02D)
    static let line = Color(hex: 0xF1EAE0)
    static let peach = Color(hex: 0xFDEAE3)
    static let border = Color(hex: 0xECE3D6)
}

enum F {
    static func d(_ s: CGFloat, _ w: Font.Weight = .heavy) -> Font { .system(size: s, weight: w, design: .rounded) }
    static func t(_ s: CGFloat, _ w: Font.Weight = .regular) -> Font { .system(size: s, weight: w) }
}

extension View {
    func card(_ r: CGFloat = 18) -> some View {
        self.background(Color.white)
            .clipShape(RoundedRectangle(cornerRadius: r, style: .continuous))
            .shadow(color: P.ink.opacity(0.05), radius: 7, y: 4)
    }
}

// MARK: - Screen flow
enum Screen { case signIn, capture, scanning, review, share, track, settings, fLanding, fPick, fShare, fPay, fDone }

private let screenNames: [String: Screen] = [
    "signIn": .signIn, "capture": .capture, "scanning": .scanning, "review": .review, "share": .share,
    "track": .track, "settings": .settings, "fLanding": .fLanding, "fPick": .fPick,
    "fShare": .fShare, "fPay": .fPay, "fDone": .fDone
]

@MainActor
final class AppState: ObservableObject {
    // Live when a backend URL is configured; otherwise the offline clickthrough prototype.
    let client: APIClient? = Config.apiBaseURL.map { APIClient(baseURL: $0) }
    var live: Bool { client != nil }

    // ponytail: launch-arg override only for screenshot verification (`-screen review`). Harmless in prod.
    @Published var screen: Screen = {
        if let i = CommandLine.arguments.firstIndex(of: "-screen"),
           i + 1 < CommandLine.arguments.count,
           let s = screenNames[CommandLine.arguments[i + 1]] { return s }
        // Live owner app starts at sign-in; offline (or dev JWT) skips straight to capture.
        if Config.apiBaseURL != nil && ProcessInfo.processInfo.environment["PISAH_OWNER_JWT"] == nil { return .signIn }
        return .capture
    }()
    @Published var email = ""
    @Published var password = ""
    @Published var name = "Sara"
    @Published var nl = true
    @Published var tt = false
    @Published var sotong = false
    @Published var autoFillAmount = true
    @Published var settingsBusy = false

    // Live state — populated from the backend; nil/empty in offline mode.
    @Published var slug: String?
    @Published var shareURL: String?   // full openable link from createSplit
    @Published var serverItems: [ItemDTO] = []
    @Published var serverOwedSen: Int?
    @Published var serverTaxSen: Int?
    @Published var serverShareLines: [ShareLine]?
    @Published var serverOwnerName: String?
    @Published var shareAutoFillAmount = true
    @Published var ownerQrURL: URL?
    @Published var profileQrURL: URL?       // owner's saved DuitNow QR from payment settings
    @Published var profileQrPreview: Data?  // local preview while uploading / offline pick
    @Published var trackParticipants: [ParticipantDTO]?
    @Published var collectedSenServer: Int?
    @Published var totalSenServer: Int?
    @Published var parsed: ScanResponse?   // OCR result populating the review screen
    @Published var errorMessage: String?

    init() {
        autoFillAmount = UserDefaults.standard.object(forKey: "autoFillAmount") as? Bool ?? true
        // Dev convenience: inject an owner JWT without a sign-in screen.
        if let jwt = ProcessInfo.processInfo.environment["PISAH_OWNER_JWT"], let c = client {
            Task {
                await c.setOwnerJWT(jwt)
                loadPaymentSettings()
            }
        }
    }

    private func applyPaymentSettings(_ s: PaymentSettingsDTO) {
        autoFillAmount = s.autoFillAmount
        UserDefaults.standard.set(s.autoFillAmount, forKey: "autoFillAmount")
        profileQrURL = s.ownerQrUrl.flatMap(URL.init(string:))
        if profileQrURL != nil { profileQrPreview = nil }
    }

    // ---- formatting ----
    func rm(_ n: Double) -> String { String(format: "RM %.2f", n) }
    func rmSen(_ s: Int) -> String { String(format: "RM %.2f", Double(s) / 100) }

    // ---- offline share math (ported from prototype renderVals) ----
    var sub: Double { (nl ? 12.5 : 0) + (tt ? 2.8 : 0) + (sotong ? 18.0 / 3 : 0) }
    var tax: Double { sub * (15.5 / 96.9) }
    var total: Double { sub + tax }
    var collected: Double { 27.8 + total }

    // ---- display: prefer server values when live ----
    var yourShareStr: String { serverOwedSen.map(rmSen) ?? rm(total) }
    var taxStr: String { serverTaxSen.map(rmSen) ?? rm(tax) }

    var shareLines: [(String, String)] {
        if let lines = serverShareLines {
            if lines.isEmpty { return [("No items selected yet", rmSen(0))] }
            return lines.map { ($0.name, rmSen($0.amtSen)) }
        }
        var a: [(String, String)] = []
        if nl { a.append(("Nasi Lemak Ayam", rm(12.5))) }
        if tt { a.append(("Teh Tarik", rm(2.8))) }
        if sotong { a.append(("Sambal Sotong · shared ÷3", rm(18.0 / 3))) }
        if a.isEmpty { a.append(("No items selected yet", rm(0))) }
        return a
    }

    var shareURLStr: String {
        if let shareURL { return Self.displayShareURL(shareURL) }
        if let slug { return Self.displaySharePath(slug: slug) }
        return Self.displaySharePath(slug: "demo")
    }
    var shareLink: String { shareURL ?? "https://\(shareURLStr)" }

    private static func displayShareURL(_ url: String) -> String {
        url.replacingOccurrences(of: "https://", with: "")
            .replacingOccurrences(of: "http://", with: "")
    }

    private static func displaySharePath(slug: String) -> String {
        if let base = Config.apiBaseURL, var host = base.host {
            if let port = base.port { host = "\(host):\(port)" }
            return "\(host)/r/\(slug)"
        }
        return "pisah.app/r/\(slug)"
    }
    var shareMessage: String { "Split the bill with me on Pisah 👉 \(shareLink)" }

    var hasDuitNowQR: Bool { profileQrPreview != nil || profileQrURL != nil }

    var collectedStr: String { collectedSenServer.map(rmSen) ?? rm(collected) }
    var collectedFrac: Double {
        if let c = collectedSenServer, let t = totalSenServer, t > 0 { return min(1.0, Double(c) / Double(t)) }
        return min(1.0, (collected / 112.4 * 100).rounded() / 100)
    }

    func go(_ s: Screen) { withAnimation(.easeOut(duration: 0.22)) { screen = s } }

    // Offline shutter: mock OCR with a delay, keep the prototype demo intact.
    func shutter() {
        parsed = nil
        go(.scanning)
        DispatchQueue.main.asyncAfter(deadline: .now() + 2.6) { [weak self] in
            guard let self, self.screen == .scanning else { return }
            self.go(.review)
        }
    }

    // A real captured/picked receipt image: scan it (live) or mock it (offline), then review.
    func onCaptured(_ jpeg: Data) {
        parsed = nil
        go(.scanning)
        Task {
            if live, let client {
                do {
                    parsed = try await client.scanReceipt(jpeg)
                    try? await Task.sleep(nanoseconds: 600_000_000)
                } catch { fail(error) }
            } else {
                try? await Task.sleep(nanoseconds: 2_000_000_000) // mock OCR
            }
            if screen == .scanning { go(.review) }
        }
    }

    // ---- review screen content: parsed receipt when present, else prototype demo ----
    var reviewMerchant: String { parsed?.merchant ?? "Nasi Lemak House" }

    // (qtyTag, name, amount) rows for the review list.
    var reviewRows: [(String, String, String)] {
        if let p = parsed {
            return p.items.map { ("\($0.qty)×", $0.name, String(format: "%.2f", Double($0.lineTotalSen) / 100)) }
        }
        return [("2×", "Nasi Lemak Ayam", "25.00"), ("1×", "Sambal Sotong", "18.00"),
                ("3×", "Teh Tarik", "8.40"), ("2×", "Milo Ais", "9.00")]
    }
    var parsedReceipt: Bool { parsed != nil }

    private func fail(_ e: Error) { errorMessage = "\(e)" }

    // ---- owner sign-in (Supabase GoTrue password grant) ----
    @Published var authBusy = false
    func signIn() {
        errorMessage = nil
        guard live, let client else { go(.capture); return } // offline: no auth needed
        authBusy = true
        Task {
            do {
                try await client.signIn(email: email.trimmingCharacters(in: .whitespaces), password: password)
                if let n = await client.ownerDisplayName() { name = n }
                password = ""
                loadPaymentSettings()
                go(.capture)
            } catch { fail(error) }
            authBusy = false
        }
    }

    func signInWithGoogle() {
        errorMessage = nil
        guard live, let client else { return }
        authBusy = true
        Task {
            do {
                try await client.signInWithGoogle()
                if let n = await client.ownerDisplayName() { name = n }
                loadPaymentSettings()
                go(.capture)
            } catch { fail(error) }
            authBusy = false
        }
    }

    // ---- owner payment settings ----
    func loadPaymentSettings() {
        guard live, let client else { return }
        Task {
            do { applyPaymentSettings(try await client.getPaymentSettings()) }
            catch { fail(error) }
        }
    }

    func setAutoFillAmount(_ value: Bool) {
        autoFillAmount = value
        UserDefaults.standard.set(value, forKey: "autoFillAmount")
        guard live, let client else { return }
        Task {
            do { applyPaymentSettings(try await client.updatePaymentSettings(autoFillAmount: value)) }
            catch { fail(error) }
        }
    }

    func uploadDuitNowQR(_ jpeg: Data) {
        profileQrPreview = jpeg
        guard live, let client else { return }
        settingsBusy = true
        Task {
            defer { settingsBusy = false }
            do { applyPaymentSettings(try await client.uploadDuitNowQR(jpeg)) }
            catch { fail(error) }
        }
    }


    // ---- owner: create split from the reviewed items, then go to share ----
    func startShare() {
        guard live, let client else { go(.share); return }
        var input = parsed.map { Self.inputFrom($0, ownerName: name) } ?? Self.reviewSplitInput(ownerName: name)
        input.ownerQrUrl = profileQrURL?.absoluteString
        Task {
            do {
                let r = try await client.createSplit(input)
                slug = r.slug
                shareURL = r.shareUrl
                totalSenServer = r.split.totalSen
                if let u = r.split.ownerQrUrl { ownerQrURL = URL(string: u) }
            } catch { fail(error) }
            go(.share)
        }
    }

    // Map an OCR result into a create-split payload (Textract lumps SST+service into tax).
    static func inputFrom(_ p: ScanResponse, ownerName: String) -> CreateSplitInput {
        CreateSplitInput(
            merchant: p.merchant.isEmpty ? "Receipt" : p.merchant, ownerName: ownerName, ownerQrUrl: nil,
            subtotalSen: p.subtotalSen, sstSen: p.taxSen, serviceSen: 0, roundingSen: 0,
            totalSen: p.totalSen > 0 ? p.totalSen : p.subtotalSen + p.taxSen,
            items: p.items.map { .init(name: $0.name, qty: $0.qty, unitPriceSen: $0.unitPriceSen, lineTotalSen: $0.lineTotalSen) })
    }

    // ---- friend: join the split, load its items, then go to pick ----
    func joinSplit() {
        guard live, let client, let slug else { go(.fPick); return }
        Task {
            do {
                _ = try await client.join(slug: slug, name: name)
                serverItems = try await client.getSplit(slug: slug).items
            } catch { fail(error) }
            go(.fPick)
        }
    }

    // ---- friend: toggle an item locally + sync claims to server ----
    func toggleItem(_ key: String) {
        switch key {
        case "nl": nl.toggle()
        case "tt": tt.toggle()
        case "sotong": sotong.toggle()
        default: break
        }
        guard live, let client, let slug else { return }
        let ids = selectedItemIDs()
        Task {
            do { applyShare(try await client.setClaims(slug: slug, itemIDs: ids)) }
            catch { fail(error) }
        }
    }

    private func selectedItemIDs() -> [String] {
        var names: [String] = []
        if nl { names.append("Nasi Lemak Ayam") }
        if tt { names.append("Teh Tarik") }
        if sotong { names.append("Sambal Sotong") }
        return serverItems.filter { names.contains($0.name) }.map(\.id)
    }

    private func applyShare(_ r: ShareResponse) {
        serverOwedSen = r.owedSen
        serverTaxSen = r.taxSen
        serverShareLines = r.lines
        serverOwnerName = r.ownerName
        shareAutoFillAmount = r.autoFillAmount ?? true
        if let u = r.ownerQrUrl { ownerQrURL = URL(string: u) }
    }

    func loadShare() {
        guard live, let client, let slug else { return }
        Task { do { applyShare(try await client.getShare(slug: slug)) } catch { fail(error) } }
    }

    func pay() {
        guard live, let client, let slug else { go(.fDone); return }
        Task { do { _ = try await client.markPaid(slug: slug) } catch { fail(error) }; go(.fDone) }
    }

    // ---- owner: load tracking + stream live payment updates ----
    func loadTrack() {
        guard live, let client, let slug else { return }
        Task {
            do {
                let t = try await client.track(slug: slug)
                trackParticipants = t.participants
                collectedSenServer = t.collectedSen
                totalSenServer = t.split.totalSen
            } catch { fail(error) }
        }
    }

    func listenTrack() {
        guard live, let client, let slug else { return }
        Task {
            for await ev in await client.events(slug: slug) {
                collectedSenServer = ev.collectedSen
                if let t = try? await client.track(slug: slug) { trackParticipants = t.participants }
            }
        }
    }

    // Reviewed receipt -> create payload. Mirrors the prototype's Nasi Lemak House bill (sen).
    static func reviewSplitInput(ownerName: String) -> CreateSplitInput {
        CreateSplitInput(
            merchant: "Nasi Lemak House", ownerName: ownerName, ownerQrUrl: nil,
            subtotalSen: 9690, sstSen: 581, serviceSen: 969, roundingSen: 0, totalSen: 11240,
            items: [
                .init(name: "Nasi Lemak Ayam", qty: 2, unitPriceSen: 1250, lineTotalSen: 2500),
                .init(name: "Sambal Sotong", qty: 1, unitPriceSen: 1800, lineTotalSen: 1800),
                .init(name: "Teh Tarik", qty: 3, unitPriceSen: 280, lineTotalSen: 840),
                .init(name: "Milo Ais", qty: 2, unitPriceSen: 450, lineTotalSen: 900),
            ])
    }
}

// MARK: - Deterministic faux DuitNow QR (ported 1:1)
func qrGrid(seed: Int) -> [[Bool]] {
    let size = 25
    var s = (seed &* 2654435761) % 2147483647
    func rnd() -> Double { s = (s &* 16807) % 2147483647; return Double(s) / 2147483647.0 }
    var grid = Array(repeating: Array(repeating: false, count: size), count: size)
    func finder(_ r0: Int, _ c0: Int) {
        for r in 0..<7 { for c in 0..<7 {
            let edge = r == 0 || r == 6 || c == 0 || c == 6
            let inner = r >= 2 && r <= 4 && c >= 2 && c <= 4
            grid[r0 + r][c0 + c] = edge || inner
        } }
    }
    finder(0, 0); finder(0, size - 7); finder(size - 7, 0)
    func inFinder(_ r: Int, _ c: Int) -> Bool {
        (r < 8 && c < 8) || (r < 8 && c >= size - 8) || (r >= size - 8 && c < 8)
    }
    for r in 0..<size { for c in 0..<size where !inFinder(r, c) { grid[r][c] = rnd() > 0.5 } }
    return grid
}

struct QRView: View {
    let seed: Int
    let cell: CGFloat
    var body: some View {
        let g = qrGrid(seed: seed)
        let n = g.count
        Canvas { ctx, _ in
            for r in 0..<n { for c in 0..<n where g[r][c] {
                let rect = CGRect(x: CGFloat(c) * cell, y: CGFloat(r) * cell, width: cell, height: cell)
                ctx.fill(Path(roundedRect: rect, cornerRadius: cell * 0.18), with: .color(P.ink))
            } }
        }
        .frame(width: CGFloat(n) * cell, height: CGFloat(n) * cell)
    }
}

// MARK: - Shared chrome
struct AppBackground: View {
    var body: some View {
        RadialGradient(gradient: Gradient(colors: [Color(hex: 0xFBEFE8), Color(hex: 0xF1E7DA), Color(hex: 0xEADDCB)]),
                       center: .top, startRadius: 0, endRadius: 760)
            .ignoresSafeArea()
    }
}

// Faux mobile-browser bar shown on the friend (web link) screens.
struct FauxBrowserBar: View {
    @EnvironmentObject var s: AppState

    var body: some View {
        HStack(spacing: 9) {
            HStack(spacing: 5) { ForEach(0..<3, id: \.self) { _ in Circle().fill(Color(hex: 0xD8CDBC)).frame(width: 9, height: 9) } }
            HStack(spacing: 7) {
                Image(systemName: "lock.fill").font(.system(size: 9)).foregroundColor(P.green)
                Text(s.shareURLStr).font(F.t(12, .medium)).foregroundColor(P.brown)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 12).padding(.vertical, 7)
            .background(Color.white).clipShape(RoundedRectangle(cornerRadius: 9))
        }
        .padding(.horizontal, 16).padding(.vertical, 14)
        .background(Color(hex: 0xEFE8DF))
    }
}

struct BackChip: View {
    let action: () -> Void
    var body: some View {
        Button(action: action) {
            Image(systemName: "chevron.left").font(.system(size: 14, weight: .bold)).foregroundColor(P.ink)
                .frame(width: 34, height: 34).background(P.cream2)
                .clipShape(RoundedRectangle(cornerRadius: 11, style: .continuous))
        }.buttonStyle(.plain)
    }
}

struct PrimaryButton: View {
    let title: String
    let action: () -> Void
    var body: some View {
        Button(action: action) {
            Text(title).font(F.t(15, .bold)).foregroundColor(.white)
                .frame(maxWidth: .infinity).padding(16)
                .background(P.orange).clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
                .shadow(color: P.orange.opacity(0.3), radius: 10, y: 8)
        }.buttonStyle(.plain)
    }
}

struct RootView: View {
    @StateObject private var s = AppState()
    var body: some View {
        ZStack {
            AppBackground()
            Group {
                switch s.screen {
                case .signIn: SignInView()
                case .capture: CaptureView()
                case .scanning: ScanningView()
                case .review: ReviewView()
                case .share: ShareView()
                case .track: TrackView()
                case .settings: SettingsView()
                case .fLanding: FriendLandingView()
                case .fPick: FriendPickView()
                case .fShare: FriendShareView()
                case .fPay: FriendPayView()
                case .fDone: FriendDoneView()
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .environmentObject(s)
    }
}

@main
struct PisahApp: App {
    var body: some Scene {
        WindowGroup { RootView() }
    }
}
