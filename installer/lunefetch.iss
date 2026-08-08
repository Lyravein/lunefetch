#ifndef AppVersion
  #error AppVersion must be provided with /DAppVersion=x.y.z
#endif
#ifndef SourceDir
  #error SourceDir must be provided with /DSourceDir=path
#endif
#ifndef OutputDir
  #error OutputDir must be provided with /DOutputDir=path
#endif

#define AppName "Lunefetch"
#define Publisher "lyravein"
#define NativeHost "com.lyravein.lunefetch"

[Setup]
AppId={{E7B78F90-0B97-48FA-920E-4469194F85BB}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#Publisher}
AppPublisherURL=https://github.com/Lyravein/lunefetch
AppSupportURL=https://github.com/Lyravein/lunefetch/issues
DefaultDirName={localappdata}\Programs\Lunefetch
DefaultGroupName=Lunefetch
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutputDir}
OutputBaseFilename=Lunefetch-Setup-{#AppVersion}-windows-amd64
SetupIconFile={#SourceDir}\lunefetch.ico
UninstallDisplayIcon={app}\lunefetch.ico
Compression=lzma2
SolidCompression=yes
WizardStyle=modern

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Files]
Source: "{#SourceDir}\lunefetch.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\lunefetch-native-host.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\lunefetch.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Lunefetch"; Filename: "{app}\lunefetch.exe"; WorkingDir: "{app}"; IconFilename: "{app}\lunefetch.ico"
Name: "{group}\Uninstall Lunefetch"; Filename: "{uninstallexe}"
Name: "{autodesktop}\Lunefetch"; Filename: "{app}\lunefetch.exe"; WorkingDir: "{app}"; IconFilename: "{app}\lunefetch.ico"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Google\Chrome\NativeMessagingHosts\{#NativeHost}"; ValueType: string; ValueName: ""; ValueData: "{app}\{#NativeHost}.json"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Chromium\NativeMessagingHosts\{#NativeHost}"; ValueType: string; ValueName: ""; ValueData: "{app}\{#NativeHost}.json"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\{#NativeHost}"; ValueType: string; ValueName: ""; ValueData: "{app}\{#NativeHost}.json"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Microsoft\Edge\NativeMessagingHosts\{#NativeHost}"; ValueType: string; ValueName: ""; ValueData: "{app}\{#NativeHost}.json"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Vivaldi\NativeMessagingHosts\{#NativeHost}"; ValueType: string; ValueName: ""; ValueData: "{app}\{#NativeHost}.json"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Mozilla\NativeMessagingHosts\{#NativeHost}"; ValueType: string; ValueName: ""; ValueData: "{app}\{#NativeHost}-firefox.json"; Flags: uninsdeletekey

[Run]
Filename: "{app}\lunefetch.exe"; Description: "Launch Lunefetch"; WorkingDir: "{app}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: files; Name: "{app}\{#NativeHost}.json"
Type: files; Name: "{app}\{#NativeHost}-firefox.json"
Type: dirifempty; Name: "{app}"

[Code]
function JSONPath(const Value: String): String;
begin
  Result := Value;
  StringChangeEx(Result, '\', '\\', True);
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  HostPath: String;
  ChromiumManifest: String;
  FirefoxManifest: String;
begin
  if CurStep <> ssPostInstall then
    Exit;

  HostPath := JSONPath(ExpandConstant('{app}\lunefetch-native-host.exe'));
  ChromiumManifest :=
    '{"name":"{#NativeHost}","description":"Lunefetch native messaging host",' +
    '"path":"' + HostPath + '","type":"stdio",' +
    '"allowed_origins":["chrome-extension://iidkhocioaefjlhhigiaphejnlidchke/"]}';
  FirefoxManifest :=
    '{"name":"{#NativeHost}","description":"Lunefetch native messaging host",' +
    '"path":"' + HostPath + '","type":"stdio",' +
    '"allowed_extensions":["lunefetch@lyravein"]}';

  SaveStringToFile(ExpandConstant('{app}\{#NativeHost}.json'), ChromiumManifest, False);
  SaveStringToFile(ExpandConstant('{app}\{#NativeHost}-firefox.json'), FirefoxManifest, False);
end;
