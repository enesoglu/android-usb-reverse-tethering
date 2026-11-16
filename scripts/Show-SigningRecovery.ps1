# Maintainer-only local UI. Never run this in a GitHub workflow or paste its password into chat.
$ErrorActionPreference='Stop'
Add-Type -AssemblyName PresentationFramework
Add-Type -AssemblyName System.Security
$folder=Join-Path $env:LOCALAPPDATA 'UsbRelayMaintainer\signing'
$key=Join-Path $folder 'usb-relay-release.jks'
$encrypted=[IO.File]::ReadAllBytes((Join-Path $folder 'password.dpapi'))
$password=[Text.Encoding]::UTF8.GetString([Security.Cryptography.ProtectedData]::Unprotect($encrypted,$null,[Security.Cryptography.DataProtectionScope]::CurrentUser))
$window=New-Object Windows.Window
$window.Title='USB Relay — Maintainer signing-key recovery'
$window.Width=650;$window.Height=390;$window.WindowStartupLocation='CenterScreen'
$panel=New-Object Windows.Controls.StackPanel
$panel.Margin=24
$window.Content=$panel
$text=New-Object Windows.Controls.TextBlock
$text.Text='Keep an independent recovery backup. Save the encrypted keystore somewhere secure and put its password in your password manager. The local DPAPI password file cannot be moved to another Windows account as a recovery method. GitHub Secrets cannot be read back as a backup.'
$text.TextWrapping='Wrap';$text.Margin='0,0,0,18';$panel.Children.Add($text)|Out-Null
$copy=New-Object Windows.Controls.Button
$copy.Content='Copy signing password to clipboard';$copy.Margin='0,0,0,12'
$copy.Add_Click({[Windows.Clipboard]::SetText($password);$status.Text='Password copied. Save it in your password manager before closing; this window clears its own clipboard value on close.'})
$panel.Children.Add($copy)|Out-Null
$save=New-Object Windows.Controls.Button
$save.Content='Save a backup of the encrypted keystore';$save.Margin='0,0,0,12'
$save.Add_Click({
 $dialog=New-Object Microsoft.Win32.SaveFileDialog
 $dialog.FileName='USB-Relay-Android-release.jks';$dialog.Filter='Encrypted Android keystore|*.jks'
 if($dialog.ShowDialog()) {if([IO.File]::Exists($dialog.FileName)) {$status.Text='Choose a new filename; existing backups are not overwritten.'} else {[IO.File]::Copy($key,$dialog.FileName,$false);$status.Text='Encrypted keystore copied. Store its password independently in a secure password manager.'}}
})
$panel.Children.Add($save)|Out-Null
$status=New-Object Windows.Controls.TextBlock
$status.Text='Private key and password are never displayed in command output or committed to the repository.'
$status.TextWrapping='Wrap';$panel.Children.Add($status)|Out-Null
$window.Add_Closed({try{if([Windows.Clipboard]::GetText() -ceq $password){[Windows.Clipboard]::Clear()}}catch{} })
$window.ShowDialog()|Out-Null
$password=$null
