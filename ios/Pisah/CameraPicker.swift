import SwiftUI
import AVFoundation
import PhotosUI
import UIKit

// Full-screen receipt camera with live preview and album pick.
struct ReceiptCameraView: View {
    let onImage: (Data) -> Void
    @Environment(\.dismiss) private var dismiss
    @StateObject private var camera = CameraSession()
    @State private var pickedPhoto: PhotosPickerItem?

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()

            if camera.isAvailable {
                CameraPreviewView(session: camera.session)
                    .ignoresSafeArea()
            } else {
                VStack(spacing: 12) {
                    Image(systemName: "camera.fill")
                        .font(.system(size: 36))
                        .foregroundStyle(.white.opacity(0.5))
                    Text("Camera unavailable")
                        .font(.subheadline.weight(.medium))
                        .foregroundStyle(.white.opacity(0.7))
                }
            }

            VStack {
                HStack {
                    Button { dismiss() } label: {
                        Image(systemName: "xmark")
                            .font(.system(size: 16, weight: .semibold))
                            .foregroundStyle(.white)
                            .frame(width: 40, height: 40)
                            .background(.black.opacity(0.45))
                            .clipShape(Circle())
                    }
                    Spacer()
                }
                .padding(.horizontal, 20)
                .padding(.top, 8)

                Spacer()

                HStack(alignment: .center) {
                    PhotosPicker(selection: $pickedPhoto, matching: .images) {
                        Image(systemName: "photo.on.rectangle")
                            .font(.system(size: 20))
                            .foregroundStyle(.white)
                            .frame(width: 52, height: 52)
                            .background(.black.opacity(0.45))
                            .clipShape(Circle())
                    }

                    Spacer()

                    if camera.isAvailable {
                        Button {
                            camera.capturePhoto { data in
                                guard let data else { return }
                                onImage(data)
                                dismiss()
                            }
                        } label: {
                            ZStack {
                                Circle().stroke(.white, lineWidth: 4).frame(width: 76, height: 76)
                                Circle().fill(.white).frame(width: 62, height: 62)
                            }
                        }
                        .disabled(!camera.isReady)
                        .opacity(camera.isReady ? 1 : 0.5)
                    }

                    Spacer()

                    if camera.isAvailable {
                        Button { camera.flipCamera() } label: {
                            Image(systemName: "arrow.triangle.2.circlepath.camera")
                                .font(.system(size: 20))
                                .foregroundStyle(.white)
                                .frame(width: 52, height: 52)
                                .background(.black.opacity(0.45))
                                .clipShape(Circle())
                        }
                        .disabled(!camera.isReady)
                        .opacity(camera.isReady ? 1 : 0.5)
                    } else {
                        Color.clear.frame(width: 52, height: 52)
                    }
                }
                .padding(.horizontal, 36)
                .padding(.bottom, 40)
            }
        }
        .statusBarHidden(true)
        .onAppear { camera.start() }
        .onDisappear { camera.stop() }
        .onChange(of: pickedPhoto) { _, item in
            guard let item else { return }
            Task {
                if let data = try? await item.loadTransferable(type: Data.self),
                   let jpeg = jpegData(from: data) {
                    onImage(jpeg)
                    dismiss()
                }
            }
        }
    }

    private func jpegData(from data: Data) -> Data? {
        guard let img = UIImage(data: data) else { return nil }
        return img.jpegData(compressionQuality: 0.8)
    }
}

private struct CameraPreviewView: UIViewRepresentable {
    let session: AVCaptureSession

    func makeUIView(context: Context) -> PreviewUIView {
        let view = PreviewUIView()
        view.previewLayer.session = session
        view.previewLayer.videoGravity = .resizeAspectFill
        return view
    }

    func updateUIView(_ uiView: PreviewUIView, context: Context) {
        uiView.previewLayer.session = session
    }

    final class PreviewUIView: UIView {
        override class var layerClass: AnyClass { AVCaptureVideoPreviewLayer.self }
        var previewLayer: AVCaptureVideoPreviewLayer { layer as! AVCaptureVideoPreviewLayer }

        override func layoutSubviews() {
            super.layoutSubviews()
            previewLayer.frame = bounds
        }
    }
}

@MainActor
final class CameraSession: NSObject, ObservableObject {
    let session = AVCaptureSession()
    @Published private(set) var isReady = false
    @Published private(set) var isAvailable = AVCaptureDevice.default(for: .video) != nil

    private let output = AVCapturePhotoOutput()
    private var captureCompletion: ((Data?) -> Void)?
    private var position: AVCaptureDevice.Position = .back
    private var isConfigured = false

    func start() {
        guard isAvailable else { return }
        Task { await configureAndRun() }
    }

    func stop() {
        guard session.isRunning else { return }
        session.stopRunning()
        isReady = false
    }

    func flipCamera() {
        position = position == .back ? .front : .back
        Task { await reconfigureInput() }
    }

    func capturePhoto(completion: @escaping (Data?) -> Void) {
        guard isReady else { completion(nil); return }
        captureCompletion = completion
        let settings = AVCapturePhotoSettings()
        if output.availablePhotoCodecTypes.contains(.jpeg) {
            settings.flashMode = .auto
        }
        output.capturePhoto(with: settings, delegate: self)
    }

    private func configureAndRun() async {
        guard !isConfigured else {
            if !session.isRunning { session.startRunning() }
            isReady = session.isRunning
            return
        }

        let granted = await requestAccess()
        guard granted else { return }

        session.beginConfiguration()
        session.sessionPreset = .photo

        guard let device = cameraDevice(for: position),
              let input = try? AVCaptureDeviceInput(device: device),
              session.canAddInput(input) else {
            session.commitConfiguration()
            return
        }
        session.addInput(input)

        guard session.canAddOutput(output) else {
            session.commitConfiguration()
            return
        }
        session.addOutput(output)
        session.commitConfiguration()

        isConfigured = true
        session.startRunning()
        isReady = session.isRunning
    }

    private func reconfigureInput() async {
        session.beginConfiguration()
        for input in session.inputs {
            session.removeInput(input)
        }
        if let device = cameraDevice(for: position),
           let input = try? AVCaptureDeviceInput(device: device),
           session.canAddInput(input) {
            session.addInput(input)
        }
        session.commitConfiguration()
    }

    private func cameraDevice(for position: AVCaptureDevice.Position) -> AVCaptureDevice? {
        AVCaptureDevice.default(.builtInWideAngleCamera, for: .video, position: position)
    }

    private func requestAccess() async -> Bool {
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized: return true
        case .notDetermined:
            return await withCheckedContinuation { cont in
                AVCaptureDevice.requestAccess(for: .video) { cont.resume(returning: $0) }
            }
        default: return false
        }
    }
}

extension CameraSession: AVCapturePhotoCaptureDelegate {
    nonisolated func photoOutput(_ output: AVCapturePhotoOutput,
                                 didFinishProcessingPhoto photo: AVCapturePhoto,
                                 error: Error?) {
        let data = photo.fileDataRepresentation()
        Task { @MainActor in
            captureCompletion?(data)
            captureCompletion = nil
        }
    }
}

// Photo library picker for the capture screen gallery button.
struct PhotoLibraryPicker: View {
    let onImage: (Data) -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var pickedPhoto: PhotosPickerItem?

    var body: some View {
        NavigationStack {
            VStack(spacing: 16) {
                PhotosPicker(selection: $pickedPhoto, matching: .images) {
                    Label("Choose from Photos", systemImage: "photo.on.rectangle.angled")
                        .font(.headline)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 16)
                        .background(Color(hex: 0xF5EFE6))
                        .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
                }
                .buttonStyle(.plain)
                .padding(.horizontal, 24)
                .padding(.top, 24)

                Spacer()
            }
            .navigationTitle("Gallery")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
            .background(Color(hex: 0xFAF6F0).ignoresSafeArea())
        }
        .onChange(of: pickedPhoto) { _, item in
            guard let item else { return }
            Task {
                if let data = try? await item.loadTransferable(type: Data.self),
                   let img = UIImage(data: data),
                   let jpeg = img.jpegData(compressionQuality: 0.8) {
                    onImage(jpeg)
                    dismiss()
                }
            }
        }
    }
}
