param([string]$Jdk = "$env:ProgramFiles\Android\Android Studio\jbr")
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Security
$root = Split-Path $PSScriptRoot -Parent
$folder = Join-Path $env:LOCALAPPDATA 'UsbRelayMaintainer\signing'
New-Item -ItemType Directory -Force -Path $folder | Out-Null
$key = Join-Path $folder 'usb-relay-release.jks'
$secret = Join-Path $folder 'password.dpapi'
if ((Test-Path -LiteralPath $key) -or (Test-Path -LiteralPath $secret)) { throw 'Signing files already exist; refusing to overwrite a release identity' }
$random = New-Object byte[] 32
$rng = [Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($random)
$rng.Dispose()
$password = [Convert]::ToBase64String($random)
$env:USB_RELAY_SIGNING_PASSWORD = $password
try {
    & "$Jdk\bin\keytool.exe" -genkeypair -keystore $key -storepass:env USB_RELAY_SIGNING_PASSWORD -keypass:env USB_RELAY_SIGNING_PASSWORD -alias usb-relay-release -keyalg RSA -keysize 4096 -validity 10000 -dname 'CN=USB Relay Release,OU=Open Source'
    if ($LASTEXITCODE -ne 0) { throw 'Release signing-key creation failed' }
    $protected = [Security.Cryptography.ProtectedData]::Protect([Text.Encoding]::UTF8.GetBytes($password),$null,[Security.Cryptography.DataProtectionScope]::CurrentUser)
    [IO.File]::WriteAllBytes($secret,$protected)
    & "$Jdk\bin\keytool.exe" -exportcert -rfc -keystore $key -storepass:env USB_RELAY_SIGNING_PASSWORD -alias usb-relay-release -file "$root\packaging\android-signing-certificate.pem"
    if ($LASTEXITCODE -ne 0) { throw 'Public signing certificate export failed' }
} finally { Remove-Item Env:USB_RELAY_SIGNING_PASSWORD -ErrorAction SilentlyContinue; $password = $null }
Write-Output 'Release key created outside the repository. Password protected with current-user DPAPI. Back up both files securely.'
