import SwiftUI
import UIKit

// Owner sign-in (Supabase GoTrue). Shown only in live mode; offline skips straight to capture.
struct SignInView: View {
    @EnvironmentObject var s: AppState
    @FocusState private var focus: Field?
    enum Field { case email, password }

    var body: some View {
        VStack(spacing: 0) {
            Spacer()
            HStack(spacing: 0) {
                Text("Pisah").font(F.d(40)).foregroundColor(P.ink)
                Text(" ·").font(F.d(40)).foregroundColor(P.orange)
            }
            Text("Sign in to keep splitting")
                .font(F.t(14, .medium)).foregroundColor(P.brown).padding(.top, 6).padding(.bottom, 28)

            VStack(spacing: 12) {
                field("Email", text: $s.email, secure: false)
                    .keyboardType(.emailAddress).textInputAutocapitalization(.never).autocorrectionDisabled()
                    .textContentType(.username).focused($focus, equals: .email)
                    .submitLabel(.next).onSubmit { focus = .password }
                field("Password", text: $s.password, secure: true)
                    .textContentType(.password).focused($focus, equals: .password)
                    .submitLabel(.go).onSubmit { s.signIn() }
            }

            if let err = s.errorMessage {
                Text(err).font(F.t(12, .medium)).foregroundColor(P.orange)
                    .multilineTextAlignment(.center).padding(.top, 12).padding(.horizontal, 4)
            }

            Button { s.signIn() } label: {
                ZStack {
                    Text("Sign in").font(F.t(15, .bold)).foregroundColor(.white).opacity(s.authBusy ? 0 : 1)
                    if s.authBusy { ProgressView().tint(.white) }
                }
                .frame(maxWidth: .infinity).padding(16)
                .background(P.orange).clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
                .shadow(color: P.orange.opacity(0.3), radius: 10, y: 8)
            }
            .buttonStyle(.plain).disabled(s.authBusy).padding(.top, 18)

            Text("No app needed for friends — they pay by DuitNow QR")
                .font(F.t(11, .medium)).foregroundColor(P.mut).multilineTextAlignment(.center).padding(.top, 16)
            Spacer()
            Spacer()
        }
        .padding(.horizontal, 30)
    }

    private func field(_ placeholder: String, text: Binding<String>, secure: Bool) -> some View {
        Group {
            if secure { SecureField(placeholder, text: text) } else { TextField(placeholder, text: text) }
        }
        .font(F.t(15, .semibold)).foregroundColor(P.ink)
        .padding(.horizontal, 16).padding(.vertical, 15)
        .background(P.cream)
        .overlay(RoundedRectangle(cornerRadius: 14).stroke(P.border, lineWidth: 1.5))
        .clipShape(RoundedRectangle(cornerRadius: 14))
    }
}

// Small receipt skeleton used in the camera viewfinder.
private struct ReceiptSkeleton: View {
    var body: some View {
        VStack(spacing: 9) {
            RoundedRectangle(cornerRadius: 3).fill(P.ink).frame(width: 100, height: 9)
            ForEach([(0.52, 0.20), (0.60, 0.16), (0.44, 0.22), (0.56, 0.18)], id: \.0) { w in
                HStack {
                    RoundedRectangle(cornerRadius: 3).fill(Color(hex: 0xDDD3C5)).frame(width: 150 * w.0, height: 6)
                    Spacer()
                    RoundedRectangle(cornerRadius: 3).fill(Color(hex: 0xDDD3C5)).frame(width: 150 * w.1, height: 6)
                }
            }
            Divider().overlay(Color(hex: 0xC9BDAB))
            HStack {
                RoundedRectangle(cornerRadius: 3).fill(P.ink).frame(width: 50, height: 7)
                Spacer()
                RoundedRectangle(cornerRadius: 3).fill(P.ink).frame(width: 42, height: 7)
            }
        }
        .padding(16).frame(width: 168).background(P.paper)
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

struct CaptureView: View {
    @EnvironmentObject var s: AppState
    @State private var showCamera = false
    @State private var showLibrary = false
    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Pisah").font(F.d(22)).foregroundColor(P.ink)
                Spacer()
                Button { s.go(.settings) } label: {
                    ZStack {
                        Circle().stroke(P.brown, lineWidth: 2).frame(width: 15, height: 15)
                        Circle().stroke(P.brown, lineWidth: 2).frame(width: 7, height: 7)
                    }.frame(width: 36, height: 36).background(P.cream2).clipShape(Circle())
                }.buttonStyle(.plain)
            }.padding(.horizontal, 26).padding(.top, 8)

            VStack(alignment: .leading, spacing: 6) {
                Text("Snap your\nreceipt").font(F.d(27)).foregroundColor(P.ink).lineSpacing(2)
                Text("We'll read every item with AI").font(F.t(13, .medium)).foregroundColor(P.brown)
            }.frame(maxWidth: .infinity, alignment: .leading).padding(.horizontal, 26).padding(.top, 14)

            ZStack {
                ReceiptSkeleton().rotationEffect(.degrees(-4)).shadow(color: .black.opacity(0.4), radius: 14, y: 10)
                VStack {
                    Spacer()
                    Text("Align receipt within the frame").font(F.t(11, .semibold)).foregroundColor(.white.opacity(0.7)).padding(.bottom, 16)
                }
                ViewfinderCorners()
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity).background(Color(hex: 0x1C1610))
            .clipShape(RoundedRectangle(cornerRadius: 28, style: .continuous)).padding(.horizontal, 26).padding(.vertical, 16)

            HStack {
                Button { showLibrary = true } label: {
                    VStack(spacing: 4) {
                        RoundedRectangle(cornerRadius: 3).stroke(P.brown, lineWidth: 2).frame(width: 18, height: 15)
                            .frame(width: 48, height: 48).background(P.cream2).clipShape(RoundedRectangle(cornerRadius: 14))
                        Text("Gallery").font(F.t(10, .semibold)).foregroundColor(P.brown)
                    }
                }.buttonStyle(.plain)
                Spacer()
                Button { if s.live { showCamera = true } else { s.shutter() } } label: {
                    ZStack {
                        Circle().stroke(P.orange, lineWidth: 4).frame(width: 76, height: 76)
                        Circle().fill(P.orange).frame(width: 58, height: 58)
                    }
                }.buttonStyle(.plain)
                Spacer()
                VStack(spacing: 4) {
                    Circle().stroke(P.brown, lineWidth: 2).frame(width: 16, height: 16)
                        .frame(width: 48, height: 48).background(P.cream2).clipShape(RoundedRectangle(cornerRadius: 14))
                    Text("Flash").font(F.t(10, .semibold)).foregroundColor(P.brown)
                }
            }.padding(.horizontal, 40).padding(.bottom, 26)
        }
        .fullScreenCover(isPresented: $showCamera) {
            ReceiptCameraView { s.onCaptured($0) }
        }
        .sheet(isPresented: $showLibrary) {
            PhotoLibraryPicker { s.onCaptured($0) }
        }
    }
}

// Orange L-brackets in the four corners of the viewfinder.
struct ViewfinderCorners: View {
    var body: some View {
        ZStack {
            ForEach(0..<4, id: \.self) { i in
                let top = i < 2, left = i % 2 == 0
                Corner(top: top, left: left).stroke(P.orange, style: StrokeStyle(lineWidth: 3, lineCap: .round))
                    .frame(width: 28, height: 28)
                    .frame(maxWidth: .infinity, maxHeight: .infinity,
                           alignment: Alignment(horizontal: left ? .leading : .trailing, vertical: top ? .top : .bottom))
                    .padding(18)
            }
        }
    }
    struct Corner: Shape {
        let top: Bool, left: Bool
        func path(in r: CGRect) -> Path {
            var p = Path()
            let a = CGPoint(x: left ? r.minX : r.maxX, y: top ? r.maxY : r.minY)
            let b = CGPoint(x: left ? r.minX : r.maxX, y: top ? r.minY : r.maxY)
            let c = CGPoint(x: left ? r.maxX : r.minX, y: top ? r.minY : r.maxY)
            p.move(to: a); p.addLine(to: b); p.addLine(to: c)
            return p
        }
    }
}

struct ScanningView: View {
    @EnvironmentObject var s: AppState
    @State private var scan = false
    @State private var pulse = false
    var body: some View {
        VStack(spacing: 0) {
            ZStack {
                ReceiptSkeleton().shadow(color: .black.opacity(0.4), radius: 14, y: 10)
                GeometryReader { g in
                    Rectangle()
                        .fill(LinearGradient(colors: [.clear, P.orange, .clear], startPoint: .leading, endPoint: .trailing))
                        .frame(height: 3).shadow(color: P.orange.opacity(0.5), radius: 9)
                        .offset(y: scan ? g.size.height * 0.74 : g.size.height * 0.14)
                }
                VStack {
                    HStack {
                        Spacer()
                        HStack(spacing: 6) {
                            Circle().fill(P.orange).frame(width: 8, height: 8).opacity(pulse ? 0.35 : 1)
                            Text("AI reading").font(F.t(11, .bold)).foregroundColor(Color(hex: 0xFFD9CE))
                        }
                        .padding(.horizontal, 11).padding(.vertical, 6)
                        .background(P.orange.opacity(0.16)).overlay(RoundedRectangle(cornerRadius: 20).stroke(P.orange.opacity(0.4)))
                        .clipShape(RoundedRectangle(cornerRadius: 20))
                    }
                    Spacer()
                }.padding(18)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity).background(Color(hex: 0x1C1610))
            .clipShape(RoundedRectangle(cornerRadius: 28, style: .continuous)).padding(.horizontal, 26).padding(.vertical, 18)

            VStack(alignment: .leading, spacing: 4) {
                Text("Reading your receipt…").font(F.d(23)).foregroundColor(P.ink)
                Text("Hang tight, almost done").font(F.t(13, .medium)).foregroundColor(P.brown)
            }.frame(maxWidth: .infinity, alignment: .leading).padding(.horizontal, 26)

            VStack(alignment: .leading, spacing: 12) {
                if s.live {
                    if let p = s.parsed {
                        checkRow("Detected", bold: p.merchant.isEmpty ? "Receipt" : p.merchant)
                        checkRow("Found", bold: itemCountLabel(p.items.count))
                        checkRow("Calculated", bold: "SST & service charge")
                    } else {
                        loadingRow("Detecting merchant…")
                        loadingRow("Reading line items…")
                        loadingRow("Calculating SST & service charge…")
                    }
                } else {
                    checkRow("Detected", bold: "Nasi Lemak House")
                    checkRow("Found", bold: "8 items")
                    loadingRow("Calculating SST & service charge…")
                }
            }.padding(.horizontal, 26).padding(.top, 14).padding(.bottom, 30)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .onAppear {
            withAnimation(.easeInOut(duration: 2.2).repeatForever(autoreverses: true)) { scan = true }
            withAnimation(.easeInOut(duration: 1).repeatForever(autoreverses: true)) { pulse = true }
        }
    }
    private func itemCountLabel(_ count: Int) -> String {
        count == 1 ? "1 item" : "\(count) items"
    }

    private func checkRow(_ lead: String, bold: String) -> some View {
        HStack(spacing: 11) {
            ZStack { Circle().fill(P.green).frame(width: 22, height: 22); Image(systemName: "checkmark").font(.system(size: 10, weight: .bold)).foregroundColor(.white) }
            (Text(lead + " ").font(F.t(14, .semibold)) + Text(bold).font(F.t(14, .bold))).foregroundColor(P.ink)
        }
    }

    private func loadingRow(_ text: String) -> some View {
        HStack(spacing: 11) {
            ProgressView().progressViewStyle(.circular).tint(P.orange).frame(width: 22, height: 22)
            Text(text).font(F.t(14, .semibold)).foregroundColor(P.brown)
        }
    }
}

struct ReviewView: View {
    @EnvironmentObject var s: AppState
    var body: some View {
        VStack(spacing: 0) {
            VStack(spacing: 0) {
                HStack {
                    BackChip { s.go(.capture) }
                    Spacer()
                    VStack(spacing: 2) {
                        Text(s.reviewMerchant).font(F.d(16)).foregroundColor(P.ink)
                        Text("29 Jun · 8:24 PM").font(F.t(11, .medium)).foregroundColor(P.brown)
                    }
                    Spacer()
                    Text("Edit").font(F.t(13, .bold)).foregroundColor(P.orange)
                }.padding(.horizontal, 22).padding(.bottom, 14)
            }.background(Color.white)

            ScrollView {
                VStack(spacing: 0) {
                    let rows = s.reviewRows
                    ForEach(Array(rows.enumerated()), id: \.offset) { idx, it in
                        HStack(spacing: 11) {
                            Text(it.0).font(F.t(12, .bold)).foregroundColor(P.orange)
                                .frame(width: 28, height: 28).background(P.peach).clipShape(RoundedRectangle(cornerRadius: 8))
                            Text(it.1).font(F.t(14, .semibold)).foregroundColor(P.ink).frame(maxWidth: .infinity, alignment: .leading)
                            Text(it.2).font(F.t(14, .bold)).foregroundColor(P.ink)
                        }.padding(12)
                        if idx < rows.count - 1 { Divider().overlay(P.line) }
                    }
                }.padding(6).card()

                Text("+ Add an item").font(F.t(13, .bold)).foregroundColor(P.orange).padding(12)

                VStack(spacing: 0) {
                    if let p = s.parsed {
                        totalRow("Subtotal", s.rmSen(p.subtotalSen))
                        totalRow("SST & service", s.rmSen(p.taxSen))
                        Divider().overlay(P.border).padding(.top, 4)
                        HStack(alignment: .firstTextBaseline) {
                            Text("Total").font(F.d(16)).foregroundColor(P.ink)
                            Spacer()
                            Text(s.rmSen(p.totalSen > 0 ? p.totalSen : p.subtotalSen + p.taxSen)).font(F.d(20)).foregroundColor(P.orange)
                        }.padding(.top, 11)
                    } else {
                        totalRow("Subtotal", "RM 96.90")
                        totalRow("SST 6%", "RM 5.81")
                        totalRow("Service charge 10%", "RM 9.69")
                        totalRow("Rounding adj.", "+RM 0.00")
                        Divider().overlay(P.border).padding(.top, 4)
                        HStack(alignment: .firstTextBaseline) {
                            Text("Total").font(F.d(16)).foregroundColor(P.ink)
                            Spacer()
                            Text("RM 112.40").font(F.d(20)).foregroundColor(P.orange)
                        }.padding(.top, 11)
                    }
                }.padding(.horizontal, 16).padding(.vertical, 14).card()
            }.padding(.horizontal, 20).padding(.top, 16)

            PrimaryButton(title: "Share the split →") { s.startShare() }.padding(.horizontal, 20).padding(.vertical, 16)
        }
        .background(P.cream)
    }
    private func totalRow(_ l: String, _ r: String) -> some View {
        HStack { Text(l).font(F.t(13, .medium)).foregroundColor(P.brown); Spacer(); Text(r).font(F.t(13, .semibold)).foregroundColor(P.ink) }.padding(.vertical, 5)
    }
}

struct ShareView: View {
    @EnvironmentObject var s: AppState
    @Environment(\.openURL) private var openURL
    @State private var copied = false

    private func enc(_ str: String) -> String { str.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? "" }
    private var waURL: URL? { URL(string: "https://wa.me/?text=\(enc(s.shareMessage))") }
    private var tgURL: URL? { URL(string: "https://t.me/share/url?url=\(enc(s.shareLink))&text=\(enc("Split the bill with me on Pisah"))") }

    var body: some View {
        VStack(spacing: 0) {
            VStack(spacing: 5) {
                ZStack {
                    Circle().fill(P.peach).frame(width: 64, height: 64)
                    Image(systemName: "checkmark").font(.system(size: 24, weight: .bold)).foregroundColor(P.green)
                }
                Text("Split is ready!").font(F.d(25)).foregroundColor(P.ink)
                Text("Send the link — friends pick what\nthey had & pay you back")
                    .font(F.t(13, .medium)).foregroundColor(P.brown).multilineTextAlignment(.center)
            }.padding(.top, 24).padding(.horizontal, 26)

            HStack(spacing: 10) {
                Text(s.shareURLStr).font(F.t(14, .semibold)).foregroundColor(P.ink).lineLimit(1).frame(maxWidth: .infinity, alignment: .leading)
                Button {
                    UIPasteboard.general.string = s.shareLink
                    withAnimation { copied = true }
                    DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) { withAnimation { copied = false } }
                } label: {
                    Text(copied ? "Copied ✓" : "Copy").font(F.t(12, .bold)).foregroundColor(.white)
                        .padding(.horizontal, 14).padding(.vertical, 8)
                        .background(copied ? P.green : P.ink).clipShape(RoundedRectangle(cornerRadius: 10))
                }.buttonStyle(.plain)
            }.padding(.horizontal, 14).padding(.vertical, 13).background(P.cream)
                .overlay(RoundedRectangle(cornerRadius: 14).stroke(P.border)).clipShape(RoundedRectangle(cornerRadius: 14))
                .padding(.horizontal, 24).padding(.top, 14)

            HStack(spacing: 10) {
                channel("WhatsApp", Color(hex: 0x1FB155), url: waURL)
                channel("Telegram", Color(hex: 0x2AA3E8), url: tgURL)
            }.padding(.horizontal, 24).padding(.top, 14)

            Button { s.go(.fLanding) } label: {
                HStack(spacing: 12) {
                    Text("👀").font(.system(size: 17, weight: .bold)).frame(width: 38, height: 38).background(P.orange).clipShape(RoundedRectangle(cornerRadius: 11))
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Preview the friend's view").font(F.t(14, .bold)).foregroundColor(P.ink)
                        Text("See what your friends will tap").font(F.t(11, .medium)).foregroundColor(Color(hex: 0x9A7B4F))
                    }
                    Spacer()
                    Text("→").font(F.t(18, .bold)).foregroundColor(P.orange)
                }.padding(.horizontal, 16).padding(.vertical, 15)
                    .background(Color(hex: 0xFFF6EC))
                    .overlay(RoundedRectangle(cornerRadius: 16).stroke(Color(hex: 0xF0C89A), style: StrokeStyle(lineWidth: 1.5, dash: [5])))
                    .clipShape(RoundedRectangle(cornerRadius: 16))
            }.buttonStyle(.plain).padding(.horizontal, 24).padding(.top, 18)

            Spacer()
            Button { s.go(.track) } label: {
                Text("Track payments").font(F.t(15, .bold)).foregroundColor(P.ink).frame(maxWidth: .infinity).padding(15)
                    .background(Color.white).overlay(RoundedRectangle(cornerRadius: 16).stroke(P.ink, lineWidth: 1.5)).clipShape(RoundedRectangle(cornerRadius: 16))
            }.buttonStyle(.plain).padding(.horizontal, 24).padding(.bottom, 24)
        }
    }
    private func channel(_ t: String, _ c: Color, url: URL?) -> some View {
        Button { if let url { openURL(url) } } label: {
            Text(t).font(F.t(13, .bold)).foregroundColor(.white).frame(maxWidth: .infinity).padding(.vertical, 14).background(c).clipShape(RoundedRectangle(cornerRadius: 14))
        }.buttonStyle(.plain).disabled(url == nil)
    }
}

struct TrackView: View {
    @EnvironmentObject var s: AppState
    var body: some View {
        VStack(spacing: 0) {
            HStack(spacing: 12) {
                BackChip { s.go(.share) }
                VStack(alignment: .leading, spacing: 2) {
                    Text("Nasi Lemak House").font(F.d(20)).foregroundColor(P.ink)
                    Text("29 Jun · Total RM 112.40").font(F.t(12, .medium)).foregroundColor(P.brown)
                }
                Spacer()
            }.padding(.horizontal, 24).padding(.top, 8)

            VStack(spacing: 10) {
                HStack(alignment: .firstTextBaseline) {
                    Text(s.collectedStr).font(F.d(18)).foregroundColor(P.green)
                    Spacer()
                    Text("of RM 112.40 collected").font(F.t(12, .medium)).foregroundColor(P.brown)
                }
                GeometryReader { g in
                    ZStack(alignment: .leading) {
                        Capsule().fill(Color(hex: 0xE6DCCD))
                        Capsule().fill(P.green).frame(width: g.size.width * s.collectedFrac)
                    }
                }.frame(height: 10)
            }.padding(16).background(P.cream).clipShape(RoundedRectangle(cornerRadius: 18)).padding(.horizontal, 24).padding(.top, 14)

            VStack(spacing: 12) {
                if let parts = s.trackParticipants {
                    ForEach(Array(parts.enumerated()), id: \.element.id) { idx, p in
                        payer(String(p.name.prefix(1)).uppercased(), avatarColors[idx % avatarColors.count],
                              p.isOwner ? "\(p.name) · your share" : p.name, s.rmSen(p.owedSen),
                              p.paid ? .paid : (p.isOwner ? .none : .pending))
                            .opacity(p.isOwner ? 0.7 : 1)
                    }
                } else {
                    payer("S", P.green, s.name, s.yourShareStr, .paid)
                    payer("J", P.purple, "Jaz", "RM 27.80", .paid)
                    payer("L", P.amber, "Lim", "RM 16.00", .pending)
                    payer("A", P.orange, "You (Aiman)", "RM 24.20 · your share", .none).opacity(0.7)
                }
            }.padding(.horizontal, 20).padding(.top, 16)

            Spacer()
            Text("Nudge Lim on WhatsApp").font(F.t(15, .bold)).foregroundColor(.white).frame(maxWidth: .infinity).padding(15)
                .background(P.ink).clipShape(RoundedRectangle(cornerRadius: 16)).padding(.horizontal, 20).padding(.bottom, 24)
        }
        .task { s.loadTrack(); s.listenTrack() }
    }
    private let avatarColors: [Color] = [P.green, P.purple, P.amber, P.orange]
    enum Status { case paid, pending, none }
    private func payer(_ initial: String, _ c: Color, _ name: String, _ sub: String, _ st: Status) -> some View {
        HStack(spacing: 12) {
            Text(initial).font(F.t(14, .bold)).foregroundColor(.white).frame(width: 38, height: 38).background(c).clipShape(Circle())
            VStack(alignment: .leading, spacing: 2) {
                Text(name).font(F.t(14, .bold)).foregroundColor(P.ink)
                Text(sub).font(F.t(12, .medium)).foregroundColor(P.brown)
            }
            Spacer()
            switch st {
            case .paid:
                HStack(spacing: 6) {
                    ZStack { Circle().fill(P.green).frame(width: 14, height: 14); Image(systemName: "checkmark").font(.system(size: 7, weight: .black)).foregroundColor(.white) }
                    Text("Paid").font(F.t(11, .bold)).foregroundColor(P.green)
                }.padding(.horizontal, 11).padding(.vertical, 6).background(Color(hex: 0xEAF6F1)).clipShape(Capsule())
            case .pending:
                Text("Pending").font(F.t(11, .bold)).foregroundColor(Color(hex: 0xB07A14)).padding(.horizontal, 11).padding(.vertical, 6).background(Color(hex: 0xFBEFD9)).clipShape(Capsule())
            case .none: EmptyView()
            }
        }
    }
}

struct SettingsView: View {
    @EnvironmentObject var s: AppState
    var body: some View {
        VStack(spacing: 0) {
            VStack(spacing: 0) {
                HStack(spacing: 12) {
                    BackChip { s.go(.capture) }
                    Text("Payment settings").font(F.d(18)).foregroundColor(P.ink)
                    Spacer()
                }.padding(.horizontal, 22).padding(.bottom, 16)
            }.background(Color.white)

            ScrollView {
                VStack(alignment: .leading, spacing: 0) {
                    sectionLabel("YOUR DUITNOW QR")
                    HStack(spacing: 16) {
                        QRView(seed: 13, cell: 3.2).padding(8).background(P.paper).overlay(RoundedRectangle(cornerRadius: 12).stroke(Color(hex: 0xEFE6D8))).clipShape(RoundedRectangle(cornerRadius: 12))
                        VStack(alignment: .leading, spacing: 3) {
                            Text("Shown to friends").font(F.t(14, .bold)).foregroundColor(P.ink)
                            Text("when they pay you back").font(F.t(12, .medium)).foregroundColor(P.brown)
                            Text("Replace QR").font(F.t(12, .bold)).foregroundColor(P.ink).padding(.horizontal, 13).padding(.vertical, 8).background(P.cream2).clipShape(RoundedRectangle(cornerRadius: 9)).padding(.top, 10)
                        }
                        Spacer()
                    }.padding(18).card(20)

                    sectionLabel("RECEIVING ACCOUNT").padding(.top, 14)
                    VStack(spacing: 0) {
                        acctRow("MB", Color(hex: 0xFFE08A), Color(hex: 0x7A5A00), "Maybank", "•••• 6721", "mb")
                        Divider().overlay(P.line)
                        acctRow("TNG", Color(hex: 0xD4E9FF), Color(hex: 0x1F6FB5), "Touch 'n Go eWallet", "•••• 4408", "tng")
                    }.card(20)
                    Text("+ Add account").font(F.t(13, .bold)).foregroundColor(P.orange).frame(maxWidth: .infinity).padding(13)

                    Button { withAnimation { s.auto.toggle() } } label: {
                        HStack {
                            VStack(alignment: .leading, spacing: 2) {
                                Text("Auto-fill amount").font(F.t(14, .bold)).foregroundColor(P.ink)
                                Text("QR opens with exact share").font(F.t(11, .medium)).foregroundColor(P.brown)
                            }
                            Spacer()
                            Toggle("", isOn: Binding(get: { s.auto }, set: { s.auto = $0 })).labelsHidden().tint(P.green)
                        }.padding(.horizontal, 16).padding(.vertical, 15).card()
                    }.buttonStyle(.plain).padding(.top, 4)
                }.padding(.horizontal, 20).padding(.top, 18).padding(.bottom, 22)
            }
        }
        .background(P.cream)
    }
    private func sectionLabel(_ t: String) -> some View {
        Text(t).font(F.t(12, .semibold)).foregroundColor(P.brown).tracking(0.5).padding(.bottom, 9)
    }
    private func acctRow(_ tag: String, _ bg: Color, _ fg: Color, _ name: String, _ num: String, _ key: String) -> some View {
        Button { withAnimation { s.acct = key } } label: {
            HStack(spacing: 12) {
                Text(tag).font(F.t(12, .heavy)).foregroundColor(fg).frame(width: 38, height: 38).background(bg).clipShape(RoundedRectangle(cornerRadius: 10))
                VStack(alignment: .leading, spacing: 2) {
                    Text(name).font(F.t(14, .bold)).foregroundColor(P.ink)
                    Text(num).font(F.t(12, .medium)).foregroundColor(P.brown)
                }
                Spacer()
                ZStack {
                    Circle().stroke(s.acct == key ? P.orange : Color(hex: 0xDCD2C4), lineWidth: 2).frame(width: 22, height: 22)
                    if s.acct == key { Circle().fill(P.orange).frame(width: 11, height: 11) }
                }
            }.padding(.horizontal, 16).padding(.vertical, 15)
        }.buttonStyle(.plain)
    }
}
