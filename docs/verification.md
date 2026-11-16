# Verification: development candidate 0.2.0

Date: 2026-11-16. Host Windows x64; Go 1.27.4; .NET SDK 10.0.202; Android compilation platform 36, min API 29. These measurements describe the development core before the first public packaging release. Packaging does not make unverified performance claims.

## Passed on the current source

| Check | Evidence |
| --- | --- |
| `check.ps1` | Exit 0: Go tests/vet, Python tests, Android build/signature check, Java tests, WPF format/build and native self-test |
| Go package integration/unit tests | ADB parsing, DPAPI, persistent identity, manager policy, TCP/UDP relay, network isolation, queue fairness, real TLS encrypted UDP |
| `go test -race ./... -count=1 -timeout=60s` | Exit 0 for all six tested packages using gcc |
| Python delivery validation | 10 tests passed, including sender-success/zero-received rejection |
| Java frame decoder | 4,376 cases passed |
| Android reconnect policy | Remote cleanup preserves retry; user stop suppresses only the current attachment; manual/new attachment allowed |
| Android artifacts | Companion and ordinary-UID attack-probe APKs built; companion signature verification passed |
| WPF release build | Completed without compiler warnings |
| Native `--self-test` | Six true results: temporary startup registry roundtrip, expected asset hashes, changed-file rejection, minimize-to-tray, restore, preview creation |
| `scripts/Test-DesktopLifecycle.ps1` | Actual primary launch, second launch signals primary, disconnect command, graceful Exit, child-daemon cleanup; real startup registration unchanged |
| Go vulnerability scan | `govulncheck` reported no known vulnerabilities for reachable Go code; this is not a complete security audit |

The UI image `.local/desktop-preview.png` uses explicitly synthetic connected-device/rate data to inspect layout. It is not a phone connection or throughput measurement.

## Actual S23 Ultra / Android 17 results

- Go/Android TLS 1.3 mutual authentication and protected Keystore raw-digest signing passed.
- Synthetic TCP, 100 verified UDP datagrams, public DNS/HTTPS returned HTTP 200.
- Ordinary-app instrumentation: untrusted provisioning, benchmark access, diagnostics and remote stop were all denied; passed=true, code 0. Test package removed afterward.
- Pairing persisted, manual desktop stop remained stopped, and manager restart automatically connected the saved phone. These checks were repeated on the final candidate. Three consecutive final TCP/100-UDP/HTTPS probes also passed.
- Owner physically unplugged the USB cable, waited and reattached it without pressing Connect. Detach was detected and the same saved phone auto-connected. All five lifecycle assertions passed.
- Receiver-hashed 512 MiB initial transfer: 80.27 Mbps, all 326 loaded UDP probes received, loaded p95 194.16 ms. This exposed a performance concern; it is not hidden from results.
- Short 128 MiB baseline: 206.66 Mbps, loaded p95 29.65 ms, queue rejections 803, all 76 loaded probes received.
- 768-packet TCP-cap comparison, 128 MiB: 242.01 Mbps, loaded p95 55.51 ms, queue rejections 983, all 50 loaded probes received. One idle probe was lost. Background flow counts differed (10 versus 15); this is not a controlled proof that the capacity change alone improved throughput.

- Repeated 512 MiB transfers with the 768 cap: 117.66 and 115.15 Mbps, loaded p95 317.94 and 314.98 ms, no relay queue rejections and no loaded UDP loss. This localizes the observed slowdown beyond the application output queue; it does not establish a sole cause.
- Android upload TLS-write coalescing experiment: 106.91 Mbps, loaded p95 321.59 ms. It did not improve the long transfer and was reverted.

- Requested 256 KiB outer OS socket buffers: 105.14 / 105.13 Mbps, loaded p95 187.78 / 169.97 ms. This experiment was reverted; it did not improve throughput.
- Current raw ADB control, 512 MiB, direct framed receiver: 267.99 / 272.75 Mbps for 1200 / 16384-byte frames, exact frame/byte counts and no sequence/sample errors. This excludes VPN and TLS and is not a full-payload SHA-256 test.
- Sampled phone TUN rejections remained zero. Aggregate relay byte counts were close to test payload plus protocol overhead, so unrelated background downloads did not explain the large gap.
- A diagnostic-only Go CPU profile identified substantial TCP outstanding-segment work. Windows pprof also samples blocked syscall stacks; its total sample percentages must not be equated to actual CPU utilization. Actual process CPU measurements use GetProcessTimes.
- Fixed 256 KiB TCP send buffer: 512 MiB at **128.43 Mbps**, all **531/531** loaded UDP probes received, loaded p95 **20.10 ms**, zero relay queue rejections and zero sampled TUN rejections. Actual relay CPU was approximately 31% of one core.
- Fixed 512 KiB TCP send buffer: 127.54 Mbps, loaded p95 45.77 ms, 447/447 probes received. Fixed 1 MiB: 135.68 Mbps, loaded p95 81.40 ms, 326/326 probes received. The 256 KiB default was selected because throughput was similar while latency was substantially lower.
- Manual Android AOT compilation of only this package did not improve the long transfer (121.03 Mbps with the 1 MiB send buffer). No automatic compilation step was added.
- Transport batching was measured at approximately 16.6 packets per flush. The hypothesis that every downlink IP packet produced its own TLS write was not supported; no extra coalescing delay was added.

The default candidate improves loaded latency and removes the observed queue rejection in that test, but does **not** meet the desired high-throughput target. The final sustained VPN rate is lower than both the historical 0.1 prototype and the present raw-ADB control. This is an application data-path limitation requiring further work, not evidence that this USB link itself is limited to 128 Mbps. Gigabit still additionally requires an actual SuperSpeed link. Do not describe the speed work as complete or independently attribute all overhead to TLS.

Artifacts are private local JSON in ignored results/. Sender success is insufficient: received bytes and SHA-256 must match. These tests measure synthetic PC-to-phone VPN goodput, not public Internet speed.

## Still pending

Long-duration stability, actual phone-side manual stop/force-stop/revocation, consent denial, screen-off/reboot, sustained QUIC, strict Android Always-on/Block-without-VPN behavior, real changed-key attack/expiry scenarios, and a verified SuperSpeed cable. Performance variation and loaded latency need further characterization. No gigabit, zero-loss or production-stability guarantee is made.

## Device follow-up commands

Exit the desktop before isolated diagnostics. Connect/authorize the phone and run `task phone:probe`, then `task phone:speed`. A successful report requires receiver confirmation; inspect the report's validation field, not merely process exit or offered bytes.

To execute the ordinary-app permission probe after the companion is installed:

```powershell
adb install -t -r build/security-test/security-test.apk
adb shell am instrument -w dev.usbreverse.securitytest/.PermissionProbe
adb uninstall dev.usbreverse.securitytest
```

All `untrusted_*_denied` results and `passed` must be true. Instrumentation targets its own test package and does not adopt shell identity. Exit codes alone are insufficient to judge Android instrumentation results.

The current development release is ready for these device checks, not certified or declared stable for unattended production use.

## Public distribution validation

The public flavor uses a separate application ID and long-lived signing certificate. Local verification passed the native release self-test (asset hash tampering, invalid SDK hash, safe extraction, traversal and overwrite rejection, tray and registry roundtrip), official Google archive extraction, and per-user silent installer install / running-app upgrade / uninstall. Full source checks and hosted CI passed before binary release. Private pairing state and signing keys are excluded from all public payloads.

Earlier phone-side results above used the development application ID. The separately signed public package was subsequently installed with its own first VPN consent: TCP, 100/100 UDP probes and public HTTPS passed, and ordinary-app provisioning/diagnostic/benchmark/stop attempts were denied. The exact GitHub-built APK and relay were downloaded and tested again with the same successful network results; saved pairing, manual-stop suppression and auto-reconnect after manager restart passed. See the [publication report](PUBLICATION-REPORT.md).
