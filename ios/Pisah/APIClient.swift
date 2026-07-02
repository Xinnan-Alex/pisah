import Foundation

// MARK: - Config
// apiBaseURL == nil  -> app runs as the offline clickthrough prototype (default).
// Priority: scheme env var (Xcode Run) > DevConfig.swift (Debug, baked in) > offline.
enum Config {
    private static func string(_ key: String, devFallback: String? = nil) -> String? {
        if let v = ProcessInfo.processInfo.environment[key], !v.isEmpty { return v }
        #if DEBUG
        if let devFallback, !devFallback.isEmpty { return devFallback }
        #endif
        return nil
    }

    static let apiBaseURL: URL? = string("PISAH_API", devFallback: DevConfig.pisahAPI).flatMap(URL.init)
}

// MARK: - DTOs (money is integer sen; mirrors the Go backend JSON)
struct SplitDTO: Codable {
    var id: String; var slug: String; var merchant: String
    var ownerName: String; var ownerQrUrl: String?
    var subtotalSen: Int; var sstSen: Int; var serviceSen: Int; var roundingSen: Int; var totalSen: Int
}
struct ItemDTO: Codable, Identifiable {
    var id: String; var name: String; var qty: Int
    var unitPriceSen: Int; var lineTotalSen: Int; var position: Int
    var claimants: Int; var claimedBy: [String]
}
struct ParticipantDTO: Codable, Identifiable {
    var id: String; var name: String; var isOwner: Bool
    var owedSen: Int; var paid: Bool; var paidAt: String?
}
struct ShareLine: Codable, Identifiable {
    var name: String; var amtSen: Int
    var id: String { name }
}
struct GetSplitResponse: Codable { var split: SplitDTO; var taxTotalSen: Int; var items: [ItemDTO] }
struct JoinResponse: Codable { var token: String; var participant: ParticipantDTO }
struct ShareResponse: Codable {
    var merchant: String; var ownerName: String; var ownerQrUrl: String?
    var lines: [ShareLine]; var taxSen: Int; var owedSen: Int
}
struct CreateSplitResponse: Codable { var id: String; var slug: String; var shareUrl: String; var split: SplitDTO }
struct TrackResponse: Codable { var split: SplitDTO; var collectedSen: Int; var participants: [ParticipantDTO] }
struct ScanResponse: Codable {
    var merchant: String; var subtotalSen: Int; var taxSen: Int; var totalSen: Int
    var items: [ScanItem]
    struct ScanItem: Codable { var name: String; var qty: Int; var unitPriceSen: Int; var lineTotalSen: Int }
}

struct CreateSplitInput: Encodable {
    var merchant: String; var ownerName: String; var ownerQrUrl: String?
    var subtotalSen: Int; var sstSen: Int; var serviceSen: Int; var roundingSen: Int; var totalSen: Int
    var items: [Item]
    struct Item: Encodable { var name: String; var qty: Int; var unitPriceSen: Int; var lineTotalSen: Int }
}

struct PaidEvent: Codable { var type: String; var participant: ParticipantDTO; var collectedSen: Int }

enum APIError: Error { case http(Int, String), notConfigured }

// MARK: - Client
// One actor instance holds the owner JWT + the friend's participant token.
actor APIClient {
    let baseURL: URL
    private var ownerJWT: String?
    private var participantToken: String?
    private let session = URLSession.shared
    private let dec = JSONDecoder()

    init(baseURL: URL) { self.baseURL = baseURL }

    func setParticipantToken(_ t: String) { participantToken = t }

    // Owner sign-in — backend proxies GoTrue so the phone only needs PISAH_API.
    func signIn(email: String, password: String) async throws {
        struct Body: Encodable { let email: String; let password: String }
        struct Tok: Decodable { var access_token: String }
        let tok: Tok = try await send(
            "api/auth/sign-in", "POST",
            body: try JSONEncoder().encode(Body(email: email, password: password)),
            auth: .none)
        ownerJWT = tok.access_token
    }

    func setOwnerJWT(_ t: String) { ownerJWT = t }

    static let oauthRedirect = "pisah://auth/callback"

    /// Google sign-in via Supabase OAuth in ASWebAuthenticationSession.
    func signInWithGoogle() async throws {
        struct OAuthStart: Decodable { let url: String }
        let enc = Self.oauthRedirect.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? Self.oauthRedirect
        let start: OAuthStart = try await send("api/auth/oauth/google?redirect_to=\(enc)", "GET", auth: .none)
        guard let url = URL(string: start.url) else { throw APIError.http(0, "bad oauth url") }
        let callback = try await Task { @MainActor in
            try await OAuthSession().start(url: url, callbackScheme: "pisah")
        }.value
        guard let token = OAuthCallback.accessToken(from: callback) else {
            throw APIError.http(0, "missing access_token in oauth callback")
        }
        ownerJWT = token
    }

    // ---- owner ----
    func scanReceipt(_ image: Data) async throws -> ScanResponse {
        try await send("api/receipts/scan", "POST", body: image, contentType: "image/jpeg", auth: .owner)
    }
    func createSplit(_ input: CreateSplitInput) async throws -> CreateSplitResponse {
        try await send("api/splits", "POST", body: try JSONEncoder().encode(input), auth: .owner)
    }
    func track(slug: String) async throws -> TrackResponse {
        try await send("api/splits/\(slug)/track", "GET", auth: .owner)
    }

    // ---- friend ----
    func getSplit(slug: String) async throws -> GetSplitResponse {
        try await send("api/splits/\(slug)", "GET", auth: .none)
    }
    func join(slug: String, name: String) async throws -> JoinResponse {
        let r: JoinResponse = try await send("api/splits/\(slug)/join", "POST",
                                             body: try JSONEncoder().encode(["name": name]), auth: .none)
        participantToken = r.token
        return r
    }
    func setClaims(slug: String, itemIDs: [String]) async throws -> ShareResponse {
        try await send("api/splits/\(slug)/claims", "POST",
                       body: try JSONEncoder().encode(["itemIds": itemIDs]), auth: .participant)
    }
    func getShare(slug: String) async throws -> ShareResponse {
        try await send("api/splits/\(slug)/share", "GET", auth: .participant)
    }
    func markPaid(slug: String) async throws -> ParticipantDTO {
        try await send("api/splits/\(slug)/paid", "POST", body: Data("{}".utf8), auth: .participant)
    }

    // ---- SSE: yields a PaidEvent each time someone pays ----
    func events(slug: String) -> AsyncStream<PaidEvent> {
        let url = requestURL(for: "api/splits/\(slug)/events")
        let session = self.session, dec = self.dec
        return AsyncStream { cont in
            let task = Task {
                do {
                    var req = URLRequest(url: url)
                    req.setValue("text/event-stream", forHTTPHeaderField: "Accept")
                    let (bytes, resp) = try await session.bytes(for: req)
                    guard (resp as? HTTPURLResponse).map({ (200..<300).contains($0.statusCode) }) ?? false else {
                        cont.finish(); return
                    }
                    for try await line in bytes.lines {
                        guard line.hasPrefix("data:") else { continue }
                        let json = line.dropFirst(5).trimmingCharacters(in: .whitespaces)
                        if let ev = try? dec.decode(PaidEvent.self, from: Data(json.utf8)) { cont.yield(ev) }
                    }
                } catch {}
                cont.finish()
            }
            cont.onTermination = { _ in task.cancel() }
        }
    }

    // ---- transport ----
    private enum Auth { case none, owner, participant }

    private func requestURL(for path: String) -> URL {
        guard let url = URL(string: path, relativeTo: baseURL)?.absoluteURL else {
            preconditionFailure("invalid api path: \(path)")
        }
        return url
    }

    private func send<T: Decodable>(_ path: String, _ method: String,
                                    body: Data? = nil, contentType: String = "application/json",
                                    auth: Auth) async throws -> T {
        var req = URLRequest(url: requestURL(for: path))
        req.httpMethod = method
        if let body { req.httpBody = body; req.setValue(contentType, forHTTPHeaderField: "Content-Type") }
        switch auth {
        case .none: break
        case .owner: if let j = ownerJWT { req.setValue("Bearer \(j)", forHTTPHeaderField: "Authorization") }
        case .participant: if let t = participantToken { req.setValue("Bearer \(t)", forHTTPHeaderField: "Authorization") }
        }
        let (data, resp) = try await session.data(for: req)
        try Self.check(resp, data)
        return try dec.decode(T.self, from: data)
    }

    private static func check(_ resp: URLResponse, _ data: Data) throws {
        guard let http = resp as? HTTPURLResponse else { throw APIError.http(0, "no response") }
        guard (200..<300).contains(http.statusCode) else {
            throw APIError.http(http.statusCode, String(data: data, encoding: .utf8) ?? "")
        }
    }
}
