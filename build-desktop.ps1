param([switch]$SkipAndroid)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Push-Location $PSScriptRoot
try {
    $bundleExecutable = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot 'build\desktop\UsbRelay.exe'))
    $active = @(Get-Process -Name UsbRelay -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $bundleExecutable })
    if ($active.Count -gt 0) { throw 'Exit USB Relay from its tray menu before rebuilding the desktop bundle' }
    if (-not $SkipAndroid) { & "$PSScriptRoot\build-android.ps1" }
    go build -trimpath -o build/usb-relayd.exe ./cmd/usb-relayd
    if ($LASTEXITCODE -ne 0) { throw 'Secure daemon build failed' }
    $manifest = @{}
    $manifest['usb-relayd.exe'] = (Get-FileHash -LiteralPath build\usb-relayd.exe -Algorithm SHA256).Hash.ToLowerInvariant()
    $manifest['usb-transfer-lab.apk'] = (Get-FileHash -LiteralPath build\usb-transfer-lab.apk -Algorithm SHA256).Hash.ToLowerInvariant()
    $manifest | ConvertTo-Json | Set-Content -LiteralPath build\assets.manifest.json -Encoding UTF8
    dotnet publish desktop/UsbRelay/UsbRelay.csproj -c Release -o build/desktop --nologo
    if ($LASTEXITCODE -ne 0) { throw 'Desktop build failed' }
    Copy-Item -LiteralPath build\usb-relayd.exe -Destination build\desktop\usb-relayd.exe -Force
    Copy-Item -LiteralPath build\usb-transfer-lab.apk -Destination build\desktop\usb-transfer-lab.apk -Force
    Copy-Item -LiteralPath build\assets.manifest.json -Destination build\desktop\assets.manifest.json -Force
    Copy-Item -LiteralPath THIRD_PARTY_NOTICES.md -Destination build\desktop\THIRD_PARTY_NOTICES.md -Force
    Copy-Item -LiteralPath third_party -Destination build\desktop -Recurse -Force
    Copy-Item -LiteralPath README.md -Destination build\desktop\README.md -Force
    Copy-Item -LiteralPath docs -Destination build\desktop -Recurse -Force
    go list -m all | Set-Content -LiteralPath build\desktop\modules.txt -Encoding UTF8
    if ($LASTEXITCODE -ne 0) { throw 'Module inventory failed' }
    Copy-Item -LiteralPath LICENSE -Destination build\desktop\LICENSE.txt -Force
    Copy-Item -LiteralPath docs\START-HERE.html -Destination build\desktop\START-HERE.html -Force
    Write-Output (Join-Path $PSScriptRoot 'build\desktop\UsbRelay.exe')
} finally { Pop-Location }
