# Security model and release gates

## Scope and trust

Protect the USB data stream from unauthenticated local peers, prevent ordinary Android apps from redirecting the tunnel, prevent automatic trust of new or changed phone identities, and bound resource usage. The trusted computing base includes Windows and the logged-in user, Android and its Keystore, the Google ADB executable and its authorized host key, the bundled application binaries, Go TLS, Android TLS and gVisor.

Initial pairing is trust-on-first-use over an already-authorized ADB link to a physically selected USB phone. This does not defend against a compromised authorized computer or malicious replacement binaries. ADB grants broad device debugging access: enable it only for trusted computers. Keep remote/wireless debugging off when unused. The app refuses non-USB device paths for pairing.

The PC can observe traffic that is not independently end-to-end encrypted. This app does not replace application HTTPS, provide Internet anonymity or prevent an administrator/root attacker from controlling an endpoint. DPAPI is user-scoped at rest; it does not isolate secrets from malware running as the same Windows user. Keystore prevents exporting a key through its API but cannot make a compromised app incapable of requesting signatures. Hardware-backed storage is not asserted without device inspection.

## Implemented controls

| Boundary | Control | Evidence |
| --- | --- | --- |
| Tunnel endpoint | TLS 1.3, required client identity, exact saved SPKI, ALPN | Real host TLS positive and negative tests |
| First enrollment | Manual selection, fresh 256-bit nonce, persistence only after authentication | Wrong nonce and failed persistence tests |
| Persistent trust | User DPAPI integrity, atomic files, no silent identity reset | Roundtrip/tamper/missing-key tests |
| Android configuration | DUMP-protected component, launcher ignores config extras | Ordinary-UID permission probe coverage on Android |
| Phone identity | P-256 non-exportable Android Keystore key | Actual Go/Android TLS and synthetic non-exportable-key signing passed |
| Executable discovery | Absolute ADB paths only; rejected current-directory/PATH ErrDot and relative SDK candidates | Search-path hijack regression test |
| Local management | Parent-owned stdio, no control network listener | Architecture and desktop lifecycle check |
| Network destinations | Deny PC-local/link-local/private LAN by default | Destination policy tests; explicit diagnostic port allowlist |
| Congestion | Bounded per-flow/global queue, stale UDP drop, flow cap, deadlines | Queue cap/fairness/staleness tests; sustained device load pending |
| Cleanup/reconnect | Scoped mapping lease, cancellation, stop suppression, key mismatch lockout | Manager tests plus actual cable detach/reconnect and application restart passed |
| Companion tampering | Embedded asset hashes; exact installed APK hash checked on every session; changed APKs go through Android signature enforcement | Changed-file rejection, actual signed device update and exact-file comparison |
| Startup | Opt-in HKCU sign-in command with quoted path, rollback | Temporary registry-value self-test |

No traffic content, destination addresses, raw ADB output, tokens or private key material is included in normal diagnostics. UI device names/serials and public fingerprints are local pairing metadata. Reports use synthetic traffic. The nonce is not a Windows process argument; it travels over stdin into the authorized device shell and is necessarily accessible to that trusted shell.

## Operational limits

IPv4 TCP/UDP only. The default configuration blocks private LAN destinations, including LAN resolvers, until explicitly enabled. PC address isolation uses addresses discovered when the session starts; reconnect after network interface changes. Resource limits reduce risk but do not make authenticated traffic incapable of exhausting CPU or bandwidth. UDP loss under congestion is expected and counted.

When the tunnel stops, Android may return to Wi-Fi/mobile unless the user enables its OS Always-on/Block-without-VPN policy. Lockdown behavior, reboot/screen-off recovery and OEM background restrictions must be tested on the real phone before claiming strict fallback protection. Turning a preference on in the desktop cannot silently grant Android's VPN permission.

Development APKs use a local development signing key. Public APKs use a separate long-lived release key; its public certificate is committed, while the private key is stored outside the checkout and in restricted release-workflow secrets. Windows binaries are not Authenticode signed. Hashes inside an unsigned application are not a trusted software distribution/update system. No automatic updater, telemetry, certificate authority, administrator service or kernel/network driver is installed.

## Gates before a public stable release

1. Extend the passed actual-device mutual-TLS and permission probes with real changed-key, first-consent denial and certificate expiry cases.
2. Extend the passed cable detach/reattach, manager restart and desktop stop checks with actual phone stop, force-stop, revocation, reboot and screen-off cases.
3. Run receiver-hashed sustained TCP downloads alongside UDP/QUIC under load; record goodput, loss, latency, CPU and memory. Repeat with a verified SuperSpeed link before gigabit claims.
4. Test Android lockdown/fallback and IPv6 behavior on supported OS versions, including DNS routing.
5. Review the attack surface independently, establish supported OS versions, vulnerability reporting and dependency update procedures.
6. Maintain GPL-3.0-only corresponding-source distribution, protected release signing, dependency notices, and independently reviewable release provenance. Windows Authenticode signing remains pending.

No certification, independent audit, zero-copy, kernel-only forwarding, guaranteed gigabit or loss-free networking is claimed by this development candidate.
