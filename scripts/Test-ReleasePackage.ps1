param([string]$Artifacts = (Join-Path (Split-Path $PSScriptRoot -Parent) 'build\release\artifacts'))
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.IO.Compression.FileSystem
$portable=@(Get-ChildItem -LiteralPath $Artifacts -Filter '*portable.zip')
if ($portable.Count -ne 1) {throw 'Expected one portable archive'}
$archive=[IO.Compression.ZipFile]::OpenRead($portable[0].FullName)
try {
 $names=@($archive.Entries | ForEach-Object {$_.FullName.Replace('\','/')})
 foreach($required in @('UsbRelay.exe','usb-relayd.exe','usb-transfer-lab.apk','coreclr.dll','LICENSE.txt','START-HERE.html','THIRD_PARTY_NOTICES.md','BUILD-INFO.json')) {
  if ($required -notin $names) {throw "Missing release file: $required"}
 }
 foreach($name in $names) {
  if ($name -match '(^|/)(\.local|\.git|progress\.md|desktop-plan\.md)(/|$)' -or $name -match '\.(jks|keystore|dpapi|pfx|p12)$' -or $name -match '(^|/)(adb\.exe|AdbWinApi\.dll|AdbWinUsbApi\.dll)$') {throw "Forbidden release payload: $name"}
 }
 $noticeFiles=@($names | Where-Object {$_ -match 'third_party/.*(LICENSE|NOTICE)'})
 if ($noticeFiles.Count -lt 8) {throw 'Missing dependency/runtime license notices'}
} finally {$archive.Dispose()}
foreach($line in Get-Content -LiteralPath (Join-Path $Artifacts 'SHA256SUMS.txt')) {
 if ($line -cnotmatch '^([a-f0-9]{64})  ([A-Za-z0-9._-]+)$') {throw ('Invalid checksum line length: '+$line.Length+' content: '+$line)}
 if ((Get-FileHash -LiteralPath (Join-Path $Artifacts $Matches[2]) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Matches[1]) {throw 'Release checksum mismatch'}
}
Write-Output 'PASS: self-contained release, required guides/licenses, no private state or Google SDK binaries, matching SHA-256 checksums'
