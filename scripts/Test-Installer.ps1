$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$version=Get-Content -Raw (Join-Path $root 'version.json') | ConvertFrom-Json
$installer=Join-Path $root ('build\release\artifacts\USB-Relay-'+$version.version+'-windows-x64-setup.exe')
$target=Join-Path $root '.local\installer-smoke'
$exe=Join-Path $target 'UsbRelay.exe'
if (Test-Path -LiteralPath $target) {throw 'Installer smoke directory already exists; inspect it before rerunning'}
if (Get-Process -Name UsbRelay -ErrorAction SilentlyContinue) {throw 'Exit USB Relay before installer smoke tests'}
$runPath='HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$before=(Get-ItemProperty -LiteralPath $runPath -Name UsbReverseTether -ErrorAction SilentlyContinue).UsbReverseTether
$arguments=@('/CURRENTUSER','/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART',('/DIR="'+$target+'"'),('/LOG="'+(Join-Path $root '.local\installer-smoke.log')+'"'))
$first=Start-Process -FilePath $installer -ArgumentList $arguments -WindowStyle Hidden -Wait -PassThru
if ($first.ExitCode -ne 0 -or -not (Test-Path -LiteralPath $exe)) {throw 'First installer run failed'}
$primary=$null
try {
 $self=Start-Process -FilePath $exe -ArgumentList @('--self-test','--output',('"'+(Join-Path $root '.local\installed-preview.png')+'"')) -WindowStyle Hidden -Wait -PassThru
 if ($self.ExitCode -ne 0) {throw 'Installed self-contained app failed its native self-tests'}
 $primary=Start-Process -FilePath $exe -ArgumentList '--background' -WindowStyle Hidden -PassThru
 Start-Sleep -Seconds 2
 $update=Start-Process -FilePath $installer -ArgumentList $arguments -WindowStyle Hidden -Wait -PassThru
 if ($update.ExitCode -ne 0) {throw 'Updating a running installed app failed'}
 if (-not $primary.WaitForExit(15000)) {throw 'Installer did not cleanly stop the old app'}
 $expected=Join-Path $root 'build\release\windows-x64\UsbRelay.exe'
 if ((Get-FileHash -LiteralPath $expected).Hash -ne (Get-FileHash -LiteralPath $exe).Hash) {throw 'Updated app differs from the built release'}
} finally {
 if ($primary -and -not $primary.HasExited) {Start-Process -FilePath $exe -ArgumentList @('--exit','--wait') -WindowStyle Hidden -Wait}
 $uninstaller=Join-Path $target 'unins000.exe'
 if (Test-Path -LiteralPath $uninstaller) {
  $uninstall=Start-Process -FilePath $uninstaller -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART') -WindowStyle Hidden -Wait -PassThru
  if ($uninstall.ExitCode -ne 0) {throw 'Uninstaller failed'}
 }
}
if (Test-Path -LiteralPath $exe) {throw 'Installed app remained after uninstall'}
$after=(Get-ItemProperty -LiteralPath $runPath -Name UsbReverseTether -ErrorAction SilentlyContinue).UsbReverseTether
if ($before -ne $after) {throw 'Installer changed an unrelated startup registration'}
Write-Output 'PASS: per-user install, self-contained startup, update of running app, graceful shutdown, uninstall, preserved startup state'
