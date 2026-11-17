# Engineering USB reverse tethering

USB reverse tethering sends a computer's Internet connection to an Android device. A fast cable is only the first part of that system. Useful performance depends on how applications turn socket data into packets, carry those packets across USB, and deliver them back into the phone's network stack.

This guide explains the design choices behind a rootless implementation, the limits of common approaches, and the lessons incorporated into USB Relay. It is an engineering reference rather than a benchmark ranking. Device-specific logs, identifiers and browsing data are deliberately excluded.

## 1. Follow the complete packet path

An ordinary Android application cannot replace the operating system's network routes arbitrarily. Android's public `VpnService` provides a user-approved virtual interface for this purpose. The application reads outgoing IP packets from its TUN descriptor and injects incoming packets by writing to it. Its own transport socket must be protected from the VPN, otherwise the tunnel can capture its own traffic. [Android VPN guide](https://developer.android.com/develop/connectivity/vpn).

One practical architecture is:

```text
Android application sockets
  -> Android TCP/IP stack -> VPN TUN -> framed, authenticated transport
  -> USB -> desktop userspace TCP/IP stack -> ordinary host sockets
  -> Internet
```

The desktop terminates the phone-facing TCP connection and opens a separate upstream connection. A userspace stack such as gVisor supplies TCP state, acknowledgments, retransmission and UDP handling. The computer does not need a virtual adapter for this design. [gVisor networking architecture](https://gvisor.dev/docs/architecture_guide/networking/), [USB Relay packet path](architecture.md).

A different design creates a desktop TUN adapter and integrates it with operating-system routing and NAT. That changes the desktop forwarding implementation; it does not automatically change the USB transport or remove Android's TUN. Treat these as separate decisions.

## 2. Separate link speed from application goodput

Keep four rates distinct: negotiated USB signaling, raw transport delivery, complete VPN delivery, and public Internet throughput. A result from one layer cannot establish another layer's ceiling.

USB connectors, cables and protocol capabilities are different properties. A USB4-capable cable does not upgrade a slower host port or phone interface. The connection operates at a mutually supported capability. USB 3.2 includes several speed classes, so the version label alone is insufficient. [USB-IF overview](https://usb.org/usb-32-0).

Inspect the negotiated connection rather than inferring it from a connector's shape, charging rate or product description. On Windows, connection-information queries can distinguish active SuperSpeed operation from mere SuperSpeed capability. Device configuration also matters: switching between debugging, file-transfer and accessory interfaces can cause re-enumeration. [Microsoft USB connection information](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/usbioctl/ns-usbioctl-_usb_node_connection_information_ex_v2).

Also distinguish bits from bytes. A signaling figure expressed in gigabits per second is not a file-transfer figure in gigabytes per second. Encoding, framing, scheduling, copies and protocol traffic reduce the application payload rate further. The useful number is data delivered to the receiver per elapsed time.

## 3. ADB is a transport with its own flow control

`adb reverse` makes a host-side TCP endpoint reachable through a device-side endpoint. It is convenient because authorized debugging and platform tools already handle device discovery and USB access. Gnirehtet documents an architecture using this path to carry Android VPN packets to its relay. [Gnirehtet technical design](https://github.com/Genymobile/gnirehtet/blob/master/DEVELOP.md).

ADB adds another framing and acknowledgment layer. Historically, an ADB stream could have only one outstanding `A_WRTE` packet before receiving an `A_OKAY`. That creates a stop-and-wait limit determined by chunk size and acknowledgment delay, even when USB has spare bandwidth.

AOSP's experimental `ADB_BURST_MODE` uses a byte window to allow multiple outstanding writes. Support is negotiated between peers. This can improve a raw transfer while leaving a packet-processing bottleneck largely unchanged; a file-push improvement is not a prediction for a VPN. [AOSP delayed-ACK design](https://android.googlesource.com/platform/packages/modules/adb/+/refs/heads/main/docs/dev/delayed_ack.md).

The setting belongs to the ADB server's environment, not an already-running client command. A product should not silently restart a shared debugging server to apply it. Keep ADB configuration, version compatibility and ownership explicit.

## 4. Android TUN is a packet interface

A socket stream and a TUN descriptor have different semantics. A stream read can return part of a message, several messages, or any convenient chunk. A standard Android TUN write represents one incoming IP packet. Applications must reconstruct transport frames before injecting individual packets. [VpnService.Builder contract](https://developer.android.com/reference/android/net/VpnService.Builder).

This makes packet rate a major cost. Delivering the same payload as many small packets requires more framing operations, runtime transitions, system calls and kernel network processing. CPU time around a TUN write can include kernel work performed in the calling thread; it cannot all be attributed to Java copying or encryption.

`writev()` does not make each iovec an independent TUN packet. Linux passes the write's complete iterator into one packet-injection operation. Concatenating several IP packets into one ordinary TUN write therefore changes packet contents rather than providing valid packet batching. [Linux TUN implementation](https://github.com/torvalds/linux/blob/master/drivers/net/tun.c).

A native descriptor loop may reduce language-runtime overhead, but it retains the kernel's per-packet work. Linux supports additional offload facilities, yet Android's standard VPN interface is created with `IFF_TUN | IFF_NO_PI`; the public builder does not expose GSO or multiqueue as ordinary options. Do not assume an arbitrary Linux TUN optimization is available to an unprivileged Android VPN. [AOSP VPN interface creation](https://android.googlesource.com/platform/frameworks/base/+/master/services/core/jni/com_android_server_connectivity_Vpn.cpp).

## 5. Negotiate packet size without weakening the protocol

A larger virtual MTU can reduce packet count for bulk TCP traffic. For example, a bounded 9000-byte inner MTU can carry more application data per TUN operation than a 1500-byte MTU. It is a design option, not a universally optimal value.

The inner MTU does not require jumbo Ethernet support on the Internet. In a terminating relay, the phone-facing and upstream TCP connections are separate. The host stack segments the upstream stream for its own network. UDP datagrams, fragmentation and path-MTU behavior still need independent handling; a TCP result does not establish their compatibility.

Both endpoints must agree on the packet limit before allocating buffers or creating the interface. USB Relay carries its agreement inside authenticated TLS: newer framing includes a bounded MTU request and response, while older peers retain the original limit. Frame length and IP-header consistency are checked before a packet enters the stack. [Relay implementation](../internal/relay/relay.go), [Android service](../android/src/UsbVpnService.java).

Retain explicit version handling. Never interpret an unknown or malformed request as permission to accept arbitrary lengths, fall back to plaintext, or replace a trusted identity.

## 6. Batch transport work; bound queued work

Transport batching is useful even though TUN injection remains packet-oriented. A writer can encode several complete IP frames into a bounded stream buffer and flush them together. This reduces small writes while preserving each packet's boundary.

A simple low-latency policy drains packets that are already ready, stopping at a byte or packet bound. Flush before waiting for future input. Waiting solely to fill a batch can make an idle interactive connection feel slow. Reverse-direction TCP acknowledgments deserve attention too: flushing a separate encrypted record for every small ACK creates avoidable transport work. [Android ready-packet loop](../android/src/UsbVpnService.java).

Queues need byte limits as well as packet limits. Increasing MTU multiplies the memory and latency hidden behind an unchanged packet count. TCP windows are another queue: making them larger can improve utilization, increase backlog, or increase stack CPU work. The phone-facing window should be tuned independently of the computer's WAN connection.

For TCP, a full internal queue can apply bounded backpressure rather than deliberately dropping packets and forcing recovery. Waiting must be cancelable, with independent transport progress and reliable shutdown. UDP requires an explicit admission/drop policy because indefinitely delaying a datagram does not preserve its usefulness. Per-flow scheduling and capacity headroom help small flows coexist with bulk traffic. [USB Relay queue](../internal/relay/queue.go).

A fair bounded queue is not automatically FQ-CoDel, and zero local queue rejection is not proof of loss-free Internet delivery. Throughput, loaded latency and memory bounds remain separate goals.

## 7. Encryption and scheduling need separate diagnosis

TLS adds real work, but a slow VPN does not by itself identify encryption as the limiting cost. Compare a framed encrypted stream with the full packet path before removing security. If encryption and framing can already deliver substantially more data, the remaining work lies elsewhere in the pipeline. Keep receiver validation and distinguish handshake cost from steady-state transfer.

Use established TLS implementations, authenticated peers, pinned identities and fresh session material. Buffering and packet-size negotiation should operate inside those controls. An apparently faster unauthenticated stream is a different product, not a transparent optimization. [Authenticated tunnel](../internal/securetunnel/server.go), [Android TLS adapter](../android/src/TunnelTLS.java).

Thread scheduling can also affect a foreground VPN service, especially when its owning application is not the visible application. Scheduling sensitivity should be measured alongside CPU time and device state. A targeted worker-priority adjustment is different from realtime execution, CPU pinning or disabling power management. It does not establish a fixed throughput floor across devices.

## 8. What accessory mode and Tetrd teach

Android Open Accessory is an alternative to ADB. The computer acts as USB host; the phone enters Android accessory mode and exposes bulk endpoints. An Android application obtains permission to access the accessory stream. AOA can coexist with a debugging interface, so a requirement to enable debugging does not prove payload travels through ADB. [AOA protocol](https://source.android.com/docs/core/interaction/accessories/aoa).

AOA is not inherently limited to USB 2.0: AOSP accessory-function code includes SuperSpeed and SuperSpeedPlus descriptors. Actual availability depends on firmware, descriptors and host support. A Windows implementation also needs a compatible interface-driver binding; libusb/WinUSB integration is part of the design. [AOSP accessory descriptors](https://android.googlesource.com/kernel/common/+/ca53b8f1b44ac5a5a9eec34e4c88c8c2cba0d49d/drivers/usb/gadget/function/f_accessory.c), [Microsoft WinUSB](https://learn.microsoft.com/en-us/windows-hardware/drivers/usbcon/introduction-to-winusb-for-developers).

In a public launch post, Tetrd's developer described accessory-mode transport and said the app was not open source. Its published policy describes an Android VPN and computer TUN adapter. Inspection of an official Linux 1.3.2 server package found libusb/AOA-related strings and gVisor function names, including `tcp.NewForwarder`. These establish packaged capabilities, not which path runs for a particular mode. They do not establish the current Windows data path, USB encryption choices, buffering policy or a performance advantage. Those details remain unverified by the available sources. [Developer statement](https://www.reddit.com/r/androidapps/comments/elu1mo/tetrd_tethering_reverse_tethering_app_noroot/), [Tetrd policy](https://tetrd.app/privacy), [official Linux artifact](https://download.tetrd.app/files/tetrd.linux_amd64.deb).

SimpleRT provides an open reference for libusb/accessory transport plus a desktop TUN. Gnirehtet demonstrates the ADB/socket-relay approach. Neither architecture's existence guarantees a particular speed on another platform. Replacing ADB also leaves Android TUN processing in place when the phone still uses `VpnService`. [SimpleRT](https://github.com/robinpaulson/SimpleRT), [Gnirehtet](https://github.com/Genymobile/gnirehtet).

## 9. Choose the next change by the limiting layer

Keep transport, packet forwarding and security behind clear interfaces. First establish negotiated USB capability, then isolate transport, encrypted framing and complete packet delivery. Choose the next change from the gap between those layers rather than from a cable label or a competitor's marketing claim.

For a maintainable implementation:

- Preserve packet boundaries, validate lengths and negotiate bounded limits.
- Keep queues, flow counts, memory and waiting times bounded; make cancellation part of every blocking path.
- Preserve authentication while changing buffering, MTU or transport.
- Separate a desktop TUN/routing decision from an ADB-to-AOA transport decision.
- Treat built-in Android tethering and its offload path as a different direction and privilege model, not a rootless reverse-VPN baseline. [AOSP tethering architecture](https://source.android.com/docs/core/ota/modular-system/tethering).
- Publish transferable mechanisms, limitations and primary references. Keep personal identifiers and raw user traces out of public examples.

The central design lesson is that a fast USB connection supplies capacity, while the complete software path determines how much of that capacity reaches an application. Efficient packet handling, bounded flow control and clear compatibility rules often deserve attention before a transport rewrite.
