param(
 [string]$Sdk = "$env:LOCALAPPDATA\Android\Sdk",
 [string]$Jdk = "$env:ProgramFiles\Android\Android Studio\jbr",
 [string]$InnoCompiler = '',
 [switch]$SkipSourceArchive
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Add-Type -AssemblyName System.Security
$root = Split-Path $PSScriptRoot -Parent
Push-Location $root
$oldKey=$env:USB_RELAY_KEYSTORE
$oldPassword=$env:USB_RELAY_SIGNING_PASSWORD
try {
 $version=Get-Content -Raw version.json | ConvertFrom-Json
 $output=Join-Path $root 'build\release\windows-x64'
 $artifacts=Join-Path $root 'build\release\artifacts'
 New-Item -ItemType Directory -Force -Path $output,$artifacts | Out-Null
 if (@(Get-Process -Name UsbRelay -ErrorAction SilentlyContinue | Where-Object {$_.Path -eq (Join-Path $output 'UsbRelay.exe')}).Count -gt 0) { throw 'Exit the release app before rebuilding it' }
 $expectedBuildRoot=[IO.Path]::GetFullPath((Join-Path $root 'build\release'))+[IO.Path]::DirectorySeparatorChar
 $resolvedOutput=[IO.Path]::GetFullPath($output)
 if (-not $resolvedOutput.StartsWith($expectedBuildRoot,[StringComparison]::OrdinalIgnoreCase)) {throw 'Release output escapes generated build directory'}
 if (Test-Path -LiteralPath $output) {
  if (((Get-Item -LiteralPath $output -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {throw 'Release output must not be a reparse point'}
  Remove-Item -LiteralPath $output -Recurse -Force
 }
 New-Item -ItemType Directory -Force -Path $output | Out-Null
 if (-not $env:USB_RELAY_KEYSTORE) {
  $signing=Join-Path $env:LOCALAPPDATA 'UsbRelayMaintainer\signing'
  $env:USB_RELAY_KEYSTORE=Join-Path $signing 'usb-relay-release.jks'
  $bytes=[IO.File]::ReadAllBytes((Join-Path $signing 'password.dpapi'))
  $env:USB_RELAY_SIGNING_PASSWORD=[Text.Encoding]::UTF8.GetString([Security.Cryptography.ProtectedData]::Unprotect($bytes,$null,[Security.Cryptography.DataProtectionScope]::CurrentUser))
 }
 & "$root\build-android.ps1" -PublicRelease -Sdk $Sdk -Jdk $Jdk
 if ($LASTEXITCODE -ne 0) { throw 'Public Android build failed' }
 $apk=Join-Path $root 'build\public\usb-transfer-lab.apk'
 $certText=Get-Content -Raw packaging\android-signing-certificate.pem
 $certBytes=[Convert]::FromBase64String(($certText -replace '-----BEGIN CERTIFICATE-----|-----END CERTIFICATE-----|\s',''))
 $sha=[Security.Cryptography.SHA256]::Create()
 $expected=([BitConverter]::ToString($sha.ComputeHash($certBytes))).Replace('-','').ToLowerInvariant()
 $sha.Dispose()
 $certOutput=& "$Sdk\build-tools\37.0.0\apksigner.bat" verify --print-certs $apk
 if ($LASTEXITCODE -ne 0 -or -not ($certOutput -match [regex]::Escape($expected))) { throw 'APK signer does not match the committed public release certificate' }
 go build -trimpath -o build/usb-relayd.exe ./cmd/usb-relayd
 if ($LASTEXITCODE -ne 0) { throw 'Release relay build failed' }
 $manifest=@{
  'usb-relayd.exe'=(Get-FileHash build\usb-relayd.exe -Algorithm SHA256).Hash.ToLowerInvariant()
  'usb-transfer-lab.apk'=(Get-FileHash $apk -Algorithm SHA256).Hash.ToLowerInvariant()
  'companionPackage'=$version.androidPackage
  'stateDirectory'='UsbRelay'
 }
 $manifest | ConvertTo-Json | Set-Content build\assets.manifest.json -Encoding UTF8
 dotnet publish desktop/UsbRelay/UsbRelay.csproj -c Release -r win-x64 --self-contained true -p:RuntimeFrameworkVersion=$($version.dotnetRuntime) -p:Version=$($version.version) -p:DebugType=None -p:DebugSymbols=false -o $output --nologo
 if ($LASTEXITCODE -ne 0) { throw 'Self-contained desktop publish failed' }
 Copy-Item build\usb-relayd.exe $output -Force
 Copy-Item $apk (Join-Path $output 'usb-transfer-lab.apk') -Force
 Copy-Item build\assets.manifest.json $output -Force
 foreach($file in @('README.md','THIRD_PARTY_NOTICES.md','SECURITY.md','PRIVACY.md')) { Copy-Item -LiteralPath $file -Destination $output -Force }
 Copy-Item LICENSE (Join-Path $output 'LICENSE.txt') -Force
 Copy-Item docs\START-HERE.html (Join-Path $output 'START-HERE.html') -Force
 Copy-Item third_party -Destination $output -Recurse -Force
 foreach($relative in Get-Content packaging\public-docs.txt) {
  if (-not $relative.StartsWith('docs/') -or $relative.Contains('..') -or $relative.Contains(':')) {throw 'Unsafe public documentation path'}
  $destination=Join-Path $output $relative
  New-Item -ItemType Directory -Force -Path (Split-Path $destination -Parent) | Out-Null
  Copy-Item -LiteralPath (Join-Path $root $relative) -Destination $destination -Force
 }
 go list -m all | Set-Content (Join-Path $output 'modules.txt') -Encoding UTF8
 $goRoot=go env GOROOT
 Copy-Item (Join-Path $goRoot 'LICENSE') (Join-Path $output 'third_party\go-LICENSE.txt') -Force
 $nugetRoot=if($env:NUGET_PACKAGES){$env:NUGET_PACKAGES}else{Join-Path $env:USERPROFILE '.nuget\packages'}
 foreach($package in @('microsoft.netcore.app.runtime.win-x64','microsoft.windowsdesktop.app.runtime.win-x64')) {
  $packageRoot=Join-Path $nugetRoot ($package+'\'+$version.dotnetRuntime)
  foreach($file in @(Get-ChildItem -LiteralPath $packageRoot -File | Where-Object {$_.Name -match 'LICENSE|NOTICE'})) {
   Copy-Item $file.FullName (Join-Path $output ('third_party\'+$package+'-'+$file.Name)) -Force
  }
 }
 $buildInfo=@{version=$version.version;androidPackage=$version.androidPackage;androidSigningCertificateSHA256=$expected;go=(go version);dotnetSDK=(dotnet --version);dotnetRuntime=$version.dotnetRuntime;source=$version.repository}
 $previousPreference=$ErrorActionPreference
 $ErrorActionPreference='Continue'
 $commit=git rev-parse --verify HEAD 2>$null
 $commitExit=$LASTEXITCODE
 $ErrorActionPreference=$previousPreference
 if ($commitExit -eq 0) {$buildInfo.commit=$commit.Trim()}
 $buildInfo | ConvertTo-Json | Set-Content (Join-Path $output 'BUILD-INFO.json') -Encoding UTF8
 $selfTest=Start-Process -FilePath (Join-Path $output 'UsbRelay.exe') -ArgumentList @('--self-test','--output',('"'+(Join-Path $root '.local\release-preview.png')+'"')) -WindowStyle Hidden -Wait -PassThru
 if ($selfTest.ExitCode -ne 0) {throw 'Release desktop self-test failed'}
 $prefix='USB-Relay-'+$version.version
 Compress-Archive -Path (Join-Path $output '*') -DestinationPath (Join-Path $artifacts ($prefix+'-windows-x64-portable.zip')) -Force
 Copy-Item $apk (Join-Path $artifacts ($prefix+'-android.apk')) -Force
 Copy-Item (Join-Path $output 'BUILD-INFO.json') (Join-Path $artifacts 'BUILD-INFO.json') -Force
 if (-not $InnoCompiler) {$InnoCompiler=Join-Path $root '.local\release-tools\InnoSetup7\ISCC.exe'}
 if (-not (Test-Path -LiteralPath $InnoCompiler)) {throw 'Inno Setup compiler not found. Run the approved scripts/Install-InnoSetup.ps1 first'}
 & $InnoCompiler '/Q' ('/DVersion='+$version.version) ('/DSourceDir='+$output) ('/DOutputDir='+$artifacts) packaging\usb-relay.iss
 if ($LASTEXITCODE -ne 0) {throw 'Installer build failed'}
 if (-not $SkipSourceArchive) {
  python scripts/package-source.py --version $version.version --output $artifacts
  if ($LASTEXITCODE -ne 0) {throw 'Corresponding source archive failed'}
 }
 $sums=@(Get-ChildItem -LiteralPath $artifacts -File | Where-Object {$_.Name -ne 'SHA256SUMS.txt'} | Sort-Object Name | ForEach-Object { (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()+'  '+$_.Name })
 [IO.File]::WriteAllLines((Join-Path $artifacts 'SHA256SUMS.txt'),$sums,(New-Object Text.UTF8Encoding($false)))
 Get-ChildItem -LiteralPath $artifacts -File | Select-Object Name,Length
} finally { $env:USB_RELAY_KEYSTORE=$oldKey;$env:USB_RELAY_SIGNING_PASSWORD=$oldPassword;Pop-Location }
