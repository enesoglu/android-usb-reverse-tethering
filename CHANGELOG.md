# Changelog

## Unreleased

- Add a separate start-minimized preference that opens directly in the system tray.
- Honor the saved window preference for Windows sign-in startup.

## 0.3.0-alpha.1

First public alpha of USB Relay for Windows x64 and Android 10+.

- Per-user Windows installer and self-contained portable package.
- English computer-and-phone setup guide and offline help.
- Setup assistant with optional Google-hosted, hash-verified platform-tools download after terms consent.
- Separate public Android application ID and persistent project release signing.
- Native tray, optional sign-in startup, trusted-device reconnect, mutual TLS and protected local identities.
- CI checks, repeatable package builds, release checksums and corresponding source archive.

### Known limitations

VPN throughput remains below the raw ADB control. The measured default was approximately 128 Mbps for a hash-verified 512 MiB transfer with loaded UDP p95 around 20 ms. IPv6, ICMP, ARM64-native builds, broad OEM validation and Windows code signing are not part of this alpha. See the verification report for unverified cases.
