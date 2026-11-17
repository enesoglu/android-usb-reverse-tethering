#ifndef Version
  #define Version "0.4.0-alpha.1"
#endif
#ifndef SourceDir
  #define SourceDir "..\build\release\windows-x64"
#endif
#ifndef OutputDir
  #define OutputDir "..\build\release\artifacts"
#endif
[Setup]
AppId={{3EBE3FC3-5B95-42D9-A8EC-75284866E5B4}
AppName=USB Relay
AppVersion={#Version}
AppPublisher=USB Relay contributors
AppPublisherURL=https://github.com/enesoglu/android-usb-reverse-tethering
AppSupportURL=https://github.com/enesoglu/android-usb-reverse-tethering/issues
AppUpdatesURL=https://github.com/enesoglu/android-usb-reverse-tethering/releases
DefaultDirName={localappdata}\Programs\USBRelay
DefaultGroupName=USB Relay
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.19045
DisableProgramGroupPage=yes
WizardStyle=modern
Compression=lzma2
SolidCompression=yes
CloseApplications=yes
RestartApplications=no
InfoBeforeFile=setup-notice.txt
OutputDir={#OutputDir}
OutputBaseFilename=USB-Relay-{#Version}-windows-x64-setup
SetupIconFile=..\desktop\UsbRelay\Assets\usb-relay.ico
UninstallDisplayIcon={app}\UsbRelay.exe
VersionInfoVersion=0.4.0.1
[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; Flags: unchecked
[Files]
Source: "{#SourceDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs
[Icons]
Name: "{group}\USB Relay"; Filename: "{app}\UsbRelay.exe"
Name: "{group}\Installation guide"; Filename: "{app}\START-HERE.html"
Name: "{autodesktop}\USB Relay"; Filename: "{app}\UsbRelay.exe"; Tasks: desktopicon
[Run]
Filename: "{app}\UsbRelay.exe"; Description: "Open USB Relay"; Flags: nowait postinstall skipifsilent
[UninstallRun]
Filename: "{app}\UsbRelay.exe"; Parameters: "--exit --wait"; Flags: runhidden waituntilterminated skipifdoesntexist; RunOnceId: "StopUSBRelay"
[Code]
function PrepareToInstall(var NeedsRestart: Boolean): String;
var Code: Integer;
begin
  Result := '';
  if FileExists(ExpandConstant('{app}\UsbRelay.exe')) then
    if not Exec(ExpandConstant('{app}\UsbRelay.exe'), '--exit --wait', '', SW_HIDE, ewWaitUntilTerminated, Code) then
      Result := 'Please exit USB Relay from its tray menu and try again.';
end;
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var Current, Expected, LegacyExpected: String;
begin
  if CurUninstallStep = usUninstall then begin
    LegacyExpected := '"' + ExpandConstant('{app}\UsbRelay.exe') + '" --background';
    Expected := '"' + ExpandConstant('{app}\UsbRelay.exe') + '" --startup';
    if RegQueryStringValue(HKCU, 'Software\Microsoft\Windows\CurrentVersion\Run', 'UsbReverseTether', Current) then
      if (CompareText(Current, Expected) = 0) or (CompareText(Current, LegacyExpected) = 0) then
        RegDeleteValue(HKCU, 'Software\Microsoft\Windows\CurrentVersion\Run', 'UsbReverseTether');
  end;
end;
