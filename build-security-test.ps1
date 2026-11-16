param([string]$Jdk = "$env:ProgramFiles\Android\Android Studio\jbr", [string]$Sdk = "$env:LOCALAPPDATA\Android\Sdk")
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    $tools = "$sdk\build-tools\37.0.0"
    $jar = "$sdk\platforms\android-36\android.jar"
    $folder = Join-Path $PSScriptRoot 'build\security-test'
    New-Item -ItemType Directory -Force -Path "$folder\classes","$folder\dex" | Out-Null
    & "$jdk\bin\javac.exe" --release 11 -classpath $jar -d "$folder\classes" android\security-test\PermissionProbe.java
    if ($LASTEXITCODE -ne 0) { throw 'Security probe compilation failed' }
    $classes = @(Get-ChildItem "$folder\classes" -Filter '*.class' -Recurse | ForEach-Object FullName)
    & "$tools\d8.bat" --lib $jar --min-api 29 --output "$folder\dex" @classes
    if ($LASTEXITCODE -ne 0) { throw 'Security probe dex build failed' }
    & "$tools\aapt2.exe" link -o "$folder\unsigned.apk" --manifest android\security-test\AndroidManifest.xml -I $jar --min-sdk-version 29 --target-sdk-version 36
    if ($LASTEXITCODE -ne 0) { throw 'Security probe packaging failed' }
    & "$jdk\bin\jar.exe" uf "$folder\unsigned.apk" -C "$folder\dex" classes.dex
    & "$tools\zipalign.exe" -f 4 "$folder\unsigned.apk" "$folder\aligned.apk"
    & "$tools\apksigner.bat" sign --ks .local\debug.keystore --ks-pass pass:android --key-pass pass:android --out "$folder\security-test.apk" "$folder\aligned.apk"
    if ($LASTEXITCODE -ne 0) { throw 'Security probe signing failed' }
    Write-Output "$folder\security-test.apk"
} finally { Pop-Location }
