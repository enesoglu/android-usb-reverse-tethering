# Install USB Relay: Windows and Android

This guide shares the computer's internet with the phone. It does not share the phone's mobile data with the computer. Keep the phone unlocked during the first connection.

## Before you start

You need a Windows x64 computer with working internet, an Android 10+ phone, and a USB cable that carries data. A cable's charging wattage does not tell you its data speed. Windows 11 and Galaxy S23 Ultra / Android 17 are the tested combination; other devices need validation.

There are three important approvals: installing this Windows app, authorizing your computer for USB debugging on the phone, and approving Android's VPN. The last two are Android security decisions and cannot be automated or bypassed.

## 1. Download the right Windows package

Go to https://github.com/enesoglu/android-usb-reverse-tethering/releases and open the newest public alpha release.

- **Setup.exe**: recommended for normal use. Installs for your Windows account and adds a Start menu shortcut.
- **Portable.zip**: extract the entire ZIP to a permanent folder, then open `UsbRelay.exe`. Do not run the app inside the ZIP and do not move it after enabling startup.
- **Android.apk**: optional advanced download. Normal setup installs the bundled companion automatically; you do not need to download an APK on the phone.
- **Source.zip**: for developers, including corresponding source and dependency sources.

Both Windows packages include the .NET runtime. No separate .NET, Python, Java, Go, Android Studio, or build commands are needed for everyday use.

The first releases are not Windows code-signed. If Windows warns about an unknown publisher, verify the GitHub account/repository and the release's SHA-256 checksums before deciding whether to continue. If an antivirus or administrator policy blocks execution, stop and review it; do not disable your protections.

## 2. Install or extract on the computer

For Setup.exe, follow the English setup screens. The default location is your private Programs folder under Local AppData. Administrator rights are not needed for this app. A desktop shortcut is optional. Launch USB Relay at the end.

For the portable package, choose **Extract all**, keep all extracted files together, and run `UsbRelay.exe`.

Use **Setup help** in the app whenever you want to reopen the setup assistant. The same guide is included offline as `START-HERE.html`.

## 3. Make Google platform tools available

ADB is Google's tool for talking to an authorized Android device. Choose one option:

- **Already installed**: USB Relay checks its managed tools folder, the standard Android SDK folder, `ANDROID_HOME`, and absolute folders in your PATH. An existing trusted copy can be used immediately.
- **Guided download**: in Setup help, read Google's SDK terms, tick the consent box, and click **Download Google platform tools**. The app downloads from Google, verifies the archive's pinned SHA-256, checks extraction paths, and installs it in your user profile. No Google SDK ZIP is hosted by this project.
- **Manual**: download Windows platform tools from https://developer.android.com/tools/releases/platform-tools, accept the applicable terms, and extract the ZIP. In USB Relay open **Preferences → Android platform tools → Browse**, choose the extracted `adb.exe`, save preferences, then restart USB Relay.

The managed download is version-pinned for review and integrity. It is not an automatic updater. Keep an independently installed SDK up to date through its official tools. Google tools retain their own terms and notices.

## 4. Enable Developer options on the phone

1. Open the phone's **Settings**.
2. Open **About phone** and find **Build number**. On Samsung: **About phone → Software information → Build number**.
3. Tap Build number **seven times**. Enter the phone's PIN/password if Android asks.
4. Go back to Settings and open **Developer options**. On some brands this is under **System** or **Additional settings**.
5. Turn on **USB debugging** and accept Android's explanation.

You do not need root, an unlocked bootloader, wireless debugging, or the USB tethering/hotspot switch. Work-device and OEM security policies may prohibit debugging; USB Relay does not bypass them.

## 5. Connect the cable and authorize this computer

1. Connect the phone directly to the computer with the data-capable USB cable.
2. Unlock the phone and watch for **Allow USB debugging?**
3. Approve only a computer you trust. **Always allow from this computer** is optional and useful on your own computer; leave it off on shared or untrusted machines.
4. Return to the Windows app. The phone should appear in the device selector.

If the computer does not recognize the phone, try another known data cable and USB port. If needed, select **File transfer** in the phone's USB notification. Do not enable USB tethering. Some manufacturers require their official ADB USB driver on Windows; see https://developer.android.com/studio/run/oem-usb. For Samsung, use https://developer.samsung.com/android-usb-driver. Driver installation is separate from this app and can require administrator permission.

## 6. Trust the phone in the Windows app

Select the phone and click **Trust & connect**. Review the trust confirmation. USB Relay installs or updates its own Android companion through the authorized ADB connection, provisions the encrypted tunnel, and saves trust only after authentication succeeds.

An APK with another signing key is not silently replaced. A phone whose saved identity changes is not automatically trusted. See Troubleshooting if you previously used a development build or reinstalled the phone app.

## 7. Approve the Android VPN

The phone opens **USB Relay**. Tap **Connect securely**, then accept Android's **Connection request / VPN** prompt. This grants the app permission to route the phone's traffic through USB.

Android normally permits one VPN per profile. USB Relay replaces another active phone VPN; stop the other VPN first if you need to preserve its connection. You do not need to enable browser-based Install unknown apps for the normal automatic ADB installation.

The Windows app should now show **Connected**. Open a browser on the phone and test a normal HTTPS page. Its internet connection is now going through the computer.

## 8. Configure everyday convenience

Open **Preferences** in the Windows app:

- **Start with Windows** launches the app after you sign in. It is off until you choose it.
- **Start minimized to the system tray** starts the app without showing its window, including when Windows launches it. It is off by default; double-click the tray icon to reopen the window.
- **Automatically connect trusted phones** reconnects a saved, matching phone while the Windows app is running.
- **Minimize to the system tray** hides the minimized window.
- **Keep running in the tray when I close the window** keeps sharing after clicking X.

Click **Save preferences**. Use the tray icon to reopen the window. Use **Disconnect** to stop sharing for this attachment; use **Exit** to stop the Windows app completely. A manual stop stays stopped until a new USB attachment or an explicit Connect. An Android force-stop requires opening the phone app or manually connecting again.

Private-LAN access is disabled by default. Enable it only if you need other LAN devices and understand the access you are granting. PC-local services remain blocked. DNS and LAN changes apply to the next connection.

## 9. Understand fallback and speed

Without Android lockdown, the phone may return to Wi-Fi or mobile data when the USB VPN ends. For strict no-fallback behavior, inspect **Settings → VPN → USB Relay → Always-on VPN / Block connections without VPN**, if your device supports them, and test the behavior yourself. These OS settings have not been validated on every device.

This alpha is not a gigabit release. The tested default delivered a SHA-256-verified 512 MiB transfer at about 128 Mbps with loaded UDP p95 around 20 ms; raw ADB on the same setup was 268–273 Mbps. Faster USB hardware alone does not remove the current application data-path overhead. See `docs/verification.md` for exact conditions and limitations.

## Updates and uninstalling

Download a newer release from the same repository, exit USB Relay from the tray, and run the new installer. Keep the portable folder together when replacing a portable version. The Windows app updates its bundled phone companion during connection. Release APKs use a stable project signing key; modified/development builds use separate keys.

To uninstall Windows, first choose tray **Exit**, then use **Settings → Apps → Installed apps → USB Relay → Uninstall**. The installer removes its own program files and its matching startup entry. Encrypted identity/pairing data is kept under `%LOCALAPPDATA%/UsbRelay` so a reinstall can preserve trust. A portable copy is removed by deleting its extracted folder after Exit; disable its startup option first.

To remove the Android app, use Android's normal app-uninstall screen. This removes its Keystore identity; a future reinstall requires forgetting the old pairing in Windows and pairing again. Windows uninstall does not silently uninstall the phone app or remove other ADB tools. Disable USB debugging on the phone when you no longer need it.

For development-build users, the public companion uses a separate application ID and Windows state directory. Your development app/data can remain intact. Exit the old desktop instance before opening the public build. The new public phone app needs its own first VPN approval.

## Troubleshooting

### The phone does not appear

Unlock it, confirm USB debugging, reconnect, try a known data cable/port, and check the official OEM ADB driver. Disconnect other Android USB devices. A charging-only cable can power the phone while providing no usable data connection.

### The phone is unauthorized or the debugging prompt is missing

Keep the phone unlocked and reconnect. Check Developer options and OEM/work-profile policies. Do not approve an unexpected computer. As a last manual troubleshooting step, Android's **Revoke USB debugging authorizations** resets all previously trusted computers, not just this app; use it only if you intend that reset.

### ADB is missing

Open Setup help and use the consent-gated download, or choose an existing absolute `adb.exe` path in Preferences and restart. Running the app from inside a ZIP or leaving its companion files behind will also break setup.

### Android asks for VPN permission, or a different VPN stops

Approve the system prompt on the phone. It cannot be approved from Windows. Android's one-VPN-per-profile rule means another VPN may be replaced. If permission was revoked or the app was force-stopped, open the phone app and connect manually.

### The phone key changed

Verify that this is your phone and that you intentionally reinstalled/reset the companion. In Windows open **Your devices → Forget**, then choose **Trust & connect** again. Do not treat an unexplained identity change as a normal retry.

### APK installation fails with another signing key

Use matching official packages from the same release. A modified build cannot update the official APK under the same app ID with a different signing key. For an intentional custom build, uninstall its conflicting phone app, rebuild with your own key, and pair again; uninstalling removes that app's private settings and identity.

### Connected, but an app cannot use the internet

Confirm the PC itself can browse. Test a normal browser on the phone. Some apps insist on Wi-Fi/cellular even with a working VPN. Check the DNS preference and network policy. This version forwards IPv4 TCP/UDP; general IPv6 and ping are not supported.

### Closing the window did not stop sharing

Close-to-tray is a preference. Choose Disconnect or tray Exit. This is deliberate so startup and automatic reconnect can work without an open window.

### Speed is lower than the internet subscription

The cable, negotiated USB link, ADB, encryption, packet relay, phone processing, and application all contribute. USB-C shape and charging wattage do not prove SuperSpeed. This alpha also has a measured VPN throughput limitation independent of the raw cable transfer. Do not compare its local synthetic benchmark directly with a public speed-test site.

### The VPN stops when the screen is off

Background policies vary by phone. Consult the manufacturer's background-app guidance and test your settings. Screen-off, reboot, and hours-long operation are still part of the validation roadmap.

## Get help safely

Open an issue at https://github.com/enesoglu/android-usb-reverse-tethering/issues/new/choose. Include Windows/Android versions, app version, cable/port information, the exact step that failed, and the app's **Copy safe diagnostics** output. Remove phone serials and personal information from screenshots. Do not upload private keys, DPAPI files, APK signing files, packet captures, or browsing history.
