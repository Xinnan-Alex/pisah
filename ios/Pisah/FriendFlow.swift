import SwiftUI

struct FriendLandingView: View {
    @EnvironmentObject var s: AppState
    var body: some View {
        VStack(spacing: 0) {
            FauxBrowserBar()
            VStack(spacing: 0) {
                Text("A").font(F.t(19, .bold)).foregroundColor(.white).frame(width: 52, height: 52).background(P.orange).clipShape(Circle()).padding(.bottom, 14)
                Text("Aiman invited you\nto split the bill").font(F.d(24)).foregroundColor(P.ink).multilineTextAlignment(.center).lineSpacing(3)
                VStack(spacing: 6) {
                    Text("Nasi Lemak House").font(F.d(16)).foregroundColor(P.ink)
                    HStack(spacing: 8) {
                        Text("8 items"); Text("·"); Text("Total RM 112.40")
                    }.font(F.t(12, .medium)).foregroundColor(P.brown)
                }.padding(16).frame(maxWidth: .infinity)
                    .background(Color.white).overlay(RoundedRectangle(cornerRadius: 18).stroke(P.line)).clipShape(RoundedRectangle(cornerRadius: 18))
                    .shadow(color: P.ink.opacity(0.06), radius: 9, y: 6).padding(.top, 18)
            }.padding(.horizontal, 28).padding(.top, 34).padding(.bottom, 24)
                .background(LinearGradient(colors: [P.peach, .white], startPoint: .top, endPoint: .bottom))

            VStack(alignment: .leading, spacing: 0) {
                Text("What's your name?").font(F.t(13, .semibold)).foregroundColor(P.ink).padding(.bottom, 8)
                TextField("e.g. Sara", text: $s.name)
                    .font(F.t(15, .semibold)).foregroundColor(P.ink).padding(.horizontal, 16).padding(.vertical, 15)
                    .background(P.cream).overlay(RoundedRectangle(cornerRadius: 14).stroke(P.border, lineWidth: 1.5)).clipShape(RoundedRectangle(cornerRadius: 14))
                    .padding(.bottom, 14)
                PrimaryButton(title: "Join the split →") { s.joinSplit() }
                Text("No app needed · pay by DuitNow QR").font(F.t(11, .medium)).foregroundColor(Color(hex: 0xA99B89)).frame(maxWidth: .infinity).padding(.top, 13)
            }.padding(.horizontal, 28).padding(.top, 8).padding(.bottom, 28)
            Spacer()
        }
    }
}

struct FriendPickView: View {
    @EnvironmentObject var s: AppState
    var body: some View {
        VStack(spacing: 0) {
            FauxBrowserBar()
            VStack(alignment: .leading, spacing: 2) {
                Text("Hi \(s.name) 👋").font(F.d(20)).foregroundColor(P.ink)
                Text("Tap the items you ordered").font(F.t(13, .medium)).foregroundColor(P.brown)
            }.frame(maxWidth: .infinity, alignment: .leading).padding(.horizontal, 22).padding(.vertical, 16).background(Color.white)

            ScrollView {
                VStack(spacing: 10) {
                    selectable(on: s.nl, name: "Nasi Lemak Ayam", price: "RM 12.50") { withAnimation { s.toggleItem("nl") } }
                    selectable(on: s.tt, name: "Teh Tarik", price: "RM 2.80") { withAnimation { s.toggleItem("tt") } }
                    claimedMilo
                    sotongCard
                }.padding(.horizontal, 18).padding(.vertical, 14)
            }

            HStack {
                VStack(alignment: .leading, spacing: 0) {
                    Text("Your share").font(F.t(11, .medium)).foregroundColor(P.brown)
                    Text(s.yourShareStr).font(F.d(19)).foregroundColor(P.ink)
                }
                Spacer()
                Button { s.go(.fShare) } label: {
                    Text("Continue →").font(F.t(14, .bold)).foregroundColor(.white).padding(.horizontal, 22).padding(.vertical, 13).background(P.orange).clipShape(RoundedRectangle(cornerRadius: 14))
                }.buttonStyle(.plain)
            }.padding(.horizontal, 22).padding(.top, 15).padding(.bottom, 18)
                .background(Color.white.shadow(.drop(color: P.ink.opacity(0.06), radius: 9, y: -6)))
        }
        .background(P.cream)
    }

    private func selectable(on: Bool, name: String, price: String, tap: @escaping () -> Void) -> some View {
        Button(action: tap) {
            HStack(spacing: 11) {
                ZStack {
                    RoundedRectangle(cornerRadius: 8).fill(on ? P.orange : .clear).frame(width: 24, height: 24)
                        .overlay(on ? nil : RoundedRectangle(cornerRadius: 8).stroke(Color(hex: 0xDCD2C4), lineWidth: 2))
                    if on { Image(systemName: "checkmark").font(.system(size: 10, weight: .bold)).foregroundColor(.white) }
                }
                VStack(alignment: .leading, spacing: 2) {
                    Text(name).font(F.t(14, .bold)).foregroundColor(P.ink)
                    Text(price).font(F.t(11, .medium)).foregroundColor(P.brown)
                }
                Spacer()
                if on { Text("You").font(F.t(12, .bold)).foregroundColor(P.orange) }
            }.padding(.horizontal, 14).padding(.vertical, 13)
                .background(Color.white)
                .overlay(RoundedRectangle(cornerRadius: 16).stroke(on ? P.orange : P.line, lineWidth: on ? 2 : 1.5))
                .clipShape(RoundedRectangle(cornerRadius: 16))
                .shadow(color: on ? P.orange.opacity(0.12) : .clear, radius: 6, y: 4)
        }.buttonStyle(.plain)
    }

    private var claimedMilo: some View {
        HStack(spacing: 11) {
            RoundedRectangle(cornerRadius: 8).stroke(Color(hex: 0xDCD2C4), lineWidth: 2).frame(width: 24, height: 24)
            VStack(alignment: .leading, spacing: 2) {
                Text("Milo Ais").font(F.t(14, .bold)).foregroundColor(P.ink)
                Text("RM 4.50").font(F.t(11, .medium)).foregroundColor(P.brown)
            }
            Spacer()
            HStack(spacing: 6) {
                Text("J").font(F.t(11, .bold)).foregroundColor(.white).frame(width: 24, height: 24).background(P.green).clipShape(Circle()).overlay(Circle().stroke(.white, lineWidth: 2))
                Text("Jaz").font(F.t(11, .medium)).foregroundColor(P.brown)
            }
        }.padding(.horizontal, 14).padding(.vertical, 13).background(Color.white)
            .overlay(RoundedRectangle(cornerRadius: 16).stroke(P.line, lineWidth: 1.5)).clipShape(RoundedRectangle(cornerRadius: 16)).opacity(0.75)
    }

    private var sotongCard: some View {
        VStack(spacing: 10) {
            HStack {
                HStack(spacing: 8) {
                    Text("SHARED").font(F.t(10, .bold)).foregroundColor(P.green).padding(.horizontal, 8).padding(.vertical, 3).background(Color(hex: 0xEAF6F1)).clipShape(RoundedRectangle(cornerRadius: 6))
                    Text("Sambal Sotong").font(F.t(14, .bold)).foregroundColor(P.ink)
                }
                Spacer()
                Text("RM 18.00").font(F.t(12, .semibold)).foregroundColor(P.brown)
            }
            HStack {
                HStack(spacing: 0) {
                    Text("J").font(F.t(10, .bold)).foregroundColor(.white).frame(width: 24, height: 24).background(P.green).clipShape(Circle()).overlay(Circle().stroke(.white, lineWidth: 2))
                    Text("L").font(F.t(10, .bold)).foregroundColor(.white).frame(width: 24, height: 24).background(P.purple).clipShape(Circle()).overlay(Circle().stroke(.white, lineWidth: 2)).offset(x: -8)
                    Text(s.sotong ? "split 3 ways" : "split 2 ways").font(F.t(11, .medium)).foregroundColor(P.brown).offset(x: -8 + 9)
                }
                Spacer()
                Button { withAnimation { s.toggleItem("sotong") } } label: {
                    Text(s.sotong ? "✓ You're in" : "+ I'm in").font(F.t(12, .bold))
                        .foregroundColor(s.sotong ? P.green : .white)
                        .padding(.horizontal, 14).padding(.vertical, s.sotong ? 6 : 7)
                        .background(s.sotong ? Color(hex: 0xEAF6F1) : P.orange)
                        .overlay(s.sotong ? Capsule().stroke(P.green, lineWidth: 1.5) : nil)
                        .clipShape(Capsule())
                }.buttonStyle(.plain)
            }
        }.padding(.horizontal, 14).padding(.vertical, 13).background(Color.white)
            .overlay(RoundedRectangle(cornerRadius: 16).stroke(s.sotong ? P.green : P.border, lineWidth: s.sotong ? 2 : 1.5))
            .clipShape(RoundedRectangle(cornerRadius: 16))
            .shadow(color: s.sotong ? P.green.opacity(0.12) : .clear, radius: 6, y: 4)
    }
}

struct FriendShareView: View {
    @EnvironmentObject var s: AppState
    var body: some View {
        VStack(spacing: 0) {
            FauxBrowserBar()
            VStack(alignment: .leading, spacing: 3) {
                Text("Here's your share").font(F.d(22)).foregroundColor(P.ink)
                Text("Going to Aiman · Nasi Lemak House").font(F.t(13, .medium)).foregroundColor(P.brown)
            }.frame(maxWidth: .infinity, alignment: .leading).padding(.horizontal, 26).padding(.top, 20).padding(.bottom, 6)

            VStack(spacing: 0) {
                ForEach(Array(s.shareLines.enumerated()), id: \.offset) { _, line in
                    HStack {
                        Text(line.0).font(F.t(13, .semibold)).foregroundColor(P.ink)
                        Spacer()
                        Text(line.1).font(F.t(13, .bold)).foregroundColor(P.ink)
                    }.padding(.vertical, 11)
                    Divider().overlay(P.border)
                }
                HStack {
                    Text("Your part of SST & service").font(F.t(13, .semibold)).foregroundColor(P.brown)
                    Spacer()
                    Text(s.taxStr).font(F.t(13, .bold)).foregroundColor(P.ink)
                }.padding(.vertical, 11)
            }.padding(.horizontal, 14).background(P.cream).clipShape(RoundedRectangle(cornerRadius: 18)).padding(.horizontal, 24).padding(.top, 14)

            HStack {
                Text("You owe").font(F.t(14, .bold)).foregroundColor(.white.opacity(0.85))
                Spacer()
                Text(s.yourShareStr).font(F.d(26)).foregroundColor(.white)
            }.padding(.horizontal, 20).padding(.vertical, 18).background(P.green).clipShape(RoundedRectangle(cornerRadius: 20)).padding(.horizontal, 24).padding(.top, 8)

            Spacer()
            PrimaryButton(title: "Pay \(s.yourShareStr) →") { s.go(.fPay) }.padding(.horizontal, 24).padding(.bottom, 24)
        }
        .task { s.loadShare() }
    }
}

struct FriendPayView: View {
    @EnvironmentObject var s: AppState
    var body: some View {
        VStack(spacing: 0) {
            FauxBrowserBar()
            VStack(spacing: 2) {
                Text("Pay Aiman").font(F.t(13, .medium)).foregroundColor(P.brown)
                Text(s.yourShareStr).font(F.d(31)).foregroundColor(P.ink)
            }.padding(.top, 22)

            VStack(spacing: 0) {
                HStack(spacing: 7) {
                    Circle().fill(P.orange).frame(width: 9, height: 9)
                    Text("DuitNow QR").font(F.t(13, .heavy)).foregroundColor(P.ink).tracking(0.5)
                }.padding(.bottom, 14)
                Group {
                    if let url = s.ownerQrURL {
                        AsyncImage(url: url) { $0.resizable().scaledToFit() } placeholder: { QRView(seed: 7, cell: 5) }
                            .frame(width: 125, height: 125)
                    } else {
                        QRView(seed: 7, cell: 5) // owner hasn't uploaded a DuitNow QR — faux placeholder
                    }
                }.padding(16).background(Color.white).clipShape(RoundedRectangle(cornerRadius: 16)).shadow(color: P.ink.opacity(0.08), radius: 8, y: 6)
                Text("AIMAN BIN ABDULLAH").font(F.t(13, .bold)).foregroundColor(P.ink).padding(.top, 14)
                Text("Scan with any bank or e-wallet app").font(F.t(11, .medium)).foregroundColor(P.brown).padding(.top, 2)
            }.padding(20).frame(maxWidth: .infinity).background(P.paper).overlay(RoundedRectangle(cornerRadius: 22).stroke(Color(hex: 0xEFE6D8))).clipShape(RoundedRectangle(cornerRadius: 22)).padding(.horizontal, 24).padding(.top, 16)

            Spacer()
            VStack(spacing: 10) {
                Button { s.pay() } label: {
                    Text("I've paid ✓").font(F.t(15, .bold)).foregroundColor(.white).frame(maxWidth: .infinity).padding(15).background(P.green).clipShape(RoundedRectangle(cornerRadius: 16))
                }.buttonStyle(.plain)
                Text("Save QR to gallery").font(F.t(13, .bold)).foregroundColor(P.brown)
            }.padding(.horizontal, 24).padding(.bottom, 24)
        }
    }
}

struct FriendDoneView: View {
    @EnvironmentObject var s: AppState
    var body: some View {
        VStack(spacing: 0) {
            FauxBrowserBar()
            Spacer()
            VStack(spacing: 0) {
                ZStack {
                    Circle().fill(P.green).frame(width: 88, height: 88).shadow(color: P.green.opacity(0.35), radius: 15, y: 12)
                    Image(systemName: "checkmark").font(.system(size: 34, weight: .heavy)).foregroundColor(.white)
                }.padding(.bottom, 22)
                Text("All done, \(s.name)!").font(F.d(26)).foregroundColor(P.ink)
                (Text("You marked ").font(F.t(14, .medium)) + Text(s.yourShareStr).font(F.t(14, .bold)) + Text(" as paid to Aiman.\nHe'll get a notification to confirm.").font(F.t(14, .medium)))
                    .foregroundColor(P.brown).multilineTextAlignment(.center).padding(.top, 8).lineSpacing(3)
                HStack(spacing: 10) {
                    Text("A").font(F.t(13, .bold)).foregroundColor(.white).frame(width: 32, height: 32).background(P.orange).clipShape(Circle())
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Nasi Lemak House").font(F.t(13, .bold)).foregroundColor(P.ink)
                        Text("Split with Aiman & 3 others").font(F.t(11, .medium)).foregroundColor(P.brown)
                    }
                }.padding(.horizontal, 20).padding(.vertical, 14).background(Color.white).overlay(RoundedRectangle(cornerRadius: 16).stroke(Color(hex: 0xE3EFE9))).clipShape(RoundedRectangle(cornerRadius: 16)).shadow(color: P.ink.opacity(0.05), radius: 9, y: 6).padding(.top, 24)
            }.padding(.horizontal, 30)
            Spacer()
            Button { s.go(.track) } label: {
                Text("See it land in Aiman's view →").font(F.t(15, .bold)).foregroundColor(.white).frame(maxWidth: .infinity).padding(15).background(P.ink).clipShape(RoundedRectangle(cornerRadius: 16))
            }.buttonStyle(.plain).padding(.horizontal, 24).padding(.bottom, 28)
        }
        .background(LinearGradient(colors: [Color(hex: 0xEAF6F1), .white], startPoint: .top, endPoint: .bottom))
    }
}
