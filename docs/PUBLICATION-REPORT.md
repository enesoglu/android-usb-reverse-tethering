# Publication and discoverability report

Reviewed November 16, 2026. This report separates completed publication work from external review and search-engine outcomes.

## Public destinations

- [Source repository](https://github.com/enesoglu/android-usb-reverse-tethering)
- [Project website](https://enesoglu.github.io/android-usb-reverse-tethering/)
- [Versioned alpha downloads](https://github.com/enesoglu/android-usb-reverse-tethering/releases/tag/v0.3.0-alpha.1)
- [Computer and phone installation guide](INSTALLATION.md)
- [Evidence and source review of SEO proposals](DISCOVERABILITY.md)

The project is GPL-3.0-only. Source, comments, UI, repository metadata and documentation are English.

## Implemented work

| Area | Completed work | Verified result / limitation |
| --- | --- | --- |
| Repository discovery | Descriptive reverse-tethering repository name, concise About text, website link and 20 relevant GitHub topics | Public metadata checked; keywords describe actual functionality and connection direction |
| Clear positioning | Short explanation of PC-to-Android internet sharing, rootless operation and free GPL licensing | No claim of guaranteed speed or comparative superiority |
| Natural-language FAQ | English questions about USB-C, Ethernet, Wi-Fi, root, Tetrd alternatives, Gnirehtet GUI expectations, reconnect and speed | English-only requirement preserved; no duplicate keyword paragraphs or hidden crawler prompts |
| Alternatives table | USB Relay, Gnirehtet, Tetrd and SimpleRT compared using upstream documentation | Requirements, cost and platforms distinguished; no subjective GUI score or speed ranking |
| Images | Updated synthetic interface preview with descriptive alt text | Screenshot is explicitly labeled as sample data |
| Website | GitHub Pages, responsive static HTML, canonical/description/social metadata, accessible FAQ, source and download links | Deployed successfully and checked in a browser; no telemetry or external fonts |
| Google verification | Owner-supplied public Search Console HTML verification tag deployed | Owner reported a successful live inspection and submitted a homepage indexing request; indexing itself has not been verified |
| Sitemap | Valid XML sitemap listing the canonical homepage | HTTP 200, application/xml, correct XML namespace, no X-Robots-Tag; Search Console still reports Couldn't fetch after correct-URL resubmission |
| Downloads | Per-user Windows setup, self-contained portable ZIP, release-signed APK, build metadata, checksums and corresponding source | All six assets published through successful GitHub release automation |
| Supply chain | Pinned Actions, APK signing secrets, committed public certificate, dependency notices, GPL corresponding-source ZIP and build provenance | Freshly downloaded installer checksums and GitHub provenance verification passed; Windows Authenticode signing remains pending |
| Software directory | English description and disclosure text prepared | AlternativeTo explicitly deferred by owner; no entry submitted |
| Community | Transparent developer announcement draft prepared | No Reddit or forum posts sent; no independent endorsements or votes fabricated |
| Curated list | [awesome-android-apps PR #27](https://github.com/fiedri/awesome-android-apps/pull/27) submitted using its own add/build helpers | Open for maintainer review; acceptance is not claimed |
| WinGet | [winget-pkgs PR #481719](https://github.com/microsoft/winget-pkgs/pull/481719) submitted with one three-file version manifest | Local validation and actual WinGet installation passed; Microsoft review and owner CLA acceptance pending |

The new repository does not meet the reviewed awesome-windows minimum-star rule. Developer-library lists were not targeted because they are a poor fit for this end-user application. A relevant FOSS Android-app catalog was used instead.

## Validation performed

- Full project checks, native Windows checks, Go race checks and vulnerability scan passed locally. Hosted source CI passed on release commit f0989b42922f50fbb17fecc6c1881d499fda07ae.
- The signed-release workflow completed successfully. Release archives were downloaded again and checked against published SHA-256 values; the Windows installer's GitHub build provenance verified for the expected repository.
- Native checks passed for asset hash verification/tampering rejection, wrong SDK hash, safe extraction, ZIP traversal and overwrite rejection, registry roundtrip and tray restore. Extraction of the pinned official Google archive also passed. Google SDK binaries are not bundled or mirrored.
- Silent per-user installation, update while the application was running, graceful shutdown and uninstall passed. An unrelated startup registration was preserved.
- WinGet schema validation succeeded. WinGet downloaded the public installer, verified its hash and installed the expected version and publisher. The test installation was removed. LocalManifestFiles was temporarily enabled with owner approval and restored to false in finally; the restored value was checked independently.
- The public companion was tested on the connected Galaxy S23 Ultra / Android 17. TCP, 100/100 UDP probes and public HTTPS returned success. Ordinary-app attempts to provision, read protected diagnostics, run a benchmark and stop the tunnel were denied.
- The exact GitHub-built APK and relay were then downloaded and tested again: TCP, 100/100 UDP and HTTPS passed; authenticated pairing persisted, deliberate stop remained stopped and the saved phone reconnected after manager restart.
- Public source and binary checks excluded private development notes, pairing state and private signing keys. The source ZIP includes pinned Go dependency source.

These checks do not constitute an independent audit, broad OEM support, long-duration stability testing or a new throughput benchmark. The documented encrypted VPN measurement remains about 128 Mbps on the measured setup; gigabit performance is not claimed.

## Search Console: unresolved reporting state

The owner submitted the indexing request and confirmed the correct sitemap URL was resubmitted. Live inspection reports crawl allowed, successful fetch and indexing allowed. Our HTTP/XML checks also pass. The Sitemaps report nevertheless still says Couldn't fetch. The internal Google fetch failure has not been reproduced or explained; no speculative website change was made.

The origin-root URL https://enesoglu.github.io/sitemap.xml returns 404; the correct project URL is https://enesoglu.github.io/android-usb-reverse-tethering/sitemap.xml and returns 200. This path distinction was checked, but correct-URL resubmission did not resolve the owner's report, so it is not presented as the confirmed cause.

Follow Google's [Sitemaps report troubleshooting](https://support.google.com/webmasters/answer/7451001?hl=en): inspect the exact submitted URL and error details and check the next fetch result. A one-page site can also be discovered through its submitted homepage. Do not repeatedly resubmit or promise an indexing deadline.

## Owner actions and next priorities

1. Review Microsoft's CLA request in the WinGet PR and personally complete the requested acceptance only if you agree. No agreement was accepted on the owner's behalf. Automated review or a maintainer may request further manifest changes.
2. Keep an independent encrypted Android keystore backup and store its password separately in a password manager. The DPAPI password blob is Windows-user-bound and GitHub Secrets cannot be read back as a recovery copy. The local Show-SigningRecovery.ps1 helper supports this owner-only step.
3. Recheck Search Console sitemap status and impressions over time; report indexing as confirmed only when Search Console actually shows it. Search visibility and AI recommendations are not guaranteed.
4. After more device and long-duration testing, improve sustained throughput, obtain trusted Windows code signing, and revisit broader package and community distribution.

No 24-hour indexing, first-page ranking, automatic backlinks, directory acceptance or AI recommendation guarantee is supported by the reviewed primary sources. Useful documentation, working downloads, transparent limitations and independent user reports are the durable priorities.
