# Community announcement draft

Use this only where project announcements are allowed. Disclose that you maintain the app, choose one relevant community, and follow its current rules. Do not post the same message repeatedly or imply independent endorsement.

## Title

I built USB Relay: a free Windows GUI for sharing PC internet with Android over USB (public alpha)

## Body

I am the developer of USB Relay, a GPLv3 app for Windows-to-Android USB reverse tethering. It uses its own Go/gVisor backend and Android VPN companion. It is not a Gnirehtet GUI wrapper.

It has a native Windows interface, tray controls, optional startup, explicit phone pairing, and automatic reconnect for trusted phones. Android root is not required; USB debugging and the first VPN approval are required.

This is a public alpha with a more efficient USB data path and an open engineering guide. Throughput and compatibility depend on the device and network; a fixed speed is not guaranteed. IPv4 TCP/UDP is supported; general IPv6 and ICMP forwarding are not. Windows binaries are currently unsigned.

I would appreciate reproducible installation reports and feedback from other devices. Please remove device serials and private data from reports.

- Website: https://enesoglu.github.io/android-usb-reverse-tethering/
- Source, downloads, and guide: https://github.com/enesoglu/android-usb-reverse-tethering

## Software directory description

USB Relay is a free, open-source Windows reverse-tethering app that shares a computer's Ethernet or Wi-Fi internet connection with Android over USB. It provides a native GUI, tray controls, optional Windows startup, trusted-phone automatic reconnect, and a TLS 1.3 authenticated tunnel. Android root is not needed; USB debugging and VPN consent are required. GPL-3.0-only. Currently public alpha, Windows x64 and Android 10+, IPv4 TCP/UDP.

Potential related products: Gnirehtet and Tetrd, specifically their PC-to-Android reverse-tethering function. Do not describe PdaNet+ or mobile-data sharing as equivalent without verifying direction and features.

AlternativeTo requires a verified email and editorial acceptance. Confirm that its editors accept the current public-alpha stage before submitting; do not relabel the app as stable or beta to bypass eligibility.
