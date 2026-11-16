# Third-party notices

The Windows relay embeds gVisor netstack, pinned in `go.mod` / `go.sum`. gVisor is licensed under Apache License 2.0. A copy is included at `third_party/gvisor-LICENSE`.

- [gVisor](https://github.com/google/gvisor): Apache-2.0.
- [google/btree](https://github.com/google/btree): Apache-2.0, indirect dependency.
- [golang.org/x/sys](https://cs.opensource.google/go/x/sys): BSD-3-Clause, indirect dependency.
- [golang.org/x/time](https://cs.opensource.google/go/x/time): BSD-3-Clause, indirect dependency.
- [golang.org/x/exp](https://cs.opensource.google/go/x/exp): BSD-3-Clause, indirect dependency.

Gnirehtet is invoked externally only by the overflow test and is not bundled. Android SDK platform/build tools, Java, Python and ADB are external tools; Google platform tools are not redistributed. The Go runtime is compiled into the relay and its BSD license is included in binary packages.

Public Windows packages include the .NET 10 runtime and Windows Desktop runtime (WPF/Windows Forms). Their runtime-pack licenses and third-party notices are copied into the binary package third_party directory. See modules.txt and BUILD-INFO.json for pinned dependency and runtime versions. The Go dependency sources are included in the corresponding-source release archive. No Google SDK binaries are bundled.
