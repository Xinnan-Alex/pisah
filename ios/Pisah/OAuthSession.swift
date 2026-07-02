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
