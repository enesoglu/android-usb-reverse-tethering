# Contributing

All code, UI text, documentation, issues and pull requests should be in English. The project is GPL-3.0-only; preserve upstream dependency notices. Never commit a keystore, DPAPI file, signing password, phone serial, real traffic capture or personal diagnostic data.

## Development requirements

Use Windows x64, Go 1.27.4+, .NET 10 SDK, Python 3.10+, Java 21, Android SDK platform 36/build-tools 37.0.0, and ADB for device tests. Android Studio is optional if you install these tools separately. The scripts default to Android Studio's JDK; pass `-Jdk` and `-Sdk` for other locations.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ./check.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File ./build-desktop.ps1
```

Exit the app before rebuilding its bundle. The development APK uses a locally generated, ignored development key. The public package is `io.github.enesoglu.usbrelay`, while Java implementation classes retain their internal `dev.usbreverse.lab` namespace. The public package and Windows state directory are separate from development installations. Do not work around APK signature or pairing failures by disabling checks.

`check.ps1` covers Go tests/vet, Python delivery validation, 4,376 Java framing cases, Android reconnect policy, signed APK compilation, WPF formatting/build, tray/restore/startup behavior, companion tampering, and safe SDK-archive extraction. Use `go test -race ./...` with a C compiler for concurrency checks. Actual Android VPN consent, OEM policies, cable reconnect and real throughput still require a phone; compilation is not evidence those cases passed.

## Change expectations

Keep changes small, explain the user-visible problem and test the behavior at the appropriate boundary. Keep network/packet logic, ADB device policy, persistent trust and UI independent. Validate failures as well as successful pairing. Use synthetic payloads for benchmarks and require receiver counts and hashes before reporting goodput.

## Build modified clients

Build from source using your own development key. Android cannot update an existing APK under the same app ID with an unrelated signing key; intentionally uninstall the conflicting copy first, then rebuild/install and pair again. This deletes that app's identity/settings. The desktop embeds expected companion hashes at build time, so rebuild the desktop after changing the APK or relay. You do not need the maintainer's private release key to build or run modified software.

## Public releases

See [release engineering](docs/RELEASING.md). A public release requires the stable private project key; the build refuses development-key fallback. Never submit key material through an issue or pull request. Go dependencies are pinned, and the corresponding-source ZIP includes vendored dependency source for the tagged build. Self-contained .NET and Go license notices are included in binary packages.
