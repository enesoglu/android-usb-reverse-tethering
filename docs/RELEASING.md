# Release engineering

The public application ID is `io.github.enesoglu.usbrelay`. Its independent Windows identity/pairing state is under `%LOCALAPPDATA%/UsbRelay`. Development instances retain their original package and state. Exit old desktop instances before release tests.

## Signing material

`scripts/New-ReleaseSigningKey.ps1` creates a private RSA release key under `%LOCALAPPDATA%/UsbRelayMaintainer/signing`, refuses to overwrite an existing identity, and protects its random password with current-user DPAPI. Only the public certificate belongs in `packaging/android-signing-certificate.pem`. The APK signer must match that committed certificate. Android APK signing is separate from each device's runtime TLS key.

Back up the encrypted keystore and store its password independently in a secure password manager or encrypted recovery backup. A DPAPI password file alone is tied to the original Windows user and is not a portable recovery method. Do not depend on GitHub Secrets as a readable backup; GitHub does not return secret values. Losing the signing key can prevent normal updates to existing installations.

The GitHub release workflow uses `ANDROID_KEYSTORE_BASE64` and `ANDROID_KEYSTORE_PASSWORD` secrets only for trusted tag releases. Base64 is encoding, not encryption; GitHub protects stored secrets. Fork PR builds never receive them. Maintain account MFA, review workflow changes, and do not print or pass secrets in command-line arguments.

## Build

Install the pinned, signature-verified Inno Setup compiler using `scripts/Install-InnoSetup.ps1`. Local release builds can read the maintainer's DPAPI-protected signing material. CI instead restores the keystore into its temporary directory and supplies password/keystore variables through the environment.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/Build-Release.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/Test-ReleasePackage.ps1
```

The release build creates a self-contained Windows x64 ZIP, a per-user English installer, a signed Android APK, BUILD-INFO, corresponding source with pinned Go dependency sources, and SHA256SUMS. It runs the native desktop integrity/tray and safe-extraction tests. The .NET runtime version is pinned in version.json and requires regular security updates. Google platform tools are not redistributed in these assets.

The first public release is an alpha and is not Windows code-signed. Do not claim independent auditing, broad OEM coverage, gigabit speeds or release reproducibility beyond the evidence in the verification report. Inspect installer/update/uninstaller behavior and real-phone permissions before publishing a new release.

## Publish

Update version.json, the desktop version, Android version code and CHANGELOG. Commit only reviewed source, confirm CI is green, then push a matching `v<version>` tag. The release workflow validates the tag/version match, builds signed artifacts, attests their provenance when available, and publishes a GitHub prerelease. The release page links the matching source, guides, checksums and APK certificate.

No private notes, local reports, signing files, Google SDK ZIP or user state may enter a source commit or release archive. `packaging/public-docs.txt` explicitly lists the documentation shipped with binaries.
