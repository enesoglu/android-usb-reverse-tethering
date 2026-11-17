# USB Relay 0.4.0-alpha.1

This alpha improves the USB data path, startup behavior and developer documentation.

## What's changed

- More efficient packet sizing and batching, while retaining compatibility with older framing.
- Bounded TCP backpressure and reserved queue headroom for UDP.
- Improved Android VPN worker scheduling.
- Optional startup directly in the system tray.
- A public engineering guide covering USB negotiation, Android VPN/TUN, ADB, accessory transport and flow control.
- Simplified public documentation and generic interface examples without device/session records.

## Downloads

Choose **Windows x64 Setup.exe** for installation or **Portable.zip** for a self-contained folder. Windows installs the bundled Android companion automatically; the separate APK is available for manual use. Source, checksums and build metadata are included with this release.

[Installation guide](https://github.com/enesoglu/android-usb-reverse-tethering/blob/main/docs/INSTALLATION.md) · [Engineering guide](https://github.com/enesoglu/android-usb-reverse-tethering/blob/main/docs/reverse-tethering-engineering.md)

Throughput varies by USB link, device scheduling and network conditions; there is no fixed minimum or gigabit guarantee. Download consistency and loaded latency remain work in progress. General IPv6 and ICMP forwarding are not implemented. Android permits one VPN per profile, and Windows binaries are not Authenticode-signed.

GPL-3.0-only, no warranty. Existing Android installations retain the release signing identity and can update normally. Never include private signing/pairing state or device identifiers in public reports.
