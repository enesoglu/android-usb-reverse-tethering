$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$exe = [System.IO.Path]::GetFullPath((Join-Path $root 'build\desktop\UsbRelay.exe'))
$existing = @(Get-Process -Name UsbRelay -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe })
if ($existing.Count -gt 0) { throw 'Exit USB Relay from its tray before lifecycle testing' }
$runPath = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$before = (Get-ItemProperty -LiteralPath $runPath -Name UsbReverseTether -ErrorAction SilentlyContinue).UsbReverseTether
$main = Start-Process -FilePath $exe -PassThru
try {
    Start-Sleep -Seconds 3
    if ($main.HasExited) { throw 'Desktop exited unexpectedly' }
    $second = Start-Process -FilePath $exe -PassThru
    if (-not $second.WaitForExit(10000)) { throw 'Second instance did not signal the primary' }
    $instances = @(Get-Process -Name UsbRelay -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe })
    if ($instances.Count -ne 1) { throw 'Expected exactly one desktop instance' }
    $disconnect = Start-Process -FilePath $exe -ArgumentList '--disconnect' -PassThru
    if (-not $disconnect.WaitForExit(10000)) { throw 'Disconnect command did not finish' }
    $exit = Start-Process -FilePath $exe -ArgumentList '--exit' -PassThru
    if (-not $exit.WaitForExit(10000)) { throw 'Exit command did not finish' }
    if (-not $main.WaitForExit(20000)) { throw 'Primary desktop did not exit cleanly' }
    $daemon = Join-Path $root 'build\desktop\usb-relayd.exe'
    if (@(Get-Process -Name usb-relayd -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $daemon }).Count -gt 0) { throw 'Child daemon remained after Exit' }
    $after = (Get-ItemProperty -LiteralPath $runPath -Name UsbReverseTether -ErrorAction SilentlyContinue).UsbReverseTether
    if ($before -ne $after) { throw 'Lifecycle test changed the actual startup registration' }
    Write-Output 'PASS: single instance, disconnect command, graceful Exit, child cleanup and unchanged startup registration'
} finally {
    if (-not $main.HasExited) { Start-Process -FilePath $exe -ArgumentList '--exit' -WindowStyle Hidden -Wait }
}
