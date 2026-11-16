param(
 [string]$Sdk = "$env:LOCALAPPDATA\Android\Sdk",
 [string]$Jdk = "$env:ProgramFiles\Android\Android Studio\jbr",
 [switch]$PublicRelease,
 [string]$KeyStore = $env:USB_RELAY_KEYSTORE,
 [string]$KeyAlias = 'usb-relay-release'
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$project = $PSScriptRoot
$build = Join-Path $project $(if ($PublicRelease) { 'build\public' } else { 'build' })
$tools = Join-Path $Sdk 'build-tools\37.0.0'
$androidJar = Join-Path $Sdk 'platforms\android-36\android.jar'
function Invoke-Checked([string]$Program, [string[]]$Arguments) {
 & $Program @Arguments
 if ($LASTEXITCODE -ne 0) { throw "$Program failed with exit code $LASTEXITCODE" }
}
if ($PublicRelease -and (-not $KeyStore -or -not $env:USB_RELAY_SIGNING_PASSWORD)) { throw 'Public builds require a release keystore and USB_RELAY_SIGNING_PASSWORD; development-key fallback is forbidden' }
New-Item -ItemType Directory -Force -Path "$build\classes","$build\dex","$project\.local" | Out-Null
$classRoot = [IO.Path]::GetFullPath("$build\classes")
$expectedRoot = [IO.Path]::GetFullPath("$project\build") + [IO.Path]::DirectorySeparatorChar
if (-not $classRoot.StartsWith($expectedRoot,[StringComparison]::OrdinalIgnoreCase)) { throw 'Class output escapes build directory' }
if (Test-Path -LiteralPath $classRoot) { Remove-Item -LiteralPath $classRoot -Recurse -Force }
New-Item -ItemType Directory -Path $classRoot | Out-Null
$sources = @(Get-ChildItem -LiteralPath "$project\android\src" -Filter '*.java' | ForEach-Object FullName)
Invoke-Checked "$Jdk\bin\javac.exe" (@('-encoding','UTF-8','--release','11','-classpath',$androidJar,'-d',$classRoot) + $sources)
$classFiles = @(Get-ChildItem -LiteralPath "$build\classes\dev\usbreverse\lab" -Filter '*.class' -Recurse | ForEach-Object FullName)
Invoke-Checked "$tools\d8.bat" (@('--lib',$androidJar,'--min-api','29','--output',"$build\dex") + $classFiles)
$manifestPath = "$project\android\AndroidManifest.xml"
if ($PublicRelease) {
 $version = Get-Content -Raw -LiteralPath "$project\version.json" | ConvertFrom-Json
 [xml]$manifest = Get-Content -Raw -LiteralPath $manifestPath
 $ns = 'http://schemas.android.com/apk/res/android'
 $manifest.DocumentElement.SetAttribute('package', $version.androidPackage)
 $manifest.DocumentElement.SetAttribute('versionCode', $ns, [string]$version.androidVersionCode) | Out-Null
 $manifest.DocumentElement.SetAttribute('versionName', $ns, $version.version) | Out-Null
 $manifest.manifest.application.SetAttribute('label', $ns, 'USB Relay') | Out-Null
 foreach ($node in $manifest.SelectNodes('//*[@*[local-name()="name"]]')) {
  $name = $node.GetAttribute('name', $ns)
  if ($name.StartsWith('.')) { $node.SetAttribute('name', $ns, ('dev.usbreverse.lab' + $name)) | Out-Null }
 }
 $manifest.manifest.application.provider.SetAttribute('authorities',$ns,($version.androidPackage+'.diagnostics')) | Out-Null
 $manifestPath = Join-Path $build 'AndroidManifest.xml'
 $manifest.Save($manifestPath)
}
Invoke-Checked "$tools\aapt2.exe" @('link','-o',"$build\unsigned.apk",'--manifest',$manifestPath,'-I',$androidJar,'-A',"$project\android\assets",'--min-sdk-version','29','--target-sdk-version','36')
Invoke-Checked "$Jdk\bin\jar.exe" @('uf',"$build\unsigned.apk",'-C',"$build\dex",'classes.dex')
Invoke-Checked "$tools\zipalign.exe" @('-f','4',"$build\unsigned.apk","$build\aligned.apk")
if ($PublicRelease) {
 Invoke-Checked "$tools\apksigner.bat" @('sign','--ks',$KeyStore,'--ks-key-alias',$KeyAlias,'--ks-pass','env:USB_RELAY_SIGNING_PASSWORD','--key-pass','env:USB_RELAY_SIGNING_PASSWORD','--out',"$build\usb-transfer-lab.apk","$build\aligned.apk")
} else {
 $key = "$project\.local\debug.keystore"
 if (-not (Test-Path -LiteralPath $key)) { Invoke-Checked "$Jdk\bin\keytool.exe" @('-genkeypair','-keystore',$key,'-storepass','android','-keypass','android','-alias','androiddebugkey','-keyalg','RSA','-keysize','2048','-validity','3650','-dname','CN=USB Transfer Lab Debug') }
 Invoke-Checked "$tools\apksigner.bat" @('sign','--ks',$key,'--ks-pass','pass:android','--key-pass','pass:android','--out',"$build\usb-transfer-lab.apk","$build\aligned.apk")
}
Invoke-Checked "$tools\apksigner.bat" @('verify',"$build\usb-transfer-lab.apk")
Write-Output "$build\usb-transfer-lab.apk"
