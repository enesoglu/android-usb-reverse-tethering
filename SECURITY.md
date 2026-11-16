# Security policy

USB Relay is a public alpha, not an independently audited security product. Only the newest alpha is maintained. Verify release provenance, APK certificate fingerprints and SHA-256 checksums before installation.

## Report a vulnerability privately

Use [GitHub private vulnerability reporting](https://github.com/enesoglu/android-usb-reverse-tethering/security/advisories/new). Do not open a public issue containing exploit details, keys, nonces, private traffic or personal device information. If private reporting is unavailable, open a minimal issue asking for a private reporting channel without publishing the vulnerability.

Include the affected version, OS versions, a synthetic reproduction, impact and a proposed fix if available. There is no guaranteed response time or paid bug-bounty program.

## Security boundaries

See [the threat model](docs/security.md). The app trusts the local Windows account, authorized ADB, Android/Keystore, and the installed application binaries. It does not protect a compromised endpoint. Initial pairing relies on authorized USB access; subsequent sessions pin the saved phone identity. TLS, app-installation signatures and runtime tunnel identities serve different purposes.

Windows binaries are not currently Authenticode-signed. Google tools are downloaded separately after consent or supplied by the user. Do not disable security tools, bypass OS VPN consent, expose ADB to untrusted networks, or share signing keys to work around an error.
