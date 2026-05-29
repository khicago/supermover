// swift-tools-version: 6.0

import PackageDescription

let package = Package(
    name: "SuperMoverApp",
    defaultLocalization: "en",
    platforms: [
        .macOS(.v14),
    ],
    products: [
        .library(
            name: "SuperMoverAppSupport",
            targets: ["SuperMoverAppSupport"]
        ),
        .executable(
            name: "SuperMoverApp",
            targets: ["SuperMoverApp"]
        ),
        .executable(
            name: "SuperMoverPackagedAppAudit",
            targets: ["SuperMoverPackagedAppAudit"]
        ),
    ],
    targets: [
        .target(
            name: "SuperMoverAppSupport",
            path: "SuperMoverAppSupport"
        ),
        .executableTarget(
            name: "SuperMoverApp",
            dependencies: ["SuperMoverAppSupport"],
            path: "SuperMoverApp",
            resources: [
                .process("Resources"),
            ]
        ),
        .executableTarget(
            name: "SuperMoverPackagedAppAudit",
            dependencies: ["SuperMoverAppSupport"],
            path: "SuperMoverPackagedAppAudit"
        ),
        .testTarget(
            name: "SuperMoverAppTests",
            dependencies: ["SuperMoverApp", "SuperMoverAppSupport"],
            path: "SuperMoverAppTests"
        ),
    ]
)
