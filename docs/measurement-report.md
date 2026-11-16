# Measurement report: historical 0.1 prototype, 2026-11-16

These measurements predate the 0.2 TLS/Keystore desktop candidate. They do not establish its throughput, interoperability or security.

Device: Samsung Galaxy S23 Ultra (SM-S918B), Android 17 / API 37. Direct PC USB connection, confirmed by both the owner and the Windows device ancestry (Samsung composite -> USB 3.0 root hub -> AMD USB 3.10 controller). No public Internet speed test was performed.

## Validated evidence

| Scenario | Result | What was validated |
| --- | --- | --- |
| Native counted transfer, accessory configuration | 289.79 Mbps | Expected/sent/Android received: 134,664,992 bytes, exact match |
| Native stdin-open control in an earlier isolated comparison | 293.47 Mbps | Expected and Android received: 134,217,728 bytes, exact match |
| Direct Android framed receiver, accessory configuration | 282.60 / 287.38 Mbps | 1200 / 16384-byte payloads; actual frame and byte counts; zero sequence and edge-sample errors |
| Direct receiver, MTP configuration, three runs per size | 179.03-277.60 Mbps | Every run received the expected bytes/frames with zero sequence and edge-sample errors; speed varied, cause unresolved |
| Windows MTP connection probe | High Speed (480 Mbps) | EX speed code 2; EX_V2 flags 2: SuperSpeed capable, not operating at SuperSpeed |
| Gnirehtet stalled-consumer reproduction, three independent runs | All reproduced | 58,228 / 71,170 / 63,868 occurrences of each original warning; no relay errors |

The overflow reproduction deliberately withholds client reads. Its offered rates (312.78-360.10 Mbps) are test inputs, not a limit for gnirehtet or the phone. It validates the observed code path, not the root cause of the original incident.

## Invalidated early measurement

The earlier claimed 1.3-1.8 Gbps native results did not verify receipt. An explicit negative control showed 134,217,728 bytes admitted by the PC while Android `wc -c` reported **zero**. The native receiver can exit on stdin EOF. These figures cannot establish link throughput or gigabit feasibility. They must not be quoted as successful delivery.

`lab/native_control.py` now keeps stdin open and requires the Android received byte count to match the planned count. It reports no validated throughput if the count differs. A unit regression test rejects the original sender-success/zero-received scenario.

## Experiments that did not materially raise framed goodput

- Simple per-frame reads versus bulk parsing versus direct NIO buffers.
- Requested Android receive buffer of 4 MiB versus default.
- Precomputing sender data versus generating batches while transferring.
- Android debuggable flag off.
- PC TCP_NODELAY on/off.

Timing counters in direct mode measured roughly 3.7 seconds in socket reads versus 0.04-0.06 seconds parsing 128 MiB. This narrows the measured wait location; it is not proof of the ultimate bottleneck. The native receiver with validated counts is in the same general throughput range.

## Current limit and next evidence

Both accessory and MTP configurations enumerated at High Speed. MTP EX_V2 explicitly indicates SuperSpeed capability present but operating flag false. This prevents a gigabit test on the current negotiated link. It does not prove the cable is defective or identify why SuperSpeed negotiation failed. The owner confirms direct USB 3.x connection, cannot identify the current cable speed, and currently lacks access to the normal cable. A known SuperSpeed cable/port comparison remains needed for gigabit testing.

Machine-readable results are local under `results/` and ignored by Git. Reports without receiver validation are historical controls only. Prefer `s23-native-counted.json`, `s23-direct-timing.json`, `s23-mtp-framed.json`, `usb-mtp-link-verified.json`, and `gnirehtet-reproduced.json`.

## Working VPN prototype results

The Android VPN and Windows gVisor relay were subsequently implemented and tested on the same phone/link. A real phone probe returned TCP=true, UDP=true, 100/100 verified 1200-byte UDP datagrams, HTTPS status 200 from example.com, and VPN state connected. This establishes basic Internet connectivity; it is not a public Internet speed test.

| Actual VPN download | Received bytes / integrity | Goodput | Idle UDP p95 | Loaded UDP p95 | Loaded UDP loss |
| --- | --- | --- | --- | --- | --- |
| Initial 128 MiB | 134,217,728 / SHA-256 match | 223.58 Mbps | 5.85 ms | 38.23 ms | 0/62 |
| Combined packet write, 128 MiB | 134,217,728 / SHA-256 match | 232.38 Mbps | 3.90 ms | 42.04 ms | 0/58 |
| Combined packet write, 512 MiB | 536,870,912 / SHA-256 match | 252.20 Mbps | 5.51 ms | 46.27 ms | 0/205 |

The 512 MiB transfer took 17.03 seconds. It is a short sustained transfer, not a long-duration soak. Payloads were synthetic PC-local HTTP data crossing the actual Android VPN. UDP RTT probes ran concurrently through the relay.

Aggregate relay telemetry over the last two downloads and background phone traffic recorded approximately 1,098 link-queue rejections. The measured UDP RTT probes had no observed loss, and TCP delivered complete hash-verified payloads; **this does not mean all IP packets were loss-free**. Queue pressure, retransmissions and loaded latency remain tuning work. Rejections do not currently distinguish individual UDP versus TCP flows.

A real-device startup regression was identified: a valid incoming IPv6 packet caused the first IPv4-only parser to terminate the session. The relay now validates IPv6 framing, counts/discards unsupported IPv6 traffic, and continues IPv4 forwarding. The TCP/UDP integration fixtures inject this frame before application traffic to cover the failure.

## Checks and remaining scope

Go TCP/UDP integration, wrong-key rejection, malformed framing and short-write tests pass. Java streaming decoder cases and Python framing/delivery/device-selection tests pass. The earlier race run passed; final verification is recorded in progress.

Long-duration stability, saturation QUIC/UDP goodput, automatic reconnect recovery, screen-off reliability, IPv6 Internet and gigabit operation remain unverified or unimplemented. The original gnirehtet phone incident has not been diagnosed or fixed by this work.
