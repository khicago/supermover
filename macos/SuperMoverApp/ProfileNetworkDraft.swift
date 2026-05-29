import Foundation

struct ProfileNetworkDraft: Equatable {
    var receiverURL = ""
    var tlsCertificatePath = ""
    var tlsPrivateKeyPath = ""
    var clearReceiverURL = false
    var clearTLSIdentity = false

    var trimmedReceiverURL: String {
        receiverURL.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var trimmedTLSCertificatePath: String {
        tlsCertificatePath.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var trimmedTLSPrivateKeyPath: String {
        tlsPrivateKeyPath.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var hasPartialTLSIdentity: Bool {
        (trimmedTLSCertificatePath.isEmpty) != (trimmedTLSPrivateKeyPath.isEmpty)
    }

    var hasRequestedChange: Bool {
        clearReceiverURL ||
        clearTLSIdentity ||
        !trimmedReceiverURL.isEmpty ||
        !trimmedTLSCertificatePath.isEmpty ||
        !trimmedTLSPrivateKeyPath.isEmpty
    }

    var setArguments: [String] {
        var args: [String] = []
        if clearReceiverURL {
            args.append("--clear-receiver-url")
        } else if !trimmedReceiverURL.isEmpty {
            args += ["--receiver-url", trimmedReceiverURL]
        }
        if clearTLSIdentity {
            args.append("--clear-tls-identity")
        } else if !trimmedTLSCertificatePath.isEmpty || !trimmedTLSPrivateKeyPath.isEmpty {
            args += ["--tls-cert", trimmedTLSCertificatePath, "--tls-key", trimmedTLSPrivateKeyPath]
        }
        return args
    }

    var contextInputs: [String] {
        [
            trimmedReceiverURL,
            trimmedTLSCertificatePath,
            trimmedTLSPrivateKeyPath,
            clearReceiverURL ? "clear-receiver-url" : "",
            clearTLSIdentity ? "clear-tls-identity" : "",
        ]
    }
}
