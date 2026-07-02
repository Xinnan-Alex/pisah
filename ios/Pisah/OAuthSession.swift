import AuthenticationServices
import Foundation
import UIKit

enum OAuthCallback {
    /// Parses Supabase GoTrue redirect: `pisah://auth/callback#access_token=...`
    static func accessToken(from url: URL) -> String? {
        let raw = url.fragment ?? url.query ?? ""
        guard !raw.isEmpty else { return nil }
        var params: [String: String] = [:]
        for pair in raw.split(separator: "&") {
            let kv = pair.split(separator: "=", maxSplits: 1)
            guard kv.count == 2 else { continue }
            let key = String(kv[0])
            let val = String(kv[1]).replacingOccurrences(of: "+", with: " ")
            params[key] = val.removingPercentEncoding ?? val
        }
        return params["access_token"]
    }
}

enum JWTClaims {
    static func displayName(from jwt: String) -> String? {
        let parts = jwt.split(separator: ".")
        guard parts.count >= 2 else { return nil }
        var payload = String(parts[1])
            .replacingOccurrences(of: "-", with: "+")
            .replacingOccurrences(of: "_", with: "/")
        while payload.count % 4 != 0 { payload += "=" }
        guard let data = Data(base64Encoded: payload),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return nil }
        if let meta = json["user_metadata"] as? [String: Any] {
            for key in ["full_name", "name"] {
                if let n = meta[key] as? String {
                    let trimmed = n.trimmingCharacters(in: .whitespacesAndNewlines)
                    if !trimmed.isEmpty { return trimmed }
                }
            }
        }
        if let email = json["email"] as? String, let at = email.firstIndex(of: "@") {
            return String(email[..<at])
        }
        return nil
    }
}

/// Opens Supabase Google OAuth in ASWebAuthenticationSession; returns on deep-link callback.
@MainActor
final class OAuthSession: NSObject, ASWebAuthenticationPresentationContextProviding {
    private var session: ASWebAuthenticationSession?

    func start(url: URL, callbackScheme: String) async throws -> URL {
        try await withCheckedThrowingContinuation { cont in
            session = ASWebAuthenticationSession(url: url, callbackURLScheme: callbackScheme) { callbackURL, err in
                if let err {
                    cont.resume(throwing: err)
                    return
                }
                guard let callbackURL else {
                    cont.resume(throwing: URLError(.badServerResponse))
                    return
                }
                cont.resume(returning: callbackURL)
            }
            session?.prefersEphemeralWebBrowserSession = false
            session?.presentationContextProvider = self
            guard session?.start() == true else {
                cont.resume(throwing: URLError(.cannotOpenFile))
                return
            }
        }
    }

    func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .flatMap(\.windows)
            .first { $0.isKeyWindow } ?? ASPresentationAnchor()
    }
}
