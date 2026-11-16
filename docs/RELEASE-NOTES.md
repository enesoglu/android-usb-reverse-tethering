# USB Relay 0.3.0-alpha.1

Share a Windows PC's internet with an Android phone over USB: rootless reverse tethering with a native tray application, trusted-device reconnect and an English setup guide.

## Choose a download

- **Windows x64 Setup.exe**: recommended per-user installer.
- **Windows x64 Portable.zip**: extract the whole archive and run UsbRelay.exe.
- **Android.apk**: optional; normal Windows setup installs the bundled phone companion automatically.
- **Source.zip**: corresponding project/dependency source and build instructions.
- **SHA256SUMS.txt**: verify downloaded files.

Start with [the installation guide](https://github.com/enesoglu/android-usb-reverse-tethering/blob/main/docs/INSTALLATION.md). Both Windows packages include .NET; Google platform tools are obtained separately through consent-gated setup or an existing adb.exe.

This is an **alpha**. Windows binaries are not Authenticode-signed. Throughput is unfinished: the measured default was about 128 Mbps on a receiver-hashed 512 MiB VPN transfer, versus 268–273 Mbps for raw ADB. Loaded UDP p95 was about 20 ms and all 531 probes arrived in that default test. These are local synthetic measurements, not a gigabit or public-internet-speed guarantee.

GPL-3.0-only, no warranty. The matching source is available from this release/tag. Do not share private signing or pairing files in bug reports.
