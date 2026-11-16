# Distribution and signing notes

Checked against upstream sources on 2026-11-16. These notes describe release engineering constraints; they do not claim that every possible distribution arrangement has been legally cleared.

## Project license and corresponding source

The project uses **GPL-3.0-only**. GPLv3 sections 0 and 5 address interactive copyright, license, redistribution and warranty notices. Sections 1 and 6 define corresponding source and ways to provide it with downloadable binaries, including equivalent network access with clear directions. Source includes the scripts needed to build and install the covered program; the license also defines exclusions for system libraries and generally available unmodified tools. Keep each binary release tied to its exact source revision, build scripts, dependency versions and license text. Provide instructions for building modified clients with a developer's own signing key; do not describe the maintainer's private release key as necessary to exercise modification rights. [GPLv3 text](https://raw.githubusercontent.com/spdx/license-list-data/main/text/GPL-3.0-only.txt), [canonical GNU license page](https://www.gnu.org/licenses/gpl-3.0.html).

Apache states that Apache-2.0 software can be included in GPLv3 projects; the reverse direction does not follow. This does not erase upstream notices. Apache-2.0 section 4 requires the license and relevant attribution, and readable NOTICE attribution where the distributed upstream work includes it. Preserve the pinned gVisor and btree licenses and audit their selected source trees rather than assuming one top-level project license replaces all dependency notices. [Apache compatibility explanation](https://www.apache.org/licenses/GPL-compatibility.html), [Apache-2.0 section 4](https://www.apache.org/licenses/LICENSE-2.0).

Go's BSD-style license requires copyright, conditions and disclaimer reproduction with binary distributions. Include the toolchain's license alongside the relay's other notices. [Go license](https://go.dev/LICENSE).

## Self-contained .NET distribution

The .NET runtime and WPF repositories carry MIT license texts requiring preservation of copyright and permission notices. Both publish additional third-party notices. Include the license and third-party notices corresponding to the actual runtime packs shipped in a self-contained Windows build; the project license alone does not cover every bundled component. [Runtime license](https://raw.githubusercontent.com/dotnet/runtime/v10.0.15/LICENSE.TXT), [runtime notices](https://raw.githubusercontent.com/dotnet/runtime/v10.0.15/THIRD-PARTY-NOTICES.TXT), [WPF license](https://raw.githubusercontent.com/dotnet/wpf/v10.0.15/LICENSE.TXT), [WPF notices](https://raw.githubusercontent.com/dotnet/wpf/v10.0.15/THIRD-PARTY-NOTICES.TXT).

## Google platform tools

The Android SDK terms require acceptance before SDK use (section 2). Section 3.1 limits the proprietary SDK grant to developing Android applications; section 3.4 restricts redistribution except where applicable third-party licenses require otherwise. Section 3.5 separately makes open-source component licenses govern those components. Section 3.8 requires preservation of notices. These provisions must be read together: acceptance is not a general redistribution grant, and the proprietary SDK grant should not be described as blanket approval for all end-user runtime uses. [Google SDK terms](https://developer.android.com/studio/terms).

AOSP adb itself has an Apache-2.0 NOTICE. That fact alone does not classify every file in Google's complete platform-tools ZIP. [AOSP adb NOTICE](https://android.googlesource.com/platform/packages/modules/adb/+/refs/heads/main/NOTICE).

Release design: do not mirror Google's SDK ZIP or executable files in GitHub assets. Offer an existing `adb.exe` path and an official manual-download link. Any optional integrated downloader should obtain a pinned archive directly from Google, show the terms and require explicit user acceptance, retain upstream notices, verify its expected hash, and reject unsafe ZIP paths. Describe this accurately as a direct dependency download, not as Google endorsement or comprehensive licensing clearance.

## Android release keys

Android requires signed APKs. Direct APK distribution uses the maintainer's app-signing key; a debug key is not the release identity. Keep that private key and its passwords out of source, assets and logs, keep a secure backup, and preserve signing continuity across updates. Losing a directly managed signing key can prevent updates. A certificate or fingerprint is public information; the private signing key is not. Android package-signing keys are separate from each phone's runtime Keystore tunnel identity. [Android app-signing guidance](https://developer.android.com/studio/publish/app-signing).

GitHub supports encrypted repository and environment secrets, including Base64-encoded binary material. Base64 is an encoding, not encryption. Pass secrets through environment variables or stdin instead of command-line arguments, do not print decoded keystores, and remove temporary key files after signing. Fork pull-request runs do not receive ordinary repository secrets. [GitHub secrets documentation](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets).

Use read-only workflow token permissions by default, grant release writes only to the release job, and keep signing steps away from untrusted pull-request code. GitHub recommends workflow dependency lockfiles, or verified full commit-SHA pinning when lockfiles are unavailable. Review action code and release logs as well as pinning versions. [GitHub secure-use guidance](https://docs.github.com/en/actions/reference/security/secure-use).

## Inno Setup 7.1.1

The pinned Inno Setup 7.1.1 license permits use for any purpose, including commercial applications, subject to its stated attribution, redistribution and modification conditions. A commercial-license purchase request on the website should not be restated as a mandatory fee under that published license. Keep the compiler's built-in attribution and do not claim authorship of Inno Setup. A documentation acknowledgment is appreciated by upstream. [License at the 7.1.1 source tag](https://raw.githubusercontent.com/jrsoftware/issrc/is-7_1_1/license.txt).

Use the unmodified pinned compiler, verify the download before execution, and record the compiler/source version in release documentation. The generated installer must include the project's GPL license and bundled dependency notices.