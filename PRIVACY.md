# Privacy

USB Relay has no accounts, advertising, analytics or cloud traffic relay. It routes phone traffic through the computer's own internet connection. It does not add encryption beyond the local USB tunnel for protocols that are not independently encrypted end to end.

## Local data

Windows stores its identity, trusted-phone metadata and preferences under the app's Local AppData directory. Identity and pairing files use current-user DPAPI. Android stores connection configuration privately and a non-exportable client key in Keystore. Device names, public fingerprints and aggregate traffic counters may appear locally. Normal diagnostics omit traffic contents, destination addresses and session secrets.

## Network access

The app opens the network connections requested by the phone. DNS uses the configured resolver through the computer. The optional setup download contacts Google's official download host only after SDK-terms consent. Opening documentation, source or issue links contacts the selected website through the user's browser. The application does not automatically upload diagnostic reports or check a remote user database.

## Removing data

Uninstalling the Windows program preserves encrypted pairing data for reinstall continuity. The guide documents the app data folder. Removing the Android app deletes its private configuration and Keystore identity and requires pairing again. The app does not remove other ADB tools or other applications' data.

## Sharing reports

Issue reports and attachments are public unless submitted through GitHub's private vulnerability reporting. Redact personal information and never upload private keys, DPAPI blobs, APK-signing keystores, raw traffic or browsing data.
