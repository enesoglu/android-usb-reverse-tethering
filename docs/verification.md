# Verification scope and current limitations

USB Relay is public alpha software. Automated checks cover the relay, Android framing, desktop lifecycle and release packaging. Device coverage is limited; behavior across other OEMs, ports, cables and Android power policies can differ.

## Covered behavior

- Authenticated TLS transport, pinned identities, nonce validation and protected pairing state.
- Legacy packet framing and negotiated larger packets, with bounds checked at both ends.
- TCP and UDP forwarding, packet fragmentation, destination policy and fair bounded queues.
- Cancellable TCP backpressure and cleanup of blocked writers.
- Reconnect policy, explicit disconnect, package integrity and ordinary-app access denial.
- Desktop build, tray/startup behavior, installation packaging and APK signing checks.

These checks support development; they are not an independent security audit or a guarantee for every device.

## Performance

The current release reduces per-packet overhead through negotiated packet sizing and ready-packet batching. Throughput also depends on USB negotiation, ADB behavior, phone scheduling, concurrent traffic and the remote endpoint. A fast cable alone does not determine application throughput. Saturation can increase latency, and a fixed minimum rate is not guaranteed.

See the [engineering guide](reverse-tethering-engineering.md) for the reasoning behind these choices and the [architecture](architecture.md) for the current implementation. Detailed local measurements and device metadata are not part of the public documentation or release artifacts.

## Remaining limits

- Windows x64 and one active Android phone.
- IPv4 TCP/UDP; general IPv6 and ICMP forwarding are not implemented.
- One Android VPN per profile; some applications require a physical Wi-Fi/cellular transport.
- Screen-off, reboot, long-duration operation and OEM restrictions require broader coverage.
- Download consistency and latency under saturation remain development work.
- Windows binaries are not Authenticode-signed; independent auditing is pending.

Android may fall back to another network when the tunnel closes unless its Always-on VPN / Block connections without VPN policy is configured and verified. Read the [security model](security.md) before relying on that behavior.
