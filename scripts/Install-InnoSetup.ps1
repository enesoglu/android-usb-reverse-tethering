$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$folder = Join-Path $root '.local\release-tools'
New-Item -ItemType Directory -Force -Path $folder | Out-Null
$installer = Join-Path $folder 'innosetup-7.1.1-x64.exe'
$hash = '8a5a6711c3766a6e5fb74f280ba0e3551e719ce94538d901fdbd1f8f9920b3f0'
if (-not (Test-Path -LiteralPath $installer)) { Invoke-WebRequest -UseBasicParsing -Uri 'https://github.com/jrsoftware/issrc/releases/download/is-7_1_1/innosetup-7.1.1-x64.exe' -OutFile $installer }
if ((Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash.ToLowerInvariant() -ne $hash) { throw 'Installer compiler checksum mismatch' }
$signature = Get-AuthenticodeSignature -LiteralPath $installer
if ($signature.Status -ne 'Valid' -or $signature.SignerCertificate.Subject -notmatch 'CN=Pyrsys B\.V\.') { throw 'Installer compiler publisher verification failed' }
$destination = Join-Path $folder 'InnoSetup7'
if (-not (Test-Path -LiteralPath (Join-Path $destination 'ISCC.exe'))) {
    $result = Start-Process -FilePath $installer -ArgumentList @('/CURRENTUSER','/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART','/NOICONS','/TASKS=',('/DIR="'+$destination+'"')) -WindowStyle Hidden -Wait -PassThru
    if ($result.ExitCode -ne 0) { throw 'Installer compiler setup failed' }
}
Write-Output (Join-Path $destination 'ISCC.exe')
