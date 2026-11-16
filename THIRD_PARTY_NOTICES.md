# Third-party notices

USB Relay's own code is GPL-3.0-only. The following dependencies retain their own licenses and copyright notices; the project license does not erase them.

- **gVisor netstack**: Apache-2.0, pinned in go.mod/go.sum. License and any upstream NOTICE are in third_party/. Source: https://github.com/google/gvisor.
- **google/btree**: Apache-2.0. Source: https://github.com/google/btree.
- **golang.org/x/sys, x/time, x/exp**: BSD-3-Clause. Their license texts are in third_party/.
- **Go runtime and standard library**: BSD-style Go license, included as third_party/go-LICENSE.txt. Source: https://go.dev/LICENSE.
- **Microsoft .NET runtime and Windows Desktop runtime**: unmodified runtime components distributed with the self-contained Windows package. Their runtime-pack LICENSE and THIRD-PARTY-NOTICES files are copied into that package's third_party/ folder. Sources: https://github.com/dotnet/runtime, https://github.com/dotnet/wpf and https://github.com/dotnet/winforms. Framework-dependent development builds use the separately installed runtime.
- **Inno Setup**: used to build the Windows installer. The setup/uninstall components retain upstream attribution; the pinned license is included in third_party/inno-setup-LICENSE.txt. Source: https://github.com/jrsoftware/issrc/tree/is-7_1_1. The compiler itself is not bundled with the app.

Google Android SDK/platform-tools, Android Studio and the JDK are external tools, not mirrored as project release assets. The optional setup download goes directly to Google after user consent and preserves the downloaded package's notices. An existing trusted adb.exe can be used instead. AOSP adb has its own open-source licensing, while the complete Google SDK download can carry additional terms; see docs/distribution-research.md.

Gnirehtet and Tetrd are not bundled. Historical diagnostics may invoke a separately installed gnirehtet executable; its engine is not part of the application.

The corresponding-source release ZIP includes the tagged project and pinned Go dependency sources. Build instructions explain how to create modified clients with your own signing key. No maintainer private key is required or provided.
