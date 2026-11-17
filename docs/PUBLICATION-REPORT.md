# Distribution overview

## Public destinations

- [Source repository](https://github.com/enesoglu/android-usb-reverse-tethering)
- [Project website](https://projects.enesoglu.com/android-usb-reverse-tethering/)
- [Versioned downloads](https://github.com/enesoglu/android-usb-reverse-tethering/releases)
- [Installation guide](INSTALLATION.md)
- [Reverse-tethering engineering guide](reverse-tethering-engineering.md)

The project is GPL-3.0-only. Public documentation and application text are English.

## Release contents

Tagged release automation builds the Windows installer, self-contained portable package, signed Android companion, corresponding source and dependency notices. Checksums and build provenance accompany the assets. Android signing identity is retained across updates; Windows Authenticode signing remains pending.

GitHub Pages publishes the static website from the source repository. The site uses no analytics, tracking cookies or external fonts. Package-manager availability may lag the GitHub release.

## Publication boundaries

Private pairing/signing state, device/session records, local paths, network identifiers and raw measurements are excluded. Public engineering notes explain mechanisms and limitations without identifying individual devices or connections. Documentation describes supported behavior rather than guaranteeing a fixed throughput or broad compatibility.

See [release engineering](RELEASING.md), [privacy](../PRIVACY.md), and [current limitations](verification.md).
