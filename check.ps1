param([string]$Jdk = "$env:ProgramFiles\Android\Android Studio\jbr", [string]$Sdk = "$env:LOCALAPPDATA\Android\Sdk")
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    $executable = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot 'build\desktop\UsbRelay.exe'))
    if (@(Get-Process -Name UsbRelay -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $executable }).Count -gt 0) { throw 'Exit USB Relay from its tray menu before running checks' }
    go test ./... -count=1 -timeout=60s
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed' }
    python -m unittest discover -v
    if ($LASTEXITCODE -ne 0) { throw 'Python tests failed' }
    & "$PSScriptRoot\build-android.ps1" -Jdk $Jdk -Sdk $Sdk
    & "$PSScriptRoot\build-security-test.ps1" -Jdk $Jdk -Sdk $Sdk
    New-Item -ItemType Directory -Force -Path "$PSScriptRoot\build\test-classes" | Out-Null
    & "$Jdk\bin\javac.exe" --release 8 -classpath "$PSScriptRoot\build\classes" -d "$PSScriptRoot\build\test-classes" "$PSScriptRoot\tests\java\FrameDecoderTest.java" "$PSScriptRoot\tests\java\ConnectionPolicyTest.java"
    if ($LASTEXITCODE -ne 0) { throw 'Java test compilation failed' }
    & "$Jdk\bin\java.exe" -cp "$PSScriptRoot\build\classes;$PSScriptRoot\build\test-classes" FrameDecoderTest
    if ($LASTEXITCODE -ne 0) { throw 'Java protocol tests failed' }
    & "$Jdk\bin\java.exe" -cp "$PSScriptRoot\build\classes;$PSScriptRoot\build\test-classes" dev.usbreverse.lab.ConnectionPolicyTest
    if ($LASTEXITCODE -ne 0) { throw 'Android reconnect policy tests failed' }
    dotnet format whitespace desktop/UsbRelay/UsbRelay.csproj --verify-no-changes --no-restore --verbosity quiet
    if ($LASTEXITCODE -ne 0) { throw 'Desktop formatting check failed' }
    & "$PSScriptRoot\build-desktop.ps1" -SkipAndroid
    $preview = Join-Path $PSScriptRoot '.local\desktop-preview.png'
    $test = Start-Process -FilePath $executable -ArgumentList @('--self-test','--output',('"' + $preview + '"')) -WindowStyle Hidden -Wait -PassThru
    if ($test.ExitCode -ne 0) { throw 'Native desktop self-test failed; see .local preview diagnostics' }
    Get-Content -LiteralPath ([System.IO.Path]::ChangeExtension($preview, 'json'))
} finally { Pop-Location }
