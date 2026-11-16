# Architecture: desktop development candidate 0.2

## Packet path

Android applications -> Android VpnService IPv4 TUN -> URV2 framed packets in TLS 1.3 -> USB ADB reverse -> Windows gVisor netstack -> host TCP/UDP sockets -> PC Internet connection.

Neither packet compression nor a Windows network adapter is used. Android protects the transport socket from its own VPN before connecting to loopback. Go and Android's TLS implementations provide the cryptography; the application does not implement TLS primitives.

TLS negotiates ALPN `usb-relay/2`, requires P-256 client identity and verifies pinned SHA-256 SPKI hashes. Initial enrollment accepts a client identity only for a manually initiated pending connection and commits it after the fresh 32-byte nonce has been verified. Subsequent connections require the saved phone pin. The phone verifies the desktop pin provisioned through authorized ADB. TLS 1.2, a wrong nonce and changed pins fail. Handshakes time out; no plaintext version negotiation is supported.

The encrypted application hello is ASCII URV2 followed by the 32-byte nonce. The relay replies URV2 after authentication/enrollment. Each IP packet uses a two-byte big-endian length, maximum 1500. Zero length followed by byte 1 is an explicit user-disconnect control frame. Invalid sizes/header lengths terminate the session; structurally valid IPv6 frames are counted/discarded without interrupting IPv4. IPv6 Internet forwarding is not implemented.

## Module boundaries

- `internal/private`: bounded DPAPI state I/O, authenticated encryption through Windows, atomic replacement. Current-user scope, no machine-wide decryption flag.
- `internal/identity`: persistent P-256 certificate/key and SPKI pin. A corrupt identity or missing identity beside existing pairing state fails closed rather than silently resetting trust.
- `internal/securetunnel`: TLS authentication, exact peer validation, ALPN and enrollment boundary.
- `internal/relay`: packet validation, gVisor TCP/UDP forwarding, destination policy, fair bounded queue and aggregate metrics.
- `internal/adb`: hidden argument-based subprocesses, USB discovery, provisioning through stdin and exclusive mapping lease. Each session compares the installed base APK SHA-256 to the verified bundled file. A mismatch or unavailable hash requires installation, where Android enforces signing identity; matching a version string is never signature proof. It never invokes a Windows shell, overwrites existing mappings, force-stops packages or kills the shared ADB server.
- `internal/manager`: trusted devices, settings, one active session, cancellation and reconnect policy behind a small ADB Backend interface. Tests replace ADB but use real TLS clients.
- `cmd/usb-relayd`: parent-owned newline JSON stdio commands/snapshots. No TCP/HTTP management API. Parent EOF and shutdown cancel sessions.
- `desktop/UsbRelay`: WPF presentation, optional HKCU startup, NotifyIcon tray lifecycle, single-instance events and a child-manager adapter. Companion hashes are embedded in the desktop assembly and checked before execution/install.
- `android/src`: shell-protected provisioning, public controls that ignore config extras, non-exportable Android Keystore client key and foreground VPN service.

Desktop identity and settings are stored as DPAPI blobs in the current user's LocalAppData. Android config lives in private SharedPreferences; private key material stays in Keystore. Public fingerprints may be displayed; nonces, packet contents and destinations are not logged. Startup is per-user sign-in, opt-in, and rolls back the prior registry command if saving preferences fails.

## Resource and network policy

One active phone; maximum 256 network flows. The phone-facing TCP send buffer is fixed at 256 KiB with autotuning disabled, independently from the WAN-facing host socket. This applies backpressure before a large USB backlog accumulates. Global output queue 1024 packets, TCP per-flow cap 768, UDP per-flow cap 64. Round-robin scheduling prevents a ready bulk flow from monopolizing queued service. UDP packets queued over 100 ms are discarded and counted. This is not FQ-CoDel. TCP idle timeout is 10 minutes; UDP 90 seconds, based on monotonic elapsed time. Buffered transport writes batch up to 21 ready packets and flush before waiting. USB writes time out after 15 seconds. Closing a session closes host/netstack sockets and releases queued references.

The relay blocks loopback, unspecified, multicast, broadcast, link-local and the PC's discovered interface addresses. Private LANs require an explicit preference. `10.0.2.2` is allowed only for exact diagnostic ports selected by the isolated lab daemon; the production desktop never supplies diagnostic ports. There is no general localhost bridge in the desktop.

## Lifecycle

The manager polls ADB once per second. Unknown phones need Trust & connect. A saved phone can reconnect with backoff 1/2/4/8/16 seconds; successful authentication resets backoff. Phone key mismatch suppresses retry. Manual desktop or phone stop suppresses the current attachment; new transport ID/detach or explicit Connect permits another attempt. Controller cleanup uses a distinct remote-stop action so it does not record user-stop suppression on Android. A force-stopped Android app requires user action. Android's VPN consent and OS restrictions still apply.

Each manager boot has a fresh attachment namespace, combined with ADB's transport ID. Only the session's own reverse mapping is removed. Closing to tray preserves the process; Exit shuts down the manager and mapping. A second desktop launch signals the existing process. A stopped daemon is reported rather than silently restarting a partially trusted session.

## Lab compatibility

`cmd/usb-relay` is the historical plaintext URV1 lab relay, now requires `--insecure-lab` and is excluded from the desktop bundle. Old raw benchmarks use the separate URT1 synthetic framing protocol and shell-protected BenchActivity. They are not the production VPN transport. Do not use `lab.connect` with the new companion; desktop commands replace it.

Only receiver-confirmed bytes and matching hashes establish goodput. USB link rate, raw ADB throughput, encrypted VPN goodput and public Internet speed remain separate measurements. The 0.1 report is historical; it cannot validate 0.2 performance.

## Primary references

- [Android VPN API and lockdown](https://developer.android.com/develop/connectivity/vpn)
- [Android Keystore](https://developer.android.com/privacy-and-security/keystore)
- [Windows DPAPI](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata)
- [User sign-in startup registry](https://learn.microsoft.com/en-us/windows/win32/setupapi/run-and-runonce-registry-keys)
- [gVisor networking](https://gvisor.dev/docs/architecture_guide/networking/)
- [Gnirehtet technical design](https://github.com/Genymobile/gnirehtet/blob/master/DEVELOP.md)
- [Wintun userspace adapter API](https://www.wintun.net/)
- [Tetrd official listing](https://play.google.com/store/apps/details?id=com.robskie.tether)
- [Tetrd developer's historical accessory-mode explanation](https://www.reddit.com/r/androidapps/comments/elu1mo/tetrd_tethering_reverse_tethering_app_noroot/)

The Tetrd references do not disclose its complete current Windows routing implementation. They do not support a claim that it avoids all userspace processing or cannot suffer queue overflow.

## Actual-device compatibility findings

Windows native ADB reports `get-devpath` as `unknown` on this host. In that case the manager uses ADB's USB-only `-d get-serialno` selector and requires an exact match to the selected serial. Multiple USB devices make that fallback fail closed; this candidate supports one attached USB phone on this Windows backend. A network serial cannot satisfy the USB-only match.

Conscrypt signs an already-hashed TLS transcript through `NONEwithECDSA`. The Keystore generation policy therefore authorizes DIGEST_NONE plus SHA-256; TLS itself still hashes and uses TLS 1.3. The unusable pre-release alias is preserved and the corrected alias is `usb-relay-client-v2-tls`; desktop pins are never overwritten automatically. Protected diagnostics include a synthetic raw-digest signing/verification check. See [Android's TLS key guidance](https://developer.android.com/reference/android/security/keystore/KeyGenParameterSpec.Builder#setDigests(java.lang.String...)).

Shell cannot directly start a BIND_VPN_SERVICE-protected service. Remote stop instead enters the DUMP-protected provisioning activity, which requests a normal app-internal service stop. The transient provisioning activity also handles onNewIntent and ignores empty recreation intents without replacing private configuration.

A repeated real-device repro caught Android reusing the former provisioning task and delivering new configuration to its top SpeedActivity instead of ProvisionActivity (four of four failures). NEW_TASK|MULTIPLE_TASK fixed three of three minimized attempts. The final controller uses those flags, rejects AM reused-activity warnings, and gives provisioning no task affinity/history/recents entry. Main controls launch in their own normal task. onNewIntent alone did not fix this root-task reuse issue.
